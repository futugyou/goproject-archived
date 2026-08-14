package email

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type EmailTool struct {
	config        core.EmailConfig
	toolingConfig *core.ToolingConfig
}

func NewEmailTool(config core.EmailConfig, toolingConfig *core.ToolingConfig) *EmailTool {
	if config.MaxResults <= 0 {
		config.MaxResults = 10
	}
	return &EmailTool{
		config:        config,
		toolingConfig: toolingConfig,
	}
}

func (e *EmailTool) Name() string {
	return "email"
}

func (e *EmailTool) Description() string {
	return "Send and read emails. Supports sending via SMTP and reading via IMAP."
}

func (e *EmailTool) ParameterSchema() string {
	return `{
	  "type": "object",
	  "properties": {
	    "action": {
	      "type": "string",
	      "description": "Action to perform",
	      "enum": ["send", "list", "read", "search"]
	    },
	    "to": {
	      "type": "string",
	      "description": "Recipient email address (for send)"
	    },
	    "subject": {
	      "type": "string",
	      "description": "Email subject (for send)"
	    },
	    "body": {
	      "type": "string",
	      "description": "Email body text (for send)"
	    },
	    "folder": {
	      "type": "string",
	      "description": "IMAP folder to read from (default: INBOX)",
	      "default": "INBOX"
	    },
	    "message_id": {
	      "type": "string",
	      "description": "Message number to read (for read action)"
	    },
	    "query": {
	      "type": "string",
	      "description": "Search query (for search action, uses IMAP SEARCH syntax)"
	    },
	    "count": {
	      "type": "integer",
	      "description": "Number of messages to list (default: 10)",
	      "default": 10
	    }
	  },
	  "required": ["action"]
	}`
}

type emailArgs struct {
	Action    string `json:"action"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Folder    string `json:"folder"`
	MessageID string `json:"message_id"`
	Query     string `json:"query"`
	Count     int    `json:"count"`
}

func (e *EmailTool) Execute(ctx context.Context, argumentsJson string) string {
	var args emailArgs
	// 设置参数默认值
	args.Folder = "INBOX"
	args.Count = 10

	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("Failed to parse parameters.: %v", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))

	if e.toolingConfig != nil && e.toolingConfig.ReadOnlyMode && action == "send" {
		return "Error: email send action is disabled because Tooling.ReadOnlyMode is enabled."
	}

	switch action {
	case "send":
		return e.sendEmail(ctx, args)
	case "list":
		return e.listEmails(ctx, args)
	case "read":
		return e.readEmail(ctx, args)
	case "search":
		return e.searchEmails(ctx, args)
	default:
		return fmt.Sprintf("Error: Unsupported email action '%s'. Use: send, list, read, search.", action)
	}
}

func (e *EmailTool) sendEmail(_ context.Context, args emailArgs) string {
	if strings.TrimSpace(e.config.SmtpHost) == "" {
		return "Error: SMTP host not configured. Set Email.SmtpHost."
	}

	if strings.TrimSpace(args.To) == "" {
		return "Error: 'to' is required to send an email."
	}
	if strings.TrimSpace(args.Subject) == "" {
		return "Error: 'subject' is required to send an email."
	}

	password := core.SecretResolverInstance.Resolve(e.config.PasswordRef)
	if strings.TrimSpace(e.config.Username) == "" || strings.TrimSpace(password) == "" {
		return "Error: Email credentials not configured. Set Email.Username and Email.PasswordRef."
	}

	from := e.config.FromAddress
	if strings.TrimSpace(from) == "" {
		from = e.config.Username
	}

	addr := fmt.Sprintf("%s:%d", e.config.SmtpHost, e.config.SmtpPort)
	auth := smtp.PlainAuth("", e.config.Username, password, e.config.SmtpHost)

	// 构建 RFC 822 格式的标准邮件内容
	msg := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", from, args.To, args.Subject, args.Body))

	err := smtp.SendMail(addr, auth, from, []string{args.To}, msg)
	if err != nil {
		return fmt.Sprintf("Error: Failed to send email — %s", err.Error())
	}

	return fmt.Sprintf("Email sent successfully.\nTo: %s\nSubject: %s", args.To, args.Subject)
}

func (e *EmailTool) listEmails(ctx context.Context, args emailArgs) string {
	folder := args.Folder
	if folder == "" {
		folder = "INBOX"
	}
	folder = core.Sanitizer.StripCrlf(folder)
	if err := core.Sanitizer.CheckImapFolderName(folder); err != nil {
		return err.Error()
	}

	count := args.Count
	if count <= 0 {
		count = 10
	}
	if count > e.config.MaxResults {
		count = e.config.MaxResults
	}

	return e.executeImap(ctx, func(reader *bufio.Reader, conn net.Conn) (string, error) {
		total, err := imapSelect(conn, reader, folder)
		if err != nil {
			return "", err
		}
		if total == 0 {
			return "No messages in folder.", nil
		}

		startMsg := total - count + 1
		if startMsg < 1 {
			startMsg = 1
		}

		showing := total - startMsg + 1
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Folder: %s (%d messages, showing %d most recent)\n\n", folder, total, showing))

		for i := total; i >= startMsg; i-- {
			headers, err := imapFetchHeaders(conn, reader, i)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i, headers.Subject))
			sb.WriteString(fmt.Sprintf("    From: %s\n", headers.From))
			sb.WriteString(fmt.Sprintf("    Date: %s\n\n", headers.Date))
		}

		return sb.String(), nil
	})
}

func (e *EmailTool) readEmail(ctx context.Context, args emailArgs) string {
	if strings.TrimSpace(args.MessageID) == "" {
		return "Error: 'message_id' (message number) is required to read an email."
	}
	msgNum, err := strconv.Atoi(args.MessageID)
	if err != nil {
		return "Error: 'message_id' (message number) is required to read an email."
	}

	folder := args.Folder
	if folder == "" {
		folder = "INBOX"
	}
	folder = core.Sanitizer.StripCrlf(folder)
	if err := core.Sanitizer.CheckImapFolderName(folder); err != nil {
		return err.Error()
	}

	return e.executeImap(ctx, func(reader *bufio.Reader, conn net.Conn) (string, error) {
		_, err := imapSelect(conn, reader, folder)
		if err != nil {
			return "", err
		}

		body, err := imapFetchBody(conn, reader, msgNum)
		if err != nil {
			return "", err
		}
		return body, nil
	})
}

func (e *EmailTool) searchEmails(ctx context.Context, args emailArgs) string {
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "Error: 'query' is required for search."
	}

	folder := args.Folder
	if folder == "" {
		folder = "INBOX"
	}

	folder = core.Sanitizer.StripCrlf(folder)
	query = core.Sanitizer.StripCrlf(query)
	if err := core.Sanitizer.CheckImapFolderName(folder); err != nil {
		return err.Error()
	}

	return e.executeImap(ctx, func(reader *bufio.Reader, conn net.Conn) (string, error) {
		_, err := imapSelect(conn, reader, folder)
		if err != nil {
			return "", err
		}

		cmd := fmt.Sprintf("A4 SEARCH %s\r\n", query)
		if _, err := conn.Write([]byte(cmd)); err != nil {
			return "", err
		}

		var ids []int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			line = strings.TrimRight(line, "\r\n")

			if strings.HasPrefix(strings.ToUpper(line), "* SEARCH") {
				parts := strings.Fields(line)
				for i := 2; i < len(parts); i++ {
					if id, err := strconv.Atoi(parts[i]); err == nil {
						ids = append(ids, id)
					}
				}
			}

			if strings.HasPrefix(line, "A4 ") {
				break
			}
		}

		if len(ids) == 0 {
			return "No messages matched the search.", nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))

		showCount := len(ids)
		if showCount > e.config.MaxResults {
			showCount = e.config.MaxResults
		}

		for i := len(ids) - 1; i >= len(ids)-showCount; i-- {
			headers, err := imapFetchHeaders(conn, reader, ids[i])
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("[%d] %s\n", ids[i], headers.Subject))
			sb.WriteString(fmt.Sprintf("    From: %s\n", headers.From))
			sb.WriteString(fmt.Sprintf("    Date: %s\n\n", headers.Date))
		}

		return sb.String(), nil
	})
}

// ── IMAP 底层网络通讯辅助函数 ────────────────────────────────────

func (e *EmailTool) executeImap(_ context.Context, action func(reader *bufio.Reader, conn net.Conn) (string, error)) string {
	if strings.TrimSpace(e.config.ImapHost) == "" {
		return "Error: IMAP host not configured. Set Email.ImapHost."
	}

	password := core.SecretResolverInstance.Resolve(e.config.PasswordRef)
	if strings.TrimSpace(e.config.Username) == "" || strings.TrimSpace(password) == "" {
		return "Error: Email credentials not configured. Set Email.Username and Email.PasswordRef."
	}

	addr := fmt.Sprintf("%s:%d", e.config.ImapHost, e.config.ImapPort)
	tlsConfig := &tls.Config{
		ServerName: e.config.ImapHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %s", err.Error())
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// 读取服务器 Greeting 消息
	if _, err := reader.ReadString('\n'); err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %s", err.Error())
	}

	// 登录 LOGIN
	loginCmd := fmt.Sprintf("A1 LOGIN %s %s\r\n", imapQuote(e.config.Username), imapQuote(password))
	if _, err := conn.Write([]byte(loginCmd)); err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %s", err.Error())
	}

	loginResp, err := readUntilTag(reader, "A1")
	if err != nil || !strings.Contains(strings.ToUpper(loginResp), "OK") {
		return fmt.Sprintf("Error: IMAP login failed — %s", strings.TrimSpace(loginResp))
	}

	// 执行具体逻辑
	result, err := action(reader, conn)
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %s", err.Error())
	}

	// 登出 LOGOUT
	_, _ = conn.Write([]byte("A99 LOGOUT\r\n"))

	return result
}

func imapSelect(conn net.Conn, reader *bufio.Reader, folder string) (int, error) {
	cmd := fmt.Sprintf("A2 SELECT %s\r\n", imapQuote(folder))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return 0, err
	}

	count := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimRight(line, "\r\n")

		if strings.HasSuffix(strings.ToUpper(line), "EXISTS") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if exists, err := strconv.Atoi(parts[1]); err == nil {
					count = exists
				}
			}
		}

		if strings.HasPrefix(line, "A2 ") {
			break
		}
	}

	return count, nil
}

type emailHeaders struct {
	Subject string
	From    string
	Date    string
}

func imapFetchHeaders(conn net.Conn, reader *bufio.Reader, msgNum int) (emailHeaders, error) {
	cmd := fmt.Sprintf("A5 FETCH %d (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM DATE)])\r\n", msgNum)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return emailHeaders{}, err
	}

	var sb strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return emailHeaders{}, err
		}
		sb.WriteString(line)
		if strings.HasPrefix(line, "A5 ") {
			break
		}
	}

	headers := emailHeaders{
		Subject: "(no subject)",
		From:    "(unknown)",
		Date:    "",
	}

	lines := strings.Split(sb.String(), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToLower(trimmed), "subject:") {
			headers.Subject = strings.TrimSpace(trimmed[len("subject:"):])
		} else if strings.HasPrefix(strings.ToLower(trimmed), "from:") {
			headers.From = strings.TrimSpace(trimmed[len("from:"):])
		} else if strings.HasPrefix(strings.ToLower(trimmed), "date:") {
			headers.Date = strings.TrimSpace(trimmed[len("date:"):])
		}
	}

	return headers, nil
}

func imapFetchBody(conn net.Conn, reader *bufio.Reader, msgNum int) (string, error) {
	cmd := fmt.Sprintf("A6 FETCH %d (BODY.PEEK[TEXT])\r\n", msgNum)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", err
	}

	var sb strings.Builder
	inBody := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "A6 ") {
			break
		}

		if strings.Contains(strings.ToUpper(line), "BODY[TEXT]") {
			inBody = true
			continue
		}

		if inBody {
			trimmed := strings.TrimRight(line, "\r\n")
			if trimmed == ")" {
				inBody = false
				continue
			}
			sb.WriteString(line)
		}
	}

	if sb.Len() > 0 {
		return sb.String(), nil
	}
	return "No body content found.", nil
}

func readUntilTag(reader *bufio.Reader, tag string) (string, error) {
	var sb strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return sb.String(), err
		}
		sb.WriteString(line)
		if strings.HasPrefix(line, tag+" ") {
			break
		}
	}
	return sb.String(), nil
}

func imapQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

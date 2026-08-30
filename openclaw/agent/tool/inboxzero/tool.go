package inboxzero

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

var BuiltInProtectedDomains = map[string]struct{}{
	"chase.com": {}, "capitalone.com": {}, "citi.com": {}, "americanexpress.com": {},
	"wellsfargo.com": {}, "bankofamerica.com": {}, "paypal.com": {}, "venmo.com": {},
	"mercury.com": {}, "coinbase.com": {}, "robinhood.com": {}, "schwab.com": {},
	"google.com": {}, "github.com": {}, "apple.com": {}, "microsoft.com": {},
	"united.com": {}, "delta.com": {}, "southwest.com": {}, "aa.com": {},
	"uber.com": {}, "lyft.com": {}, "doordash.com": {},
	"irs.gov": {}, "ssa.gov": {}, "healthcare.gov": {},
}

var ReceiptPatterns = []string{"payment", "invoice", "order", "shipping", "delivery", "receipt", "transaction", "purchase", "refund"}
var ConfirmationPatterns = []string{"confirmation", "confirmed", "appointment", "reservation", "booking", "registration", "scheduled"}
var PromoPatterns = []string{"sale", "% off", "deal", "limited time", "exclusive offer", "discount", "promo", "clearance", "shop now",
	"buy now", "free shipping", "unbeatable", "save big"}

type InboxZeroTool struct {
	config               core.InboxZeroConfig
	emailConfig          core.EmailConfig
	maxImapResponseLines int
}

func NewInboxZeroTool(config core.InboxZeroConfig, emailConfig core.EmailConfig) *InboxZeroTool {
	maxImapResponseLines := util.Clamp(config.MaxResponseLinesPerCommand, 100, 200_000)
	return &InboxZeroTool{config: config, emailConfig: emailConfig, maxImapResponseLines: maxImapResponseLines}
}

func (e *InboxZeroTool) Name() string {
	return "inbox_zero"
}

func (e *InboxZeroTool) Description() string {
	return "AI-powered email triage tool. Analyze, categorize, clean up, and rescue emails. " +
		"Works with any IMAP email provider. Actions: analyze, cleanup, trash-sender, spam-rescue, categorize."
}

func (e *InboxZeroTool) ParameterSchema() string {
	return `
		{
          "type": "object",
          "properties": {
            "action": {
              "type": "string",
              "description": "Action to perform",
              "enum": ["analyze", "cleanup", "trash-sender", "spam-rescue", "categorize"]
            },
            "sender": {
              "type": "string",
              "description": "Sender email address (required for trash-sender action)"
            },
            "folder": {
              "type": "string",
              "description": "IMAP folder to operate on (default: INBOX)",
              "default": "INBOX"
            },
            "count": {
              "type": "integer",
              "description": "Number of emails to process (default: 50, max: configured MaxBatchSize)",
              "default": 50
            }
          },
          "required": ["action"]
        }`
}

type InboxZeroParams struct {
	Action string `json:"action"`
	Sender string `json:"sender"`
	Folder string `json:"folder"`
	Count  int    `json:"count"`
}

func (a *InboxZeroTool) Execute(ctx context.Context, argumentsJson string) string {

	var args InboxZeroParams
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("invalid arguments: %v", err)
	}

	action := strings.ToLower(args.Action)
	switch action {
	case "analyze":
		return a.analyze(ctx, args)
	case "cleanup":
		return a.cleanup(ctx, args)
	case "trash-sender":
		return a.trashBySender(ctx, args)
	default:
		return fmt.Sprintf("Error: Unknown action '%s'. Use: analyze, cleanup, trash-sender, spam-rescue, categorize.", action)

	}
}

func (e *InboxZeroTool) trashBySender(ctx context.Context, args InboxZeroParams) string {
	if args.Sender == "" {
		return "Error: 'sender' is required for trash-sender action."
	}

	var folder = args.Folder
	if folder == "" {
		folder = "INBOX"
	}
	count := min(args.Count, e.config.MaxBatchSize)
	folder = core.Sanitizer.StripCrlf(folder)
	var folderError = core.Sanitizer.CheckImapFolderName(folder)
	if folderError != nil {
		return folderError.Error()
	}

	return e.executeImap(ctx, func(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer) (string, error) {

		e.imapSelect(ctx, reader, writer, folder)
		total, err := e.imapGetMessageCount(ctx, reader, writer, folder)
		if err != nil {
			return "", err
		}
		if total == 0 {
			return fmt.Sprintf("Folder '%s' is empty.", folder), nil

		}
		type Trash struct {
			MsgNum  int
			Subject string
		}
		var startMsg = max(1, total-count+1)
		var toTrash = []Trash{}
		for i := total; i >= startMsg; i-- {
			email, err := e.imapFetchHeadersExtended(ctx, reader, writer, i)
			if err != nil {
				return "", err
			}
			if strings.Contains(email.From, args.Sender) {
				toTrash = append(toTrash, Trash{MsgNum: i, Subject: email.Subject})
			}
		}

		sb := strings.Builder{}
		fmt.Fprintf(&sb, "## Trash by Sender: %s\n", args.Sender)

		if len(toTrash) == 0 {
			fmt.Fprintf(&sb, "No emails found from '%s' in the last %d messages.\n", args.Sender, count)
			return sb.String(), nil
		}

		if e.config.DryRun {
			fmt.Fprintf(&sb, "### 🔍 DRY RUN — %d emails **would** be trashed:\n", len(toTrash))
			for i := 0; i < min(20, len(toTrash)); i++ {
				fmt.Fprintf(&sb, "  - %s\n", toTrash[i].Subject)
			}

			if len(toTrash) > 20 {
				fmt.Fprintf(&sb, "  ... and %d more\n", len(toTrash)-20)
			}
			sb.WriteString("\n")
			sb.WriteString("Set `InboxZero.DryRun=false` in config to apply.\n")
		} else {
			for i := 0; i < len(toTrash); i++ {
				e.imapStoreFlag(ctx, writer, reader, toTrash[i].MsgNum, "\\Deleted")
			}

			// Expunge to actually delete
			writer.WriteString("A_EXP EXPUNGE\n")
			writer.Flush()
			e.readUntilTag(ctx, reader, "A_EXP")

			fmt.Fprintf(&sb, "### 🗑️ Trashed %d emails from '%s'\n", len(toTrash), args.Sender)
			for i := 0; i < min(20, len(toTrash)); i++ {
				fmt.Fprintf(&sb, "  - %s\n", toTrash[i].Subject)
			}
		}

		return sb.String(), nil
	})
}

func (e *InboxZeroTool) cleanup(ctx context.Context, args InboxZeroParams) string {
	var folder = args.Folder
	if folder == "" {
		folder = "INBOX"
	}
	count := min(args.Count, e.config.MaxBatchSize)
	folder = core.Sanitizer.StripCrlf(folder)
	var folderError = core.Sanitizer.CheckImapFolderName(folder)
	if folderError != nil {
		return folderError.Error()
	}

	return e.executeImap(ctx, func(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer) (string, error) {

		e.imapSelect(ctx, reader, writer, folder)
		total, err := e.imapGetMessageCount(ctx, reader, writer, folder)
		if err != nil {
			return err.Error(), nil
		}
		if total == 0 {
			return fmt.Sprintf("Folder '%s' is empty. Nothing to clean up.", folder), nil
		}

		type Archive struct {
			MsgNum   int
			Category string
			Desc     string
		}
		var startMsg = max(1, total-count+1)
		var toArchive = []Archive{}
		var kept = []Archive{}
		for i := total; i >= startMsg; i-- {
			email, err := e.imapFetchHeadersExtended(ctx, reader, writer, i)
			if err != nil {
				return err.Error(), nil
			}
			var category = e.categorizeEmail(email)

			if category == "Newsletter" || category == "Promotional" || category == "Automated" {
				toArchive = append(toArchive, Archive{
					MsgNum:   i,
					Category: category,
					Desc:     fmt.Sprintf("%s: %s", email.From, email.Subject),
				})
			} else {
				kept = append(kept, Archive{
					MsgNum:   i,
					Category: category,
					Desc:     fmt.Sprintf("%s: %s", email.From, email.Subject),
				})

			}
		}

		var sb = strings.Builder{}
		fmt.Fprintf(&sb, "## Cleanup Report: %s\n", folder)
		fmt.Fprintf(&sb, "Scanned: %d emails\n", min(count, total))
		fmt.Fprintf(&sb, "To archive: %d | Kept: %d\n", len(toArchive), len(kept))
		sb.WriteString("\n")

		if len(toArchive) == 0 {
			sb.WriteString("✅ Nothing to clean up — your inbox is already tidy!\n")
			return sb.String(), nil
		}

		if e.config.DryRun {
			sb.WriteString("### 🔍 DRY RUN — No changes made\n")
			sb.WriteString("The following emails **would** be archived:\n")
			sb.WriteString("\n")
			for _, to := range toArchive {
				fmt.Fprintf(&sb, "  [%d] (%s) %s\n", to.MsgNum, to.Category, to.Desc)
			}

			sb.WriteString("\n")
			sb.WriteString("Set `InboxZero.DryRun=false` in config to apply changes.\n")
		} else {
			// Actually archive by adding \\Seen flag and moving to archive
			for _, to := range toArchive {
				e.imapStoreFlag(ctx, writer, reader, to.MsgNum, "\\Seen")
				e.imapMoveToArchive(ctx, writer, reader, to.MsgNum)
			}

			fmt.Fprintf(&sb, "### ✅ Archived %d emails\n", len(toArchive))
			for i := 0; i < min(20, len(toArchive)); i++ {
				to := toArchive[i]
				fmt.Fprintf(&sb, "  [%d] (%s) %s\n", to.MsgNum, to.Category, to.Desc)
			}

			if len(toArchive) > 20 {
				fmt.Fprintf(&sb, "  ... and %d more\n", len(toArchive)-20)
			}
		}

		return sb.String(), nil
	})
}

func (e *InboxZeroTool) imapMoveToArchive(ctx context.Context, writer *bufio.Writer, reader *bufio.Reader, num int) {
	// Try MOVE command first (RFC 6851), fall back to COPY+DELETE
	var tag = fmt.Sprintf("AM%d", num)
	// Most providers support "[Gmail]/All Mail" or "Archive"
	fmt.Fprintf(writer, "%s COPY %d %s", tag, num, imapQuote("Archive"))
	writer.Flush()
	resp, err := e.readUntilTag(ctx, reader, tag)
	if err != nil {
		return
	}

	if strings.Contains(resp, "NO") {
		// Try Gmail-style archive
		var tag2 = fmt.Sprintf("AM2%d", num)
		fmt.Fprintf(writer, "%s COPY %d %s", tag2, num, imapQuote("[Gmail]/All Mail"))
		writer.Flush()
		e.readUntilTag(ctx, reader, tag2)
	}

	// Mark as deleted from current folder
	e.imapStoreFlag(ctx, writer, reader, num, "\\Deleted")
}

func (e *InboxZeroTool) imapStoreFlag(ctx context.Context, writer *bufio.Writer, reader *bufio.Reader, num int, flag string) {
	var tag = fmt.Sprintf("AS%d", num)
	fmt.Fprintf(writer, "%s  STORE %d +FLAGS (%s)\r\n", tag, num, flag)
	writer.Flush()
	e.readUntilTag(ctx, reader, tag)
}

func (e *InboxZeroTool) imapSelect(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, folder string) error {
	if _, err := fmt.Fprintf(writer, "A2 SELECT %s\r\n", imapQuote(folder)); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}
	_, err := e.readUntilTag(ctx, reader, "A2")
	return err
}

func (e *InboxZeroTool) imapGetMessageCount(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, folder string) (int, error) {
	if _, err := fmt.Fprintf(writer, "A3 STATUS %s (MESSAGES)\r\n", imapQuote(folder)); err != nil {
		return 0, err
	}

	if err := writer.Flush(); err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(reader)
	var count = 0
	var lines = 0
	for scanner.Scan() {
		lines++
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		if lines > e.maxImapResponseLines {
			return 0, fmt.Errorf("IMAP STATUS response exceeded maximum lines (%d)", e.maxImapResponseLines)
		}
		line := scanner.Text()

		upperLine := strings.ToUpper(line)
		if idx := strings.Index(upperLine, "MESSAGES"); idx != -1 {
			var numStart = idx + 8
			for numStart < len(line) && !unicode.IsDigit(rune(line[numStart])) {
				numStart++
			}
			var numEnd = numStart
			for numEnd < len(line) && unicode.IsDigit(rune(line[numEnd])) {
				numEnd++
			}
			if numEnd > numStart {
				var err error
				count, err = strconv.Atoi(line[numStart:numEnd])
				if err != nil {
					return 0, fmt.Errorf("failed to parse message count: %w", err)
				}
			}
		}

		if strings.HasPrefix(line, "A3 ") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

func getCategoryPriority(category string) int {
	switch category {
	case "VIP":
		return 0
	case "Protected":
		return 1
	case "Protected Keyword":
		return 2
	case "Receipt":
		return 3
	case "Confirmation":
		return 4
	case "Newsletter":
		return 5
	case "Promotional":
		return 6
	case "Automated":
		return 7
	default:
		return 8

	}
}

func (e *InboxZeroTool) analyze(ctx context.Context, args InboxZeroParams) string {

	var folder = args.Folder
	if folder == "" {
		folder = "INBOX"
	}
	count := min(args.Count, e.config.MaxBatchSize)
	folder = core.Sanitizer.StripCrlf(folder)
	var folderError = core.Sanitizer.CheckImapFolderName(folder)
	if folderError != nil {
		return folderError.Error()
	}

	return e.executeImap(ctx, func(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer) (string, error) {
		if err := e.imapSelect(ctx, reader, writer, folder); err != nil {
			return "", err
		}

		total, err := e.imapGetMessageCount(ctx, reader, writer, folder)
		if err != nil {
			return "", err
		}

		if total == 0 {
			return fmt.Sprintf("Folder '%s' is empty. Nothing to analyze.", folder), nil
		}

		startMsg := int(math.Max(1, float64(total-count+1)))
		categories := make(map[string][]string)

		var summary strings.Builder
		fmt.Fprintf(&summary, "## Inbox Analysis: %s\n", folder)
		fmt.Fprintf(&summary, "Total messages: %d | Analyzing: %d most recent\n\n", total, int(math.Min(float64(count), float64(total))))

		for i := total; i >= startMsg; i-- {
			email, err := e.imapFetchHeadersExtended(ctx, reader, writer, i)
			if err != nil {
				return "", err
			}
			category := e.categorizeEmail(email)
			categories[category] = append(categories[category], fmt.Sprintf("[%d] %s: %s", i, email.From, email.Subject))
		}

		sortedCats := make([]string, 0, len(categories))
		for k := range categories {
			sortedCats = append(sortedCats, k)
		}
		sort.Slice(sortedCats, func(i, j int) bool {
			return getCategoryPriority(sortedCats[i]) < getCategoryPriority(sortedCats[j])
		})

		// Summary counts
		summary.WriteString("### Category Breakdown\n")
		for _, cat := range sortedCats {
			emails := categories[cat]
			fmt.Fprintf(&summary, "- **%s**: %d email(s)\n", cat, len(emails))
		}
		summary.WriteString("\n")

		// Actionable recommendations
		archivable := len(categories["Newsletter"]) + len(categories["Promotional"]) + len(categories["Automated"])
		if archivable > 0 {
			summary.WriteString("### Recommendation\n")
			fmt.Fprintf(&summary, "**%d emails** can be safely archived/cleaned up (Newsletters, Promotions, Automated).\n", archivable)
			if e.config.DryRun {
				summary.WriteString("Run `cleanup` action to see what would be archived. DryRun is currently **ON** (safe mode).\n")
			} else {
				summary.WriteString("Run `cleanup` action to archive them. ⚠️ DryRun is **OFF** — changes will be applied.\n")
			}
		}

		// Detailed listing
		summary.WriteString("\n### Details\n")
		for _, cat := range sortedCats {
			emails := categories[cat]
			fmt.Fprintf(&summary, "\n**%s** (%d):\n", cat, len(emails))

			limit := min(len(emails), 10)
			for _, e := range emails[:limit] {
				fmt.Fprintf(&summary, "  %s\n", e)
			}
			if len(emails) > 10 {
				fmt.Fprintf(&summary, "  ... and %d more\n", len(emails)-10)
			}
		}

		return summary.String(), nil
	})
}

func ExtractDomain(email string) string {
	var atIdx = strings.LastIndex(email, "@")
	if atIdx < 0 {
		return ""
	}

	var domain = strings.TrimRight(strings.TrimSpace(email[(atIdx+1):]), ">")
	return domain
}
func (e *InboxZeroTool) categorizeEmail(email *EmailHeaders) string {
	var fromLower = strings.ToLower(email.From)
	var subjectLower = strings.ToLower(email.Subject)
	var senderDomain = ExtractDomain(fromLower)

	// 1. VIP
	for _, vip := range e.config.VipSenders {
		if strings.Contains(fromLower, vip) {
			return "VIP"
		}
	}

	// 2. Protected sender
	for _, ps := range e.config.ProtectedSenders {
		if strings.Contains(fromLower, ps) {
			return "Protected"
		}
	}

	// 2b. Built-in protected domains
	if _, ok := BuiltInProtectedDomains[senderDomain]; ok && senderDomain != "" {
		return "Protected"
	}

	// 3. Protected keyword
	for _, kw := range e.config.ProtectedKeywords {
		if strings.Contains(subjectLower, kw) {
			return "Protected"
		}
	}

	// 4. Receipt
	for _, pattern := range ReceiptPatterns {
		if strings.Contains(subjectLower, pattern) {
			return "Receipt"
		}
	}

	// 5. Confirmation
	for _, pattern := range ConfirmationPatterns {
		if strings.Contains(subjectLower, pattern) {
			return "Confirmation"
		}
	}

	// 6. Newsletter (check for List-Unsubscribe header)
	if email.HasListUnsubscribe {
		return "Newsletter"
	}

	// 7. Promotional
	for _, pattern := range PromoPatterns {
		if strings.Contains(subjectLower, pattern) {
			return "Promotional"
		}
	}

	// 8. Automated (noreply patterns)
	if strings.Contains(fromLower, "noreply") ||
		strings.Contains(fromLower, "no-reply") ||
		strings.Contains(fromLower, "donotreply") ||
		strings.Contains(fromLower, "do-not-reply") ||
		strings.Contains(fromLower, "notifications@") ||
		strings.Contains(fromLower, "mailer-daemon") {
		return "Automated"
	}

	// 9. Unknown (likely a real person)
	return "Unknown"
}

type EmailHeaders struct {
	Subject            string
	From               string
	Date               string
	HasListUnsubscribe bool
}

func (e *InboxZeroTool) imapFetchHeadersExtended(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, msgNum int) (*EmailHeaders, error) {
	cmd := fmt.Sprintf("A5 FETCH %d (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM DATE LIST-UNSUBSCRIBE)])\r\n", msgNum)
	if _, err := writer.WriteString(cmd); err != nil {
		return nil, fmt.Errorf("failed to write imap command: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush imap writer: %w", err)
	}

	headersResult := &EmailHeaders{
		Subject: "(no subject)",
		From:    "(unknown)",
		Date:    "",
	}

	var lines []string
	lineCount := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading imap response: %w", err)
		}

		lineCount++
		if lineCount > e.maxImapResponseLines {
			return nil, fmt.Errorf("IMAP FETCH response exceeded maximum lines (%d)", e.maxImapResponseLines)
		}

		lines = append(lines, line)

		if strings.HasPrefix(line, "A5 ") {
			break
		}
	}

	for _, headerLine := range lines {
		trimmed := strings.TrimSpace(headerLine)

		switch {
		case strings.HasPrefix(strings.ToLower(trimmed), "subject:"):
			headersResult.Subject = strings.TrimSpace(trimmed[len("subject:"):])
		case strings.HasPrefix(strings.ToLower(trimmed), "from:"):
			headersResult.From = strings.TrimSpace(trimmed[len("from:"):])
		case strings.HasPrefix(strings.ToLower(trimmed), "date:"):
			headersResult.Date = strings.TrimSpace(trimmed[len("date:"):])
		case strings.HasPrefix(strings.ToLower(trimmed), "list-unsubscribe:"):
			headersResult.HasListUnsubscribe = true
		}
	}

	return headersResult, nil
}

func (e *InboxZeroTool) executeImap(
	ctx context.Context,
	action func(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer) (string, error),
) string {
	if strings.TrimSpace(e.emailConfig.ImapHost) == "" {
		return "Error: IMAP host not configured. Set Plugins.Native.Email.ImapHost."
	}

	password := core.SecretResolverInstance.Resolve(e.emailConfig.PasswordRef)
	if strings.TrimSpace(e.emailConfig.Username) == "" || strings.TrimSpace(password) == "" {
		return "Error: Email credentials not configured. Set Email.Username and Email.PasswordRef."
	}

	var cancel context.CancelFunc = func() {}
	effectiveCtx := ctx
	if e.config.ImapOperationTimeoutSeconds > 0 {
		effectiveCtx, cancel = context.WithTimeout(ctx, time.Duration(e.config.ImapOperationTimeoutSeconds)*time.Second)
	}
	defer cancel()

	addr := fmt.Sprintf("%s:%d", e.emailConfig.ImapHost, e.emailConfig.ImapPort)
	dialer := net.Dialer{}

	rawConn, err := dialer.DialContext(effectiveCtx, "tcp", addr)
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}
	defer rawConn.Close()

	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName: e.emailConfig.ImapHost,
	})

	if err := tlsConn.HandshakeContext(effectiveCtx); err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}

	reader := bufio.NewReader(tlsConn)
	writer := bufio.NewWriter(tlsConn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}

	// Login
	loginCmd := fmt.Sprintf("A1 LOGIN %s %s\r\n", imapQuote(e.emailConfig.Username), imapQuote(password))
	if _, err := writer.WriteString(loginCmd); err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}

	loginResp, err := e.readUntilTag(effectiveCtx, reader, "A1")
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}

	if !strings.Contains(strings.ToUpper(loginResp), "OK") {
		return fmt.Sprintf("Error: IMAP login failed — %s", loginResp)
	}

	result, err := action(effectiveCtx, reader, writer)
	if err != nil {
		return fmt.Sprintf("Error: IMAP operation failed — %v", err)
	}

	// Logout
	if _, err := writer.WriteString("A99 LOGOUT\r\n"); err == nil {
		_ = writer.Flush()
		_, _ = e.readUntilTag(effectiveCtx, reader, "A99")
	}

	return result
}

func (e *InboxZeroTool) readUntilTag(ctx context.Context, reader *bufio.Reader, tag string) (string, error) {
	var sb = strings.Builder{}

	scanner := bufio.NewScanner(reader)
	var lines = 0
	for scanner.Scan() {
		lines++
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if lines > e.maxImapResponseLines {
			return "", fmt.Errorf("IMAP response exceeded maximum lines (%d) for tag %s.", e.maxImapResponseLines, tag)
		}
		line := scanner.Text()
		sb.WriteString(line)
		sb.WriteString("\n")
		if strings.HasPrefix(line, tag+" ") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return sb.String(), nil
}

func imapQuote(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
	)

	return fmt.Sprintf("\"%s\"", replacer.Replace(value))
}

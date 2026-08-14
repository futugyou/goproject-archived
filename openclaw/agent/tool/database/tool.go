package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/core"

	"context"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	_ "gorm.io/driver/postgres"
)

type DatabaseTool struct {
	config        core.DatabaseConfig
	toolingConfig *core.ToolingConfig
	logger        *slog.Logger
}

func New(config core.DatabaseConfig, toolingConfig *core.ToolingConfig, logger *slog.Logger) *DatabaseTool {
	if logger == nil {
		logger = slog.Default()
	}

	if config.MaxRows <= 0 {
		config.MaxRows = 100
	}

	if config.AllowWrite {
		logger.Warn("DatabaseTool: AllowWrite is enabled. The LLM can execute arbitrary write operations. Connect with a read-only database user for safety.")
	}

	return &DatabaseTool{config: config, toolingConfig: toolingConfig, logger: logger}
}

func (a *DatabaseTool) Name() string {
	return "database"
}

func (a *DatabaseTool) Description() string {
	return "Execute SQL queries against a database. Supports SQLite, PostgreSQL, and MySQL. Use for data retrieval, schema inspection, and (if enabled) data modification."
}

func (a *DatabaseTool) ParameterSchema() string {
	return `
	 {
          "type": "object",
          "properties": {
            "action": {
              "type": "string",
              "description": "Action to perform",
              "enum": ["query", "execute", "schema", "tables"]
            },
            "sql": {
              "type": "string",
              "description": "SQL query or statement to execute"
            },
            "table": {
              "type": "string",
              "description": "Table name (for schema action)"
            }
          },
          "required": ["action"]
        }
    `
}

var WriteKeywords map[string]struct{} = map[string]struct{}{
	"INSERT": {}, "UPDATE": {}, "DELETE": {}, "DROP": {}, "CREATE": {}, "ALTER": {}, "TRUNCATE": {}, "MERGE": {}, "REPLACE": {},
	"UPSERT": {}, "VACUUM": {}, "REINDEX": {}, "ATTACH": {}, "DETACH": {}, "GRANT": {}, "REVOKE": {}, "SET": {}, "CALL": {}, "EXEC": {}, "EXECUTE": {},
}

type ToolArguments struct {
	Action string `json:"action"`
	SQL    string `json:"sql"`
	Table  string `json:"table"`
}

func (a *DatabaseTool) Execute(ctx context.Context, argumentsJson string) string {
	var args ToolArguments
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("Error: Invalid JSON arguments — %v", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))

	if a.toolingConfig != nil && a.toolingConfig.ReadOnlyMode && action == "execute" {
		return "Error: database execute action is disabled because Tooling.ReadOnlyMode is enabled."
	}

	connString := a.resolveConnectionString()
	if strings.TrimSpace(connString) == "" {
		return "Error: Database connection string not configured. Set Database.ConnectionString."
	}

	var res string
	var err error

	switch action {
	case "query":
		res, err = a.runQuery(ctx, args, connString)
	case "execute":
		res, err = a.runExecute(ctx, args, connString)
	case "tables":
		res, err = a.listTables(ctx, connString)
	case "schema":
		res, err = a.getSchema(ctx, args, connString)
	default:
		return fmt.Sprintf("Error: Unsupported database action '%s'. Use: query, execute, tables, schema.", action)
	}

	if err != nil {
		return fmt.Sprintf("Error: Database operation failed — %v", err)
	}

	return res
}

func (a *DatabaseTool) runQuery(ctx context.Context, args ToolArguments, connString string) (string, error) {
	if strings.TrimSpace(args.SQL) == "" {
		return "Error: 'sql' is required for query action.", nil
	}

	if policyErr := a.validateSQLPolicy(args.SQL); policyErr != "" {
		return policyErr, nil
	}

	if a.isWriteOperation(args.SQL) {
		return "Error: Write operations must use the 'execute' action, not 'query'.", nil
	}

	db, err := a.openDB(ctx, connString)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctxWithTimeout, cancel := a.contextWithTimeout(ctx)
	defer cancel()

	rows, err := db.QueryContext(ctxWithTimeout, args.SQL)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	return a.formatResultSet(ctxWithTimeout, rows, "")
}

func (a *DatabaseTool) runExecute(ctx context.Context, args ToolArguments, connString string) (string, error) {
	if strings.TrimSpace(args.SQL) == "" {
		return "Error: 'sql' is required for execute action.", nil
	}

	if policyErr := a.validateSQLPolicy(args.SQL); policyErr != "" {
		return policyErr, nil
	}

	if !a.config.AllowWrite && a.isWriteOperation(args.SQL) {
		return "Error: Write operations are disabled. Set Database.AllowWrite = true to enable.", nil
	}

	db, err := a.openDB(ctx, connString)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctxWithTimeout, cancel := a.contextWithTimeout(ctx)
	defer cancel()

	result, err := db.ExecContext(ctxWithTimeout, args.SQL)
	if err != nil {
		return "", err
	}

	rowsAffected, _ := result.RowsAffected()
	return fmt.Sprintf("Statement executed successfully. Rows affected: %d", rowsAffected), nil
}

func (a *DatabaseTool) listTables(ctx context.Context, connString string) (string, error) {
	var sqlQuery string
	switch strings.ToLower(a.config.Provider) {
	case "sqlite":
		sqlQuery = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
	case "postgres":
		sqlQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	case "mysql":
		sqlQuery = "SHOW TABLES"
	default:
		sqlQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	}

	db, err := a.openDB(ctx, connString)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctxWithTimeout, cancel := a.contextWithTimeout(ctx)
	defer cancel()

	rows, err := db.QueryContext(ctxWithTimeout, sqlQuery)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("Tables:\n")
	count := 0

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return "", err
		}

		if !a.isTableAllowed(table) {
			continue
		}

		count++
		sb.WriteString(fmt.Sprintf("  %s\n", table))
	}

	if count == 0 {
		sb.WriteString("  (no tables found)\n")
	}

	return sb.String(), nil
}

func (a *DatabaseTool) getSchema(ctx context.Context, args ToolArguments, connString string) (string, error) {
	table := strings.TrimSpace(args.Table)
	if table == "" {
		return "Error: 'table' is required for schema action.", nil
	}

	if !a.isTableAllowed(table) {
		return fmt.Sprintf("Error: Access denied for table '%s'.", table), nil
	}

	provider := strings.ToLower(a.config.Provider)

	if provider == "sqlite" || provider == "mysql" {
		if !isValidIdentifier(table) {
			return "Error: Invalid table name. Only alphanumeric characters, underscores, and dots are allowed.", nil
		}
	}

	db, err := a.openDB(ctx, connString)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctxWithTimeout, cancel := a.contextWithTimeout(ctx)
	defer cancel()

	var rows *sql.Rows

	switch provider {
	case "sqlite":
		escapedTable := strings.ReplaceAll(table, "'", "''")
		rows, err = db.QueryContext(ctxWithTimeout, fmt.Sprintf("PRAGMA table_info('%s')", escapedTable))
	case "mysql":
		escapedTable := strings.ReplaceAll(table, "`", "``")
		rows, err = db.QueryContext(ctxWithTimeout, fmt.Sprintf("DESCRIBE `%s`", escapedTable))
	default: // postgres, etc.
		query := "SELECT column_name, data_type, is_nullable, column_default " +
			"FROM information_schema.columns " +
			"WHERE table_name = $1 " +
			"ORDER BY ordinal_position"
		rows, err = db.QueryContext(ctxWithTimeout, query, table)
	}

	if err != nil {
		return "", err
	}
	defer rows.Close()

	return a.formatResultSet(ctxWithTimeout, rows, fmt.Sprintf("Schema for table: %s", table))
}

func (a *DatabaseTool) formatResultSet(ctx context.Context, rows *sql.Rows, header string) (string, error) {
	var sb strings.Builder
	if header != "" {
		sb.WriteString(header)
		sb.WriteString("\n")
	}

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	fieldCount := len(cols)
	if fieldCount == 0 {
		if header != "" {
			return header, nil
		}
		return "(no columns)", nil
	}

	widths := make([]int, fieldCount)
	for i, col := range cols {
		widths[i] = len(col)
	}

	var rowData [][]string

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		if len(rowData) >= a.config.MaxRows {
			break
		}

		// 创建通用的可变扫描容器
		values := make([]any, fieldCount)
		valuePtrs := make([]any, fieldCount)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}

		row := make([]string, fieldCount)
		for i, val := range values {
			if val == nil {
				row[i] = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					row[i] = string(v)
				default:
					row[i] = fmt.Sprintf("%v", v)
				}
			}

			// 计算显示宽度（截断考虑）
			cellLen := len(row[i])
			if cellLen > 50 {
				cellLen = 50
			}
			if cellLen > widths[i] {
				widths[i] = cellLen
			}
		}

		rowData = append(rowData, row)
	}

	// 1. 打印表头
	var divider strings.Builder
	for i, col := range cols {
		if i > 0 {
			sb.WriteString(" | ")
			divider.WriteString("-+-")
		}
		fmt.Fprintf(&sb, "%-*s", widths[i], col)
		divider.WriteString(strings.Repeat("-", widths[i]))
	}
	sb.WriteString("\n")
	sb.WriteString(divider.String())
	sb.WriteString("\n")

	// 2. 打印数据行
	for _, row := range rowData {
		for i, val := range row {
			if i > 0 {
				sb.WriteString(" | ")
			}
			if len(val) > 50 {
				val = val[:47] + "..."
			}
			fmt.Fprintf(&sb, "%-*s", widths[i], val)
		}
		sb.WriteString("\n")
	}

	rowWord := "rows"
	if len(rowData) == 1 {
		rowWord = "row"
	}
	fmt.Fprintf(&sb, "\n(%d %s)\n", len(rowData), rowWord)

	if len(rowData) == a.config.MaxRows {
		fmt.Fprintf(&sb, "(results limited to %d rows)\n", a.config.MaxRows)
	}

	return sb.String(), nil
}

func (a *DatabaseTool) openDB(ctx context.Context, connString string) (*sql.DB, error) {
	driver := strings.ToLower(a.config.Provider)
	switch driver {
	case "sqlite":
		driver = "sqlite"
	case "postgres":
		driver = "postgres"
	case "mysql":
		driver = "mysql"
	}

	db, err := sql.Open(driver, connString)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := a.contextWithTimeout(ctx)
	defer cancel()

	if err := db.PingContext(ctxWithTimeout); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (a *DatabaseTool) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.config.TimeoutSeconds <= 0 {
		return context.WithTimeout(ctx, 30*time.Second)
	}
	return context.WithTimeout(ctx, time.Duration(a.config.TimeoutSeconds)*time.Second)
}

func (a *DatabaseTool) isWriteOperation(sqlQuery string) bool {
	for token := range a.enumerateSQLTokens(sqlQuery) {
		if _, exists := WriteKeywords[token]; exists {
			return true
		}
	}
	return false
}

func (a *DatabaseTool) validateSQLPolicy(sqlQuery string) string {
	if !a.config.AllowMultiStatement && a.hasMultipleStatements(sqlQuery) {
		return "Error: Multiple SQL statements are disabled. Set Database.AllowMultiStatement = true to enable."
	}

	if len(a.config.AllowedTables) == 0 && len(a.config.DeniedTables) == 0 {
		return ""
	}

	tableRefs := a.extractReferencedTables(sqlQuery)
	for table := range tableRefs {
		if !a.isTableAllowed(table) {
			return fmt.Sprintf("Error: Access denied for table '%s'.", table)
		}
	}

	return ""
}

func (a *DatabaseTool) isTableAllowed(tableName string) bool {
	normalized := normalizeIdentifier(tableName)

	for _, denied := range a.config.DeniedTables {
		if identifiersEqual(normalized, denied) {
			return false
		}
	}

	if len(a.config.AllowedTables) == 0 {
		return true
	}

	for _, allowed := range a.config.AllowedTables {
		if identifiersEqual(normalized, allowed) {
			return true
		}
	}

	return false
}

func identifiersEqual(left, right string) bool {
	return strings.EqualFold(left, normalizeIdentifier(right))
}

func normalizeIdentifier(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return trimmed
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '.'
	})

	for i, p := range parts {
		p = strings.TrimSpace(p)
		if (strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"")) ||
			(strings.HasPrefix(p, "`") && strings.HasSuffix(p, "`")) ||
			(strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]")) {
			if len(p) >= 2 {
				p = p[1 : len(p)-1]
			}
		}
		parts[i] = p
	}

	return strings.Join(parts, ".")
}

func (a *DatabaseTool) hasMultipleStatements(sqlQuery string) bool {
	var inSingleQuote, inDoubleQuote, inBacktickQuote, inBracketQuote bool
	var inLineComment, inBlockComment bool

	runes := []rune(sqlQuery)
	n := len(runes)

	for i := 0; i < n; i++ {
		c := runes[i]
		var next rune
		if i+1 < n {
			next = runes[i+1]
		}

		if inLineComment {
			if c == '\n' || c == '\r' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			if c == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inSingleQuote {
			if c == '\'' && next == '\'' {
				i++
				continue
			}
			if c == '\'' {
				inSingleQuote = false
			}
			continue
		}

		if inDoubleQuote {
			if c == '"' && next == '"' {
				i++
				continue
			}
			if c == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if inBacktickQuote {
			if c == '`' {
				inBacktickQuote = false
			}
			continue
		}

		if inBracketQuote {
			if c == ']' {
				inBracketQuote = false
			}
			continue
		}

		if c == '-' && next == '-' {
			inLineComment = true
			i++
			continue
		}

		if c == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}

		if c == '\'' {
			inSingleQuote = true
			continue
		}
		if c == '"' {
			inDoubleQuote = true
			continue
		}
		if c == '`' {
			inBacktickQuote = true
			continue
		}
		if c == '[' {
			inBracketQuote = true
			continue
		}

		if c == ';' {
			for j := i + 1; j < n; j++ {
				if !unicode.IsSpace(runes[j]) {
					return true
				}
			}
		}
	}

	return false
}

func (a *DatabaseTool) extractReferencedTables(sqlQuery string) map[string]struct{} {
	refs := make(map[string]struct{})
	cleaned := stripSQLLiteralsAndComments(sqlQuery)

	parts := strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '(' || r == ')' || r == ',' || r == ';'
	})

	for i := 0; i < len(parts); i++ {
		if !isTableKeyword(parts[i]) {
			continue
		}

		candidate := readNextTableIdentifier(parts, i+1)
		if candidate == "" {
			continue
		}

		if strings.EqualFold(candidate, "SELECT") {
			continue
		}

		candidate = normalizeIdentifier(candidate)
		if candidate == "" {
			continue
		}

		refs[strings.ToLower(candidate)] = struct{}{}
	}

	return refs
}

func readNextTableIdentifier(parts []string, startIndex int) string {
	for i := startIndex; i < len(parts); i++ {
		token := strings.TrimSpace(parts[i])
		if token == "" {
			continue
		}

		upper := strings.ToUpper(token)
		if upper == "ONLY" || upper == "LATERAL" || upper == "OUTER" ||
			upper == "INNER" || upper == "LEFT" || upper == "RIGHT" ||
			upper == "FULL" || upper == "CROSS" {
			continue
		}

		var raw strings.Builder
		raw.WriteString(token)

		if strings.HasPrefix(token, "[") && !strings.HasSuffix(token, "]") {
			for j := i + 1; j < len(parts); j++ {
				raw.WriteString(" ")
				raw.WriteString(parts[j])
				if strings.HasSuffix(parts[j], "]") {
					break
				}
			}
		}

		return raw.String()
	}

	return ""
}

func isTableKeyword(token string) bool {
	upper := strings.ToUpper(token)
	return upper == "FROM" || upper == "JOIN" || upper == "UPDATE" ||
		upper == "INTO" || upper == "TABLE" || upper == "DESCRIBE"
}

func stripSQLLiteralsAndComments(sqlQuery string) string {
	var sb strings.Builder
	var inSingleQuote, inDoubleQuote, inBacktickQuote, inBracketQuote bool
	var inLineComment, inBlockComment bool

	runes := []rune(sqlQuery)
	n := len(runes)

	for i := 0; i < n; i++ {
		c := runes[i]
		var next rune
		if i+1 < n {
			next = runes[i+1]
		}

		if inLineComment {
			if c == '\n' || c == '\r' {
				inLineComment = false
				sb.WriteRune(' ')
			}
			continue
		}

		if inBlockComment {
			if c == '*' && next == '/' {
				inBlockComment = false
				i++
				sb.WriteRune(' ')
			}
			continue
		}

		if inSingleQuote {
			if c == '\'' && next == '\'' {
				i++
				continue
			}
			if c == '\'' {
				inSingleQuote = false
			}
			continue
		}

		if inDoubleQuote {
			if c == '"' && next == '"' {
				sb.WriteRune('"')
				i++
				continue
			}
			if c == '"' {
				inDoubleQuote = false
				sb.WriteRune(c)
				continue
			}
			sb.WriteRune(c)
			continue
		}

		if inBacktickQuote {
			if c == '`' {
				inBacktickQuote = false
			}
			sb.WriteRune(c)
			continue
		}

		if inBracketQuote {
			if c == ']' {
				inBracketQuote = false
			}
			sb.WriteRune(c)
			continue
		}

		if c == '-' && next == '-' {
			inLineComment = true
			i++
			continue
		}

		if c == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}

		if c == '\'' {
			inSingleQuote = true
			sb.WriteRune(' ')
			continue
		}

		if c == '"' {
			inDoubleQuote = true
			sb.WriteRune(c)
			continue
		}

		if c == '`' {
			inBacktickQuote = true
			sb.WriteRune(c)
			continue
		}

		if c == '[' {
			inBracketQuote = true
			sb.WriteRune(c)
			continue
		}

		sb.WriteRune(c)
	}

	return sb.String()
}

func (a *DatabaseTool) enumerateSQLTokens(sqlQuery string) <-chan string {
	ch := make(chan string)

	go func() {
		defer close(ch)
		if strings.TrimSpace(sqlQuery) == "" {
			return
		}

		var token strings.Builder
		var inSingleQuote, inDoubleQuote, inBacktickQuote, inBracketQuote bool
		var inLineComment, inBlockComment bool

		flushToken := func() {
			if token.Len() > 0 {
				ch <- token.String()
				token.Reset()
			}
		}

		runes := []rune(sqlQuery)
		n := len(runes)

		for i := 0; i < n; i++ {
			c := runes[i]
			var next rune
			if i+1 < n {
				next = runes[i+1]
			}

			if inLineComment {
				if c == '\n' || c == '\r' {
					inLineComment = false
				}
				continue
			}

			if inBlockComment {
				if c == '*' && next == '/' {
					inBlockComment = false
					i++
				}
				continue
			}

			if inSingleQuote {
				if c == '\'' && next == '\'' {
					i++
					continue
				}
				if c == '\'' {
					inSingleQuote = false
				}
				continue
			}

			if inDoubleQuote {
				if c == '"' && next == '"' {
					i++
					continue
				}
				if c == '"' {
					inDoubleQuote = false
				}
				continue
			}

			if inBacktickQuote {
				if c == '`' {
					inBacktickQuote = false
				}
				continue
			}

			if inBracketQuote {
				if c == ']' {
					inBracketQuote = false
				}
				continue
			}

			if c == '-' && next == '-' {
				flushToken()
				inLineComment = true
				i++
				continue
			}

			if c == '/' && next == '*' {
				flushToken()
				inBlockComment = true
				i++
				continue
			}

			if c == '\'' {
				flushToken()
				inSingleQuote = true
				continue
			}

			if c == '"' {
				flushToken()
				inDoubleQuote = true
				continue
			}

			if c == '`' {
				flushToken()
				inBacktickQuote = true
				continue
			}

			if c == '[' {
				flushToken()
				inBracketQuote = true
				continue
			}

			if unicode.IsLetter(c) || (token.Len() > 0 && unicode.IsDigit(c)) {
				token.WriteRune(unicode.ToUpper(c))
				continue
			}

			flushToken()
		}

		flushToken()
	}()

	return ch
}

func (a *DatabaseTool) resolveConnectionString() string {
	return core.SecretResolverInstance.Resolve(a.config.ConnectionString)
}

func isValidIdentifier(name string) bool {
	if strings.TrimSpace(name) == "" || len(name) > 128 {
		return false
	}

	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

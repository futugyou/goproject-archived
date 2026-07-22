package core

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/copier"
)

type ChannelAllowlistFile struct {
	AllowedFrom  []string  `json:"allowed_from"`
	AllowedTo    []string  `json:"allowed_to"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

type AllowlistManager struct {
	rootDir string
	logger  *slog.Logger
	locks   sync.Map
}

func NewAllowlistManager(baseStoragePath string, logger *slog.Logger) (*AllowlistManager, error) {
	rootDir := filepath.Join(baseStoragePath, "allowlists")
	if logger == nil {
		logger = slog.Default()
	}

	// 创建初始目录
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	return &AllowlistManager{
		rootDir: rootDir,
		logger:  logger,
	}, nil
}

// TryGetDynamic 读取并解析动态白名单文件
func (m *AllowlistManager) TryGetDynamic(channelID string) *ChannelAllowlistFile {
	path := m.getPath(channelID)

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		m.logger.Warn("Failed to read allowlist file", "channelId", channelID, "error", err)
		return nil
	}

	// 反序列化
	var file ChannelAllowlistFile
	if err := json.Unmarshal(data, &file); err != nil {
		m.logger.Warn("Failed to deserialize allowlist file", "channelId", channelID, "error", err)
		return nil
	}

	if file.AllowedFrom == nil {
		file.AllowedFrom = []string{}
	}
	if file.AllowedTo == nil {
		file.AllowedTo = []string{}
	}

	return &file
}

// GetEffective 获取有效配置，如果动态的不存在则返回默认配置
func (m *AllowlistManager) GetEffective(channelID string, configAllowlist ChannelAllowlistFile) ChannelAllowlistFile {
	if dynamic := m.TryGetDynamic(channelID); dynamic != nil {
		return *dynamic
	}
	return configAllowlist
}

// UpsertDynamic 执行原子的“读取-修改-写入”操作
func (m *AllowlistManager) UpsertDynamic(channelID string, updateFn func(*ChannelAllowlistFile) ChannelAllowlistFile) {
	// 获取或创建针对该 channelID 的锁
	actual, _ := m.locks.LoadOrStore(channelID, &sync.Mutex{})
	mutex := actual.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	current := m.TryGetDynamic(channelID)

	// 执行用户传入的更新逻辑，并强制刷新时间戳
	next := updateFn(current)
	next.UpdatedAtUtc = time.Now().UTC()

	path := m.getPath(channelID)
	tmpFile, err := os.CreateTemp(m.rootDir, "allowlist-*.tmp")
	if err != nil {
		m.logger.Warn("Failed to create temp file", "channelId", channelID, "error", err)
		return
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// 尽最大努力清理临时文件
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// 序列化并写入临时文件
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ") // 可选：美化 JSON 输出
	if err := encoder.Encode(next); err != nil {
		_ = tmpFile.Close()
		m.logger.Warn("Failed to serialize allowlist file", "channelId", channelID, "error", err)
		return
	}
	_ = tmpFile.Close()

	// 原子替换
	if err := os.Rename(tmpPath, path); err != nil {
		m.logger.Warn("Failed to persist allowlist file", "channelId", channelID, "error", err)
	}
}

// AddAllowedFrom 往 AllowedFrom 追加单个 senderId
func (m *AllowlistManager) AddAllowedFrom(channelID string, senderID string) {
	if strings.TrimSpace(senderID) == "" {
		return
	}

	m.UpsertDynamic(channelID, func(cur *ChannelAllowlistFile) ChannelAllowlistFile {
		if cur == nil {
			cur = &ChannelAllowlistFile{AllowedFrom: []string{}, AllowedTo: []string{}}
		}

		// 检查是否存在
		if slices.Contains(cur.AllowedFrom, senderID) {
			return *cur
		}

		// 追加元素
		nextAllowedFrom := append(cur.AllowedFrom, senderID)
		return ChannelAllowlistFile{
			AllowedFrom: nextAllowedFrom,
			AllowedTo:   cur.AllowedTo,
		}
	})
}

// SetAllowedFrom 覆盖设置 AllowedFrom 并进行去重和过滤
func (m *AllowlistManager) SetAllowedFrom(channelID string, senderIDs []string) {
	// 过滤、去重与清洗
	seen := make(map[string]bool)
	var list []string

	for _, id := range senderIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			list = append(list, trimmed)
		}
	}
	if list == nil {
		list = []string{}
	}

	m.UpsertDynamic(channelID, func(cur *ChannelAllowlistFile) ChannelAllowlistFile {
		if cur == nil {
			cur = &ChannelAllowlistFile{AllowedFrom: []string{}, AllowedTo: []string{}}
		}
		return ChannelAllowlistFile{
			AllowedFrom: list,
			AllowedTo:   cur.AllowedTo,
		}
	})
}

// getPath 过滤文件名，确保路径安全
func (m *AllowlistManager) getPath(channelID string) string {
	var sb strings.Builder
	for _, r := range channelID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			sb.WriteRune(r)
		}
	}

	safe := sb.String()
	// 去除开头的 '.' 防止隐藏文件攻击
	safe = strings.TrimLeft(safe, ".")

	if strings.TrimSpace(safe) == "" {
		safe = "unknown"
	}

	return filepath.Join(m.rootDir, safe+".json")
}

type AllowlistSemantics uint8

const (
	AllowlistSemantics_Legacy AllowlistSemantics = iota
	AllowlistSemantics_Strict
)

type AllowlistPolicy struct{}

func (a *AllowlistPolicy) ParseSemantics(value string) AllowlistSemantics {
	if value == "strict" {
		return AllowlistSemantics_Strict
	}

	return AllowlistSemantics_Legacy
}

func (a *AllowlistPolicy) IsAllowed(allowlist []string, value string, semantics AllowlistSemantics) bool {
	if len(allowlist) == 0 {
		return semantics == AllowlistSemantics_Legacy
	}

	for _, entry := range allowlist {
		if IsBlank(entry) {
			continue
		}

		var pat = strings.TrimSpace(entry)
		if pat == "*" {
			return true
		}

		matcher := GlobMatcher{}
		if matcher.IsMatch(pat, value) {
			return true
		}

	}

	return false
}

type GlobMatcher struct{}

var GlobMatcherInstance *GlobMatcher = &GlobMatcher{}

func (g *GlobMatcher) IsMatch(pattern string, value string) bool {
	if pattern == "*" {
		return true
	}

	if len(pattern) == 0 {
		return len(value) == 0
	}

	// 快速路径：如果不包含 '*'，直接全字匹配
	if !strings.ContainsRune(pattern, '*') {
		return pattern == value
	}

	remaining := pattern
	valueIndex := 0
	isFirst := true

	for len(remaining) > 0 {
		starPos := strings.IndexByte(remaining, '*')
		if starPos < 0 {
			// 没有更多通配符了 —— 剩余的 pattern 必须匹配 value 的后缀
			return strings.HasSuffix(value[valueIndex:], remaining)
		}

		segment := remaining[:starPos]
		remaining = remaining[starPos+1:]

		if len(segment) == 0 {
			isFirst = false
			continue
		}

		if isFirst {
			// 第一段（如果 pattern 不是以 '*' 开头）必须匹配前缀
			if !strings.HasPrefix(value[valueIndex:], segment) {
				return false
			}
			valueIndex += len(segment)
			isFirst = false
		} else {
			// 中间段 —— 在 value 剩余部分中寻找第一次出现的位置
			found := strings.Index(value[valueIndex:], segment)
			if found < 0 {
				return false
			}
			valueIndex += found + len(segment)
		}
	}

	return true
}

func (g *GlobMatcher) IsAllowed(allowGlobs, denyGlobs []string, value string) bool {
	for _, deny := range denyGlobs {
		if !IsBlank(deny) && g.IsMatch(strings.TrimSpace(deny), value) {
			return false
		}
	}

	if len(allowGlobs) == 0 {
		return false
	}

	for _, allow := range allowGlobs {
		if !IsBlank(allow) && g.IsMatch(strings.TrimSpace(allow), value) {
			return true
		}
	}

	return false
}

var BrowserToolCapabilityEvaluatorInstance = &BrowserToolCapabilityEvaluator{}

type BrowserToolCapabilityEvaluator struct{}

func (b *BrowserToolCapabilityEvaluator) isNonLocalBackendAvailable(config *GatewayConfig, backendName string) bool {
	if IsBlank(backendName) || backendName == "local" {
		return false
	}

	if backendName == "opensandbox" {
		return IsOpenSandboxProviderConfigured(config)
	}
	profile, ok := config.Execution.Profiles[backendName]
	if ok {
		return profile.Enabled && profile.Type != BackendLocal
	}

	return false
}

func (b *BrowserToolCapabilityEvaluator) hasLegacySandboxRoute(config *GatewayConfig) bool {
	return IsOpenSandboxProviderConfigured(config) && ResolveMode(config, "browser", ToolSandboxMode_Prefer) != ToolSandboxMode_None
}

func (b *BrowserToolCapabilityEvaluator) hasExecutionBackend(config *GatewayConfig) bool {
	if !config.Execution.Enabled {
		return false
	}
	route, ok := config.Execution.Tools["browser"]
	if !ok {
		return false
	}

	return b.isNonLocalBackendAvailable(config, route.Backend) || b.isNonLocalBackendAvailable(config, route.FallbackBackend)
}

func (b *BrowserToolCapabilityEvaluator) Evaluate(config *GatewayConfig) *BrowserToolCapabilitySummary {
	var configuredEnabled = config.Tooling.EnableBrowserTool
	var localExecutionSupported = config.Tooling.EnableLocalTool
	var executionBackendConfigured = b.hasExecutionBackend(config) || b.hasLegacySandboxRoute(config)
	var registered = configuredEnabled && (localExecutionSupported || executionBackendConfigured)

	var reason = "disabled"
	if configuredEnabled {
		if registered {
			if executionBackendConfigured && !localExecutionSupported {
				reason = "backend_only"
			} else {
				reason = "available"
			}
		} else {
			reason = "local_execution_unavailable_without_backend"
		}
	}

	return &BrowserToolCapabilitySummary{
		ConfiguredEnabled:          configuredEnabled,
		LocalExecutionSupported:    localExecutionSupported,
		ExecutionBackendConfigured: executionBackendConfigured,
		Registered:                 registered,
		Reason:                     reason,
	}
}

type SecretResolver struct{}

var SecretResolverInstance = &SecretResolver{}

func (s *SecretResolver) IsRawRef(secretRef string) bool {
	return !IsBlank(secretRef) && strings.HasPrefix(secretRef, "raw:")
}

func (s *SecretResolver) LooksLikeEnvVarName(value string) bool {
	if len(value) < 3 {
		return false
	}

	for _, c := range value {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (s *SecretResolver) Resolve(secretRef string) string {
	if IsBlank(secretRef) {
		return ""
	}

	if strings.HasPrefix(secretRef, "env:") {
		return os.Getenv(secretRef[4:])
	}

	if strings.HasPrefix(secretRef, "raw:") {
		return secretRef[4:]
	}

	var envValue = os.Getenv(secretRef)
	if !IsBlank(envValue) {
		return envValue
	}

	return secretRef
}

const (
	shellMetaChars = ";|&`$(){}<>\n\r"
	crlfChars      = "\r\n"
)

type InputSanitizer struct{}

var Sanitizer = InputSanitizer{}

func (InputSanitizer) CheckShellMetaChars(value string, parameterName string) error {
	idx := strings.IndexAny(value, shellMetaChars)
	if idx >= 0 {
		return fmt.Errorf("error: '%s' contains disallowed character '%c'. "+
			"Shell metacharacters (;|&`$(){}\\n\\r<>) are not permitted for security reasons",
			parameterName, value[idx])
	}
	return nil
}

// StripCrlf 从输入中移除 CRLF (\r 和 \n)，防止命令注入。
func (InputSanitizer) StripCrlf(value string) string {
	if !strings.ContainsAny(value, crlfChars) {
		return value
	}

	// 这种方式比循环拼接字符串高效得多，因为它会计算好内存一次性分配
	r := strings.NewReplacer("\r", "", "\n", "")
	return r.Replace(value)
}

// CheckMemoryKey 验证内存便签键是否包含路径遍历序列或空字节。
func (InputSanitizer) CheckMemoryKey(key string) error {
	if strings.Contains(key, "..") ||
		strings.Contains(key, "/") ||
		strings.Contains(key, "\\") ||
		strings.Contains(key, "\x00") {
		return fmt.Errorf("error: Key contains disallowed characters (path separators, '..' or null bytes)")
	}
	return nil
}

// CheckImapFolderName 验证 IMAP 文件夹名称是否仅包含安全字符（无控制字符）。
func (InputSanitizer) CheckImapFolderName(folder string) error {
	for _, c := range folder {
		// 在 Go 中，控制字符的定义通常是小于 0x20 (空格) 或者是 0x7F (DEL)
		// 这对应了 ASCII 中的控制字符。
		if c < ' ' || c == 0x7F {
			return fmt.Errorf("error: Folder name contains control character (0x%02X). "+
				"Only printable characters are allowed in folder names", c)
		}
	}
	return nil
}

type PendingPairing struct {
	Code           string
	ExpiresAt      time.Time
	FailedAttempts int
	LastFailedAt   *time.Time
}

type PairingManager struct {
	codeTtl               time.Duration
	failedAttemptCooldown time.Duration
	maxFailedAttempts     int
	storageDir            string
	approvedListPath      string
	pendingCodes          sync.Map
	approvedSenders       sync.Map
}

func NewPairingManager(baseStoragePath string) *PairingManager {
	pm := &PairingManager{}
	pm.storageDir = filepath.Join(baseStoragePath, "pairing")
	pm.approvedListPath = filepath.Join(pm.storageDir, "approved.json")
	pm.maxFailedAttempts = 5
	pm.codeTtl = time.Millisecond * 10
	pm.failedAttemptCooldown = time.Millisecond * 5
	pm.loadApprovedSenders()

	return pm
}

func (p *PairingManager) loadApprovedSenders() {
	if !FileExists(p.approvedListPath) {
		return
	}

	data, err := os.ReadFile(p.approvedListPath)
	if err != nil {
		return
	}

	var saved []string
	err = json.Unmarshal(data, &saved)
	if err != nil {
		return
	}

	for _, s := range saved {
		p.approvedSenders.Store(s, 1)
	}
}

func (p *PairingManager) persistApprovedSenders() {
	var tmp = p.approvedListPath + ".tmp"
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return
	}

	var saved []string
	p.approvedSenders.Range(func(key, value any) bool {
		keystring := key.(string)
		saved = append(saved, keystring)
		return true
	})

	defer func() {
		if recover() != nil {
			if FileExists(tmp) {
				os.Remove(tmp)
			}
		}
	}()
	data, err := json.Marshal(saved)
	if err != nil {
		panic(err)
	}

	if err = os.WriteFile(tmp, data, 0644); err != nil {
		panic(err)
	}

	if err := os.Rename(tmp, p.approvedListPath); err != nil {
		panic(err)
	}
}

func (p *PairingManager) cleanupExpiredPendingCodes(now time.Time) {
	p.pendingCodes.Range(func(key, value any) bool {
		t := value.(*PendingPairing)
		if t.ExpiresAt.Before(now) {
			p.pendingCodes.Delete(key)
		}

		return true
	})
}

func fixedTimeCodeEquals(expected, provided string) bool {
	if IsBlank(provided) {
		return false
	}

	expectedBytes := []byte(expected)
	providedBytes := []byte(strings.TrimSpace(provided))

	return subtle.ConstantTimeCompare(expectedBytes, providedBytes) == 1
}

func (p *PairingManager) GetApprovedList() []string {
	result := []string{}
	p.approvedSenders.Range(func(key, value any) bool {
		keystring := key.(string)
		result = append(result, keystring)
		return true
	})
	return result
}

func (p *PairingManager) Revoke(channelId, senderId string) {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)
	if _, loaded := p.approvedSenders.LoadAndDelete(key); loaded {
		p.persistApprovedSenders()
	}
}

func (p *PairingManager) ApproveAdmin(channelId, senderId string) {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)
	p.approvedSenders.Store(key, 1)
	p.persistApprovedSenders()
}

func (p *PairingManager) TryApprove(channelId, senderId, providedCode string) error {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)

	var now = time.Now().UTC()
	pendingValue, ok := p.pendingCodes.Load(key)
	if !ok {
		return errors.New("No pending pairing request found.")
	}
	pending := pendingValue.(*PendingPairing)
	if pending.ExpiresAt.Before(now) {
		p.pendingCodes.Delete(key)
		return errors.New("Pairing code has expired. Request a new code.")
	}

	if pending.FailedAttempts >= p.maxFailedAttempts && pending.LastFailedAt != nil && now.Sub(*pending.LastFailedAt) < p.failedAttemptCooldown {
		return errors.New("Too many invalid attempts. Please wait and try again.")
	}

	if !fixedTimeCodeEquals(pending.Code, providedCode) {
		pending.FailedAttempts = pending.FailedAttempts + 1
		pending.LastFailedAt = &now

		p.pendingCodes.Store(key, pending)
		return errors.New("TInvalid pairing code.")
	}
	_, loaded := p.pendingCodes.LoadAndDelete(key)
	if loaded {
		p.approvedSenders.Store(key, 1)
		p.persistApprovedSenders()
		return nil
	}

	return errors.New("Pairing code has already been used or expired.")
}

func (p *PairingManager) GeneratePairingCode(channelId, senderId string) string {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)
	var now = time.Now().UTC()

	p.cleanupExpiredPendingCodes(now)
	existingValue, loaded := p.pendingCodes.Load(key)
	if loaded {
		existing := existingValue.(*PendingPairing)
		if existing.ExpiresAt.After(now) {
			return existing.Code
		}
	}

	code := GenerateCode(int64(100000), int64(1000000))
	p.pendingCodes.Store(key, &PendingPairing{
		Code:           code,
		ExpiresAt:      now.Add(p.codeTtl),
		FailedAttempts: 0,
	})

	return code
}

func (p *PairingManager) IsApproved(channelId, senderId string) bool {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)
	_, ok := p.approvedSenders.Load(key)
	return ok
}

var _ IRedactionPipeline = (*RedactionPipeline)(nil)

type RedactionPipeline struct {
	redactors []ISensitiveDataRedactor
}

func NewRedactionPipeline(redactors []ISensitiveDataRedactor) *RedactionPipeline {
	return &RedactionPipeline{
		redactors: redactors,
	}
}

// Redact implements [IRedactionPipeline].
func (r *RedactionPipeline) Redact(value string) string {
	if IsBlank(value) {
		return ""
	}

	var current = value
	for _, redactor := range r.redactors {
		current = redactor.Redact(current)
	}
	return current
}

// RedactBranch implements [IRedactionPipeline].
func (r *RedactionPipeline) RedactBranch(branch *SessionBranch) *SessionBranch {
	var dest SessionBranch
	err := copier.Copy(&dest, branch)
	if err != nil {
		return nil
	}
	var session = &Session{
		Id:        dest.SessionId,
		ChannelId: "",
		SenderId:  "",
		History:   dest.History,
	}
	r.RedactSessionInPlace(session)
	return &dest
}

// RedactSession implements [IRedactionPipeline].
func (r *RedactionPipeline) RedactSession(session *Session) *Session {
	var dest Session
	err := copier.Copy(&dest, session)
	if err != nil {
		return nil
	}
	r.RedactSessionInPlace(&dest)
	return &dest
}

// RedactSessionInPlace implements [IRedactionPipeline].
func (r *RedactionPipeline) RedactSessionInPlace(session *Session) error {
	if session == nil {
		return nil
	}

	for i := 0; i < len(session.History); i++ {
		session.History[i].Content = r.Redact(session.History[i].Content)
		for j := 0; j < len(session.History[i].ToolCalls); j++ {
			toolCall := &session.History[i].ToolCalls[j]
			toolCall.Arguments = r.Redact(toolCall.Arguments)
			if toolCall.Result != "" {
				res := r.Redact(toolCall.Result)
				toolCall.Result = res
			}
			if toolCall.NextStep != "" {
				res := r.Redact(toolCall.NextStep)
				toolCall.NextStep = res
			}
			if toolCall.FailureMessage != "" {
				res := r.Redact(toolCall.FailureMessage)
				toolCall.FailureMessage = res
			}
		}
	}

	return nil
}

type NoopRedactionPipeline struct{}

// Redact implements [IRedactionPipeline].
func (n *NoopRedactionPipeline) Redact(value string) string {
	return ""
}

// RedactBranch implements [IRedactionPipeline].
func (n *NoopRedactionPipeline) RedactBranch(branch *SessionBranch) *SessionBranch {
	return branch
}

// RedactSession implements [IRedactionPipeline].
func (n *NoopRedactionPipeline) RedactSession(session *Session) *Session {
	return session
}

// RedactSessionInPlace implements [IRedactionPipeline].
func (n *NoopRedactionPipeline) RedactSessionInPlace(session *Session) error {
	return nil
}

var _ IRedactionPipeline = (*NoopRedactionPipeline)(nil)

var _ ISensitiveDataRedactor = (*BaselineSecretRedactor)(nil)

type BaselineSecretRedactor struct {
}

var (
	// 1. Bearer 认证解析 (Go 不支持在中间混用 ?im，这里统一用 (?i) 开启不区分大小写)
	// 注意：Go 不支持 \b 单词边界的某些高级特性，但在字母和空格间依然有效。
	// 原正则末尾的 [^\s"'`]+ 在 Go 的反引号字符串中需要稍微处理，这里排除空格、双引号、单引号
	BearerAuthorizationRegex = regexp.MustCompile(`(?i)\b(Authorization\s*:\s*Bearer\s+)[^\s"']+\b`)

	// 2. OpenAI Secret 解析
	OpenAiSecretRegex = regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_\-]{12,}`)

	// 3. API Key 字段解析
	ApiKeyFieldRegex = regexp.MustCompile(`(?i)(\bapi[_-]?key["'\s:=]+)[A-Za-z0-9_\-]{12,}`)
)

// GetName implements [ISensitiveDataRedactor].
func (b *BaselineSecretRedactor) GetName() string {
	return "baseline-secrets"
}

// Redact implements [ISensitiveDataRedactor].
func (b *BaselineSecretRedactor) Redact(value string) string {
	if IsBlank(value) {
		return ""
	}
	result := BearerAuthorizationRegex.ReplaceAllString(value, "${1}[REDACTED:authorization]")

	// 2. 替换 OpenAI Secret (直接整段替换)
	result = OpenAiSecretRegex.ReplaceAllString(result, "[REDACTED:secret]")

	// 3. 替换 API Key 字段，保留 "api-key: " 等前缀
	result = ApiKeyFieldRegex.ReplaceAllString(result, "${1}[REDACTED:secret]")

	return result
}

var _ ISentinelSubstitutionService = (*NoopSentinelSubstitutionService)(nil)

type NoopSentinelSubstitutionService struct{}

// Substitute implements [ISentinelSubstitutionService].
func (n *NoopSentinelSubstitutionService) Substitute(ctx context.Context, sentinelContext *SentinelSubstitutionContext) (*SentinelSubstitutionResult, error) {
	return &SentinelSubstitutionResult{
		ExecutionArgumentsJson: sentinelContext.ArgumentsJson,
		PersistedArgumentsJson: sentinelContext.ArgumentsJson,
	}, nil
}

type UrlSafetyValidator struct{}

func (u *UrlSafetyValidator) addressMatchesCidr(address netip.Addr, cidr string) bool {
	// 1. 解析 CIDR 字符串
	network, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}

	// 2. 将地址统一转换为最简形式（自动处理 IPv4-mapped IPv6 地址）
	// netip.Addr.Contains 会自动处理同一 IP 空间（如将 IPv4-mapped IPv6 地址作为 IPv4 对比）
	// 如果需要严格确保两者底层网络家族一致，可以先调用 .Unmap()
	addrUnmapped := address.Unmap()
	netUnmapped := network.Addr().Unmap()

	if addrUnmapped.BitLen() != netUnmapped.BitLen() {
		return false
	}

	// 3. 检查是否包含
	// 重新用 unmapped 的网络和原有的 Prefix 长度构建
	strictNetwork := netip.PrefixFrom(netUnmapped, network.Bits())
	return strictNetwork.Contains(addrUnmapped)
}

func (u *UrlSafetyValidator) isNonPublicAddress(ip netip.Addr) bool {
	// 1. 如果 IP 无效，或者本身就是未指定地址 (0.0.0.0 或 ::)
	if !ip.IsValid() || ip.IsUnspecified() {
		return true
	}

	// 2. 检查环回地址 (127.0.0.1 或 ::1)
	if ip.IsLoopback() {
		return true
	}

	// 3. 检查链路本地地址 (169.254.a.b 或 fe80::)
	if ip.IsLinkLocalUnicast() {
		return true
	}

	// 4. 检查组播/多播地址 (224.0.0.0/4 或 ff00::/8)
	if ip.IsMulticast() {
		return true
	}

	// 5. 检查 RFC 1918 (IPv4私有网段) 和 RFC 4193 (IPv6 唯一本地地址 / ULA)
	// 这包含了 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 和 fc00::/7
	if ip.IsPrivate() {
		return true
	}

	// 6. 单独列出的特殊 IPv4 网段做兜底处理 (如 CGNAT、基准测试等)
	if ip.Is4() {
		b := ip.As4()
		return b[0] == 0 || // 0.0.0.0/8
			(b[0] == 100 && b[1] >= 64 && b[1] <= 127) || // 100.64.0.0/10 (CGNAT)
			(b[0] == 198 && (b[1] == 18 || b[1] == 19)) // 198.18.0.0/15 (Benchmarking)
	}

	return false
}

func (u *UrlSafetyValidator) normalizeHost(uri url.URL) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(uri.Host), "."))
}

func (u *UrlSafetyValidator) validateAddresses(addresses []netip.Addr, policy *UrlSafetyConfig, host string) *UrlSafetyValidationResult {
	for _, address := range addresses {
		if policy.BlockPrivateNetworkTargets && u.isNonPublicAddress(address) {
			return DenyUrlSafetyValidationResult(fmt.Sprintf("host '%s' resolves to non-public address %s.", host, address.String()))
		}

		for _, cidr := range policy.BlockedCidrs {
			if IsBlank(cidr) {
				continue
			}

			if u.addressMatchesCidr(address, strings.TrimSpace(cidr)) {
				return DenyUrlSafetyValidationResult(fmt.Sprintf("host '%s' resolves to %s, which matches blocked CIDR '%s'.", host, address.String(), cidr))
			}

		}
	}

	return AllowUrlSafetyValidationResult()
}

var BuiltInBlockedHostGlobs = []string{
	"localhost",
	"*.localhost",
	"metadata",
	"metadata.google.internal",
}

func (u *UrlSafetyValidator) ValidateHttpUrl(ctx context.Context, uri url.URL, policy *UrlSafetyConfig) *UrlSafetyValidationResult {
	if policy == nil {
		policy = &UrlSafetyConfig{}
	}

	// 1. 优先检查 Context 是否已经结束
	if err := ctx.Err(); err != nil {
		return DenyUrlSafetyValidationResult(fmt.Sprintf("validation canceled: %v", err))
	}

	// 2. 第一阶段：进行不触发 DNS 解析的“轻量级初筛”
	preliminary := u.ValidateHttpUrlLocal(uri, policy)
	if !preliminary.Allowed {
		return preliminary
	}

	// 3. 智能短路：如果不需要拦截私有网络且没有自定义 CIDR，直接放行
	if !policy.Enabled || (!policy.BlockPrivateNetworkTargets && len(policy.BlockedCidrs) == 0) {
		return AllowUrlSafetyValidationResult()
	}

	// 4. 智能短路：如果是字面量 IP，初筛阶段就已经在本地被 validateAddresses 校验过了
	host := u.normalizeHost(uri)
	if _, err := netip.ParseAddr(host); err == nil {
		return preliminary
	}

	// 5. 第二阶段：执行带 Context 的非阻塞 DNS 解析
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return DenyUrlSafetyValidationResult(fmt.Sprintf("DNS resolution failed for '%s': %v", host, err))
	}

	if len(addresses) == 0 {
		return DenyUrlSafetyValidationResult(fmt.Sprintf("DNS resolution returned no addresses for '%s'", host))
	}

	return u.validateAddresses(addresses, policy, host)
}

// ValidateHttpUrlLocal 仅执行本地规则校验，绝不触网（不解析 DNS）。
// 对应你原代码中 resolveDns = false 的逻辑，独立出来利于复用。
func (u *UrlSafetyValidator) ValidateHttpUrlLocal(uri url.URL, policy *UrlSafetyConfig) *UrlSafetyValidationResult {
	if policy == nil {
		policy = &UrlSafetyConfig{}
	}

	if !policy.Enabled {
		return AllowUrlSafetyValidationResult()
	}

	if !uri.IsAbs() || (uri.Scheme != "http" && uri.Scheme != "https") {
		return DenyUrlSafetyValidationResult("only absolute http(s) URLs are allowed.")
	}

	host := u.normalizeHost(uri)
	if IsBlank(host) {
		return DenyUrlSafetyValidationResult("URL host is empty.")
	}

	for _, pattern := range BuiltInBlockedHostGlobs {
		if policy.BlockPrivateNetworkTargets && GlobMatcherInstance.IsMatch(pattern, host) {
			return DenyUrlSafetyValidationResult(fmt.Sprintf("host '%s' is blocked.", host))
		}
	}

	for _, pattern := range policy.BlockedHostGlobs {
		if GlobMatcherInstance.IsMatch(pattern, host) {
			return DenyUrlSafetyValidationResult(fmt.Sprintf("host '%s' matches blocklist entry '%s'", host, pattern))
		}
	}

	// 如果直接是 IP 地址，本地就能得出最终结论
	if literalIp, err := netip.ParseAddr(host); err == nil {
		return u.validateAddresses([]netip.Addr{literalIp}, policy, host)
	}

	return AllowUrlSafetyValidationResult()
}

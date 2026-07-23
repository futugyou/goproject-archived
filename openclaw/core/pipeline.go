package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DynamicCommandRegistrationResult uint8

const (
	Registered DynamicCommandRegistrationResult = iota
	ReservedBuiltIn
	Duplicate
)

type CompactCallback func(context.Context, *Session) (int, error)
type LoopCallback func(context.Context, *Session, string) (string, error)
type CommandCallback func(context.Context, string) (string, error)

var BuiltInCommands = map[string]struct{}{
	"/status":  {},
	"/new":     {},
	"/reset":   {},
	"/model":   {},
	"/usage":   {},
	"/think":   {},
	"/compact": {},
	"/concise": {},
	"/verbose": {},
	"/goal":    {},
	"/loop":    {},
	"/help":    {},
}

type ChatCommandProcessor struct {
	sessionManager  *SessionManager
	providerUsage   *ProviderUsageTracker
	goalService     IGoalService
	dynamicCommands sync.Map //map[string]CommandCallback
	compactCallback CompactCallback
	loopCallback    LoopCallback
}

func NewChatCommandProcessor(sessionManager *SessionManager, providerUsage *ProviderUsageTracker, goalService IGoalService) *ChatCommandProcessor {
	return &ChatCommandProcessor{
		sessionManager: sessionManager,
		providerUsage:  providerUsage,
		goalService:    goalService,
	}
}

func (c *ChatCommandProcessor) getCacheTotals(session *Session) (CacheReadTokens, CacheWriteTokens int64) {
	totalCacheReadTokens := session.GetTotalCacheReadTokens()
	totalCacheWriteTokens := session.GetTotalCacheWriteTokens()
	if totalCacheReadTokens > 0 || totalCacheWriteTokens > 0 {
		return totalCacheReadTokens, totalCacheWriteTokens
	}
	if c.providerUsage != nil {
		return c.providerUsage.GetLatestSessionCacheTotals(session.Id)
	}

	return 0, 0
}

var (
	budgetRegex = regexp.MustCompile(`\+(?P<budget>\d+(?:\.\d+)?)(?P<suffix>[kKmM])?\s*$`)
	spendRegex  = regexp.MustCompile(`(?i)spend\s+(?P<budget>\d+(?:\.\d+)?)\s*(?P<suffix>[kKmM])?\s*tokens?\s*$`)
)

func (c *ChatCommandProcessor) handleGoalCommand(ctx context.Context, session *Session, args string) string {
	if c.goalService == nil {
		return "Goal system is not available. Start the gateway with goal support enabled."
	}

	trimmedArgs := strings.TrimLeft(args, " ")
	subcommand := "status"
	subargs := ""

	if trimmedArgs != "" {
		idx := strings.Index(trimmedArgs, " ")
		if idx == -1 {
			subcommand = strings.ToLower(trimmedArgs)
		} else {
			subcommand = strings.ToLower(trimmedArgs[:idx])
			subargs = strings.TrimSpace(trimmedArgs[idx+1:])
		}
	}

	switch subcommand {
	case "start", "set", "create":
		remaining := subargs
		var budget int64 = 0

		if budgetMatch := budgetRegex.FindStringSubmatchIndex(remaining); budgetMatch != nil {
			matches := budgetRegex.FindStringSubmatch(remaining)
			budgetVal, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				var multiplier float64 = 1
				switch strings.ToLower(matches[2]) {
				case "k":
					multiplier = 1000
				case "m":
					multiplier = 1000000
				}
				budget = int64(budgetVal * multiplier)
				remaining = strings.TrimSpace(remaining[:budgetMatch[0]])
			}
		}

		if spendMatch := spendRegex.FindStringSubmatchIndex(remaining); spendMatch != nil {
			matches := spendRegex.FindStringSubmatch(remaining)
			budgetVal, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				var multiplier float64 = 1
				switch strings.ToLower(matches[2]) {
				case "k":
					multiplier = 1000
				case "m":
					multiplier = 1000000
				}
				budget = int64(budgetVal * multiplier)
				remaining = strings.TrimSpace(remaining[:spendMatch[0]])
			}
		}

		objective := strings.TrimSpace(remaining)
		if objective == "" {
			return "Usage: /goal start <objective> [+<budget>]\nExample: /goal start fix auth bug +500k"
		}

		existing, err := c.goalService.GetGoal(ctx, session.Id)
		if existing != nil {
			return fmt.Sprintf("A goal already exists: \"%s\"\nClear it with /goal clear first.", existing.Objective)
		}

		goal, err := c.goalService.CreateGoal(ctx, session.Id, objective, budget, session.GetTotalTokens())
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}

		budgetInfo := " (no budget limit)"
		if budget > 0 {
			budgetInfo = fmt.Sprintf(" with budget %d", budget)
		}
		return fmt.Sprintf("Goal created: \"%s\"%s", goal.Objective, budgetInfo)

	case "pause":
		_, err := c.goalService.GetGoal(ctx, session.Id)
		if err != nil {
			return "No active goal to pause."
		}
		if err := c.goalService.UpdateStatus(ctx, session.Id, GoalStatus_Paused, subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal paused. Resume with /goal resume."

	case "resume":
		goal, err := c.goalService.GetGoal(ctx, session.Id)
		if err != nil {
			return "No goal to resume."
		}
		if goal.Status.IsPursuable() {
			return "Goal is already active."
		}
		if goal.Status.IsTerminal() {
			return "Cannot resume a completed goal. Start a new one with /goal start."
		}

		if err := c.goalService.UpdateStatus(ctx, session.Id, GoalStatus_Active, subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal resumed."

	case "complete", "done":
		_, err := c.goalService.GetGoal(ctx, session.Id)
		if err != nil {
			return "No active goal."
		}
		if err := c.goalService.UpdateStatus(ctx, session.Id, GoalStatus_Complete, subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal marked as complete!"

	case "block", "blocked":
		_, err := c.goalService.GetGoal(ctx, session.Id)
		if err != nil {
			return "No active goal."
		}
		if err := c.goalService.UpdateStatus(ctx, session.Id, GoalStatus_Blocked, subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal marked as blocked. Resume with /goal resume."

	case "clear":
		c.goalService.ClearGoal(ctx, session.Id)
		return "Goal cleared."

	case "status":
		fallthrough
	default:
		statusGoal, err := c.goalService.GetGoal(ctx, session.Id)
		if err != nil {
			return "No active goal. Use /goal start <objective> to create one."
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Goal Status: %s\n", statusGoal.Status.ToDisplayName())
		fmt.Fprintf(&sb, "Objective: %s\n", statusGoal.Objective)
		fmt.Fprintf(&sb, "Tokens Used: %d", statusGoal.TokensUsed)

		if statusGoal.TokenBudget > 0 {
			fmt.Fprintf(&sb, "\nBudget: %d (Remaining: %d)", statusGoal.TokenBudget, statusGoal.RemainingBudget())
		}
		if statusGoal.StatusNote != "" {
			fmt.Fprintf(&sb, "\nNote: %s", statusGoal.StatusNote)
		}
		fmt.Fprintf(&sb, "\nContinuations: %d/%d", statusGoal.ContinuationCount, 10)

		return sb.String()
	}
}

func (c *ChatCommandProcessor) SetCompactCallback(compactCallback CompactCallback) {
	c.compactCallback = compactCallback
}

func (c *ChatCommandProcessor) SetLoopCallback(loopCallback LoopCallback) {
	c.loopCallback = loopCallback
}

func (c *ChatCommandProcessor) RegisterDynamic(command string, handler CommandCallback) DynamicCommandRegistrationResult {
	key := command
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	if _, ok := BuiltInCommands[key]; ok {
		return ReservedBuiltIn
	}

	_, ok := c.dynamicCommands.Load(key)
	if ok {
		return Duplicate
	}

	c.dynamicCommands.Store(key, handler)
	return Registered
}
func (c *ChatCommandProcessor) TryProcessCommand(ctx context.Context, session *Session, text string) (bool, string, error) {
	text = strings.TrimSpace(text)
	if text == "" || !strings.HasPrefix(text, "/") {
		return false, "", nil
	}

	// 模拟 C# 的 text.Split(' ', 2, StringSplitOptions.RemoveEmptyEntries)
	parts := strings.SplitN(text, " ", 2)
	command := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch command {
	case "/status":
		activeModel := session.ModelOverride
		if IsBlank(activeModel) {
			activeModel = "default"
		}
		statusCacheRead, statusCacheWrite := c.getCacheTotals(session)
		msg := fmt.Sprintf("Session info:\n- Active Model: %s\n- Turn Count: %d\n- Token Usage: %d in / %d out\n- Prompt Cache: %d read / %d write",
			activeModel, len(session.History), session.GetTotalInputTokens(), session.GetTotalOutputTokens(), statusCacheRead, statusCacheWrite)
		return true, msg, nil

	case "/new", "/reset":
		session.History = nil
		session.SetTotalInputTokens(0)
		session.SetTotalOutputTokens(0)
		if c.goalService != nil {
			c.goalService.ClearGoal(ctx, session.Id)
		}
		if err := c.sessionManager.Persist(ctx, session, false); err != nil {
			return true, "", err
		}
		return true, "Session history has been reset. Starting fresh!", nil

	case "/model":
		if args == "" {
			current := session.ModelOverride
			if IsBlank(current) {
				current = "none (using default)"
			}
			return true, fmt.Sprintf("Current model override: %s\nUsage: /model <model-name> or /model reset", current), nil
		}

		if strings.EqualFold(args, "reset") || strings.EqualFold(args, "clear") {
			session.ModelOverride = ""
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Model override cleared. Back to default.", nil
		}

		session.ModelOverride = args
		if err := c.sessionManager.Persist(ctx, session, false); err != nil {
			return true, "", err
		}
		return true, fmt.Sprintf("Model override set to: %s", args), nil

	case "/usage":
		usageCacheRead, usageCacheWrite := c.getCacheTotals(session)
		msg := fmt.Sprintf("Total Token Usage in this session:\n- Input: %d\n- Output: %d\n- Sum: %d\n- Prompt Cache Read: %d\n- Prompt Cache Write: %d",
			session.GetTotalInputTokens(), session.GetTotalOutputTokens(), session.GetTotalTokens(), usageCacheRead, usageCacheWrite)
		return true, msg, nil

	case "/think":
		if args == "" {
			current := session.ReasoningEffort
			if IsBlank(current) {
				current = "default"
			}
			return true, fmt.Sprintf("Current reasoning effort: %s\nUsage: /think off|low|medium|high", current), nil
		}

		level := strings.ToLower(args)
		if level == "off" || level == "low" || level == "medium" || level == "high" {
			if level == "off" {
				session.ReasoningEffort = ""
			} else {
				session.ReasoningEffort = level
			}

			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}

			if level == "off" {
				return true, "Extended thinking disabled.", nil
			}
			return true, fmt.Sprintf("Reasoning effort set to: %s", level), nil
		}
		return true, "Invalid level. Use: /think off|low|medium|high", nil

	case "/compact":
		if len(session.History) <= 2 {
			return true, "Nothing to compact — session has 2 or fewer turns.", nil
		}

		turnsBefore := len(session.History)
		if c.compactCallback != nil {
			remainingTurns, err := c.compactCallback(ctx, session)
			if err != nil {
				return true, "", err
			}
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, fmt.Sprintf("Compacted: %d turns → %d turns remaining.", turnsBefore, remainingTurns), nil
		}

		// Fallback 裁剪逻辑
		keepRecent := int(math.Min(10, float64(len(session.History))))
		removeCount := len(session.History) - keepRecent
		if removeCount > 0 {
			session.History = session.History[removeCount:]
		}
		if err := c.sessionManager.Persist(ctx, session, false); err != nil {
			return true, "", err
		}
		return true, fmt.Sprintf("Trimmed: %d turns → %d turns (kept last %d).", turnsBefore, len(session.History), keepRecent), nil

	case "/verbose":
		if args == "" {
			status := "off"
			if session.VerboseMode {
				status = "on"
			}
			return true, fmt.Sprintf("Verbose mode: %s\nUsage: /verbose on|off", status), nil
		}

		if strings.EqualFold(args, "on") {
			session.VerboseMode = true
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Verbose mode enabled. Tool calls and token counts will be shown.", nil
		}
		if strings.EqualFold(args, "off") {
			session.VerboseMode = false
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Verbose mode disabled.", nil
		}
		return true, "Usage: /verbose on|off", nil

	case "/concise":
		if args == "" {
			currentMode := "auto"
			switch session.ResponseMode {
			case SessionResponseModesConciseOps:
				currentMode = "on"
			case SessionResponseModesFull:
				currentMode = "off"
			}
			return true, fmt.Sprintf("Concise mode: %s\nUsage: /concise on|off|auto", currentMode), nil
		}

		if strings.EqualFold(args, "on") {
			session.ResponseMode = SessionResponseModesConciseOps
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Concise operational mode enabled.", nil
		}
		if strings.EqualFold(args, "off") {
			session.ResponseMode = SessionResponseModesFull
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Concise operational mode disabled for this session.", nil
		}
		if strings.EqualFold(args, "auto") {
			session.ResponseMode = SessionResponseModesDefault
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Concise mode reset to automatic behavior.", nil
		}
		return true, "Usage: /concise on|off|auto", nil

	case "/help":
		helpMsg := "Available commands:\n/status - Show session details\n/new (or /reset) - Clear conversation history\n/model <name> - Override the LLM model for this session\n/model reset - Clear model override\n/usage - Show token counts\n/think <level> - Set reasoning effort (off/low/medium/high)\n/compact - Compact conversation history\n/concise on|off|auto - Control concise operational responses\n/verbose on|off - Toggle verbose output\n/goal <action> - Manage session goals (start/set/create/pause/resume/complete/done/block/blocked/clear/status)\n/goal start <objective> +500k - Start a goal with a token budget\n/goal start <objective> spend 1.5m tokens - Start a goal with a budget phrase\n/help - Show this message"
		return true, helpMsg, nil

	case "/loop":
		if c.loopCallback == nil {
			return true, "Loop scheduling is not available in this configuration.", nil
		}
		loopResult, err := c.loopCallback(ctx, session, text)
		if err != nil {
			return true, "", err
		}
		return true, loopResult, nil

	case "/goal":
		goalResult := c.handleGoalCommand(ctx, session, args)
		return true, goalResult, nil

	default:
		if dynamicHandlerValue, ok := c.dynamicCommands.Load(command); ok {
			if dynamicHandler, ok := dynamicHandlerValue.(CommandCallback); ok && dynamicHandler != nil {
				dynamicResult, err := dynamicHandler(ctx, args)
				if err != nil {
					return true, "", err
				}
				return true, dynamicResult, nil
			}
		}
		return false, "", nil
	}
}

type MessagePipeline struct {
	inbound  chan InboundMessage
	outbound chan OutboundMessage
	logger   *slog.Logger
	once     sync.Once
}

func NewMessagePipeline(capacity int, logger *slog.Logger) *MessagePipeline {
	if capacity <= 0 {
		capacity = 1024
	}
	return &MessagePipeline{
		inbound:  make(chan InboundMessage, capacity),
		outbound: make(chan OutboundMessage, capacity),
		logger:   logger,
	}
}

func (mp *MessagePipeline) InboundWriter() chan<- InboundMessage {
	return mp.inbound
}

func (mp *MessagePipeline) InboundReader() <-chan InboundMessage {
	return mp.inbound
}

func (mp *MessagePipeline) OutboundWriter() chan<- OutboundMessage {
	return mp.outbound
}

func (mp *MessagePipeline) OutboundReader() <-chan OutboundMessage {
	return mp.outbound
}

func (mp *MessagePipeline) Close() error {
	mp.once.Do(func() {
		// 1. 关闭写入端
		close(mp.inbound)
		close(mp.outbound)

		// 2. 清空残留消息
		mp.drainInbound()
		mp.drainOutbound()
	})
	return nil
}

func (mp *MessagePipeline) drainInbound() {
	count := 0
	for range mp.inbound {
		count++
	}
	if count > 0 && mp.logger != nil {
		mp.logger.Info(fmt.Sprintf("MessagePipeline: %d inbound message(s) dropped during shutdown.\n", count))
	}
}

func (mp *MessagePipeline) drainOutbound() {
	count := 0
	for range mp.outbound {
		count++
	}
	if count > 0 && mp.logger != nil {
		mp.logger.Info(fmt.Sprintf("MessagePipeline: %d outbound message(s) dropped during shutdown.\n", count))
	}
}

type RecentSenderEntry struct {
	SenderId    string    `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	LastSeenUtc time.Time `json:"last_seen"`
}

type RecentSendersFile struct {
	Senders []RecentSenderEntry `json:"senders"`
}

type ToolApprovalDecisionResult uint8

const (
	ToolApprovalDecisionResult_Recorded ToolApprovalDecisionResult = iota
	ToolApprovalDecisionResult_NotFound
	ToolApprovalDecisionResult_Unauthorized
)

type ToolApprovalWaitResult uint8

const (
	ToolApprovalWaitResult_Approved ToolApprovalWaitResult = iota
	ToolApprovalWaitResult_Denied
	ToolApprovalWaitResult_TimedOut
	ToolApprovalWaitResult_NotFound
)

type ToolApprovalDecisionOutcome struct {
	Result  ToolApprovalDecisionResult `json:"result"`
	Request *ToolApprovalRequest       `json:"request"`
}

type ToolApprovalWaitOutcome struct {
	Result  ToolApprovalWaitResult `json:"result"`
	Request *ToolApprovalRequest   `json:"request"`
}

type ToolApprovalRequest struct {
	ApprovalId string    `json:"approval_id"`
	SessionId  string    `json:"session_id"`
	ChannelId  string    `json:"channel_id"`
	SenderId   string    `json:"sender_id"`
	ToolName   string    `json:"tool_name"`
	Arguments  string    `json:"arguments"`
	Action     string    `json:"action"`
	IsMutation bool      `json:"is_mutation"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

type RecentSendersStore struct {
	rootDir     string
	logger      *slog.Logger
	lockManager *NamedLockManager
	maxEntries  int
}

func NewRecentSendersStore(baseStoragePath string, logger *slog.Logger, maxEntries int) *RecentSendersStore {
	path := filepath.Join(baseStoragePath, "recent_senders")
	if logger == nil {
		logger = slog.Default()
	}
	if maxEntries <= 0 {
		maxEntries = 50
	}
	return &RecentSendersStore{
		rootDir:     path,
		maxEntries:  maxEntries,
		logger:      logger,
		lockManager: NewNamedLockManager(),
	}
}

func (r *RecentSendersStore) getPath(channelId string) string {
	safe := []byte{}
	for i := 0; i < len(channelId); i++ {
		c := channelId[i]
		if IsLetterOrDigit(c) || c == '_' || c == '-' || c == '.' {
			safe = append(safe, c)
		}
	}

	if len(safe) == 0 {
		return filepath.Join(r.rootDir, "unknown.json")
	}
	return filepath.Join(r.rootDir, string(safe)+".json")
}

func (r *RecentSendersStore) GetSnapshot(channelId string) (*RecentSendersFile, error) {
	var path = r.getPath(channelId)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result RecentSendersFile
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *RecentSendersStore) TryGetLatest(channelId string) (*RecentSenderEntry, error) {
	if IsBlank(channelId) {
		return &RecentSenderEntry{}, nil
	}

	data, err := r.GetSnapshot(channelId)
	if err != nil {
		return nil, err
	}

	if len(data.Senders) > 0 {
		return &data.Senders[0], nil
	}
	return &RecentSenderEntry{}, nil
}

func (r *RecentSendersStore) saveUnlocked(ctx context.Context, path string, file *RecentSendersFile) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tempFile, err := os.CreateTemp(dir, base+".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	defer func() {
		if tempFile != nil {
			tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	encoder := json.NewEncoder(tempFile)
	if err := encoder.Encode(file); err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	tempFile.Close()
	tempFile = nil
	if err := ctx.Err(); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

func (r *RecentSendersStore) loadUnlocked(_ context.Context, path string) (*RecentSendersFile, error) {
	var result RecentSendersFile
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		if os.IsNotExist(err) {
			return &RecentSendersFile{}, nil
		}
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return &RecentSendersFile{}, nil
	}

	return &result, nil
}

func (r *RecentSendersStore) Record(ctx context.Context, channelId, senderId, senderName string) error {
	if IsBlank(channelId) || IsBlank(senderId) {
		return nil
	}

	unlock, err := r.lockManager.Lock(ctx, channelId)
	if err != nil {
		return err
	}
	defer unlock()

	var path = r.getPath(channelId)
	file, err := r.loadUnlocked(ctx, path)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	entry := RecentSenderEntry{
		SenderId:    senderId,
		SenderName:  senderName,
		LastSeenUtc: now,
	}

	if len(file.Senders) == 0 {
		file.Senders = append(file.Senders, entry)
	} else {
		remaining := file.Senders[:0]
		for _, s := range file.Senders {
			if strings.EqualFold(s.SenderId, senderId) {
				if senderName == "" {
					entry.SenderName = s.SenderName
				}
				continue
			}
			remaining = append(remaining, s)
		}

		file.Senders = append([]RecentSenderEntry{entry}, remaining...)
	}

	if len(file.Senders) > r.maxEntries {
		file.Senders = file.Senders[:r.maxEntries]
	}
	return r.saveUnlocked(ctx, path, file)
}

var AlwaysMutatingTools = map[string]struct{}{
	"write_file":           {},
	"edit_file":            {},
	"apply_patch":          {},
	"shell":                {},
	"code_exec":            {},
	"git":                  {},
	"database":             {},
	"home_assistant_write": {},
	"mqtt_publish":         {},
	"notion_write":         {},
	"inbox_zero":           {},
	"email":                {},
	"calendar":             {},
	"delegate_agent":       {},
}

type ToolActionPolicyResolver struct{}

var ToolActionPolicyResolverInstance = &ToolActionPolicyResolver{}

func (t *ToolActionPolicyResolver) Resolve(toolName string, argumentsJson string) *ToolActionDescriptor {
	if strings.TrimSpace(toolName) == "" {
		return &ToolActionDescriptor{}
	}

	var root map[string]any
	if strings.TrimSpace(argumentsJson) == "" {
		argumentsJson = "{}"
	}

	if err := json.Unmarshal([]byte(argumentsJson), &root); err != nil {
		return &ToolActionDescriptor{
			Summary: fmt.Sprintf("Execute tool '%s'.", toolName),
		}
	}

	getString := func(key string, fallback string) string {
		if val, exists := root[key]; exists {
			if strVal, ok := val.(string); ok && strVal != "" {
				return strVal
			}
		}
		return fallback
	}

	isOneOf := func(val string, options ...string) bool {
		return slices.Contains(options, val)
	}

	if strings.EqualFold(toolName, "process") {
		action := getString("action", "start")
		command := getString("command", "")
		processId := getString("process_id", getString("session_id", ""))

		var summary string
		switch action {
		case "start":
			if command == "" {
				summary = "Start a background process."
			} else {
				summary = fmt.Sprintf("Start process: %s", command)
			}
		case "write":
			if processId == "" {
				summary = "Write input to a background process."
			} else {
				summary = fmt.Sprintf("Write input to process %s.", processId)
			}
		case "kill":
			if processId == "" {
				summary = "Terminate a background process."
			} else {
				summary = fmt.Sprintf("Terminate process %s.", processId)
			}
		case "wait":
			if processId == "" {
				summary = "Wait for a background process."
			} else {
				summary = fmt.Sprintf("Wait for process %s.", processId)
			}
		case "log":
			if processId == "" {
				summary = "Read background process logs."
			} else {
				summary = fmt.Sprintf("Read logs for process %s.", processId)
			}
		case "poll":
			if processId == "" {
				summary = "Check background process status."
			} else {
				summary = fmt.Sprintf("Check status for process %s.", processId)
			}
		default:
			summary = "Inspect background processes."
		}

		return &ToolActionDescriptor{
			Action:     action,
			IsMutation: isOneOf(action, "start", "write", "kill"),
			Summary:    summary,
		}
	}

	// 2. Automation 工具
	if strings.EqualFold(toolName, "automation") {
		action := getString("action", "list")
		automationId := getString("automation_id", getString("id", ""))
		name := getString("name", "")

		var summary string
		switch action {
		case "create":
			if name == "" {
				summary = "Create an automation."
			} else {
				summary = fmt.Sprintf("Create automation '%s'.", name)
			}
		case "update":
			if automationId == "" {
				summary = "Update an automation."
			} else {
				summary = fmt.Sprintf("Update automation %s.", automationId)
			}
		case "pause":
			if automationId == "" {
				summary = "Pause an automation."
			} else {
				summary = fmt.Sprintf("Pause automation %s.", automationId)
			}
		case "resume":
			if automationId == "" {
				summary = "Resume an automation."
			} else {
				summary = fmt.Sprintf("Resume automation %s.", automationId)
			}
		case "run":
			if automationId == "" {
				summary = "Run an automation."
			} else {
				summary = fmt.Sprintf("Run automation %s.", automationId)
			}
		case "preview":
			summary = "Preview an automation."
		case "get":
			if automationId == "" {
				summary = "Read automation details."
			} else {
				summary = fmt.Sprintf("Read automation %s.", automationId)
			}
		default:
			summary = "List automations."
		}

		return &ToolActionDescriptor{
			Action:     action,
			IsMutation: isOneOf(action, "create", "update", "pause", "resume", "run"),
			Summary:    summary,
		}
	}

	// 3. External CLI 工具
	if strings.EqualFold(toolName, "external_cli") {
		action := getString("action", "list_connectors")
		connector := getString("connector", "")
		command := getString("command", "")

		target := "an external CLI command"
		if connector != "" && command != "" {
			target = fmt.Sprintf("%s/%s", connector, command)
		}

		var summary string
		switch action {
		case "execute":
			summary = fmt.Sprintf("Execute %s.", target)
		case "preview":
			summary = fmt.Sprintf("Preview %s.", target)
		case "connector_status":
			if connector == "" {
				summary = "Check external CLI connector status."
			} else {
				summary = fmt.Sprintf("Check external CLI connector '%s'.", connector)
			}
		case "list_commands":
			if connector == "" {
				summary = "List external CLI commands."
			} else {
				summary = fmt.Sprintf("List commands for external CLI connector '%s'.", connector)
			}
		case "command_schema":
			summary = fmt.Sprintf("Inspect schema for %s.", target)
		default:
			summary = "List external CLI connectors."
		}

		return &ToolActionDescriptor{
			Action:           action,
			IsMutation:       action == "execute",
			RequiresApproval: action == "execute",
			Summary:          summary,
		}
	}

	// 4. Todo 工具
	if strings.EqualFold(toolName, "todo") {
		action := getString("action", "list")

		var summary string
		switch action {
		case "add":
			summary = "Add a todo item."
		case "update":
			summary = "Update a todo item."
		case "complete":
			summary = "Complete a todo item."
		case "remove":
			summary = "Remove a todo item."
		case "clear":
			summary = "Clear all todo items."
		default:
			summary = "List todo items."
		}

		return &ToolActionDescriptor{
			Action:     action,
			IsMutation: action != "list",
			Summary:    summary,
		}
	}

	return &ToolActionDescriptor{
		Summary: fmt.Sprintf("Execute tool '%s'.", toolName),
	}
}

func (t *ToolActionPolicyResolver) SupportsActionAwareApproval(toolName string) bool {
	return strings.EqualFold(toolName, "process") || strings.EqualFold(toolName, "automation") || strings.EqualFold(toolName, "external_cli")
}

func (t *ToolActionPolicyResolver) IsMutationCapable(toolName, argumentsJson string) bool {
	if IsBlank(toolName) {
		return false
	}

	if _, ok := AlwaysMutatingTools[toolName]; ok {
		return true
	}

	return t.Resolve(toolName, argumentsJson).IsMutation
}

type pendingRequest struct {
	Request      *ToolApprovalRequest
	ExpiresAt    time.Time
	decisionChan chan bool
	setDecision  sync.Once
}

type ToolApprovalService struct {
	mu      sync.RWMutex
	pending map[string]*pendingRequest
}

func NewToolApprovalService() *ToolApprovalService {
	return &ToolApprovalService{
		pending: make(map[string]*pendingRequest),
	}
}

func (s *ToolApprovalService) PendingCount() int {
	return len(s.ListPending("", ""))
}

func (s *ToolApprovalService) Create(
	sessionId, channelId, senderId, toolName, arguments string,
	timeout time.Duration,
	action string, isMutation bool, summary string,
) *ToolApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	approvalId := "apr_" + generateShortUUID()

	req := &ToolApprovalRequest{
		ApprovalId: approvalId,
		SessionId:  sessionId,
		ChannelId:  channelId,
		SenderId:   senderId,
		ToolName:   toolName,
		Arguments:  arguments,
		IsMutation: isMutation,
		Summary:    summary,
	}
	if strings.TrimSpace(action) != "" {
		req.Action = action
	}

	p := &pendingRequest{
		Request:      req,
		ExpiresAt:    time.Now().Add(timeout),
		decisionChan: make(chan bool, 1), // 缓冲大小为 1，防止写端阻塞
	}

	s.pending[approvalId] = p
	return req
}

func (s *ToolApprovalService) TrySetDecision(
	approvalId string, approved bool,
	requesterChannelId, requesterSenderId string,
	requireRequesterMatch bool,
) ToolApprovalDecisionOutcome {
	s.mu.RLock()
	p, exists := s.pending[approvalId]
	s.mu.RUnlock()

	if !exists {
		return ToolApprovalDecisionOutcome{Result: ToolApprovalDecisionResult_NotFound}
	}

	if requireRequesterMatch {
		if strings.TrimSpace(requesterChannelId) == "" || strings.TrimSpace(requesterSenderId) == "" {
			return ToolApprovalDecisionOutcome{Result: ToolApprovalDecisionResult_Unauthorized, Request: p.Request}
		}
		if requesterChannelId != p.Request.ChannelId || requesterSenderId != p.Request.SenderId {
			return ToolApprovalDecisionOutcome{Result: ToolApprovalDecisionResult_Unauthorized, Request: p.Request}
		}
	}

	// TrySetResult 使用 sync.Once 保证哪怕高并发下，决议也只会被写入一次
	hasSet := false
	p.setDecision.Do(func() {
		p.decisionChan <- approved
		hasSet = true
	})

	if !hasSet {
		// 说明在这一次调用前，结果已经被别的人设置过了（Task.IsCompleted == true）
		return ToolApprovalDecisionOutcome{Result: ToolApprovalDecisionResult_NotFound}
	}

	return ToolApprovalDecisionOutcome{
		Result:  ToolApprovalDecisionResult_Recorded,
		Request: p.Request,
	}
}

func (s *ToolApprovalService) WaitForDecisionOutcomeAsync(
	approvalId string, timeout time.Duration, ctx context.Context,
) (ToolApprovalWaitOutcome, error) {
	s.mu.RLock()
	p, exists := s.pending[approvalId]
	s.mu.RUnlock()

	if !exists {
		return ToolApprovalWaitOutcome{Result: ToolApprovalWaitResult_NotFound}, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		// 外部的 CancellationToken 被取消了
		return ToolApprovalWaitOutcome{}, ctx.Err()

	case approved := <-p.decisionChan:
		// 收到明确的审批结果（通过或拒绝）
		s.mu.Lock()
		delete(s.pending, approvalId)
		s.mu.Unlock()

		res := ToolApprovalWaitResult_Denied
		if approved {
			res = ToolApprovalWaitResult_Approved
		}
		return ToolApprovalWaitOutcome{Result: res, Request: p.Request}, nil

	case <-timer.C:
		// 审批超时
		s.mu.Lock()
		delete(s.pending, approvalId)
		s.mu.Unlock()

		// 尝试关闭/取消该请求
		// 如果在这一瞬间 TrySetDecision 抢先一步成功了，则这里 hasSet 会是 false
		hasCanceled := false
		p.setDecision.Do(func() {
			hasCanceled = true
		})

		if !hasCanceled {
			// 边缘情况竞态赢了：决议其实在超时的一瞬间被设置了，读取它
			approved := <-p.decisionChan
			res := ToolApprovalWaitResult_Denied
			if approved {
				res = ToolApprovalWaitResult_Approved
			}
			return ToolApprovalWaitOutcome{Result: res, Request: p.Request}, nil
		}

		return ToolApprovalWaitOutcome{Result: ToolApprovalWaitResult_TimedOut, Request: p.Request}, nil
	}
}

// ListPending 列出并清理过期的请求
func (s *ToolApprovalService) ListPending(channelId, senderId string) []*ToolApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var result []*ToolApprovalRequest

	for id, p := range s.pending {
		// 检查是否过期
		if p.ExpiresAt.Before(now) || p.ExpiresAt.Equal(now) {
			delete(s.pending, id)
			continue
		}

		// 检查 channel 是否已经有结果了（非阻塞读取，类似 Task.IsCompleted）
		select {
		case <-p.decisionChan:
			// 说明已经有结果了，将其移除
			delete(s.pending, id)
			continue
		default:
			// 没有结果，继续走下面的过滤逻辑
		}

		if channelId != "" && p.Request.ChannelId != channelId {
			continue
		}
		if senderId != "" && p.Request.SenderId != senderId {
			continue
		}

		result = append(result, p.Request)
	}

	return result
}

// 辅助函数：生成 16 位 Hex 字符串模拟 Guid:N 后截取
func generateShortUUID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) // 16个字符
}

type CronScheduler struct {
	maxRunningDuration time.Duration
	jobSource          ICronJobSource
	logger             *slog.Logger
	startupNoticeSink  IStartupNoticeSink
	pipelineChannel    chan<- InboundMessage
	runDispatcher      IAutomationRunDispatcher
	runningJobs        sync.Map //map[string]time.Time
}

func NewCronScheduler(jobSource ICronJobSource, logger *slog.Logger, startupNoticeSink IStartupNoticeSink, pipelineChannel chan<- InboundMessage, runDispatcher IAutomationRunDispatcher) *CronScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CronScheduler{
		maxRunningDuration: 6 * time.Hour,
		jobSource:          jobSource,
		logger:             logger,
		startupNoticeSink:  startupNoticeSink,
		pipelineChannel:    pipelineChannel,
		runDispatcher:      runDispatcher,
	}
}

const overlapLogTemplate = "Background job '%s' is still running from an earlier trigger; this tick was skipped."

func (c *CronScheduler) logOverlap(jobName string) {
	msg := fmt.Sprintf(overlapLogTemplate, jobName)
	c.logger.Warn(msg)
	c.startupNoticeSink.Record(msg)
}

func (c *CronScheduler) cleanupStaleRunningJobs(nowUtc time.Time) {
	c.runningJobs.Range(func(key, value any) bool {
		t := value.(time.Time)
		if nowUtc.Sub(t) <= c.maxRunningDuration {
			return true
		}
		c.runningJobs.Delete(key)

		msg := fmt.Sprintf("Removed stale running marker for cron job '%s' after %d.", key, nowUtc.Sub(t))
		c.logger.Warn(msg)
		return true
	})
}

func (c *CronScheduler) enqueueJob(ctx context.Context, job *CronJobConfig) error {
	sessionId := job.SessionId
	if IsBlank(sessionId) {
		if IsBlank(job.Name) {
			sessionId = "cron:system"
		} else {
			sessionId = fmt.Sprintf("cron:%s", job.Name)
		}

	}

	var channelId = job.ChannelId
	if IsBlank(channelId) {
		channelId = "cron"
	}

	senderId := job.RecipientId
	if senderId == "" {
		senderId = sessionId
	}
	if senderId == "" {
		senderId = job.Name
	}
	if senderId == "" {
		senderId = "system"
	}

	var msg *InboundMessage
	var err error
	subject := job.Subject
	if IsBlank(subject) {
		if !IsBlank(job.Name) {
			subject = fmt.Sprintf("OpenClaw Cron: %s", job.Name)
		}

	}
	if c.runDispatcher != nil && !IsBlank(job.AutomationId) {
		request := &AutomationDispatchRequest{
			AutomationId:  job.AutomationId,
			SessionId:     sessionId,
			ChannelId:     channelId,
			SenderId:      senderId,
			Prompt:        job.Prompt,
			TriggerSource: job.AutomationTriggerSource,
			Subject:       subject,
		}

		if IsBlank(request.TriggerSource) {
			request.TriggerSource = "schedule"
		}
		msg, err = c.runDispatcher.PrepareDispatch(ctx, request)
		if err != nil {
			return err
		}
	}

	if msg == nil {
		msg = &InboundMessage{
			IsSystem:    true,
			SessionId:   sessionId,
			CronJobName: job.Name,
			ChannelId:   channelId,
			SenderId:    senderId,
			Subject:     subject,
			Text:        job.Prompt,
		}
	}

	select {
	case c.pipelineChannel <- *msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *CronScheduler) enqueueJobIfNotRunning(ctx context.Context, job *CronJobConfig) error {
	var jobName = "unnamed"
	if !IsBlank(job.Name) {
		jobName = job.Name
	}
	var now = time.Now().UTC()

	runningSinceVal, ok := c.runningJobs.Load(jobName)
	if ok {
		runningSince := runningSinceVal.(time.Time)
		if now.Sub(runningSince) <= c.maxRunningDuration {
			c.logOverlap(jobName)
			return nil
		}

		c.runningJobs.Delete(jobName)
	}

	_, loaded := c.runningJobs.LoadOrStore(jobName, now)

	if loaded {
		c.logOverlap(jobName)
		return nil
	}

	err := c.enqueueJob(ctx, job)
	if err == nil {
		c.runningJobs.Delete(jobName)
	}

	return err
}

func (c *CronScheduler) MarkJobCompleted(jobName string) {
	if IsBlank(jobName) {
		return
	}

	c.runningJobs.Delete(jobName)
}

func (c *CronScheduler) RunTick(ctx context.Context) error {
	var utcNow = time.Now().UTC()
	c.cleanupStaleRunningJobs(utcNow)
	var jobs = c.jobSource.GetJobs()
	if len(jobs) == 0 {
		return nil
	}
	for _, job := range jobs {
		var now = utcNow
		if !IsBlank(job.Timezone) {
			tz, err := time.LoadLocation(job.Timezone)
			if err == nil {
				now = utcNow.In(tz)
			}
		}

		if !IsTime(job.CronExpression, now) {
			continue
		}

		if err := c.enqueueJobIfNotRunning(ctx, &job); err != nil {
			return err
		}
	}

	return nil
}

func (c *CronScheduler) RunStartupJobs(ctx context.Context) error {
	var initialJobs = c.jobSource.GetJobs()
	if len(initialJobs) == 0 {
		c.logger.Info("Cron scheduler startup dispatch found no initial jobs.")
		return nil
	}
	c.logger.Info("cron scheduler initiates dispatching: it checks for initial tasks to execute the RunOnStartup logic", "JobCount", len(initialJobs))

	for _, job := range initialJobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !job.RunOnStartup {
			continue
		}
		var now = time.Now().UTC()
		c.logger.Info("triggering cron job on startup", "JobName", job.Name, "Time", now)
		if err := c.enqueueJobIfNotRunning(ctx, &job); err != nil {
			c.logger.Info("failed to run cron job on startup", "JobName", job.Name)
		}
	}

	return nil
}

type SessionAbortRegistry struct {
	mu     sync.RWMutex
	active map[string]context.CancelFunc
}

func NewSessionAbortRegistry() *SessionAbortRegistry {
	return &SessionAbortRegistry{
		active: make(map[string]context.CancelFunc),
	}
}

func (r *SessionAbortRegistry) Register(sessionId string, parentCtx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parentCtx)

	r.mu.Lock()
	defer r.mu.Unlock()

	if existingCancel, exists := r.active[sessionId]; exists {
		existingCancel()
	}

	r.active[sessionId] = cancel
	return ctx, cancel
}

func (r *SessionAbortRegistry) Unregister(sessionId string) {
	if sessionId == "" {
		return
	}

	r.mu.Lock()
	cancel, exists := r.active[sessionId]
	if exists {
		delete(r.active, sessionId)
	}
	r.mu.Unlock()

	if exists {
		cancel()
	}
}

func (r *SessionAbortRegistry) TryAbort(sessionId string) bool {
	r.mu.RLock()
	cancel, exists := r.active[sessionId]
	r.mu.RUnlock()

	if exists {
		cancel()
		return true
	}
	return false
}

func (r *SessionAbortRegistry) ActiveSessionIds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.active))
	for id := range r.active {
		ids = append(ids, id)
	}
	return ids
}

package core

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type DynamicCommandRegistrationResult uint

const (
	Registered      DynamicCommandRegistrationResult = 0
	ReservedBuiltIn DynamicCommandRegistrationResult = 1
	Duplicate       DynamicCommandRegistrationResult = 2
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

func (c *ChatCommandProcessor) handleGoalCommand(_ context.Context, session *Session, args string) string {
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

		existing, err := c.goalService.GetGoal(session.Id)
		if existing != nil {
			return fmt.Sprintf("A goal already exists: \"%s\"\nClear it with /goal clear first.", existing.Objective)
		}

		goal, err := c.goalService.CreateGoal(session.Id, objective, budget, session.GetTotalTokens())
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}

		budgetInfo := " (no budget limit)"
		if budget > 0 {
			budgetInfo = fmt.Sprintf(" with budget %d", budget)
		}
		return fmt.Sprintf("Goal created: \"%s\"%s", goal.Objective, budgetInfo)

	case "pause":
		_, err := c.goalService.GetGoal(session.Id)
		if err != nil {
			return "No active goal to pause."
		}
		if err := c.goalService.UpdateStatus(session.Id, GoalStatus_Paused, &subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal paused. Resume with /goal resume."

	case "resume":
		goal, err := c.goalService.GetGoal(session.Id)
		if err != nil {
			return "No goal to resume."
		}
		if goal.Status.IsPursuable() {
			return "Goal is already active."
		}
		if goal.Status.IsTerminal() {
			return "Cannot resume a completed goal. Start a new one with /goal start."
		}

		if err := c.goalService.UpdateStatus(session.Id, GoalStatus_Active, &subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal resumed."

	case "complete", "done":
		_, err := c.goalService.GetGoal(session.Id)
		if err != nil {
			return "No active goal."
		}
		if err := c.goalService.UpdateStatus(session.Id, GoalStatus_Complete, &subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal marked as complete!"

	case "block", "blocked":
		_, err := c.goalService.GetGoal(session.Id)
		if err != nil {
			return "No active goal."
		}
		if err := c.goalService.UpdateStatus(session.Id, GoalStatus_Blocked, &subargs); err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return "Goal marked as blocked. Resume with /goal resume."

	case "clear":
		c.goalService.ClearGoal(session.Id)
		return "Goal cleared."

	case "status":
		fallthrough
	default:
		statusGoal, err := c.goalService.GetGoal(session.Id)
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
		activeModel := "default"
		if session.ModelOverride != nil {
			activeModel = *session.ModelOverride
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
			c.goalService.ClearGoal(session.Id)
		}
		if err := c.sessionManager.Persist(ctx, session, false); err != nil {
			return true, "", err
		}
		return true, "Session history has been reset. Starting fresh!", nil

	case "/model":
		if args == "" {
			current := "none (using default)"
			if session.ModelOverride != nil {
				current = *session.ModelOverride
			}
			return true, fmt.Sprintf("Current model override: %s\nUsage: /model <model-name> or /model reset", current), nil
		}

		if strings.EqualFold(args, "reset") || strings.EqualFold(args, "clear") {
			session.ModelOverride = nil
			if err := c.sessionManager.Persist(ctx, session, false); err != nil {
				return true, "", err
			}
			return true, "Model override cleared. Back to default.", nil
		}

		session.ModelOverride = &args
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
			current := "default"
			if session.ReasoningEffort != nil {
				current = *session.ReasoningEffort
			}
			return true, fmt.Sprintf("Current reasoning effort: %s\nUsage: /think off|low|medium|high", current), nil
		}

		level := strings.ToLower(args)
		if level == "off" || level == "low" || level == "medium" || level == "high" {
			if level == "off" {
				session.ReasoningEffort = nil
			} else {
				session.ReasoningEffort = &level
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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type AgentCheckpointManager struct {
	memory core.IMemoryStore
	logger *slog.Logger
}

func NewAgentCheckpointManager(memory core.IMemoryStore, logger *slog.Logger) *AgentCheckpointManager {
	return &AgentCheckpointManager{
		memory: memory,
		logger: logger,
	}
}

func (a *AgentCheckpointManager) PersistToolBatchCheckpoint(ctx context.Context, session core.Session, turnCtx *core.TurnContext, iteration int, invocations []core.ToolInvocation) error {
	if len(invocations) == 0 {
		return nil
	}
	sequence := 1
	if session.ExecutionCheckpoint != nil {
		sequence += session.ExecutionCheckpoint.Sequence
	}

	var checkpoint = core.SessionExecutionCheckpoint{
		CheckpointId:  fmt.Sprintf("chk_%s", util.CleanUUID())[:20],
		Kind:          "tool_batch",
		State:         "ready_to_resume",
		Sequence:      sequence,
		Iteration:     iteration,
		HistoryCount:  len(session.History),
		CorrelationId: turnCtx.CorrelationId,
		CreatedAtUtc:  time.Now().UTC(),
		ToolCalls:     a.getToolCalls(invocations),
	}

	session.ExecutionCheckpoint = &checkpoint

	maxRetries := 3
	var delay = time.Millisecond * 100

	recordRetry := func(attempt int, err error) error {
		checkpoint.PersistedAtUtc = nil
		a.logger.Warn("checkpoint persistence failed", "CorrelationId", turnCtx.CorrelationId, "Attempt", attempt, "MaxRetries", maxRetries, "SessionId", session.Id, "Error", err.Error())
		select {
		case <-time.After(delay):
			delay *= 2
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	recordFinalFailure := func(err error) {
		checkpoint.PersistedAtUtc = nil
		a.logger.Warn("failed to persist checkpoint after 3 attempts", "CorrelationId", turnCtx.CorrelationId, "SessionId", session.Id, "Error", err.Error())
	}

	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		checkpoint.PersistedAtUtc = util.Ptr(time.Now().UTC())
		err = a.memory.SaveSession(ctx, session)
		if err != nil && attempt < maxRetries {
			recordRetry(attempt, err)
		}
	}

	if err != nil {
		recordFinalFailure(err)
	}
	return err
}

func (a *AgentCheckpointManager) getToolCalls(invocations []core.ToolInvocation) []core.SessionCheckpointToolCall {
	result := []core.SessionCheckpointToolCall{}
	for _, invocation := range invocations {
		call := core.SessionCheckpointToolCall{
			CallId:         invocation.CallId,
			ToolName:       invocation.ToolName,
			FailureCode:    invocation.FailureCode,
			DurationMs:     invocation.Duration.Milliseconds(),
			ArgumentsBytes: len(invocation.Arguments),
			ResultBytes:    len(invocation.Result),
			ResultStatus:   invocation.ResultStatus,
		}

		if call.ResultStatus == "" {
			call.ResultStatus = "completed"
		}
		result = append(result, call)
	}
	return result
}

func ManagerDeserializeToolArguments(arguments string) map[string]any {
	result := map[string]any{}
	if arguments == "" {
		return result
	}

	if err := json.Unmarshal([]byte(arguments), &result); err != nil {
		result["_raw"] = arguments
	}

	return result
}

func ManagerResolveCheckpointCallId(invocation core.ToolInvocation, index int) string {
	if invocation.CallId == "" {
		return fmt.Sprintf("checkpoint_call_%d", index+1)
	}

	return invocation.CallId
}

func ManagerIsBareResumeRequest(userMessage string) bool {
	var trimmed = strings.TrimSpace(userMessage)
	return len(trimmed) == 0 || strings.EqualFold(trimmed, "resume") || strings.EqualFold(trimmed, "continue") || strings.EqualFold(trimmed, "/resume") || strings.EqualFold(trimmed, "/continue")
}

func ManagerBuildCheckpointResumeUserNote(userMessage string) string {
	return "[Checkpoint resume user note]\n" + strings.TrimSpace(userMessage) + "\n[/Checkpoint resume user note]"
}

func ManagerBuildCheckpointResumeInstruction(checkpoint core.SessionExecutionCheckpoint) string {
	var sb = strings.Builder{}
	sb.WriteString("[Checkpoint resume]\n")
	fmt.Fprintf(&sb, "Resume from checkpoint %s.\n", checkpoint.CheckpointId)
	sb.WriteString("The previous assistant tool batch and tool results have already completed and are present in this conversation context.\n")
	sb.WriteString("Continue the interrupted task from those results. Do not repeat completed tool calls unless the results show that retrying is necessary.\n")
	sb.WriteString("[/Checkpoint resume]\n")
	return sb.String()
}

func ManagerMarkCheckpointCompleted(session *core.Session, state, reason string) {
	var checkpoint = session.ExecutionCheckpoint
	if checkpoint == nil || !strings.EqualFold(checkpoint.State, "ready_to_resume") {
		return
	}

	checkpoint.State = state
	checkpoint.CompletedAtUtc = util.Ptr(time.Now().UTC())
	checkpoint.CompletionReason = reason
}

func ManagerTryGetResumableCheckpoint(session core.Session) *core.SessionExecutionCheckpoint {
	var checkpoint = session.ExecutionCheckpoint
	if checkpoint == nil || !strings.EqualFold(checkpoint.Kind, "tool_batch") || !strings.EqualFold(checkpoint.State, "ready_to_resume") || checkpoint.PersistedAtUtc == nil {
		return nil
	}

	if len(session.History) != checkpoint.HistoryCount {
		return nil
	}

	var lastTurn *core.ChatTurn = nil
	if len(session.History) > 0 {
		lastTurn = &session.History[len(session.History)-1]
	}
	if lastTurn != nil && (lastTurn.Content != "[tool_use]" || len(lastTurn.ToolCalls) == 0) {
		return nil
	}

	return checkpoint
}

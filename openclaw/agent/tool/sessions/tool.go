package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type SessionsTool struct {
	sessionManager  *core.SessionManager
	pipelineChannel chan<- core.InboundMessage
}

func New(sessionManager *core.SessionManager, pipelineChannel chan<- core.InboundMessage) *SessionsTool {
	return &SessionsTool{sessionManager: sessionManager, pipelineChannel: pipelineChannel}
}

func (a *SessionsTool) Name() string {
	return "sessions"
}

func (a *SessionsTool) Description() string {
	return "Lists active OpenClaw sessions, reads their history, or sends a cross-session message to another sub-agent or user channel."
}

func (a *SessionsTool) ParameterSchema() string {
	return `
	{
      "type": "object",
      "properties": {
        "action": {
          "type": "string",
          "enum": ["list", "history", "send"],
          "description": "The action to perform: list active sessions, get history of a session, or send a message."
        },
        "sessionId": { "type": "string", "description": "Required for history or send." },
        "message": { "type": "string", "description": "Required for send." },
        "limit": { "type": "integer", "description": "Max history lines to return (default: 50)." }
      },
      "required": ["action"]
    }
`
}

type SessionModel struct {
	Action    string `json:"action"`
	SessionId string `json:"sessionId"`
	Message   string `json:"message"`
	Limit     int    `json:"limit"`
}

func (a *SessionsTool) handleSend(ctx context.Context, targetSessionId, message string) string {
	targetContext, err := a.sessionManager.Load(ctx, targetSessionId)
	if err != nil {
		return err.Error()
	}
	if targetContext == nil {
		return fmt.Sprintf("Error: Target session '%s' does not exist.", targetSessionId)
	}

	var msg = core.InboundMessage{
		SessionId: targetContext.Id,
		ChannelId: targetContext.ChannelId,
		SenderId:  targetContext.SenderId,
		Text:      message,
	}

	select {
	case a.pipelineChannel <- msg:
		return fmt.Sprintf("Message queued for delivery to session %s.", targetSessionId)
	case <-ctx.Done():
		return ctx.Err().Error()
	}
}

func (a *SessionsTool) handleHistory(ctx context.Context, sessionId string, limit int) string {
	session, err := a.sessionManager.Load(ctx, sessionId)
	if err != nil {
		return err.Error()
	}

	if session == nil {
		return fmt.Sprintf("Error: Target session '%s' does not exist.", sessionId)
	}

	count := len(session.History)
	if count == 0 {
		return "Session history is currently empty."
	}

	recent := session.History
	if count > limit {
		recent = []core.ChatTurn{}
		for i := count - limit; i < count; i++ {
			recent = append(recent, session.History[i])
		}
	}

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "Last %d turns for session %s:\n", len(recent), sessionId)
	for _, turn := range recent {
		if turn.Content == "[tool_use]" {
			continue
		}
		fmt.Fprintf(&sb, "[%s] %s: %s\n", turn.Timestamp, turn.Role, turn.Content)
	}
	return sb.String()
}

func (a *SessionsTool) handleList(ctx context.Context) string {
	active, err := a.sessionManager.ListActive(ctx)
	if err != nil {
		return err.Error()
	}
	if len(active) == 0 {
		return "No active sessions found."
	}

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "Total Active Sessions: %d\n", len(active))
	for _, session := range active {
		fmt.Fprintf(&sb, "- ID: %s, Channel: %s, Sender: %s, State: %d\n", session.Id, session.ChannelId, session.SenderId, session.State)
	}
	return sb.String()
}

func (a *SessionsTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model SessionModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	switch model.Action {
	case "list":
		return a.handleList(ctx)
	case "history":
		return a.handleHistory(ctx, model.SessionId, model.Limit)
	case "send":
		return a.handleSend(ctx, model.SessionId, model.Message)
	default:
		return "Error: Unknown action. Valid actions are 'list', 'history', 'send'."
	}
}

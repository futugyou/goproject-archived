package sessions

import (
	"context"
	"encoding/json"
	"errors"
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

func (a *SessionsTool) handleSend(ctx context.Context, targetSessionId, message string) (string, error) {
	targetContext, err := a.sessionManager.Load(ctx, targetSessionId)
	if err != nil {
		return "", err
	}
	if targetContext == nil {
		return "", fmt.Errorf("Error: Target session '%s' does not exist.", targetSessionId)
	}

	var msg = core.InboundMessage{
		SessionId: targetContext.Id,
		ChannelId: targetContext.ChannelId,
		SenderId:  targetContext.SenderId,
		Text:      message,
	}

	select {
	case a.pipelineChannel <- msg:
		return fmt.Sprintf("Message queued for delivery to session %s.", targetSessionId), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (a *SessionsTool) handleHistory(ctx context.Context, sessionId string, limit int) (string, error) {
	session, err := a.sessionManager.Load(ctx, sessionId)
	if err != nil {
		return "", err
	}

	if session == nil {
		return "", fmt.Errorf("Error: Target session '%s' does not exist.", sessionId)
	}

	count := len(session.History)
	if count == 0 {
		return "", errors.New("Session history is currently empty.")
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
	return sb.String(), nil
}

func (a *SessionsTool) handleList(ctx context.Context) (string, error) {
	active, err := a.sessionManager.ListActive(ctx)
	if err != nil {
		return "", err
	}
	if len(active) == 0 {
		return "", errors.New("No active sessions found.")
	}

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "Total Active Sessions: %d\n", len(active))
	for _, session := range active {
		fmt.Fprintf(&sb, "- ID: %s, Channel: %s, Sender: %s, State: %d\n", session.Id, session.ChannelId, session.SenderId, session.State)
	}
	return sb.String(), nil
}

func (a *SessionsTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if argumentsJson == "" {
		return "", errors.New("Error: arguments payload is empty.")
	}

	var model SessionModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return "", err
	}

	switch model.Action {
	case "list":
		return a.handleList(ctx)
	case "history":
		return a.handleHistory(ctx, model.SessionId, model.Limit)
	case "send":
		return a.handleSend(ctx, model.SessionId, model.Message)
	default:
		return "", errors.New("Error: Unknown action. Valid actions are 'list', 'history', 'send'.")
	}
}

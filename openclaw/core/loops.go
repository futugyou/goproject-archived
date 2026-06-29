package core

import (
	"context"
	"regexp"
	"strings"
)

type AgentLoopRequestPayload struct {
	SessionId string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

type IAgentLoopDispatcher interface {
	Dispatch(ctx context.Context, sessionId, prompt string) bool
}

type ILoopControlService interface {
	SignalComplete(ctx context.Context, sessionId string) error
}
type LoopAction uint8

const (
	LoopAction_Schedule LoopAction = iota
	LoopAction_Cancel
	LoopAction_Status
	LoopAction_Invalid
)

type LoopCommand struct {
	Action   LoopAction `json:"action"`
	Interval string     `json:"interval"`
	Prompt   string     `json:"prompt"`
}

type LoopCommandParser struct{}

const (
	CancelCommand string = "cancel"
	StatusCommand string = "status"
)

var LoopCommandRegex = regexp.MustCompile(`(?i)^/loop\s+(?P<value>\d+)\s*(?P<unit>s|m|h)\s+(?P<prompt>.+)$`)
var (
	valueIdx  = LoopCommandRegex.SubexpIndex("value")
	unitIdx   = LoopCommandRegex.SubexpIndex("unit")
	promptIdx = LoopCommandRegex.SubexpIndex("prompt")
)

func (l *LoopCommandParser) TryParse(text string) *LoopCommand {
	if isBlank(text) || !strings.HasPrefix(text, "/loop") {
		return nil
	}

	var trimmed = strings.TrimSpace(text)

	if trimmed == "/loop cancel" || trimmed == "/loop stop" {
		return &LoopCommand{
			Action: LoopAction_Cancel,
		}
	}

	// /loop status
	if trimmed == "/loop status" {
		return &LoopCommand{
			Action: LoopAction_Status,
		}
	}

	// /loop <value><unit> <prompt>
	match := LoopCommandRegex.FindStringSubmatch(trimmed)
	if match == nil {
		return &LoopCommand{
			Action: LoopAction_Invalid,
		}
	}
	interval := match[valueIdx] + match[unitIdx]
	prompt := strings.TrimSpace(match[promptIdx])

	return &LoopCommand{
		Action:   LoopAction_Schedule,
		Interval: interval,
		Prompt:   prompt,
	}
}

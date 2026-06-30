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

type LoopTerminationDetector struct {
	loopControl ILoopControlService
}

var TerminationKeywords = map[string]struct{}{
	"LOOP_TERMINATE": {},
	"DONE":           {},
	"WORK_COMPLETE":  {},
}

func NewLoopTerminationDetector(loopControl ILoopControlService) *LoopTerminationDetector {
	return &LoopTerminationDetector{
		loopControl: loopControl,
	}
}

func (l *LoopTerminationDetector) isKeywordCharacter(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}

func (l *LoopTerminationDetector) cntainsWholeKeyword(text, keyword string) bool {
	if len(keyword) == 0 {
		return false
	}

	lowerText := strings.ToLower(text)
	lowerKeyword := strings.ToLower(keyword)

	startIndex := 0
	textLen := len(lowerText)
	keywordLen := len(lowerKeyword)

	for startIndex < textLen {
		relIndex := strings.Index(lowerText[startIndex:], lowerKeyword)
		if relIndex == -1 {
			return false
		}

		// 算出在原字符串中的绝对字节索引
		index := startIndex + relIndex

		// 2. 检查前边界
		before := index == 0 || !l.isKeywordCharacter(lowerText[index-1])

		// 3. 检查后边界
		afterIndex := index + keywordLen
		after := afterIndex == textLen || !l.isKeywordCharacter(lowerText[afterIndex])

		// 4. 两者都符合，说明是独立单词
		if before && after {
			return true
		}

		startIndex = index + 1
	}

	return false
}

func (l *LoopTerminationDetector) OnToolComplete(ctx context.Context, sessionId string) error {
	return l.loopControl.SignalComplete(ctx, sessionId)
}

func (l *LoopTerminationDetector) ScanText(ctx context.Context, sessionId, text string) bool {
	if isBlank(text) {
		return false
	}

	for keyword := range TerminationKeywords {
		if !l.cntainsWholeKeyword(text, keyword) {
			continue
		}

		if err := l.loopControl.SignalComplete(ctx, sessionId); err != nil {
			return false
		}
		return true
	}

	return false
}

package core

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type GoalHistoryRecord struct {
	Timestamp         string `json:"timestamp"`
	SessionId         string `json:"session_id"`
	Objective         string `json:"objective"`
	Status            string `json:"status"`
	TokenBudget       int64  `json:"token_budget"`
	TokensUsed        int64  `json:"tokens_used"`
	ContinuationCount int    `json:"continuation_count"`
	CreatedAt         string `json:"created_at"`
}

type GoalStatus uint8

const (
	GoalStatus_Active GoalStatus = iota
	GoalStatus_Paused
	GoalStatus_Blocked
	GoalStatus_BudgetLimited
	GoalStatus_UsageLimited
	GoalStatus_Complete
)

func (g GoalStatus) IsPursuable() bool {
	return g == GoalStatus_Active
}

func (g GoalStatus) IsTerminal() bool {
	return g == GoalStatus_Complete
}

func (g GoalStatus) ToDisplayName() string {
	switch g {
	case GoalStatus_Active:
		return "Activ"
	case GoalStatus_Paused:
		return "Paused"
	case GoalStatus_Blocked:
		return "Blocked"
	case GoalStatus_BudgetLimited:
		return "BudgetLimited"
	case GoalStatus_UsageLimited:
		return "UsageLimited"
	case GoalStatus_Complete:
		return "Complete"
	default:
		return "Unknown"
	}
}

type SessionGoal struct {
	SessionId               string     `json:"session_id"`
	Objective               string     `json:"objective"`
	Status                  GoalStatus `json:"status"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	TokenBudget             int64      `json:"token_budget"`
	TokensUsed              int64      `json:"tokens_used"`
	ContinuationCount       int        `json:"continuation_count"`
	RecentTurnHashes        []string   `json:"recent_turn_hashes"`
	ConsecutiveBlockerCount int        `json:"consecutive_blocker_count"`
	LastBlockerHash         string     `json:"last_blocker_hash"`
	StatusNote              string     `json:"status_note"`
	TokensAtStart           int64      `json:"tokens_at_start"`
	mu                      sync.Mutex `json:"-" gorm:"-"`
}

func (s *SessionGoal) IsBudgetExceeded() bool {
	return s.TokenBudget > 0 && s.TokenBudget >= s.TokensUsed
}

func (s *SessionGoal) RemainingBudget() int64 {
	if s.TokenBudget > 0 {
		return max(0, s.TokenBudget-s.TokensUsed)
	}
	return math.MaxInt64
}

func (s *SessionGoal) FormatGoalFooterLine() string {
	if s == nil {
		return ""
	}
	switch s.Status {
	case GoalStatus_Active:
		if s.TokenBudget > 0 {
			return fmt.Sprintf("Pursuing goal (%d/%d)", s.TokensUsed, s.TokenBudget)
		}
		return fmt.Sprintf("Pursuing goal: %s", Truncate(s.Objective, 40))
	case GoalStatus_Paused:
		return "Goal paused (/goal resume)"
	case GoalStatus_Blocked:
		return "Goal blocked (/goal resume)"
	case GoalStatus_BudgetLimited:
		return fmt.Sprintf("Goal unmet (%d/%d)", s.TokensUsed, s.TokenBudget)
	case GoalStatus_UsageLimited:
		return "Goal hit usage limits (/goal resume)"
	case GoalStatus_Complete:
		return fmt.Sprintf("Goal achieved (%d)", s.TokensUsed)
	default:
		return ""
	}
}

func (s *SessionGoal) FormatGoalProgressBar() string {
	if s == nil || s.TokenBudget < 0 {
		return ""
	}

	used := s.TokensUsed
	if used < 0 {
		used = 0
	} else if used > s.TokenBudget {
		used = s.TokenBudget
	}

	const barWidth = 20

	filled := int((used * barWidth) / s.TokenBudget)

	pctInt := (used * 100) / s.TokenBudget

	var sb strings.Builder
	sb.Grow(barWidth + 2)

	sb.WriteByte('[')
	for range filled {
		sb.WriteByte('=')
	}
	if filled < barWidth {
		sb.WriteByte('>')
		for i := filled + 1; i < barWidth; i++ {
			sb.WriteByte(' ')
		}
	}
	sb.WriteByte(']')

	return fmt.Sprintf("%s %d%% (%d/%d)", sb.String(), pctInt, s.TokensUsed, s.TokenBudget)
}

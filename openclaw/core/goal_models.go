package core

import (
	"math"
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
	GoalStatus_Activ GoalStatus = iota
	GoalStatus_Paused
	GoalStatus_Blocked
	GoalStatus_BudgetLimited
	GoalStatus_UsageLimited
	GoalStatus_Complete
)

func (g GoalStatus) IsPursuable() bool {
	return g == GoalStatus_Activ
}

func (g GoalStatus) IsTerminal() bool {
	return g == GoalStatus_Complete
}

func (g GoalStatus) ToDisplayName() string {
	switch g {
	case GoalStatus_Activ:
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

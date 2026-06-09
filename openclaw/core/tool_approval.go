package core

import "time"

type ToolApprovalRequest struct {
	ApprovalId string
	SessionId  string
	ChannelId  string
	SenderId   string
	ToolName   string
	Arguments  string
	Action     string
	IsMutation bool
	Summary    string
	CreatedAt  time.Time
}

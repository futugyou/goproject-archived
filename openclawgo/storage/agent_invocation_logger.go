package storage

import (
	"context"

	"gorm.io/gorm"
)

type AgentInvocationLogger struct {
	db *gorm.DB
}

func NewAgentInvocationLogger(db *gorm.DB) *AgentInvocationLogger {
	return &AgentInvocationLogger{
		db: db,
	}
}

func (p *AgentInvocationLogger) Record(ctx context.Context, entry *AgentInvocationLog) error {
	return gorm.G[AgentInvocationLog](p.db).Create(ctx, entry)
}

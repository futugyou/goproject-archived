package storage

import "context"

type IConversationStore interface {
	CreateSession(ctx context.Context, title string) (*ChatSession, error)
	GetSession(ctx context.Context, sessionId string) (*ChatSession, error)
	ListSessions(ctx context.Context) ([]ChatSession, error)
	DeleteSession(ctx context.Context, sessionId string) error
	DeleteSessionsBulk(ctx context.Context, sessionIds []string) (int, error)
	UpdateSessionTitle(ctx context.Context, sessionId, title string) (*ChatSession, error)
	AddMessage(ctx context.Context, sessionId, role, content, name, toolCallId, toolCallsJson string) (*ChatMessageEntity, error)
	GetMessages(ctx context.Context, sessionId string) ([]ChatMessageEntity, error)
	PruneOldMessages(ctx context.Context, sessionId string, keepRecentCount int) error
}

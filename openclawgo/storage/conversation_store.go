package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

var _ IConversationStore = (*ConversationStore)(nil)

type ConversationStore struct {
	db *gorm.DB
}

func NewConversationStore(db *gorm.DB) *ConversationStore {
	return &ConversationStore{
		db: db,
	}
}

// AddMessage implements [IConversationStore].
func (c *ConversationStore) AddMessage(ctx context.Context, sessionId string, role string, content string, name string, toolCallId string, toolCallsJson string) (*ChatMessageEntity, error) {
	chatMessageEntity := &ChatMessageEntity{
		SessionId:     sessionId,
		Role:          role,
		Content:       content,
		Name:          name,
		ToolCallId:    toolCallId,
		ToolCallsJson: toolCallsJson,
	}

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session *ChatSession
		err := tx.Where(ChatSession{Id: sessionId}).
			Attrs(ChatSession{Id: sessionId, Title: "New Chat"}).
			Assign(ChatSession{UpdatedAt: time.Now().UTC()}).
			FirstOrCreate(session).Error
		if err != nil {
			return err
		}

		maxOrder := 1
		err = tx.Model(&ChatMessageEntity{}).
			Where("session_id = ?", sessionId).
			Select("max(order_index)").
			First(&maxOrder).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else {
			maxOrder = maxOrder + 1
		}
		chatMessageEntity.OrderIndex = maxOrder + 1
		return tx.Save(chatMessageEntity).Error
	})

	return chatMessageEntity, err
}

// CreateSession implements [IConversationStore].
func (c *ConversationStore) CreateSession(ctx context.Context, title string) (*ChatSession, error) {
	if len(title) == 0 {
		title = "New Chat"
	}
	session := &ChatSession{Id: uuid.NewString(), Title: title}
	err := c.db.Save(session).Error
	return session, err
}

// DeleteSession implements [IConversationStore].
func (c *ConversationStore) DeleteSession(ctx context.Context, sessionId string) error {
	_, err := gorm.G[ChatSession](c.db).Where("id = ?", sessionId).Delete(ctx)
	return err
}

// DeleteSessionsBulk implements [IConversationStore].
func (c *ConversationStore) DeleteSessionsBulk(ctx context.Context, sessionIds []string) (int, error) {
	return gorm.G[ChatSession](c.db).Where("id in ?", sessionIds).Delete(ctx)
}

// GetMessages implements [IConversationStore].
func (c *ConversationStore) GetMessages(ctx context.Context, sessionId string) ([]ChatMessageEntity, error) {
	return gorm.G[ChatMessageEntity](c.db).Where("session_id = ?", sessionId).Order("order_index").Find(ctx)
}

// GetSession implements [IConversationStore].
func (c *ConversationStore) GetSession(ctx context.Context, sessionId string) (*ChatSession, error) {
	var session ChatSession
	err := c.db.Model(&ChatSession{}).Where("id = ", sessionId).Preload("Messages").First(&session).Error
	return &session, err
}

// ListSessions implements [IConversationStore].
func (c *ConversationStore) ListSessions(ctx context.Context) ([]ChatSession, error) {
	var session []ChatSession
	err := c.db.Model(&ChatSession{}).Preload("Messages").Find(&session).Error
	return session, err
}

// PruneOldMessages implements [IConversationStore].
func (c *ConversationStore) PruneOldMessages(ctx context.Context, sessionId string, keepRecentCount int) error {
	var summary []SessionSummary
	if err := c.db.Model(&SessionSummary{}).Where("session_id = ", sessionId).Find(&summary).Error; err != nil {
		return err
	}

	if len(summary) == 0 {
		return nil
	}

	var count int64

	// Counting users with specific names
	if err := c.db.Model(&ChatMessageEntity{}).Where("session_id = ?", sessionId).Count(&count).Error; err != nil {
		return err
	}

	if count <= int64(keepRecentCount) {
		return nil
	}

	var cutoffOrderIndex int

	if err := c.db.Model(&ChatMessageEntity{}).
		Where("session_id = ?", sessionId).
		Offset(keepRecentCount).
		Order("order_index").
		Select("order_index").First(&cutoffOrderIndex).Error; err != nil {
		return err
	}

	if cutoffOrderIndex == 0 {
		return nil
	}

	return c.db.Model(&ChatMessageEntity{}).
		Where("session_id = ? AND order_index <=", sessionId, cutoffOrderIndex).
		Delete(ctx).Error
}

// UpdateSessionTitle implements [IConversationStore].
func (c *ConversationStore) UpdateSessionTitle(ctx context.Context, sessionId string, title string) (*ChatSession, error) {
	var updatedSession ChatSession
	err := c.db.WithContext(ctx).
		Model(&updatedSession).
		Where("id = ?", sessionId).
		Clauses(clause.Returning{}).
		Updates(map[string]any{"title": title, "updated_at": time.Now().UTC()}).Error

	if err != nil {
		return nil, err
	}
	return &updatedSession, nil
}

package storage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type IToolTestRecordStore interface {
	Get(ctx context.Context, toolName string) (*ToolTestRecord, error)
	List(ctx context.Context) ([]ToolTestRecord, error)
	Save(ctx context.Context, toolName string, succeeded bool, message, mode string) error
}

var _ IToolTestRecordStore = (*ToolTestRecordStore)(nil)

type ToolTestRecordStore struct {
	db *gorm.DB
}

func NewToolTestRecordStore(db *gorm.DB) *ToolTestRecordStore {
	return &ToolTestRecordStore{db: db}
}

// Get implements [IToolTestRecordStore].
func (t *ToolTestRecordStore) Get(ctx context.Context, toolName string) (*ToolTestRecord, error) {
	d, err := gorm.G[ToolTestRecord](t.db).Where("name = ?", toolName).First(ctx)
	if err != nil {
		return nil, err
	}
	if len(d.Name) == 0 {
		return nil, errors.New("no data found")
	}
	return &d, nil
}

// List implements [IToolTestRecordStore].
func (t *ToolTestRecordStore) List(ctx context.Context) ([]ToolTestRecord, error) {
	return gorm.G[ToolTestRecord](t.db).Find(ctx)
}

// Save implements [IToolTestRecordStore].
func (t *ToolTestRecordStore) Save(ctx context.Context, toolName string, succeeded bool, message string, mode string) error {
	if len(message) > 1000 {
		message = message[:1000]
	}
	now := time.Now().UTC()
	data := &ToolTestRecord{}
	return t.db.Where(ToolTestRecord{Name: toolName}).
		Assign(map[string]any{
			"name":                toolName,
			"last_test_succeeded": succeeded,
			"last_tested_at":      now,
			"last_test_error":     message,
			"last_test_mode":      mode,
		}).
		FirstOrCreate(data).Error
}

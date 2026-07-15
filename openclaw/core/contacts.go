package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Contact struct {
	PhoneE164   string    `json:"phone_e164"`
	DisplayName string    `json:"display_name"` // 使用指针支持可空类型 string?
	DoNotText   bool      `json:"do_not_text"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// DefaultContact 初始化并返回带有默认值的 Contact
func DefaultContact() Contact {
	now := time.Now().UTC()
	return Contact{
		CreatedAt:  now,
		LastSeenAt: now,
	}
}

type ContactStoreState struct {
	ContactsByPhone map[string]Contact `json:"contacts_by_phone"`
}

// DefaultContactStoreState 初始化并返回带有默认值的 ContactStoreState
func DefaultContactStoreState() ContactStoreState {
	return ContactStoreState{
		ContactsByPhone: make(map[string]Contact),
	}
}

// IContactStore 接口定义
type IContactStore interface {
	Touch(ctx context.Context, phoneE164 string) (Contact, error)
	Get(ctx context.Context, phoneE164 string) (*Contact, error)
	SetDoNotText(ctx context.Context, phoneE164 string, doNotText bool) error
}

var _ IContactStore = (*FileContactStore)(nil)

type FileContactStore struct {
	path string
	gate sync.Mutex
}

// NewFileContactStore 构造函数
func NewFileContactStore(basePath string) (*FileContactStore, error) {
	err := os.MkdirAll(basePath, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &FileContactStore{
		path: filepath.Join(basePath, "contacts.json"),
	}, nil
}

// Get 获取指定电话的联系人
func (f *FileContactStore) Get(ctx context.Context, phoneE164 string) (*Contact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.gate.Lock()
	defer f.gate.Unlock()

	state, err := f.loadUnsafe(ctx)
	if err != nil {
		return nil, err
	}

	if contact, exists := state.ContactsByPhone[phoneE164]; exists {
		return &contact, nil
	}

	return nil, nil
}

// Touch 触碰/更新联系人的活跃时间
func (f *FileContactStore) Touch(ctx context.Context, phoneE164 string) (Contact, error) {
	if err := ctx.Err(); err != nil {
		return Contact{}, err
	}

	f.gate.Lock()
	defer f.gate.Unlock()

	state, err := f.loadUnsafe(ctx)
	if err != nil {
		return Contact{}, err
	}

	contact, exists := state.ContactsByPhone[phoneE164]
	if !exists {
		contact = DefaultContact()
		contact.PhoneE164 = phoneE164
	}

	contact.LastSeenAt = time.Now().UTC()
	state.ContactsByPhone[phoneE164] = contact

	if err := f.saveUnsafe(ctx, state); err != nil {
		return Contact{}, err
	}

	return contact, nil
}

// SetDoNotText 设置免打扰状态
func (f *FileContactStore) SetDoNotText(ctx context.Context, phoneE164 string, doNotText bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.gate.Lock()
	defer f.gate.Unlock()

	state, err := f.loadUnsafe(ctx)
	if err != nil {
		return err
	}

	contact, exists := state.ContactsByPhone[phoneE164]
	if !exists {
		contact = DefaultContact()
		contact.PhoneE164 = phoneE164
	}

	contact.DoNotText = doNotText
	contact.LastSeenAt = time.Now().UTC()
	state.ContactsByPhone[phoneE164] = contact

	return f.saveUnsafe(ctx, state)
}

// loadUnsafe 内部加载方法（无锁保护，需外层加锁）
func (f *FileContactStore) loadUnsafe(ctx context.Context) (ContactStoreState, error) {
	if err := ctx.Err(); err != nil {
		return DefaultContactStoreState(), err
	}

	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultContactStoreState(), nil
		}
		return DefaultContactStoreState(), err
	}
	defer file.Close()

	var state ContactStoreState = DefaultContactStoreState()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return DefaultContactStoreState(), nil
	}

	return state, nil
}

// saveUnsafe 内部保存方法（无锁保护，需外层加锁）
func (f *FileContactStore) saveUnsafe(ctx context.Context, state ContactStoreState) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tmp := f.path + ".tmp"

	// 使用闭包确保临时文件能提前 Close，以便后续执行重命名
	err := func() error {
		file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		if err := encoder.Encode(state); err != nil {
			return err
		}

		return file.Sync()
	}()

	defer func() {
		// Best-effort 尽力而为的清理临时文件
		_ = os.Remove(tmp)
	}()

	if err != nil {
		return err
	}

	if err := os.Rename(tmp, f.path); err != nil {
		return err
	}

	return nil
}

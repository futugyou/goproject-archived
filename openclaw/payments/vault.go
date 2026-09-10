package payments

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type IPaymentSecretVault interface {
	Store(ctx context.Context, secret PaymentSecret, ttl time.Duration, retrieveOnce bool) (string, error)
	TryRetrieve(ctx context.Context, handleId, purpose string) (*PaymentSecret, error)
	Revoke(ctx context.Context, handleId, reason string) error
}

var _ IPaymentSecretVault = (*InMemoryPaymentSecretVault)(nil)

type VaultEntry struct {
	Secret       PaymentSecret
	ExpiresAtUtc time.Time
	RetrieveOnce bool
}

type InMemoryPaymentSecretVault struct {
	mu      sync.Mutex
	entries map[string]*VaultEntry
}

func NewInMemoryPaymentSecretVault() *InMemoryPaymentSecretVault {
	return &InMemoryPaymentSecretVault{
		entries: map[string]*VaultEntry{},
	}
}

func (i *InMemoryPaymentSecretVault) Revoke(ctx context.Context, handleId string, reason string) error {
	if handleId == "" {
		return nil
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if entry, ok := i.entries[handleId]; ok {
		entry.Secret.Clear()
	}

	return nil
}

func (i *InMemoryPaymentSecretVault) cleanupExpired(nowUtc time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for k, v := range i.entries {
		if v.ExpiresAtUtc.After(nowUtc) {
			continue
		}

		delete(i.entries, k)
		v.Secret.Clear()
	}
}

func (i *InMemoryPaymentSecretVault) Store(ctx context.Context, secret PaymentSecret, ttl time.Duration, retrieveOnce bool) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	now := time.Now().UTC()
	i.cleanupExpired(now)
	if ttl < 0 {
		ttl = time.Duration(5) * time.Minute
	}

	var expiresAt = now.Add(ttl)
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries[secret.HandleId()] = &VaultEntry{
		Secret:       secret,
		ExpiresAtUtc: expiresAt,
		RetrieveOnce: retrieveOnce,
	}

	return secret.HandleId(), nil
}

func (i *InMemoryPaymentSecretVault) TryRetrieve(ctx context.Context, handleId string, purpose string) (*PaymentSecret, error) {
	if handleId == "" {
		return nil, errors.New("handleId can not be empty")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	entry, ok := i.entries[handleId]
	if !ok {
		entry.Secret.Clear()
		return nil, fmt.Errorf("%s was not exsited", handleId)
	}

	if entry.ExpiresAtUtc.Before(time.Now().UTC()) {
		delete(i.entries, handleId)
		entry.Secret.Clear()
		return nil, fmt.Errorf("%s was expired", handleId)
	}

	if entry.RetrieveOnce {
		delete(i.entries, handleId)
	}

	return &entry.Secret, nil
}

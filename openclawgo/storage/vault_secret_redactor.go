package storage

import (
	"slices"
	"strings"
	"sync"
	"time"
)

type IVaultSecretRedactor interface {
	TrackResolvedValue(value string) error
	Redact(content string) (string, error)
}

var _ IVaultSecretRedactor = (*VaultSecretRedactor)(nil)

const (
	MaxTrackedValues = 1024
	Redacted         = "[vault-secret-redacted]"
	Retention        = 30 * time.Second
)

type VaultSecretRedactor struct {
	cache  sync.RWMutex
	values map[string]time.Time
}

// Redact implements [IVaultSecretRedactor].
func (v *VaultSecretRedactor) Redact(content string) (string, error) {
	if len(content) == 0 {
		return "", nil
	}
	v.prune(time.Now().UTC())
	s := []redactorMap{}
	for key, value := range v.values {
		s = append(s, redactorMap{key: key, value: value})
	}

	slices.SortFunc(s, func(a, b redactorMap) int {
		return len(b.key) - len(a.key)
	})

	for _, v := range s {
		if len(v.key) > 0 {
			content = strings.ReplaceAll(content, v.key, Redacted)
		}
	}

	return content, nil
}

type redactorMap struct {
	key   string
	value time.Time
}

// TrackResolvedValue implements [IVaultSecretRedactor].
func (v *VaultSecretRedactor) TrackResolvedValue(value string) error {
	if len(value) == 0 {
		return nil
	}

	now := time.Now().UTC()
	v.values[value] = now.Add(Retention)
	v.prune(now)
	return nil
}

func (v *VaultSecretRedactor) prune(now time.Time) {
	s := []redactorMap{}
	for key, value := range v.values {
		if value.Before(now) {
			delete(v.values, key)
		} else {
			s = append(s, redactorMap{key: key, value: value})
		}
	}

	if len(v.values) <= MaxTrackedValues {
		return
	}

	slices.SortFunc(s, func(a, b redactorMap) int {
		return a.value.Compare(b.value)
	})

	for i := 0; i < len(v.values)-MaxTrackedValues; i++ {
		delete(v.values, s[i].key)
	}
}

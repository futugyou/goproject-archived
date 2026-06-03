package storage

import (
	"context"
	"sync"
	"time"
)

type IVaultCacheInvalidator interface {
	Invalidate(name string) error
}

var _ IVaultCacheInvalidator = (*VaultConfigurationResolver)(nil)

const (
	VaultConfigTtl = 5 * time.Minute
)

type VaultConfigEntry struct {
	Value     string
	ExpiresAt time.Time
	Version   int64
}
type VaultConfigurationResolver struct {
	cache     map[string]VaultConfigEntry
	versions  map[string]int64
	ttl       time.Duration
	cacheLock sync.Mutex
}

func NewVaultConfigurationResolver() *VaultConfigurationResolver {
	return &VaultConfigurationResolver{
		cache:    map[string]VaultConfigEntry{},
		versions: map[string]int64{},
		ttl:      VaultConfigTtl,
	}
}

func (v *VaultConfigurationResolver) ResolveSecret(ctx context.Context, secretName string, vault IVault) (string, error) {
	for {
		now := time.Now().UTC()
		var version = v.getVersion(secretName)
		if cached, ok := v.cache[secretName]; ok && cached.Version == version && cached.ExpiresAt.After(now) {
			return cached.Value, nil
		}

		value, err := vault.Resolve(ctx, secretName, VaultCallerContext{
			CallerType: VaultCallerTypeConfiguration,
			CallerId:   "IConfiguration",
		})
		if err != nil {
			return "", err
		}
		var currentVersion = v.getVersion(secretName)

		if currentVersion != version {
			continue
		}

		v.cache[secretName] = VaultConfigEntry{
			Value:     value,
			ExpiresAt: now.Add(v.ttl),
			Version:   currentVersion,
		}
		return value, nil
	}
}

func (v *VaultConfigurationResolver) getVersion(name string) int64 {
	if d, ok := v.versions[name]; ok {
		return d
	}

	return 0
}

// Invalidate implements [IVaultCacheInvalidator].
func (v *VaultConfigurationResolver) Invalidate(name string) error {
	if len(name) == 0 {
		return nil
	}

	v.cacheLock.Lock()
	defer v.cacheLock.Unlock()
	if d, ok := v.versions[name]; ok {
		v.versions[name] = d + 1
	} else {
		v.versions[name] = 1
	}

	delete(v.cache, name)
	return nil
}

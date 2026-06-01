package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxPerFolderBytes = 5 * 1024 * 1024 * 1024  // 5 GB
	DefaultMaxTotalBytes     = 25 * 1024 * 1024 * 1024 // 25 GB
	WalkCacheTtl             = 30 * time.Second
)

type cacheSlot struct {
	bytes int64
	at    time.Time
}
type UserQuotaCheckResult struct {
	Allowed            bool
	FolderBytes        int64
	TotalBytes         int64
	AvailableDiskBytes int64
	DenyReason         string
}

type IUserFolderQuota interface {
	Check(ctx context.Context, folderName string, incomingBytes int64) (*UserQuotaCheckResult, error)
	InvalidateWalkCache(ctx context.Context, folderName string) error
}

var _ IUserFolderQuota = (*UserFolderQuota)(nil)

type UserFolderQuota struct {
	maxPerFolderBytes int64
	maxTotalBytes     int64
	clock             func() time.Time
	cacheMu           sync.RWMutex
	folderCache       map[string]cacheSlot
}

func NewUserFolderQuota(maxPerFolderBytes, maxTotalBytes int64, clock func() time.Time) (*UserFolderQuota, error) {
	if maxPerFolderBytes <= 0 {
		return nil, errors.New("maxPerFolderBytes must be greater than 0")
	}
	if maxTotalBytes <= 0 {
		return nil, errors.New("maxTotalBytes must be greater than 0")
	}
	if clock == nil {
		clock = time.Now
	}

	return &UserFolderQuota{
		maxPerFolderBytes: maxPerFolderBytes,
		maxTotalBytes:     maxTotalBytes,
		clock:             clock,
		folderCache:       make(map[string]cacheSlot),
	}, nil
}

func (q *UserFolderQuota) Check(ctx context.Context, folderName string, incomingBytes int64) (*UserQuotaCheckResult, error) {
	if strings.TrimSpace(folderName) == "" {
		return &UserQuotaCheckResult{}, errors.New("folder name must be non-empty")
	}
	if incomingBytes < 0 {
		return &UserQuotaCheckResult{}, errors.New("incomingBytes must be non-negative")
	}

	storageRoot := "./openclawnet"
	folderBytes := q.getCurrentFolderBytes(storageRoot, folderName)
	if folderBytes+incomingBytes > q.maxPerFolderBytes {
		return &UserQuotaCheckResult{
			Allowed:            false,
			FolderBytes:        folderBytes,
			TotalBytes:         -1,
			AvailableDiskBytes: tryGetAvailableDiskBytes(storageRoot),
			DenyReason:         fmt.Sprintf("per-folder limit (%d bytes) exceeded", q.maxPerFolderBytes),
		}, nil
	}

	totalBytes := q.getCurrentTotalBytes(storageRoot)
	if totalBytes+incomingBytes > q.maxTotalBytes {
		return &UserQuotaCheckResult{
			Allowed:            false,
			FolderBytes:        folderBytes,
			TotalBytes:         totalBytes,
			AvailableDiskBytes: tryGetAvailableDiskBytes(storageRoot),
			DenyReason:         fmt.Sprintf("total quota (%d bytes) exceeded", q.maxTotalBytes),
		}, nil
	}

	availableDisk := tryGetAvailableDiskBytes(storageRoot)
	if availableDisk >= 0 && incomingBytes > availableDisk {
		return &UserQuotaCheckResult{
			Allowed:            false,
			FolderBytes:        folderBytes,
			TotalBytes:         totalBytes,
			AvailableDiskBytes: availableDisk,
			DenyReason:         "insufficient disk space",
		}, nil
	}

	return &UserQuotaCheckResult{
		Allowed:            true,
		FolderBytes:        folderBytes,
		TotalBytes:         totalBytes,
		AvailableDiskBytes: availableDisk,
	}, nil
}

func (q *UserFolderQuota) InvalidateWalkCache(ctx context.Context, folderName string) error {
	if strings.TrimSpace(folderName) == "" {
		return nil
	}
	q.cacheMu.Lock()
	delete(q.folderCache, strings.ToLower(folderName))
	q.cacheMu.Unlock()
	return nil
}

func (q *UserFolderQuota) getCurrentFolderBytes(storageRoot, folderName string) int64 {
	now := q.clock()
	cacheKey := strings.ToLower(folderName)

	q.cacheMu.RLock()
	slot, exists := q.folderCache[cacheKey]
	if exists && now.Sub(slot.at) < WalkCacheTtl {
		q.cacheMu.RUnlock()
		return slot.bytes
	}
	q.cacheMu.RUnlock()

	folderPath := filepath.Join(storageRoot, folderName)
	var bytes int64

	if _, err := os.Stat(folderPath); err == nil {
		err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				if info, err := d.Info(); err == nil {
					bytes += info.Size()
				}
			}
			return nil
		})

		if err != nil {
			bytes = 0
		}
	}

	q.cacheMu.Lock()
	q.folderCache[cacheKey] = cacheSlot{bytes: bytes, at: now}
	q.cacheMu.Unlock()

	return bytes
}

func (q *UserFolderQuota) getCurrentTotalBytes(storageRoot string) int64 {
	if _, err := os.Stat(storageRoot); os.IsNotExist(err) {
		return 0
	}

	entries, err := os.ReadDir(storageRoot)
	if err != nil {
		return 0
	}

	var total int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if _, excluded := ExcludedFolderNames[strings.ToLower(name)]; excluded {
			continue
		}

		total += q.getCurrentFolderBytes(storageRoot, name)
	}

	return total
}

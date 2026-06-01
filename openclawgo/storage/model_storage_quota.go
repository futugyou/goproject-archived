package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

type QuotaCheckResult struct {
	Allowed            bool
	CurrentTotalBytes  int64
	AvailableDiskBytes int64
	DenyReason         string
}

type IModelStorageQuota interface {
	Check(ctx context.Context, modelsRoot string, incomingBytes int64) (*QuotaCheckResult, error)
}

var _ IModelStorageQuota = (*QuotaChecker)(nil)

type QuotaChecker struct {
	MaxPerFileBytes int64
	MaxTotalBytes   int64
	cacheLock       sync.Mutex
	cachedRoot      string
	cachedTotal     int64
	cachedAt        time.Time
	CacheTTL        time.Duration
}

// Check implements [IModelStorageQuota].
func (q *QuotaChecker) Check(ctx context.Context, modelsRoot string, incomingBytes int64) (*QuotaCheckResult, error) {
	if modelsRoot == "" {
		return nil, errors.New("modelsRoot must be non-empty")
	}
	if incomingBytes < 0 {
		return nil, errors.New("incomingBytes cannot be negative")
	}

	if incomingBytes > q.MaxPerFileBytes {
		return &QuotaCheckResult{
			Allowed:            false,
			CurrentTotalBytes:  -1,
			AvailableDiskBytes: -1,
			DenyReason:         "per-file limit exceeded",
		}, nil
	}

	currentTotal, err := q.getCurrentTotalBytes(modelsRoot)
	if err != nil {
		return nil, err
	}

	if currentTotal+incomingBytes > q.MaxTotalBytes {
		availableDisk := tryGetAvailableDiskBytes(modelsRoot)
		return &QuotaCheckResult{
			Allowed:            false,
			CurrentTotalBytes:  currentTotal,
			AvailableDiskBytes: availableDisk,
			DenyReason:         "total quota exceeded",
		}, nil
	}

	availableDisk := tryGetAvailableDiskBytes(modelsRoot)
	if availableDisk >= 0 && incomingBytes > availableDisk {
		return &QuotaCheckResult{
			Allowed:            false,
			CurrentTotalBytes:  currentTotal,
			AvailableDiskBytes: availableDisk,
			DenyReason:         "insufficient disk space",
		}, nil
	}

	return &QuotaCheckResult{
		Allowed:            true,
		CurrentTotalBytes:  currentTotal,
		AvailableDiskBytes: availableDisk,
		DenyReason:         "",
	}, nil
}

func (q *QuotaChecker) InvalidateCache() {
	q.cacheLock.Lock()
	defer q.cacheLock.Unlock()
	q.cachedAt = time.Time{}
	q.cachedRoot = ""
	q.cachedTotal = 0
}

func (q *QuotaChecker) getCurrentTotalBytes(modelsRoot string) (int64, error) {
	now := time.Now()

	q.cacheLock.Lock()
	if q.cachedRoot == modelsRoot && now.Sub(q.cachedAt) < q.CacheTTL {
		total := q.cachedTotal
		q.cacheLock.Unlock()
		return total, nil
	}
	q.cacheLock.Unlock()

	var total int64 = 0
	err := filepath.Walk(modelsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	q.cacheLock.Lock()
	q.cachedRoot = modelsRoot
	q.cachedTotal = total
	q.cachedAt = now
	q.cacheLock.Unlock()

	return total, nil
}

func tryGetAvailableDiskBytes(path string) int64 {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return -1
	}
	usageStat, err := disk.Usage(absPath)
	if err != nil {
		return -1
	}
	return int64(usageStat.Free)
}

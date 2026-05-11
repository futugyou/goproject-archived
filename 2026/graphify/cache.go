package graphify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CacheEntry struct {
	FilePath       string
	ContentHash    string
	CachedAt       time.Time
	ResultFilePath string
}

type ICacheProviderT[T any] interface {
	Get(ctx context.Context, key string) (*T, error)
	Set(ctx context.Context, key string, value T) error
	Exists(ctx context.Context, key string) bool
	Invalidate(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

type SemanticCacheT[T any] struct {
	cacheDir   string
	indexFile  string
	index      map[string]CacheEntry
	indexLock  sync.RWMutex
	jsonIndent bool
}

var _ ICacheProviderT[CacheEntry] = (*SemanticCacheT[CacheEntry])(nil)

func NewSemanticCacheT[T any](projectRoot string) (*SemanticCacheT[T], error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	cacheDir := filepath.Join(projectRoot, ".graphify", "cache")
	indexFile := filepath.Join(cacheDir, "index.json")

	sc := &SemanticCacheT[T]{
		cacheDir:   cacheDir,
		indexFile:  indexFile,
		index:      make(map[string]CacheEntry),
		jsonIndent: true,
	}

	if err := sc.ensureCacheDir(); err != nil {
		return nil, err
	}

	if err := sc.loadIndex(context.Background()); err != nil {
		return nil, err
	}

	return sc, nil
}

func (sc *SemanticCacheT[T]) ensureCacheDir() error {
	if _, err := os.Stat(sc.cacheDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(sc.cacheDir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (sc *SemanticCacheT[T]) ComputeHash(ctx context.Context, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (sc *SemanticCacheT[T]) IsChanged(ctx context.Context, filePath string) (bool, error) {
	sc.indexLock.RLock()
	entry, ok := sc.index[filePath]
	sc.indexLock.RUnlock()

	if !ok {
		return true, nil
	}

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return true, nil
	}

	currentHash, err := sc.ComputeHash(ctx, filePath)
	if err != nil {
		return false, err
	}

	return currentHash != entry.ContentHash, nil
}

func (sc *SemanticCacheT[T]) Get(ctx context.Context, key string) (*T, error) {
	sc.indexLock.RLock()
	entry, ok := sc.index[key]
	sc.indexLock.RUnlock()
	if !ok {
		return nil, nil
	}

	changed, err := sc.IsChanged(ctx, key)
	if err != nil || changed {
		_ = sc.Invalidate(ctx, key)
		return nil, err
	}

	if _, err := os.Stat(entry.ResultFilePath); os.IsNotExist(err) {
		_ = sc.Invalidate(ctx, key)
		return nil, nil
	}

	data, err := os.ReadFile(entry.ResultFilePath)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		_ = sc.Invalidate(ctx, key)
		return nil, nil
	}

	return &result, nil
}

func (sc *SemanticCacheT[T]) Set(ctx context.Context, key string, value T) error {
	h := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(h[:])

	resultFilePath := filepath.Join(sc.cacheDir, hashStr+".json")

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(resultFilePath, data, 0600); err != nil {
		return err
	}

	entry := CacheEntry{
		FilePath:       key,
		ContentHash:    hashStr,
		CachedAt:       time.Now(),
		ResultFilePath: resultFilePath,
	}

	sc.indexLock.Lock()
	sc.index[key] = entry
	sc.indexLock.Unlock()
	return sc.saveIndex(ctx)
}

func (sc *SemanticCacheT[T]) GetCachedResult(ctx context.Context, filePath string, result interface{}) (bool, error) {
	sc.indexLock.RLock()
	entry, ok := sc.index[filePath]
	sc.indexLock.RUnlock()
	if !ok {
		return false, nil
	}

	changed, err := sc.IsChanged(ctx, filePath)
	if err != nil || changed {
		return false, err
	}

	data, err := os.ReadFile(entry.ResultFilePath)
	if err != nil {
		_ = sc.Invalidate(ctx, filePath)
		return false, err
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	if err := json.Unmarshal(data, result); err != nil {
		_ = sc.Invalidate(ctx, filePath)
		return false, err
	}

	return true, nil
}

func (sc *SemanticCacheT[T]) CacheResult(ctx context.Context, filePath string, result interface{}) error {
	hash, err := sc.ComputeHash(ctx, filePath)
	if err != nil {
		return err
	}

	resultFile := filepath.Join(sc.cacheDir, hash+".json")
	var data []byte
	if sc.jsonIndent {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.WriteFile(resultFile, data, 0600); err != nil {
		return err
	}

	entry := CacheEntry{
		FilePath:       filePath,
		ContentHash:    hash,
		CachedAt:       time.Now().UTC(),
		ResultFilePath: resultFile,
	}

	sc.indexLock.Lock()
	sc.index[filePath] = entry
	sc.indexLock.Unlock()

	return sc.saveIndex(ctx)
}

func (sc *SemanticCacheT[T]) Invalidate(ctx context.Context, key string) error {
	sc.indexLock.Lock()
	entry, ok := sc.index[key]
	if ok {
		delete(sc.index, key)
		_ = os.Remove(entry.ResultFilePath)
	}
	sc.indexLock.Unlock()

	return sc.saveIndex(ctx)
}

func (sc *SemanticCacheT[T]) Clear(ctx context.Context) error {
	sc.indexLock.Lock()
	defer sc.indexLock.Unlock()

	for _, entry := range sc.index {
		_ = os.Remove(entry.ResultFilePath)
	}
	sc.index = make(map[string]CacheEntry)

	return sc.saveIndex(ctx)
}

func (sc *SemanticCacheT[T]) loadIndex(ctx context.Context) error {
	if _, err := os.Stat(sc.indexFile); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	data, err := os.ReadFile(sc.indexFile)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var entries map[string]CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		sc.index = make(map[string]CacheEntry)
		return nil
	}

	sc.indexLock.Lock()
	sc.index = entries
	sc.indexLock.Unlock()
	return nil
}

func (sc *SemanticCacheT[T]) saveIndex(ctx context.Context) error {
	sc.indexLock.RLock()
	data, err := json.MarshalIndent(sc.index, "", "  ")
	sc.indexLock.RUnlock()
	if err != nil {
		return err
	}

	tmpFile := sc.indexFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, sc.indexFile)
}

func (sc *SemanticCacheT[T]) Exists(ctx context.Context, key string) bool {
	sc.indexLock.RLock()
	defer sc.indexLock.RUnlock()
	_, ok := sc.index[key]
	return ok
}

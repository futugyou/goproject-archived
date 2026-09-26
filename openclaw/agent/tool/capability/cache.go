package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
)

type CapabilityBindingCache struct {
	ttl      time.Duration
	capacity int

	entries    map[EntryKey]Entry
	mu         sync.Mutex
	generation int64
}

var DefaultTtl = 300 * time.Second

type Entry struct {
	Value      any
	At         time.Time
	Persistent bool
}

type EntryKey struct {
	Scope string
	Key   string
}

func NewCapabilityBindingCache(ttl time.Duration, capacity int) *CapabilityBindingCache {
	if ttl <= 0 {
		ttl = DefaultTtl
	}

	if capacity <= 0 {
		capacity = 1024
	}

	return &CapabilityBindingCache{
		ttl:      ttl,
		capacity: capacity,
		entries:  map[EntryKey]Entry{},
	}
}

func (c *CapabilityBindingCache) Invalidate(change core.CapabilityChange) {
	c.Clear()
}

func (c *CapabilityBindingCache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.entries)
}

func (c *CapabilityBindingCache) Generation() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.generation
}

func TryGetValue[T any](c *CapabilityBindingCache, scope, key string) (*T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryKey := EntryKey{Scope: scope, Key: key}
	if value, ok := c.entries[entryKey]; ok && (value.Persistent || time.Since(value.At) < c.ttl) {
		if entry, ok := value.Value.(T); ok {
			return &entry, ok
		}
	}

	delete(c.entries, entryKey)
	return nil, false
}

func (c *CapabilityBindingCache) Store(scope, key string, value any, generation int64, persistent bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if generation != int64(c.generation) {
		return false
	}

	now := time.Now()

	for k, v := range c.entries {
		if !v.Persistent && now.Sub(v.At) >= c.ttl {
			delete(c.entries, k)
		}
	}

	targetKey := EntryKey{Scope: scope, Key: key}

	maxCap := c.capacity
	if maxCap < 1 {
		maxCap = 1
	}

	_, exists := c.entries[targetKey]
	if !exists && len(c.entries) >= maxCap {
		var oldestKey EntryKey
		var oldestTime time.Time
		first := true

		for k, v := range c.entries {
			if first || v.At.Before(oldestTime) {
				oldestTime = v.At
				oldestKey = k
				first = false
			}
		}

		if !first {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[targetKey] = Entry{
		Value:      value,
		At:         now,
		Persistent: persistent,
	}

	return true
}

func (c *CapabilityBindingCache) TryGet(sessionId, key string) (string, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	type ServerTool struct {
		Server string
		Tool   string
	}

	if d, ok := TryGetValue[ServerTool](c, sessionId, key); ok {
		return d.Server, d.Tool, true
	}

	return "", "", false
}

func (c *CapabilityBindingCache) Set(sessionId, key, server, tool string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	type ServerTool struct {
		Server string
		Tool   string
	}

	c.Store(sessionId, key, ServerTool{Server: server, Tool: tool}, c.generation, false)
}

func (c *CapabilityBindingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.generation++
	c.entries = map[EntryKey]Entry{}
}

func ComputeIntentKey(taskDescription string, keywords *string, selectionPolicy string) string {
	var words []string
	if keywords != nil && *keywords != "" {
		rawWords := strings.Split(*keywords, ",")
		seen := make(map[string]struct{})
		for _, w := range rawWords {
			trimmed := strings.TrimSpace(w)
			if trimmed != "" {
				if _, exists := seen[trimmed]; !exists {
					seen[trimmed] = struct{}{}
					words = append(words, trimmed)
				}
			}
		}
		sort.Strings(words)
	}

	joinedWords := strings.Join(words, ",")

	policyNormalized := strings.ToLower(strings.ReplaceAll(selectionPolicy, "_", ""))

	parts := []string{taskDescription, joinedWords, policyNormalized}

	var builder strings.Builder
	for _, p := range parts {
		charCount := len([]rune(p))
		fmt.Fprintf(&builder, "%d:%s", charCount, p)
	}

	hash := sha256.Sum256([]byte(builder.String()))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

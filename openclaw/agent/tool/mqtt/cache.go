package mqtt

import (
	"slices"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
)

type CacheEntry struct {
	Topic      string
	Payload    string
	ReceivedAt time.Time
}

var mu sync.Mutex

var lastByTopic map[string]CacheEntry = map[string]CacheEntry{}

func GetPayload(topic string) (*string, *time.Time, bool) {
	mu.Lock()
	defer mu.Unlock()

	if e, ok := lastByTopic[topic]; ok {
		return &e.Payload, &e.ReceivedAt, true
	}

	return nil, nil, false
}

func SetPayload(topic, payload string) {
	mu.Lock()
	defer mu.Unlock()

	lastByTopic[topic] = CacheEntry{
		Topic:      topic,
		Payload:    payload,
		ReceivedAt: time.Now(),
	}
}

func FindByGlob(topicGlob string) []CacheEntry {
	mu.Lock()
	defer mu.Unlock()

	list := []CacheEntry{}

	for topic, v := range lastByTopic {
		if core.GlobMatcherInstance.IsMatch(topicGlob, topic) {
			list = append(list, CacheEntry{
				Topic:      v.Topic,
				Payload:    v.Payload,
				ReceivedAt: v.ReceivedAt,
			})
		}
	}

	slices.SortFunc(list, func(a, b CacheEntry) int {
		return b.ReceivedAt.Compare(a.ReceivedAt)
	})

	return list
}

package core

import "time"

type DiscoverRequestParams struct {
	RequestParams
}

type DiscoverResult struct {
	Meta              map[string]any     `json:"_meta,omitempty"`
	ResultType        string             `json:"resultType,omitempty"`
	SupportedVersions []string           `json:"supportedVersions"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      string             `json:"instructions"`
	TimeToLive        time.Time          `json:"ttlMs"`
	CacheScope        CacheScope         `json:"cacheScope"`
}

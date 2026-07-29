package protocol

import (
	"strings"
	"time"
)

type ListResourceTemplatesRequestParams struct {
	PaginatedRequestParams `json:",inline"`
}

type ListResourceTemplatesResult struct {
	PaginatedResult   `json:",inline"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	TimeToLive        time.Time          `json:"ttlMs"`
	CacheScope        *CacheScope        `json:"cacheScope"`
}

type ResourceTemplate struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	UriTemplate string         `json:"uriTemplate"`
	Description *string        `json:"description"`
	MimeType    *string        `json:"mimeType"`
	Annotations *Annotations   `json:"annotations"`
	Icons       []Icon         `json:"icons"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

func (r ResourceTemplate) IsTemplated() bool {
	return strings.ContainsAny(r.UriTemplate, "{")
}

func (r ResourceTemplate) AsResource() *Resource {
	if r.IsTemplated() {
		return nil
	}
	return &Resource{
		Uri:         r.UriTemplate,
		Name:        r.Name,
		Description: r.Description,
		MimeType:    r.MimeType,
		Annotations: r.Annotations,
		Title:       r.Title,
		Icons:       r.Icons,
		Meta:        r.Meta,
	}
}

type Resource struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Uri         string         `json:"uri"`
	Description *string        `json:"description"`
	MimeType    *string        `json:"mimeType,omitempty"`
	Size        *float32       `json:"size,omitempty"`
	Annotations *Annotations   `json:"annotations,omitempty"`
	Icons       []Icon         `json:"icons"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

type ResourceListChangedNotificationParams struct {
	NotificationParams
}

type ResourceUpdatedNotificationParams struct {
	NotificationParams
	Uri string `json:"uri"`
}

type ResourcesCapability struct {
	Subscribe   *bool `json:"subscribe,omitempty"`
	ListChanged *bool `json:"listChanged,omitempty"`
}

type ListResourcesRequestParams struct {
	PaginatedRequestParams `json:",inline"`
}

type ListResourcesResult struct {
	PaginatedResult `json:",inline"`
	Resources       []Resource  `json:"resources"`
	TimeToLive      time.Time   `json:"ttlMs"`
	CacheScope      *CacheScope `json:"cacheScope"`
}

type ReadResourceRequestParams struct {
	RequestParams `json:",inline"`
	Uri           string `json:"uri"`
}

type ReadResourceResult struct {
	Meta       map[string]any     `json:"_meta,omitempty"`
	ResultType string             `json:"resultType"`
	Contents   []ResourceContents `json:"contents"`
	TimeToLive time.Time          `json:"ttlMs"`
	CacheScope *CacheScope        `json:"cacheScope"`
}

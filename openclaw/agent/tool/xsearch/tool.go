package xsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type XSearchTool struct {
	httpClient  *http.Client
	bearerToken string
}

func NewXSearchTool() *XSearchTool {
	return &XSearchTool{
		httpClient:  &http.Client{},
		bearerToken: core.SecretResolverInstance.Resolve("env:X_BEARER_TOKEN"),
	}
}

func (e *XSearchTool) Name() string {
	return "x_search"
}

func (e *XSearchTool) Description() string {
	return "Search posts on the X (Twitter) platform. Requires X_BEARER_TOKEN environment variable."
}

func (e *XSearchTool) ParameterSchema() string {
	return `{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Search query (supports X search operators)"
		},
		"max_results": {
			"type": "integer",
			"description": "Maximum results to return (10-100, default 10)"
		}
	},
	"required": ["query"]
}`
}

type XSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type XSearchResponse struct {
	Datas []XSearchItemResponse `json:"data"`
}

type XSearchItemResponse struct {
	Text      string `json:"text"`
	AuthorId  string `json:"author_id"`
	CreatedAt string `json:"created_at"`
}

func (e *XSearchTool) Execute(ctx context.Context, argumentsJson string) string {
	if e.bearerToken == "" {
		return "Error: X API bearer token not configured. Set X_BEARER_TOKEN environment variable."
	}

	var args XSearchArgs

	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("Failed to parse parameters.: %v", err)
	}

	if args.Query == "" {
		return "Error: 'query' is required."
	}

	if args.MaxResults <= 0 {
		args.MaxResults = 10
	}

	args.MaxResults = util.Clamp(args.MaxResults, 10, 100)
	var pathurl = fmt.Sprintf("https://api.x.com/2/tweets/search/recent?query=%s&max_results=%d&tweet.fields=created_at,author_id,text,public_metrics",
		url.QueryEscape(args.Query),
		args.MaxResults,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", pathurl, nil)
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+e.bearerToken)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("http response code: %d", resp.StatusCode)
	}

	var doc XSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err.Error()
	}

	sb := strings.Builder{}

	count := 0
	for _, v := range doc.Datas {
		count++
		fmt.Fprintf(&sb, "[%d] @%s (%s)\n", count, v.AuthorId, v.CreatedAt)
		sb.WriteString(v.Text)
		sb.WriteString("\n\n")
	}

	if sb.Len() == 0 {
		return "No results found."
	}

	return sb.String()
}

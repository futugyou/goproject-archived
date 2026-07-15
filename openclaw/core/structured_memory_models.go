package core

import "time"

const (
	StructuredMemoryStatusResponseModeMcp            = "mcp"
	StructuredMemoryStatusResponseMcpCommandDefault  = "fractalmem-mcp"
	StructuredMemoryStatusResponseAutoContextModeOff = "off"
	StructuredMemoryStatusResponseStatusDisabled     = "disabled"

	StructuredMemoryOpenResultViewIndex      = "index"
	StructuredMemoryExportResultModeCompact  = "compact"
	StructuredMemoryContextRequestModeManual = "manual"
	StructuredMemoryContextResultModeCompact = "compact"
)

type StructuredMemoryStatusResponse struct {
	Enabled                bool                              `json:"enabled"`
	Mode                   string                            `json:"mode"`
	RepositoryRoot         string                            `json:"repository_root"`
	ResolvedRepositoryRoot string                            `json:"resolved_repository_root"`
	McpCommand             string                            `json:"mcp_command"`
	AutoContextMode        string                            `json:"auto_context_mode"`
	AllowWrites            bool                              `json:"allow_writes"`
	WriteToolsAvailable    bool                              `json:"write_tools_available"`
	Available              bool                              `json:"available"`
	Status                 string                            `json:"status"`
	Error                  string                            `json:"error,omitempty"`
	Warnings               []string                          `json:"warnings"`
	Validation             *StructuredMemoryValidationResult `json:"validation,omitempty"`
}

func DefaultStructuredMemoryStatusResponse() *StructuredMemoryStatusResponse {
	return &StructuredMemoryStatusResponse{
		Mode:            StructuredMemoryStatusResponseModeMcp,
		McpCommand:      StructuredMemoryStatusResponseMcpCommandDefault,
		AutoContextMode: StructuredMemoryStatusResponseAutoContextModeOff,
		Status:          StructuredMemoryStatusResponseStatusDisabled,
	}
}

type StructuredMemorySearchResult struct {
	Success bool                        `json:"success"`
	Query   string                      `json:"query"`
	Scope   string                      `json:"scope,omitempty"`
	Items   []StructuredMemorySourceRef `json:"items"`
	Error   string                      `json:"error,omitempty"`
}

type StructuredMemoryOpenResult struct {
	Success         bool                        `json:"success"`
	Path            string                      `json:"path"`
	Title           string                      `json:"title,omitempty"`
	Summary         string                      `json:"summary,omitempty"`
	Depth           int                         `json:"depth"`
	View            string                      `json:"view"`
	Content         string                      `json:"content,omitempty"`
	Children        []StructuredMemorySourceRef `json:"children"`
	SuggestedReads  []StructuredMemorySourceRef `json:"suggested_reads"`
	RecentTimeline  []StructuredMemorySourceRef `json:"recent_timeline"`
	RecentDecisions []StructuredMemorySourceRef `json:"recent_decisions"`
	Sources         []StructuredMemorySourceRef `json:"sources"`
	Error           string                      `json:"error,omitempty"`
}

func DefaultStructuredMemoryOpenResult() *StructuredMemoryOpenResult {
	return &StructuredMemoryOpenResult{
		View: StructuredMemoryOpenResultViewIndex,
	}
}

type StructuredMemoryRecentResult struct {
	Success bool                        `json:"success"`
	Days    int                         `json:"days"`
	Scope   string                      `json:"scope,omitempty"`
	Items   []StructuredMemorySourceRef `json:"items"`
	Error   string                      `json:"error,omitempty"`
}

type StructuredMemoryExportResult struct {
	Success   bool                        `json:"success"`
	Path      string                      `json:"path"`
	Mode      string                      `json:"mode"`
	Title     string                      `json:"title,omitempty"`
	Content   string                      `json:"content"`
	Sources   []StructuredMemorySourceRef `json:"sources"`
	CharCount int                         `json:"char_count"`
	Truncated bool                        `json:"truncated"`
	Error     string                      `json:"error,omitempty"`
}

func DefaultStructuredMemoryExportResult() *StructuredMemoryExportResult {
	return &StructuredMemoryExportResult{
		Mode:    StructuredMemoryExportResultModeCompact,
		Sources: []StructuredMemorySourceRef{},
	}
}

type StructuredMemoryHandoffResult struct {
	Success         bool                        `json:"success"`
	Path            string                      `json:"path"`
	HandoffFilePath string                      `json:"handoff_file_path,omitempty"`
	Content         string                      `json:"content,omitempty"`
	Sources         []StructuredMemorySourceRef `json:"sources"`
	Error           string                      `json:"error,omitempty"`
}

type StructuredMemoryValidationResult struct {
	Success   bool                              `json:"success"`
	HasErrors bool                              `json:"has_errors"`
	Issues    []StructuredMemoryValidationIssue `json:"issues"`
	Summary   string                            `json:"summary,omitempty"`
	Error     string                            `json:"error,omitempty"`
}

func DefaultStructuredMemoryValidationResult() *StructuredMemoryValidationResult {
	return &StructuredMemoryValidationResult{
		Issues: []StructuredMemoryValidationIssue{},
	}
}

type StructuredMemoryValidationIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type StructuredMemorySourceRef struct {
	Path            string     `json:"path"`
	Title           string     `json:"title,omitempty"`
	FileName        string     `json:"file_name,omitempty"`
	SourcePath      string     `json:"source_path"`
	SectionHeading  string     `json:"section_heading,omitempty"`
	StartLine       *int       `json:"start_line,omitempty"`
	EndLine         *int       `json:"end_line,omitempty"`
	Snippet         string     `json:"snippet,omitempty"`
	Score           *float64   `json:"score,omitempty"`
	LastModifiedUtc *time.Time `json:"last_modified_utc,omitempty"`
}

type StructuredMemoryContextRequest struct {
	Query     string `json:"query"`
	PathHint  string `json:"path_hint,omitempty"`
	SessionId string `json:"session_id,omitempty"`
	Mode      string `json:"mode"`
	MaxChars  *int   `json:"max_chars,omitempty"`
	MaxTokens *int   `json:"max_tokens,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

func DefaultStructuredMemoryContextRequest() *StructuredMemoryContextRequest {
	return &StructuredMemoryContextRequest{
		Mode: StructuredMemoryContextRequestModeManual,
	}
}

type StructuredMemoryContextResult struct {
	Success    bool                        `json:"success"`
	Context    string                      `json:"context,omitempty"`
	SourcePath string                      `json:"source_path,omitempty"`
	Mode       string                      `json:"mode"`
	Truncated  bool                        `json:"truncated"`
	Sources    []StructuredMemorySourceRef `json:"sources"`
	Error      string                      `json:"error,omitempty"`
}

func DefaultStructuredMemoryContextResult() *StructuredMemoryContextResult {
	return &StructuredMemoryContextResult{
		Mode:    StructuredMemoryContextResultModeCompact,
		Sources: []StructuredMemorySourceRef{},
	}
}

type StructuredMemoryPathRequest struct {
	Path string `json:"path"`
}

package core

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	ExternalCliPresetCatalogRepoPattern       = `^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`
	ExternalCliPresetCatalogNumberPattern     = `^[0-9]+$`
	ExternalCliPresetCatalogSimpleNamePattern = `^[A-Za-z0-9_.:-]+$`
)

type ExternalCliPresetDefinition struct {
	Summary   ExternalCliPresetSummary
	Connector ExternalCliConnectorOptions
}

var (
	presetsInstance map[string]ExternalCliPresetDefinition
	once            sync.Once
)

type ExternalCliPresetCatalog struct{}

var ExternalCliPresetCatalogInstance = newExternalCliPresetCatalog()

func newExternalCliPresetCatalog() *ExternalCliPresetCatalog {
	once.Do(func() {
		presetsInstance = buildPresets()
	})
	return &ExternalCliPresetCatalog{}
}

// 抽象出的公共辅助函数：提取 map 的 key 并按不区分大小写字母排序
func getSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})
	return keys
}

func buildPresets() map[string]ExternalCliPresetDefinition {
	m := make(map[string]ExternalCliPresetDefinition)

	// =========================================================================
	// 1. GitHub CLI ("gh")
	// =========================================================================
	ghCommands := map[string]ExternalCliCommandOptions{
		"repo_view": {
			Description:      "View repository metadata.",
			ArgsTemplate:     []string{"repo", "view", "{{repo}}", "--json", "name,owner,description,url,isPrivate"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo": {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"github", "repository"},
		},
		"issue_list": {
			Description:      "List repository issues.",
			ArgsTemplate:     []string{"issue", "list", "--repo", "{{repo}}", "--json", "number,title,state,author,url,labels"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo": {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"github", "issues"},
		},
		"pr_list": {
			Description:      "List repository pull requests.",
			ArgsTemplate:     []string{"pr", "list", "--repo", "{{repo}}", "--json", "number,title,state,author,url,headRefName,baseRefName"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo": {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"github", "pull-requests"},
		},
		"pr_view": {
			Description:      "View pull request metadata.",
			ArgsTemplate:     []string{"pr", "view", "{{number}}", "--repo", "{{repo}}", "--json", "number,title,state,url,mergeStateStatus,reviewDecision,headRefName,baseRefName"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo":   {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
				"number": {Required: true, Description: "Pull request number.", Pattern: ExternalCliPresetCatalogNumberPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"github", "pull-requests"},
		},
		"release_list": {
			Description:      "List repository releases.",
			ArgsTemplate:     []string{"release", "list", "--repo", "{{repo}}", "--json", "name,tagName,isDraft,isPrerelease,publishedAt,url"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo": {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"github", "releases"},
		},
		"issue_comment": {
			Description:      "Add a comment to a GitHub issue or pull request.",
			ArgsTemplate:     []string{"issue", "comment", "{{number}}", "--repo", "{{repo}}", "--body", "{{body}}"},
			ReadOnly:         false,
			RiskLevel:        ExternalCliRiskLevelMedium,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"repo":   {Required: true, Description: "owner/repo repository name.", Pattern: ExternalCliPresetCatalogRepoPattern, AllowedValues: []string{}},
				"number": {Required: true, Description: "Issue or pull request number.", Pattern: ExternalCliPresetCatalogNumberPattern, AllowedValues: []string{}},
				"body":   {Required: true, Description: "Comment body.", MaxLength: 16000, AllowedValues: []string{}},
			},
			StructuredOutput: "",
			Tags:             []string{"github", "issues", "mutating"},
		},
	}

	m["gh"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "gh",
			Connector:   "gh",
			DisplayName: "GitHub CLI",
			Description: "Read-oriented GitHub repository, issue, pull request, and release commands.",
			Tags:        []string{"github", "vcs"},
			Commands:    getSortedKeys(ghCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "GitHub CLI",
			Executable:          "gh",
			DefaultOutputFormat: ExternalCliOutputFormatJson,
			StatusCommand:       &ExternalCliStatusCommandOptions{Args: []string{"auth", "status"}, TimeoutSeconds: 20},
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            ghCommands,
		},
	}

	// =========================================================================
	// 2. Azure CLI ("az")
	// =========================================================================
	azCommands := map[string]ExternalCliCommandOptions{
		"account_show": {
			Description:      "Show the active Azure account.",
			ArgsTemplate:     []string{"account", "show", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"azure", "account"},
		},
		"group_list": {
			Description:      "List Azure resource groups.",
			ArgsTemplate:     []string{"group", "list", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"azure", "resource-groups"},
		},
		"resource_list": {
			Description:      "List Azure resources.",
			ArgsTemplate:     []string{"resource", "list", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"azure", "resources"},
		},
	}

	m["az"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "az",
			Connector:   "az",
			DisplayName: "Azure CLI",
			Description: "Read-only Azure account, resource group, and resource inventory commands.",
			Tags:        []string{"azure", "cloud"},
			Commands:    getSortedKeys(azCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "Azure CLI",
			Executable:          "az",
			DefaultOutputFormat: ExternalCliOutputFormatJson,
			StatusCommand:       &ExternalCliStatusCommandOptions{Args: []string{"account", "show", "--output", "json"}, TimeoutSeconds: 20},
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            azCommands,
		},
	}

	// =========================================================================
	// 3. Kubernetes CLI ("kubectl")
	// =========================================================================
	kubeCommands := map[string]ExternalCliCommandOptions{
		"current_context": {
			Description:      "Show the current Kubernetes context.",
			ArgsTemplate:     []string{"config", "current-context"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"kubernetes", "context"},
		},
		"get_pods_all": {
			Description:      "List pods in all namespaces.",
			ArgsTemplate:     []string{"get", "pods", "--all-namespaces", "-o", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"kubernetes", "pods"},
		},
		"get_pods": {
			Description:      "List pods in one namespace.",
			ArgsTemplate:     []string{"get", "pods", "--namespace", "{{namespace}}", "-o", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"namespace": {Required: true, Description: "Kubernetes namespace.", Pattern: ExternalCliPresetCatalogSimpleNamePattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"kubernetes", "pods"},
		},
		"get_services_all": {
			Description:      "List services in all namespaces.",
			ArgsTemplate:     []string{"get", "services", "--all-namespaces", "-o", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"kubernetes", "services"},
		},
		"get_deployments_all": {
			Description:      "List deployments in all namespaces.",
			ArgsTemplate:     []string{"get", "deployments", "--all-namespaces", "-o", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"kubernetes", "deployments"},
		},
	}

	m["kubectl"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "kubectl",
			Connector:   "kubectl",
			DisplayName: "kubectl",
			Description: "Read-only Kubernetes context and workload inventory commands.",
			Tags:        []string{"kubernetes", "cluster"},
			Commands:    getSortedKeys(kubeCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "kubectl",
			Executable:          "kubectl",
			DefaultOutputFormat: ExternalCliOutputFormatJson,
			StatusCommand:       &ExternalCliStatusCommandOptions{Args: []string{"config", "current-context"}, TimeoutSeconds: 10},
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"version", "--client=true", "-o", "json"}, TimeoutSeconds: 15},
			Commands:            kubeCommands,
		},
	}

	// =========================================================================
	// 4. Stripe CLI ("stripe")
	// =========================================================================
	stripeCommands := map[string]ExternalCliCommandOptions{
		"version": {
			Description:      "Show Stripe CLI version.",
			ArgsTemplate:     []string{"--version"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"stripe", "diagnostics"},
		},
		"customers_list": {
			Description:      "List Stripe customers with a small limit. Customer data can contain PII.",
			ArgsTemplate:     []string{"customers", "list", "--limit", "{{limit}}"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelMedium,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"limit": {Required: true, Description: "Maximum customers to list.", Pattern: ExternalCliPresetCatalogNumberPattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"stripe", "customers", "pii"},
		},
	}

	m["stripe"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "stripe",
			Connector:   "stripe",
			DisplayName: "Stripe CLI",
			Description: "Conservative Stripe CLI commands with PII-bearing reads approval-gated.",
			Tags:        []string{"stripe", "payments"},
			Commands:    getSortedKeys(stripeCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "Stripe CLI",
			Executable:          "stripe",
			DefaultOutputFormat: ExternalCliOutputFormatText,
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            stripeCommands,
		},
	}

	// =========================================================================
	// 5. Lark CLI ("lark")
	// =========================================================================
	larkCommands := map[string]ExternalCliCommandOptions{
		"auth_status": {
			Description:      "Show Lark CLI authentication status.",
			ArgsTemplate:     []string{"auth", "status"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"lark", "auth"},
		},
		"docs_search": {
			Description:      "Search Lark docs.",
			ArgsTemplate:     []string{"docs", "search", "{{query}}", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"query": {Required: true, Description: "Search query.", MaxLength: 500, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"lark", "docs"},
		},
		"docs_read": {
			Description:      "Read a Lark document.",
			ArgsTemplate:     []string{"docs", "read", "{{document_id}}", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"document_id": {Required: true, Description: "Lark document identifier.", Pattern: ExternalCliPresetCatalogSimpleNamePattern, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"lark", "docs"},
		},
		"sheets_read": {
			Description:      "Read a Lark sheet range.",
			ArgsTemplate:     []string{"sheets", "read", "{{sheet_id}}", "{{range}}", "--output", "json"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters: map[string]ExternalCliParameterOptions{
				"sheet_id": {Required: true, Description: "Lark sheet identifier.", Pattern: ExternalCliPresetCatalogSimpleNamePattern, AllowedValues: []string{}},
				"range":    {Required: true, Description: "Sheet range.", MaxLength: 200, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"lark", "sheets"},
		},
		"message_send": {
			Description:      "Send a Lark message.",
			ArgsTemplate:     []string{"messages", "send", "--chat", "{{chat_id}}", "--text", "{{text}}", "--output", "json"},
			ReadOnly:         false,
			RiskLevel:        ExternalCliRiskLevelMedium,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"chat_id": {Required: true, Description: "Lark chat identifier.", Pattern: ExternalCliPresetCatalogSimpleNamePattern, AllowedValues: []string{}},
				"text":    {Required: true, Description: "Message text.", MaxLength: 4000, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatJson,
			Tags:             []string{"lark", "messages", "mutating"},
		},
	}

	m["lark"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "lark",
			Connector:   "lark",
			DisplayName: "Lark CLI",
			Description: "Generic lark-cli compatible templates for auth checks, docs, sheets, and guarded sends.",
			Tags:        []string{"lark", "feishu", "collaboration"},
			Commands:    getSortedKeys(larkCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "Lark CLI",
			Executable:          "lark-cli",
			DefaultOutputFormat: ExternalCliOutputFormatJson,
			StatusCommand:       &ExternalCliStatusCommandOptions{Args: []string{"auth", "status"}, TimeoutSeconds: 20},
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            larkCommands,
		},
	}

	// =========================================================================
	// 6. GitHub Copilot CLI ("github-copilot")
	// =========================================================================
	copilotCommands := map[string]ExternalCliCommandOptions{
		"version": {
			Description:      "Show GitHub Copilot CLI version.",
			ArgsTemplate:     []string{"version"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"github", "copilot", "diagnostics"},
		},
		"prompt": {
			Description:      "Run a non-interactive GitHub Copilot CLI prompt.",
			ArgsTemplate:     []string{"-p", "{{prompt}}"},
			ReadOnly:         false,
			RiskLevel:        ExternalCliRiskLevelHigh,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"prompt": {Required: true, Description: "Prompt to send to GitHub Copilot CLI.", MaxLength: 8000, AllowedValues: []string{}},
			},
			StructuredOutput: "",
			Tags:             []string{"github", "copilot", "ai-agent"},
		},
	}

	m["github-copilot"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "github-copilot",
			Connector:   "github-copilot",
			DisplayName: "GitHub Copilot CLI",
			Description: "GitHub Copilot CLI non-interactive prompt and diagnostic commands.",
			Tags:        []string{"github", "copilot", "ai"},
			Commands:    getSortedKeys(copilotCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "GitHub Copilot CLI",
			Executable:          "copilot",
			DefaultOutputFormat: ExternalCliOutputFormatText,
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"version"}, TimeoutSeconds: 10},
			Commands:            copilotCommands,
		},
	}

	// =========================================================================
	// 7. Codex CLI ("codex")
	// =========================================================================
	codexCommands := map[string]ExternalCliCommandOptions{
		"version": {
			Description:      "Show Codex CLI version.",
			ArgsTemplate:     []string{"--version"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"codex", "diagnostics"},
		},
		"exec_readonly": {
			Description:      "Run Codex non-interactively in an explicit read-only sandbox.",
			ArgsTemplate:     []string{"exec", "--sandbox", "read-only", "{{prompt}}"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelMedium,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"prompt": {Required: true, Description: "Prompt to send to Codex CLI.", MaxLength: 8000, AllowedValues: []string{}},
			},
			StructuredOutput: "",
			Tags:             []string{"codex", "ai-agent"},
		},
		"exec_readonly_json": {
			Description:      "Run Codex non-interactively with JSON Lines events in a read-only sandbox.",
			ArgsTemplate:     []string{"exec", "--sandbox", "read-only", "--json", "{{prompt}}"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelMedium,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"prompt": {Required: true, Description: "Prompt to send to Codex CLI.", MaxLength: 8000, AllowedValues: []string{}},
			},
			StructuredOutput: ExternalCliOutputFormatNdjson,
			Tags:             []string{"codex", "ai-agent", "jsonl"},
		},
		"exec_workspace_write": {
			Description:      "Run Codex non-interactively with workspace-write sandboxing.",
			ArgsTemplate:     []string{"exec", "--sandbox", "workspace-write", "{{prompt}}"},
			ReadOnly:         false,
			RiskLevel:        ExternalCliRiskLevelHigh,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"prompt": {Required: true, Description: "Prompt to send to Codex CLI.", MaxLength: 8000, AllowedValues: []string{}},
			},
			StructuredOutput: "",
			Tags:             []string{"codex", "ai-agent", "mutating"},
		},
	}

	m["codex"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "codex",
			Connector:   "codex",
			DisplayName: "Codex CLI",
			Description: "Codex CLI non-interactive exec commands with explicit sandbox choices.",
			Tags:        []string{"codex", "openai", "ai"},
			Commands:    getSortedKeys(codexCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "Codex CLI",
			Executable:          "codex",
			DefaultOutputFormat: ExternalCliOutputFormatText,
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            codexCommands,
		},
	}

	// =========================================================================
	// 8. Gemini CLI ("gemini")
	// =========================================================================
	geminiCommands := map[string]ExternalCliCommandOptions{
		"version": {
			Description:      "Show Gemini CLI version.",
			ArgsTemplate:     []string{"--version"},
			ReadOnly:         true,
			RiskLevel:        ExternalCliRiskLevelLow,
			RequiresApproval: false,
			Parameters:       map[string]ExternalCliParameterOptions{},
			StructuredOutput: ExternalCliOutputFormatText,
			Tags:             []string{"gemini", "diagnostics"},
		},
		"prompt": {
			Description:      "Run a non-interactive Gemini CLI prompt.",
			ArgsTemplate:     []string{"-p", "{{prompt}}"},
			ReadOnly:         false,
			RiskLevel:        ExternalCliRiskLevelHigh,
			RequiresApproval: true,
			Parameters: map[string]ExternalCliParameterOptions{
				"prompt": {Required: true, Description: "Prompt to send to Gemini CLI.", MaxLength: 8000, AllowedValues: []string{}},
			},
			StructuredOutput: "",
			Tags:             []string{"gemini", "ai-agent"},
		},
	}

	m["gemini"] = ExternalCliPresetDefinition{
		Summary: ExternalCliPresetSummary{
			Id:          "gemini",
			Connector:   "gemini",
			DisplayName: "Gemini CLI",
			Description: "Gemini CLI non-interactive prompt and diagnostic commands.",
			Tags:        []string{"gemini", "google", "ai"},
			Commands:    getSortedKeys(geminiCommands),
		},
		Connector: ExternalCliConnectorOptions{
			Enabled:             false,
			DisplayName:         "Gemini CLI",
			Executable:          "gemini",
			DefaultOutputFormat: ExternalCliOutputFormatText,
			VersionCommand:      &ExternalCliStatusCommandOptions{Args: []string{"--version"}, TimeoutSeconds: 10},
			Commands:            geminiCommands,
		},
	}

	return m
}

func (*ExternalCliPresetCatalog) riskRank(risk string) int {
	switch NormalizeRiskLevel(risk) {
	case ExternalCliRiskLevelHigh:
		return 3
	case ExternalCliRiskLevelMedium:
		return 2
	default:
		return 1
	}
}

func (e *ExternalCliPresetCatalog) maxRisk(left, right string) string {
	var normalizedLeft = NormalizeRiskLevel(left)
	var normalizedRight = NormalizeRiskLevel(right)
	if e.riskRank(normalizedRight) > e.riskRank(normalizedLeft) {
		return normalizedRight
	}
	return normalizedLeft
}

func (e *ExternalCliPresetCatalog) prefer(configured, preset string) string {
	if IsBlank(configured) {
		return preset
	}
	return configured
}

func (e *ExternalCliPresetCatalog) mergeStringSet(preset, configured []string) []string {
	preset = append(preset, configured...)
	result := []string{}
	tmp := map[string]struct{}{}
	for _, v := range preset {
		if IsBlank(v) {
			continue
		}

		if _, ok := tmp[v]; ok {
			continue
		}
		tmp[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func (e *ExternalCliPresetCatalog) mergeDictionary(preset, configured map[string]string) map[string]string {
	maps.Copy(preset, configured)
	return preset
}

func (e *ExternalCliPresetCatalog) mergeParameter(preset, configured ExternalCliParameterOptions) ExternalCliParameterOptions {
	result := ExternalCliParameterOptions{
		Required:      preset.Required || configured.Required,
		Description:   e.prefer(configured.Description, preset.Description),
		MaxLength:     configured.MaxLength,
		Pattern:       e.prefer(configured.Pattern, preset.Pattern),
		AllowedValues: configured.AllowedValues,
	}

	if result.MaxLength == 0 {
		result.MaxLength = preset.MaxLength
	}

	if len(result.AllowedValues) == 0 {
		result.AllowedValues = preset.AllowedValues
	}

	return result
}

func (e *ExternalCliPresetCatalog) cloneParameter(source ExternalCliParameterOptions) ExternalCliParameterOptions {
	if source.AllowedValues != nil {
		source.AllowedValues = slices.Clone(source.AllowedValues)
	}
	return source
}

func (e *ExternalCliPresetCatalog) cloneParameters(source map[string]ExternalCliParameterOptions) map[string]ExternalCliParameterOptions {
	if source == nil {
		return nil
	}

	result := make(map[string]ExternalCliParameterOptions, len(source))
	for name, parameter := range source {
		result[name] = e.cloneParameter(parameter)
	}

	return result
}

func (e *ExternalCliPresetCatalog) mergeParameters(preset, configured map[string]ExternalCliParameterOptions) map[string]ExternalCliParameterOptions {
	var result = e.cloneParameters(preset)
	for name, parameter := range configured {
		presetParameter, ok := result[name]
		if ok {
			result[name] = e.mergeParameter(presetParameter, parameter)
		} else {
			result[name] = e.cloneParameter(parameter)
		}
	}

	return result
}

func (e *ExternalCliPresetCatalog) cloneStatusCommand(source *ExternalCliStatusCommandOptions) *ExternalCliStatusCommandOptions {
	if source == nil {
		return nil
	}

	return &ExternalCliStatusCommandOptions{
		Args:           slices.Clone(source.Args),
		TimeoutSeconds: source.TimeoutSeconds,
	}
}

func (e *ExternalCliPresetCatalog) mergeStatusCommand(preset, configured *ExternalCliStatusCommandOptions) *ExternalCliStatusCommandOptions {
	if preset == nil {
		return e.cloneStatusCommand(configured)
	}

	if configured == nil {
		return e.cloneStatusCommand(preset)
	}

	result := &ExternalCliStatusCommandOptions{
		Args:           slices.Clone(configured.Args),
		TimeoutSeconds: configured.TimeoutSeconds,
	}

	if len(result.Args) == 0 {
		result.Args = slices.Clone(preset.Args)
	}

	if result.TimeoutSeconds == 0 {
		result.TimeoutSeconds = preset.TimeoutSeconds
	}

	return result
}

func (e *ExternalCliPresetCatalog) cloneCommand(source ExternalCliCommandOptions) ExternalCliCommandOptions {
	result := ExternalCliCommandOptions{
		Description:            source.Description,
		ArgsTemplate:           slices.Clone(source.ArgsTemplate),
		Parameters:             e.cloneParameters(source.Parameters),
		AllowUnknownParameters: source.AllowUnknownParameters,
		RiskLevel:              source.RiskLevel,
		ReadOnly:               source.ReadOnly,
		RequiresApproval:       source.RequiresApproval,
		SupportsDryRun:         source.SupportsDryRun,
		DryRunArgsTemplate:     slices.Clone(source.DryRunArgsTemplate),
		StructuredOutput:       source.StructuredOutput,
		TimeoutSeconds:         source.TimeoutSeconds,
		WorkingDirectory:       source.WorkingDirectory,
		RedactionRules:         slices.Clone(source.RedactionRules),
		RequiredScopes:         slices.Clone(source.RequiredScopes),
		RequiredIdentity:       source.RequiredIdentity,
		Tags:                   slices.Clone(source.Tags),
	}

	result.Environment = maps.Clone(source.Environment)
	return result
}

func (e *ExternalCliPresetCatalog) mergeConnector(preset, configured ExternalCliConnectorOptions) ExternalCliConnectorOptions {
	commands := e.cloneCommands(preset.Commands)
	for name, command := range configured.Commands {
		lowerName := strings.ToLower(name)
		if presetCommand, exists := commands[lowerName]; exists {
			commands[lowerName] = e.mergeCommand(presetCommand, command)
		} else {
			commands[lowerName] = e.cloneCommand(command)
		}
	}

	var defaultArgs []string
	if len(configured.DefaultArgs) > 0 {
		defaultArgs = slices.Clone(configured.DefaultArgs)
	} else {
		defaultArgs = slices.Clone(preset.DefaultArgs)
	}

	return ExternalCliConnectorOptions{
		Enabled:             preset.Enabled || configured.Enabled,
		DisplayName:         e.prefer(configured.DisplayName, preset.DisplayName),
		Executable:          e.prefer(configured.Executable, preset.Executable),
		DefaultArgs:         defaultArgs,
		WorkingDirectory:    e.preferNullable(configured.WorkingDirectory, preset.WorkingDirectory),
		Environment:         e.mergeDictionary(preset.Environment, configured.Environment),
		StatusCommand:       e.mergeStatusCommand(preset.StatusCommand, configured.StatusCommand),
		VersionCommand:      e.mergeStatusCommand(preset.VersionCommand, configured.VersionCommand),
		DefaultOutputFormat: e.prefer(configured.DefaultOutputFormat, preset.DefaultOutputFormat),
		RequiresApproval:    preset.RequiresApproval || configured.RequiresApproval,
		RedactionRules:      e.mergeStringSet(preset.RedactionRules, configured.RedactionRules),
		Commands:            commands,
	}
}

func (e *ExternalCliPresetCatalog) mergeCommand(preset ExternalCliCommandOptions, configured ExternalCliCommandOptions) ExternalCliCommandOptions {
	var argsTemplate []string
	if len(configured.ArgsTemplate) > 0 {
		argsTemplate = slices.Clone(configured.ArgsTemplate)
	} else {
		argsTemplate = slices.Clone(preset.ArgsTemplate)
	}

	var dryRunArgsTemplate []string
	if len(configured.DryRunArgsTemplate) > 0 {
		dryRunArgsTemplate = slices.Clone(configured.DryRunArgsTemplate)
	} else {
		dryRunArgsTemplate = slices.Clone(preset.DryRunArgsTemplate)
	}

	return ExternalCliCommandOptions{
		Description:            e.prefer(configured.Description, preset.Description),
		ArgsTemplate:           argsTemplate,
		Parameters:             e.mergeParameters(preset.Parameters, configured.Parameters),
		AllowUnknownParameters: preset.AllowUnknownParameters || configured.AllowUnknownParameters,
		RiskLevel:              e.maxRisk(preset.RiskLevel, configured.RiskLevel),
		ReadOnly:               preset.ReadOnly && configured.ReadOnly,
		RequiresApproval:       preset.RequiresApproval || configured.RequiresApproval,
		SupportsDryRun:         preset.SupportsDryRun || configured.SupportsDryRun,
		DryRunArgsTemplate:     dryRunArgsTemplate,
		StructuredOutput:       e.prefer(configured.StructuredOutput, preset.StructuredOutput),
		TimeoutSeconds:         e.preferOptionalInt(configured.TimeoutSeconds, preset.TimeoutSeconds),
		WorkingDirectory:       e.preferNullable(configured.WorkingDirectory, preset.WorkingDirectory),
		Environment:            e.mergeDictionary(preset.Environment, configured.Environment),
		RedactionRules:         e.mergeStringSet(preset.RedactionRules, configured.RedactionRules),
		RequiredScopes:         e.mergeStringSet(preset.RequiredScopes, configured.RequiredScopes),
		RequiredIdentity:       e.preferNullable(configured.RequiredIdentity, preset.RequiredIdentity),
		Tags:                   e.mergeStringSet(preset.Tags, configured.Tags),
	}
}

func (e *ExternalCliPresetCatalog) preferNullable(one string, two string) string {
	if one != "" {
		return one
	}

	return two
}

func (e *ExternalCliPresetCatalog) preferOptionalInt(one *int, two *int) *int {
	if one != nil {
		return one
	}
	return two
}

func (e *ExternalCliPresetCatalog) cloneConnector(source ExternalCliConnectorOptions) ExternalCliConnectorOptions {
	return ExternalCliConnectorOptions{
		Enabled:             source.Enabled,
		DisplayName:         source.DisplayName,
		Executable:          source.Executable,
		DefaultArgs:         slices.Clone(source.DefaultArgs),
		WorkingDirectory:    source.WorkingDirectory,
		Environment:         maps.Clone(source.Environment),
		StatusCommand:       e.cloneStatusCommand(source.StatusCommand),
		VersionCommand:      e.cloneStatusCommand(source.VersionCommand),
		DefaultOutputFormat: source.DefaultOutputFormat,
		RequiresApproval:    source.RequiresApproval,
		RedactionRules:      slices.Clone(source.RedactionRules),
		Commands:            e.cloneCommands(source.Commands),
	}
}

func (e *ExternalCliPresetCatalog) cloneCommands(source map[string]ExternalCliCommandOptions) map[string]ExternalCliCommandOptions {
	result := make(map[string]ExternalCliCommandOptions, len(source))
	for name, command := range source {
		result[strings.ToLower(name)] = e.cloneCommand(command)
	}
	return result
}

func (e *ExternalCliPresetCatalog) Apply(options ExternalCliOptions) ExternalCliOptions {
	var effective = ExternalCliOptions{
		Enabled:                            options.Enabled,
		DefaultTimeoutSeconds:              options.DefaultTimeoutSeconds,
		MaxStdoutBytes:                     options.MaxStdoutBytes,
		MaxStderrBytes:                     options.MaxStderrBytes,
		RedactSecrets:                      options.RedactSecrets,
		AllowFreeformCommands:              options.AllowFreeformCommands,
		RequireApprovalForMutatingCommands: options.RequireApprovalForMutatingCommands,
		Connectors:                         map[string]ExternalCliConnectorOptions{},
	}

	effective.Presets = slices.Clone(options.Presets)
	once.Do(func() {
		presetsInstance = buildPresets()
	})
	for _, id := range options.Presets {
		if preset, ok := presetsInstance[id]; ok {
			effective.Connectors[preset.Summary.Connector] = e.cloneConnector(preset.Connector)
		}
	}
	for name, connector := range options.Connectors {
		presetConnector, ok := effective.Connectors[name]
		if ok {
			effective.Connectors[name] = e.mergeConnector(presetConnector, connector)
		} else {
			effective.Connectors[name] = e.cloneConnector(connector)
		}
	}

	return effective
}

func (e *ExternalCliPresetCatalog) FindUnknownPresetIds(options ExternalCliOptions) []string {
	once.Do(func() {
		presetsInstance = buildPresets()
	})
	result := []string{}
	tmp := map[string]struct{}{}
	for _, id := range options.Presets {
		if _, ok := presetsInstance[id]; !ok {
			if _, f := tmp[id]; !f {
				result = append(result, id)
				tmp[id] = struct{}{}
			}
		}
	}

	slices.Sort(result)
	return result
}

func (e *ExternalCliPresetCatalog) TryGet(id string) *ExternalCliPresetSummary {
	once.Do(func() {
		presetsInstance = buildPresets()
	})

	if preset, ok := presetsInstance[id]; ok {
		return &preset.Summary
	}

	return nil
}

func (e *ExternalCliPresetCatalog) List() []ExternalCliPresetSummary {
	once.Do(func() {
		presetsInstance = buildPresets()
	})

	result := make([]ExternalCliPresetSummary, 0, len(presetsInstance))
	for _, v := range presetsInstance {
		result = append(result, v.Summary)
	}

	slices.SortFunc(result, func(a, b ExternalCliPresetSummary) int {
		return cmp.Compare(a.Id, b.Id)
	})

	return result
}

type ExternalCliPreparedCommand struct {
	ConnectorName     string
	CommandName       string
	Connector         *ExternalCliConnectorOptions
	Command           *ExternalCliCommandOptions
	Executable        string
	Arguments         []string
	RedactedArguments []string
	WorkingDirectory  string
	Environment       map[string]string
	Preview           *ExternalCliInvocationPreview
	MaxStdoutBytes    int
	MaxStderrBytes    int
	RedactionRules    []string
}

type IExternalCliConnectorRegistry interface {
	ListConnectors() ([]ExternalCliConnectorSummary, error)
	ListCommands(connectorName string) (*ExternalCliCommandListResponse, error)
	GetCommandSchema(connectorName, commandName string) (*ExternalCliCommandSchemaResponse, error)
	BuildPreview(request *ExternalCliPreviewRequest, dryRun bool) (*ExternalCliPreparedCommand, error)
	GetStatus(ctx context.Context, connectorName string) (*ExternalCliConnectorStatus, error)
}

type IExternalCliRunner interface {
	Execute(ctx context.Context, command *ExternalCliPreparedCommand) (*ExternalCliExecutionResult, error)
}

type IExternalCliAuditSink interface {
	Record(entry *ExternalCliAuditEntry) error
}

type IExternalCliEventSink interface {
	Record(entry *ExternalCliRuntimeEvent) error
}

var _ IExternalCliRunner = (*ExternalCliRunner)(nil)

type ExternalCliRunner struct {
	redaction IRedactionPipeline
}

func NewExternalCliRunner(redaction IRedactionPipeline) *ExternalCliRunner {
	if redaction == nil {
		redaction = &NoopRedactionPipeline{}
	}
	return &ExternalCliRunner{
		redaction: redaction,
	}
}
func (r *ExternalCliRunner) Execute(ctx context.Context, command *ExternalCliPreparedCommand) (*ExternalCliExecutionResult, error) {
	startedAt := time.Now().UTC()

	timeoutDuration := time.Duration(command.Preview.TimeoutSeconds) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, command.Executable, command.Arguments...)

	// 配置工作目录与环境变量
	if strings.TrimSpace(command.WorkingDirectory) != "" {
		cmd.Dir = command.WorkingDirectory
	}
	if len(command.Environment) > 0 {
		cmd.Env = os.Environ()
		for k, v := range command.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	configureSysProcAttr(cmd)

	// 重定向标准输出流
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return r.createErrorResult(command, startedAt, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return r.createErrorResult(command, startedAt, err)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return r.createErrorResult(command, startedAt, err)
	}

	// 异步读取流（带容量上限，防止因缓冲区满导致进程阻塞）
	type readResult struct {
		bytes     []byte
		truncated bool
		err       error
	}

	stdoutChan := make(chan readResult, 1)
	stderrChan := make(chan readResult, 1)

	go func() {
		b, trunc, e := readCapped(stdoutPipe, command.MaxStdoutBytes)
		stdoutChan <- readResult{b, trunc, e}
	}()

	go func() {
		b, trunc, e := readCapped(stderrPipe, command.MaxStderrBytes)
		stderrChan <- readResult{b, trunc, e}
	}()

	// 等待命令执行完毕
	waitErr := cmd.Wait()

	completedAt := time.Now().UTC()
	durationMs := float64(completedAt.Sub(startedAt).Milliseconds())

	// 获取读取结果
	stdoutRes := <-stdoutChan
	stderrRes := <-stderrChan

	// 判断是否超时
	timedOut := false
	exitCode := 0
	var errMsg string

	if waitErr != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			timedOut = true
			exitCode = -1
			errMsg = "External CLI command timed out."

			killProcessTree(cmd)
		} else {
			// 提取 Exit Code
			if exitError, ok := waitErr.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
				} else {
					exitCode = -1
				}
			} else {
				exitCode = -1
				errMsg = waitErr.Error()
			}
		}
	}

	// 处理文本脱敏
	stdoutText := r.redact(string(stdoutRes.bytes), command.RedactionRules)
	stderrText := r.redact(string(stderrRes.bytes), command.RedactionRules)

	// 解析结构化 JSON
	outputFormat := strings.ToLower(strings.TrimSpace(command.Preview.StructuredOutput))
	parsedJson, parseErr := parseStructuredOutput(outputFormat, stdoutText)

	var parseErrStr string
	if parseErr != nil {
		parseErrStr = parseErr.Error()
	}

	return &ExternalCliExecutionResult{
		Preview:         command.Preview,
		Success:         !timedOut && exitCode == 0,
		ExitCode:        exitCode,
		Stdout:          stdoutText,
		Stderr:          stderrText,
		StdoutTruncated: stdoutRes.truncated,
		StderrTruncated: stderrRes.truncated,
		TimedOut:        timedOut,
		DurationMs:      durationMs,
		StartedAtUtc:    startedAt,
		CompletedAtUtc:  completedAt,
		ParsedJson:      parsedJson,
		ParseError:      parseErrStr,
		ErrorMessage:    errMsg,
	}, nil
}

// 辅助方法：启动失败时的快速返回
func (r *ExternalCliRunner) createErrorResult(command *ExternalCliPreparedCommand, startedAt time.Time, err error) (*ExternalCliExecutionResult, error) {
	now := time.Now().UTC()
	return &ExternalCliExecutionResult{
		Preview:        command.Preview,
		Success:        false,
		ExitCode:       -1,
		StartedAtUtc:   startedAt,
		CompletedAtUtc: now,
		DurationMs:     float64(now.Sub(startedAt).Milliseconds()),
		ErrorMessage:   r.redaction.Redact(err.Error()),
	}, nil
}

// --- 核心脱敏逻辑 ---
func (r *ExternalCliRunner) redact(value string, redactionRules []string) string {
	current := r.redaction.Redact(value)

	for _, pattern := range redactionRules {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // 保持 best-effort，非法的正则直接跳过
		}
		current = re.ReplaceAllString(current, "[REDACTED:external-cli]")
	}
	return current
}

// --- 核心读取逻辑：安全读取防止内存溢出，同时消耗掉剩余流防止死锁 ---
func readCapped(reader io.Reader, maxBytes int) ([]byte, bool, error) {
	buffer := make([]byte, 8192)
	var out bytes.Buffer
	remaining := maxBytes
	truncated := false

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if remaining <= 0 {
				truncated = true
			} else if n > remaining {
				out.Write(buffer[:remaining])
				remaining = 0
				truncated = true
			} else {
				out.Write(buffer[:n])
				remaining -= n
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, false, err
		}
	}
	return out.Bytes(), truncated, nil
}

// --- 核心解析逻辑：支持 JSON 和 NDJSON ---
func parseStructuredOutput(outputFormat, stdout string) (json.RawMessage, error) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, nil
	}

	if outputFormat == "json" {
		// 验证其是否为合法的 JSON，防止把脏字符串直接当成 RawMessage 返回
		var b json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &b); err != nil {
			return nil, err
		}
		return b, nil
	}

	if outputFormat == "ndjson" {
		lines := strings.Split(stdout, "\n")

		// 用 bytes.Buffer 来高效拼接 JSON 数组：[item1,item2,...]
		var buf bytes.Buffer
		buf.WriteByte('[')

		hasRecords := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// 验证当前行是否为合法的单条 JSON
			var item json.RawMessage
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				return nil, err
			}

			// 如果不是第一条记录，中间加逗号分隔
			if hasRecords {
				buf.WriteByte(',')
			}
			buf.Write([]byte(line))
			hasRecords = true
		}
		buf.WriteByte(']')

		if !hasRecords {
			return nil, nil
		}

		return buf.Bytes(), nil
	}

	return nil, nil
}

var PlaceholderRegex = regexp.MustCompile(`\{\{\s*(?P<name>[A-Za-z_][A-Za-z0-9_\-]*)\s*\}\}`)

type ExternalCliConnectorRegistry struct {
	options   *ExternalCliOptions
	redaction IRedactionPipeline
}

func NewExternalCliConnectorRegistry(config *GatewayConfig, redaction IRedactionPipeline) *ExternalCliConnectorRegistry {
	options := ExternalCliPresetCatalogInstance.Apply(config.ExternalCli)
	return &ExternalCliConnectorRegistry{
		options:   &options,
		redaction: redaction,
	}
}

// GetCommandSchema implements [IExternalCliConnectorRegistry].
func (e *ExternalCliConnectorRegistry) GetCommandSchema(connectorName string, commandName string) (*ExternalCliCommandSchemaResponse, error) {
	connectorResolvedName, connector, err := e.getConnector(connectorName)
	if err != nil {
		return nil, err
	}
	commandResolvedName, command, err := e.getCommand(connector, commandName)
	if err != nil {
		return nil, err
	}

	parameterKeys := []string{}
	for key, v := range command.Parameters {
		if v.Required {
			parameterKeys = append(parameterKeys, key)
		}
	}

	var required = DistinctStrings(append(e.findPlaceholders(command.ArgsTemplate), parameterKeys...))
	slices.SortFunc(required, func(a, b string) int {
		return strings.Compare(a, b)
	})

	return &ExternalCliCommandSchemaResponse{
		Connector:          connectorResolvedName,
		Command:            commandResolvedName,
		Description:        command.Description,
		Parameters:         command.Parameters,
		RequiredParameters: required,
		RiskLevel:          NormalizeRiskLevel(command.RiskLevel),
		ReadOnly:           command.ReadOnly,
		RequiresApproval:   e.requiresApproval(connector, command),
		SupportsDryRun:     command.SupportsDryRun,
		StructuredOutput:   e.resolveOutputFormat(connector, command),
	}, nil
}

// ListCommands implements [IExternalCliConnectorRegistry].
func (e *ExternalCliConnectorRegistry) ListCommands(connectorName string) (*ExternalCliCommandListResponse, error) {
	name, connector, err := e.getConnector(connectorName)
	if err != nil {
		return nil, err
	}

	result := make([]ExternalCliCommandSummary, 0, len(e.options.Connectors))
	for key, command := range connector.Commands {
		summary := ExternalCliCommandSummary{
			Name:             key,
			Description:      command.Description,
			RiskLevel:        NormalizeRiskLevel(command.RiskLevel),
			ReadOnly:         command.ReadOnly,
			RequiresApproval: e.requiresApproval(connector, &command),
			SupportsDryRun:   command.SupportsDryRun,
			StructuredOutput: e.resolveOutputFormat(connector, &command),
			Tags:             command.Tags,
		}

		result = append(result, summary)
	}

	slices.SortFunc(result, func(a, b ExternalCliCommandSummary) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &ExternalCliCommandListResponse{
		Connector: name,
		Items:     result,
	}, nil
}

// ListConnectors implements [IExternalCliConnectorRegistry].
func (e *ExternalCliConnectorRegistry) ListConnectors() ([]ExternalCliConnectorSummary, error) {
	result := make([]ExternalCliConnectorSummary, 0, len(e.options.Connectors))
	for key, v := range e.options.Connectors {
		summary := ExternalCliConnectorSummary{
			Name:         key,
			DisplayName:  v.DisplayName,
			Enabled:      v.Enabled,
			Executable:   v.Executable,
			CommandCount: len(v.Commands),
		}
		if IsBlank(summary.DisplayName) {
			summary.DisplayName = key
		}

		result = append(result, summary)
	}

	slices.SortFunc(result, func(a, b ExternalCliConnectorSummary) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result, nil
}

func (e *ExternalCliConnectorRegistry) getConnector(connectorName string) (name string, connector *ExternalCliConnectorOptions, err error) {
	if IsBlank(connectorName) {
		err = errors.New("External CLI connector is required")
		return
	}

	conn, ok := e.options.Connectors[connectorName]
	if !ok {
		err = fmt.Errorf("Unknown external CLI connector '%s'", connectorName)
		return
	}
	name = connectorName
	connector = &conn
	return
}

func (e *ExternalCliConnectorRegistry) requiresApproval(connector *ExternalCliConnectorOptions, command *ExternalCliCommandOptions) bool {
	return connector.RequiresApproval || command.RequiresApproval || NormalizeRiskLevel(command.RiskLevel) == ExternalCliRiskLevelHigh || (!command.ReadOnly && e.options.RequireApprovalForMutatingCommands)
}

func (e *ExternalCliConnectorRegistry) resolveOutputFormat(connector *ExternalCliConnectorOptions, command *ExternalCliCommandOptions) string {
	if IsBlank(command.StructuredOutput) {
		return NormalizeOutputFormat(connector.DefaultOutputFormat)
	}

	return NormalizeOutputFormat(command.StructuredOutput)
}

func (e *ExternalCliConnectorRegistry) getCommand(connector *ExternalCliConnectorOptions, commandName string) (name string, command *ExternalCliCommandOptions, err error) {
	if IsBlank(commandName) {
		err = errors.New("External CLI command is required")
		return
	}

	comm, ok := connector.Commands[commandName]
	if !ok {
		err = fmt.Errorf("Unknown external CLI command '%s'", commandName)
		return
	}
	name = commandName
	command = &comm
	return
}

func (e *ExternalCliConnectorRegistry) findPlaceholders(template []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range template {
		// FindAllStringSubmatchIndex / FindAllStringSubmatch 可以获取匹配组
		matches := PlaceholderRegex.FindAllStringSubmatch(item, -1)
		for _, match := range matches {
			// match[0] 是完整匹配 ({{ name }})
			// match[1] 是第 1 个括号捕获的内容 (name)
			rawName := match[1]

			lowerName := strings.ToLower(rawName)
			if !seen[lowerName] {
				seen[lowerName] = true
				result = append(result, rawName)
			}
		}
	}

	return result
}

func getEnvIgnoreCase(env map[string]string, target string) string {
	if env == nil {
		return ""
	}
	for k, v := range env {
		if strings.EqualFold(k, target) {
			return v
		}
	}
	return ""
}

func (e *ExternalCliConnectorRegistry) resolveExecutable(executable string, environment map[string]string) string {
	if strings.TrimSpace(executable) == "" {
		return ""
	}

	if filepath.IsAbs(executable) {
		if FileExists(executable) {
			return executable
		}
		return ""
	}

	pathVal := getEnvIgnoreCase(environment, "PATH")
	if pathVal == "" {
		pathVal = os.Getenv("PATH")
	}
	if strings.TrimSpace(pathVal) == "" {
		return ""
	}

	candidates := []string{}
	if runtime.GOOS == "windows" {
		pathext := getEnvIgnoreCase(environment, "PATHEXT")
		if pathext == "" {
			pathext = os.Getenv("PATHEXT")
		}
		if strings.TrimSpace(pathext) == "" {
			pathext = ".EXE;.BAT;.CMD"
		}

		exts := strings.Split(pathext, ";")
		seen := make(map[string]bool)

		for _, ext := range exts {
			ext = strings.TrimSpace(ext)
			if ext == "" {
				continue
			}

			var candidate string
			if strings.HasSuffix(strings.ToLower(executable), strings.ToLower(ext)) {
				candidate = executable
			} else {
				candidate = executable + ext
			}

			key := strings.ToLower(candidate)
			if !seen[key] {
				seen[key] = true
				candidates = append(candidates, candidate)
			}
		}

		key := strings.ToLower(executable)
		if !seen[key] {
			seen[key] = true
			candidates = append(candidates, executable)
		}
	} else {
		candidates = []string{executable}
	}

	dirs := filepath.SplitList(pathVal)
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		for _, candidate := range candidates {
			fullPath := filepath.Join(dir, candidate)
			if FileExists(fullPath) {
				return fullPath
			}
		}
	}

	return ""
}

// GetStatus implements [IExternalCliConnectorRegistry].
func (e *ExternalCliConnectorRegistry) GetStatus(ctx context.Context, connectorName string) (*ExternalCliConnectorStatus, error) {
	name, connector, err := e.getConnector(connectorName)
	if err != nil {
		return nil, err
	}

	warnings := []string{}
	var resolvedExecutable = e.resolveExecutable(connector.Executable, connector.Environment)
	if IsBlank(resolvedExecutable) {
		warnings = append(warnings, fmt.Sprintf("Executable '%s' was not found on PATH", connector.Executable))
	}

	version := ""
	if !e.options.Enabled {
		warnings = append(warnings, "External CLI connectors are disabled.")
	}

	if !connector.Enabled {
		warnings = append(warnings, fmt.Sprintf("External CLI connector '%s' is disabled", name))
	}
	var executableForStatus = ""
	if e.options.Enabled && connector.Enabled {
		executableForStatus = resolvedExecutable
	}

	if executableForStatus != "" && connector.VersionCommand != nil && len(connector.VersionCommand.Args) > 0 {
		version, err = e.runStatusCommand(ctx, executableForStatus, connector.VersionCommand, connector, false)
		if err != nil {
			return nil, err
		}
	}

	authStatus := "unknown"
	authenticated := false
	if executableForStatus != "" && connector.StatusCommand != nil && len(connector.StatusCommand.Args) > 0 {
		statusOutput, err := e.runStatusCommand(ctx, executableForStatus, connector.StatusCommand, connector, true)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(statusOutput, "exit=0\n") {
			authenticated = true
			authStatus = "authenticated"
		} else if strings.HasPrefix(statusOutput, "exit=") {
			authenticated = false
			authStatus = "not_authenticated"
		}
	}

	displayName := connector.DisplayName
	if IsBlank(displayName) {
		displayName = name
	}
	return &ExternalCliConnectorStatus{
		Connector:              name,
		DisplayName:            displayName,
		Enabled:                connector.Enabled,
		Executable:             connector.Executable,
		ExecutableFound:        resolvedExecutable != "",
		ResolvedExecutablePath: resolvedExecutable,
		Version:                version,
		Authenticated:          &authenticated,
		AuthenticationStatus:   authStatus,
		Warnings:               warnings,
		LastCheckedAtUtc:       time.Now().UTC(),
	}, nil
}

func (e *ExternalCliConnectorRegistry) runStatusCommand(parentCtx context.Context, executable string, command *ExternalCliStatusCommandOptions, connector *ExternalCliConnectorOptions, captureExitCode bool) (string, error) {
	timeoutSec := e.options.DefaultTimeoutSeconds
	if timeoutSec > 20 {
		timeoutSec = 20
	}
	if command.TimeoutSeconds > 0 {
		timeoutSec = command.TimeoutSeconds
	}
	if timeoutSec < 1 {
		timeoutSec = 1
	} else if timeoutSec > 120 {
		timeoutSec = 120
	}

	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := append([]string{}, connector.DefaultArgs...)
	args = append(args, command.Args...)

	cmd := exec.CommandContext(ctx, executable, args...)
	workDir, err := e.resolveWorkingDirectory(connector.WorkingDirectory)
	if err != nil {
		return "", err
	}
	cmd.Dir = workDir

	if len(connector.Environment) > 0 {
		for k, v := range connector.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) && !errors.Is(parentCtx.Err(), context.Canceled) {
		if captureExitCode {
			return "exit=-1\ntimeout", nil
		}
		return "timeout", nil
	}
	outStr := strings.TrimSpace(stdout.String())
	if outStr == "" {
		outStr = strings.TrimSpace(stderr.String())
	}

	outStr = e.redact(outStr, connector.RedactionRules)

	if captureExitCode {
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		return fmt.Sprintf("exit=%d\n%s", exitCode, outStr), nil
	}

	return outStr, nil
}

func (s *ExternalCliConnectorRegistry) redact(value string, redactionRules []string) string {
	if !s.options.RedactSecrets {
		return value
	}

	current := s.redaction.Redact(value)
	for _, pattern := range redactionRules {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		current = re.ReplaceAllString(current, "[REDACTED:external-cli]")
	}

	return current
}

func (e *ExternalCliConnectorRegistry) resolveWorkingDirectory(configured string) (string, error) {
	if strings.TrimSpace(configured) == "" {
		return "", nil
	}
	expanded := os.ExpandEnv(configured)

	if expanded == "~" || strings.HasPrefix(expanded, "~/") || strings.HasPrefix(expanded, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		if expanded == "~" {
			expanded = homeDir
		} else {
			expanded = filepath.Join(homeDir, expanded[2:])
		}
	}

	fullPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for '%s': %w", configured, err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("external CLI working directory '%s' does not exist", configured)
		}
		return "", fmt.Errorf("failed to check working directory '%s': %w", configured, err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("external CLI working directory '%s' is a file, not a directory", configured)
	}

	return fullPath, nil
}

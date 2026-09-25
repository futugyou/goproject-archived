package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/openclaw/core"
)

type TurnRoutingRequest struct {
	Session     *core.Session
	UserMessage string
	Messages    []chatcompletion.ChatMessage
	BaseOptions chatcompletion.ChatOptions
}

type TurnRoutingDecision struct {
	Tier                                string
	ModelProfileId                      string
	DirectModelFallbackProfileId        string
	DisableTools                        bool
	AllowedTools                        []string
	PreferredTags                       []string
	ReasoningLevel                      string
	ResponsePolicy                      string
	ImageCapableModelProfileId          string
	CacheContinuitySafeguardsEnabled    bool
	CacheContinuityMaxConversationTurns int
	CacheContinuityResetOnProfileSwitch bool
	SystemPromptSuffix                  string
	Reason                              string
}

type ITurnRoutingPolicy interface {
	Resolve(ctx context.Context, request TurnRoutingRequest) (*TurnRoutingDecision, error)
}

type NoopTurnRoutingPolicy struct{}

func (n *NoopTurnRoutingPolicy) Resolve(ctx context.Context, request TurnRoutingRequest) (*TurnRoutingDecision, error) {
	return &TurnRoutingDecision{}, nil
}

var (
	debugKeywords        = []string{"error", "bug", "exception", "traceback", "failed", "root cause", "报错", "根因", "修复", "stack trace", "debug"}
	researchKeywords     = []string{"调研", "research", "对比", "compare", "survey", "分析报告", "competitive analysis", "综述"}
	architectureKeywords = []string{"architecture", "架构", "重构", "refactor", "monorepo", "codebase", "module", "dependency"}
	compareKeywords      = []string{"对比", "compare", "audit", "审计", "review", "评估"}
	planningKeywords     = []string{"plan", "planning", "方案", "计划", "roadmap", "milestone", "步骤", "实施"}
	strictFormatKeywords = []string{"json", "yaml", "csv", "schema", "只返回", "不要解释", "按格式", "only return", "no explanation"}
	highRiskKeywords     = []string{"deploy", "rollback", "migration", "delete", "overwrite", "覆盖", "production", "生产", "部署", "删除", "客户", "法务", "财务"}
	productionKeywords   = []string{"production", "生产", "prod", "线上", "正式环境"}
	customerKeywords     = []string{"customer", "客户", "用户邮件", "client"}
	deleteKeywords       = []string{"delete", "remove", "drop", "truncate", "删除", "清空", "覆盖", "overwrite"}
	formalKeywords       = []string{"formal", "正式", "official", "公文", "合同", "法律"}
	constraintKeywords   = []string{"必须", "不能", "不要", "只能", "must", "shall", "required", "forbidden", "不允许", "至少", "最多"}
	teachingKeywords     = []string{"how does", "explain", "what is", "why does", "how to", "教我", "解释", "为什么", "怎么", "是什么", "how can", "tell me about", "walk me through", "介绍", "说明"}
	implementKeywords    = []string{"implement", "write function", "write a", "create a", "写个", "实现", "用法", "帮我写", "生成代码", "add a", "build a", "make a", "写一个", "编写"}
	complaintKeywords    = []string{"不对", "太泛了", "重新写", "wrong", "too vague", "redo", "try again", "not right"}
)

// Compiled regular expressions matching C#'s GeneratedRegex attributes.
var (
	codeBlockRegex = regexp.MustCompile("(?m)```[\\s\\S]*?```")
	filePathRegex  = regexp.MustCompile(`(?m)(?:^|[\s"'` + "`" + `(])([a-zA-Z_][\w.-]*/[\w./-]+\.[\w]+)`)
	urlRegex       = regexp.MustCompile(`(?m)https?://\S+`)
	tracebackRegex = regexp.MustCompile(`(?m)Traceback \(most recent|stderr:|\.py", line \d+`)
)

// TurnRoutingSignals holds extracted turn classification signals.
type TurnRoutingSignals struct {
	Debug            bool
	RepoArch         bool
	HighRisk         bool
	LongContext      bool
	StrictFormat     bool
	HasCodeBlock     bool
	HasFileReference bool
	HasUrl           bool
	DeepConversation bool
	Research         bool
	Planning         bool
}

// ExtractSignals analyzes input text and turn index to construct routing signals.
func ExtractSignals(text string, turnIndex int) TurnRoutingSignals {
	hasCodeBlock := codeBlockRegex.MatchString(text)
	hasFileReference := filePathRegex.MatchString(text)
	hasUrl := urlRegex.MatchString(text)

	// Calculate total length of code block matches
	codeBlocks := codeBlockRegex.FindAllString(text, -1)
	codeBlockTotalLen := 0
	for _, block := range codeBlocks {
		codeBlockTotalLen += len(block)
	}

	fileMatches := filePathRegex.FindAllString(text, -1)

	longContext := len(text) >= 6000 || codeBlockTotalLen >= 1500 || len(fileMatches) >= 2
	debug := keywordCount(text, debugKeywords) > 0 || tracebackRegex.MatchString(text)
	repoArch := keywordCount(text, architectureKeywords) > 0 || keywordCount(text, compareKeywords) > 0

	prodCount := keywordCount(text, productionKeywords)
	deleteCount := keywordCount(text, deleteKeywords)
	highRisk := keywordCount(text, highRiskKeywords) > 0 ||
		(prodCount > 0 && deleteCount > 0) ||
		keywordCount(text, customerKeywords) > 0 ||
		(prodCount > 0 && debug)

	strictFormat := keywordCount(text, strictFormatKeywords) > 0 || keywordCount(text, constraintKeywords) > 1
	deepConversation := turnIndex >= 4
	research := keywordCount(text, researchKeywords) > 0
	planning := keywordCount(text, planningKeywords) > 0

	return TurnRoutingSignals{
		Debug:            debug,
		RepoArch:         repoArch,
		HighRisk:         highRisk,
		LongContext:      longContext,
		StrictFormat:     strictFormat,
		HasCodeBlock:     hasCodeBlock,
		HasFileReference: hasFileReference,
		HasUrl:           hasUrl,
		DeepConversation: deepConversation,
		Research:         research,
		Planning:         planning,
	}
}

// ApplyFlagOverrides escalates routing tier based on extracted signals.
func ApplyFlagOverrides(tier int, signals TurnRoutingSignals) int {
	result := tier
	if signals.HighRisk {
		result = max(result, 2)
	}
	if signals.Debug && signals.LongContext {
		result = max(result, 2)
	}
	if (signals.Research && signals.Planning) || (signals.RepoArch && signals.Planning) {
		result = max(result, 3)
	}
	if signals.RepoArch {
		result = max(result, 1)
	}
	if signals.Planning {
		result = max(result, 1)
	}
	return result
}

// ApplyContextRule adjusts tier when conversation reaches a specific turn index depth.
func ApplyContextRule(tier int, turnIndex int, turnIndexThreshold int) int {
	if turnIndex >= turnIndexThreshold {
		return max(tier, 1)
	}
	return tier
}

// ApplyStickyTier enforces tier monotonicity using previous tier history.
func ApplyStickyTier(tier int, previousTier string, enabled bool) int {
	if !enabled {
		return tier
	}

	previous := -1
	switch strings.ToUpper(strings.TrimSpace(previousTier)) {
	case "T0":
		previous = 0
	case "T1":
		previous = 1
	case "T2":
		previous = 2
	case "T3":
		previous = 3
	}

	if previous > tier {
		return previous
	}
	return tier
}

// Helper functions

func keywordCount(text string, keywords []string) int {
	lowerText := strings.ToLower(text)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

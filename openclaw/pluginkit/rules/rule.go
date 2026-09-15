package rules

type TokenJuiceRule struct {
	ID            string               `json:"id"`
	Family        string               `json:"family"`
	Description   *string              `json:"description,omitempty"`
	Priority      int                  `json:"priority"`
	Match         *RuleMatchBlock      `json:"match,omitempty"`
	Transforms    *RuleTransformsBlock `json:"transforms,omitempty"`
	Summarize     *RuleSummarizeBlock  `json:"summarize,omitempty"`
	Failure       *RuleFailureBlock    `json:"failure,omitempty"`
	Counters      []RuleCounter        `json:"counters,omitempty"`
	Filters       *RuleFiltersBlock    `json:"filters,omitempty"`
	OutputMatches []RuleOutputMatch    `json:"outputMatches,omitempty"`
	OnEmpty       *string              `json:"onEmpty,omitempty"`
	CounterSource string               `json:"counterSource,omitempty"`
}

func NewTokenJuiceRule() TokenJuiceRule {
	return TokenJuiceRule{
		Family: "generic",
	}
}

type RuleMatchBlock struct {
	ToolNames          []string   `json:"toolNames,omitempty"`
	Argv0              []string   `json:"argv0,omitempty"`
	ArgvIncludes       [][]string `json:"argvIncludes,omitempty"`
	ArgvIncludesAny    [][]string `json:"argvIncludesAny,omitempty"`
	CommandIncludes    []string   `json:"commandIncludes,omitempty"`
	CommandIncludesAny []string   `json:"commandIncludesAny,omitempty"`
	CommandRegex       *string    `json:"commandRegex,omitempty"`
	ExitCodes          []int      `json:"exitCodes,omitempty"`
	OutputRegex        *string    `json:"outputRegex,omitempty"`
}

type RuleTransformsBlock struct {
	StripAnsi      bool `json:"stripAnsi"`
	DedupeAdjacent bool `json:"dedupeAdjacent"`
	TrimEmptyEdges bool `json:"trimEmptyEdges"`
}

type RuleSummarizeBlock struct {
	Head int `json:"head"`
	Tail int `json:"tail"`
}

func NewRuleSummarizeBlock() RuleSummarizeBlock {
	return RuleSummarizeBlock{Head: 8, Tail: 8}
}

type RuleFailureBlock struct {
	PreserveOnFailure bool `json:"preserveOnFailure"`
	Head              int  `json:"head"`
	Tail              int  `json:"tail"`
}

func NewRuleFailureBlock() RuleFailureBlock {
	return RuleFailureBlock{Head: 12, Tail: 12}
}

type RuleCounter struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Flags   string `json:"flags,omitempty"`
}

type RuleFiltersBlock struct {
	SkipPatterns []string `json:"skipPatterns,omitempty"`
	KeepPatterns []string `json:"keepPatterns,omitempty"`
}

type RuleOutputMatch struct {
	Pattern string `json:"pattern"`
	Message string `json:"message"`
}

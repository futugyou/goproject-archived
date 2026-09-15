package reduction

import (
	"regexp"
	"strings"

	"github.com/futugyou/openclaw/pluginkit/formatters"
	"github.com/futugyou/openclaw/pluginkit/rules"
)

func Reduce(rule rules.TokenJuiceRule, rawText string, exitCode int) (string, map[string]int) {
	text := rawText

	// Step 1: Strip ANSI
	if rule.Transforms != nil && rule.Transforms.StripAnsi {
		text = formatters.StripAnsi(text)
	}

	// Step 2: OutputMatches
	if len(rule.OutputMatches) > 0 {
		for _, om := range rule.OutputMatches {
			re, err := regexp.Compile("(?m)" + om.Pattern) // (?m) 启用多行匹配
			if err == nil && re.MatchString(text) {
				return om.Message, make(map[string]int)
			}
		}
	}

	// Step 3: Split into lines
	linesRaw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, len(linesRaw))
	copy(lines, linesRaw)

	// Step 4: Trim empty edges
	if rule.Transforms != nil && rule.Transforms.TrimEmptyEdges {
		lines = formatters.TrimEmptyEdges(lines)
	}

	// Step 5: Dedupe adjacent
	if rule.Transforms != nil && rule.Transforms.DedupeAdjacent {
		lines = formatters.DedupeAdjacent(lines)
	}

	// Step 6: Apply skip/keep filters
	counterLines := make([]string, len(lines))
	copy(counterLines, lines)

	if rule.Filters != nil && len(rule.Filters.SkipPatterns) > 0 {
		var compiled []*regexp.Regexp
		for _, p := range rule.Filters.SkipPatterns {
			if r, err := regexp.Compile(p); err == nil {
				compiled = append(compiled, r)
			}
		}

		if len(compiled) > 0 {
			var filtered []string
			for _, line := range lines {
				skip := false
				for _, r := range compiled {
					if r.MatchString(line) {
						skip = true
						break
					}
				}
				if !skip {
					filtered = append(filtered, line)
				}
			}
			lines = filtered
		}
	}

	if rule.Filters != nil && len(rule.Filters.KeepPatterns) > 0 {
		var compiled []*regexp.Regexp
		for _, p := range rule.Filters.KeepPatterns {
			if r, err := regexp.Compile(p); err == nil {
				compiled = append(compiled, r)
			}
		}

		if len(compiled) > 0 {
			var kept []string
			for _, line := range lines {
				for _, r := range compiled {
					if r.MatchString(line) {
						kept = append(kept, line)
						break
					}
				}
			}
			if len(kept) > 0 {
				lines = kept
			}
		}
	}

	// Step 7: onEmpty
	if len(lines) == 0 && rule.OnEmpty != nil {
		return *rule.OnEmpty, make(map[string]int)
	}

	// Step 8: Counters
	facts := make(map[string]int)
	if len(rule.Counters) > 0 {
		source := lines
		if rule.CounterSource == "preKeep" {
			source = counterLines
		}

		for _, counter := range rule.Counters {
			if counter.Pattern == "" {
				continue
			}
			re, err := formatters.CompilePattern(counter.Pattern, counter.Flags)
			if err == nil {
				facts[counter.Name] = formatters.CountPattern(source, re)
			}
		}
	}

	// Step 9: Head/Tail summarization
	head := 8
	tail := 8

	if rule.Summarize != nil {
		if rule.Summarize.Head > 0 {
			head = rule.Summarize.Head
		}
		if rule.Summarize.Tail > 0 {
			tail = rule.Summarize.Tail
		}
	}

	if exitCode != 0 && rule.Failure != nil && rule.Failure.PreserveOnFailure {
		if rule.Failure.Head > 0 {
			head = rule.Failure.Head
		} else {
			head = 12
		}

		if rule.Failure.Tail > 0 {
			tail = rule.Failure.Tail
		} else {
			tail = 12
		}
	}

	compacted := formatters.HeadTail(lines, head, tail)
	return strings.TrimSpace(strings.Join(compacted, "\n")), facts
}

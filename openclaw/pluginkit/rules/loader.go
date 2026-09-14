package rules

import (
	"embed"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed */*.json
var embeddedRules embed.FS

type RuleLoader struct {
	userRulesDir string
	projectCache sync.Map
}

func NewRuleLoader() *RuleLoader {
	homeDir, err := os.UserHomeDir()
	userRulesDir := ""
	if err == nil {
		userRulesDir = filepath.Join(homeDir, ".config", "tokenjuice", "rules")
	}
	return &RuleLoader{
		userRulesDir: userRulesDir,
	}
}

func (rl *RuleLoader) LoadMergedRules(projectRoot *string) []TokenJuiceRule {
	if projectRoot != nil && *projectRoot != "" {
		if val, ok := rl.projectCache.Load(*projectRoot); ok {
			return val.([]TokenJuiceRule)
		}
		rules := rl.loadMergedInternal(projectRoot)
		rl.projectCache.Store(*projectRoot, rules)
		return rules
	}

	return rl.loadMergedInternal(nil)
}

func (rl *RuleLoader) loadMergedInternal(projectRoot *string) []TokenJuiceRule {
	merged := make(map[string]TokenJuiceRule)

	for _, rule := range rl.loadBuiltinRules() {
		merged[rule.ID] = rule
	}

	if rl.userRulesDir != "" {
		if info, err := os.Stat(rl.userRulesDir); err == nil && info.IsDir() {
			for _, rule := range rl.loadFromDirectory(rl.userRulesDir) {
				merged[rule.ID] = rule
			}
		}
	}

	if projectRoot != nil && *projectRoot != "" {
		projectDir := filepath.Join(*projectRoot, ".tokenjuice", "rules")
		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			for _, rule := range rl.loadFromDirectory(projectDir) {
				merged[rule.ID] = rule
			}
		}
	}

	result := make([]TokenJuiceRule, 0, len(merged))
	for _, rule := range merged {
		result = append(result, rule)
	}

	sortRules(result)
	return result
}

func (rl *RuleLoader) loadBuiltinRules() []TokenJuiceRule {
	var rules []TokenJuiceRule

	// 遍历 embedded FS 读取规则文件
	var walkFS func(path string)
	walkFS = func(path string) {
		entries, err := embeddedRules.ReadDir(path)
		if err != nil {
			return
		}

		for _, entry := range entries {
			fullPath := path
			if fullPath != "" && fullPath != "." {
				fullPath += "/" + entry.Name()
			} else {
				fullPath = entry.Name()
			}

			if entry.IsDir() {
				walkFS(fullPath)
				continue
			}

			if !strings.Contains(strings.ToLower(fullPath), "tokenjuice") {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(fullPath), ".json") {
				continue
			}

			file, err := embeddedRules.Open(fullPath)
			if err != nil {
				continue
			}

			data, err := io.ReadAll(file.(io.Reader))
			if err != nil {
				continue
			}

			rule := NewTokenJuiceRule()
			if err := json.Unmarshal(data, &rule); err == nil && rule.ID != "" {
				rules = append(rules, rule)
			}
		}
	}

	walkFS(".")
	sortRules(rules)
	return rules
}

func (rl *RuleLoader) loadFromDirectory(dir string) []TokenJuiceRule {
	var rules []TokenJuiceRule

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".json") {
			return nil
		}

		normalizedPath := strings.ReplaceAll(path, string(os.PathSeparator), "/")
		if strings.Contains(normalizedPath, "/fixtures/") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		rule := NewTokenJuiceRule()
		if err := json.Unmarshal(data, &rule); err == nil && rule.ID != "" {
			rules = append(rules, rule)
		}

		return nil
	})

	return rules
}

func sortRules(rules []TokenJuiceRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		iIsFallback := rules[i].ID == "generic/fallback"
		jIsFallback := rules[j].ID == "generic/fallback"

		if iIsFallback != jIsFallback {
			return !iIsFallback // 非 fallback 优先 (排序在前)
		}
		return rules[i].Priority > rules[j].Priority
	})
}

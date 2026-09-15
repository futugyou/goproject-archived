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

var projectCache sync.Map

var userRulesDir = func() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "tokenjuice", "rules")
}()

func LoadMergedRules(projectRoot *string) []TokenJuiceRule {
	if projectRoot != nil && *projectRoot != "" {
		if val, ok := projectCache.Load(*projectRoot); ok {
			return val.([]TokenJuiceRule)
		}
		rules := loadMergedInternal(projectRoot)
		projectCache.Store(*projectRoot, rules)
		return rules
	}

	return loadMergedInternal(nil)
}

func loadMergedInternal(projectRoot *string) []TokenJuiceRule {
	merged := make(map[string]TokenJuiceRule)

	for _, rule := range loadBuiltinRules() {
		merged[rule.ID] = rule
	}

	if userRulesDir != "" {
		if info, err := os.Stat(userRulesDir); err == nil && info.IsDir() {
			for _, rule := range loadFromDirectory(userRulesDir) {
				merged[rule.ID] = rule
			}
		}
	}

	if projectRoot != nil && *projectRoot != "" {
		projectDir := filepath.Join(*projectRoot, ".tokenjuice", "rules")
		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			for _, rule := range loadFromDirectory(projectDir) {
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

func loadBuiltinRules() []TokenJuiceRule {
	var rules []TokenJuiceRule

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

func loadFromDirectory(dir string) []TokenJuiceRule {
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
			return !iIsFallback
		}
		return rules[i].Priority > rules[j].Priority
	})
}

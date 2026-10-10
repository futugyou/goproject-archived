package primitives

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type JsonScenarioLoader struct{}

func NewJsonScenarioLoader() *JsonScenarioLoader {
	return &JsonScenarioLoader{}
}

func (l *JsonScenarioLoader) Load(ctx context.Context, directoryPath string) ([]AgentScenario, error) {
	info, err := os.Stat(directoryPath)
	if err != nil || !info.IsDir() {
		return []AgentScenario{}, nil
	}

	var files []string
	err = filepath.WalkDir(directoryPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	var scenarios []AgentScenario

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		var raw json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		var list []AgentScenario
		if err := json.Unmarshal(raw, &list); err == nil {
			for i := range list {
				copyAgentScenario(&list[i])
			}
			scenarios = append(scenarios, list...)
			continue
		}

		var single AgentScenario
		if err := json.Unmarshal(raw, &single); err == nil {
			copyAgentScenario(&single)
			scenarios = append(scenarios, single)
		}
	}

	return scenarios, nil
}

func copyAgentScenario(s *AgentScenario) {
	if s.Type == "" {
		s.Type = "agent"
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	s.Metadata = ensureMap(s.Metadata)

	// Input
	if s.Input.Context == nil {
		s.Input.Context = []string{}
	}
	s.Input.Variables = ensureMap(s.Input.Variables)

	// Expected
	if s.Expected.MustCallTools == nil {
		s.Expected.MustCallTools = []string{}
	}
	if s.Expected.MustNotCallTools == nil {
		s.Expected.MustNotCallTools = []string{}
	}
	if s.Expected.FinalAnswerContains == nil {
		s.Expected.FinalAnswerContains = []string{}
	}
	if s.Expected.FinalAnswerMustNotContain == nil {
		s.Expected.FinalAnswerMustNotContain = []string{}
	}
	if s.Expected.ExpectedEvents == nil {
		s.Expected.ExpectedEvents = []string{}
	}
	if s.Expected.ForbiddenEvents == nil {
		s.Expected.ForbiddenEvents = []string{}
	}
	s.Expected.ExpectedState = ensureMap(s.Expected.ExpectedState)

	// Oracles
	if s.Oracles == nil {
		s.Oracles = []ScenarioOracleDefinition{}
	} else {
		for i := range s.Oracles {
			if s.Oracles[i].Tools == nil {
				s.Oracles[i].Tools = []string{}
			}
			s.Oracles[i].Metadata = ensureMap(s.Oracles[i].Metadata)
		}
	}

	// ScriptedTrace
	if s.Scripted != nil {
		if s.Scripted.Status == "" {
			s.Scripted.Status = ScenarioRunStatusesCompleted
		}
		if s.Scripted.Steps == nil {
			s.Scripted.Steps = []TraceStep{}
		} else {
			for i := range s.Scripted.Steps {
				step := &s.Scripted.Steps[i]
				if step.Kind == "" {
					step.Kind = TraceStepKindsEvent
				}
				step.Metadata = ensureMap(step.Metadata)
			}
		}
		s.Scripted.Metadata = ensureMap(s.Scripted.Metadata)
	}
}

func ensureMap(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

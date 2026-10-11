package primitives

import (
	"strings"
)

type ScenarioOracleRegistry struct {
	factories map[string]func(ScenarioOracleDefinition) IScenarioOracle
}

func NewScenarioOracleRegistry(unsafeTools []string) *ScenarioOracleRegistry {
	if len(unsafeTools) == 0 {
		unsafeTools = DefaultUnsafeTools
	}

	effectiveUnsafeToolsMap := map[string]struct{}{}
	effectiveUnsafeTools := []string{}
	for _, s := range unsafeTools {
		s = strings.TrimSpace(s)
		if s != "" {
			if _, ok := effectiveUnsafeToolsMap[s]; !ok {
				effectiveUnsafeToolsMap[s] = struct{}{}
				effectiveUnsafeTools = append(effectiveUnsafeTools, s)
			}
		}
	}

	factories := map[string]func(ScenarioOracleDefinition) IScenarioOracle{
		ScenarioOracleTypesToolCalled:    func(definition ScenarioOracleDefinition) IScenarioOracle { return NewToolCalledOracle(definition) },
		ScenarioOracleTypesToolNotCalled: func(definition ScenarioOracleDefinition) IScenarioOracle { return NewToolNotCalledOracle(definition) },
		ScenarioOracleTypesMaxToolCalls:  func(definition ScenarioOracleDefinition) IScenarioOracle { return NewMaxToolCallsOracle(definition) },
		ScenarioOracleTypesFinalAnswerContains: func(definition ScenarioOracleDefinition) IScenarioOracle {
			return NewFinalAnswerContainsOracle(definition)
		},
		ScenarioOracleTypesFinalAnswerNotContains: func(definition ScenarioOracleDefinition) IScenarioOracle {
			return NewFinalAnswerNotContainsOracle(definition)
		},
		ScenarioOracleTypesApprovalRequired: func(definition ScenarioOracleDefinition) IScenarioOracle {
			return NewApprovalRequiredOracle(definition)
		},
		ScenarioOracleTypesApprovalNotRequired: func(definition ScenarioOracleDefinition) IScenarioOracle {
			return NewApprovalNotRequiredOracle(definition)
		},
		ScenarioOracleTypesNoUnsafeTool: func(definition ScenarioOracleDefinition) IScenarioOracle {
			return NewNoUnsafeToolOracle(definition, effectiveUnsafeTools)
		},
	}

	return &ScenarioOracleRegistry{factories: factories}
}

func (s *ScenarioOracleRegistry) Create(definition ScenarioOracleDefinition) IScenarioOracle {
	if factory, ok := s.factories[definition.Type]; ok {
		return factory(definition)
	}

	return NewUnknownScenarioOracle(definition.Type)
}

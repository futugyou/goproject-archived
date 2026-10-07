package primitives

import (
	"context"
	"slices"

	"github.com/futugyou/openclaw/core"
)

type ToolApprovalPolicyScenario struct {
	BaseScenario
}

func NewToolApprovalPolicyScenario() *ToolApprovalPolicyScenario {
	s := &ToolApprovalPolicyScenario{
		BaseScenario: NewBaseScenario("approvals.tool_approval_policy", "Tool approval policy", HarnessRegressionCategoryApprovals, true),
	}
	s.SetEvaluator(s)
	return s
}

var HighRiskTools = []string{"shell", "code_exec", "git", "external_cli", "payment"}

func (s *ToolApprovalPolicyScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}
	toolGovernanceDescriptorCatalog := core.NewToolGovernanceDescriptorCatalog()
	missing := []string{}
	for _, tool := range HighRiskTools {
		approvalDescriptor := core.ToolActionPolicyResolverInstance.Resolve(tool, "{}")
		descriptor := toolGovernanceDescriptorCatalog.Resolve(tool, "", approvalDescriptor)
		if !descriptor.RequiresApproval {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return s.Failed("High-risk tool descriptors no longer require approval.", HarnessRegressionScenarioTextJoin(missing), ""), nil
	}

	supervised := core.GatewayConfig{
		Tooling: core.ToolingConfig{
			AutonomyMode:          "supervised",
			RequireToolApproval:   true,
			ApprovalRequiredTools: []string{"shell", "write_file", "code_exec", "git", "external_cli", "payment"},
		},
	}

	configuredMissing := []string{}
	for _, tool := range HighRiskTools {
		if !slices.Contains(supervised.Tooling.ApprovalRequiredTools, tool) {
			configuredMissing = append(configuredMissing, tool)
		}
	}

	if len(configuredMissing) > 0 {
		return s.Failed("Synthetic supervised approval policy does not cover high-risk tool(s).", HarnessRegressionScenarioTextJoin(configuredMissing), ""), nil
	}

	if regCtx.Config != nil {
		if regCtx.Config.Tooling.AutonomyMode == "supervised" && !regCtx.Config.Tooling.RequireToolApproval {
			return s.Warning("High-risk tool descriptors require approval, but the loaded supervised config has RequireToolApproval=false.", "Run openclaw admin approvals simulate --tool shell to inspect effective runtime policy."), nil
		}
	}
	return s.Passed("High-risk tool descriptors and supervised approval policy remain approval-first.", ""), nil
}

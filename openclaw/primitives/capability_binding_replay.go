package primitives

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/capability"
	"github.com/futugyou/openclaw/core"

	"github.com/futugyou/openclaw/agent"
)

// CapabilityBindingReplayFixture

type CapabilityBindingReplayFixture struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Recorded      core.CapabilityBindingTrajectory `json:"recorded"`
	SessionId     string                           `json:"sessionId"`
	Expected      core.CapabilityBindingTrajectory `json:"expected"`
}

func NewCapabilityBindingReplayFixture() *CapabilityBindingReplayFixture {
	return &CapabilityBindingReplayFixture{
		SchemaVersion: 2,
		Recorded:      core.CapabilityBindingTrajectory{},
		Expected:      core.CapabilityBindingTrajectory{},
	}
}

func FromMetaRun(run *core.SessionMetaRunRecord, sessionId string) (*CapabilityBindingReplayFixture, error) {
	if run == nil {
		return nil, fmt.Errorf("run record cannot be nil")
	}

	var targetBinding *core.CapabilityBindingTrajectory
	for _, result := range run.StepResults {
		if result.ExecutionEvidence != nil && result.ExecutionEvidence.CapabilityBinding != nil {
			targetBinding = result.ExecutionEvidence.CapabilityBinding
			break
		}
	}

	if targetBinding == nil {
		return nil, fmt.Errorf("the exported meta-run carries no capability binding trajectory")
	}

	expected, err := cloneTrajectory(targetBinding)
	if err != nil {
		return nil, fmt.Errorf("failed to clone expected trajectory: %w", err)
	}

	recorded, err := cloneTrajectory(targetBinding)
	if err != nil {
		return nil, fmt.Errorf("failed to clone recorded trajectory: %w", err)
	}

	return &CapabilityBindingReplayFixture{
		SchemaVersion: 2,
		SessionId:     sessionId,
		Expected:      *expected,
		Recorded:      *recorded,
	}, nil
}

func cloneTrajectory(value *core.CapabilityBindingTrajectory) (*core.CapabilityBindingTrajectory, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned core.CapabilityBindingTrajectory
	if err := json.Unmarshal(bytes, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

// CapabilityBindingReplayResult

type CapabilityBindingReplayResult struct {
	Passed     bool                              `json:"passed"`
	Message    string                            `json:"message"`
	Reproduced *core.CapabilityBindingTrajectory `json:"reproduced"`
}

// CapabilityBindingReplay

type CapabilityBindingReplay struct {
	fixture *CapabilityBindingReplayFixture
}

func NewCapabilityBindingReplay(fixture *CapabilityBindingReplayFixture) (*CapabilityBindingReplay, error) {
	if fixture == nil {
		return nil, fmt.Errorf("fixture cannot be nil")
	}
	if fixture.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported replay fixture schema version: %d", fixture.SchemaVersion)
	}
	if fixture.Expected.Binding != "static" && fixture.Expected.Binding != "dynamic" {
		return nil, fmt.Errorf("replay fixture binding must be static or dynamic")
	}
	if fixture.Expected.Binding == "static" && (fixture.Expected.Server == nil || fixture.Expected.Tool == nil) {
		return nil, fmt.Errorf("a static replay fixture must carry the recorded server and tool")
	}
	if fixture.Expected.Binding == "dynamic" && (fixture.Expected.TaskDescription == nil || strings.TrimSpace(*fixture.Expected.TaskDescription) == "") {
		return nil, fmt.Errorf("a dynamic replay fixture must carry the recorded task description")
	}

	return &CapabilityBindingReplay{fixture: fixture}, nil
}

func (r *CapabilityBindingReplay) Run(ctx context.Context) (*CapabilityBindingReplayResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	provider := newRecordedProvider(&r.fixture.Recorded)
	registry := capability.NewCapabilityProviderRegistry([]core.ICapabilityProvider{provider}, r.fixture.Recorded.Provider)
	executor := agent.NewCapabilitySlotExecutor(*registry, capability.NewCapabilityBindingCache(0, 0))

	noExecution := func(ctx context.Context, tool core.ITool, args string) (*agent.ToolExecutionResult, error) {
		return &agent.ToolExecutionResult{
			Invocation: &core.ToolInvocation{
				ToolName:  tool.Name(),
				Arguments: args,
				Result:    "replay",
			},
			ResultText: "replay",
		}, nil
	}

	capRef := r.buildCapabilityRef()

	// 模拟缓存命中（若 recorded 中包含缓存命中标识）
	if r.fixture.Recorded.CacheHit {
		executor.Execute(ctx, capRef, "{}", r.fixture.SessionId, noExecution, "", nil)
	}

	result := executor.Execute(ctx, capRef, "{}", r.fixture.SessionId, noExecution, "", nil)

	reproduced := result.BindingTrajectory
	if reproduced == nil {
		return &CapabilityBindingReplayResult{
			Passed:  false,
			Message: "Replayed slot produced no binding trajectory.",
		}, nil
	}

	mismatches := make([]string, 0)
	if reproduced.IntentKey != r.fixture.Expected.IntentKey {
		mismatches = append(mismatches, "intentKey")
	}
	if reproduced.SelectionPolicy != r.fixture.Expected.SelectionPolicy {
		mismatches = append(mismatches, "selectionPolicy")
	}
	if reproduced.Provider != r.fixture.Expected.Provider {
		mismatches = append(mismatches, "provider")
	}
	if reproduced.SchemaFingerprint != r.fixture.Expected.SchemaFingerprint {
		mismatches = append(mismatches, "schemaFingerprint")
	}
	if reproduced.Binding != r.fixture.Expected.Binding {
		mismatches = append(mismatches, "binding")
	}
	if !derefEqual(reproduced.Server, r.fixture.Expected.Server) {
		mismatches = append(mismatches, "server")
	}
	if !derefEqual(reproduced.Tool, r.fixture.Expected.Tool) {
		mismatches = append(mismatches, "tool")
	}
	if reproduced.CacheHit != r.fixture.Expected.CacheHit {
		mismatches = append(mismatches, "cacheHit")
	}
	if !candidatesEqual(reproduced.Candidates, r.fixture.Expected.Candidates) {
		mismatches = append(mismatches, "candidates")
	}
	if !candidatesEqual(reproduced.Attempted, r.fixture.Expected.Attempted) {
		mismatches = append(mismatches, "attempted")
	}

	if len(mismatches) == 0 {
		return &CapabilityBindingReplayResult{
			Passed:     true,
			Message:    "Recorded binding reproduced.",
			Reproduced: reproduced,
		}, nil
	}

	return &CapabilityBindingReplayResult{
		Passed:     false,
		Reproduced: reproduced,
		Message:    fmt.Sprintf("Binding diverged from the recorded trajectory: %s.", strings.Join(mismatches, ", ")),
	}, nil
}

func (r *CapabilityBindingReplay) buildCapabilityRef() *core.MetaCapabilityRefDefinition {
	expected := r.fixture.Recorded
	if expected.Binding == "static" {
		server := ""
		if expected.Server != nil {
			server = *expected.Server
		}
		tool := ""
		if expected.Tool != nil {
			tool = *expected.Tool
		}

		return &core.MetaCapabilityRefDefinition{
			Provider: expected.Provider,
			Binding:  "static",
			Static: &core.MetaCapabilityStaticBinding{
				Target:   server,
				ToolName: tool,
			},
		}
	}

	taskDesc := ""
	if expected.TaskDescription != nil {
		taskDesc = *expected.TaskDescription
	}

	keywords := []string{}
	if expected.KeyWords != nil && *expected.KeyWords != "" {
		rawWords := strings.SplitSeq(*expected.KeyWords, ",")
		for w := range rawWords {
			trimmed := strings.TrimSpace(w)
			if trimmed != "" {
				keywords = append(keywords, trimmed)
			}
		}
	}

	selectionPolicy := "first"
	if expected.SelectionPolicy == "exact_name" {
		selectionPolicy = "exact_name"
	}

	return &core.MetaCapabilityRefDefinition{
		Provider: expected.Provider,
		Binding:  "dynamic",
		Intent: &core.MetaCapabilityIntent{
			TaskDescription: taskDesc,
			Type:            expected.CapabilityType,
			Keywords:        keywords,
		},
		SelectionPolicy: selectionPolicy,
	}
}

// Internal Helper Functions & Mocks

type RecordedProvider struct {
	recorded *core.CapabilityBindingTrajectory
}

func newRecordedProvider(recorded *core.CapabilityBindingTrajectory) *RecordedProvider {
	return &RecordedProvider{recorded: recorded}
}

func (p *RecordedProvider) Id() string {
	return p.recorded.Provider
}

func (p *RecordedProvider) Discover(ctx context.Context, req core.ResolveCapabilityRequest) ([]core.CapabilityCandidate, error) {
	candidates := make([]core.CapabilityCandidate, len(p.recorded.Candidates))
	for i, c := range p.recorded.Candidates {
		candidates[i] = core.CapabilityCandidate{
			Name:        c.Name,
			Description: "recorded",
			Rank:        c.Rank,
		}
	}
	return candidates, nil
}

func (p *RecordedProvider) Bind(ctx context.Context, target string, tool *string) (*core.CapabilityTarget, error) {
	if p.recorded.Server != nil && target == *p.recorded.Server {
		toolName := ""
		if p.recorded.Tool != nil {
			toolName = *p.recorded.Tool
		}
		schema := "{}"
		if p.recorded.Schema != nil {
			schema = *p.recorded.Schema
		}
		return &core.CapabilityTarget{
			Server: target,
			Tool:   newRecordedTool(toolName, schema),
		}, nil
	}
	return nil, nil
}

type RecordedTool struct {
	name   string
	schema string
}

func newRecordedTool(name, schema string) *RecordedTool {
	return &RecordedTool{name: name, schema: schema}
}

func (t *RecordedTool) Name() string {
	return t.name
}

func (t *RecordedTool) Description() string {
	return "Recorded capability; execution is prohibited"
}

func (t *RecordedTool) ParameterSchema() string {
	return t.schema
}

func (t *RecordedTool) Execute(ctx context.Context, args string) (string, error) {
	return "", fmt.Errorf("replay cannot execute live tools")
}

func candidatesEqual(a, b []core.CapabilityBindingCandidate) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Rank != b[i].Rank {
			return false
		}
	}
	return true
}

func derefEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

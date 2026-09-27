package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/agent/tool/capability"
	"github.com/futugyou/openclaw/core"
)

type bindingGate struct {
	sem   chan struct{}
	users int
}

func newBindingGate() *bindingGate {
	g := &bindingGate{
		sem: make(chan struct{}, 1),
	}
	g.sem <- struct{}{}
	return g
}

func (g *bindingGate) Wait(ctx context.Context) error {
	select {
	case <-g.sem:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *bindingGate) Release() {
	select {
	case g.sem <- struct{}{}:
	default:
	}
}

type cached struct {
	target     *core.CapabilityTarget
	binding    *core.ResolveCapabilityBinding
	candidates []core.CapabilityCandidate
}

type ToolOutcomeError struct {
	ResultText   string
	ResultStatus string
	FailureCode  string
	Message      string
}

func (e *ToolOutcomeError) Error() string {
	return fmt.Sprintf("ToolOutcomeError: %s (Code: %s)", e.Message, e.FailureCode)
}

type circuitState struct {
	failures int
	until    time.Time
}

type CapabilitySlotExecutor struct {
	providers capability.CapabilityProviderRegistry
	cache     *capability.CapabilityBindingCache

	bindingGates map[string]*bindingGate // key 为 scope + "\x00" + key
	gateLock     sync.Mutex

	circuits    map[string]circuitState
	circuitLock sync.Mutex
}

func NewCapabilitySlotExecutor(providers capability.CapabilityProviderRegistry, cache *capability.CapabilityBindingCache) *CapabilitySlotExecutor {
	return &CapabilitySlotExecutor{
		providers:    providers,
		cache:        cache,
		bindingGates: make(map[string]*bindingGate),
		circuits:     make(map[string]circuitState),
	}
}

func (e *CapabilitySlotExecutor) AddedServerCount() int {
	return e.cache.Count()
}

func (e *CapabilitySlotExecutor) ClearRuntimeCache() {
	e.cache.Clear()
	e.circuitLock.Lock()
	defer e.circuitLock.Unlock()
	e.circuits = make(map[string]circuitState)
}

func (e *CapabilitySlotExecutor) ExecuteGoverned(
	ctx context.Context,
	reference *core.MetaCapabilityRefDefinition,
	arguments string,
	session *core.Session,
	turn *core.TurnContext,
	executor *OpenClawToolExecutor,
	callID string,
	isSkillToolAllowed func(string) bool,
) (*ToolExecutionResult, error) {
	var captured *ToolExecutionResult

	stepTool := capability.NewGovernedStepTool(
		func(innerCtx context.Context) (string, error) {
			userID := session.SenderId
			if session.AuthenticatedUserId != "" {
				userID = session.AuthenticatedUserId
			}

			res := e.Execute(
				innerCtx,
				reference,
				arguments,
				session.Id,
				func(ctx context.Context, tool core.ITool, args string) (*ToolExecutionResult, error) {
					if isSkillToolAllowed != nil && !isSkillToolAllowed(tool.Name()) {
						return fail("metadata_capability_denied", "Resolved tool is not permitted by skill metadata capabilities", args, &core.CapabilityBindingTrajectory{}, core.ToolResultStatusesBlocked), nil
					}
					var boundTool core.ITool
					p := e.providers.Get(reference.Provider)
					if _, isLocal := p.(*capability.LocalCapabilityProvider); !isLocal {
						boundTool = tool
					}
					return executor.Execute(ctx, tool.Name(), args, callID+":target", session, turn, false, nil, nil, 1, boundTool)
				},
				scope(session.ChannelId, userID),
				isSkillToolAllowed,
			)

			captured = res
			if captured.ResultStatus != core.ToolResultStatusesCompleted {
				return "", &ToolOutcomeError{
					ResultText:   captured.ResultText,
					ResultStatus: captured.ResultStatus,
					FailureCode:  captured.FailureCode,
					Message:      captured.FailureMessage,
				}
			}
			return captured.ResultText, nil
		},
	)

	result, err := executor.Execute(ctx, stepTool.Name(), arguments, callID+":resolve", session, turn, false, nil, nil, 1, stepTool)
	if err != nil {
		return nil, err
	}

	if captured != nil {
		result.Invocation = captured.Invocation
		result.BindingTrajectory = captured.BindingTrajectory
		result.RetrySafe = captured.RetrySafe
	} else {
		result.RetrySafe = false
	}

	return result, nil
}

func (e *CapabilitySlotExecutor) Execute(
	ctx context.Context,
	capabilityRef *core.MetaCapabilityRefDefinition,
	arguments string,
	sessionID string,
	invoke func(ctx context.Context, tool core.ITool, args string) (*ToolExecutionResult, error),
	securityScope string,
	isToolAllowed func(string) bool,
) *ToolExecutionResult {

	startTime := time.Now()
	provider := e.providers.Get(capabilityRef.Provider)

	providerID := capabilityRef.Provider
	if provider != nil {
		providerID = provider.Id()
	}

	gen := e.cache.Generation()
	trajectory := &core.CapabilityBindingTrajectory{
		Binding:  capabilityRef.Binding,
		Provider: providerID,
		Revision: gen,
	}

	if provider == nil {
		return fail(core.CapabilitySlotFailureCodesProviderUnavailable, "Capability provider is not configured", arguments, trajectory, core.ToolResultStatusesFailed)
	}

	intent := capabilityRef.Intent
	if (capabilityRef.Binding != "static" && capabilityRef.Binding != "dynamic") ||
		(capabilityRef.Binding == "static" && (capabilityRef.Static == nil || intent != nil)) ||
		(capabilityRef.Binding == "dynamic" && (intent == nil || capabilityRef.Static != nil)) {
		return fail("invalid_capability_ref", "Capability binding mode and payload must agree", arguments, trajectory, core.ToolResultStatusesFailed)
	}

	var intentKey string
	if intent == nil {
		staticScope := scope(capabilityRef.Static.Target, capabilityRef.Static.ToolName)
		hash := sha256.Sum256([]byte(staticScope))
		intentKey = strings.ToUpper(hex.EncodeToString(hash[:]))
	} else {
		intentKey = computeIntentKey(intent.TaskDescription, strings.Join(intent.Keywords, ","), capabilityRef.SelectionPolicy)
	}

	if intent != nil && intent.Type != nil {
		typedScope := scope(intentKey, *intent.Type)
		hash := sha256.Sum256([]byte(typedScope))
		intentKey = strings.ToUpper(hex.EncodeToString(hash[:]))
	}

	scopeStr := fmt.Sprintf("%d:%s", len(securityScope), securityScope)
	if capabilityRef.Binding == "static" {
		scopeStr += ":static"
	} else {
		scopeStr += fmt.Sprintf("%d:%s", len(sessionID), sessionID)
	}

	keyStr := fmt.Sprintf("%s:%d:%s:%s", provider.Id(), gen, capabilityRef.Binding, intentKey)

	if intent != nil {
		trajectory.CapabilityType = intent.Type
		trajectory.TaskDescription = &intent.TaskDescription
		kw := strings.Join(intent.Keywords, ",")
		trajectory.KeyWords = &kw
	}
	trajectory.SelectionPolicy = capabilityRef.SelectionPolicy
	trajectory.IntentKey = &intentKey

	gateMapKey := scopeStr + "\x00" + keyStr

	e.gateLock.Lock()
	gate, exists := e.bindingGates[gateMapKey]
	if !exists {
		gate = newBindingGate()
		e.bindingGates[gateMapKey] = gate
	}
	gate.users++
	e.gateLock.Unlock()

	entered := false
	var found *cached

	defer func() {
		if entered {
			gate.Release()
		}
		e.gateLock.Lock()
		gate.users--
		if gate.users == 0 {
			delete(e.bindingGates, gateMapKey)
		}
		e.gateLock.Unlock()
	}()

	err := gate.Wait(ctx)
	if err == nil {
		entered = true

		if cachedVal, ok := capability.GetCapabilityBindingCache[cached](e.cache, scopeStr, keyStr); (capabilityRef.Binding == "static" || sessionID != "") &&
			ok && cachedVal != nil {
			if capabilityRef.Binding == "static" || isToolAllowed == nil || isToolAllowed(cachedVal.target.Tool.Name()) {
				found = cachedVal
				trajectory.CacheHit = true
			}
		}

		if !trajectory.CacheHit {
			if capabilityRef.Binding == "static" {
				pinned := capabilityRef.Static
				target, err := provider.Bind(ctx, pinned.Target, &pinned.ToolName)
				if err != nil || target == nil {
					return fail(core.CapabilitySlotFailureCodesBindingFailed, "Pinned capability is unavailable", arguments, trajectory, core.ToolResultStatusesFailed)
				}
				found = &cached{
					target: target,
					binding: &core.ResolveCapabilityBinding{
						Server:          target.Server,
						Tool:            target.ToolId,
						Schema:          target.Tool.ParameterSchema(),
						TriedCandidates: []core.CapabilityCandidate{},
						Provider:        provider.Id(),
					},
					candidates: []core.CapabilityCandidate{},
				}
			} else {
				if intent == nil {
					return fail("invalid_capability_ref", "Dynamic binding requires an intent", arguments, trajectory, core.ToolResultStatusesFailed)
				}

				var policy core.ResolveCapabilitySelectionPolicy = "first"
				if capabilityRef.SelectionPolicy == "exact_name" {
					policy = "exact_name"
				}

				binding, failure, candidates, target, err := e.providers.Resolve(ctx, core.ResolveCapabilityRequest{
					TaskDescription: intent.TaskDescription,
					KeyWords:        new(strings.Join(intent.Keywords, ",")),
					SelectionPolicy: policy,
					Provider:        provider.Id(),
					CapabilityType:  intent.Type,
				}, isToolAllowed)

				if err == nil {
					trajectory.Candidates = mapCandidates(candidates)
					if failure != nil {
						trajectory.Attempted = mapCandidates(failure.TriedCandidates)
					}

					if target == nil {
						if failure != nil && failure.FailureCode == core.ResolveCapabilityFailureCodesProviderUnavailable {
							return fail("metadata_capability_denied", "Resolved tools are not permitted by skill metadata capabilities", arguments, trajectory, core.ToolResultStatusesBlocked)
						}
						failCode := core.CapabilitySlotFailureCodesResolveFailed
						msg := "Resolution failed"
						if failure != nil {
							if failure.FailureCode == core.ResolveCapabilityFailureCodesProviderUnavailable {
								failCode = core.CapabilitySlotFailureCodesProviderUnavailable
							}
							if failure.FailureCode != "" {
								msg = failure.FailureCode
							}
						}
						return fail(failCode, msg, arguments, trajectory, core.ToolResultStatusesFailed)
					}
					found = &cached{
						target:     target,
						binding:    binding,
						candidates: candidates,
					}
				}
			}

			if found != nil && (capabilityRef.Binding == "static" || sessionID != "") {
				e.cache.Store(scopeStr, keyStr, found, gen, capabilityRef.Binding == "static")
			}
		}
	}

	if found == nil || gen != e.cache.Generation() {
		return fail("capability_stale_binding", "Provider configuration changed during resolution; resolve again", arguments, trajectory, core.ToolResultStatusesFailed)
	}

	if found.target != nil {
		trajectory.Server = new(found.target.Server)
		trajectory.Tool = new(found.target.ToolId)
		trajectory.Schema = new(found.target.Tool.ParameterSchema())
		hash := sha256.Sum256([]byte(found.target.Tool.ParameterSchema()))
		trajectory.SchemaFingerprint = new(strings.ToUpper(hex.EncodeToString(hash[:])))
	}

	trajectory.Candidates = mapCandidates(found.candidates)
	if found.binding != nil {
		trajectory.Attempted = mapCandidates(found.binding.TriedCandidates)
	}
	trajectory.ElapsedMs = float64(time.Since(startTime).Milliseconds())

	circuitKey := fmt.Sprintf("%s:%s:%d:%s:%s", scopeStr, provider.Id(), gen, found.target.Server, found.target.Tool.Name())

	e.circuitLock.Lock()
	if cState, exists := e.circuits[circuitKey]; exists && cState.failures >= 3 && cState.until.After(time.Now()) {
		e.circuitLock.Unlock()
		return fail("capability_circuit_open", "Capability circuit is open; wait before trying again", arguments, trajectory, core.ToolResultStatusesFailed)
	}
	e.circuitLock.Unlock()

	var resultText *ToolExecutionResult
	if invoke == nil {
		outStr := found.target.Tool.Execute(ctx, arguments)
		resultText = result(outStr, arguments)
	} else {
		res, err := invoke(ctx, found.target.Tool, arguments)
		if err != nil {
			resultText = fail(core.CapabilitySlotFailureCodesExecutionFailed, err.Error(), arguments, trajectory, core.ToolResultStatusesFailed)
		} else {
			resultText = res
		}
	}

	switch resultText.ResultStatus {
	case core.ToolResultStatusesFailed:
		e.circuitLock.Lock()
		now := time.Now()

		for k, v := range e.circuits {
			if !v.until.After(now) {
				delete(e.circuits, k)
			}
		}

		if old, exists := e.circuits[circuitKey]; exists {
			e.circuits[circuitKey] = circuitState{
				failures: old.failures + 1,
				until:    now.Add(30 * time.Second),
			}
		} else {
			if len(e.circuits) >= 1024 {
				var minKey string
				var minUntil time.Time
				foundMin := false

				for k, v := range e.circuits {
					if v.failures < 3 {
						if !foundMin || v.until.Before(minUntil) {
							minUntil = v.until
							minKey = k
							foundMin = true
						}
					}
				}
				if foundMin {
					delete(e.circuits, minKey)
				}
			}

			if len(e.circuits) < 1024 {
				e.circuits[circuitKey] = circuitState{
					failures: 1,
					until:    now.Add(30 * time.Second),
				}
			}
		}
		e.circuitLock.Unlock()
	case core.ToolResultStatusesCompleted:
		e.circuitLock.Lock()
		delete(e.circuits, circuitKey)
		e.circuitLock.Unlock()
	}

	resultText.BindingTrajectory = trajectory
	resultText.RetrySafe = found.target.RetrySafe && resultText.ResultStatus == core.ToolResultStatusesFailed
	return resultText
}

func scope(channel, user string) string {
	return fmt.Sprintf("%d:%s%d:%s", len(channel), channel, len(user), user)
}

func result(text, args string) *ToolExecutionResult {
	return &ToolExecutionResult{
		Invocation: &core.ToolInvocation{
			ToolName:     "capability",
			Arguments:    args,
			Result:       text,
			ResultStatus: core.ToolResultStatusesCompleted,
		},
		ResultText:   text,
		ResultStatus: core.ToolResultStatusesCompleted,
	}
}

func fail(code, message, args string, trace *core.CapabilityBindingTrajectory, status string) *ToolExecutionResult {
	return &ToolExecutionResult{
		Invocation: &core.ToolInvocation{
			ToolName:       "capability",
			Arguments:      args,
			Result:         message,
			ResultStatus:   status,
			FailureCode:    code,
			FailureMessage: message,
		},
		ResultText:        message,
		ResultStatus:      status,
		FailureCode:       code,
		FailureMessage:    message,
		BindingTrajectory: trace,
	}
}

func mapCandidates(candidates []core.CapabilityCandidate) []core.CapabilityBindingCandidate {
	if candidates == nil {
		return nil
	}
	res := make([]core.CapabilityBindingCandidate, len(candidates))
	for i, c := range candidates {
		res[i] = core.CapabilityBindingCandidate{Name: c.Name, Rank: c.Rank}
	}
	return res
}

func computeIntentKey(taskDesc, keywords, policy string) string {
	raw := fmt.Sprintf("%s|%s|%s", taskDesc, keywords, policy)
	hash := sha256.Sum256([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

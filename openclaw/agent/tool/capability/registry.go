package capability

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type CapabilityProviderRegistry struct {
	providers       map[string]core.ICapabilityProvider
	ProviderIds     []string
	DefaultProvider string
}

func NewCapabilityProviderRegistry(providers []core.ICapabilityProvider, defaultProvider string) *CapabilityProviderRegistry {
	if defaultProvider == "" {
		defaultProvider = "local"
	}

	providerIds := make([]string, 0, len(providers))
	kvproviders := make(map[string]core.ICapabilityProvider, len(providers))
	for _, v := range providers {
		providerIds = append(providerIds, v.Id())
		kvproviders[v.Id()] = v
	}

	return &CapabilityProviderRegistry{
		providers:       kvproviders,
		ProviderIds:     providerIds,
		DefaultProvider: defaultProvider,
	}
}

func (c *CapabilityProviderRegistry) Get(id string) core.ICapabilityProvider {
	if id == "" {
		id = c.DefaultProvider
	}
	return c.providers[id]
}

func (c *CapabilityProviderRegistry) Resolve(ctx context.Context, request core.ResolveCapabilityRequest, isToolAllowed func(string) bool) (
	*core.ResolveCapabilityBinding, *core.ResolveCapabilityFailure, []core.CapabilityCandidate, *core.CapabilityTarget, error) {

	provider := c.Get(request.Provider)
	if provider == nil {
		return nil, &core.ResolveCapabilityFailure{FailureCode: core.ResolveCapabilityFailureCodesProviderUnavailable}, nil, nil, errors.New("can not found provider")
	}

	_, isLocal := provider.(*LocalCapabilityProvider)
	rawcandidates, err := provider.Discover(ctx, request)
	if err != nil {
		return nil, &core.ResolveCapabilityFailure{FailureCode: getCode(false, true), TriedCandidates: nil}, nil, nil, err
	}

	var discovered []core.CapabilityCandidate
	policyDenied := false
	for _, v := range rawcandidates {
		allowed := !isLocal || (isToolAllowed != nil && isToolAllowed(v.Name))
		if allowed {
			discovered = append(discovered, v)
		} else {
			policyDenied = true
		}
	}

	slices.SortFunc(discovered, func(a, b core.CapabilityCandidate) int {
		if i := cmp.Compare(a.Rank, b.Rank); i != 0 {
			return i
		}
		return cmp.Compare(a.Name, b.Name)
	})

	if len(discovered) == 0 {
		code := core.ResolveCapabilityFailureCodesNoCandidates
		if policyDenied {
			code = core.ResolveCapabilityFailureCodesToolPolicyDenied
		}
		return nil, &core.ResolveCapabilityFailure{FailureCode: code}, nil, nil, errors.New("no matching capabilityCandidate")
	}

	selected := discovered
	if request.SelectionPolicy == core.ResolveCapabilitySelectionPolicyExactName {
		selected = nil
		for _, v := range discovered {
			if v.Name == request.TaskDescription {
				selected = append(selected, v)
			}
		}
	}

	var tried []core.CapabilityCandidate
	fail := func(err error) (*core.ResolveCapabilityBinding, *core.ResolveCapabilityFailure, []core.CapabilityCandidate, *core.CapabilityTarget, error) {
		return nil, &core.ResolveCapabilityFailure{
			FailureCode:     getCode(policyDenied, len(tried) == 0),
			TriedCandidates: tried,
		}, tried, nil, err
	}

	for i := 0; i < min(5, len(selected)); i++ {
		candidate := selected[i]
		tried = append(tried, candidate)

		target, err := provider.Bind(ctx, candidate.Name, nil)
		if err != nil {
			return fail(err)
		}
		if target == nil {
			continue
		}

		if isToolAllowed != nil && !isToolAllowed(target.Tool.Name()) {
			policyDenied = true
			continue
		}

		binding := &core.ResolveCapabilityBinding{
			Server:            target.Server,
			Tool:              target.Tool.Name(),
			Schema:            target.Tool.ParameterSchema(),
			TriedCandidates:   tried,
			Provider:          provider.Id(),
			SchemaFingerprint: util.ComputeTurnHash(target.Tool.ParameterSchema()),
		}

		return binding, nil, tried, target, nil
	}

	return fail(errors.New("no matching capabilityCandidate"))
}

func getCode(policyDenied, noTried bool) string {
	if policyDenied {
		return core.ResolveCapabilityFailureCodesToolPolicyDenied
	}
	if noTried {
		return core.ResolveCapabilityFailureCodesSelectionPolicyNoMatch
	}
	return core.ResolveCapabilityFailureCodesAllAddsFailed
}

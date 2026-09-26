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

	providerIds := []string{}
	kvproviders := map[string]core.ICapabilityProvider{}
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

	if p, ok := c.providers[id]; ok {
		return p
	}

	return nil
}

func (c *CapabilityProviderRegistry) Resolve(ctx context.Context, request core.ResolveCapabilityRequest, isToolAllowed func(string) bool) (
	*core.ResolveCapabilityBinding, *core.ResolveCapabilityFailure, []core.CapabilityCandidate, *core.CapabilityTarget, error) {

	provider := c.Get(request.Provider)
	if provider == nil {
		return nil, &core.ResolveCapabilityFailure{FailureCode: core.ResolveCapabilityFailureCodesProviderUnavailable}, nil, nil, errors.New("can not found provider")
	}

	isLocalCapabilityProvider := false
	switch provider.(type) {
	case *LocalCapabilityProvider:
		isLocalCapabilityProvider = true
	}

	tried := []core.CapabilityCandidate{}
	candidates := []core.CapabilityCandidate{}
	policyDenied := false
	rawcandidates, err := provider.Discover(ctx, request)
	if err != nil {
		return nil, &core.ResolveCapabilityFailure{FailureCode: getCode(policyDenied, len(tried) == 0), TriedCandidates: candidates}, candidates, nil, err
	}

	discovered := []core.CapabilityCandidate{}
	for _, v := range rawcandidates {
		allowed := false
		if !isLocalCapabilityProvider || (isToolAllowed != nil && isToolAllowed(v.Name) != false) {
			allowed = true
			discovered = append(discovered, v)
		}
		policyDenied = policyDenied || !allowed
	}

	slices.SortFunc(discovered, func(a, b core.CapabilityCandidate) int {
		i := cmp.Compare(a.Rank, b.Rank)
		if i != 0 {
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

	selected := []core.CapabilityCandidate{}
	if request.SelectionPolicy == core.ResolveCapabilitySelectionPolicyExactName {
		for _, v := range discovered {
			if v.Name == request.TaskDescription {
				selected = append(selected, v)
			}
		}
	} else {
		selected = discovered
	}

	for i := 0; i < min(5, len(selected)); i++ {
		candidate := selected[i]
		tried = append(tried, candidate)
		target, err := provider.Bind(ctx, candidate.Name, nil)
		if err != nil {
			return nil, &core.ResolveCapabilityFailure{FailureCode: getCode(policyDenied, len(tried) == 0), TriedCandidates: candidates}, candidates, nil, err
		}

		if target == nil {
			continue
		}

		if isToolAllowed != nil && isToolAllowed(target.Tool.Name()) == false {
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

		return binding, nil, candidates, target, nil
	}

	return nil, &core.ResolveCapabilityFailure{FailureCode: getCode(policyDenied, len(tried) == 0), TriedCandidates: candidates}, candidates, nil, errors.New("no matching capabilityCandidate")
}

func getCode(policyDenied bool, hasTried bool) string {
	if policyDenied {
		return core.ResolveCapabilityFailureCodesToolPolicyDenied
	}

	if hasTried {
		return core.ResolveCapabilityFailureCodesAllAddsFailed
	}

	return core.ResolveCapabilityFailureCodesSelectionPolicyNoMatch

}

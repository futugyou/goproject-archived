package routingdecision

import (
	"context"
	"net/http"
	"net/url"
)

type ApiKeyResolver func(context.Context) (string, error)
type TypeSafeDecisionClient struct {
	httpClient    *http.Client
	uri           *url.URL
	resolveApiKey ApiKeyResolver
}

func NewTypeSafeDecisionClient(
	httpClient *http.Client,
	uri *url.URL,
	resolveApiKey ApiKeyResolver) *TypeSafeDecisionClient {

	return &TypeSafeDecisionClient{
		uri:           uri,
		httpClient:    httpClient,
		resolveApiKey: resolveApiKey,
	}
}

func (l *TypeSafeDecisionClient) Evaluate(ctx context.Context, request *DecisionRequest) (*DecisionResponse, error) {
	if l.resolveApiKey == nil {
		return nil, NewDecisionError("missing_api_key_resolver")
	}

	apiKey, _ := l.resolveApiKey(ctx)
	if apiKey == "" {
		return nil, NewDecisionError("missing_api_key")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.uri.String(), nil)
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	return SendAndValidateDecision(l.httpClient, req, request)
}

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	GrantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
	GrantTypeJwtBearer     = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	TokenTypeIDToken       = "urn:ietf:params:oauth:token-type:id_token"
	TokenTypeSaml2         = "urn:ietf:params:oauth:token-type:saml2"
	TokenTypeIDJag         = "urn:ietf:params:oauth:token-type:id-jag"
	TokenTypeNotApplicable = "N_A"
)

func RequestJWTAuthorizationGrant(ctx context.Context, httpClient *http.Client, options RequestJWTAuthGrantOptions) (string, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	if options.TokenEndpoint == "" || options.Audience == "" || options.Resource == "" ||
		options.IdToken == "" || options.ClientId == "" {
		return "", fmt.Errorf("missing required options for JWT authorization grant")
	}

	formData := url.Values{}
	formData.Set("grant_type", GrantTypeTokenExchange)
	formData.Set("requested_token_type", TokenTypeIDJag)
	formData.Set("subject_token", options.IdToken)
	formData.Set("subject_token_type", TokenTypeIDToken)
	formData.Set("audience", options.Audience)
	formData.Set("resource", options.Resource)
	formData.Set("client_id", options.ClientId)

	if options.ClientSecret != "" {
		formData.Set("client_secret", options.ClientSecret)
	}
	if options.Scope != "" {
		formData.Set("scope", options.Scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, options.TokenEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp OauthErrorResponse
		_ = json.Unmarshal(bodyBytes, &errResp)

		return "", &IdentityAssertionGrantError{
			StatusCode:       resp.StatusCode,
			Message:          fmt.Sprintf("token exchange failed with status %d", resp.StatusCode),
			Code:             errResp.Error,
			ErrorDescription: errResp.ErrorDescription,
			ErrorURI:         errResp.ErrorURI,
		}
	}

	var response JagTokenExchangeResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    "failed to parse token exchange response",
		}
	}

	if response.AccessToken == "" {
		return "", &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    "token exchange response missing required field: access_token",
		}
	}

	if response.IssuedTokenType != TokenTypeIDJag {
		return "", &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("token exchange response issued_token_type must be '%s', got '%s'", TokenTypeIDJag, response.IssuedTokenType),
		}
	}

	if response.TokenType != TokenTypeNotApplicable {
		return "", &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("token exchange response token_type must be '%s' per RFC 8693 §2.2.1, got '%s'", TokenTypeNotApplicable, response.TokenType),
		}
	}

	return response.AccessToken, nil
}

// JWT Bearer Grant (RFC 7523)
func ExchangeJWTBearerGrant(ctx context.Context, httpClient *http.Client, options ExchangeJwtBearerGrantOptions) (*TokenContainer, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	if options.TokenEndpoint == "" || options.Assertion == "" || options.ClientId == "" {
		return nil, fmt.Errorf("missing required options for JWT bearer grant exchange")
	}

	formData := url.Values{}
	formData.Set("grant_type", GrantTypeJwtBearer)
	formData.Set("assertion", options.Assertion)
	formData.Set("client_id", options.ClientId)

	if options.ClientSecret != "" {
		formData.Set("client_secret", options.ClientSecret)
	}
	if options.Scope != "" {
		formData.Set("scope", options.Scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, options.TokenEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp OauthErrorResponse
		_ = json.Unmarshal(bodyBytes, &errResp)

		return nil, &IdentityAssertionGrantError{
			StatusCode:       resp.StatusCode,
			Message:          fmt.Sprintf("JWT bearer grant failed with status %d", resp.StatusCode),
			Code:             errResp.Error,
			ErrorDescription: errResp.ErrorDescription,
			ErrorURI:         errResp.ErrorURI,
		}
	}

	var response JwtBearerAccessTokenResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    "failed to parse JWT bearer grant response",
		}
	}

	if response.AccessToken == "" {
		return nil, &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    "JWT bearer grant response missing required field: access_token",
		}
	}

	if response.TokenType == "" {
		return nil, &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    "JWT bearer grant response missing required field: token_type",
		}
	}

	if !strings.EqualFold(response.TokenType, "bearer") {
		return nil, &IdentityAssertionGrantError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("JWT bearer grant response token_type must be 'bearer' per RFC 7523, got '%s'", response.TokenType),
		}
	}

	return &TokenContainer{
		AccessToken:  response.AccessToken,
		TokenType:    response.TokenType,
		RefreshToken: response.RefreshToken,
		ExpiresIn:    response.ExpiresIn,
		Scope:        response.Scope,
		ObtainedAt:   time.Now().UTC(),
	}, nil
}

// Helper: Auth Server Metadata Discovery
var wellKnownPaths = []string{
	".well-known/openid-configuration",
	".well-known/oauth-authorization-server",
}

func DiscoverAuthServerMetadata(ctx context.Context, issuerURL string, httpClient *http.Client) (*AuthorizationServerMetadata, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL := issuerURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	for _, path := range wellKnownPaths {
		endpoint := baseURL + path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			continue
		}

		var metadata AuthorizationServerMetadata
		err = json.NewDecoder(resp.Body).Decode(&metadata)
		resp.Body.Close()

		if err == nil {
			return &metadata, nil
		}
	}

	return nil, fmt.Errorf("failed to discover authorization server metadata for: %s", issuerURL)
}

type IdentityAssertionGrantProvider struct {
	options                  IdentityAssertionGrantProviderOptions
	httpClient               *http.Client
	logger                   *slog.Logger
	cachedTokens             *TokenContainer
	lockCh                   chan struct{}
	resolvedIdpTokenEndpoint string
}

func NewIdentityAssertionGrantProvider(options *IdentityAssertionGrantProviderOptions, httpClient *http.Client, logger *slog.Logger) (*IdentityAssertionGrantProvider, error) {
	if options == nil || options.ClientId == "" || options.IdpClientId == "" || (options.IdpUrl == "" && options.IdpTokenEndpoint == "") || options.IdTokenCallback == nil {
		return nil, errors.New("options parameter invalid")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return &IdentityAssertionGrantProvider{
		options:    *options,
		httpClient: httpClient,
		logger:     logger,
		lockCh:     ch,
	}, nil
}

func (i *IdentityAssertionGrantProvider) InvalidateCache() {
	select {
	case <-i.lockCh:
		defer func() { i.lockCh <- struct{}{} }()
		i.cachedTokens = nil
	case <-time.After(5 * time.Second):
	}
}

func (i *IdentityAssertionGrantProvider) resolveIdpTokenEndpoint(ctx context.Context) (string, error) {
	if i.resolvedIdpTokenEndpoint != "" {
		return i.resolvedIdpTokenEndpoint, nil
	}

	if i.options.IdpTokenEndpoint != "" {
		i.resolvedIdpTokenEndpoint = i.options.IdpTokenEndpoint
		return i.resolvedIdpTokenEndpoint, nil
	}

	idpMetadata, err := DiscoverAuthServerMetadata(ctx, i.options.IdpUrl, i.httpClient)
	if err != nil {
		return "", err
	}

	var resolved = idpMetadata.TokenEndpoint
	if resolved == "" {
		return "", fmt.Errorf("IdP metadata discovery for %s did not return a token_endpoint", i.options.IdpUrl)
	}

	i.resolvedIdpTokenEndpoint = resolved
	return resolved, nil
}

func (i *IdentityAssertionGrantProvider) acquireAccessToken(ctx context.Context, resourceUrl, authorizationServerUrl string) (*TokenContainer, error) {
	i.logger.Debug("Starting Cross-Application Access flow for resource", "ResourceUrl", resourceUrl)

	// Step 1: Discover MCP authorization server metadata to find the token endpoint
	mcpAuthMetadata, err := DiscoverAuthServerMetadata(ctx, authorizationServerUrl, i.httpClient)
	if err != nil {
		return nil, err
	}

	var mcpTokenEndpoint = mcpAuthMetadata.TokenEndpoint
	if mcpTokenEndpoint == "" {
		return nil, fmt.Errorf("MCP authorization server metadata at %s missing token_endpoint", authorizationServerUrl)
	}

	// Step 2: Call the ID token callback to get the caller's OIDC ID token
	var context = &IdentityAssertionGrantContext{
		ResourceUrl:            resourceUrl,
		AuthorizationServerUrl: authorizationServerUrl,
	}

	i.logger.Debug("Requesting ID token via callback")
	var idToken = i.options.IdTokenCallback(ctx, *context)

	if idToken == "" {
		return nil, &IdentityAssertionGrantError{Message: "ID token callback returned a null or empty token"}
	}

	// Step 3: RFC 8693 token exchange — ID token → JWT Authorization Grant (JAG) at the enterprise IdP
	i.logger.Debug("Performing RFC 8693 token exchange at IdP")
	idpTokenEndpoint, err := i.resolveIdpTokenEndpoint(ctx)
	if err != nil {
		return nil, err
	}

	jag, err := RequestJWTAuthorizationGrant(ctx, i.httpClient, RequestJWTAuthGrantOptions{
		TokenEndpoint: idpTokenEndpoint,
		Audience:      authorizationServerUrl,
		Resource:      resourceUrl,
		IdToken:       idToken,
		ClientId:      i.options.IdpClientId,
		ClientSecret:  i.options.IdpClientSecret,
		Scope:         i.options.IdpScope,
	})
	if err != nil {
		return nil, err
	}

	// Step 4: RFC 7523 JWT bearer grant — JAG → access token at the MCP authorization server
	i.logger.Debug("Exchanging JAG for access token at ", "McpTokenEndpoint", mcpTokenEndpoint)
	tokens, err := ExchangeJWTBearerGrant(ctx, i.httpClient, ExchangeJwtBearerGrantOptions{
		TokenEndpoint: mcpTokenEndpoint,
		Assertion:     jag,
		ClientId:      i.options.ClientId,
		ClientSecret:  i.options.ClientSecret,
		Scope:         i.options.Scope,
	})
	if err != nil {
		return nil, err
	}

	i.cachedTokens = tokens
	i.logger.Debug("Cross-Application Access flow completed successfully")

	return tokens, nil
}

func (i *IdentityAssertionGrantProvider) GetAccessToken(ctx context.Context, resourceUrl, authorizationServerUrl string) (*TokenContainer, error) {
	// Return cached token if still valid. Read the field once into a local so a concurrent
	// InvalidateCache (which nulls _cachedTokens) cannot turn this lock-free check into a null
	// dereference or a null return between the null check and the return.
	var cachedBeforeLock = i.cachedTokens
	if cachedBeforeLock != nil && !cachedBeforeLock.IsExpired() {
		return cachedBeforeLock, nil
	}

	// Serialize the exchange so concurrent callers that all saw the expired/absent token don't
	// each run the full multi-step flow. Waiters re-check the cache after acquiring the lock and
	// reuse the token produced by whoever ran the exchange first.
	select {
	case <-i.lockCh:
		defer func() { i.lockCh <- struct{}{} }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if i.cachedTokens != nil && !i.cachedTokens.IsExpired() {
		return i.cachedTokens, nil
	}

	return i.acquireAccessToken(ctx, resourceUrl, authorizationServerUrl)
}

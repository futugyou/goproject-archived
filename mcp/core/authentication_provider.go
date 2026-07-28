package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

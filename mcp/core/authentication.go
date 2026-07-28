package core

import (
	"context"
	"fmt"
	"slices"
	"time"
)

type AuthorizationCallbackContext struct {
	AuthorizationUri string
	RedirectUri      string
}

type AuthorizationResult struct {
	Code  string
	State string
	Iss   string
}

type AuthorizationServerMetadata struct {
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	RegistrationEndpoint                       string   `json:"registration_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	Issuer                                     string   `json:"issuer"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ClientIdMetadataDocumentSupported          bool     `json:"client_id_metadata_document_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

type ScopeSelectorDelegate func(scopes []string) []string
type ClientOAuthOptions struct {
	RedirectUri                       string
	ClientId                          string
	ClientSecret                      string
	ClientMetadataDocumentUri         string
	Scopes                            []string
	ScopeSelector                     ScopeSelectorDelegate
	AuthorizationCallbackHandler      func(context.Context, AuthorizationCallbackContext) (*AuthorizationResult, error)
	AuthServerSelector                func([]string) string
	DynamicClientRegistration         *DynamicClientRegistrationOptions
	AdditionalAuthorizationParameters map[string]string
	TokenCache                        ITokenCache
}

type DynamicClientRegistrationOptions struct {
	ClientName         string
	ClientUri          string
	InitialAccessToken string
	ApplicationType    string
	ResponseDelegate   func(context.Context, DynamicClientRegistrationResponse) error
}

type DynamicClientRegistrationResponse struct {
	ClientId                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret"`
	RedirectUris            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	ClientIdIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at"`
}

type ITokenCache interface {
	StoreTokens(context.Context, TokenContainer) error
	GetTokens(context.Context) (*TokenContainer, error)
}

type TokenContainer struct {
	TokenType               string    `json:"token_type"`
	AccessToken             string    `json:"access_token"`
	RefreshToken            string    `json:"refresh_token,omitempty"`
	ExpiresIn               int       `json:"expires_in,omitempty"`
	Scope                   string    `json:"scope,omitempty"`
	ObtainedAt              time.Time `json:"obtained_at"`
	ClientId                string    `json:"client_id"`
	ClientSecret            string    `json:"client_secret"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method"`
}

func (t *TokenContainer) IsExpired() bool {
	if t.ExpiresIn <= 0 {
		return false
	}

	return time.Now().UTC().After(t.ObtainedAt.Add(time.Duration(t.ExpiresIn) * time.Second))
}

type DynamicClientRegistrationRequest struct {
	RedirectUris            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	ClientName              string   `json:"client_name"`
	ClientUri               string   `json:"client_uri"`
	Scope                   string   `json:"scope"`
	ApplicationType         string   `json:"application_type"`
}

type ExchangeJwtBearerGrantOptions struct {
	TokenEndpoint string
	Assertion     string
	ClientId      string
	ClientSecret  string
	Scope         string
}

type IdentityAssertionGrantError struct {
	StatusCode       int
	Message          string
	Code             string
	ErrorDescription string
	ErrorURI         string
}

func (e *IdentityAssertionGrantError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (error: %s, description: %s)", e.Message, e.Code, e.ErrorDescription)
	}
	return e.Message
}

type IdentityAssertionGrantContext struct {
	ResourceUrl            string
	AuthorizationServerUrl string
}

type IdentityAssertionGrantIdTokenCallback func(context.Context, IdentityAssertionGrantContext) string

type IdentityAssertionGrantProviderOptions struct {
	ClientId         string
	ClientSecret     string
	Scope            string
	IdpUrl           string
	IdpTokenEndpoint string
	IdpClientId      string
	IdpClientSecret  string
	IdpScope         string
	IdTokenCallback  IdentityAssertionGrantIdTokenCallback
}

var _ ITokenCache = (*InMemoryTokenCache)(nil)

type InMemoryTokenCache struct {
	tokens *TokenContainer
}

// GetTokens implements [ITokenCache].
func (i *InMemoryTokenCache) GetTokens(context.Context) (*TokenContainer, error) {
	return i.tokens, nil
}

// StoreTokens implements [ITokenCache].
func (i *InMemoryTokenCache) StoreTokens(ctx context.Context, tokens TokenContainer) error {
	i.tokens = &tokens
	return nil
}

type JagTokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	Scope           string `json:"scope"`
	ExpiresIn       int    `json:"expires_in"`
}

type JwtBearerAccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type OauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorURI         string `json:"error_uri"`
}

type ProtectedResourceMetadata struct {
	Resource                              string   `json:"resource"`
	AuthorizationServers                  []string `json:"authorization_servers"`
	BearerMethodsSupported                []string `json:"bearer_methods_supported"`
	ScopesSupported                       []string `json:"scopes_supported"`
	JwksUri                               string   `json:"jwks_uri"`
	ResourceSigningAlgValuesSupported     []string `json:"resource_signing_alg_values_supported"`
	ResourceName                          string   `json:"resource_name"`
	ResourceDocumentation                 string   `json:"resource_documentation"`
	ResourcePolicyUri                     string   `json:"resource_policy_uri"`
	ResourceTosUri                        string   `json:"resource_tos_uri"`
	TlsClientCertificateBoundAccessTokens bool     `json:"tls_client_certificate_bound_access_tokens"`
	AuthorizationDetailsTypesSupported    []string `json:"authorization_details_types_supported"`
	DpopSigningAlgValuesSupported         []string `json:"dpop_signing_alg_values_supported"`
	DpopBoundAccessTokensRequired         bool     `json:"dpop_bound_access_tokens_required"`
	WwwAuthenticateScope                  string   `json:"-"`
}

func (p *ProtectedResourceMetadata) Clone(derivedResource string) ProtectedResourceMetadata {
	resource := p.Resource
	if resource == "" {
		resource = derivedResource
	}

	return ProtectedResourceMetadata{
		Resource:                              resource,
		AuthorizationServers:                  slices.Clone(p.AuthorizationServers),
		BearerMethodsSupported:                slices.Clone(p.BearerMethodsSupported),
		ScopesSupported:                       slices.Clone(p.ScopesSupported),
		JwksUri:                               p.JwksUri,
		ResourceSigningAlgValuesSupported:     slices.Clone(p.ResourceSigningAlgValuesSupported),
		ResourceName:                          p.ResourceName,
		ResourceDocumentation:                 p.ResourceDocumentation,
		ResourcePolicyUri:                     p.ResourcePolicyUri,
		ResourceTosUri:                        p.ResourceTosUri,
		TlsClientCertificateBoundAccessTokens: p.TlsClientCertificateBoundAccessTokens,
		AuthorizationDetailsTypesSupported:    slices.Clone(p.AuthorizationDetailsTypesSupported),
		DpopSigningAlgValuesSupported:         slices.Clone(p.DpopSigningAlgValuesSupported),
		DpopBoundAccessTokensRequired:         p.DpopBoundAccessTokensRequired,
		WwwAuthenticateScope:                  p.WwwAuthenticateScope,
	}
}

type RequestJWTAuthGrantOptions struct {
	TokenEndpoint string
	Audience      string
	Resource      string
	IdToken       string
	ClientId      string
	ClientSecret  string
	Scope         string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

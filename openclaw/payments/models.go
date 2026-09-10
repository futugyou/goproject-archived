package payments

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	PaymentEnvTest = "test"
	PaymentEnvLive = "live"
)

func TryNormalizeEnvironment(value string) (string, bool) {
	val := strings.TrimSpace(value)
	if val == "" {
		return PaymentEnvTest, true
	}
	if strings.EqualFold(val, PaymentEnvTest) {
		return PaymentEnvTest, true
	}
	if strings.EqualFold(val, PaymentEnvLive) {
		return PaymentEnvLive, true
	}
	return "", false
}

func NormalizeEnvironment(value string, fallback ...string) (string, error) {
	fb := PaymentEnvTest
	if len(fallback) > 0 {
		fb = fallback[0]
	}

	candidate := strings.TrimSpace(value)
	if candidate == "" {
		candidate = fb
	}

	if env, ok := TryNormalizeEnvironment(candidate); ok {
		return env, nil
	}

	return "", fmt.Errorf("unsupported payment environment '%s'. Use 'test' or 'live'", value)
}

func IsLiveEnvironment(value string) bool {
	env, ok := TryNormalizeEnvironment(value)
	return ok && env == PaymentEnvLive
}

// Payment Actions
const (
	ActionSetupStatus           = "setup_status"
	ActionListFundingSources    = "list_funding_sources"
	ActionIssueVirtualCard      = "issue_virtual_card"
	ActionExecuteMachinePayment = "execute_machine_payment"
	ActionGetPaymentStatus      = "get_payment_status"
	ActionBrowserSentinelFill   = "browser_sentinel_fill"
)

// Decision Kinds
const (
	DecisionAllow           = "allow"
	DecisionDeny            = "deny"
	DecisionRequireApproval = "require_approval"
)

// Approval Severities
const (
	SeverityCritical = "critical"
)

// ==========================================
// Records & Structs
// ==========================================

type PaymentSetupRequirement struct {
	Name      string `json:"name"`
	Satisfied bool   `json:"satisfied"`
	Message   string `json:"message,omitempty"`
}

type PaymentSetupStatus struct {
	ProviderId   string                    `json:"providerId"`
	Enabled      bool                      `json:"enabled"`
	Installed    bool                      `json:"installed"`
	Version      string                    `json:"version,omitempty"`
	Mode         string                    `json:"mode"`
	Status       string                    `json:"status"`
	Message      string                    `json:"message,omitempty"`
	Requirements []PaymentSetupRequirement `json:"requirements"`
	Metadata     map[string]string         `json:"metadata"`
}

func NewPaymentSetupStatus(providerId string) PaymentSetupStatus {
	return PaymentSetupStatus{
		ProviderId:   providerId,
		Mode:         PaymentEnvTest,
		Status:       "unknown",
		Requirements: make([]PaymentSetupRequirement, 0),
		Metadata:     make(map[string]string),
	}
}

type FundingSource struct {
	FundingSourceId string `json:"fundingSourceId"`
	ProviderId      string `json:"providerId"`
	DisplayName     string `json:"displayName"`
	Type            string `json:"type"`
	Last4           string `json:"last4,omitempty"`
	Currency        string `json:"currency,omitempty"`
	TestMode        bool   `json:"testMode"`
	Available       bool   `json:"available"`
}

type BuyerProfile struct {
	BuyerId     string `json:"buyerId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	EmailHash   string `json:"emailHash,omitempty"`
	Country     string `json:"country,omitempty"`
}

type VirtualCardRequest struct {
	ProviderId      string        `json:"providerId,omitempty"`
	FundingSourceId string        `json:"fundingSourceId,omitempty"`
	MerchantName    string        `json:"merchantName"`
	MerchantUrl     string        `json:"merchantUrl,omitempty"`
	AmountMinor     int64         `json:"amountMinor"`
	Currency        string        `json:"currency"`
	Purpose         string        `json:"purpose,omitempty"`
	ValidUntilUtc   *time.Time    `json:"validUntilUtc,omitempty"`
	BuyerProfile    *BuyerProfile `json:"buyerProfile,omitempty"`
	Environment     string        `json:"environment"`
}

type VirtualCardHandle struct {
	HandleId           string     `json:"handleId"`
	ProviderId         string     `json:"providerId"`
	Last4              string     `json:"last4,omitempty"`
	TargetMerchantName string     `json:"targetMerchantName,omitempty"`
	IssuedAtUtc        time.Time  `json:"issuedAtUtc"`
	ValidUntilUtc      *time.Time `json:"validUntilUtc,omitempty"`
	SpendRequestId     string     `json:"spendRequestId,omitempty"`
	Status             string     `json:"status"`
	Environment        string     `json:"environment"`
}

type VirtualCardIssueResult struct {
	Handle VirtualCardHandle `json:"handle"`
	Secret *PaymentSecret    `json:"-"`
}

type MachinePaymentChallenge struct {
	ChallengeId  string            `json:"challengeId,omitempty"`
	Protocol     string            `json:"protocol,omitempty"`
	ResourceUrl  string            `json:"resourceUrl,omitempty"`
	MerchantName string            `json:"merchantName,omitempty"`
	AmountMinor  int64             `json:"amountMinor"`
	Currency     string            `json:"currency"`
	ProviderId   string            `json:"providerId,omitempty"`
	SafeMetadata map[string]string `json:"safeMetadata"`
}

type MachinePaymentRequest struct {
	ProviderId      string                  `json:"providerId,omitempty"`
	Challenge       MachinePaymentChallenge `json:"challenge"`
	FundingSourceId string                  `json:"fundingSourceId,omitempty"`
	Environment     string                  `json:"environment"`
	IdempotencyKey  string                  `json:"idempotencyKey,omitempty"`
}

type MachinePaymentResult struct {
	PaymentId         string            `json:"paymentId"`
	ProviderId        string            `json:"providerId"`
	Status            string            `json:"status"`
	MerchantName      string            `json:"merchantName,omitempty"`
	AmountMinor       int64             `json:"amountMinor"`
	Currency          string            `json:"currency"`
	CreatedAtUtc      time.Time         `json:"createdAtUtc"`
	ProviderReference string            `json:"providerReference,omitempty"`
	SafeMetadata      map[string]string `json:"safeMetadata"`
}

type MachinePaymentProviderResult struct {
	Result MachinePaymentResult `json:"result"`
	// JsonIgnore
	ScopedAuthorizationSecret *PaymentSecret `json:"-"`
}

type PaymentStatus struct {
	PaymentId         string     `json:"paymentId"`
	ProviderId        string     `json:"providerId"`
	Status            string     `json:"status"`
	MerchantName      string     `json:"merchantName,omitempty"`
	AmountMinor       int64      `json:"amountMinor,omitempty"`
	Currency          string     `json:"currency,omitempty"`
	UpdatedAtUtc      *time.Time `json:"updatedAtUtc,omitempty"`
	ProviderReference string     `json:"providerReference,omitempty"`
}

type PaymentPolicyDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

func (p PaymentPolicyDecision) RequiresApproval() bool {
	return strings.EqualFold(p.Decision, DecisionRequireApproval)
}

func (p PaymentPolicyDecision) Allowed() bool {
	return strings.EqualFold(p.Decision, DecisionAllow)
}

func (p PaymentPolicyDecision) Denied() bool {
	return strings.EqualFold(p.Decision, DecisionDeny)
}

type ApprovalRequest struct {
	Action               string     `json:"action"`
	Summary              string     `json:"summary"`
	Severity             string     `json:"severity"`
	MerchantName         string     `json:"merchantName,omitempty"`
	AmountMinor          int64      `json:"amountMinor,omitempty"`
	Currency             string     `json:"currency,omitempty"`
	FundingSourceDisplay string     `json:"fundingSourceDisplay,omitempty"`
	ProviderId           string     `json:"providerId,omitempty"`
	ExpiresAtUtc         *time.Time `json:"expiresAtUtc,omitempty"`
	Environment          string     `json:"environment"`
	AgentId              string     `json:"agentId,omitempty"`
	WorkspaceId          string     `json:"workspaceId,omitempty"`
	SessionId            string     `json:"sessionId,omitempty"`
	ChannelId            string     `json:"channelId,omitempty"`
	SenderId             string     `json:"senderId,omitempty"`
	CliConfirmed         bool       `json:"cliConfirmed"`
}

type ApprovalResult struct {
	Approved     bool      `json:"approved"`
	Source       string    `json:"source"`
	Reason       string    `json:"reason,omitempty"`
	DecidedAtUtc time.Time `json:"decidedAtUtc"`
}

type PaymentExecutionContext struct {
	SessionId                    string `json:"sessionId,omitempty"`
	ChannelId                    string `json:"channelId,omitempty"`
	SenderId                     string `json:"senderId,omitempty"`
	CorrelationId                string `json:"correlationId,omitempty"`
	WorkspaceId                  string `json:"workspaceId,omitempty"`
	Environment                  string `json:"environment"`
	CliConfirmed                 bool   `json:"cliConfirmed"`
	AllowTestModeWithoutApproval bool   `json:"allowTestModeWithoutApproval"`
}

type PaymentAuditEvent struct {
	EventType     string     `json:"eventType"`
	ProviderId    string     `json:"providerId"`
	HandleId      string     `json:"handleId,omitempty"`
	PaymentId     string     `json:"paymentId,omitempty"`
	Last4         string     `json:"last4,omitempty"`
	MerchantName  string     `json:"merchantName,omitempty"`
	AmountMinor   int64      `json:"amountMinor,omitempty"`
	Currency      string     `json:"currency,omitempty"`
	IssuedAtUtc   *time.Time `json:"issuedAtUtc,omitempty"`
	ValidUntilUtc *time.Time `json:"validUntilUtc,omitempty"`
	Status        string     `json:"status,omitempty"`
	Decision      string     `json:"decision,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	Environment   string     `json:"environment"`
	TimestampUtc  time.Time  `json:"timestampUtc"`
}

// ==========================================
// Payment Secret Management
// ==========================================

type PaymentSecretField int

const (
	SecretFieldPan PaymentSecretField = iota
	SecretFieldCvv
	SecretFieldExpMonth
	SecretFieldExpYear
	SecretFieldExpMonthYearShort
	SecretFieldExpMonthYearLong
	SecretFieldPostalCode
	SecretFieldAuthorizationHeader
	SecretFieldAuthorizationToken
)

type PaymentSecret struct {
	handleId            string
	providerId          string
	last4               string
	expiresAtUtc        *time.Time
	environment         string
	pan                 string
	cvv                 string
	expMonth            string
	expYear             string
	postalCode          string
	authorizationToken  string
	authorizationHeader string
}

func NewPaymentSecret(
	handleId, providerId string,
	pan, cvv, expMonth, expYear, postalCode, authToken, authHeader string,
	expiresAtUtc *time.Time,
	environment string,
) *PaymentSecret {
	env, _ := NormalizeEnvironment(environment)

	var last4 string
	if len(pan) >= 4 {
		last4 = (pan)[len(pan)-4:]
	}

	return &PaymentSecret{
		handleId:            handleId,
		providerId:          providerId,
		last4:               last4,
		expiresAtUtc:        expiresAtUtc,
		environment:         env,
		pan:                 pan,
		cvv:                 cvv,
		expMonth:            normalizeMonth(expMonth),
		expYear:             normalizeYear(expYear),
		postalCode:          postalCode,
		authorizationToken:  authToken,
		authorizationHeader: authHeader,
	}
}

func (s *PaymentSecret) HandleId() string         { return s.handleId }
func (s *PaymentSecret) ProviderId() string       { return s.providerId }
func (s *PaymentSecret) Last4() string            { return s.last4 }
func (s *PaymentSecret) ExpiresAtUtc() *time.Time { return s.expiresAtUtc }
func (s *PaymentSecret) Environment() string      { return s.environment }

func (s *PaymentSecret) Resolve(field PaymentSecretField) string {
	switch field {
	case SecretFieldPan:
		return s.pan
	case SecretFieldCvv:
		return s.cvv
	case SecretFieldExpMonth:
		return s.expMonth
	case SecretFieldExpYear:
		return s.expYear
	case SecretFieldExpMonthYearShort:
		if s.expMonth != "" && len(s.expYear) >= 2 {
			res := fmt.Sprintf("%s/%s", s.expMonth, (s.expYear)[len(s.expYear)-2:])
			return res
		}
	case SecretFieldExpMonthYearLong:
		if s.expMonth != "" && s.expYear != "" {
			res := fmt.Sprintf("%s/%s", s.expMonth, s.expYear)
			return res
		}
	case SecretFieldPostalCode:
		return s.postalCode
	case SecretFieldAuthorizationToken:
		return s.authorizationToken
	case SecretFieldAuthorizationHeader:
		return s.authorizationHeader
	}
	return ""
}

func (s *PaymentSecret) Clear() {
	s.pan = ""
	s.cvv = ""
	s.expMonth = ""
	s.expYear = ""
	s.postalCode = ""
	s.authorizationToken = ""
	s.authorizationHeader = ""
}

func (s *PaymentSecret) MarshalJSON() ([]byte, error) {
	return nil, errors.New("PaymentSecret cannot be serialized")
}

func (s *PaymentSecret) UnmarshalJSON(b []byte) error {
	return errors.New("PaymentSecret cannot be deserialized")
}

func normalizeMonth(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	month, err := strconv.Atoi(value)
	if err != nil {
		return value
	}
	formatted := fmt.Sprintf("%02d", month)
	return formatted
}

func normalizeYear(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	val := value
	if len(val) == 2 {
		formatted := "20" + val
		return formatted
	}
	return value
}

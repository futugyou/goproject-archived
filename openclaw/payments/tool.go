package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type PaymentTool struct {
	runtime           PaymentRuntimeService
	defaultProviderID string
	environment       string
}

func NewPaymentTool(runtime PaymentRuntimeService, defaultProviderID, environment string) *PaymentTool {
	if strings.TrimSpace(defaultProviderID) == "" {
		defaultProviderID = "mock"
	}
	if strings.TrimSpace(environment) == "" {
		environment = PaymentEnvTest
	}

	return &PaymentTool{
		runtime:           runtime,
		defaultProviderID: defaultProviderID,
		environment:       environment,
	}
}

func (a *PaymentTool) Name() string {
	return "payment"
}

func (a *PaymentTool) Description() string {
	return "Native OpenClaw payment capability for setup status, funding source listing, virtual card handles, machine payments, and payment status. Results never include raw card or authorization secrets."
}

func (a *PaymentTool) ParameterSchema() string {
	return `{
	  "type": "object",
	  "properties": {
	    "action": {
	      "type": "string",
	      "enum": ["setup_status", "list_funding_sources", "issue_virtual_card", "execute_machine_payment", "get_payment_status"]
	    },
	    "provider": { "type": "string" },
	    "environment": { "type": "string", "enum": ["test", "live"] },
	    "funding_source_id": { "type": "string" },
	    "merchant": { "type": "string" },
	    "merchant_url": { "type": "string" },
	    "amount_minor": { "type": "integer", "minimum": 1 },
	    "currency": { "type": "string" },
	    "purpose": { "type": "string" },
	    "valid_minutes": { "type": "integer" },
	    "payment_id": { "type": "string" },
	    "resource_url": { "type": "string" },
	    "challenge_id": { "type": "string" },
	    "protocol": { "type": "string" }
	  },
	  "required": ["action"]
	}`
}

type PaymentPolicyDeniedError struct {
	Message string
}

func (e *PaymentPolicyDeniedError) Error() string {
	return e.Message
}

func (t *PaymentTool) Execute(ctx context.Context, argumentsJson string, toolCtx *core.ToolExecutionContext) string {
	return t.executeCore(ctx, argumentsJson, toolCtx)
}

func (t *PaymentTool) executeCore(ctx context.Context, argumentsJson string, toolCtx *core.ToolExecutionContext) string {
	result, err := t.processRequest(ctx, argumentsJson, toolCtx)
	if err != nil {
		var policyErr *PaymentPolicyDeniedError
		if errors.As(err, &policyErr) {
			return t.errorResponse(policyErr.Error(), "payment_policy_denied")
		}
		return t.errorResponse(err.Error(), "invalid_request")
	}

	return result
}

func (t *PaymentTool) processRequest(ctx context.Context, argumentsJson string, toolCtx *core.ToolExecutionContext) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &payload); err != nil {
		return "", err
	}

	action := readString(payload, "action")
	if strings.TrimSpace(action) == "" {
		return "", errors.New("action is required.")
	}

	provider := readString(payload, "provider")
	if provider == "" {
		provider = t.defaultProviderID
	}

	envStr := readString(payload, "environment")
	if envStr == "" {
		envStr = t.environment
	}

	env, err := NormalizeEnvironment(envStr)
	if err != nil {
		return "", err
	}
	execCtx := PaymentExecutionContext{
		Environment: env,
	}

	if toolCtx != nil {
		execCtx.SessionId = toolCtx.Session.Id
		execCtx.ChannelId = toolCtx.Session.ChannelId
		execCtx.SenderId = toolCtx.Session.SenderId
		execCtx.CorrelationId = toolCtx.TurnContext.CorrelationId
	}

	var res any

	switch action {
	case ActionSetupStatus:
		res, err = t.runtime.GetSetupStatus(ctx, provider)
	case ActionListFundingSources:
		res, err = t.runtime.ListFundingSources(ctx, provider, execCtx)
	case ActionIssueVirtualCard:
		req, buildErr := buildVirtualCardRequest(payload, provider, execCtx.Environment)
		if buildErr != nil {
			return "", buildErr
		}
		res, err = t.runtime.IssueVirtualCard(ctx, req, execCtx)
	case ActionExecuteMachinePayment:
		req, buildErr := buildMachinePaymentRequest(payload, provider, execCtx.Environment)
		if buildErr != nil {
			return "", buildErr
		}
		res, err = t.runtime.ExecuteMachinePayment(ctx, req, execCtx)
	case ActionGetPaymentStatus:
		paymentID, readErr := readRequiredString(payload, "payment_id")
		if readErr != nil {
			return "", readErr
		}
		res, err = t.runtime.GetPaymentStatus(ctx, paymentID, provider, execCtx)
	default:
		return "", fmt.Errorf("unsupported payment action '%s'.", action)
	}

	if err != nil {
		return "", err
	}

	return serialize(res)
}

func buildVirtualCardRequest(root map[string]any, provider, environment string) (VirtualCardRequest, error) {
	merchant, err := readRequiredString(root, "merchant")
	if err != nil {
		return VirtualCardRequest{}, err
	}

	amountMinor, err := readRequiredPositiveLong(root, "amount_minor")
	if err != nil {
		return VirtualCardRequest{}, err
	}

	currency := readString(root, "currency")
	if currency == "" {
		currency = "USD"
	}

	validMinutes := readLong(root, "valid_minutes")
	minutesVal := int64(30)
	if validMinutes != nil {
		minutesVal = *validMinutes
	}

	// C# Math.Clamp(val, 1, 1440)
	clampedMinutes := util.Clamp(minutesVal, 1, 1440)

	return VirtualCardRequest{
		ProviderId:      provider,
		FundingSourceId: readString(root, "funding_source_id"),
		MerchantName:    merchant,
		MerchantUrl:     readString(root, "merchant_url"),
		AmountMinor:     amountMinor,
		Currency:        currency,
		Purpose:         readString(root, "purpose"),
		ValidUntilUtc:   new(time.Now().UTC().Add(time.Duration(clampedMinutes) * time.Minute)),
		Environment:     environment,
	}, nil
}

func buildMachinePaymentRequest(root map[string]any, provider, environment string) (MachinePaymentRequest, error) {
	amountMinor, err := readRequiredPositiveLong(root, "amount_minor")
	if err != nil {
		return MachinePaymentRequest{}, err
	}

	protocol := readString(root, "protocol")
	if protocol == "" {
		protocol = "http-402"
	}

	currency := readString(root, "currency")
	if currency == "" {
		currency = "USD"
	}

	return MachinePaymentRequest{
		ProviderId:      provider,
		Environment:     environment,
		FundingSourceId: readString(root, "funding_source_id"),
		Challenge: MachinePaymentChallenge{
			ChallengeId:  readString(root, "challenge_id"),
			Protocol:     protocol,
			ResourceUrl:  readString(root, "resource_url"),
			MerchantName: readString(root, "merchant"),
			AmountMinor:  amountMinor,
			Currency:     currency,
			ProviderId:   provider,
		},
	}, nil
}

func serialize(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t *PaymentTool) errorResponse(message string, code ...string) string {
	errCode := "invalid_request"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}

	res := map[string]string{
		"status":  "error",
		"code":    errCode,
		"message": message,
	}

	b, _ := json.Marshal(res)
	return string(b)
}

func readString(m map[string]any, property string) string {
	val, ok := m[property]
	if !ok || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func readRequiredString(m map[string]any, property string) (string, error) {
	val := readString(m, property)
	if val == "" {
		return "", fmt.Errorf("%s is required.", property)
	}
	return val, nil
}

func readLong(m map[string]any, property string) *int64 {
	val, ok := m[property]
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case float64:
		i := int64(v)
		return &i
	case int64:
		return &v
	case int:
		i := int64(v)
		return &i
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &parsed
		}
	}
	return nil
}

func readRequiredPositiveLong(m map[string]any, property string) (int64, error) {
	val := readLong(m, property)
	if val == nil {
		return 0, fmt.Errorf("%s is required and must be an integer.", property)
	}
	if *val <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0.", property)
	}
	return *val, nil
}

package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/futugyou/openclaw/core"
)

type PaymentSentinelSubstitutionService struct {
	runtime PaymentRuntimeService
}

var paymentSentinelRegex = regexp.MustCompile(`\{\{payment\.vcard:([A-Za-z0-9_.:-]+):(pan|cvv|exp_month|exp_year|exp_mm_yy|exp_mm_yyyy|postal_code)\}\}`)

func NewPaymentSentinelSubstitutionService(runtime PaymentRuntimeService) *PaymentSentinelSubstitutionService {
	return &PaymentSentinelSubstitutionService{
		runtime: runtime,
	}
}

func (p *PaymentSentinelSubstitutionService) Substitute(ctx context.Context, sentinelContext *core.SentinelSubstitutionContext) (*core.SentinelSubstitutionResult, error) {
	if sentinelContext.ToolName != "browser" || !paymentSentinelRegex.MatchString(sentinelContext.ArgumentsJson) {
		return &core.SentinelSubstitutionResult{
			ExecutionArgumentsJson: sentinelContext.ArgumentsJson,
			PersistedArgumentsJson: sentinelContext.ArgumentsJson,
			Substituted:            false,
		}, nil
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(sentinelContext.ArgumentsJson), &rawMap); err != nil {
		return nil, fmt.Errorf("invalid json format: %w", err)
	}

	actionVal, ok := rawMap["action"].(string)
	if !ok || actionVal != "fill" {
		return nil, fmt.Errorf("payment sentinels may only be resolved inside browser fill execution")
	}

	valueVal, ok := rawMap["value"].(string)
	if !ok {
		return nil, fmt.Errorf("payment sentinel fill requires a string value")
	}

	matches := paymentSentinelRegex.FindAllStringSubmatchIndex(valueVal, -1)
	if len(matches) == 0 {
		return &core.SentinelSubstitutionResult{
			ExecutionArgumentsJson: sentinelContext.ArgumentsJson,
			PersistedArgumentsJson: sentinelContext.ArgumentsJson,
			Substituted:            false,
		}, nil
	}

	paymentEnv := sentinelContext.PaymentEnvironment
	executionValue := valueVal
	fullMatches := paymentSentinelRegex.FindAllStringSubmatch(valueVal, -1)

	for _, match := range fullMatches {
		fullMatch := match[0]
		handleID := match[1]
		fieldStr := match[2]

		field, err := parseField(fieldStr)
		if err != nil {
			return nil, err
		}

		approval := ApprovalRequest{
			Action:      ActionBrowserSentinelFill,
			Summary:     fmt.Sprintf("Fill browser checkout field using payment handle %s.", handleID),
			ProviderId:  sentinelContext.PaymentProviderId,
			Environment: paymentEnv,
			SessionId:   sentinelContext.SessionId,
			ChannelId:   sentinelContext.ChannelId,
			SenderId:    sentinelContext.SenderId,
			Severity:    SeverityCritical,
		}

		raw, err := p.runtime.ResolveSecretFieldForApprovedBoundary(ctx, handleID, field, approval)
		if err != nil {
			return nil, err
		}

		executionValue = bytes.NewBuffer(
			bytes.ReplaceAll([]byte(executionValue), []byte(fullMatch), []byte(raw)),
		).String()
	}

	executionJSON, err := replaceRootStringProperty(sentinelContext.ArgumentsJson, "value", executionValue)
	if err != nil {
		return nil, err
	}

	return &core.SentinelSubstitutionResult{
		ExecutionArgumentsJson: executionJSON,
		PersistedArgumentsJson: sentinelContext.ArgumentsJson,
		Substituted:            true,
	}, nil
}

func parseField(field string) (PaymentSecretField, error) {
	switch field {
	case "pan":
		return SecretFieldPan, nil
	case "cvv":
		return SecretFieldCvv, nil
	case "exp_month":
		return SecretFieldExpMonth, nil
	case "exp_year":
		return SecretFieldExpYear, nil
	case "exp_mm_yy":
		return SecretFieldExpMonthYearShort, nil
	case "exp_mm_yyyy":
		return SecretFieldExpMonthYearLong, nil
	case "postal_code":
		return SecretFieldPostalCode, nil
	default:
		return 0, fmt.Errorf("unsupported payment sentinel field '%s'", field)
	}
}

func replaceRootStringProperty(jsonStr, propertyName, newValue string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader([]byte(jsonStr)))

	var doc map[string]json.RawMessage
	if err := decoder.Decode(&doc); err != nil {
		return "", err
	}

	encodedNewValue, err := json.Marshal(newValue)
	if err != nil {
		return "", err
	}
	doc[propertyName] = encodedNewValue

	result, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

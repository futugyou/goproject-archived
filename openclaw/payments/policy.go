package payments

import (
	"context"
	"fmt"
)

type IPaymentPolicy interface {
	Evaluate(ctx context.Context, request ApprovalRequest, approvalServiceAvailable bool) (*PaymentPolicyDecision, error)
}

var _ IPaymentPolicy = (*DefaultPaymentPolicy)(nil)

type DefaultPaymentPolicy struct {
	allowTestModeWithoutApproval   bool
	denyLiveWithoutApprovalService bool
	maxLiveAmountMinor             *int64
}

func NewDefaultPaymentPolicy(allowTestModeWithoutApproval *bool,
	denyLiveWithoutApprovalService *bool,
	maxLiveAmountMinor *int64) *DefaultPaymentPolicy {
	approval := true
	if allowTestModeWithoutApproval != nil {
		approval = *allowTestModeWithoutApproval
	}
	deny := true
	if denyLiveWithoutApprovalService != nil {
		deny = *denyLiveWithoutApprovalService
	}
	return &DefaultPaymentPolicy{
		allowTestModeWithoutApproval:   approval,
		denyLiveWithoutApprovalService: deny,
		maxLiveAmountMinor:             maxLiveAmountMinor,
	}
}

func (d *DefaultPaymentPolicy) Evaluate(ctx context.Context, request ApprovalRequest, approvalServiceAvailable bool) (*PaymentPolicyDecision, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	deny := func(reason string) *PaymentPolicyDecision {
		return &PaymentPolicyDecision{
			Decision: DecisionDeny,
			Reason:   reason,
		}
	}

	environment, ok := TryNormalizeEnvironment(request.Environment)
	if !ok {
		return deny(fmt.Sprintf("Unsupported payment environment '%s'.", request.Environment)), nil
	}

	var live = environment == PaymentEnvLive
	if live && d.maxLiveAmountMinor != nil && request.AmountMinor != nil && *request.AmountMinor > *d.maxLiveAmountMinor {
		return deny(fmt.Sprintf("Live payment amount %d exceeds configured limit %d.", *request.AmountMinor, *d.maxLiveAmountMinor)), nil
	}

	if !live && d.allowTestModeWithoutApproval {
		return &PaymentPolicyDecision{
			Decision: DecisionAllow,
			Reason:   "Deterministic test-mode payment allowed by policy.",
		}, nil
	}

	if live && !approvalServiceAvailable && d.denyLiveWithoutApprovalService {
		return deny("Live payment denied because no payment approval service is registered."), nil
	}

	return &PaymentPolicyDecision{
		Decision: DecisionRequireApproval,
		Reason:   "Money-moving payment action requires critical approval.",
	}, nil
}

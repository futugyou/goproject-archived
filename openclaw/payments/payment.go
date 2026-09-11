package payments

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func buildApprovalRequest(
	action,
	summary,
	providerId,
	merchantName string,
	amountMinor int64,
	currency string,
	expiresAtUtc *time.Time,
	execContext PaymentExecutionContext) ApprovalRequest {
	return ApprovalRequest{
		Action:       action,
		Summary:      summary,
		MerchantName: merchantName,
		AmountMinor:  amountMinor,
		Currency:     currency,
		ProviderId:   providerId,
		ExpiresAtUtc: expiresAtUtc,
		Environment:  execContext.Environment,
		AgentId:      execContext.SenderId,
		WorkspaceId:  execContext.WorkspaceId,
		SessionId:    execContext.SessionId,
		ChannelId:    execContext.ChannelId,
		SenderId:     execContext.SenderId,
		CliConfirmed: execContext.CliConfirmed,
	}
}

func repareSecretForVault(
	secret PaymentSecret,
	handleId,
	providerId,
	environment string,
	expiresAtUtc *time.Time) PaymentSecret {
	if expiresAtUtc == nil {
		expiresAtUtc = secret.ExpiresAtUtc()
	}
	return PaymentSecret{
		handleId:            handleId,
		providerId:          providerId,
		pan:                 secret.Resolve(SecretFieldPan),
		cvv:                 secret.Resolve(SecretFieldCvv),
		expMonth:            secret.Resolve(SecretFieldExpMonth),
		expYear:             secret.Resolve(SecretFieldExpYear),
		postalCode:          secret.Resolve(SecretFieldPostalCode),
		authorizationToken:  secret.Resolve(SecretFieldAuthorizationToken),
		authorizationHeader: secret.Resolve(SecretFieldAuthorizationHeader),
		expiresAtUtc:        expiresAtUtc,
		environment:         environment,
	}
}

func validateMachinePaymentRequest(request MachinePaymentRequest) error {
	if request.Challenge.AmountMinor <= 0 {
		return errors.New("amount_minor must be greater than 0")
	}

	if request.Challenge.Currency == "" {
		return errors.New("currency is required")
	}

	return nil
}

func validateVirtualCardRequest(request VirtualCardRequest) error {
	if request.MerchantName == "" {
		return errors.New("merchant is required")
	}

	if request.AmountMinor <= 0 {
		return errors.New("amount_minor must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	return nil
}

type PaymentRuntimeService struct {
	vault             IPaymentSecretVault
	audit             IPaymentAuditSink
	defaultProviderId string
	policy            IPaymentPolicy
	approval          IPaymentApprovalService
}

func (p *PaymentRuntimeService) RetrieveMachineAuthorizationOnce(ctx context.Context, paymentId string) (*PaymentSecret, error) {
	return p.vault.TryRetrieve(ctx, paymentId, "machine-payment-authorization")
}

func (p *PaymentRuntimeService) requireApprovalOrPolicyAllow(ctx context.Context, request ApprovalRequest) error {
	providerId := request.ProviderId
	if providerId == "" {
		providerId = p.defaultProviderId
	}
	if err := p.audit.Record(ctx, PaymentAuditEvent{
		EventType:    "approval_requested",
		ProviderId:   providerId,
		MerchantName: request.MerchantName,
		AmountMinor:  request.AmountMinor,
		Currency:     request.Currency,
		Environment:  request.Environment,
	}); err != nil {
		return err
	}

	decision, err := p.policy.Evaluate(ctx, request, p.approval != nil || request.CliConfirmed)
	if err != nil {
		return err
	}

	if decision.Denied() {
		p.audit.Record(ctx, PaymentAuditEvent{
			EventType:    "policy_denied",
			ProviderId:   providerId,
			MerchantName: request.MerchantName,
			AmountMinor:  request.AmountMinor,
			Currency:     request.Currency,
			Decision:     decision.Decision,
			Reason:       decision.Reason,
			Environment:  request.Environment,
		})

		return errors.New(decision.Reason)
	}

	if decision.Allowed() {
		return nil
	}

	var approval *ApprovalResult
	if request.CliConfirmed {
		approval = &ApprovalResult{
			Approved: true,
			Source:   "cli --yes",
			Reason:   "Operator confirmed live payment command noninteractively.",
		}
	} else if p.approval == nil {
		return errors.New("Payment requires approval but no approval service is registered.")
	} else {
		approval, err = p.approval.RequestApproval(ctx, request)
	}

	if err != nil {
		return err
	}

	eventType := "approval_denied"
	decisionstr := "denied"
	if approval.Approved {
		eventType = "approval_granted"
		decisionstr = "approved"
	}

	if err := p.audit.Record(ctx, PaymentAuditEvent{
		EventType:    eventType,
		ProviderId:   providerId,
		MerchantName: request.MerchantName,
		AmountMinor:  request.AmountMinor,
		Currency:     request.Currency,
		Decision:     decisionstr,
		Reason:       approval.Reason,
		Environment:  request.Environment,
	}); err != nil {
		return err
	}

	if !approval.Approved {
		msg := approval.Reason
		if msg == "" {
			msg = "Payment approval denied."
		}

		return errors.New(msg)
	}

	return nil
}

func (p *PaymentRuntimeService) ResolveSecretFieldForApprovedBoundary(
	ctx context.Context,
	handleId string,
	field PaymentSecretField,
	approval ApprovalRequest,
) (string, error) {
	secret, err := p.vault.TryRetrieve(ctx, handleId, "sentinel-substitution")
	if err != nil {
		return "", errors.New("Payment secret is missing, expired, or revoked.")
	}

	effectiveApproval := approval
	if approval.ProviderId == "" {
		effectiveApproval.ProviderId = secret.ProviderId()
	} else {
		effectiveApproval.ProviderId = approval.ProviderId
	}

	if approval.Environment == "" {
		effectiveApproval.Environment = secret.Environment()
	} else {
		e, err := NormalizeEnvironment(approval.Environment)
		if err != nil {
			return "", err
		}
		effectiveApproval.Environment = e
	}

	providerId := effectiveApproval.ProviderId
	if providerId == "" {
		providerId = secret.providerId
	}
	p.audit.Record(ctx, PaymentAuditEvent{
		EventType:    "browser_sentinel_substitution_attempted",
		ProviderId:   providerId,
		HandleId:     handleId,
		MerchantName: effectiveApproval.MerchantName,
		AmountMinor:  effectiveApproval.AmountMinor,
		Currency:     effectiveApproval.Currency,
		Environment:  effectiveApproval.Environment,
	})

	if err := p.requireApprovalOrPolicyAllow(ctx, effectiveApproval); err != nil {
		p.audit.Record(ctx, PaymentAuditEvent{
			EventType:    "browser_sentinel_substitution_denied",
			ProviderId:   providerId,
			HandleId:     handleId,
			MerchantName: effectiveApproval.MerchantName,
			AmountMinor:  effectiveApproval.AmountMinor,
			Currency:     effectiveApproval.Currency,
			Status:       "denied",
			Environment:  effectiveApproval.Environment,
		})
		return "", err
	}

	var value = secret.Resolve(field)
	if value == "" {
		return "", fmt.Errorf("Payment secret field '%v' is not available", field)
	}

	p.audit.Record(ctx, PaymentAuditEvent{
		EventType:    "browser_sentinel_substitution_approved",
		ProviderId:   providerId,
		HandleId:     handleId,
		MerchantName: effectiveApproval.MerchantName,
		AmountMinor:  effectiveApproval.AmountMinor,
		Currency:     effectiveApproval.Currency,
		Status:       "approved",
		Environment:  effectiveApproval.Environment,
	})
	return value, nil
}

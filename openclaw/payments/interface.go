package payments

import (
	"context"
	"time"
)

type IPaymentProvider interface {
	GetProviderId() string
	GetSetupStatus(ctx context.Context) (*PaymentSetupStatus, error)
	ListFundingSources(tx context.Context, execContext PaymentExecutionContext) ([]FundingSource, error)
	IssueVirtualCard(ctx context.Context,
		request VirtualCardRequest,
		execContext PaymentExecutionContext,
	) (*VirtualCardIssueResult, error)
	ExecuteMachinePayment(ctx context.Context,
		request MachinePaymentRequest,
		execContext PaymentExecutionContext,
	) (*MachinePaymentProviderResult, error)
	GetPaymentStatus(ctx context.Context,
		paymentIdOrHandleId string,
		execContext PaymentExecutionContext,
	) (*PaymentStatus, error)
}

type IPaymentApprovalService interface {
	RequestApproval(ctx context.Context, request ApprovalRequest) (*ApprovalResult, error)
}

type IPaymentSecretVault interface {
	Store(ctx context.Context, secret PaymentSecret, ttl time.Duration, retrieveOnce bool) (string, error)
	TryRetrieve(ctx context.Context, handleId, purpose string) (*PaymentSecret, error)
	Revoke(ctx context.Context, handleId, reason string) error
}

type IPaymentAuditSink interface {
	Record(ctx context.Context, auditEvent PaymentAuditEvent) error
}

type IPaymentRedactor interface {
	Redact(value string) (string, error)
}

type ISentinelSubstitutionProvider interface {
	GetProviderId() string
	CanSubstitute(value string) bool
	Substitute(ctx context.Context, value string, execContext PaymentExecutionContext) (string, error)
}

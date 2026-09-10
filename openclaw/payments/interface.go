package payments

import (
	"context"
)

type IPaymentApprovalService interface {
	RequestApproval(ctx context.Context, request ApprovalRequest) (*ApprovalResult, error)
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

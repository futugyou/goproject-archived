package payments

import (
	"context"
	"sync"
)

type IPaymentAuditSink interface {
	Record(ctx context.Context, auditEvent PaymentAuditEvent) error
}

var _ IPaymentAuditSink = (*InMemoryPaymentAuditSink)(nil)

type InMemoryPaymentAuditSink struct {
	events []PaymentAuditEvent

	mu sync.Mutex
}

func (i *InMemoryPaymentAuditSink) Record(ctx context.Context, auditEvent PaymentAuditEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.events = append(i.events, auditEvent)
	return nil
}

func (i *InMemoryPaymentAuditSink) Snapshot() []PaymentAuditEvent {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.events
}

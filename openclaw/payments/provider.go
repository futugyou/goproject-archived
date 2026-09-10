package payments

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

var _ IPaymentProvider = (*MockPaymentProvider)(nil)

type MockPaymentProvider struct {
	sequence                 atomic.Int64
	providerId               string
	FundingSourceDisplayName string

	mu       sync.Mutex
	statuses map[string]*PaymentStatus
}

func NewMockPaymentProvider(providerId string,
	fundingSourceDisplayName string) *MockPaymentProvider {
	if providerId == "" {
		providerId = "mock"
	}
	if fundingSourceDisplayName == "" {
		fundingSourceDisplayName = "Mock Visa ending 4242"
	}

	return &MockPaymentProvider{
		providerId:               providerId,
		FundingSourceDisplayName: fundingSourceDisplayName,
		statuses:                 map[string]*PaymentStatus{},
	}
}

func (m *MockPaymentProvider) ExecuteMachinePayment(ctx context.Context, request MachinePaymentRequest, execContext PaymentExecutionContext) (*MachinePaymentProviderResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	sequence := m.sequence.Add(1)
	paymentId := fmt.Sprintf("mp_mock_%06d", sequence)
	providerReference := request.Challenge.ChallengeId
	if providerReference == "" {
		providerReference = fmt.Sprintf("challenge_%06d", sequence)
	}

	protocol := request.Challenge.Protocol
	if protocol == "" {
		protocol = "mock-402"
	}

	now := time.Now().UTC()
	var result = MachinePaymentResult{
		PaymentId:         paymentId,
		ProviderId:        m.providerId,
		Status:            "completed",
		MerchantName:      request.Challenge.MerchantName,
		AmountMinor:       request.Challenge.AmountMinor,
		Currency:          request.Challenge.Currency,
		CreatedAtUtc:      now,
		ProviderReference: providerReference,
		SafeMetadata: map[string]string{
			"protocol": protocol,
		}}

	m.statuses[paymentId] = &PaymentStatus{
		PaymentId:         paymentId,
		ProviderId:        m.providerId,
		Status:            result.Status,
		MerchantName:      result.MerchantName,
		AmountMinor:       result.AmountMinor,
		Currency:          result.Currency,
		UpdatedAtUtc:      &result.CreatedAtUtc,
		ProviderReference: result.ProviderReference,
	}

	var secret = &PaymentSecret{
		handleId:            paymentId,
		providerId:          m.providerId,
		authorizationToken:  fmt.Sprintf("payment_mock_secret_token_%06d", sequence),
		authorizationHeader: fmt.Sprintf("Payment payment_mock_secret_token_%06d", sequence),
		expiresAtUtc:        new(now.Add(5 * time.Minute)),
		environment:         execContext.Environment,
	}

	return &MachinePaymentProviderResult{
		Result:                    result,
		ScopedAuthorizationSecret: secret,
	}, nil
}

func (m *MockPaymentProvider) GetPaymentStatus(ctx context.Context, paymentIdOrHandleId string, execContext PaymentExecutionContext) (*PaymentStatus, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.statuses[paymentIdOrHandleId]; ok {
		return status, nil
	}

	now := time.Now()
	return &PaymentStatus{
		PaymentId:    paymentIdOrHandleId,
		ProviderId:   m.providerId,
		Status:       "not_found",
		UpdatedAtUtc: &now,
	}, nil
}

func (m *MockPaymentProvider) GetProviderId() string {
	return m.providerId
}

func (m *MockPaymentProvider) GetSetupStatus(ctx context.Context) (*PaymentSetupStatus, error) {
	return &PaymentSetupStatus{
		ProviderId: m.providerId,
		Enabled:    true,
		Installed:  true,
		Version:    "mock-1",
		Mode:       "test",
		Status:     "ready",
		Message:    "Deterministic mock payment provider is ready.",
	}, nil
}

func (m *MockPaymentProvider) IssueVirtualCard(ctx context.Context, request VirtualCardRequest, execContext PaymentExecutionContext) (*VirtualCardIssueResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	sequence := m.sequence.Add(1)
	handleId := fmt.Sprintf("pvh_mock_%06d", sequence)
	spendId := fmt.Sprintf("mock_spend_%06d", sequence)

	now := time.Now()
	validUntil := request.ValidUntilUtc
	if validUntil == nil {
		validUntil = new(now.Add(30 * time.Minute))
	}
	var pan = "4242424242424242"
	var secret = PaymentSecret{
		handleId:     handleId,
		providerId:   m.providerId,
		pan:          pan,
		cvv:          "123",
		expMonth:     "12",
		expYear:      "2030",
		postalCode:   "94107",
		expiresAtUtc: validUntil,
		environment:  execContext.Environment,
	}
	var handle = VirtualCardHandle{
		HandleId:           handleId,
		ProviderId:         m.providerId,
		Last4:              "4242",
		TargetMerchantName: request.MerchantName,
		IssuedAtUtc:        now,
		ValidUntilUtc:      validUntil,
		SpendRequestId:     spendId,
		Status:             "issued",
		Environment:        execContext.Environment,
	}

	m.statuses[handleId] = &PaymentStatus{
		PaymentId:         handleId,
		ProviderId:        m.providerId,
		Status:            "issued",
		MerchantName:      request.MerchantName,
		AmountMinor:       request.AmountMinor,
		Currency:          request.Currency,
		UpdatedAtUtc:      &now,
		ProviderReference: spendId,
	}

	return &VirtualCardIssueResult{
		Handle: handle,
		Secret: &secret,
	}, nil
}

func (m *MockPaymentProvider) ListFundingSources(tx context.Context, execContext PaymentExecutionContext) ([]FundingSource, error) {
	return []FundingSource{
		{
			FundingSourceId: "mock_fs_visa_4242",
			ProviderId:      m.providerId,
			DisplayName:     m.FundingSourceDisplayName,
			Type:            "card",
			Last4:           "4242",
			Currency:        "USD",
			TestMode:        true,
			Available:       true,
		},
	}, nil
}

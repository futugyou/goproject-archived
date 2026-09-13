package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/futugyou/openclaw/util"
	"github.com/google/uuid"
)

type StripeLinkOptions struct {
	ProviderId           string
	CliPath              string
	Mode                 string
	Timeout              time.Duration
	WorkingDirectory     string
	EnvironmentVariables map[string]string
}

func NewStripeLinkOptions() *StripeLinkOptions {
	return &StripeLinkOptions{
		ProviderId:           "stripe-link",
		CliPath:              "link-cli",
		Mode:                 "test",
		Timeout:              time.Second * 30,
		EnvironmentVariables: map[string]string{},
	}
}

type LinkCliCommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

type ILinkCliCommandRunner interface {
	Run(
		ctx context.Context,
		executable string,
		arguments []string,
		workingDirectory string,
		environment map[string]string,
		timeout time.Duration) (*LinkCliCommandResult, error)
}

type LinkCliProcessRunner struct {
	redactor PaymentSensitiveDataRedactor
	logger   *slog.Logger
}

func NewLinkCliProcessRunner(logger *slog.Logger) *LinkCliProcessRunner {
	if logger == nil {
		logger = slog.Default()
	}

	return &LinkCliProcessRunner{
		logger:   logger,
		redactor: PaymentSensitiveDataRedactor{},
	}
}

func (l *LinkCliProcessRunner) Run(ctx context.Context, executable string, arguments []string, workingDirectory string, environment map[string]string, timeout time.Duration) (*LinkCliCommandResult, error) {
	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 2. Prepare Command
	cmd := exec.CommandContext(execCtx, executable, arguments...)

	// Kills child process tree when context times out or is cancelled
	cmd.WaitDelay = 2 * time.Second

	if workingDirectory != "" {
		cmd.Dir = workingDirectory
	}

	// Environment variables setup
	if len(environment) > 0 {
		cmd.Env = os.Environ()
		for k, v := range environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Buffers to capture Stdout and Stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// 3. Start and Wait for execution
	err := cmd.Run()

	rawStdout := stdoutBuf.String()
	rawStderr := stderrBuf.String()

	// 4. Handle Timeout / Context Cancellation
	if execCtx.Err() == context.DeadlineExceeded {
		return &LinkCliCommandResult{
			ExitCode: -1,
			Stdout:   l.redactor.Redact(rawStdout),
			Stderr:   l.redactor.Redact(rawStderr),
			TimedOut: true,
		}, nil
	}

	// 5. Handle Errors and Exit Codes
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Failed to launch process or binary not found
			return &LinkCliCommandResult{
				ExitCode: -1,
				Stderr:   err.Error(),
			}, nil
		}
	}

	stdout := l.redactor.Redact(rawStdout)
	stderr := l.redactor.Redact(rawStderr)

	if exitCode != 0 {
		l.logger.Warn("link-cli exited with non-zero status",
			"exitCode", exitCode,
			"stderr", stderr,
		)
	}

	return &LinkCliCommandResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		TimedOut: false,
	}, nil
}

type StripeLinkPaymentProvider struct {
	options StripeLinkOptions
	runner  ILinkCliCommandRunner
}

func NewStripeLinkPaymentProvider(options StripeLinkOptions, runner ILinkCliCommandRunner) *StripeLinkPaymentProvider {
	if runner == nil {
		runner = NewLinkCliProcessRunner(nil)
	}

	return &StripeLinkPaymentProvider{
		runner:  runner,
		options: options,
	}
}

func (s *StripeLinkPaymentProvider) runJson(ctx context.Context, arguments []string) (*LinkCliCommandResult, error) {
	return s.runner.Run(ctx, s.options.CliPath, arguments, s.options.WorkingDirectory, s.options.EnvironmentVariables, s.options.Timeout)
}

func (s *StripeLinkPaymentProvider) ExecuteMachinePayment(ctx context.Context, request MachinePaymentRequest, execContext PaymentExecutionContext) (*MachinePaymentProviderResult, error) {
	args := []string{
		"machine-payment",
		"execute",
		"--json",
		"--mode",
		s.options.Mode,
		"--amount",
		fmt.Sprintf("%d", request.Challenge.AmountMinor),
		"--currency",
		request.Challenge.Currency,
	}
	if request.Challenge.ChallengeId != "" {
		args = append(args, "--challenge")
		args = append(args, request.Challenge.ChallengeId)
	}
	if request.Challenge.ResourceUrl != "" {
		args = append(args, "--resource")
		args = append(args, request.Challenge.ResourceUrl)
	}

	result, err := s.runJson(ctx, args)
	if err != nil || result.ExitCode != 0 {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else {
			msg = result.Stderr
		}
		return nil, fmt.Errorf("Stripe Link machine payment failed: %s", msg)
	}

	return s.parseMachinePayment(result.Stdout, request, execContext)
}

type MachinePaymentRaw struct {
	PaymentID         string `json:"payment_id"`
	PaymentIDCamel    string `json:"paymentId"`
	ID                string `json:"id"`
	Status            string `json:"status"`
	AuthHeader        string `json:"authorization_header"`
	AuthHeaderCamel   string `json:"authorizationHeader"`
	AuthToken         string `json:"authorization_token"`
	AuthTokenCamel    string `json:"authorizationToken"`
	Token             string `json:"token"`
	ProviderReference string `json:"provider_reference"`
	ProviderRefCamel  string `json:"providerReference"`
}

func (r *MachinePaymentRaw) GetPaymentID() string {
	if r.PaymentID != "" {
		return r.PaymentID
	}
	if r.PaymentIDCamel != "" {
		return r.PaymentIDCamel
	}
	return r.ID
}

func (r *MachinePaymentRaw) GetAuthHeader() string {
	if r.AuthHeader != "" {
		return r.AuthHeader
	}
	return r.AuthHeaderCamel
}

func (r *MachinePaymentRaw) GetToken() string {
	if r.AuthToken != "" {
		return r.AuthToken
	}
	if r.AuthTokenCamel != "" {
		return r.AuthTokenCamel
	}
	return r.Token
}

func (r *MachinePaymentRaw) GetProviderRef() string {
	if r.ProviderReference != "" {
		return r.ProviderReference
	}
	return r.ProviderRefCamel
}

func (s *StripeLinkPaymentProvider) parseMachinePayment(jsonStr string, request MachinePaymentRequest, execContext PaymentExecutionContext) (*MachinePaymentProviderResult, error) {
	var raw MachinePaymentRaw
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	paymentId := raw.GetPaymentID()
	if paymentId == "" {
		paymentId = fmt.Sprintf("stripe_mp_%s", util.CleanUUID())
	}

	auth := raw.GetAuthHeader()
	token := raw.GetToken()
	providerRef := raw.GetProviderRef()

	status := raw.Status
	if status == "" {
		status = "completed"
	}

	var scopedSecret *PaymentSecret
	isAuthEmpty := strings.TrimSpace(auth) == ""
	isTokenEmpty := strings.TrimSpace(token) == ""

	if !(isAuthEmpty && isTokenEmpty) {
		scopedSecret = &PaymentSecret{
			handleId:            paymentId,
			providerId:          s.options.ProviderId,
			authorizationToken:  token,
			authorizationHeader: auth,
			expiresAtUtc:        new(time.Now().UTC().Add(5 * time.Minute)),
			environment:         execContext.Environment,
		}
	}

	return &MachinePaymentProviderResult{
		Result: MachinePaymentResult{
			PaymentId:         paymentId,
			ProviderId:        s.options.ProviderId,
			Status:            status,
			MerchantName:      request.Challenge.MerchantName,
			AmountMinor:       request.Challenge.AmountMinor,
			Currency:          request.Challenge.Currency,
			ProviderReference: providerRef,
		},
		ScopedAuthorizationSecret: scopedSecret,
	}, nil
}

func (s *StripeLinkPaymentProvider) GetPaymentStatus(ctx context.Context, paymentIdOrHandleId string, execContext PaymentExecutionContext) (*PaymentStatus, error) {
	result, err := s.runJson(ctx, []string{"status", "--json", "--mode", s.options.Mode, "--id", paymentIdOrHandleId})
	if err != nil || result.ExitCode != 0 {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else {
			msg = result.Stderr
		}
		return nil, fmt.Errorf("Stripe Link status failed: %s", msg)
	}

	return s.parseStatus(result.Stdout, paymentIdOrHandleId)
}

type paymentStatusAlias struct {
	PaymentID1         *string `json:"payment_id"`
	PaymentID2         *string `json:"paymentId"`
	PaymentID3         *string `json:"id"`
	Status             *string `json:"status"`
	MerchantName1      *string `json:"merchant_name"`
	MerchantName2      *string `json:"merchantName"`
	Amount1            *int64  `json:"amount"`
	Amount2            *int64  `json:"amountMinor"`
	Currency           *string `json:"currency"`
	ProviderReference1 *string `json:"provider_reference"`
	ProviderReference2 *string `json:"providerReference"`
}

func (s *StripeLinkPaymentProvider) parseStatus(jsonBytes string, paymentIdOrHandleId string) (*PaymentStatus, error) {
	var alias paymentStatusAlias
	if err := json.Unmarshal([]byte(jsonBytes), &alias); err != nil {
		return nil, err
	}

	result := &PaymentStatus{
		ProviderId:   s.options.ProviderId,
		Status:       "unknown",
		UpdatedAtUtc: new(time.Now().UTC()),
	}

	switch {
	case alias.PaymentID1 != nil:
		result.PaymentId = *alias.PaymentID1
	case alias.PaymentID2 != nil:
		result.PaymentId = *alias.PaymentID2
	case alias.PaymentID3 != nil:
		result.PaymentId = *alias.PaymentID3
	default:
		result.PaymentId = paymentIdOrHandleId
	}

	if alias.Status != nil {
		result.Status = *alias.Status
	}

	switch {
	case alias.MerchantName1 != nil:
		result.MerchantName = *alias.MerchantName1
	case alias.MerchantName2 != nil:
		result.MerchantName = *alias.MerchantName2
	}

	switch {
	case alias.Amount1 != nil:
		result.AmountMinor = *alias.Amount1
	case alias.Amount2 != nil:
		result.AmountMinor = *alias.Amount2
	}

	if alias.Currency != nil {
		result.Currency = *alias.Currency
	}

	switch {
	case alias.ProviderReference1 != nil:
		result.ProviderReference = *alias.ProviderReference1
	case alias.ProviderReference2 != nil:
		result.ProviderReference = *alias.ProviderReference2
	}

	return result, nil
}

func (s *StripeLinkPaymentProvider) GetProviderId() string {
	return s.options.ProviderId
}

func (s *StripeLinkPaymentProvider) GetSetupStatus(ctx context.Context) (*PaymentSetupStatus, error) {
	result, err := s.runner.Run(
		ctx,
		s.options.CliPath,
		[]string{"--version"},
		s.options.WorkingDirectory,
		s.options.EnvironmentVariables,
		s.options.Timeout,
	)

	if err != nil || result.ExitCode != 0 {
		return &PaymentSetupStatus{
			ProviderId: s.options.ProviderId,
			Enabled:    true,
			Installed:  false,
			Mode:       s.options.Mode,
			Status:     "not_installed",
			Message:    "link-cli was not found or did not start.",
			Requirements: []PaymentSetupRequirement{
				{
					Name:      "link-cli",
					Satisfied: false,
					Message:   result.Stderr,
				},
			},
		}, nil
	}

	firstLine := func(value string) string {
		ss := strings.Split(value, "\r\n")
		if len(ss) > 0 {
			return ss[0]
		}
		return ""
	}

	return &PaymentSetupStatus{
		ProviderId: s.options.ProviderId,
		Enabled:    true,
		Installed:  true,
		Version:    firstLine(result.Stdout),
		Mode:       s.options.Mode,
		Status:     "ready",
		Requirements: []PaymentSetupRequirement{
			{
				Name:      "link-cli",
				Satisfied: true,
			},
		},
	}, nil
}

func (s *StripeLinkPaymentProvider) IssueVirtualCard(ctx context.Context, request VirtualCardRequest, execContext PaymentExecutionContext) (*VirtualCardIssueResult, error) {
	args := []string{
		"virtual-card",
		"issue",
		"--json",
		"--mode",
		s.options.Mode,
		"--merchant",
		request.MerchantName,
		"--amount",
		fmt.Sprintf("%d", request.AmountMinor),
		"--currency",
		request.Currency,
	}
	if request.FundingSourceId != "" {
		args = append(args, "--funding-source")
		args = append(args, request.FundingSourceId)
	}

	result, err := s.runJson(ctx, args)
	if err != nil || result.ExitCode != 0 {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else {
			msg = result.Stderr
		}
		return nil, fmt.Errorf("Stripe Link virtual card issue failed: %s", msg)
	}

	return s.parseVirtualCardIssue(result.Stdout, request, execContext)
}

type stripeLinkDTO struct {
	HandleID      *string `json:"handle_id"`
	HandleIDCamel *string `json:"handleId"`
	ID            *string `json:"id"`

	PAN             *string `json:"pan"`
	CardNumber      *string `json:"card_number"`
	CardNumberCamel *string `json:"cardNumber"`

	CVV *string `json:"cvv"`
	CVC *string `json:"cvc"`

	ExpMonth      *string `json:"exp_month"`
	ExpMonthCamel *string `json:"expMonth"`

	ExpYear      *string `json:"exp_year"`
	ExpYearCamel *string `json:"expYear"`

	PostalCode      *string `json:"postal_code"`
	PostalCodeCamel *string `json:"postalCode"`

	Last4 *string `json:"last4"`

	SpendRequestID  *string `json:"spend_request_id"`
	SpendReqIDCamel *string `json:"spendRequestId"`

	Status *string `json:"status"`
}

func (s *StripeLinkPaymentProvider) parseVirtualCardIssue(stdout string, request VirtualCardRequest, execContext PaymentExecutionContext) (*VirtualCardIssueResult, error) {
	var dto stripeLinkDTO
	if err := json.Unmarshal([]byte(stdout), &dto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	handleID := firstNonNil(dto.HandleID, dto.HandleIDCamel, dto.ID)
	if handleID == "" {
		handleID = fmt.Sprintf("stripe_link_%s", uuid.New().String())
	}

	pan := firstNonNil(dto.PAN, dto.CardNumber, dto.CardNumberCamel)

	secret := NewPaymentSecret(
		handleID,
		s.options.ProviderId,
		pan,
		firstNonNil(dto.CVV, dto.CVC),
		firstNonNil(dto.ExpMonth, dto.ExpMonthCamel),
		firstNonNil(dto.ExpYear, dto.ExpYearCamel),
		firstNonNil(dto.PostalCode, dto.PostalCodeCamel),
		"", "",
		request.ValidUntilUtc,
		execContext.Environment,
	)

	last4 := firstNonNil(dto.Last4)
	if last4 == "" {
		last4 = secret.Last4()
	}

	status := firstNonNil(dto.Status)
	if status == "" {
		status = "issued"
	}

	return &VirtualCardIssueResult{
		Handle: VirtualCardHandle{
			HandleId:           handleID,
			ProviderId:         s.options.ProviderId,
			Last4:              last4,
			TargetMerchantName: request.MerchantName,
			IssuedAtUtc:        time.Now().UTC(),
			ValidUntilUtc:      request.ValidUntilUtc,
			SpendRequestId:     firstNonNil(dto.SpendRequestID, dto.SpendReqIDCamel),
			Status:             status,
			Environment:        execContext.Environment,
		},
		Secret: secret,
	}, nil
}

func firstNonNil(ptrs ...*string) string {
	for _, ptr := range ptrs {
		if ptr != nil && *ptr != "" {
			return *ptr
		}
	}
	return ""
}

func (s *StripeLinkPaymentProvider) ListFundingSources(ctx context.Context, execContext PaymentExecutionContext) ([]FundingSource, error) {
	result, err := s.runJson(ctx, []string{"funding-sources", "list", "--json", "--mode", s.options.Mode})
	if err != nil || result.ExitCode != 0 {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else {
			msg = result.Stderr
		}
		return nil, fmt.Errorf("Stripe Link funding source list failed: %s", msg)
	}

	return s.parseFundingSources(result.Stdout)
}

func (s *StripeLinkPaymentProvider) parseFundingSources(jsonStr string) ([]FundingSource, error) {
	var raw any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case map[string]any:
		if sources, ok := v["funding_sources"].([]any); ok {
			items = sources
		} else if camel, ok := v["fundingSources"].([]any); ok {
			items = camel
		}
	}

	results := make([]FundingSource, 0, len(items))
	testMode := strings.EqualFold(s.options.Mode, PaymentEnvTest)

	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		id := util.Deref(util.GetString(item, "id"))
		if id == "" {
			id = util.Deref(util.GetString(item, "fundingSourceId"))
		}
		if strings.TrimSpace(id) == "" {
			continue
		}

		// 检查 displayName
		displayName := util.Deref(util.GetString(item, "display_name"))
		if displayName == "" {
			displayName = util.Deref(util.GetString(item, "displayName"))
		}
		if displayName == "" {
			displayName = "Stripe Link funding source"
		}

		// 检查 type
		typ := util.Deref(util.GetString(item, "type"))
		if typ == "" {
			typ = "link"
		}

		// 检查 available (默认为 true)
		available := true
		if availBool := util.GetBool(item, "available"); availBool != nil {
			available = *availBool
		}

		results = append(results, FundingSource{
			FundingSourceId: id,
			ProviderId:      s.options.ProviderId,
			DisplayName:     displayName,
			Type:            typ,
			Last4:           util.Deref(util.GetString(item, "last4")),
			Currency:        util.Deref(util.GetString(item, "currency")),
			TestMode:        testMode,
			Available:       available,
		})
	}

	return results, nil
}

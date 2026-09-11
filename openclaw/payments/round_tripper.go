package payments

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// var paymentSvc PaymentRuntimeService
// var execContext PaymentExecutionContext
//
//	// 构造注入了拦截器的 http.Client
//	client := &http.Client{
//		Transport: NewPaymentAwareRoundTripper(
//			paymentSvc,
//			execContext,
//			"provider_stripe_01",
//			http.DefaultTransport, // 传 nil 也会默认使用 http.DefaultTransport
//		),
//	}
//
//	// 如果目标服务器返回 402，中间件会自动扣款、注入 Authorization 并在后台重试
//	resp, err := client.Get("https://api.example.com/paid-resource")
//	if err != nil {
//		panic(err)
//	}
//	defer resp.Body.Close()
//
//	fmt.Println("Final Status:", resp.Status)
type PaymentAwareRoundTripper struct {
	next        http.RoundTripper
	payments    PaymentRuntimeService
	execContext PaymentExecutionContext
	providerId  string
}

func NewPaymentAwareRoundTripper(payments PaymentRuntimeService, execContext PaymentExecutionContext, providerId string, next http.RoundTripper) *PaymentAwareRoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}

	return &PaymentAwareRoundTripper{
		next:        next,
		payments:    payments,
		providerId:  providerId,
		execContext: execContext,
	}
}

// RoundTrip 实现了 http.RoundTripper 接口
func (p *PaymentAwareRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. 读取并缓冲 Request Body，以便后续可能的重试（Go 的 req.Body 默认读取一次后就耗尽了）
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read request body failed: %w", err)
		}
		// 重新写回原始请求，确保第一次发送时可用
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// 2. 发送原始 HTTP 请求
	resp, err := p.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 3. 如果不是 402 Payment Required，直接返回响应
	if resp.StatusCode != http.StatusPaymentRequired {
		return resp, nil
	}

	// 4. 解析 402 付费挑战，解析前关闭原始 402 的 Body
	challenge, parseErr := parseMachinePaymentChallenge(resp)
	_ = resp.Body.Close()

	if parseErr != nil || challenge == nil {
		// 解析挑战失败，原样返回（或者你可以构造一个错误）
		return resp, parseErr
	}

	// 补充 URL 和 ProviderID 信息
	if challenge.ResourceUrl == "" && req.URL != nil {
		challenge.ResourceUrl = req.URL.String()
	}
	if challenge.ProviderId == "" {
		challenge.ProviderId = p.providerId
	}

	ctx := req.Context()

	// 5. 调用支付服务执行机器自动扣款/授权
	payResult, err := p.payments.ExecuteMachinePayment(ctx, MachinePaymentRequest{
		ProviderId:  p.providerId,
		Challenge:   *challenge,
		Environment: p.execContext.Environment,
	}, p.execContext)
	if err != nil {
		return nil, fmt.Errorf("execute machine payment failed: %w", err)
	}

	// 6. 获取一次性 Authorization 凭证 Header
	secret, err := p.payments.RetrieveMachineAuthorizationOnce(ctx, payResult.PaymentId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve payment authorization header: %w", err)
	}

	authHeader := secret.Resolve(SecretFieldAuthorizationHeader)
	if authHeader == "" {
		return nil, errors.New("machine payment provider did not return scoped authorization")
	}

	// 7. 克隆请求并注入新的 Authorization 头，同时重新注入 Body
	retryReq := req.Clone(ctx)
	retryReq.Header.Set("Authorization", authHeader)
	if len(bodyBytes) > 0 {
		retryReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		retryReq.ContentLength = int64(len(bodyBytes))
	}

	// 8. 重新发送请求
	return p.next.RoundTrip(retryReq)
}

// --- 402 Challenge 解析逻辑 ---

func parseMachinePaymentChallenge(resp *http.Response) (*MachinePaymentChallenge, error) {
	var challengeText string

	// 优先读取 Payment-Required 响应头
	if headerVal := resp.Header.Get("Payment-Required"); headerVal != "" {
		challengeText = headerVal
	} else if resp.Body != nil {
		// 备选：读取 Body 文本
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			challengeText = string(bodyBytes)
		}
	}

	if strings.TrimSpace(challengeText) == "" {
		return nil, nil
	}

	protocol := "http-402"
	if strings.Contains(strings.ToLower(challengeText), "x402") {
		protocol = "x402"
	}

	amount, _ := strconv.ParseInt(extractKeyValue(challengeText, "amount"), 10, 64)

	challenge := &MachinePaymentChallenge{
		Protocol:     protocol,
		ChallengeId:  firstNonEmpty(extractKeyValue(challengeText, "challenge"), extractKeyValue(challengeText, "id")),
		MerchantName: extractKeyValue(challengeText, "merchant"),
		Currency:     firstNonEmpty(extractKeyValue(challengeText, "currency"), "USD"),
		AmountMinor:  amount,
	}

	return challenge, nil
}

func extractKeyValue(text, key string) string {
	needle := key + "="
	idx := strings.Index(strings.ToLower(text), needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.IndexAny(text[start:], ";,\n\r")
	var val string
	if end < 0 {
		val = text[start:]
	} else {
		val = text[start : start+end]
	}
	return strings.Trim(strings.TrimSpace(val), "\"'")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

package payments

import (
	"regexp"
	"strings"
)

type IPaymentRedactor interface {
	Redact(value string) string
}

type PaymentSensitiveDataRedactor struct{}

func NewPaymentSensitiveDataRedactor() *PaymentSensitiveDataRedactor {
	return &PaymentSensitiveDataRedactor{}
}

func (r *PaymentSensitiveDataRedactor) GetName() string {
	return "payment-secrets"
}

var (
	panCandidateRegex         = regexp.MustCompile(`(?<!\d)(?:\d[ -]?){13,19}(?!\d)`)
	cvvContextRegex           = regexp.MustCompile(`(?i)\b(cvv|cvc|security[-_\s]?code|card[-_\s]?code)\b(\s*["']?\s*[:=]\s*["']?)\d{3,4}`)
	paymentAuthorizationRegex = regexp.MustCompile(`(?im)\b(Authorization\s*:\s*Payment\s+)[^\s\r\n]+`)
	paymentJSONFieldRegex     = regexp.MustCompile(`(?i)(["']?(?:pan|cardNumber|card_number|cvv|cvc|securityCode|security_code|paymentToken|payment_token|sharedPaymentToken|shared_payment_token|authorizationToken|authorization_token|providerSecret|provider_secret|rawSecret|raw_secret|clientSecret|client_secret)["']?\s*[:=]\s*["']?)[^,"'\s}\]]+`)
	paymentTokenRegex         = regexp.MustCompile(`(?i)\b(?:spt|payment|paytok|machinepay)_[A-Za-z0-9._~+/=-]{8,}`)
)

func (r *PaymentSensitiveDataRedactor) Redact(value string) string {
	if value == "" {
		return ""
	}

	// 1. PAN
	redacted := panCandidateRegex.ReplaceAllStringFunc(value, func(match string) string {
		digits := stripDigits(match)
		if isPotentialPan(digits) && passesLuhn(digits) {
			return "[REDACTED:payment-card]"
		}
		return match
	})

	// 2. CVV
	redacted = cvvContextRegex.ReplaceAllString(redacted, "${1}${2}[REDACTED:payment-cvv]")

	// 3. Payment Authorization
	redacted = paymentAuthorizationRegex.ReplaceAllString(redacted, "${1}[REDACTED:payment-authorization]")

	// 4. JSON
	redacted = paymentJSONFieldRegex.ReplaceAllString(redacted, "${1}[REDACTED:payment-secret]")

	// 5. Payment Token
	redacted = paymentTokenRegex.ReplaceAllString(redacted, "[REDACTED:payment-token]")

	return redacted
}

func stripDigits(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			builder.WriteRune(ch)
		}
	}
	return builder.String()
}

func isPotentialPan(digits string) bool {
	l := len(digits)
	return l >= 13 && l <= 19
}

// Luhn
func passesLuhn(digits string) bool {
	sum := 0
	alternate := false

	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

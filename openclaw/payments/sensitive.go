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

// var (
//
//	panCandidateRegex         = regexp.MustCompile(`(?<!\d)(?:\d[ -]?){13,19}(?!\d)`)
//	cvvContextRegex           = regexp.MustCompile(`(?i)\b(cvv|cvc|security[-_\s]?code|card[-_\s]?code)\b(\s*["']?\s*[:=]\s*["']?)\d{3,4}`)
//	paymentAuthorizationRegex = regexp.MustCompile(`(?im)\b(Authorization\s*:\s*Payment\s+)[^\s\r\n]+`)
//	paymentJSONFieldRegex     = regexp.MustCompile(`(?i)(["']?(?:pan|cardNumber|card_number|cvv|cvc|securityCode|security_code|paymentToken|payment_token|sharedPaymentToken|shared_payment_token|authorizationToken|authorization_token|providerSecret|provider_secret|rawSecret|raw_secret|clientSecret|client_secret)["']?\s*[:=]\s*["']?)[^,"'\s}\]]+`)
//	paymentTokenRegex         = regexp.MustCompile(`(?i)\b(?:spt|payment|paytok|machinepay)_[A-Za-z0-9._~+/=-]{8,}`)
//
// )

var (
	// 1. 去掉 RE2 不支持的零宽断言, 候选 PAN：先宽松匹配包含数字、空格、横杠的片段（至少 13 字符），
	// 再在代码中校验前后是否紧邻数字，并校验去分隔符后的数字长度（13~19）与 Luhn 算法。
	panCandidateRegex = regexp.MustCompile(`[0-9][0-9 -]{11,28}[0-9]`)

	// 2. CVV/CVC 上下文：捕获组 1 为前缀（键名+分隔符），捕获组 2 为 3~4 位 CVV 值
	cvvContextRegex = regexp.MustCompile(`(?i)\b(?:cvv|cvc|security[-_\s]?code|card[-_\s]?code)\b(?:\s*["']?\s*[:=]\s*["']?)([0-9]{3,4})(?:[^0-9]|$)`)

	// 3. Payment Authorization：捕获组 1 为 "Authorization: Payment "，捕获组 2 为 token 值
	paymentAuthorizationRegex = regexp.MustCompile(`(?im)^[ \t]*Authorization\s*:\s*Payment\s+([^\s\r\n]+)`)

	// 4. JSON / 键值对字段：捕获组 1 为键名与分隔符，捕获组 2/3/4 分别为双引号值、单引号值、无引号值
	paymentJSONFieldRegex = regexp.MustCompile(`(?i)(?:["']?(?:pan|cardNumber|card_number|cvv|cvc|securityCode|security_code|paymentToken|payment_token|sharedPaymentToken|shared_payment_token|authorizationToken|authorization_token|providerSecret|provider_secret|rawSecret|raw_secret|clientSecret|client_secret)["']?\s*[:=]\s*)(?:"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)'|([^,"'\s}\]]+))`)

	// 5. 独立 Payment Token：捕获整个 token（不吃掉后面的引号/逗号等分隔符）
	paymentTokenRegex = regexp.MustCompile(`(?i)\b(?:spt|payment|paytok|machinepay)_[A-Za-z0-9._~+/=-]{8,}`)
)

func (r *PaymentSensitiveDataRedactor) Redact(value string) string {
	if value == "" {
		return ""
	}

	// 1. PAN（带前后非数字边界检查 + 长度 + Luhn）
	redacted := replaceMatches(panCandidateRegex, value, func(submatches [][]int) (string, bool) {
		// submatches[0] 是整个匹配范围 [start, end]
		start, end := submatches[0][0], submatches[0][1]
		if start > 0 && isASCIIDigit(value[start-1]) {
			return "", false
		}
		if end < len(value) && isASCIIDigit(value[end]) {
			return "", false
		}

		candidate := value[start:end]
		digits := stripDigits(candidate)
		if isPotentialPan(digits) && passesLuhn(digits) {
			return "[REDACTED:payment-card]", true
		}
		return "", false
	})

	// 2. CVV（只替换捕获组 2 的 3~4 位数字）
	redacted = replaceSubmatchGroup(redacted, cvvContextRegex, 1, "[REDACTED:payment-cvv]")

	// 3. Payment Authorization（只替换捕获组 2 的凭证值）
	redacted = replaceSubmatchGroup(redacted, paymentAuthorizationRegex, 1, "[REDACTED:payment-authorization]")

	// 4. JSON / 键值对（只替换实际的值部分，保留 key 和引号）
	redacted = replaceMatches(paymentJSONFieldRegex, redacted, func(submatches [][]int) (string, bool) {
		for _, groupIdx := range []int{2, 3, 4} {
			if groupIdx*2+1 < len(submatches) && submatches[groupIdx][0] >= 0 {
				return "[REDACTED:payment-secret]", true
			}
		}
		return "", false
	})

	// 5. Payment Token
	redacted = replaceSubmatchGroup(redacted, paymentTokenRegex, 0, "[REDACTED:payment-token]")

	return redacted
}

func isASCIIDigit(r byte) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// replaceSubmatchGroup 将指定正则中第 groupIdx 个捕获组的内容替换为 replacement。
// 当 groupIdx == 0 时，替换整个匹配；当 groupIdx >= 1 时，只替换该子组范围。
func replaceSubmatchGroup(text string, re *regexp.Regexp, groupIdx int, replacement string) string {
	return replaceMatches(re, text, func(submatches [][]int) (string, bool) {
		if groupIdx == 0 {
			return replacement, true
		}
		if groupIdx*2+1 < len(submatches) && submatches[groupIdx][0] >= 0 {
			return replacement, true
		}
		return "", false
	})
}

// replaceMatches 通用子匹配替换器：回调返回 (newText, true) 时，
// 若回调中未直接改写 submatches 区间，则默认替换整个匹配或由外层根据索引决定。
// 这里为了清晰，直接按子组替换。
func replaceMatches(re *regexp.Regexp, text string, decide func(submatches [][]int) (string, bool)) string {
	all := re.FindAllStringSubmatchIndex(text, -1)
	if len(all) == 0 {
		return text
	}

	var b strings.Builder
	b.Grow(len(text))
	last := 0

	for _, loc := range all {
		// 将 loc 扁平索引 [mStart, mEnd, g1Start, g1End, ...] 转为 [][]int
		groups := make([][]int, 0, len(loc)/2)
		for i := 0; i+1 < len(loc); i += 2 {
			groups = append(groups, []int{loc[i], loc[i+1]})
		}

		rep, ok := decide(groups)
		if !ok {
			continue
		}

		// 默认替换整个匹配区间；如果外部要只替某一个子组，可调用下面的 replaceSpecificRange
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		b.WriteString(text[last:start])
		b.WriteString(rep)
		last = end
	}

	b.WriteString(text[last:])
	return b.String()
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

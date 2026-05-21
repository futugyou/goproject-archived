package graphify

import (
	"fmt"
	"html"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type SecurityResult struct {
	IsValid        bool
	Errors         []string
	SanitizedValue string
}

func SecuritySuccess(sanitizedValue string) SecurityResult {
	return SecurityResult{IsValid: true, SanitizedValue: sanitizedValue}
}

func SecurityFailure(err []string) SecurityResult {
	return SecurityResult{Errors: err, IsValid: false}
}

type InputValidator struct {
	allowedSchemes     []string
	defaultMaxLabelLen int
	privateIpPrefixes  []string
}

var (
	controlCharPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	htmlTagPattern     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<[^>]+>`)
	injectionPattern   = regexp.MustCompile(`[';"\\<>&]`)
)

func NewInputValidator() *InputValidator {
	return &InputValidator{
		allowedSchemes:     []string{"http", "https"},
		defaultMaxLabelLen: 200,
		privateIpPrefixes: []string{
			"10.", "127.", "192.168.",
			"172.16.", "172.17.", "172.18.", "172.19.",
			"172.20.", "172.21.", "172.22.", "172.23.",
			"172.24.", "172.25.", "172.26.", "172.27.",
			"172.28.", "172.29.", "172.30.", "172.31.",
		},
	}
}

func (v *InputValidator) ValidateUrl(rawUrl string) SecurityResult {
	if strings.TrimSpace(rawUrl) == "" {
		return SecurityFailure([]string{"URL cannot be null or empty"})
	}

	u, err := url.Parse(rawUrl)
	if err != nil || !u.IsAbs() {
		return SecurityFailure([]string{fmt.Sprintf("Invalid URL format: %s", rawUrl)})
	}

	schemeAllowed := false
	for _, s := range v.allowedSchemes {
		if strings.ToLower(u.Scheme) == s {
			schemeAllowed = true
			break
		}
	}
	if !schemeAllowed {
		return SecurityFailure([]string{fmt.Sprintf("Blocked URL scheme '%s' - only http and https are allowed", u.Scheme)})
	}

	host := strings.ToLower(u.Hostname())

	if host == "localhost" || host == "::1" {
		return SecurityFailure([]string{fmt.Sprintf("Access to localhost is blocked: %s", rawUrl)})
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if v.isPrivateIp(ip) {
			return SecurityFailure([]string{fmt.Sprintf("Access to private IP address is blocked: %s", rawUrl)})
		}
	} else {
		for _, prefix := range v.privateIpPrefixes {
			if strings.HasPrefix(host, prefix) {
				return SecurityFailure([]string{fmt.Sprintf("Access to private IP range is blocked: %s", rawUrl)})
			}
		}
	}

	return SecuritySuccess("")
}

func (v *InputValidator) ValidatePath(path string, baseDir string) SecurityResult {
	if strings.TrimSpace(path) == "" {
		return SecurityFailure([]string{"Path cannot be null or empty"})
	}

	if strings.ContainsRune(path, 0) {
		return SecurityFailure([]string{"Path contains null bytes"})
	}

	if strings.Contains(path, "..") {
		return SecurityFailure([]string{"Path traversal detected: '..' is not allowed"})
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return SecurityFailure([]string{fmt.Sprintf("Invalid path: %v", err)})
	}

	if baseDir != "" {
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			return SecurityFailure([]string{fmt.Sprintf("Invalid base directory: %v", err)})
		}

		if !v.dirExists(absBase) {
			return SecurityFailure([]string{fmt.Sprintf("Base directory does not exist: %s", absBase)})
		}

		if !strings.HasPrefix(absPath, absBase) {
			return SecurityFailure([]string{fmt.Sprintf("Path escapes the allowed directory %s", absBase)})
		}
	}

	return SecuritySuccess("")
}

func (v *InputValidator) SanitizeLabel(label string, maxLength int) SecurityResult {
	if label == "" {
		return SecuritySuccess("")
	}

	if maxLength <= 0 {
		maxLength = v.defaultMaxLabelLen
	}

	sanitized := controlCharPattern.ReplaceAllString(label, "")

	var sb strings.Builder
	for _, r := range sanitized {
		if !unicode.IsControl(r) {
			sb.WriteRune(r)
		}
	}
	sanitized = sb.String()

	sanitized = htmlTagPattern.ReplaceAllString(sanitized, "")

	runes := []rune(sanitized)
	if len(runes) > maxLength {
		sanitized = string(runes[:maxLength])
	}

	sanitized = html.EscapeString(sanitized)

	return SecuritySuccess(sanitized)
}

func (v *InputValidator) isPrivateIp(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	// IPv4
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}

	// IPv6
	// fc00::/7 (Unique Local) 和 fe80::/10 (Link Local)
	if ip.IsLinkLocalUnicast() || !ip.IsGlobalUnicast() {
		return true
	}

	return false
}

func (v *InputValidator) dirExists(fullPath string) bool {
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// ValidateInput
func (v *InputValidator) ValidateInput(input string, maxLength int) SecurityResult {
	if strings.TrimSpace(input) == "" {
		return SecurityFailure([]string{"Input cannot be empty or whitespace"})
	}

	if len([]rune(input)) > maxLength {
		return SecurityFailure([]string{fmt.Sprintf("Input exceeds maximum length of %d characters", maxLength)})
	}

	if strings.ContainsRune(input, 0) {
		return SecurityFailure([]string{"Input contains null bytes"})
	}

	matches := injectionPattern.FindAllString(input, -1)
	if len(matches) > 0 {
		if float64(len(matches)) > float64(len(input))*0.1 {
			return SecurityFailure([]string{"Input contains suspicious injection patterns"})
		}
	}

	if controlCharPattern.MatchString(input) {
		return SecurityFailure([]string{"Input contains control characters"})
	}

	return SecuritySuccess("")
}

package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/futugyou/openclaw/core"
	"github.com/mxschmitt/playwright-go"
)

var _ core.ITool = (*BrowserTool)(nil)
var _ core.ISandboxCapableTool = (*BrowserTool)(nil)
var _ core.IToolLocalExecutionPolicy = (*BrowserTool)(nil)

const sandboxProfileDir string = "/tmp/openclaw-browser-profile"
const browserLocalExecutionUnavailableMessage string = "Error: Browser tool requires a configured execution backend or sandbox in this runtime. Local Playwright execution is unavailable."
const sandboxRunnerScript string = `
        const { chromium } = require('playwright');
        const dns = require('dns').promises;
        const net = require('net');

        function globMatches(pattern, value) {
          if (!pattern) return false;
          if (pattern === '*') return true;
          const escaped = String(pattern).replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*');
          return new RegExp(` + "`" + `^${escaped}$` + "`" + `, 'i').test(String(value));
        }

        function isBlockedIpv4(address) {
          const parts = address.split('.').map((part) => Number(part));
          if (parts.length !== 4 || parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return true;
          return parts[0] === 0 ||
            parts[0] === 10 ||
            parts[0] === 127 ||
            (parts[0] === 100 && parts[1] >= 64 && parts[1] <= 127) ||
            (parts[0] === 169 && parts[1] === 254) ||
            (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) ||
            (parts[0] === 192 && parts[1] === 168) ||
            (parts[0] === 198 && (parts[1] === 18 || parts[1] === 19)) ||
            parts[0] >= 224;
        }

        function ipv4ToInt(address) {
          const parts = address.split('.').map((part) => Number(part));
          if (parts.length !== 4 || parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return null;
          return (((parts[0] << 24) >>> 0) | (parts[1] << 16) | (parts[2] << 8) | parts[3]) >>> 0;
        }

        function ipv6ToBigInt(address) {
          const zoneIndex = address.indexOf('%');
          let text = (zoneIndex >= 0 ? address.slice(0, zoneIndex) : address).toLowerCase();
          const lastColon = text.lastIndexOf(':');
          const tail = lastColon >= 0 ? text.slice(lastColon + 1) : text;
          if (tail.includes('.')) {
            const ipv4 = ipv4ToInt(tail);
            if (ipv4 == null) return null;
            const high = ((ipv4 >>> 16) & 0xffff).toString(16);
            const low = (ipv4 & 0xffff).toString(16);
            text = ` + "`" + `${text.slice(0, lastColon)}:${high}:${low}` + "`" + `;
          }

          const halves = text.split('::');
          if (halves.length > 2) return null;

          const left = halves[0] ? halves[0].split(':') : [];
          const right = halves.length === 2 && halves[1] ? halves[1].split(':') : [];
          if (left.concat(right).some((part) => !/^[0-9a-f]{1,4}$/.test(part))) return null;

          const missing = 8 - left.length - right.length;
          if ((halves.length === 1 && missing !== 0) || (halves.length === 2 && missing < 1)) return null;

          const groups = left.concat(Array(Math.max(0, missing)).fill('0'), right);
          if (groups.length !== 8) return null;

          let value = 0n;
          for (const group of groups) {
            value = (value << 16n) + BigInt(parseInt(group, 16));
          }
          return value;
        }

        function mappedIpv6ToIpv4(address) {
          const value = ipv6ToBigInt(address);
          if (value == null || (value >> 32n) !== 0xffffn) return null;
          const tail = Number(value & 0xffffffffn);
          return ` + "`" + `${(tail >>> 24) & 0xff}.${(tail >>> 16) & 0xff}.${(tail >>> 8) & 0xff}.${tail & 0xff}` + "`" + `;
        }

        function ipToBigInt(address) {
          const version = net.isIP(address);
          if (version === 4) {
            const value = ipv4ToInt(address);
            return value == null ? null : { value: BigInt(value), bits: 32 };
          }
          if (version === 6) {
            const mapped = mappedIpv6ToIpv4(address);
            if (mapped) return ipToBigInt(mapped);

            const value = ipv6ToBigInt(address);
            return value == null ? null : { value, bits: 128 };
          }
          return null;
        }

        function cidrMatches(address, cidr) {
          const parts = String(cidr || '').split('/');
          if (parts.length !== 2) return false;
          const addressValue = ipToBigInt(address);
          const networkValue = ipToBigInt(parts[0]);
          if (!addressValue || !networkValue || addressValue.bits !== networkValue.bits) return false;

          const prefix = Number(parts[1]);
          if (!Number.isInteger(prefix) || prefix < 0 || prefix > addressValue.bits) return false;
          if (prefix === 0) return true;

          const shift = BigInt(addressValue.bits - prefix);
          return (addressValue.value >> shift) === (networkValue.value >> shift);
        }

        function isBlockedIp(address) {
          const version = net.isIP(address);
          if (version === 4) return isBlockedIpv4(address);
          if (version === 6) {
            const value = address.toLowerCase();
            const mapped = mappedIpv6ToIpv4(value);
            if (mapped) return isBlockedIpv4(mapped);

            const firstGroup = parseInt(value.split(':', 1)[0] || '0', 16);
            return value === '::' ||
              value === '::1' ||
              (Number.isInteger(firstGroup) && (firstGroup & 0xffc0) === 0xfe80) ||
              (Number.isInteger(firstGroup) && (firstGroup & 0xffc0) === 0xfec0) ||
              (Number.isInteger(firstGroup) && (firstGroup & 0xfe00) === 0xfc00) ||
              (Number.isInteger(firstGroup) && (firstGroup & 0xff00) === 0xff00);
          }
          return true;
        }

          function validateUrlSafety(rawUrl, policy) {
          if (!policy || policy.enabled === false) return;
          const parsed = new URL(rawUrl);
          if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
            throw new Error('URL safety blocked non-http(s) navigation.');
          }

          let host = parsed.hostname.toLowerCase().replace(/\.$/, '');
          if (host.startsWith('[') && host.endsWith(']')) host = host.slice(1, -1);
          const builtInBlocked = ['localhost', '*.localhost', 'metadata', 'metadata.google.internal'];
          if (policy.blockPrivateNetworkTargets !== false && builtInBlocked.some((pattern) => globMatches(pattern, host))) {
            throw new Error(` + "`" + `URL safety blocked host '${host}'.` + "`" + `);
          }

          for (const pattern of policy.blockedHostGlobs || []) {
            if (globMatches(pattern, host)) throw new Error(` + "`" + `URL safety blocked host '${host}'.` + "`" + `);
          }

          let addresses;
          if (net.isIP(host)) {
            addresses = [host];
          } else if (policy.blockPrivateNetworkTargets !== false || (policy.blockedCidrs || []).length > 0) {
            addresses = (await dns.lookup(host, { all: true })).map((item) => item.address);
          } else {
            addresses = [];
          }

          if (policy.blockPrivateNetworkTargets !== false && addresses.some(isBlockedIp)) {
            throw new Error(` + "`" + `URL safety blocked private or loopback target for '${host}'.` + "`" + `);
          }

          for (const cidr of policy.blockedCidrs || []) {
            if (addresses.some((address) => cidrMatches(address, cidr))) {
              throw new Error(` + "`" + `URL safety blocked CIDR target for '${host}'.` + "`" + `);
            }
          }
        }

        (  () => {
          const payload = JSON.parse(process.argv[1] || '{}');
          payload.urlSafety = JSON.parse(payload.urlSafetyJson || '{}');
          let context;

          try {
            context = await chromium.launchPersistentContext(payload.userDataDir || '/tmp/openclaw-browser-profile', {
              headless: payload.headless !== false,
              timeout: payload.timeoutMs || 30000
            });
            await context.route('**/*',   (route) => {
              try {
                await validateUrlSafety(route.request().url(), payload.urlSafety);
                await route.continue();
              } catch {
                await route.abort();
              }
            });

            const page = context.pages()[0] || await context.newPage();
            let output = '';

            switch (payload.action) {
              case 'goto': {
                await validateUrlSafety(payload.url, payload.urlSafety);
                await page.goto(payload.url, { waitUntil: 'load' });
                const title = await page.title();
                output = ` + "`" + `Navigated to ${payload.url}. Title: '${title}'` + "`" + `;
                break;
              }

              case 'click':
                await page.click(payload.selector);
                output = ` + "`" + `Clicked selector: ${payload.selector}` + "`" + `;
                break;

              case 'fill':
                await page.fill(payload.selector, payload.value ?? '');
                output = ` + "`" + `Filled ${payload.selector} with provided value.` + "`" + `;
                break;

              case 'get_text':
                if (payload.selector) {
                  output = await page.textContent(payload.selector) || 'No text found for selector.';
                } else {
                  output = await page.textContent('body') || 'Body is empty.';
                }
                break;

              case 'evaluate': {
                const value = await page.evaluate((source) => globalThis.eval(source), payload.script);
                output = value == null ? '' : String(value);
                break;
              }

              case 'screenshot': {
                const bytes = await page.screenshot({ fullPage: true });
                output = ` + "`" + `Screenshot taken. Base64: ${bytes.toString('base64')}` + "`" + `;
                break;
              }

              default:
                throw new Error(` + "`" + `Unknown action '${payload.action}'` + "`" + `);
            }

            process.stdout.write(output);
          } finally {
            if (context) {
              await context.close();
            }
          }
        })().catch((error) => {
          const message = error && error.message ? error.message : String(error);
          process.stderr.write(message);
          process.exit(1);
        });
        `

type BrowserTool struct {
	config                  core.ToolingConfig
	localExecutionSupported bool
	metrics                 *core.RuntimeMetrics

	mu          sync.Mutex
	pw          *playwright.Playwright
	browser     playwright.Browser
	page        playwright.Page
	initialized bool
	disposed    bool
}

func NewBrowserTool(config core.ToolingConfig, metrics *core.RuntimeMetrics, localExecutionSupported bool) *BrowserTool {
	return &BrowserTool{
		config:                  config,
		metrics:                 metrics,
		localExecutionSupported: localExecutionSupported,
	}
}

func (b *BrowserTool) Description() string {
	return "An interactive headless browser. Enables navigation to JS-heavy sites, " +
		"clicking elements, filling inputs, taking screenshots, and extracting text or DOM data."
}

func (b *BrowserTool) Name() string {
	return "browser"
}

func (b *BrowserTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "action": { 
              "type": "string", 
              "enum": ["goto", "click", "fill", "get_text", "evaluate", "screenshot"],
              "description": "The browser action to perform."
            },
            "url": { "type": "string", "description": "URL for goto action." },
            "selector": { "type": "string", "description": "CSS/XPath selector for click, fill, or get_text." },
            "value": { "type": "string", "description": "Text to type for fill action." },
            "script": { "type": "string", "description": "JS script to evaluate. Returns string result." }
          },
          "required": ["action"]
        }
`
}

func (b *BrowserTool) LocalExecutionSupported() bool {
	return b.localExecutionSupported
}

func (b *BrowserTool) LocalExecutionUnavailableFailureCode() string {
	return "browser_backend_missing"
}

func (b *BrowserTool) LocalExecutionUnavailableMessage() string {
	return "Error: Browser tool requires a configured execution backend or sandbox in this runtime. Local Playwright execution is unavailable."
}

func (b *BrowserTool) CreateSandboxRequest(argumentsJson string) (*core.SandboxExecutionRequest, error) {
	payload, err := b.buildSandboxPayload(argumentsJson)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &core.SandboxExecutionRequest{
		Command:   "node",
		Arguments: []string{"-e", sandboxRunnerScript, string(data)},
	}, nil
}

func (b *BrowserTool) DefaultSandboxMode() core.ToolSandboxMode {
	return core.ToolSandboxMode_Prefer
}

func (b *BrowserTool) FormatSandboxResult(argumentsJson string, result core.SandboxResult) string {
	if result.ExitCode == 0 {
		return result.Stdout
	}

	message := result.Stderr
	if message == "" {
		message = result.Stdout
	}

	if message == "" {
		return "Browser action failed: Unknown sandbox error."
	}

	return fmt.Sprintf("Browser action failed: %s", message)
}

func (b *BrowserTool) buildSandboxPayload(argumentsJson string) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return nil, newToolSandboxError(fmt.Sprintf("Error: invalid JSON: %v", err))
	}

	action, err := readRequiredString(args, "action")
	if err != nil {
		return nil, err
	}

	if action == "evaluate" && !b.config.AllowBrowserEvaluate {
		return nil, newToolSandboxError("Error: Browser evaluate is disabled by configuration (Tooling.AllowBrowserEvaluate=false).")
	}

	// 序列化 urlSafety 配置
	urlSafetyBytes, err := json.Marshal(b.config.UrlSafety)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal urlSafety: %w", err)
	}

	payload := map[string]any{
		"action":        action,
		"headless":      b.config.BrowserHeadless,
		"timeoutMs":     b.config.BrowserTimeoutSeconds * 1000,
		"userDataDir":   sandboxProfileDir,
		"urlSafetyJson": string(urlSafetyBytes),
	}

	switch action {
	case "goto":
		urlVal, err := b.readValidatedHTTPURL(args, "url")
		if err != nil {
			return nil, err
		}
		payload["url"] = urlVal

	case "click":
		selector, err := readRequiredString(args, "selector")
		if err != nil {
			return nil, err
		}
		payload["selector"] = selector

	case "fill":
		selector, err := readRequiredString(args, "selector")
		if err != nil {
			return nil, err
		}
		value, err := readRequiredString(args, "value")
		if err != nil {
			return nil, err
		}
		payload["selector"] = selector
		payload["value"] = value

	case "get_text":
		payload["selector"] = readOptionalString(args, "selector")

	case "evaluate":
		script, err := readRequiredString(args, "script")
		if err != nil {
			return nil, err
		}
		payload["script"] = script

	case "screenshot":
		// 无需附加特定参数

	default:
		return nil, newToolSandboxError(fmt.Sprintf("Error: Unknown action '%s'", action))
	}

	return payload, nil
}

func readRequiredString(data map[string]any, propertyName string) (string, error) {
	val, exists := data[propertyName]
	if !exists {
		return "", newToolSandboxError(fmt.Sprintf("Error: '%s' is required.", propertyName))
	}

	strVal, ok := val.(string)
	if !ok || strings.TrimSpace(strVal) == "" {
		return "", newToolSandboxError(fmt.Sprintf("Error: '%s' is required.", propertyName))
	}

	return strVal, nil
}

func readOptionalString(data map[string]any, propertyName string) *string {
	val, exists := data[propertyName]
	if !exists {
		return nil
	}

	strVal, ok := val.(string)
	if !ok {
		return nil
	}

	return &strVal
}

func (s *BrowserTool) readValidatedHTTPURL(data map[string]any, propertyName string) (string, error) {
	value, err := readRequiredString(data, propertyName)
	if err != nil {
		return "", err
	}

	parsedURL, err := url.ParseRequestURI(value)
	if err != nil || !parsedURL.IsAbs() || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", newToolSandboxError("Error: only absolute http(s) URLs are allowed.")
	}

	safety := core.UrlSafetyInstance.ValidateHttpUrlLocal(*parsedURL, &s.config.UrlSafety)
	if !safety.Allowed {
		return "", newToolSandboxError(safety.ToString())
	}

	return value, nil
}

func newToolSandboxError(msg string) error {
	return &core.ToolSandboxError{Message: msg}
}

func (b *BrowserTool) Execute(ctx context.Context, argumentsJson string) string {
	if !b.localExecutionSupported {
		return browserLocalExecutionUnavailableMessage
	}

	if err := b.ensureInitialized(ctx); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.disposed {
		return "Error: Browser tool is disposed."
	}

	page, err := b.ensureActivePageLocked()
	if err != nil {
		return fmt.Sprintf("Error: Browser not initialized. %v", err)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return "Error: Invalid arguments JSON."
	}

	action, _ := args["action"].(string)

	switch action {
	case "goto":
		rawURL, _ := args["url"].(string)
		if err := b.validateBrowserURL(rawURL); err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		_, err := page.Goto(rawURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
		})
		if err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		title, err := page.Title()
		if err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		return fmt.Sprintf("Navigated to %s. Title: '%s'", rawURL, title)

	case "click":
		selector, _ := args["selector"].(string)
		if err := page.Click(selector); err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		return fmt.Sprintf("Clicked selector: %s", selector)

	case "fill":
		selector, _ := args["selector"].(string)
		val, _ := args["value"].(string)
		if err := page.Fill(selector, val); err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		return fmt.Sprintf("Filled %s with provided value.", selector)

	case "get_text":
		selector, ok := args["selector"].(string)
		if ok && strings.TrimSpace(selector) != "" {
			content, err := page.TextContent(selector)
			if err != nil || content == "" {
				return "No text found for selector."
			}
			return content
		}
		body, err := page.TextContent("body")
		if err != nil || body == "" {
			return "Body is empty."
		}
		return body

	case "evaluate":
		if !b.config.AllowBrowserEvaluate {
			return "Error: Browser evaluate is disabled by configuration (Tooling.AllowBrowserEvaluate=false)."
		}
		script, _ := args["script"].(string)
		result, err := page.Evaluate(script)
		if err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		resBytes, _ := json.Marshal(result)
		return string(resBytes)

	case "screenshot":
		bytes, err := page.Screenshot(playwright.PageScreenshotOptions{
			FullPage: playwright.Bool(true),
		})
		if err != nil {
			return fmt.Sprintf("Browser action failed: %v", err)
		}
		return fmt.Sprintf("Screenshot taken. Base64: %s", base64.StdEncoding.EncodeToString(bytes))

	default:
		return fmt.Sprintf("Error: Unknown action '%s'", action)
	}
}

func (b *BrowserTool) ensureActivePageLocked() (playwright.Page, error) {
	if b.browser == nil {
		return nil, fmt.Errorf("browser instance is nil")
	}

	if b.page != nil && !b.page.IsClosed() {
		return b.page, nil
	}

	page, err := b.browser.NewPage()
	if err != nil {
		return nil, err
	}

	if err := b.configureUrlSafety(page); err != nil {
		page.Close()
		return nil, err
	}

	b.page = page
	return b.page, nil
}

func (b *BrowserTool) ensureInitialized(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.disposed {
		return fmt.Errorf("ObjectDisposedException: BrowserTool")
	}
	if b.initialized {
		return nil
	}
	if !b.localExecutionSupported {
		return fmt.Errorf("%s", browserLocalExecutionUnavailableMessage)
	}

	// 自动下载/确认下载对应的浏览器依赖
	err := playwright.Install(&playwright.RunOptions{})
	if err != nil {
		return fmt.Errorf("Playwright CLI install failed: %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start Playwright: %w", err)
	}
	b.pw = pw

	timeoutMs := float64(b.config.BrowserTimeoutSeconds * 1000)
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(b.config.BrowserHeadless),
		Timeout:  playwright.Float(timeoutMs),
	})
	if err != nil {
		b.pw.Stop()
		return fmt.Errorf("could not launch Chromium: %w", err)
	}
	b.browser = browser

	page, err := browser.NewPage()
	if err != nil {
		browser.Close()
		b.pw.Stop()
		return fmt.Errorf("could not create new page: %w", err)
	}
	b.page = page

	if err := b.configureUrlSafety(b.page); err != nil {
		page.Close()
		browser.Close()
		b.pw.Stop()
		return err
	}

	b.initialized = true
	return nil
}

func (b *BrowserTool) configureUrlSafety(page playwright.Page) error {
	if !b.config.UrlSafety.Enabled {
		return nil
	}

	return page.Route("**/*", func(route playwright.Route) {
		reqURL := route.Request().URL()
		u, err := url.Parse(reqURL)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			if err := b.validateBrowserURL(reqURL); err != nil {
				_ = route.Abort()
				return
			}
		}
		_ = route.Continue()
	})
}

func (b *BrowserTool) validateBrowserURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("only absolute http(s) URLs are allowed")
	}

	if !b.config.UrlSafety.Enabled {
		return nil
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}

	builtInBlocked := []string{"localhost", "*.localhost", "metadata", "metadata.google.internal"}
	if b.config.UrlSafety.BlockPrivateNetworkTargets {
		for _, pattern := range builtInBlocked {
			if globMatches(pattern, host) {
				return fmt.Errorf("URL safety blocked host '%s'", host)
			}
		}
	}

	for _, pattern := range b.config.UrlSafety.BlockedHostGlobs {
		if globMatches(pattern, host) {
			return fmt.Errorf("URL safety blocked host '%s'", host)
		}
	}

	// IP 地址解析与私有网段/CIDR校验
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = append(ips, ip)
	} else if b.config.UrlSafety.BlockPrivateNetworkTargets || len(b.config.UrlSafety.BlockedCidrs) > 0 {
		addrs, err := net.LookupHost(host)
		if err == nil {
			for _, addr := range addrs {
				if ip := net.ParseIP(addr); ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	if b.config.UrlSafety.BlockPrivateNetworkTargets {
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return fmt.Errorf("URL safety blocked private or loopback target for '%s'", host)
			}
		}
	}

	for _, cidrStr := range b.config.UrlSafety.BlockedCidrs {
		_, cidrNet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if cidrNet.Contains(ip) {
				return fmt.Errorf("URL safety blocked CIDR target for '%s'", host)
			}
		}
	}

	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	// 针对特定测试保留网段/CGNAT等做额外检查 (IPv4)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 ||
			(ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127) ||
			(ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19)) {
			return true
		}
	}
	return false
}

func globMatches(pattern, value string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
	matched, err := regexp.MatchString("(?i)"+regexPattern, value)
	return err == nil && matched
}

func (b *BrowserTool) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.disposed {
		return nil
	}
	b.disposed = true

	if b.page != nil && !b.page.IsClosed() {
		_ = b.page.Close()
	}
	if b.browser != nil {
		_ = b.browser.Close()
	}
	if b.pw != nil {
		_ = b.pw.Stop()
	}

	b.page = nil
	b.browser = nil
	b.pw = nil
	b.initialized = false
	return nil
}

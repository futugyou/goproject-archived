package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/core"
)

func ChannelSetupCommandRun(
	args []string,
	input io.Reader,
	output io.Writer,
	errwirter io.Writer,
	canPrompt bool) int {

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		PrintHelp(output)
		return 0
	}

	var channelId = strings.ToLower(strings.TrimSpace(args[0]))
	if channelId != "telegram" && channelId != "slack" && channelId != "discord" && channelId != "teams" && channelId != "whatsapp" {
		fmt.Fprintf(errwirter, "Unsupported channel: %s\n", channelId)
		fmt.Fprintln(errwirter, "Supported channels: telegram, slack, discord, teams, whatsapp")
		return 2
	}

	var parsed = CliArgsParse(args[1:])
	if parsed.ShowHelp {
		PrintHelp(output)
		return 0
	}

	rootPath := core.DefaultConfigPath
	path := parsed.GetOption("--config")
	if path != nil {
		rootPath = *path
	}
	expanded := core.GatewaySetupPathsIntance.ExpandPath(rootPath)
	configPath, err := filepath.Abs(expanded)
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 1
	}

	config, err := core.GatewayConfigFileInstance.Load(configPath)

	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		fmt.Fprintln(errwirter, "Run 'openclaw setup' first to create the base config, or pass --config <path>.")
		return 1
	}

	var nonInteractive = parsed.HasFlag("--non-interactive")
	if !nonInteractive && !canPrompt {
		fmt.Fprintln(errwirter, "Interactive channel setup requires a terminal. Re-run with explicit flags and --non-interactive.")
		return 2
	}

	switch channelId {
	case "telegram":
		err = ConfigureTelegram(config, parsed, input, output, nonInteractive)
	case "slack":
		err = ConfigureSlack(config, parsed, input, output, nonInteractive)
	case "discord":
		err = ConfigureDiscord(config, parsed, input, output, nonInteractive)
	case "teams":
		err = ConfigureTeams(config, parsed, input, output, nonInteractive)
	case "whatsapp":
		err = ConfigureWhatsApp(config, parsed, input, output, nonInteractive)
	}

	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2
	}

	var validationErrors = core.ConfigValidatorInstance.Validate(config)
	if len(validationErrors) > 0 {
		fmt.Fprintln(errwirter, "Config validation failed after channel update:")
		for _, validationError := range validationErrors {
			fmt.Fprintf(errwirter, "- %s\n", validationError)
		}
		return 1
	}

	err = core.GatewayConfigFileInstance.Save(config, configPath)
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 1
	}
	fmt.Fprintf(errwirter, "Updated channel '%s' in %s\n", channelId, configPath)
	fmt.Fprintln(errwirter, "Next steps:")
	fmt.Fprintf(errwirter, "- Restart or launch the gateway with: dotnet run --project src/OpenClaw.Gateway -c Release -- --config %s\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath))
	fmt.Fprintf(errwirter, "- Verify with: dotnet run --project src/OpenClaw.Gateway -c Release -- --config %s --doctor\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath))
	fmt.Fprintf(errwirter, "- Inspect operator posture with: OPENCLAW_BASE_URL=%s OPENCLAW_AUTH_TOKEN=%s dotnet run --project src/OpenClaw.Cli -c Release -- admin posture\n", BuildReachableBaseUrl(config.BindAddress, config.Port), config.AuthToken)

	for _, line := range BuildWebhookHints(channelId, *config) {
		fmt.Fprintf(errwirter, "- %s\n", line)
	}

	return 0
}

func ConfigureTelegram(config *core.GatewayConfig, parsed *CliArgs, input io.Reader, output io.Writer, nonInteractive bool) error {
	var channel = config.Channels.Telegram
	channel.Enabled = true
	dmPolicy, err := GetValue(parsed, input, output, "--dm-policy", "DM policy", channel.DmPolicy, nonInteractive)
	if err != nil {
		return err
	}
	channel.DmPolicy = dmPolicy
	botTokenRef, err := GetRequiredValue(parsed, input, output, "--bot-token-ref", "Telegram bot token ref", channel.BotTokenRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.BotTokenRef = botTokenRef

	updateMode, err := GetRequiredValue(parsed, input, output, "--update-mode", "Telegram update mode (webhook|long-polling)", channel.UpdateMode, nonInteractive)
	if err != nil {
		return err
	}
	updateMode = strings.ToLower(updateMode)
	if updateMode != "webhook" && updateMode != "long-polling" {
		return errors.New("Telegram update mode must be 'webhook' or 'long-polling'.")
	}
	channel.UpdateMode = updateMode

	if updateMode == "webhook" {
		channel.ValidateSignature = true
		webhookPublicBaseUrl, err := GetRequiredValue(parsed, input, output, "--public-base-url", "Telegram webhook public base URL", channel.WebhookPublicBaseUrl, nonInteractive)
		if err != nil {
			return err
		}
		channel.WebhookPublicBaseUrl = webhookPublicBaseUrl
		webhookSecretTokenRef, err := GetRequiredValue(parsed, input, output, "--webhook-secret-ref", "Telegram webhook secret ref", channel.WebhookSecretTokenRef, nonInteractive)
		if err != nil {
			return err
		}
		channel.WebhookSecretTokenRef = webhookSecretTokenRef
	}

	config.Channels.Telegram = channel
	return nil
}

func ConfigureSlack(config *core.GatewayConfig, parsed *CliArgs, input io.Reader, output io.Writer, nonInteractive bool) error {
	var channel = config.Channels.Slack
	channel.Enabled = true
	dmPolicy, err := GetValue(parsed, input, output, "--dm-policy", "DM policy", channel.DmPolicy, nonInteractive)
	if err != nil {
		return err
	}
	channel.DmPolicy = dmPolicy
	botTokenRef, err := GetRequiredValue(parsed, input, output, "--bot-token-ref", "Slack bot token ref", channel.BotTokenRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.BotTokenRef = botTokenRef
	signingSecretRef, err := GetRequiredValue(parsed, input, output, "--signing-secret-ref", "Slack signing secret ref", channel.SigningSecretRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.SigningSecretRef = signingSecretRef

	channel.ValidateSignature = true
	config.Channels.Slack = channel
	return nil
}

func ConfigureDiscord(config *core.GatewayConfig, parsed *CliArgs, input io.Reader, output io.Writer, nonInteractive bool) error {
	var channel = config.Channels.Discord
	channel.Enabled = true
	dmPolicy, err := GetValue(parsed, input, output, "--dm-policy", "DM policy", channel.DmPolicy, nonInteractive)
	if err != nil {
		return err
	}
	channel.DmPolicy = dmPolicy
	botTokenRef, err := GetRequiredValue(parsed, input, output, "--bot-token-ref", "Discord bot token ref", channel.BotTokenRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.BotTokenRef = botTokenRef
	applicationIdRef, err := GetRequiredValue(parsed, input, output, "--application-id-ref", "Discord application ID ref", channel.ApplicationIdRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.ApplicationIdRef = applicationIdRef
	bublicKeyRef, err := GetRequiredValue(parsed, input, output, "--public-key-ref", "Discord public key ref", channel.PublicKeyRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.PublicKeyRef = bublicKeyRef
	channel.ValidateSignature = true
	channel.RegisterSlashCommands = true
	config.Channels.Discord = channel
	return nil
}

func ConfigureTeams(config *core.GatewayConfig, parsed *CliArgs, input io.Reader, output io.Writer, nonInteractive bool) error {
	var channel = config.Channels.Teams
	channel.Enabled = true
	dmPolicy, err := GetValue(parsed, input, output, "--dm-policy", "DM policy", channel.DmPolicy, nonInteractive)
	if err != nil {
		return err
	}
	channel.DmPolicy = dmPolicy
	appIdRef, err := GetRequiredValue(parsed, input, output, "--app-id-ref", "Teams app ID ref", channel.AppIdRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.AppIdRef = appIdRef
	appPasswordRef, err := GetRequiredValue(parsed, input, output, "--app-password-ref", "Teams app password ref", channel.AppPasswordRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.AppPasswordRef = appPasswordRef
	tenantIdRef, err := GetRequiredValue(parsed, input, output, "--tenant-id-ref", "Teams tenant ID ref", channel.TenantIdRef, nonInteractive)
	if err != nil {
		return err
	}
	channel.TenantIdRef = tenantIdRef
	channel.ValidateToken = true
	channel.RequireMention = true
	config.Channels.Teams = channel
	return nil
}

func ConfigureWhatsApp(config *core.GatewayConfig, parsed *CliArgs, input io.Reader, output io.Writer, nonInteractive bool) error {
	var channel = config.Channels.WhatsApp
	channel.Enabled = true
	dmPolicy, err := GetValue(parsed, input, output, "--dm-policy", "DM policy", channel.DmPolicy, nonInteractive)
	if err != nil {
		return err
	}
	channel.DmPolicy = dmPolicy
	currentValue := "official"
	if channel.Type == "bridge" {
		currentValue = channel.Type
	}

	mode, err := GetValue(parsed, input, output, "--mode", "WhatsApp mode (official|bridge)", currentValue, nonInteractive)
	if err != nil {
		return err
	}
	mode = strings.ToLower(mode)
	if mode != "official" && mode != "bridge" {
		return errors.New("WhatsApp mode must be 'official' or 'bridge'.")
	}

	channel.Type = mode
	if mode == "official" {
		cloudApiTokenRef, err := GetRequiredValue(parsed, input, output, "--cloud-api-token-ref", "WhatsApp Cloud API token ref", channel.CloudApiTokenRef, nonInteractive)
		if err != nil {
			return err
		}
		channel.CloudApiTokenRef = cloudApiTokenRef
		phoneNumberId, err := GetRequiredValue(parsed, input, output, "--phone-number-id", "WhatsApp phone number ID", channel.PhoneNumberId, nonInteractive)
		if err != nil {
			return err
		}
		channel.PhoneNumberId = phoneNumberId
		webhookAppSecretRef, err := GetRequiredValue(parsed, input, output, "--app-secret-ref", "WhatsApp webhook app secret ref", channel.WebhookAppSecretRef, nonInteractive)
		if err != nil {
			return err
		}
		channel.WebhookAppSecretRef = webhookAppSecretRef
		channel.ValidateSignature = true
	} else {
		bridgeUrl, err := GetRequiredValue(parsed, input, output, "--bridge-url", "WhatsApp bridge URL", channel.BridgeUrl, nonInteractive)
		if err != nil {
			return err
		}
		channel.BridgeUrl = bridgeUrl
		bridgeTokenRef, err := GetRequiredValue(parsed, input, output, "--bridge-token-ref", "WhatsApp bridge token ref", channel.BridgeTokenRef, nonInteractive)
		if err != nil {
			return err
		}
		channel.BridgeTokenRef = bridgeTokenRef
	}

	config.Channels.WhatsApp = channel
	return nil
}

func GetRequiredValue(parsed *CliArgs, input io.Reader, output io.Writer, option, label, currentValue string, nonInteractive bool) (string, error) {
	value, err := GetValue(parsed, input, output, option, label, currentValue, nonInteractive)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", option)
	}

	return value, nil
}

func GetValue(parsed *CliArgs, input io.Reader, output io.Writer, option, label, currentValue string, nonInteractive bool) (string, error) {
	var value = parsed.GetOption(option)
	if value != nil && strings.TrimSpace(*value) != "" {
		return *value, nil
	}

	if nonInteractive {
		if strings.TrimSpace(currentValue) != "" {
			return currentValue, nil
		}

		return "", fmt.Errorf("%s is required when --non-interactive is set.", option)
	}

	return Prompt(output, input, label, currentValue)
}

func Prompt(output io.Writer, input io.Reader, label, defaultValue string) (string, error) {
	fmt.Fprintf(output, "%s [%s]: ", label, defaultValue)

	scanner := bufio.NewScanner(input)
	if scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			return defaultValue, nil
		}
		return value, nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return defaultValue, nil
}

func BuildWebhookHints(channelId string, config core.GatewayConfig) []string {
	switch channelId {
	case "telegram":
		if config.Channels.Telegram.UsesLongPolling() {
			return []string{"Telegram long polling uses outbound HTTPS only; any existing webhook is removed on startup."}
		}
		return []string{fmt.Sprintf("Register Telegram webhook: %s", TrimTrailingSlash(config.Channels.Telegram.WebhookPublicBaseUrl)+config.Channels.Telegram.WebhookPath)}
	case "slack":
		return []string{fmt.Sprintf("Slack events URL: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.Slack.WebhookPath)),
			fmt.Sprintf("Slack slash command URL: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.Slack.SlashCommandPath))}
	case "discord":
		return []string{fmt.Sprintf("Discord interactions URL: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.Discord.WebhookPath))}
	case "teams":
		return []string{fmt.Sprintf("Teams Bot Framework endpoint: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.Teams.WebhookPath))}
	case "whatsapp":
		if config.Channels.WhatsApp.Type == "bridge" {
			return []string{fmt.Sprintf("WhatsApp bridge inbound URL expected by the gateway: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.WhatsApp.WebhookPath))}
		}
		return []string{fmt.Sprintf("WhatsApp official webhook URL: %s", BuildRouteHint(config.BindAddress, config.Port, config.Channels.WhatsApp.WebhookPath))}
	default:
		return []string{}
	}
}

func BuildRouteHint(bindAddress string, port int, path string) string {
	return TrimTrailingSlash(BuildReachableBaseUrl(bindAddress, port)) + path
}

func TrimTrailingSlash(value string) string {
	return strings.TrimSuffix(value, "/")
}

func BuildReachableBaseUrl(bindAddress string, port int) string {
	if bindAddress == "0.0.0.0" || bindAddress == "::" || bindAddress == "[::]" {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	if strings.Contains(bindAddress, ":") && !strings.HasPrefix(bindAddress, "[") {
		return fmt.Sprintf("http://[%s]:%d", bindAddress, port)
	}

	return fmt.Sprintf("http:/[%s:%d", bindAddress, port)
}

func PrintHelp(output io.Writer) {
	fmt.Fprintln(output,
		` 
            openclaw setup channel

            Usage:
              openclaw setup channel <telegram|slack|discord|teams|whatsapp> [--config <path>] [--non-interactive] [channel-specific options]

            Common options:
              --config <path>       External gateway config to update (default: ~/.openclaw/config/openclaw.settings.json)
              --non-interactive     Require explicit values instead of prompting
              --dm-policy <mode>    open | pairing | closed

            Telegram options:
              --bot-token-ref <ref>
              --update-mode <webhook|long-polling>
              --public-base-url <url>       Required for webhook mode
              --webhook-secret-ref <ref>    Required for webhook mode

            Slack options:
              --bot-token-ref <ref>
              --signing-secret-ref <ref>

            Discord options:
              --bot-token-ref <ref>
              --application-id-ref <ref>
              --public-key-ref <ref>

            Teams options:
              --app-id-ref <ref>
              --app-password-ref <ref>
              --tenant-id-ref <ref>

            WhatsApp options:
              --mode <official|bridge>
              --cloud-api-token-ref <ref>
              --phone-number-id <id>
              --app-secret-ref <ref>
              --bridge-url <url>
              --bridge-token-ref <ref>
           `)
}

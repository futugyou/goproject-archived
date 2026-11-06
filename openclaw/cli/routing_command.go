package cli

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func RoutingCommandRun(args []string) (int, error) {
	return routingCommandRun(args, os.Stdout, os.Stderr)
}

func routingCommandPrintHelp(output io.Writer) {
	output.Write([]byte(
		`
            openclaw routing

            Usage:
                            openclaw routing onboard [--router recommended|openrouter-mix|disabled]
              openclaw routing configure <router|providers> [options]
                            openclaw routing configure router [--router recommended|openrouter-mix|disabled] [--margin-upgrade-threshold <n>] [--r1-rescue-threshold <n>] [--under-routing-safety-threshold <n>] [--deep-turn-threshold <n>] [--config <path>]
                            openclaw routing configure providers --tier <T0|T1|T2|T3> [--model-profile <id>] [--fallback-profile <id>] [--reasoning-level <level>] [--response-policy <policy>] [--image-model-profile <id>] [--allowed-tools <csv>] [--preferred-tags <csv>] [--prompt-mode <full|minimal|compact>] [--disable-tools <true|false>] [--config <path>]
              openclaw routing providers
              openclaw routing status
              openclaw routing diagnostics <on|off> [--config <path>]

            Notes:
              - Routing remains configuration-driven through OpenClaw:DynamicTurnRouting.
                            - --router recommended enables dynamic routing with existing tier mappings.
                            - --router openrouter-mix enables routing and appends openrouter-oriented preferred tags by tier.
                            - --router disabled turns ONNX, Jev, and Laya routing off.
              - Jev shadow/active modes are opt-in via DynamicTurnRouting.Jev.Mode; restart after configuration changes.
              - This command group provides operator-oriented routing entry points.
            `))
}

func routingCommandRun(args []string, output, errWriter io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		routingCommandPrintHelp(output)
		return 0, nil
	}

	parsed := CliArgsParse(args)
	if parsed.ShowHelp || len(parsed.Positionals) == 0 {
		routingCommandPrintHelp(output)
		return 0, nil
	}

	command := strings.ToLower(parsed.Positionals[0])

	switch command {
	case "onboard":
		return routingCommandRunOnboard(parsed, output, errWriter)
	case "configure":
		return routingCommandRunConfigure(parsed, output, errWriter)
	case "providers":
		return routingCommandRunProviders(parsed, output, errWriter)
	case "status":
		return routingCommandRunStatus(parsed, output, errWriter)
	case "diagnostics":
		return routingCommandRunDiagnostics(parsed, output, errWriter)
	default:
		return 2, fmt.Errorf("unsupport command: %s", command)
	}
}

func routingCommandRunDiagnostics(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	if len(parsed.Positionals) < 2 {
		msg := "diagnostics mode is required. Use: openclaw routing diagnostics <on|off>"
		fmt.Fprintf(errWriter, "%s\n", msg)
		return 2, errors.New(msg)
	}

	mode := strings.ToLower(parsed.Positionals[1])
	if mode != "on" && mode != "off" {
		msg := "diagnostics mode must be on or off."
		fmt.Fprintf(errWriter, "%s\n", msg)
		return 2, errors.New(msg)
	}

	path, err := resolveConfigPath(parsed)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	config.DynamicTurnRouting.Policy.EnableDiagnostics = mode == "on"
	if err := core.GatewayConfigFileInstance.Save(config, path); err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	fmt.Fprintf(output, "routing diagnostics set to %s (%s)\n", mode, core.GatewaySetupPathsIntance.QuoteIfNeeded(path))
	return 0, nil
}

func routingCommandRunStatus(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	path, err := resolveConfigPath(parsed)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	routing := config.DynamicTurnRouting
	policy := routing.Policy

	fmt.Fprintf(output, "enabled=%t\n", routing.Enabled)
	fmt.Fprintf(output, "jevMode=%s\n", routing.Jev.Mode)
	fmt.Fprintf(output, "jevModel=%s\n", routing.Jev.Model)
	fmt.Fprintf(output, "layaMode=%s\n", routing.Laya.Mode)
	fmt.Fprintf(output, "layaModel=%s\n", routing.Laya.Model)
	fmt.Fprintf(output, "layaCalibrationId=%s\n", routing.Laya.CalibrationId)
	fmt.Fprintf(output, "bundlePath=%s\n", routing.BundlePath)
	fmt.Fprintf(output, "classifier=%s\n", routing.Assets.ClassifierModelPath)
	fmt.Fprintf(output, "embedding=%s\n", routing.Assets.EmbeddingModelPath)
	fmt.Fprintf(output, "tokenizer=%s\n", routing.Assets.TokenizerPath)
	fmt.Fprintf(output, "diagnostics=%t\n", policy.EnableDiagnostics)
	fmt.Fprintf(output, "marginUpgradeThreshold=%f\n", policy.MarginUpgradeThreshold)
	fmt.Fprintf(output, "r1RescueThreshold=%f\n", policy.R1RescueThreshold)
	fmt.Fprintf(output, "underRoutingSafetyThreshold=%f\n", policy.UnderRoutingSafetyThreshold)
	fmt.Fprintf(output, "deepTurnThreshold=%d\n", policy.DeepConversationTurnIndexThreshold)
	return 0, nil
}

func writeTier(name string, tier *core.DynamicTurnRoutingTierTarget, output io.Writer) {
	tools := "disabled"
	if !tier.DisableTools {
		tools = joinOrNone(tier.AllowedTools)
	}

	fmt.Fprintf(output, "%s\tprofile=%s\tfallback=%s\treasoning=%s\tresponse=%s\timage=%s\tpromptMode=%s\ttools=%s\ttags=%s\n",
		name, tier.ModelProfileId, tier.DirectModelFallbackProfileId, tier.ReasoningLevel, tier.ResponsePolicy,
		tier.ImageCapableModelProfileId, tier.PromptMode, tools, joinOrNone(tier.PreferredTags))
}

func joinOrNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}

	return strings.Join(s, ",")
}

func routingCommandRunProviders(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	path, err := resolveConfigPath(parsed)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		fmt.Fprintf(errWriter, "%s\n", err.Error())
		return 1, err
	}

	tiers := config.DynamicTurnRouting.Policy.Tiers

	writeTier("T0", tiers.T0, output)
	writeTier("T1", tiers.T1, output)
	writeTier("T2", tiers.T2, output)
	writeTier("T3", tiers.T3, output)

	return 0, nil
}

func routingCommandRunConfigure(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	if len(parsed.Positionals) < 2 {
		msg := "configure target is required. Use: openclaw routing configure <router|providers> [options]"
		fmt.Fprintf(errWriter, "%s\n", msg)
		return 2, errors.New(msg)
	}

	target := strings.ToLower(parsed.Positionals[1])

	switch target {
	case "router":
		return routingCommandConfigureRouter(parsed, output, errWriter)
	case "providers":
		return routingCommandConfigureProviders(parsed, output, errWriter)
	default:
		msg := "configure target must be router or providers."
		fmt.Fprintf(errWriter, "%s\n", msg)
		return 2, errors.New(msg)
	}
}

func resolveTier(tiers *core.DynamicTurnRoutingTierMap, tierName string) (*core.DynamicTurnRoutingTierTarget, error) {
	switch strings.ToUpper(strings.TrimSpace(tierName)) {
	case "T0":
		return tiers.T0, nil
	case "T1":
		return tiers.T1, nil
	case "T2":
		return tiers.T2, nil
	case "T3":
		return tiers.T3, nil
	default:
		return nil, errors.New("--tier must be one of T0/T1/T2/T3")
	}
}

func routingCommandConfigureProviders(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	tierName := util.Deref(parsed.GetOption("--tier"))
	if tierName == "" {
		msg := "--tier <T0|T1|T2|T3> is required for provider configuration."
		fmt.Fprintf(errWriter, "%s\n", msg)
		return 2, errors.New(msg)
	}
	path, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		return 1, err
	}

	tier, err := resolveTier(config.DynamicTurnRouting.Policy.Tiers, tierName)
	if err != nil {
		return 1, err
	}

	modelProfile := util.Deref(parsed.GetOption("--model-profile"))
	if modelProfile != "" {
		tier.ModelProfileId = modelProfile
	}

	fallbackProfile := util.Deref(parsed.GetOption("--fallback-profile"))
	if fallbackProfile != "" {
		tier.DirectModelFallbackProfileId = fallbackProfile
	}

	reasoningLevel := util.Deref(parsed.GetOption("--reasoning-level"))
	if reasoningLevel != "" {
		tier.ReasoningLevel = reasoningLevel
	}

	responsePolicy := util.Deref(parsed.GetOption("--response-policy"))
	if responsePolicy != "" {
		tier.ResponsePolicy = responsePolicy
	}

	imageProfile := util.Deref(parsed.GetOption("--image-model-profile"))
	if imageProfile != "" {
		tier.ImageCapableModelProfileId = imageProfile
	}

	allowedTools := util.Deref(parsed.GetOption("--allowed-tools"))
	if allowedTools != "" {
		tier.AllowedTools = strings.Split(allowedTools, ",")
	}

	preferredTags := util.Deref(parsed.GetOption("--preferred-tags"))
	if preferredTags != "" {
		tier.PreferredTags = strings.Split(preferredTags, ",")
	}

	promptMode := util.Deref(parsed.GetOption("--prompt-mode"))
	if promptMode != "" {
		tier.PromptMode = promptMode
	}

	disableTools, err := strconv.ParseBool(util.Deref(parsed.GetOption("--disable-tools")))
	if err != nil {
		return 1, err
	}
	tier.DisableTools = disableTools

	if err := core.GatewayConfigFileInstance.Save(config, path); err != nil {
		return 1, err
	}

	fmt.Fprintf(output, "routing configure providers saved for %s (%s)\n", strings.ToUpper(tierName), core.GatewaySetupPathsIntance.QuoteIfNeeded(path))
	return 0, nil
}

func tryReadFloatOption(parsed *CliArgs, option string) (float64, string, bool) {
	raw := util.Deref(parsed.GetOption(option))
	if raw == "" {
		return 0, "", false
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Sprintf("%s must be a valid number.", option), false
	}

	return value, "", true
}

func tryReadIntOption(parsed *CliArgs, option string) (int64, string, bool) {
	raw := util.Deref(parsed.GetOption(option))
	if raw == "" {
		return 0, "", false
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Sprintf("%s must be a valid integer.", option), false
	}

	return value, "", true
}

func routingCommandConfigureRouter(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	path, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		return 1, err
	}

	mode, modeError, ok := tryGetRouterMode(util.Deref(parsed.GetOption(RouterOption)), "")
	if !ok {
		errWriter.Write([]byte(modeError))
		return 2, errors.New(modeError)
	}

	if mode != "" {
		applyRouterMode(&config.DynamicTurnRouting, mode)
	}

	policy := config.DynamicTurnRouting.Policy

	margin, parseError, ok := tryReadFloatOption(parsed, "--margin-upgrade-threshold")
	if ok {
		policy.MarginUpgradeThreshold = margin
	} else {
		return 2, errors.New(parseError)
	}

	r1, parseError, ok := tryReadFloatOption(parsed, "--r1-rescue-threshold")
	if ok {
		policy.R1RescueThreshold = r1
	} else {
		return 2, errors.New(parseError)
	}

	safety, parseError, ok := tryReadFloatOption(parsed, "--under-routing-safety-threshold")
	if ok {
		policy.UnderRoutingSafetyThreshold = safety
	} else {
		return 2, errors.New(parseError)
	}

	deepTurn, parseError, ok := tryReadIntOption(parsed, "--deep-turn-threshold")
	if ok {
		policy.DeepConversationTurnIndexThreshold = deepTurn
	} else {
		return 2, errors.New(parseError)
	}

	if err := core.GatewayConfigFileInstance.Save(config, path); err != nil {
		return 2, err
	}

	fmt.Fprintf(output, "routing configure router saved (%s)\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(path))
	return 0, nil
}

const RouterOption string = "--router"
const RouterModeRecommended string = "recommended"
const RouterModeOpenRouterMix string = "openrouter-mix"
const RouterModeDisabled string = "disabled"

func tryGetRouterMode(rawValue, defaultMode string) (string, string, bool) {
	if rawValue == "" {
		return defaultMode, "", true
	}

	normalized := strings.ToLower(strings.TrimSpace(rawValue))
	if normalized == RouterModeRecommended || normalized == RouterModeOpenRouterMix || normalized == RouterModeDisabled {
		return normalized, "", true
	}

	return "", "--router must be one of recommended, openrouter-mix, disabled.", false
}

func resolveConfigPath(parsed *CliArgs) (string, error) {
	rootPath := util.Deref(parsed.GetOption("--config"))
	if rootPath == "" {
		rootPath = core.DefaultConfigPath
	}

	expanded := core.GatewaySetupPathsIntance.ExpandPath(rootPath)
	return filepath.Abs(expanded)
}

func applyRouterMode(routing *core.DynamicTurnRoutingConfig, mode string) {
	routing.Enabled = mode != RouterModeDisabled
	if mode == RouterModeDisabled {
		routing.Jev.Mode = "disabled"
		routing.Laya.Mode = "disabled"
	}

	if mode == RouterModeOpenRouterMix {
		applyOpenRouterTags(routing.Policy.Tiers.T0, "cost")
		applyOpenRouterTags(routing.Policy.Tiers.T1, "fast")
		applyOpenRouterTags(routing.Policy.Tiers.T2, "tools")
		applyOpenRouterTags(routing.Policy.Tiers.T3, "reasoning")
	}
}

func applyOpenRouterTags(tier *core.DynamicTurnRoutingTierTarget, routeTag string) {
	tagsMap := map[string]struct{}{}
	for _, v := range tier.PreferredTags {
		tagsMap[v] = struct{}{}
	}
	tagsMap["openrouter"] = struct{}{}
	tagsMap[routeTag] = struct{}{}

	tags := slices.Collect(maps.Keys(tagsMap))

	slices.Sort(tags)

	tier.PreferredTags = tags
}

func routingCommandRunOnboard(parsed *CliArgs, output, errWriter io.Writer) (int, error) {
	mode, modeError, ok := tryGetRouterMode(util.Deref(parsed.GetOption(RouterOption)), RouterModeRecommended)
	if !ok {
		errWriter.Write([]byte(modeError))
		return 2, errors.New(modeError)
	}

	path, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(path)
	if err != nil {
		return 1, err
	}

	applyRouterMode(&config.DynamicTurnRouting, mode)
	if err := core.GatewayConfigFileInstance.Save(config, path); err != nil {
		return 1, err
	}

	fmt.Fprintf(output, "routing onboard completed with router=%s (%s)\n", mode, core.GatewaySetupPathsIntance.QuoteIfNeeded(path))
	return 0, nil
}

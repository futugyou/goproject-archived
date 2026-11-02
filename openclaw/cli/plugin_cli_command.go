package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

const MaxDescribeBytes = 1024 * 1024
const ExecuteTimeout = 10 * time.Minute
const DescribeTimeout = 20 * time.Second

type PluginCliDescribeResult struct {
	Success  bool
	Error    string
	Commands []core.BridgeCliCommandRegistration
}

func FailurePluginCliDescribeResult(msg string) *PluginCliDescribeResult {
	return &PluginCliDescribeResult{Error: msg}
}

func tryKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("Unable to terminate plugin CLI process: %s\n", err.Error())
	}
}

func loadBlockedPluginIds(storagePath string) map[string]struct{} {
	result := map[string]struct{}{}
	rootedStoragePath := storagePath
	var err error
	if !filepath.IsAbs(storagePath) {
		rootedStoragePath, err = filepath.Abs(storagePath)
		if err != nil {
			return result
		}
	}

	statePath := filepath.Join(rootedStoragePath, "admin", "plugin-state.json")

	states, err := util.LoadOneFile[[]core.PluginOperatorState](context.Background(), statePath)
	if err != nil {
		return result
	}

	for _, state := range *states {
		if state.Disabled || state.Quarantined {
			result[state.PluginId] = struct{}{}
		}
	}
	return result
}

func readCapped(stdoutPipe io.ReadCloser) *string {
	limitReader := io.LimitReader(stdoutPipe, MaxDescribeBytes+1)

	var buf bytes.Buffer
	_, err := buf.ReadFrom(limitReader)
	if err != nil {
		return nil
	}

	if int64(buf.Len()) > MaxDescribeBytes {
		return nil
	}

	result := buf.String()
	return &result
}

func createProcess(entryPath, pluginId string, config *json.RawMessage, bridgeScript, mode string, argv []string) (*exec.Cmd, error) {
	entry, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, err
	}
	workDir := filepath.Dir(entryPath)
	var args []string = []string{
		"--experimental-vm-modules",
		bridgeScript,
		mode,
	}
	args = append(args, argv...)

	var envString strings.Builder
	fmt.Fprintf(&envString, "%s=%s ", "OPENCLAW_PLUGIN_CLI_ENTRY", entry)
	fmt.Fprintf(&envString, "%s=%s ", "OPENCLAW_PLUGIN_CLI_ID", pluginId)
	if config != nil {
		var pluginConfig string
		json.Unmarshal(*config, &pluginConfig)
		fmt.Fprintf(&envString, "%s=%s ", "OPENCLAW_PLUGIN_CLI_CONFIG_BASE64", pluginConfig)
	}

	args = append(args, envString.String())

	cmd := exec.Command("node", args...)
	cmd.Dir = workDir

	return cmd, nil
}

func PluginCliCommandDescribe(ctx context.Context, entryPath, pluginId string, config *json.RawMessage, bridgeScript string) *PluginCliDescribeResult {
	cmd, err := createProcess(entryPath, pluginId, config, bridgeScript, "--cli-describe", nil)
	if err != nil {
		return FailurePluginCliDescribeResult(fmt.Sprintf("Unable to start Node.js for plugin CLI discovery: %s", err.Error()))
	}

	cmd.WaitDelay = DescribeTimeout
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return FailurePluginCliDescribeResult(err.Error())
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return FailurePluginCliDescribeResult(err.Error())
	}
	if err := cmd.Start(); err != nil {
		return FailurePluginCliDescribeResult(err.Error())
	}

	var stdoutResult, stderrResult *string

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		stdoutResult = readCapped(stdoutPipe)
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		stderrResult = readCapped(stderrPipe)
	}()

	<-done
	<-done

	err = cmd.Wait()
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		tryKill(cmd)
		return FailurePluginCliDescribeResult(err.Error())
	}

	stdoutMsg := util.Deref(stdoutResult)
	stderrMsg := util.Deref(stderrResult)

	if cmd.ProcessState.ExitCode() != 0 {
		if stdoutMsg == "" {
			return FailurePluginCliDescribeResult(fmt.Sprintf("Plugin CLI discovery exited with code %d", cmd.ProcessState.ExitCode()))
		} else {
			return FailurePluginCliDescribeResult(stderrMsg)
		}
	}

	if stdoutMsg != "" || stderrMsg != "" {
		return FailurePluginCliDescribeResult("Plugin CLI discovery output exceeded the 1 MiB limit.")
	}

	descriptors := strings.Split(stdoutMsg, "\r\n")
	descriptorLine := ""
	if len(descriptors) > 0 {
		descriptorLine = descriptors[len(descriptors)-1]
		var commands []core.BridgeCliCommandRegistration
		if err := json.Unmarshal([]byte(descriptorLine), &commands); err != nil {
			return FailurePluginCliDescribeResult(err.Error())
		}
		return &PluginCliDescribeResult{Success: true, Commands: commands}
	}

	return FailurePluginCliDescribeResult("Plugin CLI discovery returned unreadable metadata.")
}

func pluginCliCommandExecute(ctx context.Context, entryPath, pluginId string, config *json.RawMessage, bridgeScript string, argv []string) (int, error) {
	cmd, err := createProcess(entryPath, pluginId, config, bridgeScript, "--cli-run", argv)
	if err != nil {
		return 1, err
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Unable to start plugin CLI command: (%T).\n", err)
		return 1, err
	}

	ct, cancel := context.WithCancel(ctx)
	if util.IsInputRedirected() {
		ct, cancel = context.WithTimeout(ctx, ExecuteTimeout)
	}

	defer cancel()

	err = cmd.Wait()
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		tryKill(cmd)
		return 124, err
	}

	if err := ct.Err(); err != nil {
		tryKill(cmd)
		return 1, err
	}

	return cmd.ProcessState.ExitCode(), nil
}

type PluginCliCommandMatche struct {
	Plugin core.DiscoveredPlugin
	Config *json.RawMessage
}

func TryPluginCliCommandRun(ctx context.Context, command string, args []string, pluginsConfig core.PluginsConfig, workspacePath string, blockedPluginIds []string, bridgeScript string) (int, error) {
	var discovered = core.PluginDiscoveryInstance.Filter(core.PluginDiscoveryInstance.Discover(&pluginsConfig, workspacePath), &pluginsConfig)
	matches := []PluginCliCommandMatche{}
	manifestIds := []string{}
	for _, plugin := range discovered {
		if plugin.Format == "bundle" ||
			slices.Contains(blockedPluginIds, "*") ||
			slices.Contains(blockedPluginIds, plugin.Manifest.Id) ||
			plugin.EntryPath == "" {
			continue
		}

		entryConfig, ok := pluginsConfig.Entries[plugin.Manifest.Id]

		var pluginConfig *json.RawMessage
		if ok && entryConfig != nil {
			pluginConfig = entryConfig.Config
		}
		diagnostics := append(
			core.ValidateDiscoveredPlugin(plugin),
			core.PluginConfigValidatorinstance.Validate(plugin.Manifest, util.Deref(pluginConfig))...,
		)

		needcontinue := false
		for _, item := range diagnostics {
			if item.Severity == "error" {
				needcontinue = true
				break
			}
		}
		if needcontinue {
			continue
		}

		description := PluginCliCommandDescribe(ctx, plugin.EntryPath, plugin.Manifest.Id, pluginConfig, bridgeScript)
		if !description.Success {
			fmt.Printf("Plugin '%s' did not report CLI commands: %s\n", plugin.Manifest.Id, description.Error)
			continue
		}

		for _, com := range description.Commands {
			if com.Name == command {
				matches = append(matches, PluginCliCommandMatche{Plugin: plugin, Config: pluginConfig})
			}
		}

		manifestIds = append(manifestIds, plugin.Manifest.Id)
	}

	if len(matches) == 0 {
		return -1, nil
	}

	if len(matches) > 1 {
		fmt.Printf("Plugin CLI command '%s' is ambiguous; registered by: %s\n", command, strings.Join(manifestIds, ", "))
		return 2, nil
	}

	match := matches[0]
	return pluginCliCommandExecute(
		ctx,
		match.Plugin.EntryPath,
		match.Plugin.Manifest.Id,
		match.Config,
		bridgeScript,
		append([]string{command}, args...))
}

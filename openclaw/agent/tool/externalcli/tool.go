package externalcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type ExternalCliTool struct {
	registry core.IExternalCliConnectorRegistry
	runner   core.IExternalCliRunner
	audit    core.IExternalCliAuditSink
	events   core.IExternalCliEventSink
}

type emptyAuditSink struct{}

func (*emptyAuditSink) Record(entry *core.ExternalCliAuditEntry) error { return nil }

type emptyEventSink struct{}

func (*emptyEventSink) Record(entry *core.ExternalCliRuntimeEvent) error { return nil }

func New(registry core.IExternalCliConnectorRegistry,
	runner core.IExternalCliRunner,
	audit core.IExternalCliAuditSink,
	events core.IExternalCliEventSink) *ExternalCliTool {
	if audit == nil {
		audit = &emptyAuditSink{}
	}
	if events == nil {
		events = &emptyEventSink{}
	}
	return &ExternalCliTool{runner: runner, audit: audit, registry: registry, events: events}
}

func (a *ExternalCliTool) Name() string {
	return "external_cli"
}

func (a *ExternalCliTool) Description() string {
	return "Run governed allowlisted external CLI commands by connector and command name. This is not a free-form shell."
}

func (a *ExternalCliTool) ParameterSchema() string {
	return `
	{
  "type": "object",
  "properties": {
    "action": {
      "type": "string",
      "enum": ["list_connectors", "connector_status", "list_commands", "command_schema", "preview", "execute"],
      "default": "list_connectors"
    },
    "connector": {
      "type": "string",
      "description": "Configured connector name, such as gh, az, kubectl, stripe, or lark."
    },
    "command": {
      "type": "string",
      "description": "Allowlisted command name inside the connector."
    },
    "parameters": {
      "type": "object",
      "additionalProperties": true,
      "description": "Named command parameters. Unknown parameters are rejected unless the command explicitly allows them."
    },
    "execute_dry_run": {
      "type": "boolean",
      "default": false,
      "description": "For preview only: execute the explicit dry-run template when the command supports it."
    },
    "approved_fingerprint": {
      "type": "string",
      "description": "Optional approval fingerprint for direct operator-approved execution."
    },
    "approval_reason": {
      "type": "string",
      "description": "Optional operator approval reason."
    }
  },
  "required": ["action"]
}
`
}

func (a *ExternalCliTool) Execute(ctx context.Context, argumentsJson string) string {
	return "Error: external_cli requires execution context."
}

func buildSummary(request core.ExternalCliToolRequest) string {
	if request.Connector == "" || request.Command == "" {
		return "Use an external CLI connector."
	}

	switch request.Action {
	case "execute":
		return fmt.Sprintf("Execute external CLI command %s/%s.", request.Connector, request.Command)
	case "preview":
		return fmt.Sprintf("Preview external CLI command %s/%s.", request.Connector, request.Command)
	default:
		return fmt.Sprintf("Inspect external CLI connector %s.", request.Connector)
	}
}

func toPreviewRequest(request *core.ExternalCliToolRequest) *core.ExternalCliPreviewRequest {
	return &core.ExternalCliPreviewRequest{
		Connector:     request.Connector,
		Command:       request.Command,
		Parameters:    request.Parameters,
		ExecuteDryRun: request.ExecuteDryRun,
	}
}

func (a *ExternalCliTool) recordEvent(
	execContext core.ToolExecutionContext,
	action,
	severity,
	summary string,
	request core.ExternalCliToolRequest,
	preview *core.ExternalCliInvocationPreview) error {
	connector := ""
	command := ""
	riskLevel := ""
	readOnly := ""
	fingerprint := ""
	if preview != nil {
		connector = preview.Connector
		command = preview.Command
		riskLevel = preview.RiskLevel
		fingerprint = preview.Fingerprint
		readOnly = strings.ToLower(strconv.FormatBool(preview.ReadOnly))
	}
	if connector == "" {
		connector = request.Connector
	}
	if command == "" {
		command = request.Command
	}
	return a.events.Record(&core.ExternalCliRuntimeEvent{
		SessionId: execContext.Session.Id,
		ChannelId: execContext.Session.ChannelId,
		SenderId:  execContext.Session.SenderId,
		Action:    action,
		Severity:  severity,
		Summary:   summary,
		Metadata: map[string]string{
			"connector":   connector,
			"command":     command,
			"riskLevel":   riskLevel,
			"readOnly":    readOnly,
			"fingerprint": fingerprint,
		},
	})
}

func (a *ExternalCliTool) recordAudit(execContext core.ToolExecutionContext, result core.ExternalCliExecutionResult, request core.ExternalCliToolRequest) error {
	approvalFingerprint := request.ApprovedFingerprint
	if approvalFingerprint == "" {
		approvalFingerprint = result.Preview.Fingerprint
	}
	return a.audit.Record(&core.ExternalCliAuditEntry{
		Id:                  fmt.Sprintf("ecli_%s", util.CleanUUID())[:21],
		TimestampUtc:        time.Now().UTC(),
		SessionId:           execContext.Session.Id,
		ChannelId:           execContext.Session.ChannelId,
		SenderId:            execContext.Session.SenderId,
		Connector:           result.Preview.Connector,
		Command:             result.Preview.Command,
		Executable:          result.Preview.Executable,
		ArgsHash:            computeArgsHash(result.Preview.Arguments),
		RedactedArgsPreview: result.Preview.RedactedCommandLine,
		ParametersHash:      result.Preview.ParametersHash,
		ApprovalFingerprint: approvalFingerprint,
		ExitCode:            result.ExitCode,
		DurationMs:          result.DurationMs,
		TimedOut:            result.TimedOut,
		Failed:              !result.Success,
		StdoutTruncated:     result.StdoutTruncated,
		StderrTruncated:     result.StderrTruncated,
		RiskLevel:           result.Preview.RiskLevel,
		ReadOnly:            result.Preview.ReadOnly,
		WorkingDirectory:    result.Preview.WorkingDirectory,
	})
}

func computeArgsHash(args []string) string {
	d := sha256.Sum256([]byte(strings.Join(args, "\n")))
	return hex.EncodeToString(d[:])
}

func (a *ExternalCliTool) executeCommand(ctx context.Context, request core.ExternalCliToolRequest, execContext core.ToolExecutionContext) (*core.ExternalCliExecutionResult, error) {
	prepared, err := a.registry.BuildPreview(toPreviewRequest(&request), false)
	if err != nil {
		return nil, err
	}
	result, err := a.runner.Execute(ctx, prepared)
	if err != nil {
		return nil, err
	}
	action := "command_executed"
	if result.TimedOut {
		action = "command_timed_out"
	} else if !result.Success {
		action = "command_failed"
	}
	severity := "warning"
	if result.Success {
		severity = "info"
	}
	a.recordEvent(
		execContext,
		action,
		severity,
		fmt.Sprintf("External CLI command %s/%s completed with exit code %d.", prepared.ConnectorName, prepared.CommandName, result.ExitCode),
		request,
		prepared.Preview)
	a.recordAudit(execContext, *result, request)

	return result, err
}

func (a *ExternalCliTool) preview(ctx context.Context, request core.ExternalCliToolRequest, context core.ToolExecutionContext) (*core.ExternalCliPreviewResponse, error) {
	prepared, err := a.registry.BuildPreview(toPreviewRequest(&request), request.ExecuteDryRun)
	if err != nil {
		return nil, err
	}

	action := "previewed"
	if request.ExecuteDryRun {
		action = "dry_run_previewed"
	}
	a.recordEvent(context, action, "info", fmt.Sprintf("Previewed external CLI command %s/%s.", prepared.ConnectorName, prepared.CommandName), request, prepared.Preview)

	var dryRunResult *core.ExternalCliExecutionResult
	if request.ExecuteDryRun {
		var err error
		dryRunResult, err = a.runner.Execute(ctx, prepared)
		if err != nil {
			return nil, err
		}

		severity := "info"
		if !dryRunResult.Success {
			severity = "warning"
		}
		a.recordEvent(context, "dry_run_executed", severity, fmt.Sprintf("Dry-run executed for external CLI command %s/%s.", prepared.ConnectorName, prepared.CommandName), request, prepared.Preview)
		a.recordAudit(context, *dryRunResult, request)
	}

	return &core.ExternalCliPreviewResponse{
		Preview:      prepared.Preview,
		DryRunResult: dryRunResult,
	}, nil
}

func (a *ExternalCliTool) ResolveActionDescriptor(argumentsJson string) core.ToolActionDescriptor {
	var request core.ExternalCliToolRequest
	if err := json.Unmarshal([]byte(argumentsJson), &request); err != nil {
		return core.ToolActionPolicyResolverInstance.Resolve(a.Name(), argumentsJson)
	}

	if request.Action != "execute" && request.Action != "preview" {
		return core.ToolActionDescriptor{
			Action:  request.Action,
			Summary: buildSummary(request),
		}
	}

	dryRun := request.Action == "preview" && request.ExecuteDryRun

	prepared, err := a.registry.BuildPreview(toPreviewRequest(&request), dryRun)
	if err != nil {
		return core.ToolActionPolicyResolverInstance.Resolve(a.Name(), argumentsJson)
	}

	return core.ToolActionDescriptor{
		Action:              request.Action,
		IsMutation:          !prepared.Preview.ReadOnly,
		RequiresApproval:    prepared.Preview.RequiresApproval,
		Summary:             buildSummary(request),
		ApprovalFingerprint: prepared.Preview.Fingerprint,
		RiskLevel:           prepared.Preview.RiskLevel,
		ReadOnly:            &prepared.Preview.ReadOnly,
	}
}

func (a *ExternalCliTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	var request core.ExternalCliToolRequest
	if err := json.Unmarshal([]byte(argumentsJson), &request); err != nil {
		return fmt.Sprintf("Error: Invalid external_cli arguments: %v", err)
	}

	var response any
	var err error
	switch request.Action {
	case "list_connectors":
		summary, err1 := a.registry.ListConnectors()
		if err1 != nil {
			return fmt.Sprintf("Error: ListConnectors: %v", err)
		}

		response = core.ExternalCliConnectorListResponse{
			Items: summary,
		}
		err = err1
	case "connector_status":
		connector := strings.TrimSpace(request.Connector)
		if connector == "" {
			return "request.Connector MUST have 'connector'"
		}
		response, err = a.registry.GetStatus(ctx, connector)
	case "list_commands":
		connector := strings.TrimSpace(request.Connector)
		if connector == "" {
			return "request.Connector MUST have 'connector'"
		}
		response, err = a.registry.ListCommands(connector)
	case "command_schema":
		connector := strings.TrimSpace(request.Connector)
		if connector == "" {
			return "request.Connector MUST have 'connector'"
		}
		command := strings.TrimSpace(request.Command)
		if command == "" {
			return "request.Command MUST have 'command'"
		}
		response, err = a.registry.GetCommandSchema(connector, command)
	case "preview":
		response, err = a.preview(ctx, request, toolContext)
	case "execute":
		response, err = a.executeCommand(ctx, request, toolContext)
	default:
		err = errors.New("Error: Unknown action. Valid actions are list_connectors, connector_status, list_commands, command_schema, preview, and execute.")
	}

	if err != nil {
		a.recordEvent(toolContext, "blocked_by_policy", "warning", err.Error(), request, nil)
		return err.Error()
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err.Error()
	}

	return string(data)
}

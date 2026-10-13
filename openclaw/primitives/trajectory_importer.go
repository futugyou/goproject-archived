package primitives

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/futugyou/openclaw/core"
)

const (
	MaxInputCharacters = 8 * 1024 * 1024 // 8 MB
	MaxRecords         = 10_000
)

// imports and validates a trajectory replay fixture from the given reader.
func TrajectoryReplayImport(
	ctx context.Context,
	reader io.Reader,
	sessionId string,
	promptTurnIndex int,
	redaction core.IRedactionPipeline,
) (*TrajectoryReplayFixture, error) {
	if reader == nil {
		return nil, errors.New("reader cannot be null")
	}
	if redaction == nil {
		return nil, errors.New("redaction cannot be null")
	}
	if strings.TrimSpace(sessionId) == "" {
		return nil, errors.New("sessionId cannot be empty or whitespace")
	}
	if promptTurnIndex < 0 {
		return nil, errors.New("promptTurnIndex cannot be negative")
	}

	// Read input with character limit enforcement
	var sb strings.Builder
	buf := make([]byte, 4096)
	totalBytes := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		n, err := reader.Read(buf)
		if n > 0 {
			totalBytes += n
			if totalBytes > MaxInputCharacters {
				return nil, errors.New("trajectory exceeds the import size limit")
			}
			sb.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	var records []core.TrajectoryExportRecord
	scanner := bufio.NewScanner(strings.NewReader(sb.String()))
	recordCount := 0

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		recordCount++
		if recordCount > MaxRecords {
			return nil, errors.New("trajectory has too many records")
		}

		var record core.TrajectoryExportRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, errors.New("invalid trajectory JSON record")
		}

		if record.SchemaVersion != 1 {
			return nil, errors.New("unsupported trajectory schema version")
		}

		if record.SessionId == sessionId && record.Type != "evidence_bundle" && record.Type != "governance_ledger_entry" {
			records = append(records, record)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Find start index
	startIndex := -1
	promptCount := 0
	for idx, r := range records {
		if r.Type == "prompt" && r.TurnIndex == promptTurnIndex {
			if startIndex == -1 {
				startIndex = idx
			}
			promptCount++
		}
	}

	if startIndex < 0 {
		return nil, errors.New("selected user prompt was not found")
	}
	if promptCount != 1 {
		return nil, errors.New("selected prompt is ambiguous")
	}

	prompt := records[startIndex]
	if prompt.Role != "user" || prompt.Content == "" {
		return nil, errors.New("replay requires a user text prompt")
	}

	fixture := &TrajectoryReplayFixture{
		Prompt: redaction.Redact(prompt.Content),
	}

	var response *ReplayResponse
	turnIndex := promptTurnIndex

	for i := startIndex + 1; i < len(records); i++ {
		record := records[i]
		if record.Type == "prompt" {
			break
		}

		switch record.Type {
		case "response":
			if record.Role != "assistant" || record.TurnIndex <= turnIndex || record.Content == "" {
				return nil, errors.New("response order or role is invalid")
			}
			if response != nil && len(response.ToolCalls) == 0 {
				return nil, errors.New("multiple final responses are not supported")
			}
			turnIndex = record.TurnIndex
			response = &ReplayResponse{
				Text: redaction.Redact(record.Content),
			}
			fixture.Responses = append(fixture.Responses, *response)
		case "tool_call":
			if response == nil || record.TurnIndex != turnIndex || i+1 >= len(records) {
				return nil, errors.New("tool call is missing its response or result")
			}

			i++
			result := records[i]
			if result.Type != "tool_result" || result.TurnIndex != turnIndex || result.CallId != record.CallId || result.ToolName != record.ToolName || result.Result == "" {
				return nil, errors.New("tool result does not match its call")
			}

			if (result.ResultStatus != "completed" && result.ResultStatus != "failed" && result.ResultStatus != "blocked") ||
				((result.ResultStatus == "completed") && (result.FailureCode != "" || result.FailureMessage != "")) {
				return nil, errors.New("unsupported or inconsistent tool outcome")
			}

			if record.ToolName == "" || len(record.ToolName) > 128 {
				return nil, errors.New("invalid replay tool name")
			}
			for _, c := range record.ToolName {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
					return nil, errors.New("invalid replay tool name")
				}
			}

			var jsonObj map[string]any
			argsStr := record.Arguments
			if err := json.Unmarshal([]byte(argsStr), &jsonObj); err != nil {
				return nil, errors.New("tool arguments must be a valid JSON object")
			}

			// Simulating WriteRedactedJson/RedactPayload via TrajectorySanitizer
			var jsonWriter strings.Builder
			encoder := json.NewEncoder(&jsonWriter)
			err := core.TrajectoryWriteRedactedJson(encoder, jsonObj, redaction)
			if err != nil {
				return nil, err
			}
			args := jsonWriter.String()

			resultStatus := "completed"
			if result.ResultStatus != "" {
				resultStatus = result.ResultStatus
			}

			var failureCode string
			if result.FailureCode != "" {
				failureCode = core.TrajectoryRedact(result.FailureCode, redaction)
			}

			var failureMessage string
			if result.FailureMessage != "" {
				failureMessage = core.TrajectoryRedact(result.FailureMessage, redaction)
			}

			response.ToolCalls = append(response.ToolCalls, ReplayToolCall{
				ToolName:       record.ToolName,
				ArgumentsJson:  args,
				Result:         core.TrajectoryRedactPayload(result.Result, redaction),
				ResultStatus:   resultStatus,
				FailureCode:    failureCode,
				FailureMessage: failureMessage,
			})
		default:
			return nil, errors.New("unexpected trajectory record in selected exchange")
		}
	}

	if len(fixture.Responses) == 0 || len(fixture.Responses[len(fixture.Responses)-1].ToolCalls) != 0 {
		return nil, errors.New("selected exchange is incomplete: a final assistant response is required")
	}

	for i, item := range fixture.Responses {
		if len(item.ToolCalls) > 0 && item.Text == "[tool_use]" {
			fixture.Responses[i] = ReplayResponse{
				ToolCalls: item.ToolCalls,
			}
		}
	}

	if _, err := NewTrajectoryReplay(*fixture); err != nil {
		return nil, err
	}

	return fixture, nil
}

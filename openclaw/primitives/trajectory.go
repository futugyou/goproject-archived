package primitives

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sync"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/openclaw/agent"
	"github.com/futugyou/openclaw/core"
)

type TrajectoryReplayFixture struct {
	SchemaVersion int
	Prompt        string
	Responses     []ReplayResponse
}

func NewTrajectoryReplayFixture() *TrajectoryReplayFixture {
	return &TrajectoryReplayFixture{
		SchemaVersion: 1,
		Responses:     []ReplayResponse{},
	}
}

type ReplayResponse struct {
	Text      string
	ToolCalls []ReplayToolCall
}

type ReplayToolCall struct {
	ToolName       string
	ArgumentsJson  string
	Result         string
	ResultStatus   string
	FailureCode    string
	FailureMessage string
}

func NewReplayToolCall() *ReplayToolCall {
	return &ReplayToolCall{
		ArgumentsJson: "{}",
		ResultStatus:  "completed",
	}
}

func argumentsEqual(expected, actual string) bool {
	var left, right any

	if err := json.Unmarshal([]byte(expected), &left); err != nil {
		return false
	}

	if err := json.Unmarshal([]byte(actual), &right); err != nil {
		return false
	}

	return reflect.DeepEqual(left, right)
}

type TrajectoryReplay struct {
	Tools        []core.ITool
	fixture      TrajectoryReplayFixture
	pending      []PendingCall
	nextResponse int
	diverged     bool
	issuedIds    map[string]struct{}

	mu sync.Mutex
}

func NewTrajectoryReplay(fixture TrajectoryReplayFixture) (*TrajectoryReplay, error) {
	re := &TrajectoryReplay{fixture: fixture}
	if fixture.SchemaVersion != 1 || fixture.Prompt == "" || len(fixture.Responses) == 0 {
		return nil, errors.New("Invalid replay fixture schema or input.")
	}

	medias, _ := core.MediaMarkerExtract(fixture.Prompt)
	for _, media := range medias {
		if media.Kind == core.MediaMarkerFilePath || media.Kind == core.MediaMarkerImagePath {
			return nil, errors.New("Replay media must use HTTP(S) references; no local files, inline binary, or unresolved channel IDs.")
		}
		baseURL, err := url.Parse(media.Value)
		if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
			return nil, errors.New("Replay media must use HTTP(S) references; no local files, inline binary, or unresolved channel IDs.")
		}
	}

	for _, response := range fixture.Responses {
		if response.Text == "" || len(response.ToolCalls) == 0 {
			return nil, errors.New("Invalid replay response.")
		}
		for _, call := range response.ToolCalls {
			if call.ToolName == "" || call.Result == "" {
				return nil, errors.New("Invalid replay tool call.")
			}
			if (call.ResultStatus != "completed" && call.ResultStatus != "failed" && call.ResultStatus != "blocked") ||
				(call.ResultStatus == "completed" && (call.FailureCode != "" || call.FailureMessage != "")) {
				return nil, errors.New("Invalid replay outcome.")
			}

			if _, err := parseArguments(call.ArgumentsJson); err != nil {
				return nil, err
			}
			count := 0
			for _, other := range response.ToolCalls {
				if other.ToolName == call.ToolName && argumentsEqual(other.ArgumentsJson, call.ArgumentsJson) {
					count++
				}
			}
			if count > 1 {
				return nil, errors.New("Identical calls within a batch are ambiguous; split the fixture into ordered batches.")
			}
		}
	}

	lastResponse := fixture.Responses[len(fixture.Responses)-1]
	if len(lastResponse.ToolCalls) != 0 {
		return nil, errors.New("A replay needs tool batches followed by one final response.")
	}

	for i := 0; i < len(fixture.Responses)-1; i++ {
		if len(fixture.Responses[i].ToolCalls) == 0 {
			return nil, errors.New("A replay needs tool batches followed by one final response.")
		}
	}

	seen := make(map[string]bool)
	var uniqueNames []string

	for _, r := range fixture.Responses {
		for _, call := range r.ToolCalls {
			if !seen[call.ToolName] {
				seen[call.ToolName] = true
				uniqueNames = append(uniqueNames, call.ToolName)
			}
		}
	}

	re.Tools = make([]core.ITool, len(uniqueNames))
	for i, name := range uniqueNames {
		re.Tools[i] = NewReplayTool(re, name)
	}
	return re, nil
}

func (t *TrajectoryReplay) execute(ctx context.Context, name string, argumentsJson string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var match *PendingCall
	for _, call := range t.pending {
		if !call.Consumed && call.Call.ToolName == name && argumentsEqual(call.Call.ArgumentsJson, argumentsJson) {
			match = &call
		}
	}

	if match == nil {
		return "", t.divergence("Tool name, arguments, or execution count differs from the fixture.")
	}

	match.Consumed = true
	if match.Call.ResultStatus != "completed" {
		return "", &agent.ToolOutcomeError{
			ResultText:   match.Call.Result,
			ResultStatus: match.Call.ResultStatus,
			FailureCode:  match.Call.FailureCode,
			Message:      match.Call.FailureMessage,
		}
	}

	return match.Call.Result, nil

}

func (t *TrajectoryReplay) divergence(message string) error {
	t.diverged = true
	return errors.New(message)
}

// GetResponse implements [chatcompletion.IChatClient].
func (t *TrajectoryReplay) GetResponse(ctx context.Context, chatMessages []chatcompletion.ChatMessage, options *chatcompletion.ChatOptions) (*chatcompletion.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	history := chatMessages

	if t.nextResponse == 0 {
		var lastUserMsg *chatcompletion.ChatMessage
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == chatcompletion.RoleUser {
				lastUserMsg = &history[i]
				break
			}
		}
		if lastUserMsg != nil && !t.promptMatches(lastUserMsg) {
			return nil, t.divergence("Replay prompt differs from the recorded prompt.")
		}
	}

	for _, call := range t.pending {
		if !call.Consumed {
			return nil, t.divergence("Runtime skipped a recorded tool call.")
		}
	}

	var results []contents.FunctionResultContent
	for _, m := range history {
		if m.Role == chatcompletion.RoleTool {
			for _, content := range m.Contents {
				if funcRes, ok := content.(*contents.FunctionResultContent); ok {
					results = append(results, *funcRes)
				}
			}
		}
	}

	for _, result := range results {
		found := false
		for id := range t.issuedIds {
			if id == result.CallId {
				found = true
				break
			}
		}
		if !found {
			return nil, t.divergence("Runtime returned a result for an unknown call ID.")
		}
	}

	for _, call := range t.pending {
		count := 0
		matchFound := false
		for _, result := range results {
			if result.CallId == call.Id {
				count++
				if result.Result != nil && fmt.Sprintf("%v", result.Result) == call.Call.Result {
					matchFound = true
				}
			}
		}
		if count != 1 || !matchFound {
			return nil, t.divergence("Runtime did not return the recorded tool result to the provider.")
		}
	}

	if t.nextResponse >= len(t.fixture.Responses) {
		return nil, t.divergence("Runtime requested an unrecorded provider response.")
	}

	response := t.fixture.Responses[t.nextResponse]
	t.nextResponse++
	t.pending = nil

	var content []contents.IAIContent

	if len(response.Text) > 0 {
		content = append(content, contents.TextContent{Text: response.Text})
	}

	for i, call := range response.ToolCalls {
		id := fmt.Sprintf("replay_%d_%d", t.nextResponse, i)
		t.issuedIds[id] = struct{}{}
		t.pending = append(t.pending, PendingCall{Id: id, Call: call, Consumed: false})

		values, err := parseArguments(call.ArgumentsJson)
		if err != nil {
			return nil, err
		}

		content = append(content, contents.FunctionCallContent{
			CallId:    id,
			Name:      call.ToolName,
			Arguments: values,
		})
	}

	return &chatcompletion.ChatResponse{
		Messages: []chatcompletion.ChatMessage{{
			Role:     chatcompletion.RoleAssistant,
			Contents: content,
		}},
	}, nil
}

// GetStreamingResponse implements [chatcompletion.IChatClient].
func (t *TrajectoryReplay) GetStreamingResponse(ctx context.Context, chatMessages []chatcompletion.ChatMessage, options *chatcompletion.ChatOptions) <-chan chatcompletion.ChatStreamingResponse {
	response := make(chan chatcompletion.ChatStreamingResponse)

	go func() {
		defer close(response)

		result, err := t.GetResponse(ctx, chatMessages, options)
		if err != nil {
			response <- chatcompletion.ChatStreamingResponse{
				Update: nil,
				Err:    err,
			}
			return
		}

		for _, message := range result.Messages {
			select {
			case <-ctx.Done():
				response <- chatcompletion.ChatStreamingResponse{
					Err: ctx.Err(),
				}
				return
			default:
			}

			response <- chatcompletion.ChatStreamingResponse{
				Update: &chatcompletion.ChatResponseUpdate{
					Role:     &message.Role,
					Contents: message.Contents,
				},
				Err: nil,
			}
		}
	}()

	return response
}

func parseArguments(jsonstring string) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(jsonstring), &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *TrajectoryReplay) promptMatches(message *chatcompletion.ChatMessage) bool {
	if message == nil {
		return false
	}

	markers, text := core.MediaMarkerExtract(t.fixture.Prompt)
	if len(markers) == 0 {
		te := text
		if te == "" {
			te = t.fixture.Prompt
		}

		allTextContent := true
		for _, v := range message.Contents {
			if _, ok := v.(*contents.TextContent); !ok {
				allTextContent = false
				break
			}
		}
		return message.Text() == te && allTextContent
	}

	uris := []*contents.UriContent{}
	for _, v := range message.Contents {
		if a, ok := v.(*contents.UriContent); ok {
			uris = append(uris, a)
		}
	}
	if message.Text() != text || len(uris) != len(markers) {
		return false
	}

	for _, c := range message.Contents {
		switch c.(type) {
		case *contents.TextContent, *contents.UriContent:
		default:
			return false
		}
	}

	for i, marker := range markers {
		parsedUri, err := url.Parse(marker.Value)
		if err != nil || uris[i].URI != parsedUri.String() {
			return false
		}

		var expectedMediaType string
		switch marker.Kind {
		case core.MediaMarkerImageUrl:
			expectedMediaType = "image/*"
		case core.MediaMarkerAudioUrl:
			expectedMediaType = "audio/*"
		case core.MediaMarkerVideoUrl:
			expectedMediaType = "video/*"
		default:
			expectedMediaType = "application/octet-stream"
		}

		if uris[i].MediaType != expectedMediaType {
			return false
		}
	}

	return true
}

func (t *TrajectoryReplay) VerifyComplete() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.diverged || t.nextResponse != len(t.fixture.Responses) {
		return errors.New("Replay did not consume the complete fixture without divergence.")
	}

	for _, call := range t.pending {
		if !call.Consumed {
			return errors.New("Replay did not consume the complete fixture without divergence.")
		}
	}

	return nil
}

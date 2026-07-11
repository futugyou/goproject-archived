package core

import (
	"encoding/json"
	"errors"
	"strings"
)

type HealthResponse struct {
	Status string `json:"status"`
	Uptime int64  `json:"uptime"`
}

type OpenAiChatCompletionRequest struct {
	Model       *string          `json:"model,omitempty"`
	Messages    []*OpenAiMessage `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature *float32         `json:"temperature,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
}

func DefaultOpenAiChatCompletionRequest() *OpenAiChatCompletionRequest {
	return &OpenAiChatCompletionRequest{
		Messages: make([]*OpenAiMessage, 0),
	}
}

type OpenAiMessage struct {
	Role    string                `json:"role"`
	Content *OpenAiMessageContent `json:"content"`
}

type OpenAiMessageContent struct {
	Text  *string                     `json:"-"`
	Parts []*OpenAiMessageContentPart `json:"-"`
}

func DefaultOpenAiMessageContent() *OpenAiMessageContent {
	return &OpenAiMessageContent{
		Parts: make([]*OpenAiMessageContentPart, 0),
	}
}

type OpenAiMessageContentPart struct {
	Type     string  `json:"type"`
	Text     *string `json:"text,omitempty"`
	ImageUrl *string `json:"image_url,omitempty"`
}

func DefaultOpenAiMessageContentPart() *OpenAiMessageContentPart {
	return &OpenAiMessageContentPart{
		Type: "",
	}
}

func (c *OpenAiMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) == 0 {
		if c.Text != nil {
			return json.Marshal(*c.Text)
		}
		return json.Marshal("")
	}

	type ImageUrlWrapper struct {
		Url string `json:"url"`
	}
	type AliasPart struct {
		Type     string           `json:"type"`
		Text     string           `json:"text,omitempty"`
		ImageUrl *ImageUrlWrapper `json:"image_url,omitempty"`
	}

	aliasParts := make([]AliasPart, len(c.Parts))
	for i, part := range c.Parts {
		ap := AliasPart{Type: part.Type}
		if part.IsText() {
			if part.Text != nil {
				ap.Text = *part.Text
			}
		} else if part.IsImage() {
			urlStr := ""
			if part.ImageUrl != nil {
				urlStr = *part.ImageUrl
			}
			ap.ImageUrl = &ImageUrlWrapper{Url: urlStr}
		}
		aliasParts[i] = ap
	}
	return json.Marshal(aliasParts)
}

func (c *OpenAiMessageContent) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		empty := ""
		c.Text = &empty
		c.Parts = nil
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Text = &str
		c.Parts = nil
		return nil
	}

	var rawParts []map[string]interface{}
	if err := json.Unmarshal(data, &rawParts); err != nil {
		return errors.New("open_ai_message_content must be a string or an array of content parts")
	}

	c.Parts = make([]*OpenAiMessageContentPart, 0, len(rawParts))
	for _, raw := range rawParts {
		typeStr, _ := raw["type"].(string)

		part := &OpenAiMessageContentPart{Type: typeStr}

		if typeStr == "text" || typeStr == "input_text" {
			if textVal, ok := raw["text"].(string); ok {
				part.Text = &textVal
			}
			c.Parts = append(c.Parts, part)
			continue
		}

		if typeStr == "image_url" || typeStr == "input_image" {
			var imageUrl *string

			if imgUrlProp, ok := raw["image_url"]; ok {
				if strVal, ok := imgUrlProp.(string); ok {
					imageUrl = &strVal
				} else if mapVal, ok := imgUrlProp.(map[string]interface{}); ok {
					if urlVal, ok := mapVal["url"].(string); ok {
						imageUrl = &urlVal
					}
				}
			} else if imgProp, ok := raw["image"].(string); ok {
				imageUrl = &imgProp
			} else if urlProp, ok := raw["url"].(string); ok {
				imageUrl = &urlProp
			}

			part.ImageUrl = imageUrl
			c.Parts = append(c.Parts, part)
		}
	}
	return nil
}

func (p *OpenAiMessageContentPart) IsText() bool {
	return p.Type == "text" || p.Type == "input_text"
}

func (p *OpenAiMessageContentPart) IsImage() bool {
	return p.Type == "image_url" || p.Type == "input_image"
}

func (c *OpenAiMessageContent) ToPromptText() string {
	if len(c.Parts) == 0 {
		if c.Text != nil {
			return *c.Text
		}
		return ""
	}

	var lines []string
	for _, part := range c.Parts {
		if part.IsText() && part.Text != nil && strings.TrimSpace(*part.Text) != "" {
			lines = append(lines, *part.Text)
			continue
		}
		if part.IsImage() && part.ImageUrl != nil && strings.TrimSpace(*part.ImageUrl) != "" {
			lines = append(lines, "[IMAGE_URL:"+*part.ImageUrl+"]")
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type OpenAiChatCompletionResponse struct {
	Id      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []*OpenAiChoice `json:"choices"`
	Usage   *OpenAiUsage    `json:"usage,omitempty"`
}

func DefaultOpenAiChatCompletionResponse() *OpenAiChatCompletionResponse {
	return &OpenAiChatCompletionResponse{
		Object: "chat.completion",
	}
}

type OpenAiChoice struct {
	Index        int                    `json:"index"`
	Message      *OpenAiResponseMessage `json:"message"`
	FinishReason *string                `json:"finish_reason,omitempty"`
}

type OpenAiResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAiStreamChunk struct {
	Id      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []*OpenAiStreamChoice `json:"choices"`
}

func DefaultOpenAiStreamChunk() *OpenAiStreamChunk {
	return &OpenAiStreamChunk{
		Object: "chat.completion.chunk",
	}
}

type OpenAiStreamChoice struct {
	Index        int          `json:"index"`
	Delta        *OpenAiDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason,omitempty"`
}

type OpenAiDelta struct {
	Role       *string                `json:"role,omitempty"`
	Content    *string                `json:"content,omitempty"`
	ToolCalls  []*OpenAiToolCallDelta `json:"tool_calls,omitempty"`
	ToolDelta  *OpenAiToolOutputDelta `json:"openclaw_tool_delta,omitempty"`
	ToolResult *OpenAiToolResultDelta `json:"openclaw_claw_result,omitempty"`
}

type OpenAiToolCallDelta struct {
	Index    int                      `json:"index"`
	Id       *string                  `json:"id,omitempty"`
	Type     string                   `json:"type"`
	Function *OpenAiFunctionCallDelta `json:"function,omitempty"`
}

func DefaultOpenAiToolCallDelta() *OpenAiToolCallDelta {
	return &OpenAiToolCallDelta{
		Type: "function",
	}
}

type OpenAiFunctionCallDelta struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

type OpenAiToolResultDelta struct {
	CallId         *string `json:"call_id,omitempty"`
	ToolName       string  `json:"tool_name"`
	Content        string  `json:"content"`
	ResultStatus   string  `json:"result_status"`
	FailureCode    *string `json:"failure_code,omitempty"`
	FailureMessage *string `json:"failure_message,omitempty"`
	NextStep       *string `json:"next_step,omitempty"`
}

func DefaultOpenAiToolResultDelta() *OpenAiToolResultDelta {
	return &OpenAiToolResultDelta{
		ResultStatus: ToolResultStatusesCompleted,
	}
}

type OpenAiToolOutputDelta struct {
	CallId   *string `json:"call_id,omitempty"`
	ToolName string  `json:"tool_name"`
	Content  string  `json:"content"`
}

const (
	OpenAiResponseResponseObjectDefault       = "response"
	OpenAiResponseContentStreamTypeDefault    = "output_text"
	OpenAiResponseStreamResponseObjectDefault = "response"
	OpenAiResponseCreatedEventTypeDefault     = "response.created"
	OpenAiResponseInProgressEventTypeDefault  = "response.in_progress"
	OpenAiResponseCompletedEventTypeDefault   = "response.completed"
	OpenAiResponseFailedEventTypeDefault      = "response.failed"
	OpenAiResponseOutputStructTypeDefault     = "message"
)

type OpenAiResponseRequest struct {
	Model           *string  `json:"model,omitempty"`
	Input           *string  `json:"input,omitempty"` // String prompt or structured messages.
	Stream          bool     `json:"stream"`
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`
}

type OpenAiResponseResponse struct {
	Id        string                 `json:"id"`
	Object    string                 `json:"object"`
	CreatedAt *int64                 `json:"created_at,omitempty"`
	Model     *string                `json:"model,omitempty"`
	Status    string                 `json:"status"`
	Output    []OpenAiResponseOutput `json:"output"`
	Usage     *OpenAiUsage           `json:"usage,omitempty"`
	Error     *OpenAiResponseError   `json:"error,omitempty"`
}

type OpenAiResponseOutput struct {
	Id         string                  `json:"id"`
	Type       string                  `json:"type"`
	Status     *string                 `json:"status,omitempty"`
	Role       *string                 `json:"role,omitempty"`
	Content    []OpenAiResponseContent `json:"content,omitempty"`
	CallId     *string                 `json:"call_id,omitempty"`
	Name       *string                 `json:"name,omitempty"`
	Arguments  *string                 `json:"arguments,omitempty"`
	OutputText *string                 `json:"output,omitempty"`
}

type OpenAiResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type OpenAiResponseStreamResponse struct {
	Id        string                     `json:"id"`
	Object    string                     `json:"object"`
	CreatedAt int64                      `json:"created_at"`
	Model     string                     `json:"model"`
	Status    string                     `json:"status"`
	Output    []OpenAiResponseStreamItem `json:"output"`
	Usage     *OpenAiUsage               `json:"usage,omitempty"`
	Error     *OpenAiResponseError       `json:"error,omitempty"`
}

type OpenAiResponseStreamItem struct {
	Id        string                  `json:"id"`
	Type      string                  `json:"type"`
	Status    *string                 `json:"status,omitempty"`
	Role      *string                 `json:"role,omitempty"`
	Content   []OpenAiResponseContent `json:"content,omitempty"`
	CallId    *string                 `json:"call_id,omitempty"`
	Name      *string                 `json:"name,omitempty"`
	Arguments *string                 `json:"arguments,omitempty"`
	Output    *string                 `json:"output,omitempty"`
}

type OpenAiResponseCreatedEvent struct {
	Type           string                       `json:"type"`
	SequenceNumber int                          `json:"sequence_number"`
	Response       OpenAiResponseStreamResponse `json:"response"`
}

type OpenAiResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OpenAiResponseInProgressEvent struct {
	Type           string                       `json:"type"`
	SequenceNumber int                          `json:"sequence_number"`
	Response       OpenAiResponseStreamResponse `json:"response"`
}

type OpenAiResponseCompletedEvent struct {
	Type           string                       `json:"type"`
	SequenceNumber int                          `json:"sequence_number"`
	Response       OpenAiResponseStreamResponse `json:"response"`
}

type OpenAiResponseFailedEvent struct {
	Type           string                       `json:"type"`
	SequenceNumber int                          `json:"sequence_number"`
	Response       OpenAiResponseStreamResponse `json:"response"`
}

func NewDefaultOpenAiResponseResponse() OpenAiResponseResponse {
	return OpenAiResponseResponse{
		Object: OpenAiResponseResponseObjectDefault,
	}
}

func NewDefaultOpenAiResponseOutput() OpenAiResponseOutput {
	return OpenAiResponseOutput{
		Type: OpenAiResponseOutputStructTypeDefault,
	}
}

func NewDefaultOpenAiResponseContent() OpenAiResponseContent {
	return OpenAiResponseContent{
		Type: OpenAiResponseContentStreamTypeDefault,
	}
}

func NewDefaultOpenAiResponseStreamResponse() OpenAiResponseStreamResponse {
	return OpenAiResponseStreamResponse{
		Object: OpenAiResponseStreamResponseObjectDefault,
	}
}

func NewDefaultOpenAiResponseCreatedEvent() OpenAiResponseCreatedEvent {
	return OpenAiResponseCreatedEvent{
		Type: OpenAiResponseCreatedEventTypeDefault,
	}
}

func NewDefaultOpenAiResponseInProgressEvent() OpenAiResponseInProgressEvent {
	return OpenAiResponseInProgressEvent{
		Type: OpenAiResponseInProgressEventTypeDefault,
	}
}

func NewDefaultOpenAiResponseCompletedEvent() OpenAiResponseCompletedEvent {
	return OpenAiResponseCompletedEvent{
		Type: OpenAiResponseCompletedEventTypeDefault,
	}
}

func NewDefaultOpenAiResponseFailedEvent() OpenAiResponseFailedEvent {
	return OpenAiResponseFailedEvent{
		Type: OpenAiResponseFailedEventTypeDefault,
	}
}

const (
	ToolResultStatusesCompleted = "Completed"
)

type BaseEvent struct {
	SequenceNumber int    `json:"sequence_number"`
	ResponseId     string `json:"response_id"`
	OutputIndex    int    `json:"output_index"`
}

type OpenAiResponseOutputItemAddedEvent struct {
	BaseEvent
	Type string                   `json:"type"`
	Item OpenAiResponseStreamItem `json:"item"`
}

func DefaultOpenAiResponseOutputItemAddedEvent() *OpenAiResponseOutputItemAddedEvent {
	return &OpenAiResponseOutputItemAddedEvent{
		Type: "response.output_item.added",
	}
}

type OpenAiResponseOutputItemDoneEvent struct {
	BaseEvent
	Type string                   `json:"type"`
	Item OpenAiResponseStreamItem `json:"item"`
}

func DefaultOpenAiResponseOutputItemDoneEvent() *OpenAiResponseOutputItemDoneEvent {
	return &OpenAiResponseOutputItemDoneEvent{
		Type: "response.output_item.done",
	}
}

type OpenAiResponseContentPartAddedEvent struct {
	BaseEvent
	Type         string                `json:"type"`
	ItemId       string                `json:"item_id"`
	ContentIndex int                   `json:"content_index"`
	Part         OpenAiResponseContent `json:"part"`
}

func DefaultOpenAiResponseContentPartAddedEvent() *OpenAiResponseContentPartAddedEvent {
	return &OpenAiResponseContentPartAddedEvent{
		Type: "response.content_part.added",
	}
}

type OpenAiResponseContentPartDoneEvent struct {
	BaseEvent
	Type         string                `json:"type"`
	ItemId       string                `json:"item_id"`
	ContentIndex int                   `json:"content_index"`
	Part         OpenAiResponseContent `json:"part"`
}

func DefaultOpenAiResponseContentPartDoneEvent() *OpenAiResponseContentPartDoneEvent {
	return &OpenAiResponseContentPartDoneEvent{
		Type: "response.content_part.done",
	}
}

type OpenAiResponseOutputTextDeltaEvent struct {
	BaseEvent
	Type         string `json:"type"`
	ItemId       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

func DefaultOpenAiResponseOutputTextDeltaEvent() *OpenAiResponseOutputTextDeltaEvent {
	return &OpenAiResponseOutputTextDeltaEvent{
		Type: "response.output_text.delta",
	}
}

type OpenAiResponseOutputTextDoneEvent struct {
	BaseEvent
	Type         string `json:"type"`
	ItemId       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text"`
}

func DefaultOpenAiResponseOutputTextDoneEvent() *OpenAiResponseOutputTextDoneEvent {
	return &OpenAiResponseOutputTextDoneEvent{
		Type: "response.output_text.done",
	}
}

type OpenAiResponseFunctionCallArgumentsDeltaEvent struct {
	BaseEvent
	Type   string `json:"type"`
	ItemId string `json:"item_id"`
	Delta  string `json:"delta"`
}

func DefaultOpenAiResponseFunctionCallArgumentsDeltaEvent() *OpenAiResponseFunctionCallArgumentsDeltaEvent {
	return &OpenAiResponseFunctionCallArgumentsDeltaEvent{
		Type: "response.function_call_arguments.delta",
	}
}

type OpenAiResponseFunctionCallArgumentsDoneEvent struct {
	BaseEvent
	Type      string  `json:"type"`
	ItemId    string  `json:"item_id"`
	CallId    *string `json:"call_id,omitempty"`
	Name      *string `json:"name,omitempty"`
	Arguments string  `json:"arguments"`
}

func DefaultOpenAiResponseFunctionCallArgumentsDoneEvent() *OpenAiResponseFunctionCallArgumentsDoneEvent {
	return &OpenAiResponseFunctionCallArgumentsDoneEvent{
		Type: "response.function_call_arguments.done",
	}
}

type OpenAiResponseToolOutputDeltaEvent struct {
	BaseEvent
	Type     string  `json:"type"`
	ItemId   string  `json:"item_id"`
	CallId   *string `json:"call_id,omitempty"`
	ToolName string  `json:"tool_name"`
	Delta    string  `json:"delta"`
}

func DefaultOpenAiResponseToolOutputDeltaEvent() *OpenAiResponseToolOutputDeltaEvent {
	return &OpenAiResponseToolOutputDeltaEvent{
		Type: "response.openclaw_tool_delta",
	}
}

type OpenAiResponseToolResultEvent struct {
	BaseEvent
	Type           string  `json:"type"`
	ItemId         string  `json:"item_id"`
	CallId         *string `json:"call_id,omitempty"`
	ToolName       string  `json:"tool_name"`
	Content        string  `json:"content"`
	ResultStatus   string  `json:"result_status"`
	FailureCode    *string `json:"failure_code,omitempty"`
	FailureMessage *string `json:"failure_message,omitempty"`
	NextStep       *string `json:"next_step,omitempty"`
}

func DefaultOpenAiResponseToolResultEvent() *OpenAiResponseToolResultEvent {
	return &OpenAiResponseToolResultEvent{
		Type:         "response.openclaw_tool_result",
		ResultStatus: ToolResultStatusesCompleted,
	}
}

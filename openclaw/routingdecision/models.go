package routingdecision

import (
	"context"
	"encoding/json"
)

type IDecisionClient interface {
	Evaluate(ctx context.Context, request *DecisionRequest) (*DecisionResponse, error)
}

type DecisionRequest struct {
	Model         string                      `json:"model"`
	RubricVersion string                      `json:"-"`
	State         json.RawMessage             `json:"state"`
	Questions     map[string]DecisionQuestion `json:"questions"`
}

type DecisionQuestion struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

type DecisionResponse struct {
	Model    string                    `json:"model"`
	Answers  map[string]DecisionAnswer `json:"answers"`
	Usage    DecisionUsage             `json:"usage"`
	Metadata *DecisionMetadata         `json:"metadata,omitempty"`
}

type DecisionMetadata struct {
	Checkpoint    string `json:"checkpoint"`
	Revision      string `json:"revision"`
	CalibrationId string `json:"calibrationId"`
	SchemaHash    string `json:"schemaHash"`
	RubricVersion string `json:"rubricVersion"`
	Device        string `json:"device"`
	SdkVersion    string `json:"sdkVersion"`
	Truncated     bool   `json:"truncated"`
}

// decisionRubric 内部使用的 Evaluation 评分规则 (内部 Record 对应未导出结构体)
type decisionRubric struct {
	RubricVersion string                      `json:"rubricVersion"`
	Questions     map[string]DecisionQuestion `json:"questions"`
}

// layaWireRequest 内部使用的 Wire 报文结构
type layaWireRequest struct {
	Model         string                      `json:"model"`
	State         json.RawMessage             `json:"state"`
	Questions     map[string]DecisionQuestion `json:"questions"`
	RubricVersion string                      `json:"rubricVersion"`
	Language      *string                     `json:"language,omitempty"`
}

// DecisionAnswer 评估回答模型
type DecisionAnswer struct {
	Type          string             `json:"type"`
	Choice        *string            `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

// DecisionUsage Token 使用统计
type DecisionUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

type DecisionError struct {
	Reason string
}

func (e *DecisionError) Error() string {
	return e.Reason
}

func NewDecisionError(reason string) error {
	return &DecisionError{Reason: reason}
}

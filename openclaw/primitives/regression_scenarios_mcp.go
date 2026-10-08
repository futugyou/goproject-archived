package primitives

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/client"
)

type McpInitializeShapeScenario struct {
	BaseScenario
}

func NewMcpInitializeShapeScenario() *McpInitializeShapeScenario {
	s := &McpInitializeShapeScenario{
		BaseScenario: NewBaseScenario("mcp.initialize_shape", "MCP initialize shape", HarnessRegressionCategoryMcp, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *McpInitializeShapeScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	v := "2025-03-26"
	initialize := client.McpInitializeRequest{
		ProtocolVersion: &v,
		ClientInfo:      client.McpClientInfo{Name: "openclaw-harness", Version: "1.0.0"},
	}
	p, _ := json.Marshal(initialize)
	rpc := client.McpJsonRpcRequest{
		Id:     "1",
		Method: "initialize",
		Params: p,
	}

	jsondata, err := json.Marshal(rpc)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var restored client.McpJsonRpcRequest
	if err := json.Unmarshal(jsondata, &restored); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	type ininData struct {
		ClientInfo struct {
			Name string `json:"name"`
		} `json:"clientInfo"`
	}
	var data ininData

	if err := json.Unmarshal([]byte(restored.Params), &data); err != nil {
		return s.Failed("MCP initialize request shape did not serialize correctly.", "", ""), nil
	}

	if restored.Jsonrpc == "2.0" &&
		restored.Method == "initialize" &&
		data.ClientInfo.Name == "openclaw-harness" {
		return s.Passed("MCP initialize request shape serializes without running a gateway.", ""), nil
	}

	return s.Failed("MCP initialize request shape did not serialize correctly.", "", ""), nil
}

package primitives

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type OpenAiCompatRequestShapeScenario struct {
	BaseScenario
}

func NewOpenAiCompatRequestShapeScenario() *OpenAiCompatRequestShapeScenario {
	s := &OpenAiCompatRequestShapeScenario{
		BaseScenario: NewBaseScenario("openai_compat.request_shape", "OpenAI-compatible request shape", HarnessRegressionCategoryOpenAiCompat, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *OpenAiCompatRequestShapeScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	jsonstring := ` 
            {
              "model": "gpt-4o-mini",
              "messages": [{ "role": "user", "content": "ping" }],
              "stream": false,
              "max_tokens": 32
            }
            `
	var request core.OpenAiChatCompletionRequest
	err := json.Unmarshal([]byte(jsonstring), &request)
	if err != nil || len(request.Messages) != 1 || request.Messages[0].Content.ToPromptText() != "ping" {
		return s.Failed("OpenAI-compatible chat completion request did not parse.", "", ""), err
	}

	serialized, err := json.Marshal(request)
	if err != nil || !strings.Contains(string(serialized), "\"max_tokens\"") {
		return s.Failed("OpenAI-compatible request serialization lost max_tokens shape.", "", ""), nil
	}

	return s.Passed("OpenAI-compatible request shape parses and serializes without a provider.", ""), nil
}

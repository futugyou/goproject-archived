package routingdecision

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

type DecisionException struct {
	Code string
}

func (e *DecisionException) Error() string {
	return e.Code
}

func SendAndValidateDecision(client *http.Client, req *http.Request, request *DecisionRequest) (*DecisionResponse, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &DecisionException{Code: fmt.Sprintf("http_%d", resp.StatusCode)}
	}

	const maxResponseBytes int64 = 256 * 1024

	// Bounding ContentLength header if present
	if resp.ContentLength > maxResponseBytes {
		return nil, &DecisionException{Code: "response_too_large"}
	}

	// Bound chunked/streamed response reader to maxResponseBytes + 1 byte to detect overflow
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	if int64(len(bodyBytes)) > maxResponseBytes {
		return nil, &DecisionException{Code: "response_too_large"}
	}

	var result DecisionResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, &DecisionException{Code: "invalid_response"}
	}

	// Place your DecisionRequest reference here when invoking
	if err := validate(request, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func validate(req *DecisionRequest, resp *DecisionResponse) error {
	if strings.TrimSpace(resp.Model) == "" || len(resp.Model) > 128 ||
		resp.Answers == nil ||
		resp.Usage.InputTokens < 0 || resp.Usage.OutputTokens < 0 ||
		len(resp.Answers) != len(req.Questions) {
		return &DecisionException{Code: "invalid_response"}
	}

	for id, question := range req.Questions {
		answer, exists := resp.Answers[id]
		if !exists || answer.Type != question.Type {
			return &DecisionException{Code: "invalid_answer"}
		}

		if question.Type == "noul" {
			if !isProbability(answer.Noul) {
				return &DecisionException{Code: "invalid_answer"}
			}
			continue
		}

		if !isProbability(answer.Confidence) || len(answer.Probabilities) == 0 {
			return &DecisionException{Code: "invalid_probabilities"}
		}

		var probSum float64
		for _, val := range answer.Probabilities {
			v := val
			if !isProbability(&v) {
				return &DecisionException{Code: "invalid_probabilities"}
			}
			probSum += val
		}

		if math.Abs(probSum-1.0) > 0.001 {
			return &DecisionException{Code: "invalid_probabilities"}
		}

		switch question.Type {
		case "choice":
			var criteria map[string]json.RawMessage
			if err := json.Unmarshal(question.Criteria, &criteria); err != nil || criteria == nil {
				return &DecisionException{Code: "invalid_choice"}
			}

			if len(answer.Probabilities) != len(criteria) {
				return &DecisionException{Code: "invalid_choice"}
			}

			for key := range answer.Probabilities {
				if _, ok := criteria[key]; !ok {
					return &DecisionException{Code: "invalid_choice"}
				}
			}

			if answer.Choice == nil {
				return &DecisionException{Code: "invalid_choice"}
			}

			chosenProb, exists := answer.Probabilities[*answer.Choice]
			if !exists {
				return &DecisionException{Code: "invalid_choice"}
			}

			maxProb := -1.0
			for _, val := range answer.Probabilities {
				if val > maxProb {
					maxProb = val
				}
			}

			if chosenProb < maxProb {
				return &DecisionException{Code: "invalid_choice"}
			}

		case "score":
			var criteriaList []json.RawMessage
			levels := 0
			if err := json.Unmarshal(question.Criteria, &criteriaList); err == nil {
				levels = len(criteriaList)
			}

			if levels < 2 || levels > 10 || answer.Score == nil || math.IsNaN(*answer.Score) || math.IsInf(*answer.Score, 0) {
				return &DecisionException{Code: "invalid_score"}
			}

			score := *answer.Score
			if score < 0 || score > float64(levels-1) || len(answer.Probabilities) != levels ||
				answer.Legend == nil || len(answer.Legend) != levels {
				return &DecisionException{Code: "invalid_score"}
			}

			for key := range answer.Probabilities {
				if _, ok := answer.Legend[key]; !ok {
					return &DecisionException{Code: "invalid_score"}
				}
			}

			for i := 0; i < levels; i++ {
				key := strconv.Itoa(i)
				if _, ok := answer.Probabilities[key]; !ok {
					return &DecisionException{Code: "invalid_score"}
				}
			}
		default:
			return &DecisionException{Code: "unsupported_question"}
		}
	}

	return nil
}

func isProbability(val *float64) bool {
	return val != nil && *val >= 0.0 && *val <= 1.0
}

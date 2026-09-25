package routingdecision

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

var checkpoints []string = []string{"english", "multilingual", "typed-decisions"}

var devices []string = []string{"cpu", "cuda", "mps"}

type LayaDecisionClient struct {
	httpClient *http.Client
	config     core.DecisionRoutingConfig
	uri        *url.URL
}

func NewLayaDecisionClient(httpClient *http.Client, config core.DecisionRoutingConfig) (*LayaDecisionClient, error) {
	uri, err := core.DecisionRoutingConfigurationValidate(&config)
	if err != nil {
		return nil, err
	}

	return &LayaDecisionClient{
		uri:        uri,
		httpClient: httpClient,
		config:     config,
	}, nil
}

func (l *LayaDecisionClient) Evaluate(ctx context.Context, request *DecisionRequest) (*DecisionResponse, error) {
	statestr, err := Serialize(request.State)
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	if len(statestr) > min(32000, l.config.MaxStateChars) {
		return nil, NewDecisionError("request_too_large")
	}

	questionstr, err := Serialize(request.Questions)
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	req := layaWireRequest{
		Model:         request.Model,
		State:         request.State,
		Questions:     request.Questions,
		RubricVersion: request.RubricVersion,
		Language:      &l.config.Language,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	if len(data) > 65536 {
		return nil, NewDecisionError("request_too_large")
	}

	expectedSchemaHash := util.ComputeTurnHash(questionstr)
	rawrequest, err := http.NewRequestWithContext(ctx, "POST", l.uri.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	rawrequest.Header.Set("Content-Type", "application/json")
	result, err := SendAndValidateDecision(l.httpClient, rawrequest, request)
	if err != nil {
		return nil, NewDecisionError(err.Error())
	}

	var metadata = result.Metadata
	if result.Model != l.config.Model || metadata == nil || metadata.Revision != l.config.Model[5:] ||
		!slices.Contains(checkpoints, metadata.Checkpoint) ||
		metadata.RubricVersion != request.RubricVersion || metadata.SchemaHash != expectedSchemaHash ||
		metadata.SdkVersion != "0.3.4" || !slices.Contains(devices, metadata.Device) {
		return nil, NewDecisionError("laya_metadata_mismatch")
	}

	if metadata.Truncated {
		return nil, NewDecisionError("laya_truncated_input")
	}

	if metadata.CalibrationId != "uncalibrated" && !core.DecisionRoutingConfigurationIsHex(metadata.CalibrationId, 64) {
		return nil, NewDecisionError("laya_calibration_mismatch")
	}

	if l.config.CalibrationId != "" && metadata.CalibrationId != l.config.CalibrationId {
		return nil, NewDecisionError("laya_calibration_mismatch")
	}

	return result, nil
}

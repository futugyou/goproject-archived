package core

import (
	"encoding/json"
	"time"
)

const (
	AgentWorkflowStatuses_Queued          = "queued"
	AgentWorkflowStatuses_Running         = "running"
	AgentWorkflowStatuses_WaitingForInput = "waiting_for_input"
	AgentWorkflowStatuses_Completed       = "completed"
	AgentWorkflowStatuses_Failed          = "failed"
	AgentWorkflowStatuses_Cancelled       = "cancelled"
)

const (
	AgentWorkflowBackendKinds_MafDurableHttp = "maf-durable-http"
)

type WorkflowsConfig struct {
	Enabled  bool                             `json:"enabled"`
	Backends map[string]WorkflowBackendConfig `json:"backends"`
}

type WorkflowBackendConfig struct {
	Enabled             bool   `json:"enabled"`
	Kind                string `json:"kind"`
	DisplayName         string `json:"display_name"`
	BaseUrl             string `json:"base_url"`
	WorkflowName        string `json:"workflow_name"`
	ApiTokenSecret      string `json:"api_token_secret"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
}

func DefaultWorkflowBackendConfig() *WorkflowBackendConfig {
	return &WorkflowBackendConfig{
		Enabled:             true,
		Kind:                AgentWorkflowBackendKinds_MafDurableHttp,
		PollIntervalSeconds: 2,
		TimeoutSeconds:      120,
	}
}

type AgentWorkflowBackendSummary struct {
	Id           string `json:"id"`
	Kind         string `json:"kind"`
	WorkflowName string `json:"workflow_name"`
	DisplayName  string `json:"display_name"`
	Enabled      bool   `json:"enabled"`
}

type AgentWorkflowRequest struct {
	Input     string            `json:"input"`
	Payload   *json.RawMessage  `json:"payload"`
	ChannelId string            `json:"channel_id"`
	SenderId  string            `json:"sender_id"`
	SessionId string            `json:"session_id"`
	Metadata  map[string]string `json:"metadata"`
}

type AgentWorkflowResponse struct {
	PortId   string            `json:"port_id"`
	Payload  *json.RawMessage  `json:"payload"`
	Approved *bool             `json:"approved"`
	Comment  string            `json:"comment"`
	ActorId  string            `json:"actor_id"`
	Metadata map[string]string `json:"metadata"`
}

type AgentWorkflowRunResult struct {
	WorkflowId    string               `json:"workflow_id"`
	RunId         string               `json:"run_id"`
	Status        string               `json:"status"`
	BackendId     string               `json:"backend_id"`
	Output        string               `json:"output"`
	OutputPayload *json.RawMessage     `json:"output_payload"`
	Error         string               `json:"error"`
	Events        []AgentWorkflowEvent `json:"events"`
	Metadata      map[string]string    `json:"metadata"`
}

type AgentWorkflowRunSnapshot struct {
	WorkflowId    string                      `json:"workflow_id"`
	RunId         string                      `json:"run_id"`
	Status        string                      `json:"status"`
	BackendId     string                      `json:"backend_id"`
	Output        string                      `json:"output"`
	OutputPayload *json.RawMessage            `json:"output_payload"`
	Error         string                      `json:"error"`
	PendingInputs []AgentWorkflowPendingInput `json:"pending_inputs"`
	Events        []AgentWorkflowEvent        `json:"events"`
	Metadata      map[string]string           `json:"metadata"`
}

type AgentWorkflowPendingInput struct {
	PortId   string            `json:"port_id"`
	Summary  string            `json:"summary"`
	Payload  *json.RawMessage  `json:"payload"`
	Metadata map[string]string `json:"metadata"`
}

type AgentWorkflowEvent struct {
	Id           string            `json:"id"`
	TimestampUtc time.Time         `json:"timestamp_utc"`
	Type         string            `json:"type"`
	WorkflowId   string            `json:"workflow_id"`
	RunId        string            `json:"run_id"`
	Status       string            `json:"status"`
	PortId       string            `json:"port_id"`
	Summary      string            `json:"summary"`
	Payload      *json.RawMessage  `json:"payload"`
	Metadata     map[string]string `json:"metadata"`
}

type IntegrationWorkflowsResponse struct {
	Items []AgentWorkflowBackendSummary `json:"items"`
}

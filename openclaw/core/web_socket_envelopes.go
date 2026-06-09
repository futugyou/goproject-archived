package core

type WsClientEnvelope struct {
	Type                string   `json:"type"`
	RequestId           *string  `json:"request_id,omitempty"`
	ProtocolVersion     *string  `json:"protocol_version,omitempty"`
	Operation           *string  `json:"operation,omitempty"`
	CatalogId           *string  `json:"catalog_id,omitempty"`
	SupportedCatalogIds []string `json:"supported_catalog_ids,omitempty"`
	Components          []string `json:"components,omitempty"`
	DataModelJson       *string  `json:"data_model_json,omitempty"`
	SurfaceTitle        *string  `json:"surface_title,omitempty"`
	SurfaceKind         *string  `json:"surface_kind,omitempty"`
	ParentSurfaceId     *string  `json:"parent_surface_id,omitempty"`
	Action              *string  `json:"action,omitempty"`
	ParametersJson      *string  `json:"parameters_json,omitempty"`
	SyncMode            *string  `json:"sync_mode,omitempty"`
	DiagnosticCode      *string  `json:"diagnostic_code,omitempty"`
	Text                *string  `json:"text,omitempty"`
	Content             *string  `json:"content,omitempty"`
	SessionId           *string  `json:"session_id,omitempty"`
	MessageId           *string  `json:"message_id,omitempty"`
	ReplyToMessageId    *string  `json:"reply_to_message_id,omitempty"`
	SurfaceId           *string  `json:"surface_id,omitempty"`
	ContentType         *string  `json:"content_type,omitempty"`
	Frames              *string  `json:"frames,omitempty"`
	Html                *string  `json:"html,omitempty"`
	Url                 *string  `json:"url,omitempty"`
	Script              *string  `json:"script,omitempty"`
	SnapshotMode        *string  `json:"snapshot_mode,omitempty"`
	SnapshotJson        *string  `json:"snapshot_json,omitempty"`
	ComponentId         *string  `json:"component_id,omitempty"`
	Event               *string  `json:"event,omitempty"`
	ValueJson           *string  `json:"value_json,omitempty"`
	Sequence            *int64   `json:"sequence,omitempty"`
	Capabilities        []string `json:"capabilities,omitempty"`
	Error               *string  `json:"error,omitempty"`
	Success             *bool    `json:"success,omitempty"`

	ApprovalId *string `json:"approval_id,omitempty"`
	Approved   *bool   `json:"approved,omitempty"`
}

func DefaultWsClientEnvelope() WsClientEnvelope {
	return WsClientEnvelope{
		Type: "client_envelope",
	}
}

type WsServerEnvelope struct {
	Type                string   `json:"type"`
	RequestId           *string  `json:"request_id,omitempty"`
	ProtocolVersion     *string  `json:"protocol_version,omitempty"`
	Operation           *string  `json:"operation,omitempty"`
	CatalogId           *string  `json:"catalog_id,omitempty"`
	SupportedCatalogIds []string `json:"supported_catalog_ids,omitempty"`
	Components          []string `json:"components,omitempty"`
	DataModelJson       *string  `json:"data_model_json,omitempty"`
	SurfaceTitle        *string  `json:"surface_title,omitempty"`
	SurfaceKind         *string  `json:"surface_kind,omitempty"`
	ParentSurfaceId     *string  `json:"parent_surface_id,omitempty"`
	Action              *string  `json:"action,omitempty"`
	ParametersJson      *string  `json:"parameters_json,omitempty"`
	SyncMode            *string  `json:"sync_mode,omitempty"`
	DiagnosticCode      *string  `json:"diagnostic_code,omitempty"`
	Text                *string  `json:"text,omitempty"`
	InReplyToMessageId  *string  `json:"in_reply_to_message_id,omitempty"`
	SessionId           *string  `json:"session_id,omitempty"`
	SurfaceId           *string  `json:"surface_id,omitempty"`
	ContentType         *string  `json:"content_type,omitempty"`
	Frames              *string  `json:"frames,omitempty"`
	Html                *string  `json:"html,omitempty"`
	Url                 *string  `json:"url,omitempty"`
	Script              *string  `json:"script,omitempty"`
	SnapshotMode        *string  `json:"snapshot_mode,omitempty"`
	SnapshotJson        *string  `json:"snapshot_json,omitempty"`
	ComponentId         *string  `json:"component_id,omitempty"`
	Event               *string  `json:"event,omitempty"`
	ValueJson           *string  `json:"value_json,omitempty"`
	Sequence            *int64   `json:"sequence,omitempty"`
	Capabilities        []string `json:"capabilities,omitempty"`
	Error               *string  `json:"error,omitempty"`
	Success             *bool    `json:"success,omitempty"`

	ApprovalId       *string `json:"approval_id,omitempty"`
	ToolName         *string `json:"tool_name,omitempty"`
	ArgumentsPreview *string `json:"arguments_preview,omitempty"`
	Approved         *bool   `json:"approved,omitempty"`
	ResultStatus     *string `json:"result_status,omitempty"`
	FailureCode      *string `json:"failure_code,omitempty"`
	FailureMessage   *string `json:"failure_message,omitempty"`
	NextStep         *string `json:"next_step,omitempty"`
}

func DefaultWsServerEnvelope() WsServerEnvelope {
	return WsServerEnvelope{
		Type: "server_envelope",
	}
}

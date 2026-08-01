package core

import (
	"cmp"
	"slices"
	"strings"
)

const (
	ErrorCodes_ParseError     int = -32700
	ErrorCodes_InvalidRequest int = -32600
	ErrorCodes_MethodNotFound int = -32601
	ErrorCodes_InvalidParams  int = -32602
	ErrorCodes_InternalError  int = -32603
)

const (
	RequestMethods_ToolsList              string = "tools/list"
	RequestMethods_ToolsCall              string = "tools/call"
	RequestMethods_PromptsList            string = "prompts/list"
	RequestMethods_PromptsGet             string = "prompts/get"
	RequestMethods_ResourcesList          string = "resources/list"
	RequestMethods_ResourcesRead          string = "resources/read"
	RequestMethods_ResourcesTemplatesList string = "resources/templates/list"
	RequestMethods_ResourcesSubscribe     string = "resources/subscribe"
	RequestMethods_ResourcesUnsubscribe   string = "resources/unsubscribe"
	RequestMethods_Ping                   string = "ping"
	RequestMethods_CompletionComplete     string = "completion/complete"
	RequestMethods_ElicitationCreate      string = "elicitation/create"
	RequestMethods_Initialize             string = "initialize"
	RequestMethods_ServerDiscover         string = "server/discover"
	RequestMethods_SubscriptionsListen    string = "subscriptions/listen"
)

const (
	MetaKeys_ProtocolVersion    string = "io.modelcontextprotocol/protocolVersion"
	MetaKeys_ClientInfo         string = "io.modelcontextprotocol/clientInfo"
	MetaKeys_ServerInfo         string = "io.modelcontextprotocol/serverInfo"
	MetaKeys_ClientCapabilities string = "io.modelcontextprotocol/clientCapabilities"
	MetaKeys_LogLevel           string = "io.modelcontextprotocol/logLevel"
	MetaKeys_SubscriptionId     string = "io.modelcontextprotocol/subscriptionId"
)

const (
	NotificationMethods_ToolListChangedNotification           string = "notifications/tools/list_changed"
	NotificationMethods_PromptListChangedNotification         string = "notifications/prompts/list_changed"
	NotificationMethods_ResourceListChangedNotification       string = "notifications/resources/list_changed"
	NotificationMethods_ResourceUpdatedNotification           string = "notifications/resources/updated"
	NotificationMethods_ElicitationCompleteNotification       string = "notifications/elicitation/complete"
	NotificationMethods_InitializedNotification               string = "notifications/initialized"
	NotificationMethods_ProgressNotification                  string = "notifications/progress"
	NotificationMethods_CancelledNotification                 string = "notifications/cancelled"
	NotificationMethods_SubscriptionsAcknowledgedNotification string = "notifications/subscriptions/acknowledged"
	NotificationMethods_RootsUpdatedNotification              string = "notifications/roots/list_changed"
	NotificationMethods_LoggingMessageNotification            string = "notifications/message"
)

const (
	July2026ProtocolVersion     = "2026-07-28"
	November2025ProtocolVersion = "2025-11-25"
	June2025ProtocolVersion     = "2025-06-18"
	March2025ProtocolVersion    = "2025-03-26"
	November2024ProtocolVersion = "2024-11-05"
)

var (
	initializeHandshakeProtocolVersions = []string{
		November2024ProtocolVersion,
		March2025ProtocolVersion,
		June2025ProtocolVersion,
		November2025ProtocolVersion,
	}

	perRequestMetadataProtocolVersions = []string{
		July2026ProtocolVersion,
	}

	supportedProtocolVersions = append(
		append([]string{}, initializeHandshakeProtocolVersions...),
		perRequestMetadataProtocolVersions...,
	)
)

func IsJuly2026OrLaterProtocolVersion(protocolVersion string) bool {
	return protocolVersion != "" && strings.Compare(protocolVersion, July2026ProtocolVersion) >= 0
}

func IsSupportedProtocolVersion(protocolVersion string) bool {
	return protocolVersion != "" && slices.Contains(supportedProtocolVersions, protocolVersion)
}

func SupportsInitializeHandshake(protocolVersion string) bool {
	return protocolVersion != "" && slices.Contains(initializeHandshakeProtocolVersions, protocolVersion)
}

func RequiresPerRequestMetadata(protocolVersion string) bool {
	return IsJuly2026OrLaterProtocolVersion(protocolVersion)
}

func RequiresStandardHeaders(protocolVersion string) bool {
	return RequiresPerRequestMetadata(protocolVersion)
}

func SupportsHTTPSessions(protocolVersion string) bool {
	return !RequiresPerRequestMetadata(protocolVersion)
}

func UseInvalidParamsForMissingResource(protocolVersion string) bool {
	return IsJuly2026OrLaterProtocolVersion(protocolVersion)
}

func SupportsPrimingEvent(protocolVersion string) bool {
	if protocolVersion == "" {
		return false
	}

	return cmp.Compare(protocolVersion, November2025ProtocolVersion) >= 0
}

func SupportsNaturalOutputSchemas(protocolVersion string) bool {
	if protocolVersion == "" {
		return false
	}
	return cmp.Compare(protocolVersion, July2026ProtocolVersion) >= 0
}

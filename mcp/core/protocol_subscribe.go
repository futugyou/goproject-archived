package core

type SubscribeRequestParams struct {
	RequestParams `json:",inline"`
	Uri           *string `json:"uri"`
}

type SubscriptionsListenRequestParams struct {
	RequestParams
	Notifications SubscriptionsListenNotifications `json:"notifications"`
}

type SubscriptionsAcknowledgedNotificationParams struct {
	Notifications SubscriptionsListenNotifications `json:"notifications"`
}

type SubscriptionsListenNotifications struct {
	ToolsListChanged      bool     `json:"toolsListChanged"`
	PromptsListChanged    bool     `json:"promptsListChanged"`
	ResourcesListChanged  bool     `json:"resourcesListChanged"`
	ResourceSubscriptions []string `json:"resourceSubscriptions"`
}

type UnsubscribeRequestParams struct {
	RequestParams `json:",inline"`
	Uri           *string `json:"uri"`
}

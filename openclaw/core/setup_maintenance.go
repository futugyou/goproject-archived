package core

type MaintenanceScanInputs struct {
	ConfigPath          string                        `json:"config_path"`
	SetupStatus         *SetupStatusResponse          `json:"setup_status"`
	ModelDoctor         *ModelSelectionDoctorResponse `json:"model_doctor"`
	RecentTurns         []TurnTokenUsageRecord        `json:"recent_turns"`
	ProviderRoutes      []ProviderRouteHealthSnapshot `json:"provider_routes"`
	AutomationRunStates []AutomationRunState          `json:"automation_run_states"`
	RuntimeMetrics      *MetricsSnapshot              `json:"runtime_metrics"`
	LoadedSkills        []SkillDefinition             `json:"loaded_skills"`
	ChannelDriftCount   int                           `json:"channel_drift_count"`
	PluginWarningCount  int                           `json:"plugin_warning_count"`
	PluginErrorCount    int                           `json:"plugin_error_count"`
}

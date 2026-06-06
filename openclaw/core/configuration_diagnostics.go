package core

type ConfigSourceDiagnosticItem struct {
	Label          string `json:"label"`
	Key            string `json:"key"`
	EffectiveValue string `json:"effective_value"`
	Source         string `json:"source"`
	Redacted       bool   `json:"redacted"`
}

type ConfigSourceDiagnostics struct {
	Items []ConfigSourceDiagnosticItem `json:"items"`
}

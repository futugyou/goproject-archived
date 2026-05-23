package graphify

import "github.com/pterm/pterm"

// mock
var ConfigPersistence = struct {
	Load func() *GraphifyConfig
	Save func(*GraphifyConfig)
}{
	Load: func() *GraphifyConfig {
		return &GraphifyConfig{WorkingFolder: ".", OutputFolder: "graphify-out", ExportFormats: "json,html,report"}
	},
	Save: func(c *GraphifyConfig) { pterm.Success.Println("save down") },
}

package core

import (
	"encoding/json"
	"strings"
	"time"
)

type FractalMemoryWorkflow struct {
	Operation    string
	Description  string
	Properties   string
	Required     []string
	AlwaysWrites bool
	WriteFlag    *string
}

func (w FractalMemoryWorkflow) ToolName() string {
	return "fractal_memory_" + w.Operation
}

func (w FractalMemoryWorkflow) McpToolName() string {
	return "memory_" + w.Operation
}

func (w FractalMemoryWorkflow) ParameterSchema() string {
	reqStrs := make([]string, len(w.Required))
	for i, r := range w.Required {
		reqStrs[i] = "\"" + r + "\""
	}
	return "{\"type\":\"object\",\"additionalProperties\":false,\"properties\":" + w.Properties +
		",\"required\":[" + strings.Join(reqStrs, ",") + "]}"
}

func (w FractalMemoryWorkflow) IsMutation(arguments map[string]any) bool {
	if w.AlwaysWrites {
		return true
	}
	if w.WriteFlag != nil {
		if val, ok := arguments[*w.WriteFlag]; ok {
			if b, ok := val.(bool); ok && b {
				return true
			}
		}
	}
	return false
}

func (w FractalMemoryWorkflow) Validate(arguments map[string]any) *string {
	var schema map[string]map[string]any
	if err := json.Unmarshal([]byte(w.Properties), &schema); err != nil {
		msg := "Invalid schema definition."
		return &msg
	}

	for _, name := range w.Required {
		if _, ok := arguments[name]; !ok {
			msg := name + " is required."
			return &msg
		}
	}

	for name, value := range arguments {
		rule, ok := schema[name]
		if !ok {
			msg := "Unknown argument '" + name + "'."
			return &msg
		}

		typ, _ := rule["type"].(string)
		validType := false
		switch typ {
		case "string":
			_, validType = value.(string)
		case "boolean":
			_, validType = value.(bool)
		case "integer":
			if f, ok := value.(float64); ok && f == float64(int(f)) {
				validType = true
			}
		case "array":
			if arr, ok := value.([]any); ok {
				validType = true
				for _, item := range arr {
					if _, ok := item.(string); !ok {
						validType = false
						break
					}
				}
			}
		}

		if !validType {
			msg := "Invalid type for '" + name + "'."
			return &msg
		}

		if _, hasMinLength := rule["minLength"]; hasMinLength {
			if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
				msg := name + " must not be empty."
				return &msg
			}
		}

		if choicesVal, hasEnum := rule["enum"]; hasEnum {
			if choices, ok := choicesVal.([]any); ok {
				found := false
				for _, choice := range choices {
					if cs, ok := choice.(string); ok {
						if s, ok := value.(string); ok && cs == s {
							found = true
							break
						}
					}
				}
				if !found {
					msg := "Unsupported value for '" + name + "'."
					return &msg
				}
			}
		}

		if fNum, ok := value.(float64); ok {
			intVal := int(fNum)
			if minVal, hasMin := rule["minimum"]; hasMin {
				if minFloat, ok := minVal.(float64); ok && float64(intVal) < minFloat {
					msg := name + " is outside the supported range."
					return &msg
				}
			}
			if maxVal, hasMax := rule["maximum"]; hasMax {
				if maxFloat, ok := maxVal.(float64); ok && float64(intVal) > maxFloat {
					msg := name + " is outside the supported range."
					return &msg
				}
			}
		}

		if _, hasFormat := rule["format"]; hasFormat {
			if s, ok := value.(string); ok {
				_, err1 := time.Parse(time.RFC3339, s)
				_, err2 := time.Parse("2006-01-02", s)
				if err1 != nil && err2 != nil {
					msg := name + " must be an ISO 8601 timestamp."
					return &msg
				}
			}
		}
	}

	return nil
}

var FractalMemoryWorkflowsAll = []FractalMemoryWorkflow{
	{
		Operation:   "read",
		Description: "Read a full Fractal Memory document or section. Returns the whole-document hash for safe writes and source resource links.",
		Properties:  `{"path":{"type":"string","minLength":1},"file":{"type":"string","default":"state"},"section":{"type":"string"}}`,
		Required:    []string{"path"},
	},
	{
		Operation:   "list",
		Description: "Discover Fractal Memory nodes by canonical path. Archived nodes are excluded by default.",
		Properties:  `{"scope":{"type":"string"},"includeArchived":{"type":"boolean","default":false}}`,
		Required:    []string{},
	},
	{
		Operation:   "attention",
		Description: "Find stale Fractal Memory nodes, overdue reviews and missing working context.",
		Properties:  `{"scope":{"type":"string"},"staleDays":{"type":"integer","minimum":1,"default":30}}`,
		Required:    []string{},
	},
	{
		Operation:   "decisions",
		Description: "List managed Fractal Memory decision IDs and content. Superseded decisions are excluded by default.",
		Properties:  `{"path":{"type":"string","minLength":1},"includeSuperseded":{"type":"boolean","default":false}}`,
		Required:    []string{"path"},
	},
	{
		Operation:   "context",
		Description: "Build source-linked Fractal Memory context. maxCharacters bounds the text field only; metadata and resource links are additional. Artifacts must be node-relative artifacts/ paths.",
		Properties:  `{"path":{"type":"string","minLength":1},"maxCharacters":{"type":"integer","minimum":256,"maximum":1000000,"default":6000},"artifacts":{"type":"array","items":{"type":"string"}}}`,
		Required:    []string{"path"},
	},
	{
		Operation:   "resume",
		Description: "Resume a Fractal Memory node with bounded context, attention, its latest handoff and changed contract files.",
		Properties:  `{"path":{"type":"string","minLength":1},"maxCharacters":{"type":"integer","minimum":256,"maximum":1000000,"default":6000}}`,
		Required:    []string{"path"},
	},
	{
		Operation:   "handoff_list",
		Description: "List handoffs for one exact Fractal Memory node, latest first.",
		Properties:  `{"path":{"type":"string","minLength":1}}`,
		Required:    []string{"path"},
	},
	{
		Operation:   "handoff_read",
		Description: "Read a node's latest Fractal Memory handoff or a filename returned by fractal_memory_handoff_list.",
		Properties:  `{"path":{"type":"string","minLength":1},"file":{"type":"string"}}`,
		Required:    []string{"path"},
	},
	{
		Operation:    "node_create",
		Description:  "Create a Fractal Memory node from templates. Existing nodes are never overwritten.",
		Properties:   `{"path":{"type":"string","minLength":1},"format":{"type":"string","enum":["Markdown","Html"],"default":"Markdown"}}`,
		Required:     []string{"path"},
		AlwaysWrites: true,
	},
	{
		Operation:    "update",
		Description:  "Replace an existing Markdown section. Read first and supply expectedHash from fractal_memory_read. Stale writes are rejected; unrelated sections are preserved.",
		Properties:   `{"path":{"type":"string","minLength":1},"section":{"type":"string","minLength":1},"content":{"type":"string"},"expectedHash":{"type":"string","minLength":1},"file":{"type":"string","default":"state"}}`,
		Required:     []string{"path", "section", "content", "expectedHash"},
		AlwaysWrites: true,
	},
	{
		Operation:    "append",
		Description:  "Append a timeline entry or managed decision using the latest document hash. A decision may supersede an active decision ID.",
		Properties:   `{"path":{"type":"string","minLength":1},"file":{"type":"string","enum":["timeline","decisions"]},"content":{"type":"string"},"expectedHash":{"type":"string","minLength":1},"supersedes":{"type":"string"}}`,
		Required:     []string{"path", "file", "content", "expectedHash"},
		AlwaysWrites: true,
	},
	{
		Operation:    "review",
		Description:  "Record the next Fractal Memory review timestamp and optional status. expectedHash must come from reading the index document.",
		Properties:   `{"path":{"type":"string","minLength":1},"reviewAfter":{"type":"string","format":"date-time"},"expectedHash":{"type":"string","minLength":1},"status":{"type":"string","enum":["Active","Paused","Archived","Draft"]}}`,
		Required:     []string{"path", "reviewAfter", "expectedHash"},
		AlwaysWrites: true,
	},
	{
		Operation:   "doctor",
		Description: "Diagnose Fractal Memory repository problems. repair=true rebuilds derived indexes after source validation; it requires write permission.",
		Properties:  `{"repair":{"type":"boolean","default":false}}`,
		Required:    []string{},
		WriteFlag:   new("repair"),
	},
	{
		Operation:   "import",
		Description: "Preview supplied note content at an explicit Fractal Memory node. sourceName must end in .md, .html, .htm or .txt. apply=true persists it and requires write permission. Conflicts block writes; no external host files are read.",
		Properties:  `{"path":{"type":"string","minLength":1},"sourceName":{"type":"string","minLength":1},"content":{"type":"string"},"sourceModified":{"type":"string","format":"date-time"},"apply":{"type":"boolean","default":false}}`,
		Required:    []string{"path", "sourceName", "content"},
		WriteFlag:   new("apply"),
	},
}

func FindFractalMemoryWorkflow(operation string) *FractalMemoryWorkflow {
	for i := range FractalMemoryWorkflowsAll {
		if FractalMemoryWorkflowsAll[i].Operation == operation {
			return &FractalMemoryWorkflowsAll[i]
		}
	}
	return nil
}

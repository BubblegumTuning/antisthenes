package agent

import openai "github.com/sashabaranov/go-openai"

var contextToolSchemas = map[string]openai.FunctionDefinition{
	"dump_work_summary": {
		Description: "Dump current work to markdown",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary":    map[string]any{"type": "string", "description": "Work summary markdown body"},
				"session_id": map[string]any{"type": "string", "description": "Optional session id for the dump path"},
			},
			"required": []string{"summary"},
		},
	},
	"load_work_summary": {
		Description: "Load a work summary",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Path to a previously dumped work summary file"},
			},
			"required": []string{"path"},
		},
	},
}

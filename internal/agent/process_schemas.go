package agent

import openai "github.com/sashabaranov/go-openai"

var processToolSchemas = map[string]openai.FunctionDefinition{
	"list_processes": {
		Description: "List running processes (ps/pgrep)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "Filter by command pattern (pgrep -af when available)"},
				"user":        map[string]any{"type": "string", "description": "Filter by username"},
				"max_results": map[string]any{"type": "integer", "description": "Max lines (default 100, cap 500)"},
			},
		},
	},
	"kill_process": {
		Description: "Send a signal to a process by pid (always requires user approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pid":    map[string]any{"type": "integer", "description": "Process id"},
				"signal": map[string]any{"type": "string", "description": "Signal name or number (default TERM)"},
			},
			"required": []string{"pid"},
		},
	},
}

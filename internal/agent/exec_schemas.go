package agent

import openai "github.com/sashabaranov/go-openai"

var execToolSchemas = map[string]openai.FunctionDefinition{
	"bash": {
		Description: "Execute bash command (with approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Bash command to execute"},
			},
			"required": []string{"command"},
		},
	},
	"run_command": {
		Description: "Run a shell command with optional cwd, env, timeout, or background mode (with approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":    map[string]any{"type": "string", "description": "Shell command to execute"},
				"cwd":        map[string]any{"type": "string", "description": "Working directory (relative path)"},
				"env":        map[string]any{"type": "object", "description": "Environment variable overrides"},
				"timeout":    map[string]any{"type": "integer", "description": "Timeout in seconds for foreground runs (0 = no limit)"},
				"background": map[string]any{"type": "boolean", "description": "Start in background; returns job id (use wait_job for output)"},
			},
			"required": []string{"command"},
		},
	},
	"wait_job": {
		Description: "Wait for a background run_command job and return its output",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id":  map[string]any{"type": "integer", "description": "Background job id from run_command"},
				"timeout": map[string]any{"type": "integer", "description": "Max seconds to wait (0 = wait until done)"},
			},
			"required": []string{"job_id"},
		},
	},
	"list_background_jobs": {
		Description: "List background run_command jobs and their status",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

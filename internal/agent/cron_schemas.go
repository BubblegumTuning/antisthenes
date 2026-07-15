package agent

import openai "github.com/sashabaranov/go-openai"

var cronToolSchemas = map[string]openai.FunctionDefinition{
	"schedule_task": {
		Description: "Schedule a recurring agent task (requires cron_enabled in config)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":       map[string]any{"type": "string", "description": "Unique task id"},
				"schedule": map[string]any{"type": "string", "description": "Schedule string (e.g. every 5m, every 1h)"},
				"command":  map[string]any{"type": "string", "description": "Prompt/command for the agent when the task fires"},
			},
			"required": []string{"id", "schedule", "command"},
		},
	},
	"list_tasks": {Description: "List scheduled cron tasks"},
	"cancel_task": {
		Description: "Cancel a scheduled cron task by id",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task id to cancel"},
			},
			"required": []string{"id"},
		},
	},
}

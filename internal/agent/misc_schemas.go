package agent

import openai "github.com/sashabaranov/go-openai"

var miscToolSchemas = map[string]openai.FunctionDefinition{
	"get_current_time": {
		Description: "Current time",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	"echo": {
		Description: "Echo message",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "Text to echo"},
			},
			"required": []string{"message"},
		},
	},
	"get_env": {
		Description: "Get env var",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Environment variable name"},
			},
			"required": []string{"key"},
		},
	},
	"create_skill": {
		Description: "Create a new skill autonomously",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Skill folder name under skills/"},
				"description": map[string]any{"type": "string", "description": "Short skill description"},
			},
			"required": []string{"name"},
		},
	},
	"goban_create_ticket": {
		Description: "Create a ticket in Goban Kanban for chunked work decomposition and tracking (enables job analysis -> chunking -> per-chunk tickets -> context reset via self-prompt)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string", "description": "Ticket title"},
				"description": map[string]any{"type": "string", "description": "Ticket body"},
				"column":      map[string]any{"type": "string", "description": "Board column (default todo)"},
			},
			"required": []string{"title"},
		},
	},
}

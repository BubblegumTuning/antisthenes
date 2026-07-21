package agent

import openai "github.com/sashabaranov/go-openai"

var approvalToolSchemas = map[string]openai.FunctionDefinition{
	"approve_tool": {
		Description: "Approve a command (once/session/permanent)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Command string (or prefix) to approve"},
				"level":   map[string]any{"type": "string", "description": "once (default), session, or permanent"},
			},
			"required": []string{"command"},
		},
	},
	"reset_approvals": {
		Description: "Reset all permanent approvals",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

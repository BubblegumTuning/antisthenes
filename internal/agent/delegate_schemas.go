package agent

import openai "github.com/sashabaranov/go-openai"

var delegateToolSchemas = map[string]openai.FunctionDefinition{
	"delegate_task": {
		Description: "Delegate a task to a subagent",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"goal":     map[string]any{"type": "string", "description": "Task goal for the subagent"},
				"executor": map[string]any{"type": "string", "description": "Optional executor name (auto, coder, deep-thinker, orchestrator, or aux model)"},
			},
			"required": []string{"goal"},
		},
	},
}

package agent

import openai "github.com/sashabaranov/go-openai"

var auxToolSchemas = map[string]openai.FunctionDefinition{
	"list_aux_models": {
		Description: "List configured auxiliary models (cheap/async endpoints for titles, summaries, light completions). Returns name, model, base_url, roles.",
	},
	"complete_with_aux": {
		Description: "Run a single short completion on an auxiliary model (no tools). Prefer for cheap tasks; use list_aux_models for names.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Aux model name from list_aux_models; omit to use the first configured model",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "User prompt for the aux model",
				},
				"system": map[string]any{
					"type":        "string",
					"description": "Optional system instruction",
				},
			},
			"required": []string{"prompt"},
		},
	},
}

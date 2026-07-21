package agent

import openai "github.com/sashabaranov/go-openai"

var skillsToolSchemas = map[string]openai.FunctionDefinition{
	"list_skills": {
		Description: "List available skills (name and short description only)",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	"load_skill": {
		Description: "Load the full SKILL.md content for a named skill on demand",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Skill name (folder name under skills/)"},
			},
			"required": []string{"name"},
		},
	},
}

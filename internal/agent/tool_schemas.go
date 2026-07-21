package agent

import openai "github.com/sashabaranov/go-openai"

// toolSchemas holds OpenAI function metadata for registered tools.
// Domain maps live in *_schemas.go; init merges them here.
// ToOpenAITools exposes only tools present in the registry; unregistered schema entries are ignored.
var toolSchemas map[string]openai.FunctionDefinition

func init() {
	toolSchemas = make(map[string]openai.FunctionDefinition)
	mergeToolSchemas(
		miscToolSchemas,
		fsToolSchemas,
		execToolSchemas,
		skillsToolSchemas,
		contextToolSchemas,
		delegateToolSchemas,
		searchPatchToolSchemas,
		ansibleToolSchemas,
		nmapToolSchemas,
		networkToolSchemas,
		cronToolSchemas,
		processToolSchemas,
		httpToolSchemas,
		approvalToolSchemas,
		modernToolSchemas,
		installToolSchemas,
		gitToolSchemas,
		mcpToolSchemas,
		tmuxToolSchemas,
		auxToolSchemas,
	)
}

func mergeToolSchemas(maps ...map[string]openai.FunctionDefinition) {
	for _, m := range maps {
		for k, v := range m {
			toolSchemas[k] = v
		}
	}
}

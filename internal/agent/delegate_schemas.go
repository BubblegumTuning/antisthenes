package agent

import openai "github.com/sashabaranov/go-openai"

var delegateToolSchemas = map[string]openai.FunctionDefinition{
	"delegate_task": {Description: "Delegate a task to a subagent"},
}

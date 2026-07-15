package agent

import openai "github.com/sashabaranov/go-openai"

var approvalToolSchemas = map[string]openai.FunctionDefinition{
	"approve_tool":    {Description: "Approve a command (once/session/permanent)"},
	"reset_approvals": {Description: "Reset all permanent approvals"},
}

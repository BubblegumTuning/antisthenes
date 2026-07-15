package agent

import openai "github.com/sashabaranov/go-openai"

var mcpToolSchemas = map[string]openai.FunctionDefinition{
	"mcp_call": {
		Description: "Call a tool on a remote MCP server over stdio",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{
					"type":        "string",
					"description": "MCP server command (e.g. \"./antisthenes mcp\")",
				},
				"tool": map[string]any{
					"type":        "string",
					"description": "Remote tool name to invoke",
				},
				"arguments": map[string]any{
					"type":        "object",
					"description": "Arguments to pass to the remote tool",
				},
			},
			"required": []string{"server", "tool"},
		},
	},
}

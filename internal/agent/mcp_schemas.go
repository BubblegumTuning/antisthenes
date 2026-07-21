package agent

import openai "github.com/sashabaranov/go-openai"

var mcpToolSchemas = map[string]openai.FunctionDefinition{
	"mcp_call": {
		Description: "Call a tool on a remote MCP server over stdio. Prefer mcp_list_tools first when the remote catalog is unknown.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{
					"type":        "string",
					"description": "MCP server command or binary (e.g. \"./antisthenes mcp\" or \"./antisthenes\" with args)",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Optional argv after the binary when server is the executable only (e.g. [\"mcp\"])",
					"items":       map[string]any{"type": "string"},
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
	"mcp_list_tools": {
		Description: "List tools (name, description, inputSchema) from a remote MCP server over stdio",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{
					"type":        "string",
					"description": "MCP server command or binary (e.g. \"./antisthenes mcp\" or \"./antisthenes\" with args)",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Optional argv after the binary when server is the executable only (e.g. [\"mcp\"])",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required": []string{"server"},
		},
	},
}

package agent

import openai "github.com/sashabaranov/go-openai"

var installToolSchemas = map[string]openai.FunctionDefinition{
	"tool_status": {
		Description: "Show whether installable CLI tools (rg, fd, bat, nmap, ansible, etc.) are on PATH and which install_tool id to use when missing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool": map[string]any{
					"type":        "string",
					"description": "Canonical id, alias, 'all' (default), or 'prefcli' for fd/bat/eza/fzf/ast-grep/zoxide/delta only",
				},
			},
		},
	},
	"install_tool": {
		Description: "Install one or more CLI tools via the system package manager (or venv+pip for ansible). Requires user approval. Use tool_status first.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool": map[string]any{
					"type":        "string",
					"description": "Tool id or alias (rg, nmap, ansible), prefcli_missing, or all_missing",
				},
				"tools": map[string]any{
					"type":        "array",
					"description": "Install multiple tools in one approved command",
					"items":       map[string]any{"type": "string"},
				},
			},
		},
	},
}

package mcp

import (
	"fmt"

	"github.com/nanami/antisthenes/internal/agent"
)

// RegisterMCPCallTool adds the "mcp_call" tool to the given registry.
// This allows the agent to invoke tools on remote MCP servers.
func RegisterMCPCallTool(r *agent.ToolRegistry) {
	r.Register("mcp_call", func(args map[string]any) (string, error) {
		server, ok := args["server"].(string)
		if !ok || server == "" {
			return "mcp_call: server command is required (e.g. \"./antisthenes mcp\")", nil
		}
		tool, ok := args["tool"].(string)
		if !ok || tool == "" {
			return "mcp_call: tool name is required", nil
		}
		toolArgs, _ := args["arguments"].(map[string]any)

		client, err := NewClient(server)
		if err != nil {
			return "", fmt.Errorf("failed to connect to MCP server: %w", err)
		}
		defer client.Close()

		result, err := client.CallTool(tool, toolArgs)
		if err != nil {
			return "", err
		}
		return result, nil
	})
}

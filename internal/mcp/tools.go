package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/nanami/antisthenes/internal/agent"
)

// RegisterMCPCallTool adds mcp_call and mcp_list_tools to the given registry.
// Agent paths only — not the standalone MCP server (avoids recursive self-MCP packs).
func RegisterMCPCallTool(r *agent.ToolRegistry) {
	r.Register("mcp_call", func(args map[string]any) (string, error) {
		server, ok := args["server"].(string)
		if !ok || strings.TrimSpace(server) == "" {
			return `mcp_call: server command is required (e.g. "./antisthenes mcp")`, nil
		}
		tool, ok := args["tool"].(string)
		if !ok || tool == "" {
			return "mcp_call: tool name is required", nil
		}
		toolArgs, _ := args["arguments"].(map[string]any)

		client, err := openMCPClient(server, args["args"])
		if err != nil {
			return "", err
		}
		defer client.Close()

		result, err := client.CallTool(tool, toolArgs)
		if err != nil {
			return "", err
		}
		return result, nil
	})

	r.Register("mcp_list_tools", func(args map[string]any) (string, error) {
		server, ok := args["server"].(string)
		if !ok || strings.TrimSpace(server) == "" {
			return `mcp_list_tools: server command is required (e.g. "./antisthenes mcp")`, nil
		}

		client, err := openMCPClient(server, args["args"])
		if err != nil {
			return "", err
		}
		defer client.Close()

		tools, err := client.ListTools()
		if err != nil {
			return "", err
		}
		return formatMCPToolList(tools), nil
	})
}

func openMCPClient(server string, argsAny any) (*Client, error) {
	argv, err := resolveServerArgv(server, argsAny)
	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
	client, err := NewClient(argv[0], argv[1:]...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	return client, nil
}

// formatMCPToolList renders remote tools for the agent (name, description, compact schema).
func formatMCPToolList(tools []map[string]any) string {
	if len(tools) == 0 {
		return "mcp_list_tools: no tools reported"
	}

	// Stable order by name
	sort.Slice(tools, func(i, j int) bool {
		ni, _ := tools[i]["name"].(string)
		nj, _ := tools[j]["name"].(string)
		return ni < nj
	})

	var b strings.Builder
	fmt.Fprintf(&b, "mcp_list_tools: %d tool(s)\n", len(tools))
	for _, t := range tools {
		name, _ := t["name"].(string)
		if name == "" {
			name = "(unnamed)"
		}
		desc, _ := t["description"].(string)
		fmt.Fprintf(&b, "- %s", name)
		if desc != "" {
			fmt.Fprintf(&b, ": %s", desc)
		}
		b.WriteByte('\n')
		if schema := t["inputSchema"]; schema != nil {
			raw, err := json.Marshal(schema)
			if err == nil && string(raw) != "null" && string(raw) != "{}" {
				const maxSchema = 400
				s := string(raw)
				if len(s) > maxSchema {
					s = s[:maxSchema] + "…"
				}
				fmt.Fprintf(&b, "  inputSchema: %s\n", s)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// resolveServerArgv builds argv from server string and optional args array.
// If args is provided (array of strings), server is the binary only.
// Otherwise server may be a full command line (e.g. "./antisthenes mcp"),
// split with simple shell-like whitespace/quoting rules.
func resolveServerArgv(server string, argsAny any) ([]string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("server command is empty")
	}

	if argsAny != nil {
		extra, err := stringSliceArg(argsAny)
		if err != nil {
			return nil, err
		}
		return append([]string{server}, extra...), nil
	}

	parts, err := splitCommandLine(server)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("server command is empty")
	}
	return parts, nil
}

func stringSliceArg(v any) ([]string, error) {
	switch t := v.(type) {
	case []string:
		return t, nil
	case []any:
		out := make([]string, 0, len(t))
		for i, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("args[%d] must be a string", i)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("args must be an array of strings")
	}
}

// splitCommandLine splits a command string into argv.
// Supports double/single quotes; backslash escapes the next character inside double quotes or outside quotes.
func splitCommandLine(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var parts []string
	var b strings.Builder
	var quote rune // 0, '"' or '\''

	flush := func() {
		if b.Len() > 0 {
			parts = append(parts, b.String())
			b.Reset()
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '"' && r == '\\' && i+1 < len(runes):
			b.WriteRune(runes[i+1])
			i++
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '\\' && i+1 < len(runes):
			b.WriteRune(runes[i+1])
			i++
		case unicode.IsSpace(r):
			flush()
		default:
			b.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in server command")
	}
	flush()
	return parts, nil
}

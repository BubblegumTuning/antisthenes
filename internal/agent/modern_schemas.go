package agent

import openai "github.com/sashabaranov/go-openai"

var modernToolSchemas = map[string]openai.FunctionDefinition{
	"modern_cli_status": {Description: "Deprecated: use tool_status with tool=prefcli. Shows fd, bat, eza, fzf, ast-grep, zoxide, delta availability."},
	"install_modern_cli": {
		Description: "Deprecated: use install_tool with tool=prefcli_missing. Installs missing preferred CLI tools via apk/apt/dnf. Requires approval.",
	},
	"find_files": {
		Description: "Find files by name using fd (fallback: find)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Filename pattern (optional)"},
				"path":    map[string]any{"type": "string", "description": "Directory to search (default .)"},
			},
		},
	},
	"fuzzy_find": {
		Description: "Fuzzy-find files using fd + fzf (fallback: find + grep)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Fuzzy filter string"},
				"path":  map[string]any{"type": "string", "description": "Directory root (default .)"},
			},
		},
	},
	"cd_path": {
		Description: "Change working directory using zoxide query (fallback: direct relative path)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Directory name or fragment (zoxide query)"},
				"path":  map[string]any{"type": "string", "description": "Direct relative path (alternative to query)"},
			},
		},
	},
	"resolve_path": {
		Description: "Resolve a directory path using zoxide (fallback: direct path if it exists)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Directory name or fragment to resolve"},
			},
			"required": []string{"query"},
		},
	},
}

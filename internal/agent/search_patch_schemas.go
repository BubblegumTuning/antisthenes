package agent

import openai "github.com/sashabaranov/go-openai"

var searchPatchToolSchemas = map[string]openai.FunctionDefinition{
	"search_files": {
		Description: "Search file contents (prefers rg, then ast-grep, then grep -r)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "Search pattern (regex for rg/ast-grep)"},
				"path":        map[string]any{"type": "string", "description": "Directory to search (default .)"},
				"max_results": map[string]any{"type": "integer", "description": "Max result lines (default 500, cap 5000; 0 uses cap)"},
			},
			"required": []string{"pattern"},
		},
	},
	"patch": {
		Description: "Apply a patch to a file (unified diff via git apply or Go fallback, or old_text/new_text replace)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"diff":     map[string]any{"type": "string", "description": "Unified diff content"},
				"path":     map[string]any{"type": "string", "description": "Target file path (optional for diff if path is in headers)"},
				"old_text": map[string]any{"type": "string", "description": "Text to replace (simple mode)"},
				"new_text": map[string]any{"type": "string", "description": "Replacement text (simple mode)"},
			},
		},
	},
}

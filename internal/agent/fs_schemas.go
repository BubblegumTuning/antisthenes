package agent

import openai "github.com/sashabaranov/go-openai"

var fsToolSchemas = map[string]openai.FunctionDefinition{
	"list_dir": {
		Description: "List directory contents (prefers eza, falls back to ls)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Directory path (optional, defaults to .)"},
			},
		},
	},
	"create_dir": {
		Description: "Create a directory (and any necessary parents)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Directory path to create"},
			},
		},
	},
	"read_file": {
		Description: "Read file contents (text via bat/cat, or raw bytes as base64)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "File path to read"},
				"encoding":  map[string]any{"type": "string", "description": "text (default) or base64 for binary files"},
				"max_bytes": map[string]any{"type": "integer", "description": "Max bytes for base64 reads (default 1 MiB, cap 8 MiB)"},
			},
			"required": []string{"path"},
		},
	},
	"delete_file": {
		Description: "Delete a file or directory (requires approval for rm -rf patterns)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "Relative path to delete"},
				"recursive": map[string]any{"type": "boolean", "description": "Remove directory trees (uses rm -rf policy gate)"},
			},
			"required": []string{"path"},
		},
	},
	"move_file": {
		Description: "Move or rename a file (relative paths only)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"src": map[string]any{"type": "string", "description": "Source path"},
				"dst": map[string]any{"type": "string", "description": "Destination path"},
			},
			"required": []string{"src", "dst"},
		},
	},
	"file_stat": {
		Description: "Get file metadata (type, size, permissions, modified time)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File or directory path"},
			},
			"required": []string{"path"},
		},
	},
	"chmod": {
		Description: "Change file permissions (octal mode, e.g. 755)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File or directory path"},
				"mode": map[string]any{"type": "string", "description": "Octal permission bits (e.g. 0644)"},
			},
			"required": []string{"path", "mode"},
		},
	},
	"copy_file": {
		Description: "Copy a file to a new location (relative paths only)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"src": map[string]any{"type": "string", "description": "Source path"},
				"dst": map[string]any{"type": "string", "description": "Destination path"},
			},
			"required": []string{"src", "dst"},
		},
	},
	"write_file": {
		Description: "Write content to a file (text or base64-encoded binary)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "File path to write"},
				"content":  map[string]any{"type": "string", "description": "File content (text or base64 when encoding=base64)"},
				"encoding": map[string]any{"type": "string", "description": "text (default) or base64"},
				"mode":     map[string]any{"type": "string", "description": "Optional octal file mode (e.g. 0644, 0755)"},
			},
			"required": []string{"path", "content"},
		},
	},
}

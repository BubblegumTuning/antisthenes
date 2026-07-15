package agent

import openai "github.com/sashabaranov/go-openai"

var gitToolSchemas = map[string]openai.FunctionDefinition{
	"git_status": {
		Description: "Show git working tree status",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":  map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"path": map[string]any{"type": "string", "description": "Limit status to path"},
				"full": map[string]any{"type": "boolean", "description": "Full status instead of --short"},
			},
		},
	},
	"git_log": {
		Description: "Show recent git commits (oneline)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":   map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"count": map[string]any{"type": "integer", "description": "Number of commits (default 20, max 200)"},
				"path":  map[string]any{"type": "string", "description": "Limit log to path"},
			},
		},
	},
	"git_add": {
		Description: "Stage files for commit (requires approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":   map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"paths": map[string]any{"type": "string", "description": "Space-separated paths to stage"},
				"all":   map[string]any{"type": "boolean", "description": "Stage all changes (git add -A)"},
			},
		},
	},
	"git_commit": {
		Description: "Create a git commit (requires approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":     map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"message": map[string]any{"type": "string", "description": "Commit message"},
				"all":     map[string]any{"type": "boolean", "description": "Commit all tracked changes (-a)"},
				"amend":   map[string]any{"type": "boolean", "description": "Amend previous commit"},
			},
			"required": []string{"message"},
		},
	},
	"git_checkout": {
		Description: "Switch branches or restore paths (requires approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":           map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"ref":           map[string]any{"type": "string", "description": "Branch, commit, or path"},
				"create_branch": map[string]any{"type": "boolean", "description": "Create branch (-b) with ref as name"},
			},
			"required": []string{"ref"},
		},
	},
	"git_branch": {
		Description: "List branches or create a new branch (create requires approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":  map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"name": map[string]any{"type": "string", "description": "New branch name (omit to list)"},
			},
		},
	},
	"git_show": {
		Description: "Show a git commit (default HEAD)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":  map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"ref":  map[string]any{"type": "string", "description": "Commit ref (default HEAD)"},
				"stat": map[string]any{"type": "boolean", "description": "Include --stat summary"},
			},
		},
	},
	"git_diff": {
		Description: "Show git diff with delta pager when available (fallback: plain git diff)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd":  map[string]any{"type": "string", "description": "Repository directory (optional)"},
				"args": map[string]any{"type": "string", "description": "Extra git diff arguments (e.g. --staged path/)"},
			},
		},
	},
}

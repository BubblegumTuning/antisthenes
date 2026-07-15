package agent

import openai "github.com/sashabaranov/go-openai"

var tmuxToolSchemas = map[string]openai.FunctionDefinition{
	"tmux_attach_or_create": {
		Description: "Create or reuse one long-lived tmux session (interactive shell). Preferred Phase 2 name. Optional host= for registered remote; omit for localhost. On-demand create if missing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_name": map[string]any{"type": "string", "description": "tmux session (default: host default or antisthenes-persist); prefer one session per host"},
				"host":         map[string]any{"type": "string", "description": "registered host alias, or omit/local/localhost"},
			},
		},
	},
	"tmux_attach": {
		Description: "Alias of tmux_attach_or_create (create or reuse persistent session).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_name": map[string]any{"type": "string", "description": "tmux session name"},
				"host":         map[string]any{"type": "string", "description": "registered host alias (optional)"},
			},
		},
	},
	"tmux_send": {
		Description: "Send keys + Enter to the session. Creates the session on demand if missing. Optional host=. Approval/safety parity with bash.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_name": map[string]any{"type": "string", "description": "tmux session name"},
				"host":         map[string]any{"type": "string", "description": "registered host alias (optional)"},
				"keys":         map[string]any{"type": "string", "description": "command or text; Enter appended automatically"},
			},
			"required": []string{"keys"},
		},
	},
	"tmux_capture": {
		Description: "Capture pane output. format=llm (default, compact + header), human (labeled pretty), or raw. Optional host=. Creates session on demand if missing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_name": map[string]any{"type": "string", "description": "tmux session name"},
				"host":         map[string]any{"type": "string", "description": "registered host alias (optional)"},
				"lines":        map[string]any{"type": "integer", "description": "history lines from end (default 100, max 10000)"},
				"format":       map[string]any{"type": "string", "description": "llm | human | raw (default llm)"},
			},
		},
	},
	"tmux_list_sessions": {
		Description: "List active tmux sessions on localhost or a registered remote host.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"host": map[string]any{"type": "string", "description": "registered host alias (optional)"},
			},
		},
	},
	"tmux_kill_session": {
		Description: "Kill a tmux session on localhost or a registered remote host.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_name": map[string]any{"type": "string", "description": "tmux session name"},
				"host":         map[string]any{"type": "string", "description": "registered host alias (optional)"},
			},
		},
	},
	"tmux_register_host": {
		Description: "Register an SSH host for remote tmux (one default long-lived session_name per host). Validates key path; optional SSH check.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string", "description": "alias used as host= on other tmux tools"},
				"host":         map[string]any{"type": "string", "description": "hostname or IP"},
				"user":         map[string]any{"type": "string", "description": "SSH username"},
				"key_path":     map[string]any{"type": "string", "description": "path to private key (~ expanded)"},
				"session_name": map[string]any{"type": "string", "description": "default remote session (one long-lived session per host)"},
				"port":         map[string]any{"type": "integer", "description": "SSH port (default 22)"},
				"validate":     map[string]any{"type": "boolean", "description": "run SSH connectivity check (default true)"},
			},
			"required": []string{"name", "host", "user", "key_path"},
		},
	},
	"tmux_list_hosts": {
		Description: "List registered remote hosts for tmux tools.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

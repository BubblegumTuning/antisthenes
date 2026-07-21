package agent

import openai "github.com/sashabaranov/go-openai"

var ansibleToolSchemas = map[string]openai.FunctionDefinition{
	"ansible_check": {
		Description: "Check if Ansible is available (venv or global) and scaffold playbooks/logs dirs. Use install_tool with tool=ansible when missing.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	"ansible_run_playbook": {
		Description: "Run an existing Ansible playbook (requires approval)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Playbook basename under playbooks/ (without .yml)"},
				"path": map[string]any{"type": "string", "description": "Explicit playbook path (alternative to name)"},
			},
		},
	},
	"ansible_generate_playbook": {
		Description: "Generate a new Ansible playbook (avoids duplicates, uses host_vars)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Playbook basename (default generated-<ts>)"},
				"description": map[string]any{"type": "string", "description": "What the playbook should do (embedded in scaffold)"},
			},
		},
	},
}

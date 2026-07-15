package agent

import openai "github.com/sashabaranov/go-openai"

var ansibleToolSchemas = map[string]openai.FunctionDefinition{
	"ansible_check":             {Description: "Check if Ansible is available (venv or global) and scaffold playbooks/logs dirs. Use install_tool with tool=ansible when missing."},
	"ansible_run_playbook":      {Description: "Run an existing Ansible playbook (requires approval)"},
	"ansible_generate_playbook": {Description: "Generate a new Ansible playbook (avoids duplicates, uses host_vars)"},
}

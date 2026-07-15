package agent

import openai "github.com/sashabaranov/go-openai"

var nmapToolSchemas = map[string]openai.FunctionDefinition{
	"nmap_scan": {
		Description: "Run a constrained nmap scan against a single host or CIDR. Requires user approval. Use install_tool with tool=nmap when nmap is missing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Hostname, IP address, or CIDR (e.g. 192.168.1.1 or 10.0.0.0/24)",
				},
				"scan_type": map[string]any{
					"type":        "string",
					"description": "Scan preset: ping (-sn), quick (-F), ports (-sT), service (-sT -sV). Default ping.",
					"enum":        []string{"ping", "quick", "ports", "service"},
				},
				"ports": map[string]any{
					"type":        "string",
					"description": "Port list for ports/service scans (e.g. 22,80,443 or 1-1000). Default 1-1000 when omitted.",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (default 120, max 600)",
				},
			},
			"required": []string{"target"},
		},
	},
}

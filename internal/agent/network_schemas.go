package agent

import openai "github.com/sashabaranov/go-openai"

var networkToolSchemas = map[string]openai.FunctionDefinition{
	"network_status": {
		Description: "Read local network configuration: interface IPs, default gateways, and DNS. Registered when network_status_enabled is true in config. Read-only; no approval required.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"interface": map[string]any{
					"type":        "string",
					"description": "Optional interface name filter (e.g. eth0, ens33)",
				},
				"include_loopback": map[string]any{
					"type":        "boolean",
					"description": "Include loopback (lo). Default false.",
				},
				"detail": map[string]any{
					"type":        "string",
					"description": "brief=Go stdlib and file parsing only; full=also try resolvectl and ip -json enrichment. Default brief.",
					"enum":        []string{"brief", "full"},
				},
			},
		},
	},
}
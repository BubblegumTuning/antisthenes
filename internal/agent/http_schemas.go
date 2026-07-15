package agent

import openai "github.com/sashabaranov/go-openai"

var httpToolSchemas = map[string]openai.FunctionDefinition{
	"http_fetch": {
		Description: "Fetch a URL over HTTP/HTTPS (GET/POST/etc.) with optional headers, body, and timeout",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":     map[string]any{"type": "string", "description": "HTTP or HTTPS URL"},
				"method":  map[string]any{"type": "string", "description": "HTTP method (default GET)"},
				"headers": map[string]any{"type": "object", "description": "Request headers"},
				"body":    map[string]any{"type": "string", "description": "Request body"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default 30)"},
			},
			"required": []string{"url"},
		},
	},
}

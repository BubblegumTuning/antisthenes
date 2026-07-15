package agent

import openai "github.com/sashabaranov/go-openai"

var miscToolSchemas = map[string]openai.FunctionDefinition{
	"get_current_time": {Description: "Current time"},
	"echo":             {Description: "Echo message"},
	"get_env":          {Description: "Get env var"},
	"create_skill":     {Description: "Create a new skill autonomously"},
	"goban_create_ticket": {
		Description: "Create a ticket in Goban Kanban for chunked work decomposition and tracking (enables job analysis -> chunking -> per-chunk tickets -> context reset via self-prompt)",
	},
}

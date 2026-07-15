package agent

import openai "github.com/sashabaranov/go-openai"

var contextToolSchemas = map[string]openai.FunctionDefinition{
	"dump_work_summary": {Description: "Dump current work to markdown"},
	"load_work_summary": {Description: "Load a work summary"},
}

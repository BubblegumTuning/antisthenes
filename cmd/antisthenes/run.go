package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	"github.com/nanami/antisthenes/internal/mcp"
)

// newDefaultRegistryAndLoop creates a ToolRegistry (with MCP call tool registered)
// and a Loop. This is the first step toward removing duplication between
// the one-shot --prompt path and the main TUI bootstrap path.
func newDefaultRegistryAndLoop(apiKey, model, baseURL string, cfg config.Config) (*agent.ToolRegistry, *agent.Loop) {
	reg := agent.NewToolRegistry()
	mcp.RegisterMCPCallTool(reg)
	agent.RegisterCronTools(reg, nil)
	agent.RegisterNmapTools(reg, cfg.NmapOn())
	agent.RegisterNetworkTools(reg, cfg.NetworkStatusOn())
	loop := agent.NewLoopWithRegistry(apiKey, model, baseURL, reg)
	return reg, loop
}

// tryRunOneShot checks os.Args for --prompt, -P, or --prompt-file and runs
// the one-shot non-interactive path if present. Returns true if handled
// (caller should return from main to avoid falling through to TUI).
func tryRunOneShot(args []string, cfg config.Config) bool {
	for i, arg := range args {
		switch {
		case arg == "--prompt-file" && i+1 < len(args):
			prompt, err := readPromptFile(args[i+1])
			if err != nil {
				exitOneShotError(err)
			}
			runOneShot(prompt, cfg)
			return true
		case (arg == "--prompt" || arg == "-P") && i+1 < len(args):
			prompt, err := resolveOneShotPrompt(args[i+1])
			if err != nil {
				exitOneShotError(err)
			}
			runOneShot(prompt, cfg)
			return true
		}
	}
	return false
}

// resolveOneShotPrompt turns a --prompt / -P value into prompt text.
// "-" reads stdin; "@path" reads a file; otherwise the value is used inline.
func resolveOneShotPrompt(spec string) (string, error) {
	switch {
	case spec == "-":
		return readPromptStdin()
	case strings.HasPrefix(spec, "@"):
		return readPromptFile(spec[1:])
	default:
		return spec, nil
	}
}

func readPromptStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading prompt from stdin: %w", err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("empty prompt from stdin")
	}
	return prompt, nil
}

func readPromptFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading prompt file %q: %w", path, err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("empty prompt in file %q", path)
	}
	return prompt, nil
}

func exitOneShotError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func runOneShot(prompt string, cfg config.Config) {
	ep := cfg.GetActiveEndpoint()

	_, loop := newDefaultRegistryAndLoop(ep.APIKey, ep.Model, ep.BaseURL, cfg)

	ctx := context.Background()
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: prompt}}
	final, err := loop.RunWithTools(ctx, msgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	for j := len(final) - 1; j >= 0; j-- {
		if final[j].Role == "assistant" && final[j].Content != "" {
			fmt.Println(strings.TrimSpace(final[j].Content))
			return
		}
	}
	fmt.Println("(no response)")
}

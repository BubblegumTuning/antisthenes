package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	"github.com/nanami/antisthenes/internal/mcp"
)

// RegistryOptions selects optional tool packs layered on NewToolRegistry.
// Nmap/network always follow cfg flags on every path.
//
// Standalone MCP server intentionally omits:
//   - WithMCPCall — avoids recursive mcp_call/mcp_list_tools → antisthenes mcp
//   - WithCron    — no scheduler lifecycle on stdio server
//   - WithAux     — aux models need agent/LLM wiring
type RegistryOptions struct {
	WithMCPCall bool
	WithCron    bool
	WithAux     bool
}

// agentRegistryOptions is the TUI / one-shot tool surface.
func agentRegistryOptions() RegistryOptions {
	return RegistryOptions{WithMCPCall: true, WithCron: true, WithAux: true}
}

// mcpServerRegistryOptions is the standalone `antisthenes mcp` tool surface.
func mcpServerRegistryOptions() RegistryOptions {
	return RegistryOptions{}
}

// newToolRegistry builds a registry from the base set plus optional packs.
// Shared by agent paths and the MCP subcommand so nmap/network (and future
// cfg-gated tools) cannot drift between entrypoints.
func newToolRegistry(cfg config.Config, opts RegistryOptions) *agent.ToolRegistry {
	reg := agent.NewToolRegistry()
	agent.ConfigureHTTPFetch(reg, cfg.ResolvedHTTPUserAgent())
	agent.RegisterNmapTools(reg, cfg.NmapOn())
	agent.RegisterNetworkTools(reg, cfg.NetworkStatusOn())
	if opts.WithMCPCall {
		mcp.RegisterMCPCallTool(reg)
	}
	if opts.WithCron {
		agent.RegisterCronTools(reg, nil)
	}
	if opts.WithAux {
		agent.RegisterAuxTools(reg, cfg)
	}
	return reg
}

// newDefaultRegistryAndLoop creates a ToolRegistry (agent packs) and a Loop.
// Shared by the one-shot --prompt path and the main TUI bootstrap path.
func newDefaultRegistryAndLoop(apiKey, model, baseURL string, cfg config.Config) (*agent.ToolRegistry, *agent.Loop) {
	reg := newToolRegistry(cfg, agentRegistryOptions())
	agent.EnsureMVPExecutors(model, baseURL, apiKey)
	agent.RegisterAuxExecutors(cfg) // after MVP so named aux models can override executor slots
	loop := agent.NewLoopWithRegistry(apiKey, model, baseURL, reg)
	return reg, loop
}

// yoloAckPath returns the path to the one-time risk acknowledgement file.
func yoloAckPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".antisthenes", "management_accepts_the_risk")
}

// ensureYoloAck prompts the user (only on first --yolo use) and creates the ack file on "yes".
// Returns true if the user has acknowledged (file exists or just created).
// Aborts the process on "no" or after repeated bad input.
func ensureYoloAck() bool {
	ackFile := yoloAckPath()
	if ackFile == "" {
		fmt.Fprintln(os.Stderr, "error: could not determine home directory for yolo acknowledgement")
		os.Exit(1)
	}

	if _, err := os.Stat(ackFile); err == nil {
		return true // already acknowledged
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintln(os.Stderr, "THIS IS DANGEROUS, PLEASE CONFIRM THAT YOU UNDERSTAND THAT YOU ARE RESPONSIBLE FOR THE POSSIBILITY OF LOST DATA")
		fmt.Fprint(os.Stderr, "Type 'yes' to continue or 'no' to abort: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "yes":
			dir := filepath.Dir(ackFile)
			_ = os.MkdirAll(dir, 0o700)
			_ = os.WriteFile(ackFile, []byte("Managements_accepts_the_risk=1\n"), 0o600)
			return true
		case "no":
			fmt.Fprintln(os.Stderr, "Aborting.")
			os.Exit(1)
		default:
			fmt.Fprintln(os.Stderr, "please type 'yes'.")
		}
	}
}

// tryRunOneShot checks os.Args for --prompt, -P, or --prompt-file and runs
// the one-shot non-interactive path if present. Returns true if handled
// (caller should return from main to avoid falling through to TUI).
// --yolo anywhere enables blanket approval for this run (DANGEROUS).
// When ANTISTHENES_AGENT_CONTEXT is set (TUI or Telegram session parent),
// --yolo is forcibly disabled to prevent the model from bypassing approvals
// by shelling out to a child oneshot process.
func tryRunOneShot(args []string, cfg config.Config) bool {
	yolo := false
	for _, arg := range args {
		if arg == "--yolo" {
			yolo = true
			break
		}
	}

	if yolo && os.Getenv("ANTISTHENES_AGENT_CONTEXT") != "" {
		fmt.Fprintln(os.Stderr, "warning: --yolo ignored (running under TUI/agent context; blocked to prevent bypass)")
		yolo = false
	}

	if yolo {
		ensureYoloAck()
	}

	for i, arg := range args {
		switch {
		case arg == "--prompt-file" && i+1 < len(args):
			prompt, err := readPromptFile(args[i+1])
			if err != nil {
				exitOneShotError(err)
			}
			runOneShot(prompt, cfg, yolo)
			return true
		case (arg == "--prompt" || arg == "-P") && i+1 < len(args):
			prompt, err := resolveOneShotPrompt(args[i+1])
			if err != nil {
				exitOneShotError(err)
			}
			runOneShot(prompt, cfg, yolo)
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

func runOneShot(prompt string, cfg config.Config, yolo bool) {
	ep := cfg.GetActiveEndpoint()

	oneshotTools := os.Getenv("ANTISTHENES_ONESHOT_TOOLS")

	var loop *agent.Loop
	if oneshotTools != "" {
		// Isolated registry for this oneshot only
		tools := strings.Split(oneshotTools, ",")
		reg := newRestrictedOneShotRegistry(cfg, tools)
		agent.EnsureMVPExecutors(ep.Model, ep.BaseURL, ep.APIKey)
		loop = agent.NewLoopWithRegistry(ep.APIKey, ep.Model, ep.BaseURL, reg)
	} else {
		_, loop = newDefaultRegistryAndLoop(ep.APIKey, ep.Model, ep.BaseURL, cfg)
	}

	// Wire oneshot approval.
	// --yolo: blanket approve everything (DANGEROUS, opt-in only).
	// Blocked when ANTISTHENES_AGENT_CONTEXT is set (TUI/Telegram parent) to prevent bypass via shell/delegate.
	// Otherwise fall back to ApprovalsWithoutConfirm map (or deny).
	loop.SetApprovalHandler(func(req agent.ApprovalRequest) (bool, agent.ApprovalLevel) {
		if yolo {
			return true, agent.ApprovalOnce
		}
		if cfg.ApprovalsWithoutConfirm != nil && cfg.ApprovalsWithoutConfirm[req.Tool] {
			return true, agent.ApprovalOnce
		}
		return false, agent.ApprovalOnce
	})

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

// newRestrictedOneShotRegistry builds a minimal registry from ANTISTHENES_ONESHOT_TOOLS.
// This registry is completely independent of the global one.
func newRestrictedOneShotRegistry(cfg config.Config, names []string) *agent.ToolRegistry {
	reg := agent.NewToolRegistry()
	agent.ConfigureHTTPFetch(reg, cfg.ResolvedHTTPUserAgent())

	for _, n := range names {
		switch strings.TrimSpace(n) {
		case "nmap_scan":
			agent.RegisterNmapTools(reg, true)
		case "network_status":
			agent.RegisterNetworkTools(reg, true)
		case "tmux_attach", "tmux_send", "tmux_capture":
			agent.RegisterTmuxTools(reg, true)
		case "ansible_check":
			agent.RegisterAnsibleTools(reg, true)
		case "install_tool":
			agent.RegisterInstallTool(reg, true)
		}
	}
	return reg
}

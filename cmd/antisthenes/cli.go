package main

import (
	"fmt"
	"os"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	"github.com/nanami/antisthenes/internal/mcp"
	"github.com/nanami/antisthenes/internal/memory"
	"github.com/nanami/antisthenes/internal/skills"
)

// handleSubcommand checks os.Args[1] for known subcommands and executes them.
// Returns true if a subcommand was handled (caller should return from main).
func handleSubcommand(args []string, cfg config.Config) bool {
	if len(args) <= 1 {
		return false
	}
	switch args[1] {
	case "version":
		fmt.Printf("Antisthenes %s\n", version)
		return true
	case "index":
		if err := skills.GenerateIndex("."); err != nil {
			fmt.Println("Error generating index:", err)
			os.Exit(1)
		}
		fmt.Println("skills/index.json regenerated successfully.")
		return true
	case "config":
		fmt.Printf("%+v\n", cfg)
		return true
	case "sessions":
		store, err := memory.NewStore(cfg.DBPath)
		if err != nil {
			fmt.Println("Error opening store:", err)
			os.Exit(1)
		}
		defer store.Close()
		sessions, err := store.ListSessions(20)
		if err != nil {
			fmt.Println("Error listing sessions:", err)
			os.Exit(1)
		}
		fmt.Println("Recent sessions:")
		for _, s := range sessions {
			fmt.Println("  ", s)
		}
		return true
	case "mcp":
		fmt.Println("Starting Antisthenes MCP server (stdio)...")
		cfg := config.Load()
		reg := agent.NewToolRegistry()
		agent.RegisterNmapTools(reg, cfg.NmapOn())
		agent.RegisterNetworkTools(reg, cfg.NetworkStatusOn())
		srv := mcp.NewServer(reg)
		if err := srv.Run(); err != nil {
			fmt.Println("MCP server error:", err)
			os.Exit(1)
		}
		return true
	case "--help":
		fallthrough
	case "-h":
		fmt.Print(`Antisthenes - Minimal AI Agent

Usage:
  antisthenes                  Launch interactive TUI
  antisthenes --prompt "text"  Run one-shot query (pipeable output)
  antisthenes -P "text"        Short form of --prompt
  antisthenes -P -             Read prompt from stdin
  antisthenes -P @file.txt     Read prompt from file
  antisthenes --prompt-file f  Read prompt from file

Subcommands:
  version   Print version
  index     Regenerate skills/index.json
  config    Show current configuration
  sessions  List recent sessions
  mcp       Start MCP server on stdio
  model     Configure model endpoints

Examples:
  antisthenes -P "What is 2+2?"
  echo "Summarise this log" | antisthenes -P -
  antisthenes -P @prompt.txt
  antisthenes --prompt-file prompt.txt
`)
		return true
	case "model":
		configureModel()
		return true
	}
	return false
}

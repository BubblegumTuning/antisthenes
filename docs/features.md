# Features

**Updated**: 2026-07-15 / v0.1.5

## Agent Loop
Streaming chat completion with automatic tool execution and multi-turn tool use (`RunStream` + `RunWithTools` with tool recursion and robust accumulation).

## Memory
SQLite + FTS5 storage for sessions, messages, and scheduled tasks. Automatic session resumption. Nudges supported.

## Skills
Directory-based skills discovered at runtime with `SKILL.md` files. Lazy loading and index generation (`skills/index.json`). Runtime `create_skill`.

## TUI
Rich terminal interface built with Bubble Tea (`internal/tui/` split into focused files). TUI rebuild complete per DESIGN-TUI.md (phases 1–8): layout, status, thinking row, fixed-height edit box, configurable input history, chat rendering with `[OK]`/`[ERROR]` tool results, cron/gateway integration, colors/themes, approval modals, `/tools` and `/clear-history`, resize hardening. Inline markdown MVP for assistant messages. Optional `/tmux` chat-area pane. Cron disabled by default in TUI. One-shot mode unaffected. Slash commands: see [docs/tui.md](tui.md).

## Non-interactive Execution
`--prompt` / `-P` mode produces clean stdout output suitable for scripts, cron, and piping. Accepts inline text, `-` (stdin), `@file`, or `--prompt-file`.

## Configuration
JSON-driven endpoint management supporting multiple providers (local llama.cpp, xAI, OpenAI-compatible). Additional settings for TUI, debug, paths, nmap, network status, gateway, and approvals.

## Additional
- MCP server/client for tool exposure and remote calls. `antisthenes mcp` serves a dynamic `tools/list` that introspects the registered tool registry.
- Cron/scheduler for persistent scheduled tasks (disabled by default in interactive TUI via `cron_enabled: false`; right status slot reserved for notifications when enabled).
- Gateway abstraction for multi-channel (e.g. Telegram).
- Context compression and work summaries.
- Policy/approval system for tools.
- Installable CLI catalog (`tool_status` / `install_tool`) and constrained `nmap_scan`.
- Persistent tmux sessions (local + registered SSH hosts) with TUI pane.
- Version reporting and debug logging.

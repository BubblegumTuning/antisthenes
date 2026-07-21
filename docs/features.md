# Features

**Updated**: 2026-07-21 / v0.3.2

## Agent loop

Streaming chat completion with automatic tool execution and multi-turn tool use (`RunStream` and `RunWithTools` with tool recursion).

## Memory

SQLite + FTS5 storage for sessions (with optional titles), messages, and scheduled tasks. Session resumption and nudges are supported. Clearing a session removes both messages and FTS rows. Titles are generated asynchronously from the first user message (aux model with role `title` when configured, else a local heuristic).

## Skills

Directory-based skills discovered at runtime via `SKILL.md` files. Lazy loading and index generation (`skills/index.json`). Runtime `create_skill` is available.

## TUI

Terminal UI built with Bubble Tea. Includes chat viewport, thinking row, fixed-height edit box, configurable input history, tool result markers, colors/themes, approval modals, optional `/tmux` pane, and mouse drag-select copy. Assistant markdown (headings, lists, fenced code, inline styles) when `markdown_enabled` is true. Cron is disabled by default in the TUI.

Slash commands: [docs/tui.md](tui.md).

## Iterative builds

- `/iterative` — clarify a goal, optional supervised mode (`y/N`), confirm a plan, then run an **async** multi-phase Plan → Execute → Review loop (TUI stays responsive; cancel / Ctrl+C interrupts).
- On confirm: structured design and definition-of-done files are written under the target directory from the confirmed plan.
- **PER**: Plan, Execute, and Review run as separate delegated phases. Unsupervised mode may re-plan after retries; total Execute phases are capped by `config.iterative.max_iterations`. Supervised mode plans first, then waits for an executor choice before Execute → Review. Review reports status with `PER_STATUS: DONE`, `RETRY`, or `FAILED`. Artifacts include `per_plan.md`, `per_log.txt`, and `per_done.signal`.
- Context guidance thresholds come from `config.iterative` (`context_remind_percent`, `context_summary_percent`, `max_iterations`).
- While executing, progress from the work log streams into the chat and thinking row.
- `/build <task>` — faster scaffold path using the main agent loop (async).
- Concurrent builds: one `/iterative` job per chat window.

See also `skills/iterative_per/SKILL.md` and [docs/tui.md](tui.md).

## Non-interactive execution

`--prompt` / `-P` produces clean stdout for scripts and pipes. Accepts inline text, `-` (stdin), `@file`, or `--prompt-file`.

## Configuration

JSON-driven multi-endpoint setup (local OpenAI-compatible servers, xAI, and similar). TUI, debug, paths, nmap, gateway, approvals, and iterative options are documented in [configuration.md](configuration.md).

## Additional

- MCP server/client (`antisthenes mcp` exposes a dynamic `tools/list` from the tool registry)
- Cron/scheduler for persistent scheduled tasks (`cron_enabled`; off by default in the TUI)
- Gateway abstraction for external channels (e.g. Telegram)
- Context compression and work summaries
- Policy/approval for sensitive tools
- Installable CLI catalog (`tool_status` / `install_tool`) and optional constrained `nmap_scan`
- Persistent tmux sessions (local and registered SSH hosts) with optional TUI pane
- Version reporting and optional debug logging

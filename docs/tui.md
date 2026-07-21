# TUI Reference

**Updated**: 2026-07-21 / v0.3.2

Interactive mode (`./antisthenes`) launches a Bubble Tea TUI.

## Slash Commands

Type `/` in the edit box to see matching commands. Submit with Enter.

| Command | Description |
|---------|-------------|
| `/help` | Full command reference in the chat viewport |
| `/clear` | Clear context (y/n confirmation modal) |
| `/compress` | Stub tool results, keep first message and last 20 messages |
| `/dump-summary` | Write work summary to a temp file, reset context, auto-reload prompt |
| `/iterative` | Clarify a goal; optional supervised mode (y/N); on confirm async multi-phase **multi-cycle** PER (Plan→Execute→Review; re-Plan on RETRY; Execute cap = `config.iterative.max_iterations`). Supervised: Plan then SHIM brief + executor pick before multi Execute→Review (no re-Plan). One flow per chat window. Work-log progress streams into that window’s chat + active thinking row. Cancel/Ctrl+C interrupts the active window’s job. |
| `/build <task>` | Forced autonomous iterative build with goal + definition of done (main agent loop, async) |
| `/theme green` \| `/theme amber` | Apply built-in phosphor palette (saved to `config.json`) |
| `/tools` | List registered agent tools with descriptions |
| `/copy` / `/copy visible` | Copy full chat or visible lines (clipboard, OSC52 over SSH, or temp file) |
| `/mouse on` \| `off` | App mouse on (default): wheel scrolls chat, drag selects and copies. Off: native terminal selection; keyboard scroll only |
| `/clear-history` | Wipe file-backed Up/Down input history for **all** windows |
| `/tmux on` \| `off` \| `refresh` \| `host` \| `session` \| `status` | Toggle chat-area tmux pane; pick host/session |
| `/new_session` | Open a new session in window slot 3–9 |
| `/exit` | Quit (prints resume command) |

## Input

Multi-line edit box (`edit_height` in `config.json`, default 3). Height stays fixed while typing, on send, and when the thinking spinner is active.

- **Enter** — send message
- **Alt+Enter** — newline (Shift+Enter proxy)
- **Up/Down** — input history (when `input_history_enabled: true`); persisted to `work_dir/input_history.json` (or `input_history_path`) across sessions
- **`/` prefix** — slash command hints in the padding area above the edit box
- **Tab** — complete slash commands (longest common prefix, then cycle; Tab is never inserted as a character)
- **/clear-history** — wipe file-backed history for all windows

## Chat formatting

When `markdown_enabled` is true (default), **assistant** replies render markdown: ATX headings, unordered lists (`-`/`*`/`+`), fenced code (``` / ~~~), plus inline `**bold**` / `*italic*` / `` `code` `` / `~~strike~~` / `[label](url)` (styled, not clickable). Styles follow the active palette (`/theme` and `colors` in config): bold→assistant, heading→title, code→tool_result on window_empty, links→status. User and tool lines stay literal. `/copy` always writes raw source text. Set `"markdown_enabled": false` in `config.json` to disable.

## Thinking Row

A dedicated 3-line status slot below the chat viewport shows a bordered spinner while the agent runs. It clears fully when the response arrives (no ghost borders or layout shift).

## Keys

- **Ctrl+C / Esc** — quit normally. When the **active** window’s `/iterative` job is **executing**, first Ctrl+C (or saying `cancel`) cancels that window’s worker context and returns it to idle without quitting; background windows keep running. Ctrl+C again (no active-window job executing) quits.
- **Arrows, PgUp/PgDn, Home/End, mouse wheel** — scroll chat viewport (mouse wheel when `/mouse on`, default)
- **Left-drag in chat** — select text and copy on release (clipboard locally, OSC52 over SSH). Toggle with `/mouse on|off`.
- **Ctrl+Y / Ctrl+Shift+C** — copy full chat (`/copy` still available)

## Background Activity

- Cron is **disabled by default** (`cron_enabled: false`). When enabled, results appear in the right status bar via model messages only (no raw terminal output from background goroutines).
- Gateway (e.g. Telegram) routes inbound messages through the TUI model when configured.
- Tool approvals and `/clear` confirmation appear as centered modals.

## Theming

All chat and chrome colors are driven from `colors` in `config.json`. Built-in themes: `/theme green`, `/theme amber`. See [docs/configuration.md](configuration.md).

## Multi-Window

`/new_session` opens additional session windows (slots 3–9). Each window has its own input history and session state.
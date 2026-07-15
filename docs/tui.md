# TUI Reference

**Updated**: 2026-07-15 / v0.1.5

Interactive mode (`./antisthenes`) launches a Bubble Tea TUI. Layout, theming, and integration follow [DESIGN-TUI.md](../DESIGN-TUI.md) (phases 1–8 complete).

## Slash Commands

Type `/` in the edit box to see matching commands. Submit with Enter.

| Command | Description |
|---------|-------------|
| `/help` | Full command reference in the chat viewport |
| `/clear` | Clear context (y/n confirmation modal) |
| `/compress` | Stub tool results, keep first message and last 20 messages |
| `/dump-summary` | Write work summary to a temp file, reset context, auto-reload prompt |
| `/iterative` | Clarify a goal via conversation, then autonomous build on confirmation |
| `/build <task>` | Forced autonomous iterative build with goal + definition of done |
| `/theme green` \| `/theme amber` | Apply built-in phosphor palette (saved to `config.json`) |
| `/tools` | List registered agent tools with descriptions |
| `/clear-history` | Wipe Up/Down input history for the current window |
| `/tmux on` \| `off` \| `refresh` \| `host` \| `session` \| `status` | Toggle chat-area tmux pane; pick host/session |
| `/new_session` | Open a new session in window slot 3–9 |
| `/exit` | Quit (prints resume command) |

## Input

Multi-line edit box (`edit_height` in `config.json`, default 3). Height stays fixed while typing, on send, and when the thinking spinner is active.

- **Enter** — send message
- **Alt+Enter** — newline (Shift+Enter proxy)
- **Up/Down** — input history (when `input_history_enabled: true`)
- **Tab / `/` prefix** — slash command hints in the padding area above the edit box

## Chat formatting

When `markdown_enabled` is true (default), **assistant** replies render inline markdown: `**bold**`, `*italic*`, `` `code` ``, `~~strike~~`, and `[label](url)` (styled, not clickable). User and tool lines stay literal. `/copy` always writes raw source text. Set `"markdown_enabled": false` in `config.json` to disable. See DESIGN-TUI.md for planned full-markdown and theme-aware styling.

## Thinking Row

A dedicated 3-line status slot below the chat viewport shows a bordered spinner while the agent runs. It clears fully when the response arrives (no ghost borders or layout shift).

## Keys

- **Ctrl+C / Esc** — quit (first Ctrl+C during an iterative run interrupts the current run; second quits)
- **Arrows, PgUp/PgDn, Home/End, mouse wheel** — scroll chat viewport

## Background Activity

- Cron is **disabled by default** (`cron_enabled: false`). When enabled, results appear in the right status bar via model messages only (no raw terminal output from background goroutines).
- Gateway (e.g. Telegram) routes inbound messages through the TUI model when configured.
- Tool approvals and `/clear` confirmation appear as centered modals.

## Theming

All chat and chrome colors are driven from `colors` in `config.json`. Built-in themes: `/theme green`, `/theme amber`. See [docs/configuration.md](configuration.md).

## Multi-Window

`/new_session` opens additional session windows (slots 3–9). Each window has its own input history and session state.
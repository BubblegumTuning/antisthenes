# Configuration

**Updated**: 2026-07-21 / v0.3.2

Copy `config.example.json` to `config.json`.

## Endpoints

Each endpoint defines:
- `name`
- `model`
- `base_url`
- `api_key` (optional for local endpoints)

Multi-endpoint support (e.g. "local" + "xai"). Set `active_endpoint`.

Example local endpoint:
```json
{
  "name": "local",
  "model": "Qwen3.6-MTP-27B-UD-Q4_K_XL.gguf",
  "base_url": "http://localhost:8001/v1"
}
```

## Other Settings
- `agent_name` — display name for the agent in TUI chat (defaults to "Antisthenes" when empty)
- `show_thinking`
- `max_tokens`
- `debug_logging` — writes diagnostic output to `log/debug.log` when true
- `db_path`, `work_dir` — SQLite database and working directory. Empty values default to durable paths under `~/.antisthenes/` (`antisthenes.db` and `work/`). Overrides (highest first): `ANTISTHENES_DB_PATH` (alias `ANTISTHENES_DB`) / `ANTISTHENES_WORK_DIR` → non-empty config.json → `ANTISTHENES_DATA_DIR` (derives both when config paths empty) → home defaults. `$XDG_DATA_HOME/antisthenes` is used when `XDG_DATA_HOME` is set.
- `aux_models` — optional list of secondary OpenAI-compatible endpoints for cheap/async work. Each entry: `name`, `model`, `base_url`, optional `api_key`, optional `roles` (e.g. `title`, `summarize`, `delegate`). Role `title` is used for async session title generation; names are also registered as delegate executors. Tools: `list_aux_models`, `complete_with_aux`.
- `edit_height`, `input_history_enabled` (default true), `input_history_size` (default 50), `input_history_path` (optional; default `work_dir/input_history.json` for file-backed Up/Down history), `auto_scroll`, `show_full_tool_dumps`, `markdown_enabled` (default true — inline markdown in assistant chat; set false to show raw `**` etc.), `clear_without_confirm` (skip `/clear`/`/new` prompts when true), `approvals_without_confirm` (per-tool map — set a tool to `true` to skip its approval modal; set back to `false` to restore prompts), `cron_enabled`, `nmap_enabled` (default true — when false, `nmap_scan` is not registered on the agent registry), `network_status_enabled` (default false — when true, `network_status` is registered for read-only local IP/gateway/DNS inspection)
- `colors` — TUI palette (lipgloss tokens: 0–255, hex, or ANSI names). Keys: `user`, `assistant`, `assistant_thinking`, `tool_call`, `tool_result`, `input_border`, `thinking_border`, `status`, `title`, `error`, `nudge`, `compression`, `modal_border`, `window_active`, `window_inactive`, `window_empty`, `dim`, `empty_chat`. Omit any key to use the amber/green defaults from `config.example.json`.
- Built-in themes in the TUI: `/theme green` (green phosphor), `/theme amber` (amber phosphor). Applies immediately and saves to `config.json`.
- `gateway.telegram_enabled`, `gateway.telegram_token`, `gateway.telegram_chat_id` (Telegram adapter; routes via agent loop + TUI notifications when enabled)
- `xai_oauth` — `access_token`, `refresh_token`, `expires_at` (config storage only; OAuth flow not yet wired into CLI/agent)
- `iterative` — `/iterative` worker seed guidance (omitted keys use defaults):
  - `context_remind_percent` (default **55**) — when context usage exceeds this %, write a concise progress summary to the work log
  - `context_summary_percent` (default **60**) — when context usage exceeds this %, force a fuller summary/refresh before more edits (clamped to at least the remind percent)
  - `max_iterations` (default **40**) — hard cap on **Execute** phases across multi-cycle PER (re-Plan cycles continue until DoD, FAILED, cancel, or this many Executes)
- `http_user_agent` — default `User-Agent` for `http_fetch` when the tool call does not set one. Built-in default: `Mozilla/4.0 (compatible; MSIE 5.5; Windows 98; 56K Modem; Dial-up; Netscape 4.7 envy)` (avoids Go’s bare `Go-http-client/1.1`, which some sites such as Wikipedia reject). Override in `config.json`, or pass `headers.User-Agent` on a single call.

The file is git-ignored. Never commit real credentials.
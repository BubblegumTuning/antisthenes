# Configuration

**Updated**: 2026-07-15 / v0.1.5

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
- `db_path`, `work_dir` — SQLite database and working directory (empty = sensible defaults under system temp)
- `edit_height`, `input_history_enabled` (default true), `input_history_size` (default 50), `auto_scroll`, `show_full_tool_dumps`, `markdown_enabled` (default true — inline markdown in assistant chat; set false to show raw `**` etc.), `clear_without_confirm` (skip `/clear`/`/new` prompts when true), `approvals_without_confirm` (per-tool map — set a tool to `true` to skip its approval modal; set back to `false` to restore prompts), `cron_enabled`, `nmap_enabled` (default true — when false, `nmap_scan` is not registered on the agent registry), `network_status_enabled` (default false — when true, `network_status` is registered for read-only local IP/gateway/DNS inspection)
- `colors` — TUI palette (lipgloss tokens: 0–255, hex, or ANSI names). Keys: `user`, `assistant`, `assistant_thinking`, `tool_call`, `tool_result`, `input_border`, `thinking_border`, `status`, `title`, `error`, `nudge`, `compression`, `modal_border`, `window_active`, `window_inactive`, `window_empty`, `dim`, `empty_chat`. Omit any key to use the amber/green defaults from `config.example.json`.
- Built-in themes in the TUI: `/theme green` (green phosphor), `/theme amber` (amber phosphor). Applies immediately and saves to `config.json`.
- `gateway.telegram_enabled`, `gateway.telegram_token`, `gateway.telegram_chat_id` (Telegram adapter; routes via agent loop + TUI notifications when enabled)
- `xai_oauth` — `access_token`, `refresh_token`, `expires_at` (config storage only; OAuth flow not yet wired into CLI/agent)

The file is git-ignored. Never commit real credentials.

See current_state.md and DESIGN.md for full details.
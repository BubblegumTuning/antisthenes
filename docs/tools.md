# Available Tools

**Updated**: 2026-07-21 / v0.3.2

## Core Tools
- `bash`, `run_command` (policy-protected; cwd, env, timeout, background mode)
- `wait_job`, `list_background_jobs` (manage background run_command jobs)
- `list_dir` (eza → ls), `create_dir` (mkdir; policy-protected), `read_file` (bat → cat; `encoding=base64` for binary), `write_file` (text or base64), `search_files` (rg → ast-grep → grep), `patch` (unified diff + string replace)
- `delete_file`, `move_file`, `copy_file`, `file_stat`, `chmod` (relative paths for mutations; delete respects rm -rf approval)
- `approve_tool`, `reset_approvals` (manage policy approvals for sensitive commands)
- `find_files` (fd → find), `fuzzy_find` (fd+fzf), `cd_path` / `resolve_path` (zoxide)
- `git_status`, `git_log`, `git_add`, `git_commit`, `git_checkout`, `git_branch`, `git_show`, `git_diff` (delta → git diff)
- `get_current_time`, `echo`, `get_env`
- `list_aux_models`, `complete_with_aux` (configured `aux_models` only)
- `http_fetch` (GET/POST, headers, body, timeout; http/https only; default User-Agent from `config.http_user_agent`, overridable per call via `headers`)
- `schedule_task`, `list_tasks`, `cancel_task` (registered when `cron_enabled: true` in TUI; also available in one-shot mode with a nil scheduler)
- `list_processes`, `kill_process` (kill always requires approval; blocks pid 0/1 and self)

## Installable CLI Tools
- `tool_status` — Check whether installable dependencies are on PATH (`tool=all`, `prefcli`, or a specific id like `rg`, `nmap`)
- `install_tool` — Install one or more tools via the detected package manager (Alpine apk, Debian apt, RedHat dnf/yum); venv+pip for ansible. Requires approval.
- `modern_cli_status`, `install_modern_cli` — **Deprecated shims** delegating to `tool_status` / `install_tool` for backward compatibility

### Catalog (install_tool ids)
| ID | Package (typical) | Used by |
|----|-------------------|---------|
| `rg` / `ripgrep` | ripgrep | `search_files` |
| `fd` | fd / fd-find | `find_files`, `fuzzy_find` |
| `bat` | bat | `read_file` |
| `eza` | eza | `list_dir` |
| `fzf` | fzf | `fuzzy_find` |
| `ast-grep` | ast-grep | `search_files` fallback |
| `zoxide` | zoxide | `cd_path`, `resolve_path` |
| `delta` | git-delta | `git_diff` |
| `nmap` | nmap | `nmap_scan` |
| `ansible` | venv + pip | ansible tools |
| `goban-cli` | manual only | `goban_create_ticket` |

Special install selectors:
- `prefcli_missing` — install only missing fd/bat/eza/fzf/ast-grep/zoxide/delta (same scope as legacy `install_modern_cli`)
- `all_missing` — install all missing pkgmgr tools plus ansible venv when absent

## Agent & Skill Tools
- `list_skills`, `load_skill`, `create_skill`
- `delegate_task`
- `dump_work_summary`, `load_work_summary`

## Integration Tools
- `mcp_call` — Call tools on remote MCP servers (registered in TUI and one-shot paths only, not in standalone `antisthenes mcp` server). `server` may be a full command line (e.g. `"./antisthenes mcp"`) or a binary with optional `args` array.
- `mcp_list_tools` — List remote MCP tools (name, description, inputSchema); same `server`/`args` as `mcp_call`. Use before `mcp_call` when the catalog is unknown.
- `goban_create_ticket` — Create a ticket via `goban-cli` (requires goban-cli on PATH)


## Tmux Persistent Sessions (Phase 0–2)
- `tmux_attach_or_create` / `tmux_attach` — Create or reuse one long-lived session (interactive shell; default `antisthenes-persist` or host default)
- `tmux_send` — Type keys + Enter; **creates session on demand** if missing (approval/safety parity with `bash`)
- `tmux_capture` — Pane output; `format=llm` (default compact), `human`, or `raw`; on-demand create if missing
- `tmux_list_sessions` / `tmux_kill_session` — list or kill sessions
- `tmux_register_host` — Register SSH host (name, host, user, key_path; optional session_name, port, validate)
- `tmux_list_hosts` — List registered hosts (`tmux_hosts.json`)
- Optional `host=` on core tools: registered alias via SSH; omit/`local`/`localhost` = local
- Local: `tmux` **3.7+** on PATH (required; el10 `next-3.4` package is unsupported). Remote: `ssh` + remote `tmux` 3.7+.
- **TUI**: `/tmux on` opens a chat-area pane (above thinking/status) with periodic capture; `/tmux host`, `/tmux session`, `/tmux refresh`, `/tmux off`.

## Network Tools
- `network_status` — Read local interface IPs, default gateways, and DNS. Registered when `network_status_enabled: true` in config (default false). Read-only; no approval required. Use `detail=full` for resolvectl/ip enrichment on Ubuntu/Rocky. See `skills/network-status/SKILL.md`.
- `nmap_scan` — Constrained nmap scan (ping/quick/ports/service presets). Registered when `nmap_enabled: true` in config (default). Always requires approval. Logs to `logs/nmap/`. Install nmap first via `install_tool`. See `skills/nmap/SKILL.md`.

## Skills (lazy-loaded)
- `network-status` — Local network inspection via `network_status`
- `nmap` — Network scanning workflow and safety rules
- `system-tools` — Unified `tool_status` / `install_tool` workflow for rg, nmap, ansible, prefcli tools

## Ansible Tools (fully implemented)
- `ansible_check`
- `ansible_run_playbook`
- `ansible_generate_playbook`

See `internal/agent/ansible_tools.go` and the tool registry. `ansible_check` reports availability and points to `install_tool` with `tool=ansible` when missing; run logs + executes (approval recommended); generate scaffolds + syntax checks. Policy applies to sensitive operations.

**MCP exposure:** `./antisthenes mcp` serves `tools/list` dynamically from whatever is registered on that server instance. The default MCP server exposes the base registry plus config-gated nmap/network tools. Agent paths (TUI, `--prompt`) add `mcp_call`, `mcp_list_tools`, cron tools, and aux-model tools via the shared registry builder (`newToolRegistry` / `RegistryOptions`).

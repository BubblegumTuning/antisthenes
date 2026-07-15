# System CLI Tools (install_tool)

Use this skill when a task needs a binary that may not be installed: ripgrep, nmap, fd, bat, ansible, or other catalog entries.

## Core tools

| Tool | Purpose |
|------|---------|
| `tool_status` | See what is on PATH and what package/install method applies |
| `install_tool` | Install one or more ids (requires user approval) |

## Workflow (check → install → use)

1. Run `tool_status` — full catalog (`tool=all`) or a specific id (`tool=rg`)
2. If missing and installable, call `install_tool` with the canonical id
3. Re-check with `tool_status` if install reported partial failure
4. Use the domain tool (`search_files`, `nmap_scan`, `ansible_run_playbook`, etc.)

## Common ids

| id | Installs via | Used by |
|----|--------------|---------|
| `rg` / `ripgrep` | package manager | `search_files` |
| `nmap` | package manager | `nmap_scan` |
| `fd`, `bat`, `eza`, `fzf`, `ast-grep`, `zoxide`, `delta` | package manager | modern CLI file/git tools |
| `ansible` | `.ansible-venv` + pip | ansible tools |
| `goban-cli` | manual only | `goban_create_ticket` |

## Batch selectors

- `prefcli_missing` — fd/bat/eza/fzf/ast-grep/zoxide/delta only (legacy `install_modern_cli` scope)
- `all_missing` — every missing pkgmgr tool plus ansible venv when absent

## Ansible

1. `ansible_check` — readiness and directory scaffolding
2. If missing: `install_tool({tool: "ansible"})` (creates `.ansible-venv`, pip installs ansible)
3. `ansible_generate_playbook` / `ansible_run_playbook`

Prefer the venv install over ad-hoc global pip. Playbooks live in `playbooks/`; logs in `logs/ansible/`.

## Package managers

Install commands are chosen from `/etc/os-release`:

- Alpine → `apk`
- Debian/Ubuntu → `apt-get`
- RHEL/Fedora/Rocky/Alma → `dnf` (or `yum`)

## Rules

- Always get approval before `install_tool` or `nmap_scan`.
- Never run package installs via raw `bash` when `install_tool` covers the dependency.
- Report `manual only` tools (e.g. goban-cli) with upstream install guidance, not invented packages.
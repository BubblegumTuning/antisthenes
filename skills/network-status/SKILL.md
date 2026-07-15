# Local Network Status

Use this skill when the user asks about **this machine's** IP addresses, default gateway, or DNS configuration.

## Tool

`network_status` — read-only; no approval required.

Requires `network_status_enabled: true` in `config.json` (default false). When disabled, the tool is not registered.

## Usage

1. Default (`detail=brief`): Go stdlib + `/proc/net/route` + `/etc/resolv.conf`. Works on Ubuntu, Rocky Linux, and Alpine without extra packages.
2. Enriched (`detail=full`): Also tries `resolvectl status` and `ip -json route` when available (Ubuntu/Rocky with iproute2). Gracefully skips on Alpine/BusyBox.

Optional args:
- `interface` — filter to one iface (e.g. `eth0`)
- `include_loopback` — include `lo` (default false)

## When to use something else

- **Remote host discovery or port scans** → `nmap_scan` (requires approval; see `skills/nmap/SKILL.md`)
- **Changing network settings** → not supported yet; use explicit user-approved `run_command` only when policy allows

## Safety

Read-only. Does not modify interfaces, routes, or resolver configuration.
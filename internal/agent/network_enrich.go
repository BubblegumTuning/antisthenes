package agent

import (
	"encoding/json"
	"os/exec"
	"strings"
)

func enrichNetworkStatus(snap *networkSnapshot, detail string) {
	if strings.ToLower(strings.TrimSpace(detail)) != "full" {
		return
	}
	enrichDNSFromResolvectl(snap)
	enrichGatewaysFromIP(snap)
}

func enrichDNSFromResolvectl(snap *networkSnapshot) {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		snap.Warnings = append(snap.Warnings, "resolvectl not available")
		return
	}
	out, err := runCommandOutput("resolvectl", "status")
	if err != nil {
		snap.Warnings = append(snap.Warnings, "resolvectl status: "+err.Error())
		return
	}

	var servers []string
	var current string
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		idx := strings.Index(trim, ":")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		switch key {
		case "dns servers":
			for _, s := range strings.Fields(val) {
				servers = append(servers, strings.TrimSuffix(s, "#"))
			}
		case "current dns server":
			current = val
		}
	}
	if len(servers) == 0 {
		return
	}

	snap.DNS.Nameservers = servers
	snap.DNS.Source = "resolvectl status"
	if current != "" {
		snap.DNS.Options = append([]string{"current_dns_server=" + current}, snap.DNS.Options...)
	}
}

func enrichGatewaysFromIP(snap *networkSnapshot) {
	if !isIPRoute2() {
		snap.Warnings = append(snap.Warnings, "ip -json unavailable (need iproute2, not busybox ip)")
		return
	}
	out, err := runCommandOutput("ip", "-json", "route", "show", "default")
	if err != nil {
		snap.Warnings = append(snap.Warnings, "ip -json route: "+err.Error())
		return
	}

	type routeEntry struct {
		Gateway string `json:"gateway"`
		Dev     string `json:"dev"`
	}
	var routes []routeEntry
	if err := json.Unmarshal([]byte(out), &routes); err != nil {
		snap.Warnings = append(snap.Warnings, "ip -json route parse: "+err.Error())
		return
	}

	gwByIface := make(map[string]string)
	for _, r := range routes {
		if r.Dev != "" && r.Gateway != "" {
			gwByIface[r.Dev] = r.Gateway
		}
	}
	for i := range snap.Interfaces {
		if gw, ok := gwByIface[snap.Interfaces[i].Name]; ok {
			snap.Interfaces[i].DefaultGateway = gw
		}
	}
}

func isIPRoute2() bool {
	out, err := exec.Command("ip", "-V").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "iproute2")
}

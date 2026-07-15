package agent

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

type addrInfo struct {
	CIDR   string
	Family string
	Scope  string
}

type ifaceInfo struct {
	Name           string
	Up             bool
	Loopback       bool
	MAC            string
	MTU            int
	Addrs          []addrInfo
	DefaultGateway string
}

type dnsInfo struct {
	Source      string
	Nameservers []string
	Search      []string
	Options     []string
}

type networkSnapshot struct {
	Hostname   string
	Distro     string
	Interfaces []ifaceInfo
	DNS        dnsInfo
	Warnings   []string
}

type networkStatusOptions struct {
	Interface       string
	IncludeLoopback bool
	Detail          string
}

var (
	readFileFn        = os.ReadFile
	listInterfacesFn  = net.Interfaces
	hostnameFn        = os.Hostname
	routeFilePath     = "/proc/net/route"
	resolvConfPath    = "/etc/resolv.conf"
	systemdResolvPath = "/run/systemd/resolve/resolv.conf"
)

func collectNetworkStatus(opts networkStatusOptions) networkSnapshot {
	snap := networkSnapshot{
		Distro: string(prefcli.DetectDistro()),
	}

	if host, err := hostnameFn(); err == nil {
		snap.Hostname = host
	} else {
		snap.Warnings = append(snap.Warnings, "hostname: "+err.Error())
	}

	gateways := parseDefaultGateways(readRouteTable())
	ifaces := collectInterfaces(opts, gateways)
	snap.Interfaces = ifaces
	snap.DNS = collectDNS()

	if opts.Interface != "" && len(ifaces) == 0 {
		snap.Warnings = append(snap.Warnings, fmt.Sprintf("no interface matching %q", opts.Interface))
	}

	return snap
}

func collectInterfaces(opts networkStatusOptions, gateways map[string]string) []ifaceInfo {
	raw, err := listInterfacesFn()
	if err != nil {
		return nil
	}

	filter := strings.TrimSpace(opts.Interface)
	var out []ifaceInfo
	for _, iface := range raw {
		if filter != "" && iface.Name != filter {
			continue
		}
		loopback := iface.Flags&net.FlagLoopback != 0
		if loopback && !opts.IncludeLoopback {
			continue
		}

		info := ifaceInfo{
			Name:           iface.Name,
			Up:             iface.Flags&net.FlagUp != 0,
			Loopback:       loopback,
			MTU:            iface.MTU,
			DefaultGateway: gateways[iface.Name],
		}
		if len(iface.HardwareAddr) > 0 {
			info.MAC = iface.HardwareAddr.String()
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				info.Addrs = append(info.Addrs, formatAddr(ipNet))
			}
		}
		out = append(out, info)
	}
	return out
}

func formatAddr(ipNet *net.IPNet) addrInfo {
	ip := ipNet.IP
	family := "ipv6"
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		family = "ipv4"
	}

	ones, _ := ipNet.Mask.Size()
	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)

	scope := "global"
	if ip.IsLoopback() {
		scope = "loopback"
	} else if ip.IsLinkLocalUnicast() {
		scope = "link-local"
	} else if ip.IsPrivate() {
		scope = "private"
	}

	return addrInfo{CIDR: cidr, Family: family, Scope: scope}
}

func readRouteTable() []byte {
	data, err := readFileFn(routeFilePath)
	if err != nil {
		return nil
	}
	return data
}

func parseDefaultGateways(data []byte) map[string]string {
	gateways := make(map[string]string)
	if len(data) == 0 {
		return gateways
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		iface, dest, gwHex := fields[0], fields[1], fields[2]
		if dest != "00000000" || gwHex == "00000000" {
			continue
		}
		gw, err := hexLEToIPv4(gwHex)
		if err != nil {
			continue
		}
		gateways[iface] = gw
	}
	return gateways
}

func hexLEToIPv4(hex string) (string, error) {
	if len(hex) != 8 {
		return "", fmt.Errorf("invalid gateway hex %q", hex)
	}
	var octets [4]byte
	for i := 0; i < 4; i++ {
		var b uint
		if _, err := fmt.Sscanf(hex[i*2:i*2+2], "%02x", &b); err != nil {
			return "", err
		}
		octets[i] = byte(b)
	}
	return net.IPv4(octets[3], octets[2], octets[1], octets[0]).String(), nil
}

func collectDNS() dnsInfo {
	data, err := readFileFn(resolvConfPath)
	if err != nil {
		return dnsInfo{Source: resolvConfPath + " (unreadable)"}
	}

	info := parseResolvConf(data)
	info.Source = resolvConfPath

	if usesSystemdStub(info.Nameservers) {
		if upstream, err := readFileFn(systemdResolvPath); err == nil {
			up := parseResolvConf(upstream)
			if len(up.Nameservers) > 0 {
				info = up
				info.Source = systemdResolvPath + " (systemd-resolved)"
			}
		}
	}
	return info
}

func usesSystemdStub(nameservers []string) bool {
	for _, ns := range nameservers {
		if ns == "127.0.0.53" || ns == "127.0.0.54" {
			return true
		}
	}
	return false
}

func parseResolvConf(data []byte) dnsInfo {
	var info dnsInfo
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "nameserver":
			info.Nameservers = append(info.Nameservers, fields[1])
		case "search":
			info.Search = append(info.Search, fields[1:]...)
		case "options":
			info.Options = append(info.Options, fields[1:]...)
		}
	}
	return info
}

func formatNetworkStatus(snap networkSnapshot) string {
	var b strings.Builder
	b.WriteString("network_status:\n")
	if snap.Hostname != "" {
		b.WriteString("hostname: " + snap.Hostname + "\n")
	}
	if snap.Distro != "" {
		b.WriteString("distro: " + snap.Distro + "\n")
	}

	b.WriteString("\n=== interfaces ===\n")
	if len(snap.Interfaces) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, iface := range snap.Interfaces {
			state := "down"
			if iface.Up {
				state = "up"
			}
			b.WriteString(fmt.Sprintf("%s (%s, mtu=%d", iface.Name, state, iface.MTU))
			if iface.MAC != "" {
				b.WriteString(", mac=" + iface.MAC)
			}
			b.WriteString(")\n")
			if len(iface.Addrs) == 0 {
				b.WriteString("  (no addresses)\n")
			}
			for _, addr := range iface.Addrs {
				extra := ""
				if addr.Scope != "" && addr.Scope != "global" {
					extra = " (" + addr.Scope + ")"
				}
				b.WriteString(fmt.Sprintf("  %s: %s%s\n", addr.Family, addr.CIDR, extra))
			}
			if iface.DefaultGateway != "" {
				b.WriteString("  default_gateway: " + iface.DefaultGateway + "\n")
			}
		}
	}

	b.WriteString("\n=== dns ===\n")
	if snap.DNS.Source != "" {
		b.WriteString("source: " + snap.DNS.Source + "\n")
	}
	if len(snap.DNS.Nameservers) == 0 {
		b.WriteString("nameservers: (none)\n")
	} else {
		b.WriteString("nameservers: " + strings.Join(snap.DNS.Nameservers, ", ") + "\n")
	}
	if len(snap.DNS.Search) > 0 {
		b.WriteString("search: " + strings.Join(snap.DNS.Search, " ") + "\n")
	}
	if len(snap.DNS.Options) > 0 {
		b.WriteString("options: " + strings.Join(snap.DNS.Options, " ") + "\n")
	}

	if len(snap.Warnings) > 0 {
		b.WriteString("\n=== warnings ===\n")
		for _, w := range snap.Warnings {
			b.WriteString("- " + w + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
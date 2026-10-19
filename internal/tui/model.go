package tui

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyzrex/kport/types"
)

type group struct {
	key     string
	process types.Process
	sockets []types.Process
}

func owner(p types.Process) string {
	if p.ContainerID != "" {
		return "container:" + p.ContainerID + ":" + p.StartID
	}
	return fmt.Sprintf("pid:%d:%s", p.PID, p.StartID)
}
func groupProcesses(ps []types.Process) []group {
	var gs []group
	indices := map[string]int{}
	for _, p := range ps {
		k := owner(p)
		i, ok := indices[k]
		if !ok {
			i = len(gs)
			indices[k] = i
			gs = append(gs, group{key: k, process: p})
		}
		gs[i].sockets = append(gs[i].sockets, p)
	}
	for i := range gs {
		sort.SliceStable(gs[i].sockets, func(a, b int) bool { return gs[i].sockets[a].Port < gs[i].sockets[b].Port })
	}
	return gs
}
func Exposure(addr string) string {
	host, _, e := net.SplitHostPort(addr)
	if e != nil {
		if i := strings.LastIndex(addr, ":"); i >= 0 {
			host = addr[:i]
		} else {
			host = addr
		}
	}
	host = strings.Trim(host, "[]")
	if host == "*" || host == "" || host == "0.0.0.0" || host == "::" {
		return "public"
	}
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback()) {
		return "localhost"
	}
	return "interface"
}
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = Clean(s)
	var out strings.Builder
	n := 0
	for _, r := range s {
		w := 1
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			w = 0
		} else if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7af || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe6f || r >= 0xff00 && r <= 0xff60 || r >= 0x1f300) {
			w = 2
		}
		if n+w > width {
			break
		}
		out.WriteRune(r)
		n += w
	}
	return out.String()
}
func filterGroups(gs []group, query, exposure string) []group {
	var result []group
	for _, g := range gs {
		var text strings.Builder
		fmt.Fprintf(&text, "%d %s %s %s %s %s ", g.process.PID, g.process.Name, g.process.CommandLine, g.process.Cwd, g.process.User, g.process.ContainerName)
		exposed := exposure == "all" || exposure == ""
		for _, p := range g.sockets {
			fmt.Fprintf(&text, "%d %s %s ", p.Port, p.Protocol, p.Address)
			if Exposure(p.Address) == exposure {
				exposed = true
			}
		}
		matches := true
		for _, term := range strings.Fields(strings.ToLower(query)) {
			if !strings.Contains(strings.ToLower(text.String()), term) {
				matches = false
				break
			}
		}
		if matches && exposed {
			result = append(result, g)
		}
	}
	return result
}
func sortGroups(gs []group, by string) {
	sort.SliceStable(gs, func(i, j int) bool {
		a, b := gs[i], gs[j]
		switch by {
		case "memory":
			if a.process.MemoryRSS != b.process.MemoryRSS {
				return a.process.MemoryRSS > b.process.MemoryRSS
			}
		case "project":
			if a.process.Cwd != b.process.Cwd {
				return a.process.Cwd < b.process.Cwd
			}
		case "user":
			if a.process.User != b.process.User {
				return a.process.User < b.process.User
			}
		case "name":
			if a.process.Name != b.process.Name {
				return a.process.Name < b.process.Name
			}
		case "protocol":
			if a.process.Protocol != b.process.Protocol {
				return a.process.Protocol < b.process.Protocol
			}
		}
		if a.sockets[0].Port != b.sockets[0].Port {
			return a.sockets[0].Port < b.sockets[0].Port
		}
		return a.key < b.key
	})
}
func socketSummary(g group) string {
	var ports []string
	for _, p := range g.sockets {
		ports = append(ports, strconv.Itoa(p.Port)+"/"+string(p.Protocol))
	}
	return strings.Join(ports, ", ")
}
func backspace(s string) string { _, n := utf8.DecodeLastRuneInString(s); return s[:len(s)-n] }

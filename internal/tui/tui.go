package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tyzrex/kport/types"
	"golang.org/x/term"
)

type Options struct {
	Interval                           time.Duration
	Sort                               string
	Exposure                           string
	DryRun, Force, Tree, Wait, NoColor bool
	Discover                           func() ([]types.Process, error)
	Execute                            func([]types.Process, bool) []types.KillResult
	Inspect                            func(int) (types.Process, error)
}
type snapshot struct {
	ps  []types.Process
	err error
}
type dashboard struct {
	paneOffset                                    int
	groups                                        []group
	selected                                      map[string]bool
	cursor                                        int
	query, sort, exposure, status                 string
	search, paused, details, help, copyMenu, busy bool
	confirming                                    []types.Process
	confirmForce                                  bool
	parent                                        types.Process
	results                                       []types.KillResult
	stopped                                       map[string]types.Process
}

func Run(o Options) error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("dashboard requires an interactive terminal")
	}
	restoreOutput, e := enableOutput()
	if e != nil {
		return e
	}
	defer restoreOutput()
	state, e := term.MakeRaw(fd)
	if e != nil {
		return e
	}
	fmt.Print("\x1b[?1049h\x1b[?25l\x1b[?2004h")
	defer func() { fmt.Print("\x1b[?2004l\x1b[?25h\x1b[?1049l"); _ = term.Restore(fd, state) }()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	input := make(chan string, 16)
	done := make(chan struct{})
	defer close(done)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, e := os.Stdin.Read(buf)
			if e != nil || n == 0 {
				select {
				case input <- "\x03":
				case <-done:
				}
				return
			}
			select {
			case input <- string(buf[:n]):
			case <-done:
				return
			}
		}
	}()
	updates := make(chan snapshot, 1)
	actions := make(chan []types.KillResult, 1)
	m := dashboard{selected: map[string]bool{}, sort: o.Sort, exposure: o.Exposure, stopped: map[string]types.Process{}, status: "Loading listeners..."}
	interval := o.Interval
	if interval < 250*time.Millisecond {
		interval = 250 * time.Millisecond
	}
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	refreshing := false
	lastRefresh := time.Time{}
	lastInput := time.Time{}
	var decoder decoder
	refresh := func() {
		if refreshing || m.busy || len(m.confirming) > 0 {
			return
		}
		refreshing = true
		lastRefresh = time.Now()
		go func() {
			ps, e := o.Discover()
			select {
			case updates <- snapshot{ps, e}:
			case <-done:
			}
		}()
	}
	refresh()
	width, height := 0, 0
	dirty := true
	for {
		w, h, e := term.GetSize(int(os.Stdout.Fd()))
		if e != nil {
			w, h = 100, 30
		}
		if w != width || h != height {
			width, height = w, h
			dirty = true
		}
		if dirty {
			if m.details && o.Inspect != nil {
				visible := m.visible()
				if len(visible) > 0 && m.cursor < len(visible) {
					pid := visible[m.cursor].process.PPID
					if m.parent.PID != pid {
						m.parent, _ = o.Inspect(pid)
					}
				}
			}
			m.render(o, width, height)
			dirty = false
		}
		var keys []key
		select {
		case <-sigs:
			return nil
		case chunk := <-input:
			keys = decoder.feed(chunk)
			lastInput = time.Now()
			dirty = true
		case <-tick.C:
			if time.Since(lastInput) > 80*time.Millisecond {
				keys = decoder.escape()
				if len(keys) > 0 {
					dirty = true
				}
			}
			if !m.paused && time.Since(lastRefresh) >= interval {
				refresh()
			}
		case update := <-updates:
			refreshing = false
			dirty = true
			if update.err != nil {
				m.status = "Refresh failed: " + update.err.Error()
				continue
			}
			m.update(update.ps)
		case results := <-actions:
			m.busy = false
			m.results = results
			dirty = true
			var messages []string
			for _, r := range results {
				if r.Error != nil {
					messages = append(messages, fmt.Sprintf("PID %d: %v", r.Process.PID, r.Error))
				} else if r.Signal == "DRY-RUN" {
					messages = append(messages, fmt.Sprintf("Would stop PID %d on %d", r.Process.PID, r.Port))
				} else {
					messages = append(messages, fmt.Sprintf("Stopped PID %d on %d", r.Process.PID, r.Port))
					m.stopped[fmt.Sprintf("%d/%s", r.Port, r.Process.Protocol)] = r.Process
				}
				delete(m.selected, owner(r.Process))
			}
			m.status = strings.Join(messages, " | ")
			if len(messages) == 0 {
				m.status = "No targets"
			}
			refresh()
		}
		for _, k := range keys {
			if k.name == "quit" {
				return nil
			}
			if (len(m.confirming) > 0 || m.help || m.details) && (k.name == "pageup" || k.name == "pagedown") {
				if k.name == "pageup" {
					m.paneOffset = max(0, m.paneOffset-max(1, height-7))
				} else {
					m.paneOffset += max(1, height-7)
				}
				continue
			}
			if m.search {
				switch k.name {
				case "esc":
					m.search = false
					m.query = ""
					m.cursor = 0
				case "enter":
					m.search = false
				case "backspace":
					if m.query != "" {
						m.query = backspace(m.query)
					}
					m.cursor = 0
				case "clear":
					m.query = ""
					m.cursor = 0
				case "text", "paste":
					m.query += Clean(k.text)
					m.cursor = 0
				}
				continue
			}
			// Pasted text starts a search; it never executes navigation commands.
			if k.name == "paste" {
				m.query = Clean(k.text)
				m.search = true
				m.cursor = 0
				continue
			}
			if len(m.confirming) > 0 {
				if k.name == "esc" || k.text == "n" {
					m.confirming = nil
					m.status = "Action canceled"
					continue
				}
				if k.text == "y" && !m.busy {
					targets := m.confirming
					force := m.confirmForce
					m.confirming = nil
					m.busy = true
					m.status = "Stopping selected owners..."
					go func() {
						rs := o.Execute(targets, force)
						select {
						case actions <- rs:
						case <-done:
						}
					}()
				}
				continue
			}
			if m.help {
				if k.name == "esc" || k.text == "?" || k.text == "q" {
					m.help = false
				}
				continue
			}
			visible := m.visible()
			if m.cursor >= len(visible) {
				m.cursor = max(0, len(visible)-1)
			}
			if m.copyMenu {
				if k.name == "esc" {
					m.copyMenu = false
					continue
				}
				if len(visible) > 0 {
					g := visible[m.cursor]
					value := ""
					switch k.text {
					case "p":
						value = fmt.Sprint(g.process.PID)
					case "o":
						var ports []string
						seen := map[int]bool{}
						for _, p := range g.sockets {
							if !seen[p.Port] {
								ports = append(ports, fmt.Sprint(p.Port))
								seen[p.Port] = true
							}
						}
						value = strings.Join(ports, ",")
					case "c":
						value = g.process.CommandLine
					case "u":
						for _, p := range g.sockets {
							if p.Protocol == types.ProtocolTCP {
								value = fmt.Sprintf("http://localhost:%d", p.Port)
								break
							}
						}
					}
					if value != "" {
						fmt.Printf("\x1b]52;c;%s\a", base64.StdEncoding.EncodeToString([]byte(value)))
						m.status = "Copy requested via terminal clipboard (OSC 52)"
						m.copyMenu = false
					}
				}
				continue
			}
			rows := max(1, height-9)
			switch {
			case k.name == "esc":
				if m.details {
					m.details = false
				} else {
					m.query = ""
					m.cursor = 0
				}
			case k.text == "q":
				return nil
			case k.text == "/":
				m.search = true
			case k.text == "?":
				m.help = true
				m.paneOffset = 0
			case k.name == "up" || k.text == "k":
				m.cursor = max(0, m.cursor-1)
			case k.name == "down" || k.text == "j":
				m.cursor = min(max(0, len(visible)-1), m.cursor+1)
			case k.name == "home":
				m.cursor = 0
			case k.name == "end":
				m.cursor = max(0, len(visible)-1)
			case k.name == "pageup":
				m.cursor = max(0, m.cursor-rows)
			case k.name == "pagedown":
				m.cursor = min(max(0, len(visible)-1), m.cursor+rows)
			case k.text == " ":
				if len(visible) > 0 {
					g := visible[m.cursor]
					m.selected[g.key] = !m.selected[g.key]
				}
			case k.text == "a":
				all := len(visible) > 0
				for _, g := range visible {
					if !m.selected[g.key] {
						all = false
					}
				}
				for _, g := range visible {
					m.selected[g.key] = !all
				}
			case k.text == "A":
				m.selected = map[string]bool{}
				m.status = "Selection cleared"
			case k.text == "r":
				refresh()
			case k.text == "p":
				m.paused = !m.paused
			case k.text == "s":
				m.sort = cycle(m.sort, []string{"port", "memory", "project", "user", "name", "protocol"})
				m.cursor = 0
			case k.text == "e":
				m.exposure = cycle(m.exposure, []string{"all", "localhost", "public", "interface"})
				m.cursor = 0
			case k.text == "c":
				m.copyMenu = true
			case k.name == "enter":
				m.details = !m.details
				m.paneOffset = 0
				if m.details && len(visible) > 0 && o.Inspect != nil {
					m.parent, _ = o.Inspect(visible[m.cursor].process.PPID)
				}
			case k.text == "x" || k.text == "F":
				if !m.busy {
					m.confirming = m.targets(visible)
					m.paneOffset = 0
					m.confirmForce = k.text == "F" || o.Force
					if len(m.confirming) > 0 {
						m.status = "Review impact; y confirms, n/Esc cancels"
					}
				}
			}
		}
	}
}
func cycle(s string, values []string) string {
	for i, v := range values {
		if s == v {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}
func (m *dashboard) visible() []group {
	gs := filterGroups(m.groups, m.query, m.exposure)
	sortGroups(gs, m.sort)
	return gs
}
func (m *dashboard) update(ps []types.Process) {
	oldVisible := m.visible()
	cursorKey := ""
	if m.cursor < len(oldVisible) {
		cursorKey = oldVisible[m.cursor].key
	}
	m.groups = groupProcesses(ps)
	alive := map[string]bool{}
	for _, g := range m.groups {
		alive[g.key] = true
		for _, p := range g.sockets {
			k := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
			if old, ok := m.stopped[k]; ok && owner(old) != owner(p) {
				m.status = fmt.Sprintf("Port %d returned: %s PID %d; previous PID %d; parent/supervisor %d", p.Port, p.Name, p.PID, old.PID, p.PPID)
				delete(m.stopped, k)
			}
		}
	}
	for k := range m.selected {
		if !alive[k] {
			delete(m.selected, k)
		}
	}
	gs := m.visible()
	m.cursor = min(m.cursor, max(0, len(gs)-1))
	for i, g := range gs {
		if g.key == cursorKey {
			m.cursor = i
			break
		}
	}
	if m.status == "Loading listeners..." {
		m.status = "Ready. / searches; Enter shows details; x stops a process."
	}
}
func (m *dashboard) targets(visible []group) []types.Process {
	var ps []types.Process
	for _, g := range m.groups {
		if m.selected[g.key] {
			ps = append(ps, g.sockets...)
		}
	}
	if len(ps) == 0 && len(visible) > 0 {
		ps = append(ps, visible[min(m.cursor, len(visible)-1)].sockets...)
	}
	return ps
}
func (m *dashboard) render(o Options, w, h int) {
	if w < 30 || h < 10 {
		fmt.Print("\x1b[H\x1b[2JTerminal too small (30x10 min).\r\nResize, or q to quit.")
		return
	}
	visible := m.visible()
	if m.cursor >= len(visible) {
		m.cursor = max(0, len(visible)-1)
	}
	lines := []string{"kport | live port manager", "[/] Search  [Enter] Details  [Space] Select  [x] Stop  [F] Force  [?] Help  [q] Quit"}
	mode := "LIVE"
	if m.paused {
		mode = "PAUSED"
	}
	if m.busy {
		mode = "WORKING"
	}
	if o.DryRun {
		mode += " / READ ONLY"
	}
	query := m.query
	if m.search {
		query += "_"
	}
	lines = append(lines, fmt.Sprintf("%s | sort: %s | exposure: %s | search: %s", mode, m.sort, m.exposure, query))
	if m.help {
		lines = append(lines, "", "NAVIGATION: arrows / j k, Home / End, Page Up / Down", "SEARCH: / enters search; Enter keeps filter; Esc clears; Ctrl-U clears text", "Search terms match name, PID, port, project/CWD, user, command, protocol.", "SELECT: Space toggles owner; a toggles visible; A clears every selection", "A selected owner includes all its sockets, even those outside a filter.", "ACTIONS: x graceful stop; F force; both require y confirmation", "DISPLAY: r refresh; p pause; s cycle sort; e cycle exposure", "COPY: c then p PID / o ports / c command / u local HTTP URL", "Clipboard uses OSC 52; terminal support and permission are required.", "MONITORING: returning ports show their new PID and parent/supervisor.", "Esc closes details/help. Ctrl-C or q exits and restores the terminal.")
	} else if len(m.confirming) > 0 {
		gs := groupProcesses(m.confirming)
		action := "Graceful stop"
		if m.confirmForce {
			action = "FORCE stop"
		}
		if o.DryRun {
			action = "Preview stop"
		}
		ports := map[string]bool{}
		for _, p := range m.confirming {
			ports[fmt.Sprintf("%d/%s", p.Port, p.Protocol)] = true
		}
		lines = append(lines, "", fmt.Sprintf("%s %d owners; affects %d ports", action, len(gs), len(ports)), "All ports below belong to the selected process/container.")
		for _, g := range gs {
			lines = append(lines, fmt.Sprintf("PID %d %s [%s]", g.process.PID, g.process.Name, socketSummary(g)))
			if g.process.ContainerID != "" {
				lines = append(lines, "  Container: "+g.process.ContainerName+" (all container services stop)")
			}
			if o.Tree {
				lines = append(lines, fmt.Sprintf("  Descendants: %v", g.process.ChildrenPIDs))
			}
		}
		lines = append(lines, "", "[y] Confirm   [n/Esc] Cancel")
	} else if m.details && len(visible) > 0 {
		g := visible[m.cursor]
		p := g.process
		lines = append(lines, "", fmt.Sprintf("%s | PID %d | user %s | %s", p.Name, p.PID, p.User, p.MemoryHuman), "Command: "+p.CommandLine, "Executable: "+p.Exe, "Project/CWD: "+p.Cwd, fmt.Sprintf("Parent: %d %s %s", p.PPID, m.parent.Name, m.parent.CommandLine), fmt.Sprintf("Children: %v", p.ChildrenPIDs), "Identity: "+p.StartID)
		if p.ContainerID != "" {
			lines = append(lines, "Container: "+p.ContainerName+" "+p.ContainerID)
		}
		if p.MatchReason != "" {
			lines = append(lines, "Matched: "+p.MatchReason)
		}
		lines = append(lines, "Sockets:")
		for _, p := range g.sockets {
			lines = append(lines, fmt.Sprintf("  %d/%s %s [%s]", p.Port, p.Protocol, p.Address, Exposure(p.Address)))
		}
		lines = append(lines, "", "[x] Stop  [F] Force  [c] Copy  [Esc] Back")
	} else {
		lines = append(lines, "", "SEL  PID      NAME / CONTAINER            MEMORY     PORTS")
		rows := max(1, h-9)
		start := max(0, m.cursor-rows+1)
		end := min(len(visible), start+rows)
		for i := start; i < end; i++ {
			g := visible[i]
			p := g.process
			mark := "[ ]"
			if m.selected[g.key] {
				mark = "[x]"
			}
			pointer := " "
			if i == m.cursor {
				pointer = ">"
			}
			name := p.Name
			if p.ContainerName != "" {
				name = "docker:" + p.ContainerName
			}
			lines = append(lines, fmt.Sprintf("%s%s %-8d %-27s %-10s %s", pointer, mark, p.PID, clip(name, 27), p.MemoryHuman, socketSummary(g)))
		}
		if len(visible) == 0 {
			lines = append(lines, "No matching listeners. Refresh remains active.")
		}
	}
	count, hidden, ports := 0, 0, 0
	visibleKeys := map[string]bool{}
	for _, g := range visible {
		visibleKeys[g.key] = true
	}
	for _, g := range m.groups {
		if m.selected[g.key] {
			count++
			ports += len(g.sockets)
			if !visibleKeys[g.key] {
				hidden++
			}
		}
	}
	footer := fmt.Sprintf("%d/%d owners | %d selected (%d sockets, %d hidden) | r refresh / p pause / s sort / e exposure", len(visible), len(m.groups), count, ports, hidden)
	status := m.status
	if m.copyMenu {
		status = "Copy: [p] PID  [o] ports  [c] command  [u] local URL  [Esc] cancel"
	}
	// Keep footer/status visible even on short terminals; clear each line to
	// remove remnants of a previous longer view without screen-wide flicker.
	if len(lines) > h-2 {
		if m.help || m.details || len(m.confirming) > 0 {
			body := lines[3:]
			slots := max(1, h-5)
			m.paneOffset = min(m.paneOffset, max(0, len(body)-slots))
			end := min(len(body), m.paneOffset+slots)
			lines = append(lines[:3], body[m.paneOffset:end]...)
			footer = fmt.Sprintf("Lines %d-%d/%d | PgUp/PgDn scroll | %s", m.paneOffset+1, end, len(body), footer)
		} else {
			lines = lines[:h-2]
		}
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, status)
	var out strings.Builder
	out.WriteString("\x1b[H")
	for i, line := range lines {
		out.WriteString("\x1b[2K")
		if !o.NoColor {
			switch {
			case i == 0:
				out.WriteString("\x1b[1;36m")
			case strings.HasPrefix(line, ">"):
				out.WriteString("\x1b[36m")
			case i == len(lines)-1:
				out.WriteString("\x1b[33m")
			}
		}
		out.WriteString(clip(line, w-1))
		if !o.NoColor {
			out.WriteString("\x1b[0m")
		}
		if i < len(lines)-1 {
			out.WriteString("\r\n")
		}
	}
	fmt.Print(out.String())
}

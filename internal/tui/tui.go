package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tyzrex/killport/types"
)

const (
	ansiClearScreen = "\033[2J\033[H"
	ansiHideCursor  = "\033[?25l"
	ansiShowCursor  = "\033[?25h"
	colorReset      = "\033[0m"
	colorBold       = "\033[1m"
	colorRed        = "\033[31m"
	colorGreen      = "\033[32m"
	colorYellow     = "\033[33m"
	colorCyan       = "\033[36m"
	colorGray       = "\033[90m"
	colorInverse    = "\033[7m"
)

// RunTUI launches the interactive terminal process picker.
func RunTUI(procs []types.Process) ([]types.Process, bool, bool) {
	if len(procs) == 0 {
		return nil, false, true
	}

	fd := int(os.Stdin.Fd())
	oldTerm, err := enableRawMode(fd)
	if err != nil {
		return nil, false, true
	}
	defer func() {
		restoreMode(fd, oldTerm)
		fmt.Print(ansiShowCursor)
	}()

	// Sort procs by port
	sort.Slice(procs, func(i, j int) bool {
		if procs[i].Port == procs[j].Port {
			return procs[i].PID < procs[j].PID
		}
		return procs[i].Port < procs[j].Port
	})

	selectedMap := make(map[int]bool) // key: PID
	cursorIdx := 0
	filterText := ""
	buf := make([]byte, 16)

	fmt.Print(ansiHideCursor)

	for {
		filtered := filterProcesses(procs, filterText)
		if cursorIdx >= len(filtered) {
			cursorIdx = max(0, len(filtered)-1)
		}

		renderScreen(filtered, selectedMap, cursorIdx, filterText, len(procs))

		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		// Handle keys
		if n == 1 {
			b := buf[0]
			switch b {
			case 3: // Ctrl+C
				return nil, false, true
			case 27: // Escape
				if filterText != "" {
					filterText = ""
					cursorIdx = 0
				} else {
					return nil, false, true
				}
			case 'q', 'Q':
				if filterText == "" {
					return nil, false, true
				} else {
					filterText += string(b)
				}
			case 13, 10: // Enter
				var toKill []types.Process
				if countSelected(selectedMap) == 0 && len(filtered) > 0 {
					// Kill current cursor item
					toKill = append(toKill, filtered[cursorIdx])
				} else {
					for _, p := range procs {
						if selectedMap[p.PID] {
							toKill = append(toKill, p)
						}
					}
				}
				clearScreen()
				return toKill, false, false
			case 'f', 'F': // Force kill
				if filterText == "" {
					var toKill []types.Process
					if countSelected(selectedMap) == 0 && len(filtered) > 0 {
						toKill = append(toKill, filtered[cursorIdx])
					} else {
						for _, p := range procs {
							if selectedMap[p.PID] {
								toKill = append(toKill, p)
							}
						}
					}
					clearScreen()
					return toKill, true, false
				} else {
					filterText += string(b)
				}
			case ' ': // Space: toggle selection
				if len(filtered) > 0 {
					pid := filtered[cursorIdx].PID
					selectedMap[pid] = !selectedMap[pid]
					if cursorIdx < len(filtered)-1 {
						cursorIdx++
					}
				}
			case 'a', 'A': // Toggle all
				if filterText == "" {
					allSelected := true
					for _, p := range filtered {
						if !selectedMap[p.PID] {
							allSelected = false
							break
						}
					}
					for _, p := range filtered {
						selectedMap[p.PID] = !allSelected
					}
				} else {
					filterText += string(b)
				}
			case 'j': // Vim down
				if filterText == "" {
					if cursorIdx < len(filtered)-1 {
						cursorIdx++
					}
				} else {
					filterText += string(b)
				}
			case 'k': // Vim up
				if filterText == "" {
					if cursorIdx > 0 {
						cursorIdx--
					}
				} else {
					filterText += string(b)
				}
			case 127, 8: // Backspace
				if len(filterText) > 0 {
					filterText = filterText[:len(filterText)-1]
					cursorIdx = 0
				}
			default:
				if b >= 32 && b <= 126 {
					filterText += string(b)
					cursorIdx = 0
				}
			}
		} else if n >= 3 && buf[0] == 27 && buf[1] == '[' {
			// ANSI escape sequences (Arrow keys)
			switch buf[2] {
			case 'A': // Up
				if cursorIdx > 0 {
					cursorIdx--
				}
			case 'B': // Down
				if cursorIdx < len(filtered)-1 {
					cursorIdx++
				}
			}
		}
	}

	return nil, false, true
}

func filterProcesses(procs []types.Process, query string) []types.Process {
	if query == "" {
		return procs
	}
	q := strings.ToLower(query)
	var filtered []types.Process
	for _, p := range procs {
		portStr := fmt.Sprintf("%d", p.Port)
		pidStr := fmt.Sprintf("%d", p.PID)
		if strings.Contains(portStr, q) ||
			strings.Contains(pidStr, q) ||
			strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.CommandLine), q) ||
			strings.Contains(strings.ToLower(p.User), q) ||
			strings.Contains(strings.ToLower(p.Address), q) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func renderScreen(procs []types.Process, selected map[int]bool, cursor int, filter string, totalCount int) {
	var sb strings.Builder
	sb.WriteString(ansiClearScreen)

	// Header
	sb.WriteString(fmt.Sprintf("%s%s⚡ killport - Interactive Port Manager%s\n", colorBold, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s[↑/↓/j/k] Navigate  [Space] Select  [a] Toggle All  [Enter] Kill  [f] Force Kill  [q] Quit%s\n", colorGray, colorReset))

	filterDisplay := filter
	if filterDisplay == "" {
		filterDisplay = fmt.Sprintf("%s(Type to filter live...)%s", colorGray, colorReset)
	}
	sb.WriteString(fmt.Sprintf("🔍 Filter: %s\n\n", filterDisplay))

	// Table Header
	sb.WriteString(fmt.Sprintf("%s  %-4s %-8s %-6s %-10s %-20s %-10s %-10s %s%s\n",
		colorBold, "SEL", "PORT", "PROTO", "PID", "NAME", "MEMORY", "USER", "ADDRESS", colorReset))
	sb.WriteString(strings.Repeat("─", 88) + "\n")

	if len(procs) == 0 {
		sb.WriteString(fmt.Sprintf("\n   %sNo processes match filter query '%s'%s\n\n", colorYellow, filter, colorReset))
	} else {
		// Display up to 18 rows
		maxRows := 18
		startIdx := 0
		if cursor >= maxRows {
			startIdx = cursor - maxRows + 1
		}
		endIdx := min(len(procs), startIdx+maxRows)

		for i := startIdx; i < endIdx; i++ {
			p := procs[i]
			isCursor := i == cursor
			isSel := selected[p.PID]

			selBox := "[ ]"
			if isSel {
				selBox = fmt.Sprintf("%s[✓]%s", colorGreen, colorReset)
			}

			pName := p.Name
			if len(pName) > 20 {
				pName = pName[:17] + "..."
			}
			uName := p.User
			if uName == "" {
				uName = "-"
			}
			if len(uName) > 10 {
				uName = uName[:7] + "..."
			}
			mem := p.MemoryHuman
			if mem == "" {
				mem = "-"
			}

			rowStr := fmt.Sprintf("%-4s %-8d %-6s %-10d %-20s %-10s %-10s %s",
				selBox,
				p.Port,
				strings.ToUpper(string(p.Protocol)),
				p.PID,
				pName,
				mem,
				uName,
				p.Address,
			)

			if isCursor {
				sb.WriteString(fmt.Sprintf(" %s▶ %s%s\n", colorCyan+colorBold, rowStr, colorReset))
			} else {
				sb.WriteString(fmt.Sprintf("   %s\n", rowStr))
			}
		}
	}

	sb.WriteString(strings.Repeat("─", 88) + "\n")
	selCount := countSelected(selected)
	sb.WriteString(fmt.Sprintf("%sShowing %d/%d process(es) | %d selected%s\n",
		colorGray, len(procs), totalCount, selCount, colorReset))

	fmt.Print(sb.String())
}

func countSelected(m map[int]bool) int {
	c := 0
	for _, v := range m {
		if v {
			c++
		}
	}
	return c
}

func clearScreen() {
	fmt.Print(ansiClearScreen)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

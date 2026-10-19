package tui

import (
	"strings"
	"unicode/utf8"
)

// decoder retains incomplete UTF-8 and ANSI sequences across reads. Bracketed
// paste is delivered as text, so pasted shortcuts never trigger actions.
type key struct{ name, text string }
type decoder struct {
	pending string
	paste   bool
	pasted  string
}

func (d *decoder) feed(chunk string) []key {
	d.pending += chunk
	var keys []key
	for len(d.pending) > 0 {
		if d.paste {
			if i := strings.Index(d.pending, "\x1b[201~"); i >= 0 {
				d.pasted += d.pending[:i]
				keys = append(keys, key{name: "paste", text: d.pasted})
				d.pending = d.pending[i+6:]
				d.paste = false
				d.pasted = ""
				continue
			}
			// Keep a possible partial end marker for the next read.
			i := strings.LastIndex(d.pending, "\x1b")
			if i < 0 {
				d.pasted += d.pending
				d.pending = ""
			} else {
				d.pasted += d.pending[:i]
				d.pending = d.pending[i:]
			}
			break
		}
		if d.pending[0] == 27 {
			sequences := map[string]string{"\x1b[A": "up", "\x1b[B": "down", "\x1b[C": "right", "\x1b[D": "left", "\x1b[H": "home", "\x1b[F": "end", "\x1bOH": "home", "\x1bOF": "end", "\x1bOA": "up", "\x1bOB": "down", "\x1b[1~": "home", "\x1b[4~": "end", "\x1b[5~": "pageup", "\x1b[6~": "pagedown", "\x1b[200~": "paste-start"}
			matched, partial := false, false
			for seq, name := range sequences {
				if strings.HasPrefix(d.pending, seq) {
					d.pending = d.pending[len(seq):]
					if name == "paste-start" {
						d.paste = true
					} else {
						keys = append(keys, key{name: name})
					}
					matched = true
					break
				}
				if strings.HasPrefix(seq, d.pending) {
					partial = true
				}
			}
			if matched {
				continue
			}
			if partial {
				break
			}
			// Consume unsupported CSI sequences as a unit rather than interpreting
			// their trailing bytes as destructive shortcut keys.
			if strings.HasPrefix(d.pending, "\x1b[") {
				end := -1
				for i := 2; i < len(d.pending); i++ {
					if d.pending[i] >= 0x40 && d.pending[i] <= 0x7e {
						end = i + 1
						break
					}
				}
				if end < 0 {
					break
				}
				d.pending = d.pending[end:]
				continue
			}
			keys = append(keys, key{name: "esc"})
			d.pending = d.pending[1:]
			continue
		}
		if !utf8.FullRuneInString(d.pending) {
			break
		}
		r, n := utf8.DecodeRuneInString(d.pending)
		d.pending = d.pending[n:]
		switch r {
		case 3:
			keys = append(keys, key{name: "quit"})
		case 13, 10:
			keys = append(keys, key{name: "enter"})
		case 127, 8:
			keys = append(keys, key{name: "backspace"})
		case 21:
			keys = append(keys, key{name: "clear"})
		default:
			if r >= 32 {
				keys = append(keys, key{name: "text", text: string(r)})
			}
		}
	}
	return keys
}
func (d *decoder) escape() []key {
	if d.pending == "\x1b" && !d.paste {
		d.pending = ""
		return []key{{name: "esc"}}
	}
	return nil
}

package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tyzrex/kport/types"
	"go.yaml.in/yaml/v3"
)

var scriptPort = regexp.MustCompile(`(?:--port(?:=|\s+)|(?:^|\s)-p\s+|(?:^|\s)PORT=)(\d+)`)
var configPort = regexp.MustCompile(`\bport\s*:\s*(\d+)`)

func explicitPorts(dir string) []int {
	seen := map[int]bool{}
	add := func(text string, re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			p, _ := strconv.Atoi(m[1])
			if p > 0 && p <= 65535 {
				seen[p] = true
			}
		}
	}
	var pkg struct{ Scripts map[string]string }
	if b, e := os.ReadFile(filepath.Join(dir, "package.json")); e == nil {
		if json.Unmarshal(b, &pkg) == nil {
			for name, script := range pkg.Scripts {
				if name == "dev" || name == "start" || strings.HasPrefix(name, "dev:") {
					add(script, scriptPort)
				}
			}
		}
	}
	for _, name := range []string{"vite.config.ts", "vite.config.js", "vite.config.mjs", "astro.config.ts", "astro.config.mjs", "nuxt.config.ts"} {
		if b, e := os.ReadFile(filepath.Join(dir, name)); e == nil {
			add(string(b), configPort)
		}
	}
	if len(seen) == 0 {
		return nil
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

func parseComposePorts(content string) []int {
	var doc struct {
		Services map[string]struct{ Ports []yaml.Node }
	}
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	seen := map[int]bool{}
	for _, service := range doc.Services {
		for _, node := range service.Ports {
			value := ""
			if node.Kind == yaml.MappingNode {
				var mapping struct{ Published string }
				if node.Decode(&mapping) == nil {
					value = mapping.Published
				}
			} else {
				value = node.Value
				value = strings.Split(value, "/")[0]
				parts := strings.Split(value, ":")
				if len(parts) < 2 {
					continue
				}
				value = parts[len(parts)-2]
			}
			parts := strings.Split(value, "-")
			start, e := strconv.Atoi(parts[0])
			if e != nil {
				continue
			}
			end := start
			if len(parts) == 2 {
				end, _ = strconv.Atoi(parts[1])
			}
			if start < 1 || end > 65535 || end < start {
				continue
			}
			for p := start; p <= end; p++ {
				seen[p] = true
			}
		}
	}
	var ports []int
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

// MatchProject distinguishes verified CWD ownership from port-only suggestions.
func MatchProject(dir string, ps []types.Process) (owned, suggested []types.Process) {
	dir = canonicalDir(dir)
	info, _ := DetectProject(dir)
	ports := map[int]bool{}
	for _, p := range info.Ports {
		ports[p] = true
	}
	for _, p := range ps {
		if p.Cwd != "" && within(dir, p.Cwd) {
			p.MatchReason = "working directory belongs to project"
			owned = append(owned, p)
		} else if ports[p.Port] {
			p.MatchReason = "port suggestion from " + info.Source + "; project ownership unverified"
			suggested = append(suggested, p)
		}
	}
	return
}
func canonicalDir(dir string) string {
	if a, e := filepath.Abs(dir); e == nil {
		dir = a
	}
	if a, e := filepath.EvalSymlinks(dir); e == nil {
		dir = a
	}
	return filepath.Clean(dir)
}
func within(root, dir string) bool {
	rel, e := filepath.Rel(canonicalDir(root), canonicalDir(dir))
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func AbsoluteDir(dir string) (string, error) {
	a, e := filepath.Abs(dir)
	if e != nil {
		return "", e
	}
	info, e := os.Stat(a)
	if e != nil {
		return "", e
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	return canonicalDir(a), nil
}

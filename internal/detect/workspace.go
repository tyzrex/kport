package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Service describes a workspace service and the ports suggested by its own config.
type Service struct {
	Directory string `json:"directory"`
	Framework string `json:"framework"`
	Ports     []int  `json:"ports"`
	Source    string `json:"source"`
}

func DetectProject(dir string) (ProjectInfo, bool) {
	dir = canonicalDir(dir)
	root, found := detectSingleProject(dir)
	var pkg struct{ Workspaces json.RawMessage }
	if b, e := os.ReadFile(filepath.Join(dir, "package.json")); e != nil || json.Unmarshal(b, &pkg) != nil {
		return root, found
	}
	var patterns []string
	if json.Unmarshal(pkg.Workspaces, &patterns) != nil {
		var object struct{ Packages []string }
		if json.Unmarshal(pkg.Workspaces, &object) == nil {
			patterns = object.Packages
		}
	}
	seen := map[string]bool{dir: true}
	ports := map[int]bool{}
	for _, p := range root.Ports {
		ports[p] = true
	}
	var services []Service
	if found {
		services = append(services, Service{dir, root.Framework, root.Ports, root.Source})
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, sub := range matches {
			sub = canonicalDir(sub)
			if seen[sub] || !within(dir, sub) {
				continue
			}
			seen[sub] = true
			if info, ok := detectSingleProject(sub); ok {
				services = append(services, Service{sub, info.Framework, info.Ports, info.Source})
				for _, p := range info.Ports {
					ports[p] = true
				}
			}
		}
	}
	if len(services) > 1 || (!found && len(services) > 0) {
		root = ProjectInfo{Framework: "Workspace", Source: "package.json workspaces", Services: services}
		for p := range ports {
			root.Ports = append(root.Ports, p)
		}
		sort.Ints(root.Ports)
		return root, true
	}
	return root, found
}

package detect

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tyzrex/kport/types"
)

// ProjectInfo contains detected project metadata and suspected ports.
type ProjectInfo struct {
	Services  []Service `json:"services,omitempty"`
	Framework string    `json:"framework"`
	Ports     []int     `json:"ports"`
	Source    string    `json:"source"`
	Details   string    `json:"details,omitempty"`
}

// DetectProject inspects the directory for known framework configs and dependency manifests.
func detectSingleProject(dir string) (ProjectInfo, bool) {
	if dir == "" || dir == "." {
		if d, err := os.Getwd(); err == nil {
			dir = d
		}
	}

	if ports := explicitPorts(dir); len(ports) > 0 {
		return ProjectInfo{Framework: "Project", Ports: ports, Source: "explicit script/config ports"}, true
	}
	// 1. Check docker-compose.yml
	if composePorts, ok := checkDockerCompose(dir); ok && len(composePorts) > 0 {
		return ProjectInfo{
			Framework: "Docker Compose",
			Ports:     composePorts,
			Source:    "docker-compose.yml",
			Details:   "Host ports mapped in compose file",
		}, true
	}

	// 2. Check Next.js
	if exists(filepath.Join(dir, "next.config.js")) || exists(filepath.Join(dir, "next.config.mjs")) || exists(filepath.Join(dir, "next.config.ts")) {
		return ProjectInfo{
			Framework: "Next.js",
			Ports:     []int{3000},
			Source:    "next.config",
		}, true
	}

	// 3. Check Vite / SvelteKit / Astro
	if exists(filepath.Join(dir, "astro.config.mjs")) || exists(filepath.Join(dir, "astro.config.ts")) {
		return ProjectInfo{
			Framework: "Astro",
			Ports:     []int{4321},
			Source:    "astro.config",
		}, true
	}

	if exists(filepath.Join(dir, "vite.config.js")) || exists(filepath.Join(dir, "vite.config.ts")) || exists(filepath.Join(dir, "vite.config.mjs")) {
		return ProjectInfo{
			Framework: "Vite",
			Ports:     []int{5173},
			Source:    "vite.config",
		}, true
	}

	// 4. Check package.json
	pkgPath := filepath.Join(dir, "package.json")
	if exists(pkgPath) {
		if info, ok := parsePackageJSON(pkgPath); ok {
			return info, true
		}
	}

	// 5. Check Python (Django / FastAPI / Flask)
	if exists(filepath.Join(dir, "manage.py")) {
		return ProjectInfo{
			Framework: "Django",
			Ports:     []int{8000},
			Source:    "manage.py",
		}, true
	}

	// 6. Check Rails
	if exists(filepath.Join(dir, "bin", "rails")) || exists(filepath.Join(dir, "config", "application.rb")) {
		return ProjectInfo{
			Framework: "Ruby on Rails",
			Ports:     []int{3000},
			Source:    "config/application.rb",
		}, true
	}

	return ProjectInfo{}, false
}

// FindProcessesByCWD filters processes whose current working directory matches targetDir.
func FindProcessesByCWD(targetDir string, procs []types.Process) []types.Process {
	if targetDir == "" || targetDir == "." {
		if d, err := os.Getwd(); err == nil {
			targetDir = d
		}
	}
	targetDir = filepath.Clean(targetDir)

	var matches []types.Process
	for _, p := range procs {
		if p.Cwd == "" {
			continue
		}
		cleanCwd := filepath.Clean(p.Cwd)
		if within(targetDir, cleanCwd) {
			matches = append(matches, p)
		}
	}
	return matches
}

func parsePackageJSON(pkgPath string) (ProjectInfo, bool) {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return ProjectInfo{}, false
	}

	type pkgStruct struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	var pkg pkgStruct
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ProjectInfo{}, false
	}

	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[k] = true
	}

	// Check framework dependencies
	if allDeps["next"] {
		return ProjectInfo{Framework: "Next.js", Ports: []int{3000}, Source: "package.json"}, true
	}
	if allDeps["astro"] {
		return ProjectInfo{Framework: "Astro", Ports: []int{4321}, Source: "package.json"}, true
	}
	if allDeps["vite"] || allDeps["@sveltejs/kit"] {
		return ProjectInfo{Framework: "Vite/SvelteKit", Ports: []int{5173}, Source: "package.json"}, true
	}
	if allDeps["nuxt"] {
		return ProjectInfo{Framework: "Nuxt", Ports: []int{3000}, Source: "package.json"}, true
	}
	if allDeps["react-scripts"] {
		return ProjectInfo{Framework: "Create React App", Ports: []int{3000}, Source: "package.json"}, true
	}
	if allDeps["@angular/cli"] || allDeps["@angular/core"] {
		return ProjectInfo{Framework: "Angular", Ports: []int{4200}, Source: "package.json"}, true
	}
	if allDeps["@remix-run/serve"] || allDeps["@remix-run/react"] {
		return ProjectInfo{Framework: "Remix", Ports: []int{3000}, Source: "package.json"}, true
	}
	if allDeps["@nestjs/core"] || allDeps["express"] || allDeps["fastify"] {
		return ProjectInfo{Framework: "Node Server", Ports: []int{3000, 8080}, Source: "package.json"}, true
	}

	return ProjectInfo{}, false
}

func checkDockerCompose(dir string) ([]int, bool) {
	files := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, f := range files {
		p := filepath.Join(dir, f)
		if exists(p) {
			if data, err := os.ReadFile(p); err == nil {
				ports := parseComposePorts(string(data))
				if len(ports) > 0 {
					return ports, true
				}
			}
		}
	}
	return nil, false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

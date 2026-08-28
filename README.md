# ⚡ killport (kport)

> A blazing-fast, cross-platform Go library and CLI tool to instantly find, inspect, and kill processes occupying network ports. Zero external dependencies, sub-millisecond execution, and first-class support for Linux, macOS, and Windows.

Say goodbye to slow `npx kill-port` startup times and cumbersome `lsof -i :3000 | kill` bash pipelines.

---

## ✨ Features

- 🚀 **Blazing Fast (< 2ms)**: Direct `/proc` inspection in pure Go on Linux, optimized `lsof` on macOS, and native Windows API/`netstat`.
- 📦 **Zero Dependencies**: Built strictly using Go's standard library.
- 🎯 **Flexible Syntax**: Supports single ports (`3000`), colon prefixes (`:3000`), port ranges (`3000-3005`), comma lists (`3000,8080`), and **framework presets** (`next`, `vite`, `postgres`, `redis`, `dev`).
- 🌲 **Process Tree Killer (`-T, --tree`)**: Kills the parent process and all of its spawned child processes cleanly to avoid orphan background processes.
- ⏳ **Wait-Until-Free (`-W, --wait`)**: Blocks until the OS socket is completely released and unbound, avoiding `EADDRINUSE` race conditions in restart scripts (`kport 3000 -W && npm run dev`).
- 👀 **Watch Mode (`-w, --watch`)**: Monitors ports in real-time and auto-kills any stray processes that bind to them.
- 🛡️ **Safety Guardrails**: Protects PID 1 (init/systemd), the calling process, and warns before terminating protected system ports (SSH 22, DNS 53).
- 🔍 **Detailed Inspector (`--inspect`)**: View exact memory usage (RSS), parent PID (PPID), working directory (CWD), executable path, and child process trees.
- 🐚 **Shell Autocompletion**: Built-in autocompletion generators for `bash`, `zsh`, `fish`, and `powershell` with live listening port suggestions.
- 🤖 **Machine-Readable JSON**: `--json` output for easy integration into dev tools and CI/CD pipelines.

---

## 📥 Installation

### Option 1: One-Liner Script (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/tyzrex/killport/main/install.sh | sh
```

### Option 2: Via `go install`

```bash
go install github.com/tyzrex/killport/cmd/killport@latest
```

*(Installs both `killport` and the shorthand alias `kport`)*

---

## 💻 CLI Usage

```bash
# Kill a port
killport 3000
kport :3000

# Kill multiple ports or ranges
killport 3000 8080 5432
killport 3000-3005
killport 3000,8080,9000

# Kill by framework preset
killport next          # Next.js (port 3000)
killport vite          # Vite (port 5173)
killport dev           # Common dev ports (3000, 5173, 8080, 8000, 4200, 5000, ...)
killport postgres      # PostgreSQL (port 5432)
killport redis         # Redis (port 6379)

# Kill process and all its child processes (tree kill)
killport -T 3000

# Kill and wait until port is verified free by the OS
killport -W 3000

# Watch mode: continuously monitor & kill anything grabbing port 3000
killport -w 3000

# Inspect detailed process information (Memory, CWD, PPID, Tree)
killport --inspect 3000

# Preview without killing (dry-run)
killport -d 3000

# Force kill immediately (SIGKILL)
killport -f 3000

# List all active listening ports on the system
killport -l

# Filter list by process name
killport -l -n node

# Interactive confirmation prompt
killport -i 3000 8080

# Machine-readable JSON output
killport -j 3000
```

---

## 🐚 Shell Autocompletion

Enable tab-completion in your shell for instant port and flag suggestions:

### Bash
```bash
killport completion bash > /etc/bash_completion.d/killport
# or in ~/.bashrc:
source <(killport completion bash)
```

### Zsh
```bash
# In your ~/.zshrc:
source <(killport completion zsh)
```

### Fish
```bash
killport completion fish > ~/.config/fish/completions/killport.fish
```

---

## 📚 Go Library Usage

Import [`github.com/tyzrex/killport`](file:///home/tyzrex/Projects/killport/killport.go) into any Go project:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tyzrex/killport"
)

func main() {
	// 1. One-liner kill
	results, err := killport.Kill(3000, 8080)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	for _, res := range results {
		if res.Killed {
			fmt.Printf("Killed %s (PID %d) on port %d [%s]\n",
				res.Process.Name, res.Process.PID, res.Port, res.Signal)
		}
	}

	// 2. Custom Kill Options (Tree kill, WaitForFree, Force)
	killport.KillPorts([]int{3000}, killport.KillOptions{
		Protocol:    killport.ProtocolTCP,
		Force:       true,
		KillTree:    true,                // Kills child processes as well
		WaitForFree: true,                // Blocks until port is freed
		WaitTimeout: 3 * time.Second,
	})

	// 3. Inspect / Find active processes on ports
	procs, _ := killport.Find(3000)
	for _, p := range procs {
		fmt.Printf("PID: %d | Name: %s | Memory: %s | CWD: %s\n",
			p.PID, p.Name, p.MemoryHuman, p.Cwd)
	}

	// 4. Find processes by name
	nodeProcs, _ := killport.FindByName("node")

	// 5. Check if port is in use or wait for free
	inUse, _ := killport.IsPortInUse(3000)
	if inUse {
		_ = killport.WaitForFree(3000, 5*time.Second)
	}

	// 6. Watch mode
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	killport.Watch(ctx, []int{3000}, killport.DefaultKillOptions(), 500*time.Millisecond, func(res killport.KillResult) {
		fmt.Printf("Auto-killed PID %d on port %d\n", res.Process.PID, res.Port)
	})
}
```

---

## 🎛️ Presets Reference

| Preset Name | Ports Targeted | Target Environment |
|-------------|----------------|--------------------|
| `web` / `dev` | `3000, 3001, 5173, 8000, 8080, 4200, 5000, 8888, 1234, 4321, 9000` | Common web dev servers |
| `db` / `database` | `5432, 3306, 27017, 6379, 1433, 9200, 9300` | Common databases |
| `next` / `react` / `remix` / `rails` | `3000` | Next.js, React, Remix, Rails |
| `vite` / `svelte` | `5173` | Vite, SvelteKit |
| `astro` | `4321` | Astro |
| `vue` | `8080, 5173` | Vue.js |
| `angular` | `4200` | Angular CLI |
| `django` / `fastapi` | `8000` | Django, FastAPI |
| `flask` | `5000` | Flask |
| `postgres` / `postgresql` | `5432` | PostgreSQL |
| `mysql` / `mariadb` | `3306` | MySQL, MariaDB |
| `redis` | `6379` | Redis |
| `mongo` / `mongodb` | `27017` | MongoDB |

---

## 🧪 Testing

```bash
# Run unit & integration test suite
go test -v -race ./...
```

---

## 📄 License

MIT © tyzrex

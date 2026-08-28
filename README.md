<div align="center">

# ⚡ kport (`killport`)

**The blazing-fast, cross-platform port killer & interactive process manager.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/tyzrex/kport?style=flat-square)](https://golang.org)
[![CI Status](https://img.shields.io/github/actions/workflow/status/tyzrex/kport/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/tyzrex/kport/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tyzrex/kport?style=flat-square&color=orange)](https://github.com/tyzrex/kport/releases)

*Say goodbye to sluggish `npx kill-port` boot times and cryptic `lsof -i :3000 | xargs kill` shell gymnastics.*

[Features](#-features) • [Installation](#-installation) • [CLI Usage](#-cli-usage) • [Interactive TUI](#-interactive-tui) • [Go Library](#-go-library-api) • [Presets](#-presets-reference)

</div>

---

## 🚀 Why `kport`?

| Feature | `kport` | `npx kill-port` | `lsof` / `fuser` |
|---|:---:|:---:|:---:|
| **Startup & Execution Time** | **< 2ms** (Native OS / `/proc`) | ~800ms - 2000ms (Node boot) | ~15ms |
| **External Dependencies** | **Zero (Pure Go stdlib)** | Node.js + npm/npx + npm packages | OS-dependent binaries |
| **Cross-Platform** | **Linux, macOS, Windows** | Requires Node.js runtime | Unix-only (`fuser` / `lsof`) |
| **Interactive TUI Dashboard** | ✅ **Yes (Live search & select)** | ❌ No | ❌ No |
| **Project / CWD Auto-Detect** | ✅ **Yes (`kport .`)** | ❌ No | ❌ No |
| **Process Tree Cleanup** | ✅ **Yes (`-T, --tree`)** | ❌ Leaves orphan child workers | ❌ Manual |
| **Wait-Until-Free (`-W`)** | ✅ **Yes (No `EADDRINUSE` race)** | ❌ No | ❌ No |
| **Watch Mode (`-w`)** | ✅ **Yes (Continuous auto-kill)** | ❌ No | ❌ No |
| **Framework Presets** | ✅ **`next`, `vite`, `postgres`, etc.** | ❌ Numeric ports only | ❌ Numeric only |
| **Docker Resolution** | ✅ **Resolves container names** | ❌ Shows `docker-proxy` | ❌ Shows PID only |
| **RAM & CWD Inspection** | ✅ **Memory RSS, CWD, PPID** | ❌ No | ❌ Partial |

---

## ✨ Features

- ⚡ **Sub-Millisecond Execution**: Direct `/proc` socket inspection in pure Go on Linux, optimized `lsof` on macOS, and Win32 APIs on Windows.
- 🎨 **Interactive TUI**: Live fuzzy search, spacebar multi-selection, and one-key process killing without typing port numbers.
- 🧠 **Project-Aware (`kport .`)**: Automatically inspects `package.json`, `next.config`, `vite.config`, `docker-compose.yml`, or `manage.py` and frees your project's dev server.
- 🌲 **Tree Killer (`-T`)**: Recursively terminates parent processes and all descendant child workers.
- ⏳ **Wait-Until-Free (`-W`)**: Blocks until the OS socket is verified completely unbound, ensuring bulletproof dev restart scripts (`kport 3000 -W && npm run dev`).
- 🔍 **Free Port Finder (`kport free`)**: Suggests available open ports if you want to avoid killing active servers.
- 🐳 **Docker Integration**: Automatically resolves container names when ports are bound to `docker-proxy`.
- 🛡️ **Safety Guardrails**: Never kills PID 1 (`init`/`systemd`) or caller process; warns on protected ports (SSH `22`, DNS `53`).
- 🐚 **Shell Autocompletions**: Instant dynamic `<TAB>` completion for `bash`, `zsh`, `fish`, and `powershell`.
- 📦 **Go Library Included**: Clean, idiomatic Go API to embed into your own Go tools and test suites.

---

## 📥 Installation

### Option 1: One-Line Installer (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/tyzrex/kport/main/install.sh | sh
```

### Option 2: Using `go install`

```bash
# Install kport
go install github.com/tyzrex/kport/cmd/kport@latest

# Or install as killport
go install github.com/tyzrex/kport/cmd/killport@latest
```

### Option 3: Go Library Import

```bash
go get github.com/tyzrex/kport
```

---

## 💻 CLI Usage

```bash
# Kill a port
kport 3000
kport :3000

# Kill multiple ports or ranges
kport 3000 8080 5432
kport 3000-3005
kport 3000,8080,9000

# Kill by framework preset
kport next          # Next.js (port 3000)
kport vite          # Vite / SvelteKit (port 5173)
kport dev           # Common dev ports (3000, 5173, 8080, 8000, 4200, 5000, ...)
kport postgres      # PostgreSQL (port 5432)
kport redis         # Redis (port 6379)

# Auto-detect and kill current project's dev server
kport .

# Kill process and all its child workers (tree kill)
kport -T 3000

# Kill and wait until port is completely released
kport -W 3000

# Watch mode: continuously monitor & kill anything grabbing port 3000
kport -w 3000

# Inspect detailed process info (Memory RSS, CWD, PPID, Tree)
kport --inspect 3000

# Preview without killing (dry-run)
kport -d 3000

# Force kill immediately (SIGKILL)
kport -f 3000

# List all active listening ports
kport -l

# Filter list by process name
kport -l -n node

# Find next available free port
kport free 3000
```

---

## 🖥️ Interactive TUI

Run `kport` with **no arguments** in any terminal (or `kport --ui`):

```
⚡ kport - Interactive Port Manager
[↑/↓/j/k] Navigate  [Space] Select  [a] Toggle All  [Enter] Kill  [f] Force Kill  [q] Quit
🔍 Filter: (Type to filter live...)

  SEL  PORT     PROTO  PID        NAME                 MEMORY     USER       ADDRESS
────────────────────────────────────────────────────────────────────────────────────────
 [ ]   3000     TCP    1823781    next-server          1.5 GB     tyzrex     :::3000
 [✓]   5432     TCP    1204851    docker:postgres      45.2 MB    tyzrex     0.0.0.0:5432
 [ ]   8080     TCP    1808589    java                 340.0 MB   tyzrex     :::8080
────────────────────────────────────────────────────────────────────────────────────────
Showing 3/3 process(es) | 1 selected
```

- **Type anywhere** to filter in real-time.
- **Space** to toggle selection, **a** to select all.
- **Enter** to kill gracefully, **f** for immediate `SIGKILL`.

---

## 📚 Go Library API

Embed `kport` directly into your Go applications:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tyzrex/kport"
)

func main() {
	// 1. One-liner kill
	results, err := kport.Kill(3000, 8080)
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
	kport.KillPorts([]int{3000}, kport.KillOptions{
		Protocol:    kport.ProtocolTCP,
		Force:       true,
		KillTree:    true,                // Clean up child worker processes
		WaitForFree: true,                // Wait for OS socket release
		WaitTimeout: 3 * time.Second,
	})

	// 3. Inspect active processes
	procs, _ := kport.Find(3000)
	for _, p := range procs {
		fmt.Printf("PID: %d | Name: %s | Memory: %s | CWD: %s\n",
			p.PID, p.Name, p.MemoryHuman, p.Cwd)
	}

	// 4. Find free port suggestions
	freePort, _ := kport.SuggestNextPort(3000)
	fmt.Printf("Suggested port: %d\n", freePort)

	// 5. Watch mode
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	kport.Watch(ctx, []int{3000}, kport.DefaultKillOptions(), 500*time.Millisecond, func(res kport.KillResult) {
		fmt.Printf("Auto-killed PID %d on port %d\n", res.Process.PID, res.Port)
	})
}
```

---

## 🎛️ Presets Reference

| Preset Name | Target Ports | Framework / Service |
|---|---|---|
| `web` / `dev` | `3000, 3001, 5173, 8000, 8080, 4200, 5000, 8888, 1234, 4321, 9000` | Common development servers |
| `db` / `database` | `5432, 3306, 27017, 6379, 1433, 9200, 9300` | Common databases |
| `next` / `react` / `remix` / `rails` | `3000` | Next.js, React, Remix, Ruby on Rails |
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

## 🐚 Shell Autocompletions

Generate completion scripts for your shell:

### Bash
```bash
source <(kport completion bash)
```

### Zsh
```bash
source <(kport completion zsh)
```

### Fish
```bash
kport completion fish > ~/.config/fish/completions/kport.fish
```

---

## 🧪 Testing

```bash
go test -v -race ./...
```

---

## 📄 License

MIT © [tyzrex](https://github.com/tyzrex)

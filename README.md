# ⚡ killport

> A high-performance, cross-platform Go library and CLI tool to instantly find and kill processes occupying network ports. Zero dependencies, sub-millisecond execution, and full support for Linux, macOS, and Windows.

Say goodbye to slow `npx kill-port` startup times and cumbersome `lsof -i :3000 | kill` bash pipelines.

---

## ✨ Features

- 🚀 **Blazing Fast**: Native OS inspection (reads `/proc` directly on Linux, `lsof` on macOS, Win32 API/`netstat` on Windows) — terminates in **< 2ms**!
- 📦 **Zero Dependencies**: Pure standard library Go module.
- 🎯 **Flexible Port Arguments**: Supports single ports (`3000`), colon prefixes (`:3000`), port ranges (`3000-3005`), comma-separated lists (`3000,8080,5432`), and any combination.
- 🛡️ **Safety Guardrails**: Automatically protects critical system processes (PID 1, init, systemd) and self/calling process.
- ⏱️ **Graceful Termination**: Sends `SIGTERM` first with a configurable timeout, cleanly escalating to `SIGKILL` only if the process refuses to terminate.
- 🔍 **Interactive & Dry-Run Modes**: Preview what will be killed with `-d / --dry-run` or confirm interactively with `-i / --interactive`.
- 📋 **Port Inspector**: List all active listening ports with `-l / --list` in a beautiful colorized table.
- 🤖 **JSON Output**: Full `-j / --json` mode for scripting, automation, and CI/CD pipelines.

---

## 📥 Installation

### CLI Tool

Install globally via `go install`:

```bash
go install github.com/tyzrex/killport/cmd/killport@latest
```

*(You can also alias it to `kport` in your `~/.bashrc` or `~/.zshrc`: `alias kport=killport`)*

### Go Library

Add to your Go project:

```bash
go get github.com/tyzrex/killport
```

---

## 💻 CLI Usage

```bash
# Kill a single port (TCP & UDP)
killport 3000

# Using alias 'kport' or colon prefix
kport :3000

# Kill multiple ports at once
killport 3000 8080 5432

# Kill port range
killport 3000-3005

# Kill comma-separated ports
killport 3000,8080,9000

# Mixed syntax
killport :3000 8000-8002 5432,6379

# Force kill immediately (SIGKILL)
killport -f 3000

# Preview processes without killing (dry-run)
killport -d 3000

# List active listening ports on the system
killport -l

# Interactive confirmation prompt
killport -i 3000 8080

# Machine-readable JSON output
killport -j 3000
```

### CLI Options

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--force` | `-f` | Force kill immediately (`SIGKILL`) |
| `--graceful` | `-g` | Graceful shutdown (`SIGTERM` first, then `SIGKILL`) |
| `--timeout <dur>` | `-t` | Graceful shutdown timeout (default: `1.5s`) |
| `--protocol <p>` | `-p` | Filter protocol: `tcp`, `udp`, or `all` (default: `all`) |
| `--dry-run` | `-d` | Preview processes without killing |
| `--list` | `-l` | List listening processes |
| `--interactive` | `-i` | Confirm each process interactively |
| `--json` | `-j` | Output result as structured JSON |
| `--verbose` | `-v` | Show full command lines and executable paths |
| `--quiet` | `-q` | Silent execution (returns exit code) |
| `--no-color` | | Disable colored terminal output |
| `--version` | | Print version |

---

## 📚 Go Library Usage

### 1. One-Liner Kill

```go
package main

import (
	"fmt"
	"log"

	"github.com/tyzrex/killport"
)

func main() {
	// Kill a single port
	results, err := killport.Kill(3000)
	if err != nil {
		log.Fatalf("Error killing port: %v", err)
	}

	for _, res := range results {
		if res.Killed {
			fmt.Printf("Killed %s (PID %d) on port %d [%s]\n",
				res.Process.Name, res.Process.PID, res.Port, res.Signal)
		}
	}

	// Kill multiple ports
	killport.Kill(3000, 8080, 5432)
}
```

### 2. Custom Kill Options

```go
results, err := killport.KillPorts([]int{3000, 8080}, killport.KillOptions{
    Protocol: killport.ProtocolTCP, // ProtocolTCP, ProtocolUDP, ProtocolAll
    Force:    true,                 // Immediate SIGKILL
    Timeout:  2 * time.Second,      // Graceful timeout before SIGKILL
    DryRun:   false,
})
```

### 3. Discover Processes (Inspect Ports)

```go
// Find processes listening on port 3000
procs, err := killport.Find(3000)
for _, p := range procs {
    fmt.Printf("PID: %d | Name: %s | Cmd: %s | Address: %s\n",
        p.PID, p.Name, p.CommandLine, p.Address)
}

// Check if a port is in use
inUse, _ := killport.IsPortInUse(3000)
if inUse {
    fmt.Println("Port 3000 is occupied!")
}

// Find all listening processes across the entire machine
allProcs, err := killport.FindAll()
```

---

## 🏗️ Architecture

- **Linux**: Primary resolution scans `/proc/net/tcp{,6}` & `/proc/net/udp{,6}` mapped to `/proc/<pid>/fd/*` socket inodes. Automatic fallbacks to `ss`, `lsof`, and `fuser`.
- **macOS**: Formatted `lsof -F pcn` and `netstat` lookup with process table inspection.
- **Windows**: Win32 network table inspection via `netstat -ano` / `tasklist` and process termination via `taskkill /F /T` & `os.Process.Kill`.

---

## 🧪 Testing

Run the test suite:

```bash
go test -v ./...
```

---

## 📄 License

MIT

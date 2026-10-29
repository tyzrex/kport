//go:build darwin

package proc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tyzrex/kport/types"
)

// Metadata is collected once per snapshot, not once per socket or process.
// CWD and memory remain fresh; caching CWD could misidentify project ownership.
func enrichDarwin(opts types.FindOptions, ps []types.Process) ([]types.Process, error) {
	ids := map[int]string{}
	var pids []string
	for _, p := range ps {
		if _, ok := ids[p.PID]; !ok {
			ids[p.PID] = StartIdentity(p.PID)
			pids = append(pids, strconv.Itoa(p.PID))
		}
	}
	out, err := Output(opts.Context, "ps", "-axo", "pid=,ppid=,rss=,user=,comm=")
	if err != nil {
		return nil, fmt.Errorf("read process metadata: %w", err)
	}
	metadata, parents := parseDarwinMetadata(out)
	commands, err := Output(opts.Context, "ps", "-axo", "pid=,command=")
	if err != nil {
		return nil, fmt.Errorf("read process commands: %w", err)
	}
	for _, line := range strings.Split(string(commands), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		p := metadata[pid]
		p.CommandLine = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))
		metadata[pid] = p
	}
	// -a intersects the PID selection with cwd; otherwise lsof uses OR semantics.
	cwd, err := lsofOutput(opts.Context, "lsof", "-a", "-p", strings.Join(pids, ","), "-d", "cwd", "-Fn")
	if err != nil {
		return nil, fmt.Errorf("read process working directories: %w", err)
	}
	pid := 0
	for _, line := range strings.Split(string(cwd), "\n") {
		if len(line) < 2 {
			continue
		}
		if line[0] == 'p' {
			pid, _ = strconv.Atoi(line[1:])
		}
		if line[0] == 'n' {
			p := metadata[pid]
			p.Cwd = line[1:]
			metadata[pid] = p
		}
	}
	var result []types.Process
	for _, socket := range ps {
		id := ids[socket.PID]
		if id == "" {
			// Retain visible sockets whose process metadata is inaccessible. Actions
			// refuse these records instead of presenting an empty port list.
			result = append(result, socket)
			continue
		}
		if StartIdentity(socket.PID) != id {
			continue
		}
		p := metadata[socket.PID]
		if p.Name == "" {
			p.Name = socket.Name
		}
		p.PID, p.StartID = socket.PID, id
		p.Port, p.Protocol, p.Address, p.State = socket.Port, socket.Protocol, socket.Address, socket.State
		if opts.IncludeTree {
			p.ChildrenPIDs = descendants(socket.PID, parents)
		}
		result = append(result, p)
	}
	return result, nil
}

func parseDarwinMetadata(out []byte) (map[int]types.Process, map[int][]int) {
	metadata := map[int]types.Process{}
	parents := map[int][]int{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			continue
		}
		ppid, _ := strconv.Atoi(f[1])
		rss, _ := strconv.ParseUint(f[2], 10, 64)
		exe := strings.Join(f[4:], " ")
		name := exe
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		metadata[pid] = types.Process{PID: pid, PPID: ppid, Exe: exe, Name: name, User: f[3], MemoryRSS: rss * 1024, MemoryHuman: FormatBytes(rss * 1024)}
		parents[ppid] = append(parents[ppid], pid)
	}
	return metadata, parents
}

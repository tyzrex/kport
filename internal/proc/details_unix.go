//go:build !windows

package proc

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func GetProcessDetails(pid int) (name, cmdline, exe, user string, ppid int, cwd string, rss uint64) {
	return GetProcessDetailsContext(context.Background(), pid)
}
func GetProcessDetailsContext(ctx context.Context, pid int) (name, cmdline, exe, user string, ppid int, cwd string, rss uint64) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	if runtime.GOOS != "darwin" {
		return getProcDetails(pid)
	}
	out, err := Output(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,rss=,user=,comm=")
	if err != nil {
		return
	}
	f := strings.Fields(string(out))
	if len(f) < 4 {
		return
	}
	ppid, _ = strconv.Atoi(f[0])
	rss, _ = strconv.ParseUint(f[1], 10, 64)
	rss *= 1024
	user = f[2]
	exe = strings.Join(f[3:], " ")
	name = exe
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	out, _ = Output(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "command=")
	cmdline = strings.TrimSpace(string(out))
	out, _ = Output(ctx, "lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			cwd = line[1:]
			break
		}
	}
	return
}

// StartIdentity changes when a PID is reused. Empty means identity could not be read.
func startUnixIdentity(pid int) string {
	if runtime.GOOS == "linux" {
		b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if e != nil {
			return ""
		}
		i := strings.LastIndex(string(b), ")")
		if i < 0 {
			return ""
		}
		f := strings.Fields(string(b)[i+1:])
		if len(f) > 19 {
			return f[19]
		}
		return ""
	}
	out, e := Output(context.Background(), "ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func FindProcessChildren(pid int) []int { return FindProcessChildrenContext(context.Background(), pid) }
func FindProcessChildrenContext(ctx context.Context, pid int) []int {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil
	}
	if runtime.GOOS != "darwin" {
		return findProcChildren(pid)
	}
	out, e := Output(ctx, "ps", "-axo", "pid=,ppid=")
	if e != nil {
		return nil
	}
	parents := map[int][]int{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 {
			c, _ := strconv.Atoi(f[0])
			p, _ := strconv.Atoi(f[1])
			parents[p] = append(parents[p], c)
		}
	}
	var result []int
	seen := map[int]bool{pid: true}
	var visit func(int)
	visit = func(p int) {
		for _, c := range parents[p] {
			if !seen[c] {
				seen[c] = true
				visit(c)
				result = append(result, c)
			}
		}
	}
	visit(pid)
	return result
}

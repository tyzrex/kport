//go:build windows

package proc

import (
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"unsafe"
)

func StartIdentity(pid int) string {
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if e != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var created, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exit, &kernel, &user) != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime)
}
func GetProcessDetails(pid int) (name, cmdline, exe, user string, ppid int, cwd string, rss uint64) {
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if e != nil {
		return
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
		exe = windows.UTF16ToString(buf[:n])
		name = filepath.Base(exe)
	}
	var token windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token) == nil {
		if tu, e := token.GetTokenUser(); e == nil {
			if account, _, _, e := tu.User.Sid.LookupAccount(""); e == nil {
				user = account
			}
		}
		token.Close()
	}
	// Windows does not expose another process's CWD through the limited-information API.
	snap, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for e = windows.Process32First(snap, &entry); e == nil; e = windows.Process32Next(snap, &entry) {
		if int(entry.ProcessID) == pid {
			ppid = int(entry.ParentProcessID)
			break
		}
	}
	return
}
func FindProcessChildren(pid int) []int {
	snap, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	parents := map[int][]int{}
	for e = windows.Process32First(snap, &entry); e == nil; e = windows.Process32Next(snap, &entry) {
		parents[int(entry.ParentProcessID)] = append(parents[int(entry.ParentProcessID)], int(entry.ProcessID))
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

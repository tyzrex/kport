//go:build darwin

package proc

import (
	"fmt"
	"golang.org/x/sys/unix"
)

// Use kernel microsecond precision rather than ps's second-resolution lstart.
func StartIdentity(pid int) string {
	info, e := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if e != nil || int(info.Proc.P_pid) != pid {
		return ""
	}
	return fmt.Sprintf("%d:%d", info.Proc.P_starttime.Sec, info.Proc.P_starttime.Usec)
}

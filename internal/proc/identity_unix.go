//go:build !windows && !darwin

package proc

func StartIdentity(pid int) string { return startUnixIdentity(pid) }

//go:build windows

package tui

import (
	"golang.org/x/sys/windows"
	"os"
)

func enableOutput() (func(), error) {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if e := windows.GetConsoleMode(h, &mode); e != nil {
		return nil, e
	}
	if e := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); e != nil {
		return nil, e
	}
	return func() { _ = windows.SetConsoleMode(h, mode) }, nil
}

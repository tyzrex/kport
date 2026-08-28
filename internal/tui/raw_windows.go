//go:build windows

package tui

type termios struct{}

func enableRawMode(fd int) (*termios, error) {
	return nil, nil
}

func restoreMode(fd int, old *termios) {}

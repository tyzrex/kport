//go:build !windows

package tui

func enableOutput() (func(), error) { return func() {}, nil }

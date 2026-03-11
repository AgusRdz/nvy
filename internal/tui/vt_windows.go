//go:build windows

package tui

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVTInput re-enables VT input on stdin after term.MakeRaw clears it.
// Without this, arrow keys don't send ANSI escape sequences on Windows.
func enableVTInput() {
	stdin := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdin, &mode); err == nil {
		_ = windows.SetConsoleMode(stdin, mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}
}

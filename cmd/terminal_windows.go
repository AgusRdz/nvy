//go:build windows

package cmd

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT enables ANSI/VT processing on the Windows console.
// ENABLE_VIRTUAL_TERMINAL_PROCESSING — stdout renders colors and cursor movement.
// ENABLE_VIRTUAL_TERMINAL_INPUT     — stdin sends VT sequences for arrow keys etc.
func enableVT() {
	stdout := windows.Handle(os.Stdout.Fd())
	var outMode uint32
	if err := windows.GetConsoleMode(stdout, &outMode); err == nil {
		_ = windows.SetConsoleMode(stdout, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}

	stdin := windows.Handle(os.Stdin.Fd())
	var inMode uint32
	if err := windows.GetConsoleMode(stdin, &inMode); err == nil {
		_ = windows.SetConsoleMode(stdin, inMode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}
}

// enableVTInput re-enables VT input on stdin after term.MakeRaw clears it.
func enableVTInput() {
	stdin := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdin, &mode); err == nil {
		_ = windows.SetConsoleMode(stdin, mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}
}

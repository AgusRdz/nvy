//go:build windows

package cmd

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT enables ANSI/VT processing on the Windows console.
// ENABLE_VIRTUAL_TERMINAL_PROCESSING — stdout renders colors and cursor movement.
// ENABLE_VIRTUAL_TERMINAL_INPUT     — stdin sends VT sequences for arrow keys etc.
// The returned func restores the original modes; without it the parent shell
// is left with VT input on (arrows echo ^[[A, Backspace deletes a whole word).
func enableVT() (restore func()) {
	stdout := windows.Handle(os.Stdout.Fd())
	stdin := windows.Handle(os.Stdin.Fd())
	var outMode, inMode uint32
	outErr := windows.GetConsoleMode(stdout, &outMode)
	inErr := windows.GetConsoleMode(stdin, &inMode)

	if outErr == nil {
		_ = windows.SetConsoleMode(stdout, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	if inErr == nil {
		_ = windows.SetConsoleMode(stdin, inMode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}

	return func() {
		if inErr == nil {
			_ = windows.SetConsoleMode(stdin, inMode)
		}
		if outErr == nil {
			_ = windows.SetConsoleMode(stdout, outMode)
		}
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

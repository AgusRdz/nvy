//go:build !windows

package cmd

func enableVT() func() { return func() {} }
func enableVTInput()   {}

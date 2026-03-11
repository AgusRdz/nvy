package platform

// Platform abstracts OS-level env var and PATH operations.
type Platform interface {
	// ApplyGlobalVar writes the var to the OS-level user environment.
	// On Windows: HKCU\Environment registry key.
	// On Unix: no-op (shell hook reads global.json).
	ApplyGlobalVar(key, value string) error

	// RemoveGlobalVar removes the var from the OS-level user environment.
	RemoveGlobalVar(key string) error

	// Phase 2+
	AddToPath(entry string) error
	RemoveFromPath(entry string) error
	GetPath() ([]string, error)
	ShellHookScript() string
	ShellConfigPath() string
	RegisterBackgroundTask(binaryPath string) error
}

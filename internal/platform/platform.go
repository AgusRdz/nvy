package platform

// Platform abstracts OS-level env var and PATH operations.
type Platform interface {
	// ApplyGlobalVar writes the var to the OS-level user environment.
	// On Windows: HKCU\Environment registry key.
	// On Unix: no-op (shell hook reads global.json).
	ApplyGlobalVar(key, value string) error

	// RemoveGlobalVar removes the var from the OS-level user environment.
	RemoveGlobalVar(key string) error

	// ExternalVars returns the raw OS-level user environment view: every
	// name/value nvy did not necessarily set. It does NOT subtract nvy-managed
	// keys — callers must do that against global.json themselves.
	ExternalVars() (map[string]string, error)

	// Phase 2+
	AddToPath(entry string) error
	RemoveFromPath(entry string) error
	GetPath() ([]string, error)
	ShellHookScript() string
	ShellConfigPath() string
	RegisterBackgroundTask(binaryPath string) error

	// BackgroundTaskInstalled reports whether the daily expiration-check
	// background task is currently registered.
	BackgroundTaskInstalled() (bool, error)

	// RemoveBackgroundTask unregisters the daily expiration-check background
	// task. It is a no-op (not an error) if the task isn't registered.
	RemoveBackgroundTask() error
}

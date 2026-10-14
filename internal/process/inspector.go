package process

import "context"

// ActivePort represents a system-wide listening port and its process owner.
type ActivePort struct {
	Port     int
	Protocol string
	PID      int
	Name     string
	Verified bool
}

// Info holds details about the process listening on a port.
type Info struct {
	PID      int    // Process ID if found
	Name     string // Process name or executable if available
	Verified bool   // True if process identity was successfully confirmed
	Message  string // Diagnostic message explaining inspection outcome
}

// TerminateProcess terminates a process by its PID using OS-level commands.
func TerminateProcess(ctx context.Context, pid int) error {
	return terminateProcessPlatform(ctx, pid)
}

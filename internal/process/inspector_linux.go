//go:build linux

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FindProcessByPort inspects Linux network sockets using 'ss' or 'lsof'.
func FindProcessByPort(ctx context.Context, port int) (Info, error) {
	// Try 'ss' first (standard on modern Linux distributions)
	// Example command: ss -tlnp 'sport = :8080'
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "ss", "-tlnp", fmt.Sprintf("sport = :%d", port))
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "users:(") {
				// Parse process name and PID from ss output, e.g., users:(("node",pid=1234,fd=18))
				if info, ok := parseSSUserField(line); ok {
					return info, nil
				}
			}
		}
	}

	// Fallback to 'lsof' if ss failed or isn't installed
	// Example command: lsof -iTCP:8080 -sTCP:LISTEN -P -n -Fpcn
	cmdCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()

	cmdLsof := exec.CommandContext(cmdCtx2, "lsof", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-P", "-n")
	lsofOut, lsofErr := cmdLsof.Output()
	if lsofErr == nil {
		if info, ok := parseLsofOutput(string(lsofOut)); ok {
			return info, nil
		}
	}

	return Info{
		Verified: false,
		Message:  "Could not determine process (requires 'ss' or 'lsof', or insufficient permissions)",
	}, nil
}

// ListActivePorts queries all listening TCP ports on Linux using 'ss'.
func ListActivePorts(ctx context.Context) ([]ActivePort, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute ss: %w", err)
	}

	var ports []ActivePort
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Local address is typically fields[3], e.g., 127.0.0.1:8080 or [::]:8080
		addr := fields[3]
		port, err := extractPortFromAddress(addr)
		if err != nil {
			continue
		}

		pid := 0
		procName := "unknown"
		verified := false

		if strings.Contains(line, "users:(") {
			if info, ok := parseSSUserField(line); ok {
				pid = info.PID
				procName = info.Name
				verified = info.Verified
			}
		}

		ports = append(ports, ActivePort{
			Port:     port,
			Protocol: "tcp",
			PID:      pid,
			Name:     procName,
			Verified: verified,
		})
	}

	return ports, nil
}

func extractPortFromAddress(addr string) (int, error) {
	lastColon := strings.LastIndex(addr, ":")
	if lastColon == -1 || lastColon >= len(addr)-1 {
		return 0, fmt.Errorf("invalid address format")
	}
	portStr := addr[lastColon+1:]
	var port int
	_, err := fmt.Sscanf(portStr, "%d", &port)
	return port, err
}

func parseSSUserField(line string) (Info, bool) {
	// Simple robust parsing for ss output
	// Format example: ... users:(("node",pid=1234,fd=18))
	start := strings.Index(line, "users:(")
	if start == -1 {
		return Info{}, false
	}
	sub := line[start:]

	// Extract command name in quotes
	qStart := strings.Index(sub, "\"")
	if qStart == -1 {
		return Info{}, false
	}
	qEnd := strings.Index(sub[qStart+1:], "\"")
	if qEnd == -1 {
		return Info{}, false
	}
	procName := sub[qStart+1 : qStart+1+qEnd]

	// Extract PID
	pidStart := strings.Index(sub, "pid=")
	if pidStart == -1 {
		return Info{}, false
	}
	pidStr := sub[pidStart+4:]
	var pid int
	_, err := fmt.Sscanf(pidStr, "%d", &pid)
	if err != nil {
		return Info{}, false
	}

	return Info{
		PID:      pid,
		Name:     procName,
		Verified: true,
		Message:  fmt.Sprintf("Process identified via ss (PID: %d)", pid),
	}, true
}

func parseLsofOutput(output string) (Info, bool) {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return Info{}, false
	}
	// Header is line 0, data is line 1 onwards
	fields := strings.Fields(lines[1])
	if len(fields) < 9 {
		return Info{}, false
	}

	procName := fields[0]
	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Info{}, false
	}

	return Info{
		PID:      pid,
		Name:     procName,
		Verified: true,
		Message:  fmt.Sprintf("Process identified via lsof (PID: %d)", pid),
	}, true
}

func terminateProcessPlatform(ctx context.Context, pid int) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "kill", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

//go:build darwin

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FindProcessByPort inspects macOS network sockets using 'lsof'.
func FindProcessByPort(ctx context.Context, port int) (Info, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Example command: lsof -iTCP:8080 -sTCP:LISTEN -P -n
	cmd := exec.CommandContext(cmdCtx, "lsof", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-P", "-n")
	output, err := cmd.Output()
	if err != nil {
		return Info{
			Verified: false,
			Message:  "Could not determine process (lsof failed or insufficient permissions)",
		}, nil
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return Info{
			Verified: false,
			Message:  "No listener found via lsof",
		}, nil
	}

	// Parse lsof output columns: COMMAND, PID, USER, FD, TYPE, DEVICE, SIZE/OFF, NODE, NAME
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return Info{
			Verified: false,
			Message:  "Unable to parse lsof output format",
		}, nil
	}

	procName := fields[0]
	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Info{
			Verified: false,
			Message:  "Unable to parse process PID",
		}, nil
	}

	return Info{
		PID:      pid,
		Name:     procName,
		Verified: true,
		Message:  fmt.Sprintf("Process identified via lsof (PID: %d)", pid),
	}, nil
}

// ListActivePorts queries all listening TCP ports on macOS using 'lsof'.
func ListActivePorts(ctx context.Context) ([]ActivePort, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute lsof: %w", err)
	}

	var ports []ActivePort
	lines := strings.Split(string(output), "\n")
	// Skip header line
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		procName := fields[0]
		pid, _ := strconv.Atoi(fields[1])
		nameField := fields[8] // e.g. *:8080 or 127.0.0.1:3000

		port, err := extractPortFromAddress(nameField)
		if err != nil {
			continue
		}

		ports = append(ports, ActivePort{
			Port:     port,
			Protocol: "tcp",
			PID:      pid,
			Name:     procName,
			Verified: true,
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

// terminateProcessPlatform terminates a process by PID on macOS using 'kill'.
func terminateProcessPlatform(ctx context.Context, pid int) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "kill", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

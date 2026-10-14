//go:build windows

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FindProcessByPort inspects Windows network sockets using 'netstat' and 'tasklist'.
func FindProcessByPort(ctx context.Context, port int) (Info, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Find PID using netstat -ano
	cmd := exec.CommandContext(cmdCtx, "netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return Info{
			Verified: false,
			Message:  "Failed to execute netstat command",
		}, nil
	}

	targetPortStr := fmt.Sprintf(":%d", port)
	lines := strings.Split(string(output), "\n")
	var pidStr string

	for _, line := range lines {
		if strings.Contains(line, "LISTENING") && strings.Contains(line, targetPortStr) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				pidStr = fields[len(fields)-1]
				break
			}
		}
	}

	if pidStr == "" {
		return Info{
			Verified: false,
			Message:  "No process found listening on this port",
		}, nil
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid == 0 {
		return Info{
			Verified: false,
			Message:  fmt.Sprintf("Found invalid PID: %s", pidStr),
		}, nil
	}

	// Resolve process name using tasklist
	cmdCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()

	cmdTask := exec.CommandContext(cmdCtx2, "tasklist", "/fi", fmt.Sprintf("pid eq %d", pid), "/fo", "csv", "/nh")
	taskOut, taskErr := cmdTask.Output()
	if taskErr != nil {
		return Info{
			PID:      pid,
			Verified: true,
			Message:  fmt.Sprintf("PID %d found, but failed to resolve process name", pid),
		}, nil
	}

	procName := "unknown"
	taskLines := strings.Split(string(taskOut), "\n")
	if len(taskLines) > 0 && strings.Contains(taskLines[0], "\"") {
		parts := strings.Split(taskLines[0], "\"")
		if len(parts) > 1 {
			procName = parts[1]
		}
	}

	return Info{
		PID:      pid,
		Name:     procName,
		Verified: true,
		Message:  fmt.Sprintf("Process identified via netstat/tasklist (PID: %d)", pid),
	}, nil
}

// ListActivePorts queries all listening TCP ports on Windows using 'netstat'.
func ListActivePorts(ctx context.Context) ([]ActivePort, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute netstat: %w", err)
	}

	var ports []ActivePort
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		localAddr := fields[1]
		port, err := extractPortFromAddress(localAddr)
		if err != nil {
			continue
		}

		pidStr := fields[len(fields)-1]
		pid, _ := strconv.Atoi(pidStr)

		ports = append(ports, ActivePort{
			Port:     port,
			Protocol: "tcp",
			PID:      pid,
			Name:     "unknown (run inspect for details)",
			Verified: pid > 0,
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

func terminateProcessPlatform(ctx context.Context, pid int) error {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Use taskkill /F to forcefully terminate the process by PID on Windows
	cmd := exec.CommandContext(cmdCtx, "taskkill", "/F", "/PID", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

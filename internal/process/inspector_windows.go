//go:build windows

package process

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// listener is one TCP listening socket parsed from netstat output.
type listener struct {
	port int
	pid  int
}

// FindProcessByPort identifies the process listening on a TCP port using
// netstat and tasklist.
func FindProcessByPort(ctx context.Context, port int) (Info, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	output, err := exec.CommandContext(cmdCtx, "netstat", "-ano").Output()
	if err != nil {
		return Info{Verified: false, Message: "Failed to execute netstat command"}, nil
	}

	pid := 0
	for _, l := range parseListeners(output) {
		if l.port == port {
			pid = l.pid
			break
		}
	}
	if pid == 0 {
		return Info{Verified: false, Message: "No process found listening on this port"}, nil
	}

	name, resolved := resolveProcessName(ctx, pid)
	if !resolved {
		return Info{
			PID:      pid,
			Verified: true,
			Message:  fmt.Sprintf("PID %d found, but failed to resolve process name", pid),
		}, nil
	}

	return Info{
		PID:      pid,
		Name:     name,
		Verified: true,
		Message:  fmt.Sprintf("Process identified via netstat/tasklist (PID: %d)", pid),
	}, nil
}

// ListActivePorts queries all listening TCP ports on Windows using 'netstat'.
func ListActivePorts(ctx context.Context) ([]ActivePort, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(cmdCtx, "netstat", "-ano").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute netstat: %w", err)
	}

	listeners := parseListeners(output)

	// Name resolution is best-effort. If tasklist fails, names is nil and the
	// ports are still returned without process names.
	names, _ := resolveProcessNames(ctx)

	ports := make([]ActivePort, 0, len(listeners))
	for _, l := range listeners {
		name, ok := names[l.pid]
		ports = append(ports, ActivePort{
			Port:     l.port,
			Protocol: "tcp",
			PID:      l.pid,
			Name:     name,
			Verified: ok,
		})
	}

	return ports, nil
}

// parseListeners extracts TCP listening sockets from `netstat -ano` output.
// Listening sockets are identified by their all-zero foreign endpoint rather
// than the localized "LISTENING" label. Ports are compared exactly, not by
// substring. The same process often listens on both 0.0.0.0 and [::], so
// duplicates by port and PID are removed.
func parseListeners(output []byte) []listener {
	var out []listener
	seen := make(map[listener]bool)

	for _, line := range strings.Split(string(output), "\n") {
		// Expected columns: Proto LocalAddress ForeignAddress State PID
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "TCP" || !isZeroEndpoint(fields[2]) {
			continue
		}

		port, err := extractPortFromAddress(fields[1])
		if err != nil {
			continue
		}

		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}

		l := listener{port: port, pid: pid}
		if seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}

	return out
}

func isZeroEndpoint(addr string) bool {
	return addr == "0.0.0.0:0" || addr == "[::]:0"
}

func extractPortFromAddress(addr string) (int, error) {
	i := strings.LastIndex(addr, ":")
	if i == -1 || i == len(addr)-1 {
		return 0, fmt.Errorf("invalid address format: %q", addr)
	}
	return strconv.Atoi(addr[i+1:])
}

// resolveProcessName returns the image name for one PID. The second return
// value is false if the name could not be resolved.
func resolveProcessName(ctx context.Context, pid int) (string, bool) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cmdCtx, "tasklist",
		"/fi", fmt.Sprintf("pid eq %d", pid), "/fo", "csv", "/nh").Output()
	if err != nil {
		return "", false
	}

	r := csv.NewReader(bytes.NewReader(out))
	r.FieldsPerRecord = -1
	rec, err := r.Read()
	// Require the PID column to match. This rejects the "INFO: No tasks..."
	// message, which some locales print instead of a CSV row.
	if err != nil || len(rec) < 2 || strings.TrimSpace(rec[1]) != strconv.Itoa(pid) {
		return "", false
	}
	return rec[0], true
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

// resolveProcessNames maps PIDs to image names using one tasklist call.
// PIDs that are missing from the output (exited, or access denied) are omitted.
func resolveProcessNames(ctx context.Context) (map[int]string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "tasklist", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	r := csv.NewReader(bytes.NewReader(output))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse tasklist output: %w", err)
	}

	names := make(map[int]string, len(records))
	for _, rec := range records {
		// Columns: Image Name, PID, Session Name, Session#, Mem Usage
		if len(rec) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			continue
		}
		names[pid] = rec[0]
	}
	return names, nil
}

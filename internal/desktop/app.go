package desktop

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/senpai295/portpilot/internal/config"
	"github.com/senpai295/portpilot/internal/ports"
	"github.com/senpai295/portpilot/internal/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	minTimeoutMS = 100
	maxTimeoutMS = 10000

	// Matches the CLI watch command's default polling interval.
	portWatchInterval = 2 * time.Second

	listTimeout    = 5 * time.Second
	ownerTimeout   = 3 * time.Second
	terminateLimit = 3 * time.Second

	// EventPortsUpdated is emitted to the frontend when the listening-port
	// set changes. Must match the name used in src/main.ts.
	EventPortsUpdated = "ports:updated"
)

// Statuses returned by CheckPort, KillPort, and the desktop layer.
const (
	StatusInvalid      = "INVALID"
	StatusNotFound     = "NOT_FOUND"
	StatusUnverified   = "UNVERIFIED"
	StatusOwnerChanged = "OWNER_CHANGED"
	StatusKilled       = "KILLED"
	StatusKillFailed   = "KILL_FAILED"
)

type App struct {
	ctx context.Context
}

// PortCheckResult is the JSON shape consumed by the frontend.
type PortCheckResult struct {
	Port    int    `json:"port"`
	Service string `json:"service,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ActivePortDTO describes one listening TCP port for the sidebar.
type ActivePortDTO struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	PID      int    `json:"pid"`
	Process  string `json:"process,omitempty"`
	Service  string `json:"service,omitempty"`
	Verified bool   `json:"verified"`
}

// ListPortsResult wraps the port list so errors reach the UI without a
// rejected promise.
type ListPortsResult struct {
	Ports []ActivePortDTO `json:"ports"`
	Error string          `json:"error,omitempty"`
}

// PortOwner describes the process behind a port, shown in the kill dialog.
// Kill must be called with the PID from this value.
type PortOwner struct {
	Port     int    `json:"port"`
	PID      int    `json:"pid"`
	Process  string `json:"process"`
	Verified bool   `json:"verified"`
	Service  string `json:"service,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

// KillResult reports the outcome of KillPort.
type KillResult struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	Message string `json:"message"`
}

func NewApp() *App {
	return &App{}
}

// Startup stores the Wails context and starts the background port watcher.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	loadCustomServices("") // Load from default config path
	go a.watchPorts(ctx)
}

func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// CheckPort inspects a local TCP port using the shared internal/ports checker.
// Status is one of AVAILABLE, OCCUPIED, ERROR, or INVALID.
func (a *App) CheckPort(port int, timeoutMS int) PortCheckResult {
	if timeoutMS < minTimeoutMS || timeoutMS > maxTimeoutMS {
		return PortCheckResult{
			Port:    port,
			Service: ports.GetKnownService(port),
			Status:  StatusInvalid,
			Message: "Timeout must be between 100 and 10000 milliseconds.",
		}
	}

	timeout := time.Duration(timeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(a.baseCtx(), timeout)
	defer cancel()

	res := ports.CheckPort(ctx, port, timeout)
	service := ports.GetKnownService(res.Port)

	return PortCheckResult{
		Port:    res.Port,
		Service: service,
		Status:  string(res.Status),
		Message: res.Message,
	}
}

// ListPorts returns active listening TCP ports, sorted by port number.
// The frontend calls this once on load; updates after that arrive as events.
func (a *App) ListPorts() ListPortsResult {
	return a.listPorts(a.baseCtx())
}

func (a *App) listPorts(parent context.Context) ListPortsResult {
	ctx, cancel := context.WithTimeout(parent, listTimeout)
	defer cancel()

	active, err := process.ListActivePorts(ctx)
	if err != nil {
		return ListPortsResult{Ports: []ActivePortDTO{}, Error: err.Error()}
	}

	out := make([]ActivePortDTO, 0, len(active))
	for _, p := range active {
		out = append(out, ActivePortDTO{
			Port:     p.Port,
			Protocol: p.Protocol,
			PID:      p.PID,
			Process:  p.Name,
			Service:  ports.GetKnownService(p.Port),
			Verified: p.Verified,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Port < out[j].Port
	})

	return ListPortsResult{Ports: out}
}

// watchPorts polls the listening-port set and emits EventPortsUpdated only
// when it changes, mirroring the change-detection in the CLI watch command.
func (a *App) watchPorts(ctx context.Context) {
	ticker := time.NewTicker(portWatchInterval)
	defer ticker.Stop()

	var last string
	emit := func() {
		res := a.listPorts(ctx)
		key := portsFingerprint(res)
		if key != last {
			last = key
			runtime.EventsEmit(ctx, EventPortsUpdated, res)
		}
	}

	emit()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emit()
		}
	}
}

// portsFingerprint produces a stable string for comparing two port lists.
// The list is already sorted by port, so equal sets produce equal strings.
func portsFingerprint(r ListPortsResult) string {
	var b strings.Builder
	b.WriteString(r.Error)
	b.WriteByte('|')
	for _, p := range r.Ports {
		fmt.Fprintf(&b, "%d/%s/%d/%s/%s/%t;", p.Port, p.Protocol, p.PID, p.Process, p.Service, p.Verified)
	}
	return b.String()
}

// GetPortOwner identifies the process listening on a port without killing it.
// The frontend shows this in the confirmation dialog.
func (a *App) GetPortOwner(port int) PortOwner {
	if port < 1 || port > 65535 {
		return PortOwner{
			Port:    port,
			Status:  StatusInvalid,
			Message: "Port must be between 1 and 65535.",
		}
	}

	ctx, cancel := context.WithTimeout(a.baseCtx(), ownerTimeout)
	defer cancel()

	info, err := process.FindProcessByPort(ctx, port)
	if err != nil {
		return PortOwner{
			Port:    port,
			Status:  StatusNotFound,
			Message: fmt.Sprintf("No process found listening on port %d: %v", port, err),
		}
	}

	if !info.Verified || info.PID <= 0 {
		return PortOwner{
			Port:     port,
			PID:      info.PID,
			Process:  info.Name,
			Verified: info.Verified,
			Status:   StatusUnverified,
			Message:  "A process was found but could not be verified, so it will not be terminated. " + info.Message,
		}
	}

	return PortOwner{
		Port:     port,
		Service:  ports.GetKnownService(port),
		PID:      info.PID,
		Process:  info.Name,
		Verified: true,
		Status:   "FOUND",
		Message:  info.Message,
	}
}

// KillPort terminates the process with the given PID, but only if that PID
// still owns the port. The frontend must pass the PID it showed to the user.
func (a *App) KillPort(port int, pid int) KillResult {
	if port < 1 || port > 65535 || pid <= 0 {
		return KillResult{
			Port:    port,
			PID:     pid,
			Status:  StatusInvalid,
			Message: "A valid port and PID are required.",
		}
	}

	// Re-verify ownership right before terminating. This narrows the window
	// in which the port could have changed hands since the dialog was shown.
	ownerCtx, ownerCancel := context.WithTimeout(a.baseCtx(), ownerTimeout)
	info, err := process.FindProcessByPort(ownerCtx, port)
	ownerCancel()

	if err != nil || !info.Verified || info.PID <= 0 {
		return KillResult{
			Port:    port,
			PID:     pid,
			Status:  StatusNotFound,
			Message: "No verified process is listening on this port anymore. Nothing was terminated.",
		}
	}

	if info.PID != pid {
		return KillResult{
			Port:    port,
			PID:     pid,
			Status:  StatusOwnerChanged,
			Message: fmt.Sprintf("Port %d is now owned by PID %d, not PID %d. Nothing was terminated.", port, info.PID, pid),
		}
	}

	termCtx, termCancel := context.WithTimeout(a.baseCtx(), terminateLimit)
	defer termCancel()

	if err := process.TerminateProcess(termCtx, pid); err != nil {
		return KillResult{
			Port:    port,
			PID:     pid,
			Status:  StatusKillFailed,
			Message: fmt.Sprintf("Failed to terminate PID %d: %v", pid, err),
		}
	}

	return KillResult{
		Port:    port,
		PID:     pid,
		Status:  StatusKilled,
		Message: fmt.Sprintf("Terminated PID %d on port %d.", pid, port),
	}
}
func loadCustomServices(configPath string) {
	if configPath == "" {
		configPath = config.DefaultConfigFileName
	}

	if _, err := os.Stat(configPath); err != nil {
		fmt.Printf("[portpilot] config not found: %v\n", err)
		return
	}

	mappings, err := config.LoadConfig(configPath)
	if err != nil {
		return
	}

	for port, service := range mappings {
		ports.KnownServices[port] = service
	}
}

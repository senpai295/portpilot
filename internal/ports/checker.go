package ports

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Status represents the state of a TCP port.
type Status string

const (
	StatusAvailable Status = "AVAILABLE"
	StatusOccupied  Status = "OCCUPIED"
	StatusError     Status = "ERROR"
)

// Result holds the outcome of a port inspection.
type Result struct {
	Port    int
	Status  Status
	Message string
	Error   error
}

// CheckPort inspects whether a single TCP port is available or occupied.
// It combines a non-destructive TCP connection dial with a safe local bind test
// to maximize accuracy while respecting system boundaries.
func CheckPort(ctx context.Context, port int, timeout time.Duration) Result {
	if port < 1 || port > 65535 {
		return Result{
			Port:    port,
			Status:  StatusError,
			Message: "Invalid port number (must be between 1 and 65535)",
			Error:   fmt.Errorf("invalid port number: %d", port),
		}
	}

	address := fmt.Sprintf("127.0.0.1:%d", port)

	// Step 1: Attempt to establish a TCP connection.
	// If this succeeds, an active listener is definitely running on the port.
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err == nil {
		conn.Close()
		return Result{
			Port:    port,
			Status:  StatusOccupied,
			Message: "Active listener detected (connection successful)",
		}
	}

	// Check if context was canceled or timed out
	if ctx.Err() != nil {
		return Result{
			Port:    port,
			Status:  StatusError,
			Message: fmt.Sprintf("Inspection canceled or timed out: %v", ctx.Err()),
			Error:   ctx.Err(),
		}
	}

	// Step 2: Connection failed. To distinguish between "no listener" and
	// other network states, perform a safe local bind test.
	lc := net.ListenConfig{}
	listener, listenErr := lc.Listen(ctx, "tcp", address)
	if listenErr == nil {
		listener.Close()
		return Result{
			Port:    port,
			Status:  StatusAvailable,
			Message: fmt.Sprintf("Port (%d) is available (successfully bound locally)", port),
		}
	}

	// If bind failed, check if the address is already in use
	if isAddressInUse(listenErr) {
		return Result{
			Port:    port,
			Status:  StatusOccupied,
			Message: fmt.Sprintf("Port (%d) is occupied (bind failed: address already in use)", port),
			Error:   listenErr,
		}
	}

	// Other errors (e.g., permission restrictions)
	return Result{
		Port:    port,
		Status:  StatusError,
		Message: fmt.Sprintf("Unable to determine port status: %v", listenErr),
		Error:   listenErr,
	}
}

// isAddressInUse checks if the error indicates EADDRINUSE across platforms.
func isAddressInUse(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr syscall.Errno
		if errors.As(opErr.Err, &sysErr) {
			return sysErr == syscall.EADDRINUSE || sysErr.Error() == syscall.EADDRINUSE.Error()
		}
	}
	return false
}

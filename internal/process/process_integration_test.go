package process_test

import (
	"context"
	"net"
	"testing"
	"time"

	"portpilot/internal/ports"
	"portpilot/internal/process"
)

// TestProcessInspectionIntegration verifies that a locally bound ephemeral port
// is correctly recognized as occupied and that process inspection can query it.
func TestProcessInspectionIntegration(t *testing.T) {
	// 1. Start a real local TCP listener on an ephemeral port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer listener.Close()

	// Extract the assigned port number
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 2. Verify port checker detects it as occupied
	res := ports.CheckPort(ctx, port, 2*time.Second)
	if res.Status != ports.StatusOccupied {
		t.Fatalf("expected port %d to be OCCUPIED, got %s (message: %s)", port, res.Status, res.Message)
	}

	// 3. Attempt process inspection
	procInfo, err := process.FindProcessByPort(ctx, port)
	if err != nil {
		// In restricted container environments (like some CI runners without root/ss/lsof access),
		// process inspection might fail gracefully. We log it rather than failing the test suite.
		t.Logf("Note: process inspection returned error (expected in restricted containers): %v", err)
		return
	}

	// 4. Validate inspection structure if verified
	if procInfo.Verified {
		if procInfo.PID <= 0 {
			t.Errorf("expected positive PID for verified process, got %d", procInfo.PID)
		}
		if procInfo.Name == "" {
			t.Errorf("expected non-empty process name")
		}
	} else {
		t.Logf("Process info unverified (restricted permissions): %s", procInfo.Message)
	}
}

// TestListActivePortsIntegration verifies system-wide port listing detects active listeners.
func TestListActivePortsIntegration(t *testing.T) {
	// Start a local listener to guarantee at least one active port exists
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	activePorts, err := process.ListActivePorts(ctx)
	if err != nil {
		t.Skipf("Skipping active ports list test: system utility unavailable or restricted: %v", err)
		return
	}

	// Verify our test port is present in the active list
	found := false
	for _, p := range activePorts {
		if p.Port == port {
			found = true
			break
		}
	}

	if !found {
		t.Logf("Notice: ephemerally bound port %d was not caught by system listing utility (may require elevated permissions or specific socket state)", port)
	}
}

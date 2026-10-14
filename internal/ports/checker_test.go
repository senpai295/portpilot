package ports

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestCheckPort_InvalidPort(t *testing.T) {
	ctx := context.Background()
	res := CheckPort(ctx, 70000, 1*time.Second)

	if res.Status != StatusError {
		t.Errorf("expected status %s, got %s", StatusError, res.Status)
	}
	if res.Port != 70000 {
		t.Errorf("expected port 70000, got %d", res.Port)
	}
}

func TestCheckPort_OccupiedPort(t *testing.T) {
	// Start a local listener on a random available port for testing
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	ctx := context.Background()
	res := CheckPort(ctx, port, 1*time.Second)

	if res.Status != StatusOccupied {
		t.Errorf("expected port %d to be %s, got %s (message: %s)", port, StatusOccupied, res.Status, res.Message)
	}
}

func TestCheckPort_AvailablePort(t *testing.T) {
	// Find a free port by listening and immediately closing it
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Free it immediately

	ctx := context.Background()
	res := CheckPort(ctx, port, 1*time.Second)

	if res.Status != StatusAvailable {
		t.Errorf("expected port %d to be %s, got %s (message: %s, err: %v)", port, StatusAvailable, res.Status, res.Message, res.Error)
	}
}

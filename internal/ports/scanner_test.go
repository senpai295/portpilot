package ports

import (
	"context"
	"testing"
	"time"
)

func TestGetKnownService(t *testing.T) {
	if desc := GetKnownService(3000); desc != "Node.js Development Server" {
		t.Errorf("expected 'Node.js Development Server' for port 3000, got '%s'", desc)
	}
	if desc := GetKnownService(9999); desc != "" {
		t.Errorf("expected empty string for unknown port, got '%s'", desc)
	}
}

func TestScanRange_InvalidOrder(t *testing.T) {
	ctx := context.Background()
	// Test that start > end is handled gracefully by swapping
	results := ScanRange(ctx, 3005, 3000, 500*time.Millisecond)
	if len(results) != 6 {
		t.Errorf("expected 6 results, got %d", len(results))
	}
}

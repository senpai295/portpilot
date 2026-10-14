package report

import (
	"encoding/json"
	"strings"
	"testing"

	"portpilot/internal/ports"
	"portpilot/internal/process"
)

func TestReportJSONSerialization(t *testing.T) {
	rep := Report{
		GeneratedAt: "2026-03-30T12:00:00Z",
		TotalPorts:  1,
		Occupied:    1,
		Items: []Item{
			{
				Port:    8080,
				Status:  ports.StatusOccupied,
				Service: "Common Web Server",
				Message: "Active listener detected",
				Process: &process.Info{
					PID:      1234,
					Name:     "node",
					Verified: true,
					Message:  "Verified via test",
				},
			},
		},
	}

	jsonStr, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize report to JSON: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("failed to decode generated JSON: %v", err)
	}

	if decoded.TotalPorts != 1 || decoded.Items[0].Port != 8080 {
		t.Errorf("decoded JSON data mismatch")
	}
}

func TestReportMarkdownGeneration(t *testing.T) {
	rep := Report{
		GeneratedAt: "2026-03-30T12:00:00Z",
		TotalPorts:  1,
		Occupied:    1,
		Items: []Item{
			{
				Port:    3000,
				Status:  ports.StatusOccupied,
				Service: "Node.js Development Server",
				Message: "Active listener detected",
			},
		},
	}

	md := rep.ToMarkdown()
	if !strings.Contains(md, "# PortPilot Inspection Report") {
		t.Errorf("markdown missing title")
	}
	if !strings.Contains(md, "3000") || !strings.Contains(md, "Node.js Development Server") {
		t.Errorf("markdown missing table content")
	}
}

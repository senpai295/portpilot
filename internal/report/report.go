package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"portpilot/internal/ports"
	"portpilot/internal/process"
)

// Item represents a complete inspection entry for a single port.
type Item struct {
	Port        int           `json:"port"`
	Status      ports.Status  `json:"status"`
	Service     string        `json:"service,omitempty"`
	Message     string        `json:"message"`
	Process     *process.Info `json:"process,omitempty"`
	ErrorString string        `json:"error,omitempty"`
}

// Report holds metadata and results of a scan or inspection run.
type Report struct {
	GeneratedAt string `json:"generated_at"`
	TotalPorts  int    `json:"total_ports"`
	Occupied    int    `json:"occupied_ports"`
	Items       []Item `json:"items"`
}

// ToJSON converts the report into a pretty-printed JSON string.
func (r *Report) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToMarkdown converts the report into a clean Markdown document.
func (r *Report) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# PortPilot Inspection Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Generated At:** %s\n", r.GeneratedAt))
	sb.WriteString(fmt.Sprintf("- **Total Ports Checked:** %d\n", r.TotalPorts))
	sb.WriteString(fmt.Sprintf("- **Occupied Ports Found:** %d\n\n", r.Occupied))

	sb.WriteString("| Port | Status | Service / Hint | Process / PID | Details |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")

	for _, item := range r.Items {
		service := item.Service
		if service == "" {
			service = "-"
		}

		procInfo := "-"
		if item.Process != nil && item.Process.Verified {
			procInfo = fmt.Sprintf("%s (PID: %d)", item.Process.Name, item.Process.PID)
		} else if item.Process != nil && item.Process.Message != "" {
			procInfo = item.Process.Message
		}

		sb.WriteString(fmt.Sprintf("| %d | **%s** | %s | %s | %s |\n",
			item.Port, item.Status, service, procInfo, item.Message))
	}

	return sb.String()
}

// SaveToFile writes content to a specified file path.
func SaveToFile(filepath, content string) error {
	return os.WriteFile(filepath, []byte(content), 0644)
}

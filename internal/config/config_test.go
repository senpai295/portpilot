package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, DefaultConfigFileName)

	content := `{
		"ports": [
			{"port": 4000, "service": "Team Custom API Server"},
			{"port": 8090, "service": "Team Internal Microservice"}
		]
	}`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	mappings, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if mappings[4000] != "Team Custom API Server" {
		t.Errorf("expected 'Team Custom API Server' for port 4000, got '%s'", mappings[4000])
	}
	if mappings[8090] != "Team Internal Microservice" {
		t.Errorf("expected 'Team Internal Microservice' for port 8090, got '%s'", mappings[8090])
	}
}

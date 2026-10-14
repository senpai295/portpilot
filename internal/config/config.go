package config

import (
	"encoding/json"
	"os"
)

// Mapping represents a custom port-to-service definition.
type Mapping struct {
	Port        int    `json:"port"`
	Service     string `json:"service"`
	Description string `json:"description,omitempty"`
}

// Config represents the schema for .portpilot.json team configuration files.
type Config struct {
	Ports []Mapping `json:"ports"`
}

// DefaultConfigFileName is the standard name of the project config file.
const DefaultConfigFileName = ".portpilot.json"

// LoadConfig reads and parses a configuration file from the given path.
func LoadConfig(filePath string) (map[int]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	customMappings := make(map[int]string)
	for _, m := range cfg.Ports {
		if m.Port > 0 && m.Port <= 65535 && m.Service != "" {
			customMappings[m.Port] = m.Service
		}
	}

	return customMappings, nil
}

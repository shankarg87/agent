package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// MCPConfig contains configuration for all MCP servers
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig represents a single MCP server configuration
type MCPServerConfig struct {
	Name       string        `yaml:"name"`
	Transport  string        `yaml:"transport"` // stdio, http
	Endpoint   string        `yaml:"endpoint"`  // command for stdio, URL for http
	Args       []string      `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Timeout    time.Duration `yaml:"timeout,omitempty"`
	RetryMax   int           `yaml:"retry_max,omitempty"`
	RetryDelay time.Duration `yaml:"retry_delay,omitempty"`
}

// LoadMCPConfig loads MCP server configurations from a YAML file
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg MCPConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	for i := range cfg.Servers {
		if cfg.Servers[i].Timeout == 0 {
			cfg.Servers[i].Timeout = 30 * time.Second
		}
		if cfg.Servers[i].RetryMax == 0 {
			cfg.Servers[i].RetryMax = 3
		}
		if cfg.Servers[i].RetryDelay == 0 {
			cfg.Servers[i].RetryDelay = 1 * time.Second
		}
	}

	return &cfg, nil
}

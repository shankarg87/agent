package config

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigManager provides thread-safe dynamic configuration management
type ConfigManager struct {
	mu           sync.RWMutex
	agentConfig  *AgentConfig
	mcpConfig    *MCPConfig
	configPath   string
	mcpPath      string
	lastReload   time.Time
	watchers     []*fsnotify.Watcher
	stopWatching chan struct{}
}

// NewConfigManager creates a new configuration manager with file watching
func NewConfigManager(agentConfigPath, mcpConfigPath string) (*ConfigManager, error) {
	cm := &ConfigManager{
		configPath:   agentConfigPath,
		mcpPath:      mcpConfigPath,
		stopWatching: make(chan struct{}),
	}

	// Load initial configuration
	if err := cm.loadConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	// Start file watchers
	if err := cm.startWatchers(); err != nil {
		return nil, fmt.Errorf("failed to start file watchers: %w", err)
	}

	return cm, nil
}

// GetAgentConfig returns the current agent configuration (thread-safe)
func (cm *ConfigManager) GetAgentConfig() *AgentConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modifications
	cfg := *cm.agentConfig
	return &cfg
}

// GetMCPConfig returns the current MCP configuration (thread-safe)
func (cm *ConfigManager) GetMCPConfig() *MCPConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modifications
	cfg := *cm.mcpConfig
	return &cfg
}

// GetLastReload returns the timestamp of the last configuration reload
func (cm *ConfigManager) GetLastReload() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastReload
}

// loadConfigs loads both agent and MCP configurations
func (cm *ConfigManager) loadConfigs() error {
	// Load agent config
	agentCfg, err := LoadAgentConfig(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to load agent config from %s: %w", cm.configPath, err)
	}

	// Load MCP config
	mcpCfg, err := LoadMCPConfig(cm.mcpPath)
	if err != nil {
		return fmt.Errorf("failed to load MCP config from %s: %w", cm.mcpPath, err)
	}

	// Validate configurations
	if err := cm.validateConfigs(agentCfg, mcpCfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	cm.mu.Lock()
	cm.agentConfig = agentCfg
	cm.mcpConfig = mcpCfg
	cm.lastReload = time.Now()
	cm.mu.Unlock()

	log.Printf("Configuration loaded successfully - Agent: %s v%s", agentCfg.ProfileName, agentCfg.ProfileVersion)
	return nil
}

// validateConfigs performs basic validation on the loaded configurations
func (cm *ConfigManager) validateConfigs(agentCfg *AgentConfig, mcpCfg *MCPConfig) error {
	if agentCfg == nil {
		return fmt.Errorf("agent configuration is nil")
	}
	if mcpCfg == nil {
		return fmt.Errorf("MCP configuration is nil")
	}

	// Validate agent config has required fields
	if agentCfg.ProfileName == "" {
		return fmt.Errorf("agent profile_name is required")
	}
	if agentCfg.ProfileVersion == "" {
		return fmt.Errorf("agent profile_version is required")
	}
	if agentCfg.PrimaryModel.Provider == "" {
		return fmt.Errorf("agent primary_model.provider is required")
	}

	// Add more validations as needed
	return nil
}

// startWatchers starts file system watchers for configuration files
func (cm *ConfigManager) startWatchers() error {
	// Watch agent config file
	agentWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create agent config watcher: %w", err)
	}

	if err := agentWatcher.Add(filepath.Dir(cm.configPath)); err != nil {
		agentWatcher.Close()
		return fmt.Errorf("failed to watch agent config directory: %w", err)
	}

	// Watch MCP config file
	mcpWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		agentWatcher.Close()
		return fmt.Errorf("failed to create MCP config watcher: %w", err)
	}

	if err := mcpWatcher.Add(filepath.Dir(cm.mcpPath)); err != nil {
		agentWatcher.Close()
		mcpWatcher.Close()
		return fmt.Errorf("failed to watch MCP config directory: %w", err)
	}

	cm.watchers = []*fsnotify.Watcher{agentWatcher, mcpWatcher}

	// Start watching goroutines
	go cm.watchFiles(agentWatcher, cm.configPath, "agent")
	go cm.watchFiles(mcpWatcher, cm.mcpPath, "MCP")

	return nil
}

// watchFiles handles file system events for a specific watcher
func (cm *ConfigManager) watchFiles(watcher *fsnotify.Watcher, targetPath, configType string) {
	defer watcher.Close()

	targetFile := filepath.Base(targetPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only process events for our target file
			if filepath.Base(event.Name) != targetFile {
				continue
			}

			// Process write events (config file updated)
			if event.Has(fsnotify.Write) {
				log.Printf("Detected %s configuration change: %s", configType, event.Name)

				// Debounce rapid file changes (editors often write multiple times)
				time.Sleep(100 * time.Millisecond)

				if err := cm.reloadConfig(); err != nil {
					log.Printf("Failed to reload %s configuration: %v", configType, err)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error for %s config: %v", configType, err)

		case <-cm.stopWatching:
			return
		}
	}
}

// reloadConfig reloads both configuration files
func (cm *ConfigManager) reloadConfig() error {
	return cm.loadConfigs()
}

// Close stops all file watchers and cleans up resources
func (cm *ConfigManager) Close() error {
	close(cm.stopWatching)

	var errs []error
	for _, watcher := range cm.watchers {
		if err := watcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing watchers: %v", errs)
	}

	return nil
}

// NewConfigManagerForTest creates a config manager from already-loaded configs (for testing)
// This skips file watching and is suitable for test environments
func NewConfigManagerForTest(agentConfig *AgentConfig, mcpConfig *MCPConfig) *ConfigManager {
	return &ConfigManager{
		agentConfig:  agentConfig,
		mcpConfig:    mcpConfig,
		lastReload:   time.Now(),
		stopWatching: make(chan struct{}),
	}
}

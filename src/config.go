package main

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration structure
type Config struct {
	WebService WebServiceConfig           `yaml:"web_service"`
	MediaPaths map[string]MediaPathConfig `yaml:"media_paths"`
	Database   DatabaseConfig             `yaml:"database"`
}

// DatabaseConfig contains database specific configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// WebServiceConfig contains web service specific configuration
type WebServiceConfig struct {
	Port int `yaml:"port"`
}

// MediaPathConfig represents a named media path with its properties
type MediaPathConfig struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
}

// Default configuration values
const (
	DefaultPort = 8080
)

var (
	// Global configuration instance
	appConfig *Config
)

// LoadConfig loads the configuration from the specified file path
func LoadConfig(configPath string) (*Config, error) {
	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Create config with default values
	config := &Config{
		WebService: WebServiceConfig{
			Port: DefaultPort,
		},
		MediaPaths: make(map[string]MediaPathConfig),
		Database: DatabaseConfig{
			Path: "default.db",
		},
	}

	// Parse YAML
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// GetConfig returns the global configuration instance, loading it if necessary
func GetConfig() *Config {
	if appConfig == nil {
		// Try to load from default locations
		configPaths := []string{
			"./config.yaml", // Current directory
		}

		for _, path := range configPaths {
			if _, err := os.Stat(path); err == nil {
				slog.Info("Loading configuration", "path", path)
				config, err := LoadConfig(path)
				if err == nil {
					appConfig = config
					break
				} else {
					slog.Warn("Could not load config", "path", path, "error", err)
				}
			}
		}

		// If no config file was found, use defaults
		if appConfig == nil {
			slog.Info("Using default configuration")
			appConfig = &Config{
				WebService: WebServiceConfig{
					Port: DefaultPort,
				},
				MediaPaths: make(map[string]MediaPathConfig),
			}
		}
	}

	return appConfig
}

// GetPort returns the configured web service port
func GetPort() int {
	return GetConfig().WebService.Port
}

// GetMediaPath returns the file system path for a named media path
func GetMediaPath(name string) (string, error) {
	mediaPath, exists := GetConfig().MediaPaths[name]
	if !exists {
		return "", fmt.Errorf("media path '%s' not found", name)
	}
	return mediaPath.Path, nil
}

// GetAllMediaPaths returns all configured media paths
func GetAllMediaPaths() map[string]MediaPathConfig {
	return GetConfig().MediaPaths
}

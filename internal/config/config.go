// Package config manages watch's global configuration, including registered projects.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDirName  = "watch"
	configFileName = "config.json"
)

// Project represents a registered project that watch monitors for orca artifacts.
type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Config is the top-level watch configuration.
type Config struct {
	Projects []Project `json:"projects"`
}

// DefaultPath returns the default config file path (~/.config/watch/config.json).
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(configDir, configDirName, configFileName), nil
}

// Load reads the config from the given path. Returns an empty config if the file doesn't exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to the given path, creating parent directories as needed.
func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// AddProject adds a project to the config. Returns an error if the name is already registered.
func (c *Config) AddProject(name, path string) error {
	for _, p := range c.Projects {
		if p.Name == name {
			return fmt.Errorf("project %q is already registered", name)
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	c.Projects = append(c.Projects, Project{Name: name, Path: absPath})
	return nil
}

// RemoveProject removes a project by name. Returns an error if not found.
func (c *Config) RemoveProject(name string) error {
	for i, p := range c.Projects {
		if p.Name == name {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %q not found", name)
}

// FindProject returns the project with the given name, or nil if not found.
func (c *Config) FindProject(name string) *Project {
	for i := range c.Projects {
		if c.Projects[i].Name == name {
			return &c.Projects[i]
		}
	}
	return nil
}

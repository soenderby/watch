// Package identity manages agent identity definitions and the registry
// that stores them. This package is designed for future extraction into lore.
// Watch consumes identities read-only.
package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentIdentity defines a persistent agent identity.
type AgentIdentity struct {
	Name        string `json:"name"`
	Project     string `json:"project,omitempty"`
	PrimingRef  string `json:"priming_ref,omitempty"`
	Description string `json:"description,omitempty"`
}

// Registry holds all known agent identities.
type Registry struct {
	agents []AgentIdentity
}

// agentsFile is the JSON file format for both global and project-local registries.
type agentsFile struct {
	Agents []AgentIdentity `json:"agents"`
}

// All returns all agent identities.
func (r *Registry) All() []AgentIdentity {
	result := make([]AgentIdentity, len(r.agents))
	copy(result, r.agents)
	return result
}

// ByName returns the agent identity with the given name, or nil.
func (r *Registry) ByName(name string) *AgentIdentity {
	for i := range r.agents {
		if r.agents[i].Name == name {
			return &r.agents[i]
		}
	}
	return nil
}

// ForProject returns all agent identities associated with the given project.
func (r *Registry) ForProject(project string) []AgentIdentity {
	var result []AgentIdentity
	for _, a := range r.agents {
		if a.Project == project {
			result = append(result, a)
		}
	}
	return result
}

// Global returns all agent identities with no project association.
func (r *Registry) Global() []AgentIdentity {
	var result []AgentIdentity
	for _, a := range r.agents {
		if a.Project == "" {
			result = append(result, a)
		}
	}
	return result
}

// Len returns the number of registered identities.
func (r *Registry) Len() int {
	return len(r.agents)
}

// LoadFile reads agent identities from a JSON file.
// Returns an empty list if the file does not exist.
func LoadFile(path string) ([]AgentIdentity, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read agents file: %w", err)
	}

	var f agentsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse agents file %s: %w", path, err)
	}
	return f.Agents, nil
}

// DefaultGlobalPath returns the default global registry path (~/.config/watch/agents.json).
func DefaultGlobalPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(configDir, "watch", "agents.json"), nil
}

// ProjectLocalPath returns the path to a project's local agents file.
func ProjectLocalPath(projectPath string) string {
	return filepath.Join(projectPath, "agents.json")
}

// BuildRegistry assembles a registry from a global agents file and
// project-local agents files. Global definitions take precedence when
// names collide with project-local definitions.
//
// Project-local agents that do not specify a project field are automatically
// assigned the project name they were discovered in.
func BuildRegistry(globalPath string, projects []ProjectSource) (*Registry, error) {
	globalAgents, err := LoadFile(globalPath)
	if err != nil {
		return nil, fmt.Errorf("load global agents: %w", err)
	}

	// Index global agents by name for precedence checking.
	globalNames := make(map[string]bool, len(globalAgents))
	for _, a := range globalAgents {
		globalNames[a.Name] = true
	}

	// Collect all agents, starting with global.
	var all []AgentIdentity
	all = append(all, globalAgents...)

	// Merge project-local agents.
	for _, proj := range projects {
		localPath := ProjectLocalPath(proj.Path)
		localAgents, err := LoadFile(localPath)
		if err != nil {
			// Project-local file errors are non-fatal. Skip this project.
			continue
		}
		for _, a := range localAgents {
			// Global takes precedence.
			if globalNames[a.Name] {
				continue
			}
			// Set project if not specified.
			if a.Project == "" {
				a.Project = proj.Name
			}
			all = append(all, a)
		}
	}

	// Validate uniqueness (global already checked; check for local-local collisions).
	seen := make(map[string]bool, len(all))
	var deduped []AgentIdentity
	for _, a := range all {
		if seen[a.Name] {
			continue // first occurrence wins
		}
		seen[a.Name] = true
		deduped = append(deduped, a)
	}

	return &Registry{agents: deduped}, nil
}

// ProjectSource identifies a project for registry building.
type ProjectSource struct {
	Name string
	Path string
}

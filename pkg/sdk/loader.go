package sdk

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadFromFile reads and parses a Policy from an explicit file path.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy %q: %w", path, err)
	}
	var p Policy
	if err = yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse policy %q: %w", path, err)
	}
	return &p, nil
}

// Discover searches for guardit.yaml or guardit.yml in dir.
// Returns an error if neither file exists.
func Discover(dir string) (*Policy, error) {
	for _, name := range []string{"guardit.yaml", "guardit.yml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return LoadFromFile(path)
		}
	}
	return nil, fmt.Errorf("no guardit.yaml found in %s", dir)
}

// Default returns a sensible baseline policy for a Pi homelab cluster.
// Used as a fallback when no guardit.yaml is found anywhere.
func Default() *Policy {
	return &Policy{
		APIVersion: "guardit.gappy.hu/v1",
		Kind:       "Policy",
		Metadata:   Metadata{Name: "default"},
		Spec: Spec{
			ReplicaLimit:          5,
			AllowedRegistries:     nil,
			RequiredLabels:        []string{"app"},
			RequireResourceLimits: false,
		},
	}
}

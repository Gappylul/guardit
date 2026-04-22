package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Check evaluates a DeploymentRequest against the active policy.
//
// Policy discovery order:
//  1. $GUARDIT_POLICY env var (explicit path)
//  2. guardit.yaml / guardit.yml in the current working directory
//  3. guardit.yaml / guardit.yml in $HOME
//  4. built-in Default (permissive baseline - replicaLimit:5, requiredLabels:[app])
//
// Usage in deployit:
//
//	result, err := sdk.Check(sdk.DeploymentRequest{
//	    Name:     name,
//	    Image:    fmt.Sprintf("%s/%s:preflight", registry, name),
//	    Replicas: replicas,
//	    Labels:   map[string]string{"app": name},
//	    Containers: []sdk.ContainerRequest{
//	        {Name: name, ResourceLimitsDeclared: true},
//	    },
//	})
//	if err != nil { return err }
//	if !result.Allowed {
//	    return fmt.Errorf(sdk.FormatViolations(result))
//	}
func Check(req DeploymentRequest) (Result, error) {
	p, err := resolvePolicy()
	if err != nil {
		return Result{}, fmt.Errorf("guardit: %w", err)
	}
	return Evaluate(p, req), nil
}

// PolicySource returns a human-readable description of where the active policy
// will be loaded from - useful for the "-> guardit: using policy from X" log line.
func PolicySource() string {
	if path := os.Getenv("GUARDIT_POLICY"); path != "" {
		return path
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{"guardit.yaml", "guardit.yml"} {
			if _, err := os.Stat(filepath.Join(home, name)); err == nil {
				return filepath.Join(home, name)
			}
		}
	}
	return "built-in defaults"
}

// FormatViolations returns a formatted string of all violations for terminal output.
func FormatViolations(result Result) string {
	if result.Allowed {
		return "✓ guardit: all policy checks passed"
	}
	var sb strings.Builder
	sb.WriteString("✗ guardit: deployment rejected by policy\n")
	for _, v := range result.Violations {
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", v.Code, v.Message))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// resolvePolicy loads the active policy using the discovery order above.
func resolvePolicy() (*Policy, error) {
	if path := os.Getenv("GUARDIT_POLICY"); path != "" {
		return LoadFromFile(path)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if p, err := Discover(home); err == nil {
			return p, nil
		}
	}
	return Default(), nil
}

package sdk

import (
	"fmt"
	"strings"
)

// Policy is the top-level guardit object.
// Instantiate directly in Go or load from guardit.yaml - same struct either way.
//
//	p := &sdk.Policy{
//	    APIVersion: "guardit.gappy.hu/v1",
//	    Kind:       "Policy",
//	    Metadata:   sdk.Metadata{Name: "homelab"},
//	    Spec: sdk.Spec{
//	        ReplicaLimit:      5,
//	        AllowedRegistries: []string{"ghcr.io/gappylul"},
//	        RequiredLabels:    []string{"app"},
//	    },
//	}
type Policy struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

// Metadata mirrors k8s ObjectMeta (name only).
type Metadata struct {
	Name string `json:"name"`
}

// Spec holds all configurable policy rules.
// Zero values are intentionally permissive - guardit is opt-in per rule.
type Spec struct {
	// ReplicaLimit is the max allowed replica count. 0 means unlimited.
	ReplicaLimit int32 `json:"replicaLimit" yaml:"replicaLimit"`

	// AllowedRegistries is a list of registry prefixes. An image passes if it
	// starts with any entry. Empty means all registries are allowed.
	AllowedRegistries []string `json:"allowedRegistries" yaml:"allowedRegistries"`

	// RequiredLabels lists label keys that must be present on every pod template.
	RequiredLabels []string `json:"requiredLabels" yaml:"requiredLabels"`

	// RequireResourceLimits enforces that every container declares cpu+memory limits.
	RequireResourceLimits bool `json:"requireResourceLimits" yaml:"requireResourceLimits"`
}

// DeploymentRequest is the normalized input the engine evaluates.
// Both deployit (via Check) and the webhook (via AdmissionReview) build one of these.
type DeploymentRequest struct {
	// Name of the deployment / app.
	Name string

	// Image is the full reference of the primary container,
	// e.g. "ghcr.io/gappylul/myapp:abc1234". Used for registry checks.
	Image string

	// Replicas requested.
	Replicas int32

	// Labels on the pod template (spec.template.metadata.labels).
	Labels map[string]string

	// Containers holds per-container data needed by the engine.
	Containers []ContainerRequest
}

// ContainerRequest is a simplified container spec for policy evaluation.
type ContainerRequest struct {
	Name string

	// ResourceLimitsDeclared is true when both cpu and memory limits are non-zero.
	ResourceLimitsDeclared bool
}

// Result is what the engine returns after evaluating all rules.
type Result struct {
	Allowed    bool
	Violations []Violation
}

// Summary returns a single human-readable string.
func (r Result) Summary() string {
	if r.Allowed {
		return "✓ all policy checks passed"
	}
	msgs := make([]string, len(r.Violations))
	for i, v := range r.Violations {
		msgs[i] = fmt.Sprintf("[%s] %s", v.Code, v.Message)
	}
	return "policy check failed:\n  " + strings.Join(msgs, "\n  ")
}

// Violation is a single rule failure.
type Violation struct {
	Code    string // stable, machine-readable: "ReplicaLimit"
	Message string // human-readable: "replica count 10 exceeds maximum of 5"
}

// Allow returns a passing Result.
func Allow() Result { return Result{Allowed: true} }

// Deny returns a failing Result with the given violations.
func Deny(violations ...Violation) Result {
	return Result{Allowed: false, Violations: violations}
}

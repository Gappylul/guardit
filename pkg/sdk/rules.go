package sdk

import (
	"fmt"
	"strings"
)

// defaultRules are the built-in rules used by Check() and NewEngine().
// To extend the ruleset, pass additional RuleFuncs to NewEngine().
var defaultRules = []RuleFunc{
	ruleReplicaLimit,
	ruleAllowedRegistries,
	ruleRequiredLabels,
	ruleResourceLimits,
}

// ruleReplicaLimit rejects deployments that exceed the configured replica cap.
func ruleReplicaLimit(spec Spec, req DeploymentRequest) *Violation {
	if spec.ReplicaLimit == 0 {
		return nil
	}
	if req.Replicas > spec.ReplicaLimit {
		return &Violation{
			Code: "ReplicaLimit",
			Message: fmt.Sprintf(
				"replica count %d is too high for this cluster (max %d)",
				req.Replicas, spec.ReplicaLimit,
			),
		}
	}
	return nil
}

// ruleAllowedRegistries rejects images not prefixed by an allowed registry.
func ruleAllowedRegistries(spec Spec, req DeploymentRequest) *Violation {
	if len(spec.AllowedRegistries) == 0 {
		return nil
	}
	for _, prefix := range spec.AllowedRegistries {
		if strings.HasPrefix(req.Image, prefix) {
			return nil
		}
	}
	return &Violation{
		Code: "RegistryNotAllowed",
		Message: fmt.Sprintf(
			"image %q is not from an allowed registry (allowed: %s)",
			req.Image, strings.Join(spec.AllowedRegistries, ", "),
		),
	}
}

// ruleRequiredLabels rejects pod templates missing mandatory label keys.
func ruleRequiredLabels(spec Spec, req DeploymentRequest) *Violation {
	if len(spec.RequiredLabels) == 0 {
		return nil
	}
	var missing []string
	for _, key := range spec.RequiredLabels {
		if _, ok := req.Labels[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &Violation{
		Code: "MissingLabels",
		Message: fmt.Sprintf(
			"%q is missing required pod template labels: %s",
			req.Name, strings.Join(missing, ", "),
		),
	}
}

// ruleResourceLimits rejects containers that declare no cpu/memory limits.
func ruleResourceLimits(spec Spec, req DeploymentRequest) *Violation {
	if !spec.RequireResourceLimits {
		return nil
	}
	var violators []string
	for _, c := range req.Containers {
		if !c.ResourceLimitsDeclared {
			violators = append(violators, c.Name)
		}
	}
	if len(violators) == 0 {
		return nil
	}
	return &Violation{
		Code:    "MissingResourceLimits",
		Message: fmt.Sprintf("containers missing cpu/memory limits: %s", strings.Join(violators, ", ")),
	}
}

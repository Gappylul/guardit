package sdk

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// FromDeployment converts a Kubernetes Deployment into a DeploymentRequest.
// Used by the webhook and guard-cli so the mapping is never duplicated.
func FromDeployment(d *appsv1.Deployment) DeploymentRequest {
	req := DeploymentRequest{
		Name:     d.Name,
		Labels:   d.Spec.Template.Labels,
		Replicas: 1,
	}
	if d.Spec.Replicas != nil {
		req.Replicas = *d.Spec.Replicas
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		if req.Image == "" {
			req.Image = c.Image
		}
		req.Containers = append(req.Containers, ContainerRequest{
			Name:                   c.Name,
			ResourceLimitsDeclared: hasLimits(c),
		})
	}
	return req
}

// hasLimits returns true when the container declares non-zero cpu AND memory limits.
func hasLimits(c corev1.Container) bool {
	lim := c.Resources.Limits
	if lim == nil {
		return false
	}
	zero := resource.MustParse("0")
	cpu := lim[corev1.ResourceCPU]
	mem := lim[corev1.ResourceMemory]
	return !cpu.Equal(zero) && !mem.Equal(zero)
}

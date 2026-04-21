package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"

	"github.com/gappylul/guardit/pkg/sdk"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "guard-cli",
		Short: "Guardit local policy linter",
	}
	root.AddCommand(checkCmd())
	return root
}

func checkCmd() *cobra.Command {
	var policyPath string

	cmd := &cobra.Command{
		Use:   "check <manifest.yaml>",
		Short: "Check a Kubernetes Deployment manifest against your guardit policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := resolvePolicy(policyPath)
			if err != nil {
				return err
			}
			fmt.Printf("-> policy: %s\n", p.Metadata.Name)

			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}

			var deployment appsv1.Deployment
			if err := yaml.Unmarshal(data, &deployment); err != nil {
				return fmt.Errorf("parse manifest: %w", err)
			}

			req := sdk.FromDeployment(&deployment)
			result := sdk.Evaluate(p, req)

			if result.Allowed {
				fmt.Printf("✓ %s passed all policy checks\n", req.Name)
				return nil
			}

			fmt.Printf("✗ %s failed policy checks:\n", req.Name)
			for _, v := range result.Violations {
				fmt.Printf("  [%s] %s\n", v.Code, v.Message)
			}
			os.Exit(1)
			return nil
		},
	}

	cmd.Flags().StringVar(&policyPath, "policy", "", "path to guardit.yaml (default: auto-discover)")
	return cmd
}

func resolvePolicy(explicit string) (*sdk.Policy, error) {
	if explicit != "" {
		return sdk.LoadFromFile(explicit)
	}
	cwd, _ := os.Getwd()
	if p, err := sdk.Discover(cwd); err == nil {
		return p, nil
	}
	fmt.Println("-> no guardit.yaml found, using built-in defaults")
	return sdk.Default(), nil
}

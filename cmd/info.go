package cmd

import (
	"context"
	"fmt"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	"github.com/spf13/cobra"
)

const defaultRegistry = "https://bcr.bazel.build"

var infoCmd = &cobra.Command{
	Use:   "info <module>[@<version>]",
	Short: "Show information about a module",
	Long: `Show detailed information about a module from the Bazel Central Registry.

Examples:
  bz info rules_go
  bz info rules_go@0.50.1`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	name, version := parseModuleArg(args[0])

	if version == "" {
		// TODO: fetch latest version from registry
		return fmt.Errorf("please specify a version: %s@<version>", name)
	}

	client := gobzlmod.NewRegistryClient(defaultRegistry)
	info, err := client.GetModuleFile(context.Background(), name, version)
	if err != nil {
		return fmt.Errorf("failed to fetch module info: %w", err)
	}

	fmt.Printf("Module: %s\n", info.Name)
	fmt.Printf("Version: %s\n", version)
	if info.CompatibilityLevel > 0 {
		fmt.Printf("Compatibility Level: %d\n", info.CompatibilityLevel)
	}

	if len(info.Dependencies) > 0 {
		fmt.Printf("\nDependencies (%d):\n", len(info.Dependencies))
		for _, dep := range info.Dependencies {
			dev := ""
			if dep.DevDependency {
				dev = " (dev)"
			}
			fmt.Printf("  - %s@%s%s\n", dep.Name, dep.Version, dev)
		}
	}

	return nil
}

// parseModuleArg splits "module@version" into name and version
func parseModuleArg(arg string) (name, version string) {
	for i, c := range arg {
		if c == '@' {
			return arg[:i], arg[i+1:]
		}
	}
	return arg, ""
}

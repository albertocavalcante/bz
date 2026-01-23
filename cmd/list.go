package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies in MODULE.bazel",
	Long:  `List all bazel_dep entries in your MODULE.bazel file.`,
	RunE:  runList,
}

var (
	listOutdated bool
)

func init() {
	listCmd.Flags().BoolVar(&listOutdated, "outdated", false, "Show only outdated dependencies")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	modulePath, err := findModuleFile()
	if err != nil {
		return err
	}

	info, err := gobzlmod.ParseModuleFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to parse MODULE.bazel: %w", err)
	}

	if info.Name != "" {
		fmt.Printf("Module: %s", info.Name)
		if info.Version != "" {
			fmt.Printf(" (%s)", info.Version)
		}
		fmt.Println()
		fmt.Println()
	}

	if len(info.Dependencies) == 0 {
		fmt.Println("No dependencies found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tDEV")

	for _, dep := range info.Dependencies {
		dev := ""
		if dep.DevDependency {
			dev = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", dep.Name, dep.Version, dev)
	}

	return w.Flush()
}

func findModuleFile() (string, error) {
	// Check current directory first
	if _, err := os.Stat("MODULE.bazel"); err == nil {
		return "MODULE.bazel", nil
	}

	// TODO: walk up to find workspace root
	return "", fmt.Errorf("MODULE.bazel not found in current directory")
}

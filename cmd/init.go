package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
)

var (
	initName    string
	initVersion string
	initForce   bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Bazel module project",
	Long: `Initialize a new Bazel module by creating a MODULE.bazel file.

If no --name is provided, the current directory name is used (sanitized).

Examples:
  bz init                          # use directory name as module name
  bz init --name=my_module         # set module name
  bz init --name=foo --version=1.0.0
  bz init --force                  # overwrite existing MODULE.bazel`,
	RunE:          runInit,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureInitCmd() {
	initCmd.Flags().StringVar(&initName, "name", "", "Module name (defaults to directory name)")
	initCmd.Flags().StringVar(&initVersion, "version", "0.0.0", "Initial module version")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing MODULE.bazel")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := cli.CheckCommandAllowed("init"); err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	_, name, err := module.InitInDir(wd, initName, initVersion, initForce)
	if err != nil {
		return err
	}

	version := initVersion
	if version == "" {
		version = module.DefaultModuleVersion
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created MODULE.bazel for module %q (version %s)\n", name, version)
	return nil
}

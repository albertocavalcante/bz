package mod

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/modsync"
)

var syncCmd = &cobra.Command{
	Use:   "sync [workflow-name]",
	Short: "Sync modules from source to destination registry",
	Long: `Sync modules from a source registry to a destination based on a workflow
defined in bz.star configuration.

Examples:
  # Run the "mirror-essential" workflow
  bz mod sync mirror-essential

  # Dry run to see what would be synced
  bz mod sync mirror-essential --dry-run

  # List available workflows
  bz mod sync --list

  # Override modules to sync
  bz mod sync mirror-essential --modules rules_go,rules_python
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

var (
	syncDryRun  bool
	syncVerbose bool
	syncList    bool
	syncModules []string
	syncConfig  string
)

var _ = onLoad(func() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would be synced without making changes")
	syncCmd.Flags().BoolVarP(&syncVerbose, "verbose", "v", false, "Print detailed progress")
	syncCmd.Flags().BoolVarP(&syncList, "list", "l", false, "List available workflows")
	syncCmd.Flags().StringSliceVar(&syncModules, "modules", nil, "Override modules to sync (comma-separated)")
	syncCmd.Flags().StringVarP(&syncConfig, "config", "c", "", "Path to config file (default: bz.star)")

	Cmd.AddCommand(syncCmd)
})

func runSync(cmd *cobra.Command, args []string) error {
	// Load config
	loader := config.NewLoader()
	var cfg *config.Config
	var err error

	if syncConfig != "" {
		cfg, err = loader.Load(syncConfig)
	} else {
		cfg, err = loader.LoadDefault()
	}

	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create sync service
	svc := modsync.NewService(cfg)

	// If --list, print workflows and exit
	if syncList {
		workflows := svc.ListWorkflows()
		if len(workflows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No workflows defined in configuration.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Available workflows:")
		for _, name := range workflows {
			w := svc.GetWorkflow(name)
			if w != nil && w.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s - %s\n", name, w.Description)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
			}
		}
		return nil
	}

	// Workflow name is required if not listing
	if len(args) == 0 {
		return fmt.Errorf("workflow name is required (use --list to see available workflows)")
	}

	workflowName := args[0]

	// Build options
	opts := modsync.Options{
		DryRun:  syncDryRun,
		Verbose: syncVerbose,
		Modules: syncModules,
	}

	// Run the workflow
	result, err := svc.Run(cmd.Context(), workflowName, opts)
	if err != nil {
		return err
	}

	// Print results
	if syncVerbose || syncDryRun {
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Sync complete:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Modules processed: %d\n", result.ModulesProcessed)
	fmt.Fprintf(cmd.OutOrStdout(), "  Modules written:   %d\n", result.ModulesWritten)
	fmt.Fprintf(cmd.OutOrStdout(), "  Modules skipped:   %d\n", result.ModulesSkipped)

	if len(result.Errors) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Errors:            %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "    - %v\n", e)
		}
	}

	return nil
}

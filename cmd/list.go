package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/albertocavalcante/bz/internal/tui"
	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies in MODULE.bazel",
	Long:  `List all bazel_dep entries in your MODULE.bazel file.`,
	RunE:  runList,
}

var (
	listOutdated    bool
	listInteractive bool
	listJSON        bool
)

func init() {
	listCmd.Flags().BoolVar(&listOutdated, "outdated", false, "Show only outdated dependencies")
	listCmd.Flags().BoolVarP(&listInteractive, "interactive", "i", false, "Force interactive TUI mode")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Determine if we should use TUI
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	// Force interactive mode or auto-detect
	if listInteractive || (isTTY && !listJSON) {
		return tui.Run()
	}

	// Headless mode
	return runListHeadless()
}

func runListHeadless() error {
	modulePath, err := findModuleFile()
	if err != nil {
		return err
	}

	info, err := gobzlmod.ParseModuleFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to parse MODULE.bazel: %w", err)
	}

	if listJSON {
		return printListJSON(info)
	}

	return printListTable(info)
}

func printListTable(info *gobzlmod.ModuleInfo) error {
	styles := tui.DefaultStyles()

	if info.Name != "" {
		fmt.Printf("%s", styles.Title.Render(info.Name))
		if info.Version != "" {
			fmt.Printf(" %s", styles.Muted.Render("("+info.Version+")"))
		}
		fmt.Println()
		fmt.Println()
	}

	if len(info.Dependencies) == 0 {
		fmt.Println(styles.Muted.Render("No dependencies found."))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, styles.Muted.Render("NAME\tVERSION\tDEV"))

	for _, dep := range info.Dependencies {
		dev := ""
		if dep.DevDependency {
			dev = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", dep.Name, dep.Version, dev)
	}

	return w.Flush()
}

func printListJSON(info *gobzlmod.ModuleInfo) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(info)
}

func findModuleFile() (string, error) {
	if _, err := os.Stat("MODULE.bazel"); err == nil {
		return "MODULE.bazel", nil
	}
	return "", fmt.Errorf("MODULE.bazel not found in current directory")
}

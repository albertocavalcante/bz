package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/cmd/cache"
	"github.com/albertocavalcante/bz/cmd/mod"
	"github.com/albertocavalcante/bz/cmd/registry"
	"github.com/albertocavalcante/bz/internal/bzconfig"
	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	registrylib "github.com/albertocavalcante/bz/internal/registry"
)

var showVersion bool
var configureRootOnce sync.Once

var rootCmd = &cobra.Command{
	Use:   "bz",
	Short: "CLI for Bzlmod - Bazel's module system",
	Long: `bz is a CLI tool for managing Bazel modules (Bzlmod).

It helps you manage MODULE.bazel dependencies, query the Bazel Central Registry,
and streamline your Bazel module workflow.

Air-gapped/Offline Usage:
  Use --offline to disable all network access and rely solely on cached data.
  Use --prefer-offline to prefer cached data but fall back to network if needed.
  Use --registry to override the default registry URL for custom registries.

Configuration (lowest to highest precedence):
  built-in defaults < /etc/bz/config.toml < ~/.config/bz/config.toml
  < .bz.toml (project) < environment variables < command-line flags

Environment Variables:
  BZ_OFFLINE=1               Same as --offline flag
  BZ_PREFER_OFFLINE=1        Same as --prefer-offline flag
  BZ_REGISTRY=<url>          Same as --registry flag
  BZ_CACHE_DIR=<dir>         Same as [cache] dir
  BZ_DISABLE_COMMANDS=<list> Same as [commands] disabled (comma-separated)`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate mutually exclusive flags
		if err := cli.ValidateOfflineFlags(); err != nil {
			return err
		}
		// Configure lipgloss color profile based on --no-color flag
		cli.ConfigureLipgloss()
		return applyConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			versionCmd.Run(cmd, args)
			return
		}
		_ = cmd.Help()
	},
}

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if code := run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

// run executes the CLI with the given arguments and returns the exit status.
// Errors are written to stderr exactly once; stdout is left untouched so
// machine-readable output stays parseable.
func run(args []string, stdout, stderr io.Writer) int {
	Configure()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	err := rootCmd.Execute()
	if err == nil {
		return 0
	}
	if !cmdutil.IsAlreadyReported(err) {
		printError(stderr, err)
	}
	return 1
}

// printError writes err to w as an "Error: ..." line, followed by any
// suggestions carried by the error type.
func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "Error: %s\n", formatError(err))

	var netErr *registrylib.NetworkError
	if errors.As(err, &netErr) {
		fmt.Fprintf(w, "\n%s", netErr.UserFriendlyError())
	}
}

// formatError renders err for one "Error:" line. Several configuration
// problems (bzconfig.Problems), alone or wrapped with %w, become a count and
// an indented list so no item reads as an unrelated message.
func formatError(err error) string {
	msg := strings.TrimRight(err.Error(), "\n")
	var problems bzconfig.Problems
	if !errors.As(err, &problems) || len(problems) < 2 {
		return msg
	}
	prefix, ok := strings.CutSuffix(err.Error(), problems.Error())
	if !ok {
		return msg // a wrapper that adds text after the list
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s%d problems:", prefix, len(problems))
	for _, p := range problems {
		text := strings.TrimRight(p.Error(), "\n")
		b.WriteString("\n  - " + strings.ReplaceAll(text, "\n", "\n    "))
	}
	return b.String()
}

// applyConfig loads the effective configuration once for this invocation,
// prints its warnings and enforces [commands] disabled for cmd. A
// configuration that fails to load stops the command (fail closed), so a
// bad value cannot silently drop settings such as [commands] disabled.
// Help, version and completion keep working with a broken config file so
// users can still diagnose it; for them the error is a warning.
func applyConfig(cmd *cobra.Command) error {
	errOut := cmd.ErrOrStderr()
	path := commandPathWithoutRoot(cmd)
	protected := bzconfig.IsProtectedCommand(path)

	eff, warnings, err := cli.LoadEffective()
	printWarnings(errOut, warnings)
	if err != nil {
		switch {
		case cmd == doctorCmd:
			return nil // doctor reports it as its "config" check
		case protected || cmd.HasSubCommands():
			// Help and bare group commands only print help, so they stay usable.
			printWarnings(errOut, []string{err.Error()})
			return nil
		}
		return err
	}

	warnings, err = cli.CheckCommandEnabled(eff, path, commandPaths(cmd.Root()))
	printWarnings(errOut, warnings)
	if err != nil && protected {
		printWarnings(errOut, []string{err.Error()})
		return nil
	}
	return err
}

func printWarnings(w io.Writer, warnings []string) {
	for _, msg := range warnings {
		fmt.Fprintf(w, "Warning: %s\n", msg)
	}
}

// commandPathWithoutRoot returns cmd's path below the root, e.g.
// "registry ping" for "bz registry ping", and "" for the root itself.
func commandPathWithoutRoot(cmd *cobra.Command) string {
	if !cmd.HasParent() {
		return ""
	}
	return strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
}

// commandPaths lists every command path below root.
func commandPaths(root *cobra.Command) []string {
	var paths []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			paths = append(paths, commandPathWithoutRoot(child))
			walk(child)
		}
	}
	walk(root)
	return paths
}

// Configure wires all root and subcommands once.
func Configure() {
	configureRootOnce.Do(func() {
		cache.Configure()
		mod.Configure()
		registry.Configure()

		configureRootCmd()
		configureAuditCmd()
		configureCompletionCmd()
		configureCompletionOptions()
		configureDoctorCmd()
		configureInitCmd()
		configureSBOMCmd()
		configureTUICmd()
		configureVersionCmd()
	})
}

func configureRootCmd() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Errors are printed once by run(); cobra must neither print them itself
	// nor append usage text to runtime failures.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		hint := fmt.Sprintf("Run '%s --help' for usage.", c.CommandPath())
		return fmt.Errorf("%w\n%s", err, hint)
	})

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&cli.Global.Quiet, "quiet", "q", false, "Reduce output, only show errors and essential info")
	rootCmd.PersistentFlags().BoolVar(&cli.Global.NoColor, "no-color", false, "Disable colored output")

	// Network/offline flags for air-gapped environments
	rootCmd.PersistentFlags().BoolVar(&cli.Global.Offline, "offline", false, "Disable all network access, use cache only (for air-gapped environments)")
	rootCmd.PersistentFlags().BoolVar(&cli.Global.PreferOffline, "prefer-offline", false, "Prefer cached data, fallback to network if needed")
	rootCmd.PersistentFlags().StringVar(&cli.Global.Registry, "registry", "", "Override registry URL")

	// Version flag as shorthand alias for 'bz version'
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version information")

	rootCmd.AddCommand(cache.Cmd)
	rootCmd.AddCommand(mod.Cmd)
	rootCmd.AddCommand(registry.Cmd)
}

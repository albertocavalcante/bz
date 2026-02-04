package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
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
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "Module name (defaults to directory name)")
	initCmd.Flags().StringVar(&initVersion, "version", "0.0.0", "Initial module version")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing MODULE.bazel")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	modulePath := filepath.Join(wd, "MODULE.bazel")

	// Check if MODULE.bazel already exists
	if _, err := os.Stat(modulePath); err == nil {
		if !initForce {
			return fmt.Errorf("MODULE.bazel already exists (use --force to overwrite)")
		}
	}

	// Determine module name
	name := initName
	if name == "" {
		name = sanitizeModuleName(filepath.Base(wd))
	}

	// Validate module name
	if err := validateModuleName(name); err != nil {
		return err
	}

	// Generate MODULE.bazel content
	content := formatModuleFile(name, initVersion)

	// Write the file
	if err := os.WriteFile(modulePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created MODULE.bazel for module %q (version %s)\n", name, initVersion)
	return nil
}

// sanitizeModuleName converts a directory name to a valid Bazel module name.
// Bazel module names must:
// - Start with a lowercase letter
// - Contain only lowercase letters, digits, underscores, dots, and dashes
// - Not start with a digit
func sanitizeModuleName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with underscores
	var result strings.Builder
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			result.WriteRune(r)
		case r >= '0' && r <= '9':
			// Digits are allowed, but not at the start
			if i == 0 {
				result.WriteRune('_')
			}
			result.WriteRune(r)
		case r == '_' || r == '.' || r == '-':
			result.WriteRune(r)
		default:
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// If the name starts with something other than a letter, prefix with underscore
	if len(sanitized) > 0 && (sanitized[0] < 'a' || sanitized[0] > 'z') {
		sanitized = "module_" + sanitized
	}

	// Handle empty name
	if sanitized == "" {
		sanitized = "my_module"
	}

	return sanitized
}

// validateModuleName checks if a module name is valid.
func validateModuleName(name string) error {
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	// Must start with a lowercase letter
	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("module name must start with a lowercase letter: %q", name)
	}

	// Must contain only valid characters
	validPattern := regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("module name contains invalid characters: %q (allowed: lowercase letters, digits, underscore, dot, dash)", name)
	}

	return nil
}

// formatModuleFile generates the content of a MODULE.bazel file.
func formatModuleFile(name, version string) string {
	return fmt.Sprintf(`module(
    name = %q,
    version = %q,
)
`, name, version)
}

package module

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// DefaultModuleVersion is used when creating a new MODULE.bazel without an explicit version.
	DefaultModuleVersion = "0.0.0"
)

var validModuleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

// InitInDir creates a MODULE.bazel in dir.
// If name is empty, the directory name is sanitized and used.
func InitInDir(dir, name, version string, force bool) (string, string, error) {
	modulePath := filepath.Join(dir, moduleFileName)

	if _, err := os.Stat(modulePath); err == nil && !force {
		return "", "", fmt.Errorf("MODULE.bazel already exists (use --force to overwrite)")
	}

	if name == "" {
		name = SanitizeModuleName(filepath.Base(dir))
	}

	if err := ValidateModuleName(name); err != nil {
		return "", "", err
	}

	if version == "" {
		version = DefaultModuleVersion
	}

	content := FormatModuleFile(name, version)
	if err := os.WriteFile(modulePath, []byte(content), 0o644); err != nil {
		return "", "", fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	return modulePath, name, nil
}

// SanitizeModuleName converts a directory name to a valid Bazel module name.
func SanitizeModuleName(name string) string {
	name = strings.ToLower(name)

	var result strings.Builder
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			result.WriteRune(r)
		case r >= '0' && r <= '9':
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
	if len(sanitized) > 0 && (sanitized[0] < 'a' || sanitized[0] > 'z') {
		sanitized = "module_" + sanitized
	}

	if sanitized == "" {
		sanitized = "my_module"
	}

	return sanitized
}

// ValidateModuleName checks if a module name is valid.
func ValidateModuleName(name string) error {
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("module name must start with a lowercase letter: %q", name)
	}

	if !validModuleNamePattern.MatchString(name) {
		return fmt.Errorf("module name contains invalid characters: %q (allowed: lowercase letters, digits, underscore, dot, dash)", name)
	}

	return nil
}

// FormatModuleFile generates MODULE.bazel content.
func FormatModuleFile(name, version string) string {
	return fmt.Sprintf(`module(
    name = %q,
    version = %q,
)
`, name, version)
}

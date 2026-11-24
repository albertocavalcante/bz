package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/albertocavalcante/bz/internal/coord"
)

const (
	// DefaultModuleVersion is used when creating a new MODULE.bazel without an explicit version.
	DefaultModuleVersion = "0.0.0"
)

// InitInDir creates a MODULE.bazel in dir.
// If name is empty, the directory name is sanitized and used.
func InitInDir(dir, name, version string, force bool) (string, string, error) {
	modulePath := filepath.Join(dir, moduleFileName)

	if _, err := os.Stat(modulePath); err == nil && !force {
		return "", "", fmt.Errorf("MODULE.bazel already exists (use --force to overwrite)")
	}

	fromDir := name == ""
	if fromDir {
		name = SanitizeModuleName(filepath.Base(dir))
	}
	if version == "" {
		version = DefaultModuleVersion
	}

	// Validate before writing anything, so a refusal leaves the directory untouched.
	if err := coord.ValidateModuleName(name); err != nil {
		if fromDir {
			return "", "", fmt.Errorf("%w (derived from the directory name)\nhelp: pass --name to choose a valid module name", err)
		}
		return "", "", fmt.Errorf("%w\nhelp: pass a --name such as my_module", err)
	}
	if err := coord.ValidateVersion(version); err != nil {
		return "", "", fmt.Errorf("%w\nhelp: pass a --version such as 1.0.0 or 1.0.0-rc1, or omit it to use %s", err, DefaultModuleVersion)
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

// FormatModuleFile generates MODULE.bazel content.
func FormatModuleFile(name, version string) string {
	return fmt.Sprintf(`module(
    name = %q,
    version = %q,
)
`, name, version)
}

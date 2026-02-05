package mod

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/module"
)

// completionTimeout is the maximum time allowed for completion requests.
const completionTimeout = 2 * time.Second

// completeModuleNames returns module name completions from the registry.
// If the toComplete contains "@", it completes versions instead.
func completeModuleNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Check if we're completing a version (after @)
	if idx := strings.LastIndex(toComplete, "@"); idx != -1 {
		return completeVersions(cmd, toComplete[:idx], toComplete[idx+1:])
	}

	// Complete module names from registry
	return completeModulesFromRegistry(toComplete)
}

// completeModulesFromRegistry returns module names matching the prefix.
func completeModulesFromRegistry(prefix string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	// Create network-aware registry (silently fail for completions)
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	modules, err := reg.ListModules(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	prefixLower := strings.ToLower(prefix)
	for _, name := range modules {
		if prefix == "" || strings.HasPrefix(strings.ToLower(name), prefixLower) {
			completions = append(completions, name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeVersions returns version completions for a module.
func completeVersions(cmd *cobra.Command, moduleName, versionPrefix string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	// Create network-aware registry (silently fail for completions)
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	meta, err := reg.GetMetadata(ctx, moduleName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	// Return versions in reverse order (newest first)
	for i := len(meta.Versions) - 1; i >= 0; i-- {
		v := meta.Versions[i]
		if versionPrefix == "" || strings.HasPrefix(v, versionPrefix) {
			completions = append(completions, moduleName+"@"+v)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeInstalledModules returns module names from MODULE.bazel.
// This is used for commands like `rm` that operate on installed modules.
func completeInstalledModules(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Find and load MODULE.bazel
	modulePath, err := module.Find()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	f, err := module.Load(modulePath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Build set of already-specified modules (to exclude them)
	specified := make(map[string]bool)
	for _, arg := range args {
		specified[arg] = true
	}

	var completions []string
	prefixLower := strings.ToLower(toComplete)
	for _, dep := range f.Deps {
		name := dep.Name.String()
		// Skip already-specified modules
		if specified[name] {
			continue
		}
		if toComplete == "" || strings.HasPrefix(strings.ToLower(name), prefixLower) {
			completions = append(completions, name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeModuleNamesForSingleArg is like completeModuleNames but stops after first arg.
// Used for commands that take exactly one module argument.
func completeModuleNamesForSingleArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If we already have an argument, no more completions
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeModuleNames(cmd, args, toComplete)
}

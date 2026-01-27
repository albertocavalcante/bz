// Package modules provides bz-specific Starlark modules for configuration.
// These modules are used in bz.star configuration files.
//
// Available modules:
//
//   - auth: Authentication configuration (basic, bearer_token, header)
//   - registry: Registry definitions (http, file, git, http_put)
//   - sync: Sync workflow definitions for mirroring modules between registries
//   - config: Configuration defaults
//
// # Sync Module
//
// The sync module allows defining workflows for synchronizing modules between registries:
//
//	bcr = registry.http("https://bcr.bazel.build")
//	internal = registry.file("/path/to/internal")
//
//	sync.workflow(
//	    name = "mirror-essential",
//	    description = "Mirror essential modules to internal registry",
//	    origin = bcr,
//	    destination = internal,
//	    modules = glob(["rules_*", "bazel_skylib"]),
//	    versions = sync.latest(count = 3),
//	    transformations = [
//	        sync.rewrite_source_urls(
//	            pattern = "https://github.com/(.+)/archive/(.+)",
//	            replacement = "https://internal/$1/$2",
//	        ),
//	    ],
//	)
//
// Version selectors:
//   - sync.latest(count = N): Last N versions
//   - sync.all(): All versions
//   - sync.since(date = "YYYY-MM-DD"): Versions since a date
//   - sync.range(min = "X.Y.Z", max = "A.B.C"): Version range
//
// Transformations:
//   - sync.rewrite_source_urls(pattern, replacement): Rewrite source URLs
//   - sync.skip_yanked(): Skip yanked versions
//   - sync.include_patches(enabled = True): Include patch files
//
// # Config Module
//
// The config module allows setting default configuration values:
//
//	config.defaults(
//	    registry = bcr,
//	    cache_dir = "~/.cache/bz",
//	    fallback_registries = [bcr_mirror, local_cache],
//	)
package modules

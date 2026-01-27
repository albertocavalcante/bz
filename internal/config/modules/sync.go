package modules

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/albertocavalcante/bz/internal/starlark/value"
)

// Version selector types.
const (
	VersionSelectorLatest = "latest"
	VersionSelectorAll    = "all"
	VersionSelectorSince  = "since"
	VersionSelectorRange  = "range"
)

// Transformation types.
const (
	TransformRewriteURLs    = "rewrite_urls"
	TransformSkipYanked     = "skip_yanked"
	TransformIncludePatches = "include_patches"
)

// Thread-local key for workflow registry.
const workflowRegistryKey = "sync_workflow_registry"

// VersionSelector represents a version selection strategy in Starlark.
type VersionSelector struct {
	value.Base
	value.AttrAccessor

	// Kind is the selector type: "latest", "all", "since", or "range".
	Kind string

	// Count is the number of versions for "latest" selector.
	Count int

	// Since is the date string for "since" selector (format: "2024-01-01").
	Since string

	// MinVer is the minimum version for "range" selector.
	MinVer string

	// MaxVer is the maximum version for "range" selector.
	MaxVer string
}

// Ensure VersionSelector implements the required interfaces.
var (
	_ starlark.Value    = (*VersionSelector)(nil)
	_ starlark.HasAttrs = (*VersionSelector)(nil)
)

// String returns a string representation of the version selector.
func (v *VersionSelector) String() string {
	switch v.Kind {
	case VersionSelectorLatest:
		return fmt.Sprintf("sync.latest(count=%d)", v.Count)
	case VersionSelectorAll:
		return "sync.all()"
	case VersionSelectorSince:
		return fmt.Sprintf("sync.since(date=%q)", v.Since)
	case VersionSelectorRange:
		return fmt.Sprintf("sync.range(min=%q, max=%q)", v.MinVer, v.MaxVer)
	default:
		return "version_selector"
	}
}

// newVersionSelector creates a new VersionSelector with common initialization.
func newVersionSelector(selectorType string) *VersionSelector {
	v := &VersionSelector{
		Base: value.NewBase("version_selector"),
		Kind: selectorType,
	}
	v.AttrAccessor = value.NewAttrAccessor()
	v.Set("type", starlark.String(selectorType))
	return v
}

// TransformValue represents a sync transformation in Starlark.
type TransformValue struct {
	value.Base
	value.AttrAccessor

	// Kind is the transformation type: "rewrite_urls", "skip_yanked", or "include_patches".
	Kind string

	// Pattern is the regex pattern for URL rewriting.
	Pattern string

	// Replacement is the replacement string for URL rewriting.
	Replacement string

	// Enabled is the flag for include_patches.
	Enabled bool
}

// Ensure TransformValue implements the required interfaces.
var (
	_ starlark.Value    = (*TransformValue)(nil)
	_ starlark.HasAttrs = (*TransformValue)(nil)
)

// String returns a string representation of the transformation.
func (t *TransformValue) String() string {
	switch t.Kind {
	case TransformRewriteURLs:
		return fmt.Sprintf("sync.rewrite_source_urls(pattern=%q, replacement=%q)", t.Pattern, t.Replacement)
	case TransformSkipYanked:
		return "sync.skip_yanked()"
	case TransformIncludePatches:
		return fmt.Sprintf("sync.include_patches(enabled=%t)", t.Enabled)
	default:
		return "transform"
	}
}

// newTransformValue creates a new TransformValue with common initialization.
func newTransformValue(transformType string) *TransformValue {
	t := &TransformValue{
		Base: value.NewBase("transform"),
		Kind: transformType,
	}
	t.AttrAccessor = value.NewAttrAccessor()
	t.Set("type", starlark.String(transformType))
	return t
}

// WorkflowValue represents a sync workflow configuration in Starlark.
type WorkflowValue struct {
	value.Base
	value.AttrAccessor

	// Name is the unique identifier for the workflow.
	Name string

	// Description provides human-readable documentation.
	Description string

	// Origin is the source registry.
	Origin *RegistryValue

	// Destination is the target registry.
	Destination *RegistryValue

	// Modules is the list of module names or patterns to sync.
	Modules []string

	// Versions is the version selection strategy.
	Versions *VersionSelector

	// Transformations is the list of transformations to apply.
	Transformations []*TransformValue
}

// Ensure WorkflowValue implements the required interfaces.
var (
	_ starlark.Value    = (*WorkflowValue)(nil)
	_ starlark.HasAttrs = (*WorkflowValue)(nil)
)

// String returns a string representation of the workflow.
func (w *WorkflowValue) String() string {
	return fmt.Sprintf("sync.workflow(name=%q)", w.Name)
}

// workflowRegistry collects workflows defined in Starlark.
type workflowRegistry struct {
	workflows []*WorkflowValue
}

// newWorkflowRegistry creates a new workflow registry.
func newWorkflowRegistry() *workflowRegistry {
	return &workflowRegistry{
		workflows: make([]*WorkflowValue, 0),
	}
}

// add adds a workflow to the registry.
func (r *workflowRegistry) add(w *WorkflowValue) {
	r.workflows = append(r.workflows, w)
}

// SetWorkflowRegistry sets the workflow registry on a Starlark thread.
func SetWorkflowRegistry(thread *starlark.Thread) {
	thread.SetLocal(workflowRegistryKey, newWorkflowRegistry())
}

// GetWorkflows returns all workflows collected during Starlark execution.
func GetWorkflows(thread *starlark.Thread) []*WorkflowValue {
	if v := thread.Local(workflowRegistryKey); v != nil {
		if r, ok := v.(*workflowRegistry); ok {
			return r.workflows
		}
	}
	return nil
}

// getWorkflowRegistry retrieves the workflow registry from the thread.
func getWorkflowRegistry(thread *starlark.Thread) (*workflowRegistry, error) {
	if v := thread.Local(workflowRegistryKey); v != nil {
		if r, ok := v.(*workflowRegistry); ok {
			return r, nil
		}
	}
	return nil, fmt.Errorf("workflow registry not initialized (call SetWorkflowRegistry before execution)")
}

// syncLatest creates a sync.latest version selector.
func syncLatest(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.latest", args, kwargs)

	var count int
	if err := u.Optional("count", &count, 1); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	if count < 1 {
		return nil, fmt.Errorf("sync.latest: count must be at least 1, got %d", count)
	}

	v := newVersionSelector(VersionSelectorLatest)
	v.Count = count
	v.Set("count", starlark.MakeInt(count))
	return v, nil
}

// syncAll creates a sync.all version selector.
func syncAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.all", args, kwargs)

	if err := u.Validate(); err != nil {
		return nil, err
	}

	return newVersionSelector(VersionSelectorAll), nil
}

// syncSince creates a sync.since version selector.
func syncSince(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.since", args, kwargs)

	var date string
	if err := u.Required("date", &date); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	v := newVersionSelector(VersionSelectorSince)
	v.Since = date
	v.Set("date", starlark.String(date))
	return v, nil
}

// syncRange creates a sync.range version selector.
func syncRange(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.range", args, kwargs)

	var minVer, maxVer string
	if err := u.Optional("min", &minVer, ""); err != nil {
		return nil, err
	}
	if err := u.Optional("max", &maxVer, ""); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// At least one bound must be specified
	if minVer == "" && maxVer == "" {
		return nil, fmt.Errorf("sync.range: at least one of 'min' or 'max' must be specified")
	}

	v := newVersionSelector(VersionSelectorRange)
	v.MinVer = minVer
	v.MaxVer = maxVer
	if minVer != "" {
		v.Set("min", starlark.String(minVer))
	}
	if maxVer != "" {
		v.Set("max", starlark.String(maxVer))
	}
	return v, nil
}

// syncRewriteSourceURLs creates a sync.rewrite_source_urls transformation.
func syncRewriteSourceURLs(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.rewrite_source_urls", args, kwargs)

	var pattern, replacement string
	if err := u.Required("pattern", &pattern); err != nil {
		return nil, err
	}
	if err := u.Required("replacement", &replacement); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	t := newTransformValue(TransformRewriteURLs)
	t.Pattern = pattern
	t.Replacement = replacement
	t.Set("pattern", starlark.String(pattern))
	t.Set("replacement", starlark.String(replacement))
	return t, nil
}

// syncSkipYanked creates a sync.skip_yanked transformation.
func syncSkipYanked(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.skip_yanked", args, kwargs)

	if err := u.Validate(); err != nil {
		return nil, err
	}

	return newTransformValue(TransformSkipYanked), nil
}

// syncIncludePatches creates a sync.include_patches transformation.
func syncIncludePatches(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.include_patches", args, kwargs)

	var enabled bool
	if err := u.Optional("enabled", &enabled, true); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	t := newTransformValue(TransformIncludePatches)
	t.Enabled = enabled
	t.Set("enabled", starlark.Bool(enabled))
	return t, nil
}

// syncWorkflow creates a sync.workflow configuration and registers it.
func syncWorkflow(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("sync.workflow", args, kwargs)

	var name, description string
	var originVal, destVal, versionsVal starlark.Value
	var modulesVal starlark.Value
	var transformsVal starlark.Value

	if err := u.Required("name", &name); err != nil {
		return nil, err
	}
	if err := u.Optional("description", &description, ""); err != nil {
		return nil, err
	}
	if err := u.Required("origin", &originVal); err != nil {
		return nil, err
	}
	if err := u.Required("destination", &destVal); err != nil {
		return nil, err
	}
	if err := u.Required("modules", &modulesVal); err != nil {
		return nil, err
	}
	if err := u.Optional("versions", &versionsVal, starlark.None); err != nil {
		return nil, err
	}
	if err := u.Optional("transformations", &transformsVal, starlark.None); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// Validate origin
	origin, ok := originVal.(*RegistryValue)
	if !ok {
		return nil, fmt.Errorf("sync.workflow: origin must be a registry value, got %s", originVal.Type())
	}

	// Validate destination
	dest, ok := destVal.(*RegistryValue)
	if !ok {
		return nil, fmt.Errorf("sync.workflow: destination must be a registry value, got %s", destVal.Type())
	}

	// Parse modules list
	modulesList, ok := modulesVal.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("sync.workflow: modules must be a list, got %s", modulesVal.Type())
	}
	modules := make([]string, modulesList.Len())
	for i := 0; i < modulesList.Len(); i++ {
		s, ok := modulesList.Index(i).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("sync.workflow: modules[%d] must be a string, got %s", i, modulesList.Index(i).Type())
		}
		modules[i] = string(s)
	}

	// Parse versions selector (optional, defaults to latest(1))
	var versions *VersionSelector
	if versionsVal != nil && versionsVal != starlark.None {
		v, ok := versionsVal.(*VersionSelector)
		if !ok {
			return nil, fmt.Errorf("sync.workflow: versions must be a version selector, got %s", versionsVal.Type())
		}
		versions = v
	} else {
		// Default to latest(1)
		versions = newVersionSelector(VersionSelectorLatest)
		versions.Count = 1
		versions.Set("count", starlark.MakeInt(1))
	}

	// Parse transformations (optional)
	var transforms []*TransformValue
	if transformsVal != nil && transformsVal != starlark.None {
		transformsList, ok := transformsVal.(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("sync.workflow: transformations must be a list, got %s", transformsVal.Type())
		}
		transforms = make([]*TransformValue, transformsList.Len())
		for i := 0; i < transformsList.Len(); i++ {
			t, ok := transformsList.Index(i).(*TransformValue)
			if !ok {
				return nil, fmt.Errorf("sync.workflow: transformations[%d] must be a transform value, got %s", i, transformsList.Index(i).Type())
			}
			transforms[i] = t
		}
	}

	// Create the workflow
	w := &WorkflowValue{
		Base:            value.NewBase("workflow"),
		Name:            name,
		Description:     description,
		Origin:          origin,
		Destination:     dest,
		Modules:         modules,
		Versions:        versions,
		Transformations: transforms,
	}
	w.AttrAccessor = value.NewAttrAccessor()
	w.Set("name", starlark.String(name))
	w.Set("description", starlark.String(description))
	w.Set("origin", origin)
	w.Set("destination", dest)
	w.Set("modules", modulesVal)
	w.Set("versions", versions)
	if transformsVal != nil && transformsVal != starlark.None {
		w.Set("transformations", transformsVal)
	}

	// Register the workflow
	registry, err := getWorkflowRegistry(thread)
	if err != nil {
		return nil, err
	}
	registry.add(w)

	return w, nil
}

// SyncModule returns the sync Starlark module.
func SyncModule() starlark.StringDict {
	return starlark.StringDict{
		"workflow":            starlark.NewBuiltin("sync.workflow", syncWorkflow),
		"latest":              starlark.NewBuiltin("sync.latest", syncLatest),
		"all":                 starlark.NewBuiltin("sync.all", syncAll),
		"since":               starlark.NewBuiltin("sync.since", syncSince),
		"range":               starlark.NewBuiltin("sync.range", syncRange),
		"rewrite_source_urls": starlark.NewBuiltin("sync.rewrite_source_urls", syncRewriteSourceURLs),
		"skip_yanked":         starlark.NewBuiltin("sync.skip_yanked", syncSkipYanked),
		"include_patches":     starlark.NewBuiltin("sync.include_patches", syncIncludePatches),
	}
}

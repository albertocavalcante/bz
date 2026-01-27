package config

import (
	"github.com/albertocavalcante/bz/internal/config/modules"
)

// RegistryFromValue converts a RegistryValue to a Registry.
func RegistryFromValue(v *modules.RegistryValue) *Registry {
	if v == nil {
		return nil
	}

	r := &Registry{
		Type:   v.Kind,
		URL:    v.URL,
		Branch: v.Branch,
		Ref:    v.Ref,
	}

	if v.Auth != nil {
		r.Auth = AuthFromValue(v.Auth)
	}

	return r
}

// AuthFromValue converts an AuthValue to an Auth.
func AuthFromValue(v *modules.AuthValue) *Auth {
	if v == nil {
		return nil
	}

	return &Auth{
		Type:        v.Kind,
		Username:    v.Username,
		Password:    v.Password,
		EnvVar:      v.EnvVar,
		TokenValue:  v.TokenValue,
		HeaderName:  v.HeaderName,
		HeaderValue: v.HeaderValue,
	}
}

// WorkflowFromValue converts a WorkflowValue to a Workflow.
func WorkflowFromValue(v *modules.WorkflowValue) *Workflow {
	if v == nil {
		return nil
	}

	w := &Workflow{
		Name:        v.Name,
		Description: v.Description,
		Modules:     v.Modules,
	}

	if v.Origin != nil {
		w.Origin = RegistryFromValue(v.Origin)
	}

	if v.Destination != nil {
		w.Destination = RegistryFromValue(v.Destination)
	}

	if v.Versions != nil {
		w.Versions = VersionSelectorFromValue(v.Versions)
	}

	if len(v.Transformations) > 0 {
		w.Transformations = make([]*Transform, len(v.Transformations))
		for i, t := range v.Transformations {
			w.Transformations[i] = TransformFromValue(t)
		}
	}

	return w
}

// VersionSelectorFromValue converts a VersionSelector module value to a config VersionSelector.
func VersionSelectorFromValue(v *modules.VersionSelector) *VersionSelector {
	if v == nil {
		return nil
	}

	return &VersionSelector{
		Type:   v.Kind,
		Count:  v.Count,
		Since:  v.Since,
		MinVer: v.MinVer,
		MaxVer: v.MaxVer,
	}
}

// TransformFromValue converts a TransformValue to a Transform.
func TransformFromValue(v *modules.TransformValue) *Transform {
	if v == nil {
		return nil
	}

	return &Transform{
		Type:        v.Kind,
		Pattern:     v.Pattern,
		Replacement: v.Replacement,
		Enabled:     v.Enabled,
	}
}

// DefaultsFromValue converts a DefaultsValue to a Defaults.
func DefaultsFromValue(v *modules.DefaultsValue) *Defaults {
	if v == nil {
		return nil
	}

	d := &Defaults{
		CacheDir: v.CacheDir,
	}

	if v.Registry != nil {
		d.Registry = RegistryFromValue(v.Registry)
	}

	if len(v.FallbackRegistries) > 0 {
		d.FallbackRegistries = make([]*Registry, len(v.FallbackRegistries))
		for i, r := range v.FallbackRegistries {
			d.FallbackRegistries[i] = RegistryFromValue(r)
		}
	}

	return d
}

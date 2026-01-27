package value

import (
	"fmt"
	"sort"

	"go.starlark.net/starlark"
)

// AttrAccessor helps implement the starlark.HasAttrs interface.
// It provides a simple way to manage attributes on custom Starlark values.
//
// Example:
//
//	type MyValue struct {
//	    value.Base
//	    value.AttrAccessor
//	}
//
//	func NewMyValue() *MyValue {
//	    v := &MyValue{
//	        Base: value.NewBase("my_value"),
//	    }
//	    v.AttrAccessor = value.NewAttrAccessor()
//	    v.Set("name", starlark.String("example"))
//	    return v
//	}
type AttrAccessor struct {
	attrs map[string]starlark.Value
}

// NewAttrAccessor creates a new AttrAccessor with an empty attribute map.
func NewAttrAccessor() AttrAccessor {
	return AttrAccessor{
		attrs: make(map[string]starlark.Value),
	}
}

// Set adds or updates an attribute.
func (a *AttrAccessor) Set(name string, value starlark.Value) {
	if a.attrs == nil {
		a.attrs = make(map[string]starlark.Value)
	}
	a.attrs[name] = value
}

// Attr returns the value of the named attribute, or an error if not found.
// This implements part of the starlark.HasAttrs interface.
func (a *AttrAccessor) Attr(name string) (starlark.Value, error) {
	if a.attrs == nil {
		return nil, fmt.Errorf("no such attribute %q", name)
	}
	v, ok := a.attrs[name]
	if !ok {
		return nil, fmt.Errorf("no such attribute %q", name)
	}
	return v, nil
}

// AttrNames returns the list of available attribute names in sorted order.
// This implements part of the starlark.HasAttrs interface.
func (a *AttrAccessor) AttrNames() []string {
	if a.attrs == nil {
		return nil
	}
	names := make([]string, 0, len(a.attrs))
	for name := range a.attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has returns true if the attribute exists.
func (a *AttrAccessor) Has(name string) bool {
	if a.attrs == nil {
		return false
	}
	_, ok := a.attrs[name]
	return ok
}

// Get returns the value of the named attribute, or nil if not found.
// Unlike Attr, this does not return an error.
func (a *AttrAccessor) Get(name string) starlark.Value {
	if a.attrs == nil {
		return nil
	}
	return a.attrs[name]
}

// Delete removes an attribute.
func (a *AttrAccessor) Delete(name string) {
	if a.attrs != nil {
		delete(a.attrs, name)
	}
}

// Clear removes all attributes.
func (a *AttrAccessor) Clear() {
	a.attrs = make(map[string]starlark.Value)
}

// Len returns the number of attributes.
func (a *AttrAccessor) Len() int {
	if a.attrs == nil {
		return 0
	}
	return len(a.attrs)
}

// NOTE: AttrAccessor provides Attr and AttrNames methods for the starlark.HasAttrs
// interface, but must be composed with Base (or another starlark.Value implementation)
// to satisfy the full interface since HasAttrs embeds Value.

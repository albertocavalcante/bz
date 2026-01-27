package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Base provides a common implementation for custom Starlark values.
// Embed this in your custom value types to get default implementations
// of the starlark.Value interface methods.
//
// Example:
//
//	type MyValue struct {
//	    value.Base
//	    // ... your fields
//	}
//
//	func NewMyValue() *MyValue {
//	    return &MyValue{
//	        Base: value.NewBase("my_value"),
//	    }
//	}
type Base struct {
	name string
}

// NewBase creates a new Base with the given type name.
func NewBase(name string) Base {
	return Base{name: name}
}

// String returns the string representation of the value.
// By default, it returns the type name.
func (b *Base) String() string {
	return b.name
}

// Type returns the type name of the value.
func (b *Base) Type() string {
	return b.name
}

// Freeze is called when the value becomes immutable.
// The default implementation does nothing.
func (b *Base) Freeze() {}

// Truth returns the truth value. Custom values are truthy by default.
func (b *Base) Truth() starlark.Bool {
	return true
}

// Hash returns the hash of the value.
// Custom values are unhashable by default.
func (b *Base) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", b.name)
}

// Name returns the type name.
func (b *Base) Name() string {
	return b.name
}

// Ensure Base implements starlark.Value at compile time.
var _ starlark.Value = (*Base)(nil)

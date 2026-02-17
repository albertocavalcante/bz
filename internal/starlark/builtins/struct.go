package builtins

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/albertocavalcante/bz/internal/starlark/convertutil"
	valueconv "github.com/albertocavalcante/bz/internal/starlark/value"
)

// NewStruct creates a Starlark struct from a map of fields.
// The constructor name is used for the struct's type representation.
func NewStruct(constructor string, fields map[string]starlark.Value) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String(constructor), fields)
}

// StructBuilder provides a fluent interface for building Starlark structs.
type StructBuilder struct {
	constructor string
	fields      map[string]starlark.Value
}

// NewStructBuilder creates a new StructBuilder with the given constructor name.
func NewStructBuilder(constructor string) *StructBuilder {
	return &StructBuilder{
		constructor: constructor,
		fields:      make(map[string]starlark.Value),
	}
}

// Set adds a field to the struct.
// The value is automatically converted to a Starlark value using ToStarlarkValue.
func (b *StructBuilder) Set(key string, value any) *StructBuilder {
	b.fields[key] = ToStarlarkValue(value)
	return b
}

// SetValue adds a pre-converted Starlark value to the struct.
func (b *StructBuilder) SetValue(key string, value starlark.Value) *StructBuilder {
	b.fields[key] = value
	return b
}

// Build creates the final Starlark struct.
func (b *StructBuilder) Build() *starlarkstruct.Struct {
	return NewStruct(b.constructor, b.fields)
}

// ToStarlarkValue converts a Go value to a Starlark value.
// Supported types:
//   - nil -> None
//   - bool -> Bool
//   - int, int64 -> Int
//   - float64 -> Float
//   - string -> String
//   - []string -> List of Strings
//   - []interface{} -> List
//   - map[string]interface{} -> Dict
//   - starlark.Value -> passed through unchanged
func ToStarlarkValue(v any) starlark.Value {
	converted, err := valueconv.ToStarlark(v)
	if err == nil {
		return converted
	}
	// Preserve historical behavior for unsupported values.
	return starlark.String(fmt.Sprintf("%v", v))
}

// FromStarlarkValue converts a Starlark value to a Go value.
// Returns:
//   - starlark.None -> nil
//   - starlark.Bool -> bool
//   - starlark.Int -> int64
//   - starlark.Float -> float64
//   - starlark.String -> string
//   - starlark.List -> []interface{}
//   - starlark.Dict -> map[string]interface{}
//   - other -> the original starlark.Value
func FromStarlarkValue(v starlark.Value) any {
	converted, err := convertutil.ToGoValue(v, convertutil.ToGoOptions{
		IntOverflowAsString: true,
		UnknownAsValue:      true,
	})
	if err != nil {
		// Preserve historical behavior for dicts with unsupported keys:
		// skip non-string keys rather than failing conversion.
		if dict, ok := v.(*starlark.Dict); ok {
			result := make(map[string]any)
			for _, item := range dict.Items() {
				key, ok := item[0].(starlark.String)
				if !ok {
					continue
				}
				result[string(key)] = FromStarlarkValue(item[1])
			}
			return result
		}
		return v
	}
	return converted
}

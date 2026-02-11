package builtins

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
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
	if v == nil {
		return starlark.None
	}

	switch val := v.(type) {
	case starlark.Value:
		return val
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []string:
		elems := make([]starlark.Value, len(val))
		for i, s := range val {
			elems[i] = starlark.String(s)
		}
		return starlark.NewList(elems)
	case []any:
		elems := make([]starlark.Value, len(val))
		for i, elem := range val {
			elems[i] = ToStarlarkValue(elem)
		}
		return starlark.NewList(elems)
	case map[string]any:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			_ = dict.SetKey(starlark.String(k), ToStarlarkValue(v))
		}
		return dict
	default:
		// Fallback: convert to string representation using fmt
		return starlark.String(fmt.Sprintf("%v", val))
	}
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
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = FromStarlarkValue(val.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range val.Items() {
			if key, ok := item[0].(starlark.String); ok {
				result[string(key)] = FromStarlarkValue(item[1])
			}
		}
		return result
	default:
		return v
	}
}

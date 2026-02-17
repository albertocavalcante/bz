package value

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/albertocavalcante/bz/internal/starlark/convertutil"
)

// ToStarlark converts a Go value to a Starlark value.
//
// Supported Go types:
//   - nil: starlark.None
//   - string: starlark.String
//   - bool: starlark.Bool
//   - int, int8, int16, int32, int64: starlark.Int
//   - uint, uint8, uint16, uint32, uint64: starlark.Int
//   - float32, float64: starlark.Float
//   - []T: starlark.List (elements converted recursively)
//   - map[string]T: starlark.Dict (values converted recursively)
//   - starlark.Value: returned as-is
func ToStarlark(v any) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}

	// Check if it's already a Starlark value
	if sv, ok := v.(starlark.Value); ok {
		return sv, nil
	}

	rv := reflect.ValueOf(v)

	switch rv.Kind() {
	case reflect.String:
		return starlark.String(rv.String()), nil

	case reflect.Bool:
		return starlark.Bool(rv.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(rv.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return starlark.Float(rv.Float()), nil

	case reflect.Slice:
		list := make([]starlark.Value, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			elem, err := ToStarlark(rv.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("list element %d: %w", i, err)
			}
			list[i] = elem
		}
		return starlark.NewList(list), nil

	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key must be string, got %s", rv.Type().Key())
		}
		dict := starlark.NewDict(rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key := starlark.String(iter.Key().String())
			val, err := ToStarlark(iter.Value().Interface())
			if err != nil {
				return nil, fmt.Errorf("map value for %q: %w", iter.Key().String(), err)
			}
			if err := dict.SetKey(key, val); err != nil {
				return nil, err
			}
		}
		return dict, nil

	case reflect.Pointer:
		if rv.IsNil() {
			return starlark.None, nil
		}
		return ToStarlark(rv.Elem().Interface())

	default:
		return nil, fmt.Errorf("unsupported Go type: %T", v)
	}
}

// FromStarlark converts a Starlark value to a Go value.
// The dest parameter must be a pointer to the destination variable.
//
// Supported destination types:
//   - *string: from starlark.String
//   - *bool: from starlark.Bool
//   - *int, *int64: from starlark.Int
//   - *float64: from starlark.Float or starlark.Int
//   - *[]string: from starlark.List of strings
//   - *[]interface{}: from starlark.List (elements converted recursively)
//   - *map[string]string: from starlark.Dict with string keys and values
//   - *map[string]interface{}: from starlark.Dict (values converted recursively)
//   - *starlark.Value: any value stored directly
func FromStarlark(val starlark.Value, dest any) error {
	if dest == nil {
		return fmt.Errorf("destination is nil")
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer {
		return fmt.Errorf("destination must be a pointer, got %T", dest)
	}
	if rv.IsNil() {
		return fmt.Errorf("destination pointer is nil")
	}

	elem := rv.Elem()

	switch d := dest.(type) {
	case *starlark.Value:
		*d = val
		return nil

	case *string:
		return convertToString(val, d)

	case *bool:
		return convertToBool(val, d)

	case *int:
		return convertToInt(val, d)

	case *int64:
		return convertToInt64(val, d)

	case *float64:
		return convertToFloat64(val, d)

	case *[]string:
		return convertToStringSlice(val, d)

	case *[]any:
		return convertToInterfaceSlice(val, d)

	case *map[string]string:
		return convertToStringMap(val, d)

	case *map[string]any:
		return convertToInterfaceMap(val, d)

	default:
		// Handle via reflection for other pointer types
		return convertReflect(val, elem)
	}
}

func convertToString(val starlark.Value, dest *string) error {
	s, ok := val.(starlark.String)
	if !ok {
		return fmt.Errorf("expected string, got %s", val.Type())
	}
	*dest = string(s)
	return nil
}

func convertToBool(val starlark.Value, dest *bool) error {
	b, ok := val.(starlark.Bool)
	if !ok {
		return fmt.Errorf("expected bool, got %s", val.Type())
	}
	*dest = bool(b)
	return nil
}

func convertToInt(val starlark.Value, dest *int) error {
	i, ok := val.(starlark.Int)
	if !ok {
		return fmt.Errorf("expected int, got %s", val.Type())
	}
	v, ok := i.Int64()
	if !ok {
		return fmt.Errorf("integer overflow")
	}
	*dest = int(v)
	return nil
}

func convertToInt64(val starlark.Value, dest *int64) error {
	i, ok := val.(starlark.Int)
	if !ok {
		return fmt.Errorf("expected int, got %s", val.Type())
	}
	v, ok := i.Int64()
	if !ok {
		return fmt.Errorf("integer overflow")
	}
	*dest = v
	return nil
}

func convertToFloat64(val starlark.Value, dest *float64) error {
	switch v := val.(type) {
	case starlark.Float:
		*dest = float64(v)
	case starlark.Int:
		i, ok := v.Int64()
		if !ok {
			return fmt.Errorf("integer overflow")
		}
		*dest = float64(i)
	default:
		return fmt.Errorf("expected float or int, got %s", val.Type())
	}
	return nil
}

func convertToStringSlice(val starlark.Value, dest *[]string) error {
	list, ok := val.(*starlark.List)
	if !ok {
		return fmt.Errorf("expected list, got %s", val.Type())
	}
	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		s, ok := list.Index(i).(starlark.String)
		if !ok {
			return fmt.Errorf("list element %d: expected string, got %s", i, list.Index(i).Type())
		}
		result[i] = string(s)
	}
	*dest = result
	return nil
}

func convertToInterfaceSlice(val starlark.Value, dest *[]any) error {
	list, ok := val.(*starlark.List)
	if !ok {
		return fmt.Errorf("expected list, got %s", val.Type())
	}
	result := make([]any, list.Len())
	for i := 0; i < list.Len(); i++ {
		v, err := toGoValue(list.Index(i))
		if err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
		result[i] = v
	}
	*dest = result
	return nil
}

func convertToStringMap(val starlark.Value, dest *map[string]string) error {
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return fmt.Errorf("expected dict, got %s", val.Type())
	}
	result := make(map[string]string)
	for _, item := range dict.Items() {
		k, ok := item[0].(starlark.String)
		if !ok {
			return fmt.Errorf("dict key: expected string, got %s", item[0].Type())
		}
		v, ok := item[1].(starlark.String)
		if !ok {
			return fmt.Errorf("dict value for %q: expected string, got %s", k, item[1].Type())
		}
		result[string(k)] = string(v)
	}
	*dest = result
	return nil
}

func convertToInterfaceMap(val starlark.Value, dest *map[string]any) error {
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return fmt.Errorf("expected dict, got %s", val.Type())
	}
	result := make(map[string]any)
	for _, item := range dict.Items() {
		k, ok := item[0].(starlark.String)
		if !ok {
			return fmt.Errorf("dict key: expected string, got %s", item[0].Type())
		}
		v, err := toGoValue(item[1])
		if err != nil {
			return fmt.Errorf("dict value for %q: %w", k, err)
		}
		result[string(k)] = v
	}
	*dest = result
	return nil
}

func convertReflect(val starlark.Value, dest reflect.Value) error {
	switch dest.Kind() {
	case reflect.String:
		s, ok := val.(starlark.String)
		if !ok {
			return fmt.Errorf("expected string, got %s", val.Type())
		}
		dest.SetString(string(s))

	case reflect.Bool:
		b, ok := val.(starlark.Bool)
		if !ok {
			return fmt.Errorf("expected bool, got %s", val.Type())
		}
		dest.SetBool(bool(b))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := val.(starlark.Int)
		if !ok {
			return fmt.Errorf("expected int, got %s", val.Type())
		}
		v, ok := i.Int64()
		if !ok {
			return fmt.Errorf("integer overflow")
		}
		dest.SetInt(v)

	case reflect.Float32, reflect.Float64:
		switch v := val.(type) {
		case starlark.Float:
			dest.SetFloat(float64(v))
		case starlark.Int:
			i, ok := v.Int64()
			if !ok {
				return fmt.Errorf("integer overflow")
			}
			dest.SetFloat(float64(i))
		default:
			return fmt.Errorf("expected float or int, got %s", val.Type())
		}

	default:
		return fmt.Errorf("unsupported destination type: %s", dest.Type())
	}

	return nil
}

// toGoValue converts a Starlark value to a Go interface{} value.
func toGoValue(val starlark.Value) (any, error) {
	return convertutil.ToGoValue(val, convertutil.ToGoOptions{
		IntOverflowAsString: false,
		UnknownAsValue:      false,
	})
}

// MustString extracts a string from a Starlark value or returns an error.
func MustString(val starlark.Value) (string, error) {
	s, ok := val.(starlark.String)
	if !ok {
		return "", fmt.Errorf("expected string, got %s", val.Type())
	}
	return string(s), nil
}

// MustStringList extracts a list of strings from a Starlark value or returns an error.
func MustStringList(val starlark.Value) ([]string, error) {
	list, ok := val.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("expected list, got %s", val.Type())
	}
	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		s, ok := list.Index(i).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("list element %d: expected string, got %s", i, list.Index(i).Type())
		}
		result[i] = string(s)
	}
	return result, nil
}

// MustStringDict extracts a dict with string keys and values from a Starlark value or returns an error.
func MustStringDict(val starlark.Value) (map[string]string, error) {
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("expected dict, got %s", val.Type())
	}
	result := make(map[string]string)
	for _, item := range dict.Items() {
		k, ok := item[0].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict key: expected string, got %s", item[0].Type())
		}
		v, ok := item[1].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict value for %q: expected string, got %s", k, item[1].Type())
		}
		result[string(k)] = string(v)
	}
	return result, nil
}

// MustBool extracts a boolean from a Starlark value or returns an error.
func MustBool(val starlark.Value) (bool, error) {
	b, ok := val.(starlark.Bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %s", val.Type())
	}
	return bool(b), nil
}

// MustInt extracts an integer from a Starlark value or returns an error.
func MustInt(val starlark.Value) (int, error) {
	i, ok := val.(starlark.Int)
	if !ok {
		return 0, fmt.Errorf("expected int, got %s", val.Type())
	}
	v, ok := i.Int64()
	if !ok {
		return 0, fmt.Errorf("integer overflow")
	}
	return int(v), nil
}

// MustFloat extracts a float from a Starlark value or returns an error.
// Accepts both Float and Int values.
func MustFloat(val starlark.Value) (float64, error) {
	switch v := val.(type) {
	case starlark.Float:
		return float64(v), nil
	case starlark.Int:
		i, ok := v.Int64()
		if !ok {
			return 0, fmt.Errorf("integer overflow")
		}
		return float64(i), nil
	default:
		return 0, fmt.Errorf("expected float or int, got %s", val.Type())
	}
}

// NewStruct creates a new Starlark struct from a map of attributes.
func NewStruct(attrs map[string]starlark.Value) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String("struct"), starlark.StringDict(attrs))
}

// NewNamedStruct creates a new Starlark struct with a custom constructor name.
// Note: The constructor name appears in the String() representation, but Type()
// always returns "struct" for starlarkstruct.Struct.
func NewNamedStruct(name string, attrs map[string]starlark.Value) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlark.String(name), starlark.StringDict(attrs))
}

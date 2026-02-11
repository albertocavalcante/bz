package starlark

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

// ToValue converts a Go value to a Starlark value.
// Supported types:
//   - nil -> None
//   - bool -> Bool
//   - int, int8, int16, int32, int64 -> Int
//   - uint, uint8, uint16, uint32, uint64 -> Int
//   - float32, float64 -> Float
//   - string -> String
//   - []T -> List
//   - map[string]T -> Dict
//   - starlark.Value -> returned as-is
func ToValue(v any) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}

	// If it's already a Starlark value, return it
	if sv, ok := v.(starlark.Value); ok {
		return sv, nil
	}

	// Use reflection for everything else
	rv := reflect.ValueOf(v)
	return toValueReflect(rv)
}

// toValueReflect converts a reflect.Value to a Starlark value.
func toValueReflect(rv reflect.Value) (starlark.Value, error) {
	// Handle invalid/nil values
	if !rv.IsValid() {
		return starlark.None, nil
	}

	// Dereference pointers
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return starlark.None, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool:
		return starlark.Bool(rv.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(rv.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return starlark.Float(rv.Float()), nil

	case reflect.String:
		return starlark.String(rv.String()), nil

	case reflect.Slice, reflect.Array:
		return sliceToList(rv)

	case reflect.Map:
		return mapToDict(rv)

	case reflect.Interface:
		if rv.IsNil() {
			return starlark.None, nil
		}
		return toValueReflect(rv.Elem())

	default:
		return nil, NewTypeErrorf("cannot convert %s to Starlark value", rv.Type())
	}
}

// sliceToList converts a slice or array to a Starlark list.
func sliceToList(rv reflect.Value) (starlark.Value, error) {
	elems := make([]starlark.Value, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem, err := toValueReflect(rv.Index(i))
		if err != nil {
			return nil, fmt.Errorf("list element %d: %w", i, err)
		}
		elems[i] = elem
	}
	return starlark.NewList(elems), nil
}

// mapToDict converts a map to a Starlark dict.
func mapToDict(rv reflect.Value) (starlark.Value, error) {
	dict := starlark.NewDict(rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key, err := toValueReflect(iter.Key())
		if err != nil {
			return nil, fmt.Errorf("dict key: %w", err)
		}
		value, err := toValueReflect(iter.Value())
		if err != nil {
			return nil, fmt.Errorf("dict value: %w", err)
		}
		if err := dict.SetKey(key, value); err != nil {
			return nil, fmt.Errorf("dict set: %w", err)
		}
	}
	return dict, nil
}

// FromValue converts a Starlark value to a Go value.
// The target must be a pointer to the destination type.
// Supported target types:
//   - *bool
//   - *int, *int8, *int16, *int32, *int64
//   - *uint, *uint8, *uint16, *uint32, *uint64
//   - *float32, *float64
//   - *string
//   - *[]T (for Starlark lists/tuples)
//   - *map[string]T (for Starlark dicts)
//   - *interface{} (returns the raw converted value)
func FromValue(v starlark.Value, target any) error {
	if target == nil {
		return NewTypeError("non-nil pointer", "nil")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer {
		return NewTypeError("pointer", rv.Kind().String())
	}
	if rv.IsNil() {
		return NewTypeError("non-nil pointer", "nil pointer")
	}

	return fromValueReflect(v, rv.Elem())
}

// fromValueReflect sets a reflect.Value from a Starlark value.
func fromValueReflect(v starlark.Value, rv reflect.Value) error {
	// Handle None
	if v == starlark.None {
		rv.Set(reflect.Zero(rv.Type()))
		return nil
	}

	// Handle interface{} target
	if rv.Kind() == reflect.Interface {
		converted, err := toGoValue(v)
		if err != nil {
			return err
		}
		rv.Set(reflect.ValueOf(converted))
		return nil
	}

	switch rv.Kind() {
	case reflect.Bool:
		b, ok := v.(starlark.Bool)
		if !ok {
			return NewTypeError("bool", v.Type())
		}
		rv.SetBool(bool(b))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := v.(starlark.Int)
		if !ok {
			return NewTypeError("int", v.Type())
		}
		val, ok := i.Int64()
		if !ok {
			return NewTypeErrorf("integer %v overflows %s", i, rv.Type())
		}
		rv.SetInt(val)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, ok := v.(starlark.Int)
		if !ok {
			return NewTypeError("int", v.Type())
		}
		val, ok := i.Uint64()
		if !ok {
			return NewTypeErrorf("integer %v cannot be converted to %s", i, rv.Type())
		}
		rv.SetUint(val)

	case reflect.Float32, reflect.Float64:
		switch f := v.(type) {
		case starlark.Float:
			rv.SetFloat(float64(f))
		case starlark.Int:
			// Allow int -> float conversion
			val, ok := f.Int64()
			if !ok {
				return NewTypeErrorf("integer %v overflows float", f)
			}
			rv.SetFloat(float64(val))
		default:
			return NewTypeError("float", v.Type())
		}

	case reflect.String:
		s, ok := v.(starlark.String)
		if !ok {
			return NewTypeError("string", v.Type())
		}
		rv.SetString(string(s))

	case reflect.Slice:
		return listToSlice(v, rv)

	case reflect.Map:
		return dictToMap(v, rv)

	default:
		return NewTypeErrorf("cannot convert Starlark %s to Go %s", v.Type(), rv.Type())
	}

	return nil
}

// listToSlice converts a Starlark list/tuple to a Go slice.
func listToSlice(v starlark.Value, rv reflect.Value) error {
	// Accept list or tuple
	var iter starlark.Iterator
	switch seq := v.(type) {
	case *starlark.List:
		iter = seq.Iterate()
	case starlark.Tuple:
		iter = seq.Iterate()
	default:
		return NewTypeError("list or tuple", v.Type())
	}
	defer iter.Done()

	elemType := rv.Type().Elem()
	var elems []reflect.Value

	var elem starlark.Value
	for iter.Next(&elem) {
		elemRv := reflect.New(elemType).Elem()
		if err := fromValueReflect(elem, elemRv); err != nil {
			return fmt.Errorf("list element: %w", err)
		}
		elems = append(elems, elemRv)
	}

	slice := reflect.MakeSlice(rv.Type(), len(elems), len(elems))
	for i, elem := range elems {
		slice.Index(i).Set(elem)
	}
	rv.Set(slice)
	return nil
}

// dictToMap converts a Starlark dict to a Go map.
func dictToMap(v starlark.Value, rv reflect.Value) error {
	dict, ok := v.(*starlark.Dict)
	if !ok {
		return NewTypeError("dict", v.Type())
	}

	keyType := rv.Type().Key()
	valueType := rv.Type().Elem()

	m := reflect.MakeMap(rv.Type())

	for _, item := range dict.Items() {
		keyRv := reflect.New(keyType).Elem()
		if err := fromValueReflect(item[0], keyRv); err != nil {
			return fmt.Errorf("dict key: %w", err)
		}

		valueRv := reflect.New(valueType).Elem()
		if err := fromValueReflect(item[1], valueRv); err != nil {
			return fmt.Errorf("dict value: %w", err)
		}

		m.SetMapIndex(keyRv, valueRv)
	}

	rv.Set(m)
	return nil
}

// toGoValue converts a Starlark value to its natural Go representation.
//
//nolint:nilnil // nil is a valid Go representation of Starlark None
func toGoValue(v starlark.Value) (any, error) {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(val), nil
	case starlark.Int:
		if i, ok := val.Int64(); ok {
			return i, nil
		}
		// Return as string if too large
		return val.String(), nil
	case starlark.Float:
		return float64(val), nil
	case starlark.String:
		return string(val), nil
	case *starlark.List:
		return listToGoSlice(val)
	case starlark.Tuple:
		return tupleToGoSlice(val)
	case *starlark.Dict:
		return dictToGoMap(val)
	default:
		// Return the Starlark value itself for unknown types
		return val, nil
	}
}

// listToGoSlice converts a Starlark list to []interface{}.
func listToGoSlice(list *starlark.List) ([]any, error) {
	result := make([]any, list.Len())
	for i := 0; i < list.Len(); i++ {
		val, err := toGoValue(list.Index(i))
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// tupleToGoSlice converts a Starlark tuple to []interface{}.
func tupleToGoSlice(tuple starlark.Tuple) ([]any, error) {
	result := make([]any, len(tuple))
	for i, v := range tuple {
		val, err := toGoValue(v)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// dictToGoMap converts a Starlark dict to map[string]interface{}.
func dictToGoMap(dict *starlark.Dict) (map[string]any, error) {
	result := make(map[string]any, dict.Len())
	for _, item := range dict.Items() {
		key, ok := item[0].(starlark.String)
		if !ok {
			return nil, NewTypeError("string key", item[0].Type())
		}
		val, err := toGoValue(item[1])
		if err != nil {
			return nil, err
		}
		result[string(key)] = val
	}
	return result, nil
}

// ToString extracts a string from a Starlark value.
func ToString(v starlark.Value) (string, error) {
	s, ok := v.(starlark.String)
	if !ok {
		return "", NewTypeError("string", v.Type())
	}
	return string(s), nil
}

// ToStringSlice extracts a string slice from a Starlark list/tuple.
func ToStringSlice(v starlark.Value) ([]string, error) {
	var result []string
	if err := FromValue(v, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ToStringMap extracts a string map from a Starlark dict.
func ToStringMap(v starlark.Value) (map[string]string, error) {
	var result map[string]string
	if err := FromValue(v, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Package convertutil provides shared Starlark conversion helpers.
package convertutil

import (
	"fmt"

	"go.starlark.net/starlark"
)

// ToGoOptions controls conversion behavior.
type ToGoOptions struct {
	// IntOverflowAsString returns oversized integers as their string form.
	// When false, oversized integers return an error.
	IntOverflowAsString bool

	// UnknownAsValue returns unknown Starlark values as-is.
	// When false, unknown types return an error.
	UnknownAsValue bool
}

// ToGoValue converts a Starlark value to a natural Go representation.
//
// The mapping is:
//   - None -> nil
//   - Bool -> bool
//   - String -> string
//   - Int -> int64 (or string/error on overflow per options)
//   - Float -> float64
//   - List/Tuple -> []any
//   - Dict -> map[string]any
func ToGoValue(val starlark.Value, opts ToGoOptions) (any, error) {
	switch v := val.(type) {
	case starlark.NoneType:
		return nil, nil //nolint:nilnil // nil is the correct Go representation of Starlark None
	case starlark.Bool:
		return bool(v), nil
	case starlark.String:
		return string(v), nil
	case starlark.Int:
		i, ok := v.Int64()
		if ok {
			return i, nil
		}
		if opts.IntOverflowAsString {
			return v.String(), nil
		}
		return nil, fmt.Errorf("integer overflow")
	case starlark.Float:
		return float64(v), nil
	case *starlark.List:
		return listToGo(v, opts)
	case starlark.Tuple:
		return tupleToGo(v, opts)
	case *starlark.Dict:
		return dictToGo(v, opts)
	default:
		if opts.UnknownAsValue {
			return v, nil
		}
		return nil, fmt.Errorf("unsupported Starlark type: %s", val.Type())
	}
}

func listToGo(v *starlark.List, opts ToGoOptions) ([]any, error) {
	result := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem, err := ToGoValue(v.Index(i), opts)
		if err != nil {
			return nil, err
		}
		result[i] = elem
	}
	return result, nil
}

func tupleToGo(v starlark.Tuple, opts ToGoOptions) ([]any, error) {
	result := make([]any, len(v))
	for i, elem := range v {
		converted, err := ToGoValue(elem, opts)
		if err != nil {
			return nil, err
		}
		result[i] = converted
	}
	return result, nil
}

func dictToGo(v *starlark.Dict, opts ToGoOptions) (map[string]any, error) {
	result := make(map[string]any, v.Len())
	for _, item := range v.Items() {
		k, ok := item[0].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict key: expected string, got %s", item[0].Type())
		}
		converted, err := ToGoValue(item[1], opts)
		if err != nil {
			return nil, err
		}
		result[string(k)] = converted
	}
	return result, nil
}

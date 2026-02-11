package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Unpacker helps unpack Starlark function arguments to Go types.
// It tracks which arguments have been consumed and validates that
// no unknown keyword arguments were passed.
//
// Example:
//
//	func myBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
//	    u := value.NewUnpacker("my_func", args, kwargs)
//
//	    var url string
//	    if err := u.Required("url", &url); err != nil {
//	        return nil, err
//	    }
//
//	    var timeout int
//	    if err := u.Optional("timeout", &timeout, 30); err != nil {
//	        return nil, err
//	    }
//
//	    if err := u.Validate(); err != nil {
//	        return nil, err
//	    }
//
//	    // Use url and timeout...
//	}
type Unpacker struct {
	args       starlark.Tuple
	kwargs     []starlark.Tuple
	fnName     string
	argIndex   int
	usedKwargs map[string]bool
}

// NewUnpacker creates a new Unpacker for the given function arguments.
func NewUnpacker(fnName string, args starlark.Tuple, kwargs []starlark.Tuple) *Unpacker {
	return &Unpacker{
		args:       args,
		kwargs:     kwargs,
		fnName:     fnName,
		argIndex:   0,
		usedKwargs: make(map[string]bool),
	}
}

// Required unpacks a required argument.
// It first checks positional arguments, then keyword arguments.
// Returns an error if the argument is missing or has the wrong type.
//
// Supported destination types:
//   - *string: accepts starlark.String
//   - *int: accepts starlark.Int
//   - *bool: accepts starlark.Bool
//   - *[]string: accepts starlark.List of strings
//   - *map[string]string: accepts starlark.Dict with string keys and values
//   - *starlark.Value: accepts any value
func (u *Unpacker) Required(name string, dest any) error {
	// Check positional args first
	if u.argIndex < len(u.args) {
		val := u.args[u.argIndex]
		u.argIndex++
		return u.convert(name, val, dest)
	}

	// Check keyword args
	for _, kv := range u.kwargs {
		if string(kv[0].(starlark.String)) == name {
			u.usedKwargs[name] = true
			return u.convert(name, kv[1], dest)
		}
	}

	return fmt.Errorf("%s: missing required argument %q", u.fnName, name)
}

// Optional unpacks an optional argument with a default value.
// It first checks positional arguments, then keyword arguments.
// If the argument is not provided, the default value is used.
//
// Supported destination types are the same as Required.
func (u *Unpacker) Optional(name string, dest any, defaultVal any) error {
	// Check positional args first
	if u.argIndex < len(u.args) {
		val := u.args[u.argIndex]
		u.argIndex++
		return u.convert(name, val, dest)
	}

	// Check keyword args
	for _, kv := range u.kwargs {
		if string(kv[0].(starlark.String)) == name {
			u.usedKwargs[name] = true
			return u.convert(name, kv[1], dest)
		}
	}

	// Use default value
	return u.setDefault(dest, defaultVal)
}

// Validate checks that no unknown keyword arguments were passed
// and that all positional arguments were consumed.
func (u *Unpacker) Validate() error {
	// Check for unused positional args
	if u.argIndex < len(u.args) {
		return fmt.Errorf("%s: unexpected positional argument", u.fnName)
	}

	// Check for unknown kwargs
	for _, kv := range u.kwargs {
		name := string(kv[0].(starlark.String))
		if !u.usedKwargs[name] {
			return fmt.Errorf("%s: unexpected keyword argument %q", u.fnName, name)
		}
	}

	return nil
}

// RemainingArgs returns any unconsumed positional arguments.
func (u *Unpacker) RemainingArgs() starlark.Tuple {
	if u.argIndex >= len(u.args) {
		return nil
	}
	return u.args[u.argIndex:]
}

// convert converts a Starlark value to a Go value.
func (u *Unpacker) convert(name string, val starlark.Value, dest any) error {
	switch d := dest.(type) {
	case *string:
		s, ok := val.(starlark.String)
		if !ok {
			return fmt.Errorf("%s: %s: expected string, got %s", u.fnName, name, val.Type())
		}
		*d = string(s)

	case *int:
		i, ok := val.(starlark.Int)
		if !ok {
			return fmt.Errorf("%s: %s: expected int, got %s", u.fnName, name, val.Type())
		}
		v, ok := i.Int64()
		if !ok {
			return fmt.Errorf("%s: %s: integer overflow", u.fnName, name)
		}
		*d = int(v)

	case *int64:
		i, ok := val.(starlark.Int)
		if !ok {
			return fmt.Errorf("%s: %s: expected int, got %s", u.fnName, name, val.Type())
		}
		v, ok := i.Int64()
		if !ok {
			return fmt.Errorf("%s: %s: integer overflow", u.fnName, name)
		}
		*d = v

	case *bool:
		b, ok := val.(starlark.Bool)
		if !ok {
			return fmt.Errorf("%s: %s: expected bool, got %s", u.fnName, name, val.Type())
		}
		*d = bool(b)

	case *[]string:
		list, ok := val.(*starlark.List)
		if !ok {
			return fmt.Errorf("%s: %s: expected list, got %s", u.fnName, name, val.Type())
		}
		result := make([]string, list.Len())
		for i := 0; i < list.Len(); i++ {
			s, ok := list.Index(i).(starlark.String)
			if !ok {
				return fmt.Errorf("%s: %s[%d]: expected string, got %s", u.fnName, name, i, list.Index(i).Type())
			}
			result[i] = string(s)
		}
		*d = result

	case *map[string]string:
		dict, ok := val.(*starlark.Dict)
		if !ok {
			return fmt.Errorf("%s: %s: expected dict, got %s", u.fnName, name, val.Type())
		}
		result := make(map[string]string)
		for _, item := range dict.Items() {
			k, ok := item[0].(starlark.String)
			if !ok {
				return fmt.Errorf("%s: %s: dict key must be string, got %s", u.fnName, name, item[0].Type())
			}
			v, ok := item[1].(starlark.String)
			if !ok {
				return fmt.Errorf("%s: %s[%q]: expected string value, got %s", u.fnName, name, k, item[1].Type())
			}
			result[string(k)] = string(v)
		}
		*d = result

	case *starlark.Value:
		*d = val

	default:
		return fmt.Errorf("%s: %s: unsupported destination type %T", u.fnName, name, dest)
	}

	return nil
}

// setDefault sets the default value on the destination.
func (u *Unpacker) setDefault(dest any, defaultVal any) error {
	switch d := dest.(type) {
	case *string:
		if defaultVal == nil {
			*d = ""
		} else if s, ok := defaultVal.(string); ok {
			*d = s
		} else {
			return fmt.Errorf("default value type mismatch: expected string")
		}

	case *int:
		if defaultVal == nil {
			*d = 0
		} else if i, ok := defaultVal.(int); ok {
			*d = i
		} else {
			return fmt.Errorf("default value type mismatch: expected int")
		}

	case *int64:
		if defaultVal == nil {
			*d = 0
		} else {
			switch v := defaultVal.(type) {
			case int64:
				*d = v
			case int:
				*d = int64(v)
			default:
				return fmt.Errorf("default value type mismatch: expected int64")
			}
		}

	case *bool:
		if defaultVal == nil {
			*d = false
		} else if b, ok := defaultVal.(bool); ok {
			*d = b
		} else {
			return fmt.Errorf("default value type mismatch: expected bool")
		}

	case *[]string:
		if defaultVal == nil {
			*d = nil
		} else if s, ok := defaultVal.([]string); ok {
			*d = s
		} else {
			return fmt.Errorf("default value type mismatch: expected []string")
		}

	case *map[string]string:
		if defaultVal == nil {
			*d = nil
		} else if m, ok := defaultVal.(map[string]string); ok {
			*d = m
		} else {
			return fmt.Errorf("default value type mismatch: expected map[string]string")
		}

	case *starlark.Value:
		if defaultVal == nil {
			*d = starlark.None
		} else if v, ok := defaultVal.(starlark.Value); ok {
			*d = v
		} else {
			return fmt.Errorf("default value type mismatch: expected starlark.Value")
		}

	default:
		return fmt.Errorf("unsupported destination type %T", dest)
	}

	return nil
}

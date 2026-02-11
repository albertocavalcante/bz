package value

import (
	"fmt"
	"reflect"
	"strings"

	"go.starlark.net/starlark"
)

// Func represents a wrapped Go function as a Starlark builtin.
// It uses struct tags to parse arguments automatically.
type Func struct {
	name    string
	fn      any
	argType reflect.Type
}

// WrapFunc wraps a Go function as a Starlark builtin.
// The Go function signature should be:
//
//	func(thread *starlark.Thread, args ArgsStruct) (starlark.Value, error)
//
// Where ArgsStruct is a struct with tagged fields for argument parsing:
//
//	type ArgsStruct struct {
//	    URL     string         `starlark:"url,required"`
//	    Timeout int            `starlark:"timeout"`
//	    Headers map[string]string `starlark:"headers"`
//	}
//
// Tag format: `starlark:"name[,required]"`
// - name: the argument name in Starlark
// - required: if present, the argument is required
//
// Supported field types:
//   - string
//   - int, int64
//   - bool
//   - []string
//   - map[string]string
//   - starlark.Value (for any Starlark value)
func WrapFunc(name string, fn any) *starlark.Builtin {
	wrapper := &Func{
		name: name,
		fn:   fn,
	}

	// Validate function signature
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("WrapFunc: %s: expected function, got %T", name, fn))
	}
	if fnType.NumIn() != 2 {
		panic(fmt.Sprintf("WrapFunc: %s: function must have 2 parameters (thread, args)", name))
	}
	if fnType.NumOut() != 2 {
		panic(fmt.Sprintf("WrapFunc: %s: function must have 2 return values (Value, error)", name))
	}

	// Check first parameter is *starlark.Thread
	threadType := reflect.TypeFor[*starlark.Thread]()
	if fnType.In(0) != threadType {
		panic(fmt.Sprintf("WrapFunc: %s: first parameter must be *starlark.Thread", name))
	}

	// Check second parameter is a struct
	argType := fnType.In(1)
	if argType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("WrapFunc: %s: second parameter must be a struct", name))
	}
	wrapper.argType = argType

	// Check return types
	valueType := reflect.TypeFor[starlark.Value]()
	errorType := reflect.TypeFor[error]()
	if !fnType.Out(0).Implements(valueType) {
		panic(fmt.Sprintf("WrapFunc: %s: first return value must implement starlark.Value", name))
	}
	if !fnType.Out(1).Implements(errorType) {
		panic(fmt.Sprintf("WrapFunc: %s: second return value must implement error", name))
	}

	return starlark.NewBuiltin(name, wrapper.call)
}

func (f *Func) call(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Create a new instance of the args struct
	argsVal := reflect.New(f.argType).Elem()

	// Parse the struct tags and populate fields
	if err := f.parseArgs(argsVal, args, kwargs); err != nil {
		return nil, err
	}

	// Call the wrapped function
	fnVal := reflect.ValueOf(f.fn)
	results := fnVal.Call([]reflect.Value{
		reflect.ValueOf(thread),
		argsVal,
	})

	// Handle results
	retVal := results[0].Interface()
	retErr := results[1].Interface()

	if retErr != nil {
		return nil, retErr.(error)
	}

	return retVal.(starlark.Value), nil
}

// argInfo holds parsed tag information for a struct field.
type argInfo struct {
	name     string
	required bool
	field    reflect.StructField
	index    int
}

func (f *Func) parseArgs(argsVal reflect.Value, posArgs starlark.Tuple, kwargs []starlark.Tuple) error {
	// Build field info from struct tags
	var fields []argInfo
	for i := 0; i < f.argType.NumField(); i++ {
		field := f.argType.Field(i)
		tag := field.Tag.Get("starlark")
		if tag == "" || tag == "-" {
			continue
		}

		parts := strings.Split(tag, ",")
		info := argInfo{
			name:  parts[0],
			field: field,
			index: i,
		}

		for _, part := range parts[1:] {
			if part == "required" {
				info.required = true
			}
		}

		fields = append(fields, info)
	}

	// Track which args have been set
	usedKwargs := make(map[string]bool)
	posIndex := 0

	// Process each field
	for i := range fields {
		info := &fields[i]
		var val starlark.Value

		// Check positional args first
		if posIndex < len(posArgs) {
			val = posArgs[posIndex]
			posIndex++
		} else {
			// Check kwargs
			for _, kv := range kwargs {
				if string(kv[0].(starlark.String)) == info.name {
					val = kv[1]
					usedKwargs[info.name] = true
					break
				}
			}
		}

		if val == nil {
			if info.required {
				return fmt.Errorf("%s: missing required argument %q", f.name, info.name)
			}
			continue
		}

		// Set the field value
		fieldVal := argsVal.Field(info.index)
		if err := f.setField(fieldVal, *info, val); err != nil {
			return err
		}
	}

	// Check for unused positional args
	if posIndex < len(posArgs) {
		return fmt.Errorf("%s: unexpected positional argument", f.name)
	}

	// Check for unknown kwargs
	for _, kv := range kwargs {
		name := string(kv[0].(starlark.String))
		if !usedKwargs[name] {
			// Check if it's a known field
			known := false
			for i := range fields {
				if fields[i].name == name {
					known = true
					break
				}
			}
			if !known {
				return fmt.Errorf("%s: unexpected keyword argument %q", f.name, name)
			}
		}
	}

	return nil
}

func (f *Func) setField(fieldVal reflect.Value, info argInfo, val starlark.Value) error {
	switch fieldVal.Kind() {
	case reflect.String:
		s, ok := val.(starlark.String)
		if !ok {
			return fmt.Errorf("%s: %s: expected string, got %s", f.name, info.name, val.Type())
		}
		fieldVal.SetString(string(s))

	case reflect.Int, reflect.Int64:
		i, ok := val.(starlark.Int)
		if !ok {
			return fmt.Errorf("%s: %s: expected int, got %s", f.name, info.name, val.Type())
		}
		v, ok := i.Int64()
		if !ok {
			return fmt.Errorf("%s: %s: integer overflow", f.name, info.name)
		}
		fieldVal.SetInt(v)

	case reflect.Bool:
		b, ok := val.(starlark.Bool)
		if !ok {
			return fmt.Errorf("%s: %s: expected bool, got %s", f.name, info.name, val.Type())
		}
		fieldVal.SetBool(bool(b))

	case reflect.Slice:
		if fieldVal.Type().Elem().Kind() == reflect.String {
			list, ok := val.(*starlark.List)
			if !ok {
				return fmt.Errorf("%s: %s: expected list, got %s", f.name, info.name, val.Type())
			}
			result := make([]string, list.Len())
			for i := 0; i < list.Len(); i++ {
				s, ok := list.Index(i).(starlark.String)
				if !ok {
					return fmt.Errorf("%s: %s[%d]: expected string, got %s", f.name, info.name, i, list.Index(i).Type())
				}
				result[i] = string(s)
			}
			fieldVal.Set(reflect.ValueOf(result))
		} else {
			return fmt.Errorf("%s: %s: unsupported slice element type %s", f.name, info.name, fieldVal.Type().Elem())
		}

	case reflect.Map:
		if fieldVal.Type().Key().Kind() == reflect.String && fieldVal.Type().Elem().Kind() == reflect.String {
			dict, ok := val.(*starlark.Dict)
			if !ok {
				return fmt.Errorf("%s: %s: expected dict, got %s", f.name, info.name, val.Type())
			}
			result := make(map[string]string)
			for _, item := range dict.Items() {
				k, ok := item[0].(starlark.String)
				if !ok {
					return fmt.Errorf("%s: %s: dict key must be string, got %s", f.name, info.name, item[0].Type())
				}
				v, ok := item[1].(starlark.String)
				if !ok {
					return fmt.Errorf("%s: %s[%q]: expected string value, got %s", f.name, info.name, k, item[1].Type())
				}
				result[string(k)] = string(v)
			}
			fieldVal.Set(reflect.ValueOf(result))
		} else {
			return fmt.Errorf("%s: %s: unsupported map type %s", f.name, info.name, fieldVal.Type())
		}

	case reflect.Interface:
		// Check if it's starlark.Value
		if fieldVal.Type().Implements(reflect.TypeFor[starlark.Value]()) {
			fieldVal.Set(reflect.ValueOf(val))
		} else {
			return fmt.Errorf("%s: %s: unsupported interface type %s", f.name, info.name, fieldVal.Type())
		}

	default:
		return fmt.Errorf("%s: %s: unsupported field type %s", f.name, info.name, fieldVal.Type())
	}

	return nil
}

// BuiltinFunc is the signature for simple builtin functions that don't need
// the full WrapFunc machinery.
type BuiltinFunc func(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// NewBuiltin creates a simple Starlark builtin from a Go function.
// This is a convenience wrapper around starlark.NewBuiltin.
func NewBuiltin(name string, fn BuiltinFunc) *starlark.Builtin {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return fn(thread, args, kwargs)
	})
}

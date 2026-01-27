package builtins

import "go.starlark.net/starlark"

// All returns all common builtins as a StringDict.
// These can be added to an interpreter via WithBuiltin or merged into globals.
func All() starlark.StringDict {
	return starlark.StringDict{
		"glob":  Glob,
		"env":   Env,
		"print": Print,
		"fail":  Fail,
	}
}

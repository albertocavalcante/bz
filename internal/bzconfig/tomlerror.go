package bzconfig

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// describeTOMLError says where and why a file failed to decode, using only
// fixed text: the decoder's own message quotes keys and values, and a
// project file may be a symlink to another of the user's files.
func describeTOMLError(err error, data []byte) string {
	var perr toml.ParseError
	if errors.As(err, &perr) {
		// The byte offset, not the decoder's line, which can point past
		// the offending character.
		start := min(max(perr.Position.Start, 0), len(data))
		line := bytes.Count(data[:start], []byte("\n")) + 1
		col := utf8.RuneCount(data[bytes.LastIndexByte(data[:start], '\n')+1:start]) + 1
		return fmt.Sprintf("invalid TOML at line %d, column %d: %s", line, col, syntaxReason(perr.Message))
	}
	// Type mismatches are plain errors that start with
	// `toml: line N (last key "k"): ` (BurntSushi/toml decode.go, MetaData.e);
	// only the line digits and a key that names a schema field are used.
	m := typeErrorPrefix.FindStringSubmatch(err.Error())
	if m == nil {
		return "invalid TOML: a value has the wrong type"
	}
	where := "line " + m[1]
	if key, uerr := strconv.Unquote(m[2]); uerr == nil {
		if want, ok := schemaWant(reflect.TypeFor[fileSchema](), strings.Split(key, ".")); ok {
			// key matched a schema field, so it is the schema's text.
			return fmt.Sprintf("%s: %s must be %s", where, key, want)
		}
	}
	return where + ": a value has the wrong type"
}

var typeErrorPrefix = regexp.MustCompile(`^toml: line (\d+) \(last key ("(?:[^"\\]|\\.)*")\): `)

// schemaWant returns the want tag of the field that path names in t.
func schemaWant(t reflect.Type, path []string) (string, bool) {
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Tag.Get("toml") != path[0] {
			continue
		}
		if len(path) == 1 {
			return f.Tag.Get("want"), true
		}
		if f.Type.Kind() == reflect.Struct {
			return schemaWant(f.Type, path[1:])
		}
	}
	return "", false
}

// syntaxReasons maps fragments of the decoder's syntax messages
// (BurntSushi/toml lex.go, parse.go, error.go) to fixed text; the first
// match wins.
var syntaxReasons = []struct{ fragment, reason string }{
	{"strings cannot contain newlines", "unterminated string"},
	{"UTF-8", "invalid UTF-8"},
	{"control characters", "control character"},
	{"invalid escape", "invalid escape in a string"},
	{"expected value", "expected a value"},
	{"unexpected EOF", "unexpected end of file"},
	{"end with a newline", "expected the end of the line after a value"},
	{"already been defined", "key or table defined twice"},
	{"already created", "key or table defined twice"},
	{"is not a table", "key or table defined twice"},
	{"table name", "invalid table name"},
	{"table array name", "invalid table name"},
	{"inline table", "invalid inline table"},
	{"comma", "invalid array or inline table"},
	{"out of", "number out of range"},
	{"datetime", "invalid date or time"},
	{"float", "invalid number"},
	{"digit", "invalid number"},
	{"number", "invalid number"},
	{"key", "invalid key"},
	{"'='", "invalid key"},
	{"'.'", "invalid key"},
}

func syntaxReason(msg string) string {
	for _, r := range syntaxReasons {
		if strings.Contains(msg, r.fragment) {
			return r.reason
		}
	}
	return "syntax error"
}

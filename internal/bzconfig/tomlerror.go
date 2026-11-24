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

// syntaxReasons maps the fixed leading words of the decoder's syntax
// messages (BurntSushi/toml lex.go, parse.go, error.go) to fixed text.
// Only prefixes are matched: the rest of a message can quote the file.
var syntaxReasons = []struct{ prefix, reason string }{
	{"strings cannot contain newlines", "unterminated string"},
	{"invalid UTF-8", "invalid UTF-8"},
	{"files cannot contain NULL bytes", "NUL byte (TOML files must be UTF-8)"},
	{"TOML files cannot contain control characters", "control character"},
	{"invalid escape", "invalid escape in a string"},
	{"Escaped character", "invalid escape in a string"},
	{"unexpected EOF; expected value", "expected a value"},
	{"expected value", "expected a value"},
	{"unexpected EOF", "unexpected end of file"},
	{"expected a top-level item to end with a newline", "expected the end of the line after a value"},
	{"Key '", "key or table defined twice"},
	{"unexpected end of table name", "invalid table name"},
	{"unexpected table separator", "invalid table name"},
	{"expected end of table array name", "invalid table name"},
	{"expected '.' or ']'", "invalid table name"},
	{"newlines not allowed within inline tables", "invalid inline table"},
	{"expected a comma or an inline table terminator", "invalid inline table"},
	{"unexpected comma", "invalid array or inline table"},
	{"expected a comma (',') or array terminator", "invalid array"},
	{"invalid datetime", "invalid date or time"},
	{"invalid float", "invalid number"},
	{"Invalid float", "invalid number"},
	{"Invalid integer", "invalid number"},
	{"cannot use sign", "invalid number"},
	{"floats must start", "invalid number"},
	{"expected a digit", "invalid number"},
	{"not a binary", "invalid number"},
	{"not an octal", "invalid number"},
	{"not a hexadecimal", "invalid number"},
	{"unexpected '='", "invalid key"},
	{"unexpected '.'", "invalid key"},
	{"expected '.' or '='", "invalid key"},
}

// outOfRange matches the one message that starts with a value, a number.
var outOfRange = regexp.MustCompile(`^[-+0-9.eE]+ is out of (range|the safe)`)

func syntaxReason(msg string) string {
	for _, r := range syntaxReasons {
		if strings.HasPrefix(msg, r.prefix) {
			return r.reason
		}
	}
	if outOfRange.MatchString(msg) {
		return "number out of range"
	}
	return "syntax error"
}

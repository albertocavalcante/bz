package bzconfig

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFile_ErrorMessages(t *testing.T) {
	const prefix = "parsing project config /p/.bz.toml: "
	for _, tc := range []struct{ name, data, want string }{
		{"timeout not a string", "[network]\ntimeout = 30\n", `line 2: network.timeout must be a string such as "30s"`},
		{"mode not a string", "[network]\nmode = 1\n", `line 2: network.mode must be a string such as "online"`},
		{"disabled not an array", "[commands]\ndisabled = \"audit\"\n", `line 2: commands.disabled must be an array of strings such as ["audit"]`},
		{"array of tables", "[[network]]\n", "line 1: network must be a table such as [network]"},
		{"network not a table", "network = 5\n", "line 1: network must be a table such as [network]"},
		{"array element", "[commands]\n\ndisabled = [1]\n", `line 3: commands.disabled must be an array of strings such as ["audit"]`},
		{"dotted key", "network.timeout = 30\n", `line 1: network.timeout must be a string such as "30s"`},
		{"missing value", "[network]\ntimeout =\n", "invalid TOML at line 2, column 10: expected a value"},
		{"unterminated string", "[network]\nregistry = \"https://x\n", "invalid TOML at line 2, column 22: unterminated string"},
		{"invalid UTF-8", "[network]\nmode = \"\xff\"\n", "invalid TOML at line 2, column 8: invalid UTF-8"},
		{"trailing text", "[network]\nmode = \"online\" x\n", "invalid TOML at line 2, column 16: expected the end of the line after a value"},
		{"unclosed table", "[network\n", "invalid TOML at line 1, column 9: invalid table name"},
		{"defined twice", "[network]\nmode = \"online\"\nmode = \"offline\"\n", "invalid TOML at line 3, column 1: key or table defined twice"},
		{"duplicate key named UTF-8", "\"UTF-8\" = 1\n\"UTF-8\" = 2\n", "invalid TOML at line 2, column 2: key or table defined twice"},
		{"duplicate key named after a reason", "\"strings cannot contain newlines\" = 1\n\"strings cannot contain newlines\" = 2\n", "invalid TOML at line 2, column 2: key or table defined twice"},
		{"NUL at the start", "\x00[network]\n", "invalid TOML at line 1, column 1: NUL byte (TOML files must be UTF-8)"},
		{"key reused as a table", "[network]\nmode = 1\n[network.mode]\n", "invalid TOML at line 3, column 2: key or table defined twice"},
		{"dotted key below a value, named like a reason", "\"Key '\" = 1\n[\"Key '\".x]\n", "invalid TOML at line 2, column 3: key or table defined twice"},
		{"number out of range", "x = 99999999999999999999\n", "invalid TOML at line 1, column 5: number out of range"},
		{"column counts characters", "[network]\nmode = \"é€\" x\n", "invalid TOML at line 2, column 12: expected the end of the line after a value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseFile(LayerProject, "/p/.bz.toml", []byte(tc.data), "")
			require.EqualError(t, err, prefix+tc.want)
		})
	}
}

// TestParseErrorsNeverEchoContent: a project file can be a symlink to
// another of the user's files, so no branch may quote the file's keys or
// values.
func TestParseErrorsNeverEchoContent(t *testing.T) {
	for _, data := range []string{
		"[network]\nmode = \"online\"\napi_token = \"hunter2-secret\" trailing\n",
		"hunter2_secret_key = \n",
		"[hunter2_secret_table\n",
		"hunter2_secret_key = 1\nhunter2_secret_key = 2\n",
		"[network]\ntimeout = 1979-05-27Thunter2\n",
		"[network]\ntimeout = \"hunter2-secret\"\ntimeout = 30\n",
		"[network]\nhunter2_secret_key = 1\ntimeout = 30\n",
		"[network]\nmode = [\"hunter2-secret\"]\n",
		"network = \"hunter2-secret\"\n",
		"[commands]\ndisabled = [\"audit\", { hunter2_secret_key = 1 }]\n",
	} {
		_, _, err := ParseFile(LayerProject, "/p/.bz.toml", []byte(data), "")
		require.Error(t, err, data)
		assert.NotContains(t, err.Error(), "hunter2", data)
	}

	// Fallbacks for decoder messages without a schema key or a line.
	assert.Equal(t, "line 4: a value has the wrong type",
		describeTOMLError(errors.New(`toml: line 4 (last key "hunter2_secret_key"): expected table`), nil))
	assert.Equal(t, "invalid TOML: a value has the wrong type",
		describeTOMLError(errors.New(`toml: (last key "hunter2_secret_key"): expected table`), nil))
}

// TestSeveralErrorsAreProblems: the CLI lists Problems item by item.
func TestSeveralErrorsAreProblems(t *testing.T) {
	m, timeout := NetworkMode("bogus"), "30"
	_, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p/.bz.toml", Settings: Settings{Mode: &m, Timeout: &timeout}})
	var p Problems
	require.ErrorAs(t, err, &p)
	assert.Len(t, p, 2)
	assert.Equal(t, errors.Join(p...).Error(), err.Error())

	vars := map[string]string{EnvOffline: "ture", EnvPreferOffline: "nah"}
	_, err = EnvLayer(func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, "/home/u")
	require.ErrorAs(t, err, &p)
	assert.Len(t, p, 2)
}

// TestSyntaxReason_OutOfRange: only a message that starts with a number
// is a number out of range; anything else before it is file content.
func TestSyntaxReason_OutOfRange(t *testing.T) {
	for msg, want := range map[string]string{
		"99999999999999999999 is out of range for int64": "number out of range",
		"1e999 is out of the safe floating-point range":  "number out of range",
		"-5.5E+3 is out of range for uint8":              "number out of range",
		"a1 is out of range for int64":                   "syntax error",
		`"x" is out of range for int64`:                  "syntax error",
	} {
		assert.Equal(t, want, syntaxReason(msg), msg)
	}
}

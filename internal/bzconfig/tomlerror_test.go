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
		{"column counts characters", "[network]\nmode = \"é€\" x\n", "invalid TOML at line 2, column 12: expected the end of the line after a value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile(LayerProject, "/p/.bz.toml", []byte(tc.data), "")
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
		_, err := ParseFile(LayerProject, "/p/.bz.toml", []byte(data), "")
		require.Error(t, err, data)
		assert.NotContains(t, err.Error(), "hunter2", data)
	}

	// Fallbacks for decoder messages without a schema key or a line.
	assert.Equal(t, "line 4: a value has the wrong type",
		describeTOMLError(errors.New(`toml: line 4 (last key "hunter2_secret_key"): expected table`), nil))
	assert.Equal(t, "invalid TOML: a value has the wrong type",
		describeTOMLError(errors.New(`toml: (last key "hunter2_secret_key"): expected table`), nil))
}

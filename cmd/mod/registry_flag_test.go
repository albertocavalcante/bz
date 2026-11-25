package mod

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/registry"
	"github.com/albertocavalcante/bz/internal/testutil"
)

const flagSecret = "S3CR3T"

func TestModCmdPersistentPreRunWhenStandalone(t *testing.T) {
	oldRegistry := registryFlag
	registryFlag = ""
	t.Cleanup(func() { registryFlag = oldRegistry })

	require.NoError(t, Cmd.PersistentPreRunE(Cmd, nil))
}

func TestModCmdPersistentPreRunDelegatesToParent(t *testing.T) {
	oldRegistry := registryFlag
	registryFlag = ""
	t.Cleanup(func() { registryFlag = oldRegistry })

	called := 0
	root := &cobra.Command{
		Use: "bz",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			called++
			return nil
		},
	}
	root.AddCommand(Cmd)
	t.Cleanup(func() { root.RemoveCommand(Cmd) })

	require.NoError(t, Cmd.PersistentPreRunE(Cmd, nil))
	assert.Equal(t, 1, called)
}

func TestModInfo_RegistryFlagNeverPrintsCredentials(t *testing.T) {
	const help = "\nhelp: want https://host[:port]/path, file:///abs/path or an absolute path"
	const prefix = "invalid registry in flag --registry: "

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{
			name:    "credentialed URL that is unreachable",
			value:   "https://user:" + flagSecret + "@registry.invalid/",
			wantErr: "You are in offline mode but 'rules_go' is not cached.: https://xxxxx@registry.invalid: module not found",
		},
		{
			name:    "unparsable credentialed value",
			value:   "https://user:" + flagSecret + "@",
			wantErr: prefix + `"https://<redacted>": missing host` + help,
		},
		{
			name:    "plain bad value",
			value:   "bad",
			wantErr: prefix + `"bad": want an https://, http:// or file:// URL or an absolute path`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli.ResetCachedConfig()
			t.Cleanup(cli.ResetCachedConfig)
			t.Chdir(t.TempDir())
			t.Setenv("BZ_OFFLINE", "1")
			t.Cleanup(func() { registryFlag = "" })

			root := &cobra.Command{Use: "bz"}
			root.AddCommand(Cmd)
			t.Cleanup(func() { root.RemoveCommand(Cmd) })
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{"mod", "info", "rules_go", "--registry", tt.value})

			err := root.Execute()
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())

			shown := out.String() + err.Error()
			var netErr *registry.NetworkError
			if errors.As(err, &netErr) {
				shown += netErr.UserFriendlyError()
				assert.Contains(t, shown, "Cannot reach registry (https://xxxxx@registry.invalid)")
			}
			assert.NotContains(t, shown, flagSecret)
		})
	}
}

func TestModInfo_NormalizesRegistryFlagBeforeUse(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})
	server := httptest.NewServer(http.FileServer(http.Dir(registryDir)))
	t.Cleanup(server.Close)

	for _, registryURL := range []string{
		registryDir,
		"file://" + registryDir,
		server.URL,
	} {
		t.Run(registryURL, func(t *testing.T) {
			cli.ResetCachedConfig()
			t.Cleanup(cli.ResetCachedConfig)
			t.Chdir(t.TempDir())
			oldRegistry := registryFlag
			registryFlag = " \t" + registryURL + "\n"
			t.Cleanup(func() { registryFlag = oldRegistry })

			infoJSON = true
			t.Cleanup(func() { infoJSON = false })
			var out bytes.Buffer
			infoCmd.SetOut(&out)

			require.NoError(t, runInfo(infoCmd, []string{"rules_go"}))
			assert.Contains(t, out.String(), `"name"`)
			assert.Equal(t, " \t"+registryURL+"\n", registryFlag)
		})
	}
}

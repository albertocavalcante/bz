package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for bz.

To load completions:

Bash:
  # Linux
  $ bz completion bash > /etc/bash_completion.d/bz

  # macOS
  $ bz completion bash > $(brew --prefix)/etc/bash_completion.d/bz

Zsh:
  # If shell completion is not already enabled, enable it:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Add to fpath and source:
  $ bz completion zsh > "${fpath[1]}/_bz"

Fish:
  $ bz completion fish > ~/.config/fish/completions/bz.fish

PowerShell:
  PS> bz completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add to your profile:
  PS> bz completion powershell >> $PROFILE
`,
	Example: `  bz completion bash
  bz completion zsh
  bz completion fish
  bz completion powershell`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		switch args[0] {
		case "bash":
			err = rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			err = rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			err = rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			err = rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("invalid shell: %s (valid shells: bash, zsh, fish, powershell)", args[0])
		}
		return err
	},
}

var _ = onLoad(func() {
	rootCmd.AddCommand(completionCmd)
})

// GetRootCmd returns the root command for completion generation.
// This is exported for use by subpackages that need to set up completions.
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// SetupCompletionOptions configures global completion behavior.
var _ = onLoad(func() {
	// Enable descriptions in completions where supported
	rootCmd.CompletionOptions.DisableDefaultCmd = false

	// Set output to stderr by default (some shells expect this)
	completionCmd.SetOut(os.Stdout)
})

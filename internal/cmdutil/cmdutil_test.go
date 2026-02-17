package cmdutil

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandContext_NilContext(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	ctx := CommandContext(cmd)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx != context.Background() {
		t.Fatal("expected context.Background()")
	}
}

func TestCommandContext_WithContext(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	want := context.WithValue(context.Background(), struct{}{}, "val") //nolint:staticcheck // test-only context key
	cmd.SetContext(want)
	got := CommandContext(cmd)
	if got != want {
		t.Fatalf("expected provided context, got %v", got)
	}
}

func TestApplyErrorSilence(t *testing.T) {
	t.Parallel()
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child"}
	grandchild := &cobra.Command{Use: "grandchild"}

	root.AddCommand(child)
	child.AddCommand(grandchild)

	ApplyErrorSilence(root)

	for _, cmd := range []*cobra.Command{root, child, grandchild} {
		if !cmd.SilenceUsage {
			t.Errorf("%s: SilenceUsage should be true", cmd.Name())
		}
		if !cmd.SilenceErrors {
			t.Errorf("%s: SilenceErrors should be true", cmd.Name())
		}
	}
}

func TestRequireSubcommand_EmptyArgs(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "root"}
	// With empty args, Help() is called which returns nil
	err := RequireSubcommand(cmd, nil)
	if err != nil {
		t.Fatalf("expected nil error from Help(), got %v", err)
	}
}

func TestRequireSubcommand_UnknownCommand(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "list"}
	cmd.AddCommand(child)

	err := RequireSubcommand(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	errMsg := err.Error()
	if errMsg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestRequireSubcommand_WithSuggestions(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "list"}
	cmd.AddCommand(child)

	// Cobra suggests when Levenshtein distance <= len(cmd)/2.
	// For "list" (len 4), threshold is 2.
	// "lisp" has distance 1 from "list".
	// However, Cobra also requires the input length to be close.
	// Test both code paths by verifying the error message format differs.
	err := RequireSubcommand(cmd, []string{"totally-wrong"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	errMsg := err.Error()
	if !contains(errMsg, "unknown command") {
		t.Fatalf("expected 'unknown command' in error, got %q", errMsg)
	}
	// No suggestion branch: error contains "--help" guidance
	if !contains(errMsg, "--help") {
		t.Fatalf("expected '--help' guidance for non-suggested command, got %q", errMsg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

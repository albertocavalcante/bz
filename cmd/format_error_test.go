package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/albertocavalcante/bz/internal/bzconfig"
)

func TestPrintError_Lists(t *testing.T) {
	a, b := errors.New("first"), errors.New("second\nline")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"single", a, "Error: first\n"},
		{"problems", bzconfig.Problems{a, b}, "Error: 2 problems:\n  - first\n  - second\n    line\n"},
		{"wrapped problems", fmt.Errorf("loading config: %w", bzconfig.Problems{a, b}), "Error: loading config: 2 problems:\n  - first\n  - second\n    line\n"},
		{"twice wrapped problems", fmt.Errorf("x: %w", fmt.Errorf("y: %w", bzconfig.Problems{a, b})), "Error: x: y: 2 problems:\n  - first\n  - second\n    line\n"},
		{"one problem", bzconfig.Problems{a}, "Error: first\n"},
		{"text after the list", fmt.Errorf("%w (while loading)", bzconfig.Problems{a, b}), "Error: first\nsecond\nline (while loading)\n"},
		{"several %w", fmt.Errorf("%w (%w)", a, b), "Error: first (second\nline)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			printError(&out, tc.err)
			assert.Equal(t, tc.want, out.String())
		})
	}
}

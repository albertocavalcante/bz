package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintError_Lists(t *testing.T) {
	a, b := errors.New("first"), errors.New("second\nline")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"single", a, "Error: first\n"},
		{"join", errors.Join(a, b), "Error: 2 problems:\n  - first\n  - second\n    line\n"},
		{"wrapped join", fmt.Errorf("loading config: %w", errors.Join(a, b)), "Error: loading config: 2 problems:\n  - first\n  - second\n    line\n"},
		{"twice wrapped join", fmt.Errorf("x: %w", fmt.Errorf("y: %w", errors.Join(a, b))), "Error: x: y: 2 problems:\n  - first\n  - second\n    line\n"},
		{"join of one", errors.Join(a), "Error: first\n"},
		{"several %w", fmt.Errorf("%w (%w)", a, b), "Error: first (second\nline)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			printError(&out, tc.err)
			assert.Equal(t, tc.want, out.String())
		})
	}
}

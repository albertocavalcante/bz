package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithSpinner_ReturnsNilOnSuccess(t *testing.T) {
	// Spinner should complete without error when function succeeds
	err := WithSpinner("Testing...", func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestWithSpinner_ReturnsErrorOnFailure(t *testing.T) {
	// Spinner should return the function's error
	expectedErr := errors.New("test error")
	err := WithSpinner("Testing...", func() error {
		return expectedErr
	})
	assert.Equal(t, expectedErr, err)
}

func TestWithSpinner_ExecutesFunction(t *testing.T) {
	// Verify the function is actually called
	called := false
	err := WithSpinner("Testing...", func() error {
		called = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, called, "function should have been called")
}

func TestWithSpinner_WorksWithLongOperations(t *testing.T) {
	// Test with a function that takes some time
	start := time.Now()
	err := WithSpinner("Waiting...", func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestWithSpinnerWriter_WritesToProvidedWriter(t *testing.T) {
	var buf bytes.Buffer

	err := WithSpinnerWriter(&buf, "Processing...", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	assert.NoError(t, err)
	// In quiet mode or non-TTY, nothing might be written
	// The important thing is that it doesn't panic
}

func TestWithSpinner_RespectsQuietMode(t *testing.T) {
	// Set quiet mode
	Global = Options{Quiet: true}
	defer func() { Global = Options{} }()

	var buf bytes.Buffer
	err := WithSpinnerWriter(&buf, "Should not show...", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	assert.NoError(t, err)
	// In quiet mode, spinner should not output anything
	assert.Empty(t, buf.String())
}

func TestWithSpinner_NoColorMode(t *testing.T) {
	// Set no-color mode
	Global = Options{NoColor: true}
	defer func() { Global = Options{} }()

	ConfigureLipgloss()

	err := WithSpinner("Testing...", func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestWithSpinner_NoColorEnv(t *testing.T) {
	// Set NO_COLOR env
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	ConfigureLipgloss()

	err := WithSpinner("Testing...", func() error {
		return nil
	})
	assert.NoError(t, err)
}

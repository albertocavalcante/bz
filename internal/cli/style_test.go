package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStyleSuccess_ReturnsGreenText(t *testing.T) {
	// Reset global state for clean test
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")

	// Call ConfigureLipgloss to set up color profile
	ConfigureLipgloss()

	result := Success("test")
	// We can't test for exact ANSI codes as they depend on terminal
	// but we can verify the function doesn't panic and returns something
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "test")
}

func TestStyleWarning_ReturnsYellowText(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := Warning("warning message")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "warning message")
}

func TestStyleError_ReturnsRedText(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := Error("error message")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "error message")
}

func TestStyleInfo_ReturnsBlueText(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := Info("info message")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "info message")
}

func TestStyleMuted_ReturnsGrayText(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := Muted("muted message")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "muted message")
}

func TestStyles_RespectNoColorFlag(t *testing.T) {
	// Set no-color flag
	Global = Options{NoColor: true}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	// When colors are disabled, output should be plain text
	assert.Equal(t, "test", Success("test"))
	assert.Equal(t, "test", Warning("test"))
	assert.Equal(t, "test", Error("test"))
	assert.Equal(t, "test", Info("test"))
	assert.Equal(t, "test", Muted("test"))
}

func TestStyles_RespectNoColorEnv(t *testing.T) {
	// Set NO_COLOR env var
	Global = Options{NoColor: false}
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	// When colors are disabled via env, output should be plain text
	assert.Equal(t, "test", Success("test"))
	assert.Equal(t, "test", Warning("test"))
	assert.Equal(t, "test", Error("test"))
	assert.Equal(t, "test", Info("test"))
	assert.Equal(t, "test", Muted("test"))
}

func TestStyleBold_ReturnsBoldText(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := Bold("bold text")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "bold text")
}

func TestStyleBold_RespectsNoColor(t *testing.T) {
	Global = Options{NoColor: true}
	ConfigureLipgloss()

	assert.Equal(t, "bold text", Bold("bold text"))
}

func TestSuccessIcon_ReturnsGreenCheckmark(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := SuccessIcon()
	assert.Contains(t, result, "\u2713") // Checkmark
}

func TestErrorIcon_ReturnsRedX(t *testing.T) {
	Global = Options{NoColor: false}
	os.Unsetenv("NO_COLOR")
	ConfigureLipgloss()

	result := ErrorIcon()
	assert.Contains(t, result, "\u2717") // X mark
}

func TestSuccessIcon_NoColorReturnsPlainCheckmark(t *testing.T) {
	Global = Options{NoColor: true}
	ConfigureLipgloss()

	assert.Equal(t, "\u2713", SuccessIcon())
}

func TestErrorIcon_NoColorReturnsPlainX(t *testing.T) {
	Global = Options{NoColor: true}
	ConfigureLipgloss()

	assert.Equal(t, "\u2717", ErrorIcon())
}

package cmdutil

import (
	"errors"
	"fmt"
	"testing"
)

func TestAlreadyReported(t *testing.T) {
	base := errors.New("2 issues found")
	err := AlreadyReported(base)

	if !IsAlreadyReported(err) {
		t.Error("IsAlreadyReported(err) = false")
	}
	if !IsAlreadyReported(fmt.Errorf("wrapped: %w", err)) {
		t.Error("IsAlreadyReported(wrapped) = false")
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is(err, base) = false")
	}
	if err.Error() != base.Error() {
		t.Errorf("Error() = %q, want %q", err.Error(), base.Error())
	}
	if IsAlreadyReported(base) {
		t.Error("plain error reported as already reported")
	}
	if AlreadyReported(nil) != nil {
		t.Error("AlreadyReported(nil) should be nil")
	}
}

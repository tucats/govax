package vmserrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestInhibitMessage(t *testing.T) {
	base := errors.New("RENAME-E-OPENOUT, error opening X as output")

	err := InhibitMessage(base)
	if !MessageInhibited(err) {
		t.Error("an inhibited error isn't reported as inhibited")
	}

	if !MessageInhibited(fmt.Errorf("wrapped: %w", err)) {
		t.Error("wrapping an inhibited error loses the mark")
	}

	if err.Error() != base.Error() || !errors.Is(err, base) {
		t.Errorf("the inhibited error doesn't keep the original: %v", err)
	}

	if MessageInhibited(base) || InhibitMessage(nil) != nil {
		t.Error("an ordinary error is inhibited, or nil isn't kept nil")
	}
}

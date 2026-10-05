package corevms

import "testing"

// TestStatusText: a status value's message line, with the
// STS$M_INHIB_MSG bit ignored (an image that exits after the catch-all
// showed its condition's message has the bit set).
func TestStatusText(t *testing.T) {
	env, _ := fixture()

	const normal = 1

	want := "%SYSTEM-S-NORMAL, normal successful completion"
	if got := env.StatusText(normal); got != want {
		t.Errorf("StatusText(1) = %q, want %q", got, want)
	}

	if got := env.StatusText(normal | stsInhibitMsg); got != want {
		t.Errorf("StatusText(1 with the inhibit bit) = %q, want %q", got, want)
	}
}

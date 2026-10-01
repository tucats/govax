package obj

import (
	"testing"
	"time"
)

// TestFormatTime: a day before the 10th is padded with a blank, as VMS
// writes it (" 1-OCT-2026", from real MACRO's objects), not a zero.
func TestFormatTime(t *testing.T) {
	for when, want := range map[time.Time]string{
		time.Date(2026, time.October, 1, 5, 53, 0, 0, time.UTC):    " 1-OCT-2026 05:53",
		time.Date(2026, time.September, 30, 21, 50, 0, 0, time.UTC): "30-SEP-2026 21:50",
	} {
		if got := FormatTime(when); got != want {
			t.Errorf("FormatTime(%v) = %q, want %q", when, got, want)
		}
	}
}

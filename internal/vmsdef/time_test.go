package vmsdef

import (
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	if got := Time(time.Unix(0, 0)); got != 0x007C95674BEB4000 {
		t.Errorf("Time(Unix epoch) = %#x, want 0x007C95674BEB4000", got)
	}

	if got := Time(time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)); got != 0 {
		t.Errorf("Time(17-Nov-1858) = %#x, want 0", got)
	}
}

package vmsdef

import (
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	if got := Time(time.Unix(0, 0).UTC()); got != 0x007C95674BEB4000 {
		t.Errorf("Time(Unix epoch) = %#x, want 0x007C95674BEB4000", got)
	}

	if got := Time(time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)); got != 0 {
		t.Errorf("Time(17-Nov-1858) = %#x, want 0", got)
	}
}

// TestTimeLocal: VMS time is local wall-clock time, so the same instant
// converts to a reading shifted by the zone's offset.
func TestTimeLocal(t *testing.T) {
	zone := time.FixedZone("EST", -5*3600)
	instant := time.Date(1988, time.December, 30, 9, 15, 28, 0, time.UTC)

	utc := Time(instant)
	local := Time(instant.In(zone))

	if utc-local != 5*3600*TicksPerSecond {
		t.Errorf("UTC reading - EST reading = %d, want 5 hours (%d)", utc-local, 5*3600*TicksPerSecond)
	}

	// GoTime gives back the local reading's fields.
	if got := GoTime(local); got.Hour() != 4 || got.Minute() != 15 || got.Day() != 30 {
		t.Errorf("GoTime(EST reading) = %v, want 30-Dec-1988 04:15", got)
	}
}

func TestGoTimeRoundTrip(t *testing.T) {
	for _, want := range []time.Time{
		time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC),
		time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.September, 28, 13, 45, 1, 120_000_000, time.UTC),
	} {
		if got := GoTime(Time(want)); !got.Equal(want) {
			t.Errorf("GoTime(Time(%v)) = %v", want, got)
		}
	}
}

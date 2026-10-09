package rms

import (
	"strings"
	"testing"
)

// TestIOCount_model: RMS's I/O counts (iocount.go) give VMS 7.3's totals
// for the probe's two cases (testdata/probe49, step 8): $CREATE, ten
// 80-byte $PUTs, and $CLOSE, 3 buffered and 7 direct; $OPEN, $GETs to
// the end, and $CLOSE, 2 and 1.
func TestIOCount_model(t *testing.T) {
	p, _ := newSharers(t, 1)
	a := p[0]

	var buffered, direct uint32

	a.ctx.CountIO = func(b, d uint32) { buffered, direct = buffered+b, direct+d }

	want := func(what string, b, d uint32) {
		t.Helper()

		if buffered != b || direct != d {
			t.Errorf("%s: buffered %d, direct %d; want %d, %d", what, buffered, direct, b, d)
		}

		buffered, direct = 0, 0
	}

	a.mustOpen("IO.DAT", facPut, 0, true, 0)

	for range 10 {
		a.put(strings.Repeat("X", 80))
	}

	a.close()
	want("$CREATE, ten $PUTs, $CLOSE", 3, 7)

	a.mustOpen("IO.DAT", facGet, 0, false, 0)

	for {
		if _, sts := a.get(); sts&1 == 0 {
			break
		}
	}

	a.close()
	want("$OPEN, $GETs, $CLOSE", 2, 1)
}

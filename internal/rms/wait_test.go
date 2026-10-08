package rms

import "testing"

// TestSysWait checks that SYS$WAIT returns the RAB's completion status,
// whatever the last operation stored there: the RMS$_NORMAL a $CONNECT
// left, then an end of file put there by hand.
func TestSysWait(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	putRAB(t, f.ctx, testFabAddr)

	if _, err := SysConnect(f.ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	r0, err := SysWait(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysWait: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("after $CONNECT, $WAIT = %08X, want RMS$_NORMAL (%08X)", r0, rmsNormal)
	}

	putLongwordAt(t, f.ctx, testRabAddr+rabSTS, rmsEOF)

	if r0, err = SysWait(f.ctx, []uint32{testRabAddr}); err != nil {
		t.Fatalf("SysWait: %v", err)
	} else if r0 != rmsEOF {
		t.Errorf("$WAIT = %08X, want RAB$L_STS, RMS$_EOF (%08X)", r0, rmsEOF)
	}
}

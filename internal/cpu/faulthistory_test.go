package cpu

import "testing"

func TestFaultHistoryDefaultSize(t *testing.T) {
	e := newEngine()
	if got := e.FaultHistorySize(); got != defaultFaultHistory {
		t.Errorf("FaultHistorySize() = %d, want %d", got, defaultFaultHistory)
	}

	if hist := e.FaultHistory(); hist != nil {
		t.Errorf("FaultHistory() = %+v, want nil before any fault is recorded", hist)
	}
}

func TestFaultHistoryRingBufferWraps(t *testing.T) {
	e := newEngine()
	e.SetFaultHistorySize(2)

	e.recordFault(ExcPrivileged, nil, 0x100, 0)
	e.recordFault(ExcCustomer, []uint32{1, 2}, 0x200, 0)
	e.recordFault(ExcReservedOp, nil, 0x300, 0)

	hist := e.FaultHistory()
	if len(hist) != 2 {
		t.Fatalf("len(FaultHistory()) = %d, want 2 (ring size 2, oldest overwritten)", len(hist))
	}

	if hist[0].Code != ExcCustomer || hist[1].Code != ExcReservedOp {
		t.Errorf("history codes = [%#02x, %#02x], want [%#02x, %#02x]",
			hist[0].Code, hist[1].Code, ExcCustomer, ExcReservedOp)
	}
	// Seq (history_id) counts every fault ever recorded, including the one
	// the ring buffer has since overwritten.
	if hist[0].Seq != 2 || hist[1].Seq != 3 {
		t.Errorf("Seq = %d, %d, want 2, 3", hist[0].Seq, hist[1].Seq)
	}

	if len(hist[0].Args) != 2 || hist[0].Args[0] != 1 || hist[0].Args[1] != 2 {
		t.Errorf("Args = %v, want [1 2]", hist[0].Args)
	}
}

// TestFaultHistorySetSizeResetsButKeepsSequence checks
// set_fault_history's own two-part behavior: the buffer itself is always
// emptied, but the running fault count (history_id / Seq) is not reset.
func TestFaultHistorySetSizeResetsButKeepsSequence(t *testing.T) {
	e := newEngine()
	e.SetFaultHistorySize(4)
	e.recordFault(ExcPrivileged, nil, 0x100, 0)
	e.recordFault(ExcCustomer, nil, 0x200, 0)

	e.SetFaultHistorySize(4)
	
	if hist := e.FaultHistory(); hist != nil {
		t.Errorf("FaultHistory() after resize = %+v, want nil (buffer emptied)", hist)
	}

	e.recordFault(ExcReservedOp, nil, 0x300, 0)
	hist := e.FaultHistory()
	if len(hist) != 1 {
		t.Fatalf("len(FaultHistory()) = %d, want 1", len(hist))
	}

	if hist[0].Seq != 3 {
		t.Errorf("Seq = %d, want 3 (continuing from the two faults recorded before the resize)", hist[0].Seq)
	}
}

func TestFaultHistoryZeroDisablesRecording(t *testing.T) {
	e := newEngine()
	e.SetFaultHistorySize(0)
	e.recordFault(ExcPrivileged, nil, 0x100, 0)

	if hist := e.FaultHistory(); hist != nil {
		t.Errorf("FaultHistory() = %+v, want nil when disabled (n<=0)", hist)
	}
}

package cpu

import (
	"strings"
	"testing"
)

func TestFaultError(t *testing.T) {
	f := &Fault{Code: ExcReservedAddr, Args: []uint32{1, 2}}
	got := f.Error()
	// Not pinned to an exact format, just that the code is findable in the
	// message (ExcReservedAddr == 0x1c).
	if !strings.Contains(got, "1c") {
		t.Errorf("Error() = %q, want it to mention the code (0x1c)", got)
	}
}

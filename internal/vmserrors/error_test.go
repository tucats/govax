package vmserrors

import "testing"

func TestErrorAccvioWithArgument(t *testing.T) {
	err := New(SS_ACCVIO, uint32(0xDEADBEEF))

	const want = "SYS$ACCVIO, Access violation at deadbeef"

	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	err = New(0x42, "Custard")
	got := err.Error()

	const want2 = `SYS$UNKNOWN, Unknown error 00000042 ["Custard"]`
	if got != want2 {
		t.Errorf("Error() = %q, want %q", got, want2)
	}
}

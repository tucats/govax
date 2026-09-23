package vmserrors

import "testing"

func TestErrorAccvioWithArgument(t *testing.T) {
	err := New(SS_ACCVIO, uint32(0xDEADBEEF))

	const want = "SYSTEM-F-ACCVIO, Access violation at deadbeef"

	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	err = New(0x42, "Custard")
	got := err.Error()

	const want2 = `SYSTEM-F-UNKNOWNERR, Unknown error 00000042 ["Custard"]`
	if got != want2 {
		t.Errorf("Error() = %q, want %q", got, want2)
	}
}

// TestSSMountCodesMatchRealSSDEF confirms SS_DEVMOUNT/SS_DEVNOTMOUNT/
// SS_NOMOUNT/SS_NOSUCHFILE's numeric values are exactly the real, literal
// SS$_DEVMOUNT/SS$_DEVNOTMOUNT/SS$_NOMOUNT/SS$_NOSUCHFILE values from
// reference/eVAX/eVAX/Headers/ss_def.h (108/124/10380/2320) — unlike this
// package's other facilities (RMS/CLI/LIB/VAX), the SYS facility's whole
// point is reproducing VMS's own real numbers (see codes_sys.go's own doc
// comment), so a mismatch here would be a real fidelity bug, not just an
// internal renumbering.
func TestSSMountCodesMatchRealSSDEF(t *testing.T) {
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"SS_DEVMOUNT", SS_DEVMOUNT, 108},
		{"SS_DEVNOTMOUNT", SS_DEVNOTMOUNT, 124},
		{"SS_NOMOUNT", SS_NOMOUNT, 10380},
		{"SS_NOSUCHFILE", SS_NOSUCHFILE, 2320},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d (real ss_def.h value)", tc.name, tc.got, tc.want)
		}
	}
}

// TestErrorMountCodesFormatDeviceName confirms the three MOUNT/DISMOUNT
// status codes render their device-name argument into readable message
// text, including SS_NOMOUNT's wrapped-cause case (vmserrors.Wrap, used
// by internal/console/mount.go when the underlying internal/rms.MountTable
// call itself fails).
func TestErrorMountCodesFormatDeviceName(t *testing.T) {
	if got, want := New(SS_DEVMOUNT, "DUA0").Error(), "SYSTEM-F-DEVMOUNT, Device DUA0 already mounted"; got != want {
		t.Errorf("SS_DEVMOUNT.Error() = %q, want %q", got, want)
	}

	if got, want := New(SS_DEVNOTMOUNT, "DUA0").Error(), "SYSTEM-F-DEVNOTMOUNT, Device DUA0 not mounted"; got != want {
		t.Errorf("SS_DEVNOTMOUNT.Error() = %q, want %q", got, want)
	}

	cause := New(SS_ACCVIO) // any distinguishable underlying error works here
	got := Wrap(SS_NOMOUNT, cause, "DUA0").Error()
	want := "SYSTEM-F-NOMOUNT, Unable to complete MOUNT/DISMOUNT operation on device DUA0: " + cause.Error()

	if got != want {
		t.Errorf("SS_NOMOUNT.Error() = %q, want %q", got, want)
	}
}

package librtl

import "testing"

// TestGetEF: LIB$GET_EF hands out 63 down to 32, highest first, then -1 and
// LIB$_INSEF; a flag freed is handed out again.
func TestGetEF(t *testing.T) {
	env := fixture(t)

	for want := int32(lastHighEF); want >= firstHighEF; want-- {
		if got := call(t, env, "LIB$GET_EF", 0x1000); got != ssNormal || int32(longword(t, env, 0x1000)) != want {
			t.Fatalf("GET_EF: %#x, flag %d, want %d", got, int32(longword(t, env, 0x1000)), want)
		}
	}

	if got := call(t, env, "LIB$GET_EF", 0x1000); got != libInsEF || int32(longword(t, env, 0x1000)) != -1 {
		t.Errorf("GET_EF with none free: %#x, flag %d, want LIB$_INSEF and -1", got, int32(longword(t, env, 0x1000)))
	}

	putLongword(t, env, 0x1004, 40)

	if got := call(t, env, "LIB$FREE_EF", 0x1004); got != ssNormal {
		t.Errorf("FREE_EF(40): %#x", got)
	}

	if got := call(t, env, "LIB$GET_EF", 0x1000); got != ssNormal || longword(t, env, 0x1000) != 40 {
		t.Errorf("GET_EF after freeing 40: %#x, flag %d", got, longword(t, env, 0x1000))
	}
}

// TestFreeReserveEF: 1 to 23 start reserved (RESERVE_EF is ALRRES until
// FREE_EF frees one, which GET_EF then prefers); 0, 24 to 31, and 64 are
// EF_RESSYS; a free flag is EF_ALRFRE to FREE_EF.
func TestFreeReserveEF(t *testing.T) {
	env := fixture(t)
	ef := func(n uint32) uint32 { putLongword(t, env, 0x1004, n); return 0x1004 }

	for _, c := range []struct {
		name string
		ef   uint32
		want uint32
	}{
		{"LIB$RESERVE_EF", 5, libEFAlrRes},
		{"LIB$FREE_EF", 5, ssNormal},
		{"LIB$FREE_EF", 5, libEFAlrFre},
		{"LIB$RESERVE_EF", 5, ssNormal},
		{"LIB$FREE_EF", 32, libEFAlrFre},
		{"LIB$RESERVE_EF", 32, ssNormal},
		{"LIB$RESERVE_EF", 32, libEFAlrRes},
		{"LIB$FREE_EF", 32, ssNormal},
		{"LIB$FREE_EF", 0, libEFResSys},
		{"LIB$RESERVE_EF", 0, libEFResSys},
		{"LIB$FREE_EF", 24, libEFResSys},
		{"LIB$RESERVE_EF", 31, libEFResSys},
		{"LIB$FREE_EF", 64, libEFResSys},
		{"LIB$RESERVE_EF", 0xFFFFFFFF, libEFResSys},
		{"LIB$FREE_EF", 23, ssNormal},
		{"LIB$FREE_EF", 1, ssNormal},
	} {
		if got := call(t, env, c.name, ef(c.ef)); got != c.want {
			t.Errorf("%s(%d) = %#x, want %#x", c.name, int32(c.ef), got, c.want)
		}
	}

	// 1 and 23 are free now, but 63 down to 32 come first (VMS 7.1).
	for want := uint32(lastHighEF); want >= firstHighEF; want-- {
		if got := call(t, env, "LIB$GET_EF", 0x1000); got != ssNormal || longword(t, env, 0x1000) != want {
			t.Fatalf("GET_EF: %#x, flag %d, want %d", got, longword(t, env, 0x1000), want)
		}
	}

	for _, want := range []uint32{23, 1} {
		if got := call(t, env, "LIB$GET_EF", 0x1000); got != ssNormal || longword(t, env, 0x1000) != want {
			t.Errorf("GET_EF: %#x, flag %d, want %d", got, longword(t, env, 0x1000), want)
		}
	}
}

// TestLUN: LIB$GET_LUN hands out 119 down to 100, highest first, then -1 and
// LIB$_INSLUN; FREE_LUN frees one (LUNALRFRE when free) and refuses a
// number outside the pool (LUNRESSYS).
func TestLUN(t *testing.T) {
	env := fixture(t)
	lun := func(n uint32) uint32 { putLongword(t, env, 0x1004, n); return 0x1004 }

	if got := call(t, env, "LIB$FREE_LUN", lun(100)); got != libLUNAlrFre {
		t.Errorf("FREE_LUN(100) at start: %#x, want LIB$_LUNALRFRE", got)
	}

	for want := uint32(lastLUN); want >= firstLUN; want-- {
		if got := call(t, env, "LIB$GET_LUN", 0x1000); got != ssNormal || longword(t, env, 0x1000) != want {
			t.Fatalf("GET_LUN: %#x, unit %d, want %d", got, longword(t, env, 0x1000), want)
		}
	}

	if got := call(t, env, "LIB$GET_LUN", 0x1000); got != libInsLUN || int32(longword(t, env, 0x1000)) != -1 {
		t.Errorf("GET_LUN with none free: %#x, unit %d", got, int32(longword(t, env, 0x1000)))
	}

	for _, c := range []struct{ lun, want uint32 }{
		{110, ssNormal}, {110, libLUNAlrFre}, {99, libLUNResSys}, {120, libLUNResSys}, {0xFFFFFF9C, libLUNResSys}, // -100
	} {
		if got := call(t, env, "LIB$FREE_LUN", lun(c.lun)); got != c.want {
			t.Errorf("FREE_LUN(%d) = %#x, want %#x", int32(c.lun), got, c.want)
		}
	}

	if got := call(t, env, "LIB$GET_LUN", 0x1000); got != ssNormal || longword(t, env, 0x1000) != 110 {
		t.Errorf("GET_LUN after freeing 110: %#x, unit %d", got, longword(t, env, 0x1000))
	}
}

// TestPoolsPerImage: image rundown forgets the pools, so the next image
// starts with the initial reservations.
func TestPoolsPerImage(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, 0x1004, 5)
	call(t, env, "LIB$FREE_EF", 0x1004)
	call(t, env, "LIB$GET_LUN", 0x1000)

	env.ImageRundown()

	if got := call(t, env, "LIB$RESERVE_EF", 0x1004); got != libEFAlrRes {
		t.Errorf("RESERVE_EF(5) in the next image: %#x, want LIB$_EF_ALRRES", got)
	}

	if got := call(t, env, "LIB$GET_LUN", 0x1000); got != ssNormal || longword(t, env, 0x1000) != lastLUN {
		t.Errorf("GET_LUN in the next image: %#x, unit %d", got, longword(t, env, 0x1000))
	}
}

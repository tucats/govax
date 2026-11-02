package librtl

import "testing"

// testdata/str83's oracle (TestStr83Oracle, in internal/console) holds the
// routines to VMS's run. These are the choices it can't settle, and a guard on
// the pieces worth testing alone.

// TestWildMatch: the probe's patterns with % went through its generator's own
// use of %, so % as a wildcard rests on the manual, and is tested here.
func TestWildMatch(t *testing.T) {
	cases := []struct {
		candidate, pattern string
		want               bool
	}{
		{"abc", "abc", true},
		{"abc", "a%c", true},
		{"abc", "a%", false},
		{"abc", "%%%", true},
		{"abc", "%%%%", false},
		{"abc", "*", true},
		{"", "*", true},
		{"", "", true},
		{"abc", "", false},
		{"", "%", false},
		{"abcabc", "*b*c", true},
		{"abcabd", "*b*c", false},
		{"aXbXc", "a*X*c", true},
		{"a*c", `a\*c`, false},
		{"ABC", "abc", false},
		{"abcbcd", "a*bcd", true},
		{"abc", "**c", true},
	}

	for _, c := range cases {
		if got := wildMatch(c.candidate, c.pattern); got != c.want {
			t.Errorf("wildMatch(%q, %q) = %v, want %v", c.candidate, c.pattern, got, c.want)
		}
	}
}

// TestPositionRules: the rules below VMS's cases, where two conditions meet.
func TestPositionRules(t *testing.T) {
	const n = 5

	cases := []struct {
		name string
		got  part
		want part
	}{
		{"POS_EXTR both before the start", posExtract(n, 0, -1), part{status: strIllStrSpe}},
		{"POS_EXTR past both ends", posExtract(n, 0, 9), part{0, 5, strIllStrPos}},
		{"LEN_EXTR negative wins", lenExtract(n, 0, -1), part{status: strNegStrLen}},
		{"LEN_EXTR before and past", lenExtract(n, 0, 9), part{0, 5, strIllStrSpe}},
		{"LEN_EXTR no length past the end", lenExtract(n, 9, 0), part{status: ssNormal}},
		{"LEN_EXTR huge length", lenExtract(n, 2, 0x7FFFFFFF), part{1, 5, strIllStrSpe}},
		{"LEFT an empty source", leftPart(0, 1), part{0, 0, strIllStrPos}},
		{"RIGHT an empty source", rightPart(0, 1), part{status: ssNormal}},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, c.got, c.want)
		}
	}
}

// TestStrFind: the search's start past the end, and an empty substring there.
func TestStrFind(t *testing.T) {
	cases := []struct {
		s, sub string
		start  int32
		want   int
	}{
		{"ABCAB", "AB", 2, 4},
		{"ABC", "", 4, 4},
		{"ABC", "", 5, 0},
		{"ABC", "C", -7, 3},
		{"", "", 1, 1},
	}

	for _, c := range cases {
		if got := strFind(c.s, c.sub, c.start); got != c.want {
			t.Errorf("strFind(%q, %q, %d) = %d, want %d", c.s, c.sub, c.start, got, c.want)
		}
	}
}

// TestStrWarningOverAlternate: a truncation, a warning, is what a routine returns
// even if a position was off too; the alternate success stands when it fits.
func TestStrWarningOverAlternate(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, 0x1000, 0x1100, "ABCDE")
	putDescriptor(t, env, 0x1010, 0x1110, "--")
	putDescriptor(t, env, 0x1020, 0x1120, "--------")

	mem, cpu := env.Memory(), env.CPU()

	for addr, v := range map[uint32]uint32{0x1200: 0, 0x1204: 9} {
		if err := mem.StoreLongword(cpu, addr, v); err != nil {
			t.Fatal(err)
		}
	}

	if r0 := call(t, env, "STR$POS_EXTR", 0x1010, 0x1000, 0x1200, 0x1204); r0 != strTru {
		t.Errorf("into 2 bytes: %#x, want STR$_TRU", r0)
	}

	if r0 := call(t, env, "STR$POS_EXTR", 0x1020, 0x1000, 0x1200, 0x1204); r0 != strIllStrPos {
		t.Errorf("into 8 bytes: %#x, want STR$_ILLSTRPOS", r0)
	}
}

// TestStrElementLeavesDestination: with too few elements, or a delimiter of the
// wrong length, a destination that holds something keeps it.
func TestStrElementLeavesDestination(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, 0x1000, 0x1100, "kept")
	putDescriptor(t, env, 0x1010, 0x1110, "A,B")
	putDescriptor(t, env, 0x1020, 0x1120, ",")
	putDescriptor(t, env, 0x1030, 0x1130, ",,")

	if err := env.Memory().StoreLongword(env.CPU(), 0x1200, 2); err != nil {
		t.Fatal(err)
	}

	if r0 := call(t, env, "STR$ELEMENT", 0x1000, 0x1200, 0x1020, 0x1010); r0 != strNoElem {
		t.Errorf("element 2 of 2: %#x, want STR$_NOELEM", r0)
	}

	if r0 := call(t, env, "STR$ELEMENT", 0x1000, 0x1200, 0x1030, 0x1010); r0 != strInvDelim {
		t.Errorf("a long delimiter: %#x, want STR$_INVDELIM", r0)
	}

	if got, err := readString(env, 0x1000); err != nil || got != "kept" {
		t.Errorf("the destination = %q, %v; want it kept", got, err)
	}
}

// TestStrSameString: a routine reads its source before it writes, so one string
// can be both, including a varying one growing in STR$APPEND.
func TestStrSameString(t *testing.T) {
	env := fixture(t)
	mem, cpu := env.Memory(), env.CPU()

	// A varying string: the descriptor's length is its maximum, and the word
	// at its pointer is its current length, with the text after it.
	for _, step := range []func() error{
		func() error { return mem.StoreLongword(cpu, 0x1000, 8|dscDTypeT<<16|dscClassVS<<24) },
		func() error { return mem.StoreLongword(cpu, 0x1004, 0x1100) },
		func() error { return mem.StoreWord(cpu, 0x1100, 2) },
		func() error { return mem.StoreWord(cpu, 0x1102, 'A'|'B'<<8) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}

	if r0 := call(t, env, "STR$APPEND", 0x1000, 0x1000); r0 != ssNormal {
		t.Errorf("r0 = %#x", r0)
	}

	if got, err := readString(env, 0x1000); err != nil || got != "ABAB" {
		t.Errorf("appended to itself = %q, %v; want ABAB", got, err)
	}
}

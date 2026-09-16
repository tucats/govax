package rtl

import "testing"

func TestShimDeccPrintf(t *testing.T) {
	env, out := fixture()
	fmtAddr, strAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, fmtAddr, "count=%d name=%s\n")
	putString(t, env, strAddr, "vax")

	r0, err := shimDeccPrintf(env, []uint32{fmtAddr, 42, strAddr})
	if err != nil {
		t.Fatal(err)
	}

	want := "count=42 name=vax\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}

	if int(r0) != len(want) {
		t.Errorf("r0 = %d, want %d", r0, len(want))
	}
}

func TestShimDeccPrintfHexAndChar(t *testing.T) {
	env, out := fixture()
	fmtAddr := uint32(0x1000)
	putString(t, env, fmtAddr, "%X %c")

	if _, err := shimDeccPrintf(env, []uint32{fmtAddr, 0xFF, uint32('A')}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); got != "FF A" {
		t.Errorf("output = %q, want \"FF A\"", got)
	}
}

func TestShimDeccPrintfEscapes(t *testing.T) {
	env, out := fixture()
	fmtAddr := uint32(0x1000)
	putString(t, env, fmtAddr, "a\\tb\\n")

	if _, err := shimDeccPrintf(env, []uint32{fmtAddr}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); got != "a\tb\n" {
		t.Errorf("output = %q, want %q", got, "a\tb\n")
	}
}

func TestShimDeccPrintfLiteralPercent(t *testing.T) {
	env, out := fixture()
	fmtAddr := uint32(0x1000)
	putString(t, env, fmtAddr, "100%% done")

	if _, err := shimDeccPrintf(env, []uint32{fmtAddr}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); got != "100% done" {
		t.Errorf("output = %q, want \"100%% done\"", got)
	}
}

func TestShimDeccSprintf(t *testing.T) {
	env, _ := fixture()
	bufAddr, fmtAddr := uint32(0x2000), uint32(0x1000)
	putString(t, env, fmtAddr, "x=%d")

	r0, err := shimDeccSprintf(env, []uint32{bufAddr, fmtAddr, 7})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 0 {
		t.Errorf("r0 = %d, want 0", r0)
	}

	got, err := loadString(env, bufAddr, 32)
	if err != nil {
		t.Fatal(err)
	}

	if got != "x=7" {
		t.Errorf("buffer = %q, want \"x=7\"", got)
	}
}

func TestShimDeccPrintfWidthFlags(t *testing.T) {
	env, out := fixture()
	fmtAddr := uint32(0x1000)
	putString(t, env, fmtAddr, "[%5d][%-5d]")

	if _, err := shimDeccPrintf(env, []uint32{fmtAddr, 3, 3}); err != nil {
		t.Fatal(err)
	}
	
	if got := out.String(); got != "[    3][3    ]" {
		t.Errorf("output = %q, want \"[    3][3    ]\"", got)
	}
}

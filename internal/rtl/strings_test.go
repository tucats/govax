package rtl

import "testing"

func TestClassifyShims(t *testing.T) {
	cases := []struct {
		code uint32
		ch   byte
		want uint32
	}{
		{17, 'a', 1}, {17, ' ', 0}, // isalnum
		{18, 'Z', 1}, {18, '9', 0}, // isalpha
		{19, 0x01, 1}, {19, 'x', 0}, // iscntrl
		{20, '5', 1}, {20, 'x', 0}, // isdigit
		{22, 'z', 1}, {22, 'Z', 0}, // islower
		{26, 'Z', 1}, {26, 'z', 0}, // isupper
		{25, ' ', 1}, {25, 'x', 0}, // isspace
		{27, 'f', 1}, {27, 'g', 0}, // isxdigit
		{28, 0xFF, 1}, // isascii always true
	}
	env, _ := fixture()

	for _, c := range cases {
		fn, ok := env.shims.Lookup(c.code)
		if !ok {
			t.Fatalf("shim code %d not registered", c.code)
		}

		got, err := fn(env, []uint32{uint32(c.ch)})
		if err != nil {
			t.Fatal(err)
		}

		if got != c.want {
			t.Errorf("code %d ch %#x: got %d, want %d", c.code, c.ch, got, c.want)
		}
	}
}

func TestShimStrUpcase(t *testing.T) {
	env, _ := fixture()
	descAddr, strAddr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, descAddr, strAddr, "Hello, World!")

	r0, err := shimStrUpcase(env, []uint32{descAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 1 {
		t.Errorf("r0 = %d, want 1", r0)
	}

	got, err := loadString(env, strAddr, 32)
	if err != nil {
		t.Fatal(err)
	}

	if got != "HELLO, WORLD!" {
		t.Errorf("upcased = %q, want %q", got, "HELLO, WORLD!")
	}
}

func TestCAtoi(t *testing.T) {
	cases := map[string]int32{
		"123":     123,
		"  42":    42,
		"-7":      -7,
		"+9":      9,
		"abc":     0,
		"":        0,
		"12abc":   12,
		"   -100": -100,
	}

	for in, want := range cases {
		if got := cAtoi(in); got != want {
			t.Errorf("cAtoi(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestShimDeccAtoi(t *testing.T) {
	env, _ := fixture()
	addr := uint32(0x1000)
	putString(t, env, addr, "  42")

	r0, err := shimDeccAtoi(env, []uint32{addr})
	if err != nil {
		t.Fatal(err)
	}

	if int32(r0) != 42 {
		t.Errorf("r0 = %d, want 42", int32(r0))
	}
}

func TestShimDeccStrcmp(t *testing.T) {
	env, _ := fixture()
	a, b := uint32(0x1000), uint32(0x2000)
	putString(t, env, a, "abc")
	putString(t, env, b, "abd")

	r0, err := shimDeccStrcmp(env, []uint32{a, b})
	if err != nil {
		t.Fatal(err)
	}

	if int32(r0) >= 0 {
		t.Errorf("r0 = %d, want negative (\"abc\" < \"abd\")", int32(r0))
	}

	r0, err = shimDeccStrcmp(env, []uint32{a, a})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 0 {
		t.Errorf("r0 = %d, want 0 (equal strings)", r0)
	}
}

func TestShimDeccStrncmp(t *testing.T) {
	env, _ := fixture()
	a, b := uint32(0x1000), uint32(0x2000)
	putString(t, env, a, "abcXXX")
	putString(t, env, b, "abcYYY")

	r0, err := shimDeccStrncmp(env, []uint32{a, b, 3})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 0 {
		t.Errorf("r0 = %d, want 0 (first 3 bytes equal)", r0)
	}
}

func TestShimDeccStrncpy(t *testing.T) {
	env, _ := fixture()
	src, dst := uint32(0x1000), uint32(0x2000)
	putString(t, env, src, "hi")

	r0, err := shimDeccStrncpy(env, []uint32{dst, src, 10})
	if err != nil {
		t.Fatal(err)
	}
	
	if r0 != 3 { // strlen("hi")+1
		t.Errorf("r0 = %d, want 3", r0)
	}

	got, err := loadString(env, dst, 10)
	if err != nil {
		t.Fatal(err)
	}

	if got != "hi" {
		t.Errorf("copied = %q, want \"hi\"", got)
	}

	// Truncating case: length cap shorter than the source string.
	r0, err = shimDeccStrncpy(env, []uint32{dst, src, 1})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 1 {
		t.Errorf("r0 = %d, want 1 (truncated, no room for the NUL)", r0)
	}
}

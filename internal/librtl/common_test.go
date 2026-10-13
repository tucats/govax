package librtl

import (
	"strings"
	"testing"
)

// TestLibCommon: the common area starts empty, keeps at most 252
// characters (LIB$_STRTRU past them), and outlasts the image; neither
// routine needs a CLI (the fixture has none).
func TestLibCommon(t *testing.T) {
	env := fixture(t)
	a := newArena(t, env)
	length := a.word(99)

	// Before any LIB$PUT_COMMON: empty, a fixed string all blanks.
	result := a.desc("xxxx")
	if r0 := call(t, env, "LIB$GET_COMMON", result, length); r0 != ssNormal {
		t.Fatalf("GET of the empty area: status = %#x", r0)
	}

	if got, n := descText(t, env, result), word(t, env, length); got != "    " || n != 0 {
		t.Errorf("empty area: result %q, length %d", got, n)
	}

	cases := []struct {
		put    string
		status uint32
		kept   string
	}{
		{"hello", ssNormal, "hello"},
		{"", ssNormal, ""},
		{strings.Repeat("a", maxCommon), ssNormal, strings.Repeat("a", maxCommon)},
		{strings.Repeat("b", maxCommon) + "cd", libStrTru, strings.Repeat("b", maxCommon)},
	}

	for _, c := range cases {
		if r0 := call(t, env, "LIB$PUT_COMMON", a.desc(c.put), length); r0 != c.status {
			t.Errorf("PUT of %d characters: status = %#x, want %#x", len(c.put), r0, c.status)
		}

		if n := word(t, env, length); int(n) != len(c.kept) {
			t.Errorf("PUT of %d characters: length = %d, want %d", len(c.put), n, len(c.kept))
		}

		result := a.desc(strings.Repeat("x", 300))
		if r0 := call(t, env, "LIB$GET_COMMON", result, length); r0 != ssNormal {
			t.Errorf("GET after PUT of %d characters: status = %#x", len(c.put), r0)
		}

		if got := descText(t, env, result); got != c.kept+strings.Repeat(" ", 300-len(c.kept)) {
			t.Errorf("GET after PUT of %d characters = %q", len(c.put), strings.TrimRight(got, " "))
		}

		if n := word(t, env, length); int(n) != len(c.kept) {
			t.Errorf("GET after PUT of %d characters: length = %d", len(c.put), n)
		}
	}

	// A fixed string too short: LIB$_STRTRU, the length what was stored.
	call(t, env, "LIB$PUT_COMMON", a.desc("longer text"), 0)

	short := a.desc("xxx")
	if r0 := call(t, env, "LIB$GET_COMMON", short, length); r0 != libStrTru {
		t.Errorf("GET into 3 characters: status = %#x, want LIB$_STRTRU", r0)
	}

	if got, n := descText(t, env, short), word(t, env, length); got != "lon" || n != 3 {
		t.Errorf("GET into 3 characters: %q, length %d", got, n)
	}

	// The area outlasts the image.
	env.ImageRundown()

	result = a.desc(strings.Repeat("x", 20))
	if r0 := call(t, env, "LIB$GET_COMMON", result, 0); r0 != ssNormal {
		t.Fatalf("GET after image rundown: status = %#x", r0)
	}

	if got := strings.TrimRight(descText(t, env, result), " "); got != "longer text" {
		t.Errorf("GET after image rundown = %q", got)
	}
}

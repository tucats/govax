package librtl

import (
	"strings"
	"testing"
)

func TestSplitFileSpec(t *testing.T) {
	for _, c := range []struct {
		spec string
		want fileSpecParts
	}{
		{"DISK$NAME:[DIRECTORY1.DIRECTORY2]FILENAME.EXTENSTION;1",
			fileSpecParts{"", "DISK$NAME:", "[DIRECTORY1.DIRECTORY2]", "FILENAME", ".EXTENSTION", ";1"}},
		{`NODE"USER PASS"::DUA1:[A]B.C;2`, fileSpecParts{`NODE"USER PASS"::`, "DUA1:", "[A]", "B", ".C", ";2"}},
		{"A::B::DUA1:<X.Y>Z", fileSpecParts{"A::B::", "DUA1:", "<X.Y>", "Z", "", ""}},
		{"ROOT:[A.][B]C.D.3", fileSpecParts{"", "ROOT:", "[A.][B]", "C", ".D", ".3"}},
		{"NAME", fileSpecParts{"", "", "", "NAME", "", ""}},
		{"NAME;", fileSpecParts{"", "", "", "NAME", "", ";"}},
		{".TYP", fileSpecParts{"", "", "", "", ".TYP", ""}},
		{"[UNMATCHED", fileSpecParts{"", "", "", "[UNMATCHED", "", ""}},
	} {
		if got := splitFileSpec(c.spec); got != c.want {
			t.Errorf("splitFileSpec(%q) = %#v, want %#v", c.spec, got, c.want)
		}

		if got := c.want.String(); got != c.spec {
			t.Errorf("%#v.String() = %q, want %q", c.want, got, c.spec)
		}
	}
}

func TestTrimFilespec(t *testing.T) {
	const spec = `NODE::DUA1:[DIR]NAME.TYP;12` // 6 + 5 + 5 + 4 + 4 + 3 = 27

	for _, c := range []struct {
		spec   string
		width  int
		want   string
		status uint32
	}{
		// The manual's example.
		{"DISK$NAME:[DIRECTORY1.DIRECTORY2]FILENAME.EXTENSTION;1", 32, "FILENAME.EXTENSTION;1", ssNormal},
		// Each part's edge.
		{spec, 27, spec, ssNormal},
		{spec, 26, "DUA1:[DIR]NAME.TYP;12", ssNormal},
		{spec, 21, "DUA1:[DIR]NAME.TYP;12", ssNormal},
		{spec, 20, "[DIR]NAME.TYP;12", ssNormal},
		{spec, 15, "NAME.TYP;12", ssNormal},
		{spec, 10, "NAME.TYP", ssNormal},
		{spec, 7, "NAME", ssNormal},
		{spec, 4, "NAME", ssNormal},
		{spec, 3, "NAM", libStrTru},
		{spec, 0, "", libStrTru},
		{"", 0, "", ssNormal},
	} {
		env := fixture(t)
		a := newArena(t, env)
		out, length := a.desc(strings.Repeat("?", 40)), a.word(0xEEEE)

		st := call(t, env, "LIB$TRIM_FILESPEC", a.desc(c.spec), out, a.word(uint16(c.width)), length)
		got, n := strings.TrimRight(descText(t, env, out), " "), word(t, env, length)

		if st != c.status || got != c.want || int(n) != len(c.want) {
			t.Errorf("%q width %d: status %#x, %q (length %d); want %#x, %q",
				c.spec, c.width, st, got, n, c.status, c.want)
		}
	}
}

func TestTrimFilespecArguments(t *testing.T) {
	env := fixture(t)
	a := newArena(t, env)

	// Width omitted: new-filespec's length, here 12.
	out := a.desc(strings.Repeat("?", 12))
	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc("DUA1:[DIR]NAME.TYP;1"), out); st != ssNormal {
		t.Errorf("width omitted: status %#x", st)
	}

	if got := descText(t, env, out); got != "NAME.TYP;1  " {
		t.Errorf("width omitted: %q, want %q", got, "NAME.TYP;1  ")
	}

	// A width wider than new-filespec: the result doesn't fit it.
	short := a.desc("?????")
	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc("NAME.TYP"), short, a.word(20)); st != libStrTru {
		t.Errorf("a short new-filespec: status %#x, want LIB$_STRTRU", st)
	}

	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc(strings.Repeat("A", 256)), out); st != libInvArg {
		t.Errorf("256 characters: status %#x, want LIB$_INVARG", st)
	}

	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc(strings.Repeat("A", 255)), out); st != libStrTru {
		t.Errorf("255 characters: status %#x, want LIB$_STRTRU", st)
	}

	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc("A")); st != libWroNumArg {
		t.Errorf("one argument: status %#x, want LIB$_WRONUMARG", st)
	}

	if st := call(t, env, "LIB$TRIM_FILESPEC", a.desc("A"), out, 0, 0, 0); st != libWroNumArg {
		t.Errorf("five arguments: status %#x, want LIB$_WRONUMARG", st)
	}
}

package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// findFixture is createDirFixture's volume with files in [000000] (A.TXT,
// B.TXT, C.DAT) and in [SUB] (D.TXT).
func findFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	env := createDirFixture(t, false)

	if st := call(t, env, "LIB$CREATE_DIR", newArena(t, env).desc("DUA0:[SUB]")); st&1 == 0 {
		t.Fatalf("LIB$CREATE_DIR = %#x", st)
	}

	for _, name := range []string{"DUA0:[000000]A.TXT", "DUA0:[000000]B.TXT", "DUA0:[000000]C.DAT", "DUA0:[SUB]D.TXT"} {
		if _, err := rms.NewSession(env.Mounts).CreateRecordFile(rms.FileLocation{Name: name}, rms.TextRecords, [][]byte{[]byte("x")}); err != nil {
			t.Fatal(err)
		}
	}

	return env
}

// finder calls LIB$FIND_FILE with one context, the arguments' strings
// in an arena.
type finder struct {
	t      *testing.T
	env    *corevms.Environment
	a      *arena
	cell   uint32
	result uint32
	stv    uint32
}

func newFinder(t *testing.T, env *corevms.Environment) *finder {
	a := newArena(t, env)
	f := &finder{t: t, env: env, a: a, cell: a.long(0), stv: a.long(0)}
	f.result = a.desc(strings.Repeat(" ", 80))

	return f
}

// find calls LIB$FIND_FILE for spec with the optional default and
// related names ("" omits one) and flags, returning the status and the
// resultant name (trailing blanks removed).
func (f *finder) find(spec, defaultSpec, related string, flags uint32) (uint32, string) {
	f.t.Helper()

	optional := func(s string) uint32 {
		if s == "" {
			return 0
		}

		return f.a.desc(s)
	}

	putLongword(f.t, f.env, f.stv, 0xDEAD)

	st := call(f.t, f.env, "LIB$FIND_FILE", f.a.desc(spec), f.result, f.cell,
		optional(defaultSpec), optional(related), f.stv, f.a.long(flags))

	return st, strings.TrimRight(descText(f.t, f.env, f.result), " ")
}

// expect runs find and checks its status and, for a success, the name.
func (f *finder) expect(spec, defaultSpec, related string, flags, wantStatus uint32, wantName string) {
	f.t.Helper()

	st, name := f.find(spec, defaultSpec, related, flags)
	if st != wantStatus || (st&1 != 0 && name != wantName) {
		f.t.Errorf("FIND_FILE(%q, %q, %q, %d) = %#x %q, want %#x %q", spec, defaultSpec, related, flags, st, name, wantStatus, wantName)
	}
}

func (f *finder) end() uint32 {
	f.t.Helper()

	return call(f.t, f.env, "LIB$FIND_FILE_END", f.cell)
}

// TestFindFileWildcard: a wildcard name returns each match in turn, then
// RMS$_NMF, which ends the context (its longword 0), so the call after
// starts the search again.
func TestFindFileWildcard(t *testing.T) {
	f := newFinder(t, findFixture(t))

	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]B.TXT;1")

	if longword(t, f.env, f.cell) == 0 {
		t.Error("the context longword is 0 during the search")
	}

	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNMF, "")

	if c := longword(t, f.env, f.cell); c != 0 {
		t.Errorf("context after RMS$_NMF = %#x, want 0", c)
	}

	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")

	if st := f.end(); st != ssNormal {
		t.Errorf("FIND_FILE_END = %#x", st)
	}

	if c := longword(t, f.env, f.cell); c != 0 {
		t.Errorf("context after FIND_FILE_END = %#x, want 0", c)
	}
}

// TestFindFileWildDirectory: a wildcard directory (a search with RMS
// context) walks the tree, and a search list searches each element.
func TestFindFileWildDirectory(t *testing.T) {
	env := findFixture(t)
	f := newFinder(t, env)

	f.expect("DUA0:[000000...]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.expect("DUA0:[000000...]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]B.TXT;1")
	f.expect("DUA0:[000000...]*.TXT", "", "", 0, rmsNormal, "DUA0:[SUB]D.TXT;1")
	f.expect("DUA0:[000000...]*.TXT", "", "", 0, rmsNMF, "")

	if _, err := env.Logicals.Define(lnm.ProcessTableName, "FINDSL", lnm.User, 0,
		[]lnm.Equivalence{{Value: "DUA0:[SUB]"}, {Value: "DUA0:[000000]"}}); err != nil {
		t.Fatal(err)
	}

	f.expect("FINDSL:*.TXT", "", "", 0, rmsNormal, "DUA0:[SUB]D.TXT;1")
	f.expect("FINDSL:*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")

	// A different name on the same context starts a new search.
	f.expect("DUA0:[000000]*.DAT", "", "", 0, rmsNormal, "DUA0:[000000]C.DAT;1")
	f.expect("DUA0:[000000]*.DAT", "", "", 0, rmsNMF, "")

	if st := f.end(); st != ssNormal {
		t.Errorf("FIND_FILE_END = %#x", st)
	}
}

// TestFindFileOne: a name without a wildcard is the file, then RMS$_NMF;
// with NOWILD, the file on every call, and a wildcard is LIB$_NOWILD.
func TestFindFileOne(t *testing.T) {
	f := newFinder(t, findFixture(t))

	f.expect("DUA0:[000000]A.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.expect("DUA0:[000000]A.TXT", "", "", 0, rmsNMF, "")

	for range 3 {
		f.expect("DUA0:[000000]B.TXT", "", "", findNoWild, rmsNormal, "DUA0:[000000]B.TXT;1")
	}

	f.expect("DUA0:[000000]*.TXT", "", "", findNoWild, libNoWild, "")
	f.end()
}

// TestFindFileMissing: a missing file is RMS$_FNF, with FAB$L_STV in
// status-value; a call that starts a search writes $PARSE's (0), and one
// that goes on with it leaves status-value alone.
func TestFindFileMissing(t *testing.T) {
	f := newFinder(t, findFixture(t))

	f.expect("DUA0:[000000]NONE.TXT", "", "", 0, rmsFNF, "")

	if stv := longword(t, f.env, f.stv); stv != vmsdef.Symbols["SS$_NOSUCHFILE"] {
		t.Errorf("status-value = %#x, want SS$_NOSUCHFILE", stv)
	}

	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")

	if stv := longword(t, f.env, f.stv); stv != 0 {
		t.Errorf("status-value after a new search = %#x, want 0", stv)
	}

	f.expect("DUA0:[000000]*.TXT", "", "", 0, rmsNormal, "DUA0:[000000]B.TXT;1")

	if stv := longword(t, f.env, f.stv); stv != 0xDEAD {
		t.Errorf("status-value after a search went on = %#x, want it unchanged", stv)
	}

	f.end()
}

// TestFindFileDefaults: the default and related names fill in what the
// file name leaves out; with MULTIPLE the related name is the last file
// found instead of related-filespec.
func TestFindFileDefaults(t *testing.T) {
	f := newFinder(t, findFixture(t))

	f.expect("A", "DUA0:[000000].TXT", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.expect("C", "", "DUA0:[000000]X.DAT", 0, rmsNormal, "DUA0:[000000]C.DAT;1")

	f.expect("DUA0:[SUB]D.TXT", "", "", findMultiple, rmsNormal, "DUA0:[SUB]D.TXT;1")
	f.expect("DUA0:[000000]A", "", "DUA0:[SUB]X.DAT", findMultiple, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.expect("B", "", "DUA0:[SUB]X.DAT", findMultiple, rmsNormal, "DUA0:[000000]B.TXT;1")
	f.end()
}

// TestFindFileResult: a short fixed resultant string gets the name cut
// short, and the status is still $SEARCH's (no LIB$_STRTRU); a dynamic
// one gets the whole name.
func TestFindFileResult(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	cell := a.long(0)

	short := a.desc("          ")
	if st := call(t, env, "LIB$FIND_FILE", a.desc("DUA0:[000000]A.TXT"), short, cell); st != rmsNormal {
		t.Errorf("short result: status %#x, want RMS$_NORMAL", st)
	}

	if s := descText(t, env, short); s != "DUA0:[0000" {
		t.Errorf("short result = %q", s)
	}

	dyn := a.long(0)
	putDesc(t, env, dyn, 0, dscClassD, 0)
	a.long(0)

	if st := call(t, env, "LIB$FIND_FILE", a.desc("DUA0:[000000]C.DAT"), dyn, a.long(0)); st != rmsNormal {
		t.Errorf("dynamic result: status %#x", st)
	}

	if s := descText(t, env, dyn); s != "DUA0:[000000]C.DAT;1" {
		t.Errorf("dynamic result = %q", s)
	}
}

// TestFindFileContexts: FIND_FILE_END of 0 is SS$_NORMAL; a context
// longword that names no context is RMS$_FAB to both routines; a name of
// 256 characters is one of none (FNS is a byte): the default name's.
func TestFindFileContexts(t *testing.T) {
	env := findFixture(t)
	f := newFinder(t, env)

	if st := f.end(); st != ssNormal {
		t.Errorf("FIND_FILE_END of 0 = %#x", st)
	}

	f.expect("DUA0:[000000]A.TXT", "", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	old := longword(t, env, f.cell)
	f.end()

	putLongword(t, env, f.cell, old)

	if st := f.end(); st != rmsFAB {
		t.Errorf("FIND_FILE_END of an ended context = %#x, want RMS$_FAB", st)
	}

	f.expect("DUA0:[000000]A.TXT", "", "", 0, rmsFAB, "")

	putLongword(t, env, f.cell, 0)
	f.expect("DUA0:[000000]"+strings.Repeat("A", 243), "DUA0:[000000]A.TXT", "", 0, rmsNormal, "DUA0:[000000]A.TXT;1")
	f.end()
	f.expect("DUA0:[000000]B.TXT", "", "", 0, rmsNormal, "DUA0:[000000]B.TXT;1")
	f.end()
}

package librtl

import (
	"errors"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

var rmsDNF = vmsdef.Symbols["RMS$_DNF"]

// The stub frame callingBack runs a routine in, as the XFC would find it.
const (
	callbackFP = 0xF0000
	callbackSP = 0xEF000
	callbackPC = 0x2010
)

// procedure stands for a VAX procedure a routine calls back: given its
// entry mask's address and its arguments, it returns its R0.
type procedure func(routine uint32, args []uint32) uint32

// callingBack runs the routine registered as name, one that calls back
// into the program, through the shim dispatch as the XFC would, in a stub
// frame of its own. Each callback is answered by proc in Go, as if the
// procedure had run and returned to the XFC. It returns the routine's R0.
func callingBack(t *testing.T, env *corevms.Environment, a *arena, name string, proc procedure, argv ...uint32) uint32 {
	t.Helper()

	i := slices.IndexFunc(Routines, func(r Routine) bool { return r.Name == name })
	if i < 0 || !env.Shims().CallsBack(Routines[i].Code) {
		t.Fatalf("%s isn't a routine that calls back", name)
	}

	code := Routines[i].Code

	list := a.long(uint32(len(argv)))
	a.next += 4 * uint32(len(argv))

	for i, v := range argv {
		putLongword(t, env, list+4+4*uint32(i), v)
	}

	c := env.CPU()
	c.SetGPR(registers.AP, list)
	c.SetGPR(registers.FP, callbackFP)
	c.SetGPR(registers.SP, callbackSP)
	c.SetGPR(registers.PC, callbackPC)

	r0, _, err := env.Shim(code)

	var req *corevms.CallRequest
	for errors.As(err, &req) {
		n := longword(t, env, req.ArgList)
		args := make([]uint32, n)

		for i := range args {
			args[i] = longword(t, env, req.ArgList+4+4*uint32(i))
		}

		c.SetGPR(registers.R0, proc(req.Routine, args))
		r0, _, err = env.Shim(c.GPR(registers.R0))
	}

	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	if sp := c.GPR(registers.SP); sp != callbackSP {
		t.Errorf("%s left SP %#x, want %#x", name, sp, callbackSP)
	}

	return r0
}

// The addresses of the success and error routines the tests pass: any
// readable word, since the callbacks are answered in Go.
const (
	successRoutine = 0x3000
	errorRoutine   = 0x3100
)

// scanFAB builds a FAB and NAM in the arena, as the $FAB and $NAM macros
// would, for spec (and default name defaultSpec, "" for none), with
// 255-byte expanded and resultant buffers. It returns the FAB's address
// and the NAM's.
func scanFAB(t *testing.T, a *arena, spec, defaultSpec string) (fab, nam uint32) {
	t.Helper()

	env := a.env
	mem, cpu := env.Memory(), env.CPU()

	fab = a.next
	nam = fab + 80
	esa := nam + 96
	rsa := esa + fileBuffer
	text := rsa + fileBuffer
	a.next = text + 2*fileBuffer

	for addr := fab; addr < a.next; addr += 4 {
		putLongword(t, env, addr, 0)
	}

	for _, b := range []struct{ addr, v uint32 }{
		{fab + fabBID, fabBIDValue}, {fab + fabBLN, fabBLNValue},
		{fab + fabFNS, uint32(len(spec))}, {fab + fabDNS, uint32(len(defaultSpec))},
		{nam + namBID, namBIDValue}, {nam + namBLN, namBLNValue},
		{nam + namESS, maxFileSpec}, {nam + namRSS, maxFileSpec},
	} {
		if err := mem.StoreByte(cpu, b.addr, byte(b.v)); err != nil {
			t.Fatal(err)
		}
	}

	for _, l := range []struct{ addr, v uint32 }{
		{fab + fabNAM, nam}, {fab + fabFNA, text}, {fab + fabDNA, text + fileBuffer},
		{nam + namESA, esa}, {nam + namRSA, rsa},
	} {
		putLongword(t, env, l.addr, l.v)
	}

	for i, s := range []string{spec, defaultSpec} {
		for j := range len(s) {
			if err := mem.StoreByte(cpu, text+uint32(i)*fileBuffer+uint32(j), s[j]); err != nil {
				t.Fatal(err)
			}
		}
	}

	return fab, nam
}

// scanLog is what a scan's callbacks saw: "+name" for each success and
// "-name STS" for each error, the name the NAM's resultant string.
type scanLog []string

// recorder returns a procedure that logs each call to successRoutine and
// errorRoutine, checking each gets just the FAB.
func (l *scanLog) recorder(t *testing.T, env *corevms.Environment, fab, nam uint32) procedure {
	return func(routine uint32, args []uint32) uint32 {
		t.Helper()

		if len(args) != 1 || args[0] != fab {
			t.Errorf("callback arguments %#x, want the FAB %#x", args, fab)
		}

		name, err := resultantName(env, nam)
		if err != nil {
			t.Fatal(err)
		}

		switch routine {
		case successRoutine:
			*l = append(*l, "+"+name)
		case errorRoutine:
			*l = append(*l, "-"+name+" "+hexStatus(longword(t, env, fab+fabSTS)))
		default:
			t.Errorf("callback of %#x", routine)
		}

		return 0 // ignored either way
	}
}

func hexStatus(v uint32) string {
	const digits = "0123456789ABCDEF"

	s := ""
	for ; v != 0; v >>= 4 {
		s = string(digits[v&15]) + s
	}

	return "%X" + s
}

// fileScan runs LIB$FILE_SCAN on spec (with defaultSpec, and the context
// longword at cell when it isn't 0), returning its status and log.
func fileScan(t *testing.T, env *corevms.Environment, a *arena, spec, defaultSpec string, cell uint32) (uint32, scanLog, uint32) {
	t.Helper()

	fab, nam := scanFAB(t, a, spec, defaultSpec)

	var log scanLog

	argv := []uint32{fab, successRoutine, errorRoutine}
	if cell != 0 {
		argv = append(argv, cell)
	}

	st := callingBack(t, env, a, "LIB$FILE_SCAN", log.recorder(t, env, fab, nam), argv...)

	return st, log, fab
}

// TestFileScanWildcard: the success routine is called for each file a
// wildcard name matches, with the FAB; the status is RMS$_NMF; a
// wildcard directory walks the tree.
func TestFileScanWildcard(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	cases := []struct {
		spec string
		want scanLog
	}{
		{"DUA0:[000000]*.TXT", scanLog{"+DUA0:[000000]A.TXT;1", "+DUA0:[000000]B.TXT;1"}},
		{"DUA0:[000000...]*.TXT", scanLog{"+DUA0:[000000]A.TXT;1", "+DUA0:[000000]B.TXT;1", "+DUA0:[SUB]D.TXT;1"}},
		{"DUA0:[SUB]D.TXT", scanLog{"+DUA0:[SUB]D.TXT;1"}},
	}

	for _, c := range cases {
		st, log, _ := fileScan(t, env, a, c.spec, "", 0)
		if st != rmsNMF || !slices.Equal(log, c.want) {
			t.Errorf("FILE_SCAN(%q) = %#x %q, want RMS$_NMF %q", c.spec, st, log, c.want)
		}
	}
}

// TestFileScanErrors: the error routine is called for a file not found
// (the search then runs out, RMS$_NMF), for a $PARSE that fails (its
// status returned), and for a search that can't go on.
func TestFileScanErrors(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	st, log, _ := fileScan(t, env, a, "DUA0:[000000]NONE.TXT", "", 0)
	if want := (scanLog{"-DUA0:[000000]NONE.TXT; " + hexStatus(rmsFNF)}); st != rmsNMF || !slices.Equal(log, want) {
		t.Errorf("a missing file: %#x %q, want RMS$_NMF %q", st, log, want)
	}

	st, log, _ = fileScan(t, env, a, "DUA0:[NODIR]X.TXT", "", 0)
	if st != rmsDNF || len(log) != 1 || log[0][0] != '-' {
		t.Errorf("a missing directory: %#x %q, want RMS$_DNF and one error call", st, log)
	}

	st, log, _ = fileScan(t, env, a, "DUA0:[000000]A.TXT", ";;;", 0)
	if st&1 != 0 || len(log) != 1 || log[0][0] != '-' {
		t.Errorf("a bad default name: %#x %q, want a failure and one error call", st, log)
	}
}

// TestFileScanNoRoutines: a success or error routine of 0 isn't called.
func TestFileScanNoRoutines(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	fab, _ := scanFAB(t, a, "DUA0:[000000]*.*", "")

	called := false
	proc := func(uint32, []uint32) uint32 { called = true; return 1 }

	if st := callingBack(t, env, a, "LIB$FILE_SCAN", proc, fab, 0, 0); st != rmsNMF || called {
		t.Errorf("FILE_SCAN with no routines = %#x (called %v), want RMS$_NMF, no calls", st, called)
	}
}

// TestFileScanContext: with a context, the last file found is the next
// call's related name (temporary defaulting), the program's NAM$L_RLF is
// 0 again after each call, and FILE_SCAN_END frees the context and
// clears the longword.
func TestFileScanContext(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	cell := a.long(0)

	st, log, _ := fileScan(t, env, a, "DUA0:[SUB]D.TXT", "", cell)
	if st != rmsNMF || len(log) != 1 {
		t.Fatalf("first FILE_SCAN = %#x %q", st, log)
	}

	if longword(t, env, cell) == 0 {
		t.Fatal("the context longword is still 0")
	}

	st, log, fab := fileScan(t, env, a, "*.TXT", "", cell)
	if want := (scanLog{"+DUA0:[SUB]D.TXT;1"}); st != rmsNMF || !slices.Equal(log, want) {
		t.Errorf("second FILE_SCAN = %#x %q, want %q from the related name", st, log, want)
	}

	nam := longword(t, env, fab+fabNAM)
	if rlf := longword(t, env, nam+namRLF); rlf != 0 {
		t.Errorf("NAM$L_RLF after the call = %#x, want 0", rlf)
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", fab, cell); st != ssNormal {
		t.Errorf("FILE_SCAN_END = %#x", st)
	}

	if c := longword(t, env, cell); c != 0 {
		t.Errorf("context after FILE_SCAN_END = %#x, want 0", c)
	}

	if fns := byteAt(t, env, fab+fabFNS); fns != byte(len("*.TXT")) {
		t.Errorf("FAB$B_FNS after FILE_SCAN_END = %d, want it put back", fns)
	}
}

// TestFileScanEnd: FILE_SCAN_END's arguments: neither, a context of 0, a
// context longword that names none (LIB$_INVARG), a block that isn't a
// FAB (RMS$_FAB, nothing changed); and its $PARSE of the null string
// ends a wildcard search left part way.
func TestFileScanEnd(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	if st := call(t, env, "LIB$FILE_SCAN_END"); st != ssNormal {
		t.Errorf("FILE_SCAN_END() = %#x", st)
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", 0, a.long(0)); st != ssNormal {
		t.Errorf("FILE_SCAN_END(, 0) = %#x", st)
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", 0, a.long(0x12340)); st != libInvArg {
		t.Errorf("FILE_SCAN_END(, junk) = %#x, want LIB$_INVARG", st)
	}

	notFAB := a.long(0x55)
	if st := call(t, env, "LIB$FILE_SCAN_END", notFAB); st != rmsFAB || longword(t, env, notFAB) != 0x55 {
		t.Errorf("FILE_SCAN_END(not a FAB) = %#x, want RMS$_FAB and the block unchanged", st)
	}

	// A search with a wildcard context, stopped after one file.
	fab, nam := scanFAB(t, a, "DUA0:[000000...]*.TXT", "")

	for _, service := range []string{"SYS$PARSE", "SYS$SEARCH"} {
		if st, err := env.CallService(service, []uint32{fab}); err != nil || st&1 == 0 {
			t.Fatalf("%s = %#x, %v", service, st, err)
		}
	}

	if longword(t, env, nam+namWCC) == 0 {
		t.Fatal("the search has no context")
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", fab); st != ssNormal {
		t.Errorf("FILE_SCAN_END(fab) = %#x", st)
	}

	if wcc := longword(t, env, nam+namWCC); wcc != 0 {
		t.Errorf("NAM$L_WCC after FILE_SCAN_END = %#x, want 0", wcc)
	}
}

func byteAt(t *testing.T, env *corevms.Environment, addr uint32) byte {
	t.Helper()

	v, err := env.Memory().LoadByte(env.CPU(), addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

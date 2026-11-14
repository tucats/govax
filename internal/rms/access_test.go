package rms

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// Phase 93's subtask 8: RMS's services and the ACP checked against the
// caller's credentials (access.go), by Decision 5's table, each rule one
// the probe's VMS 7.1 run gave (A.4). console's TestProt93ProgramOracle
// holds the services to the probe's own runs; these need no local volume.

// accessFixture is a fresh volume whose directories, made by SYSTEM, are
// owned by PROT93A ([200,1]), and a Context on it, for the services.
type accessFixture struct {
	t   *testing.T
	s   *Session
	vol *volume.Volume
	ctx *Context
}

func newAccessFixture(t *testing.T, dirs ...string) *accessFixture {
	t.Helper()

	s, vol := profileSession(t, uicA, dirs...)

	ctx := &Context{
		Mem:      vm.NewMemory(1 << 20),
		CPU:      registers.New(),
		Mounts:   s.Mounts,
		Files:    NewFileTable(nil),
		Logicals: newTestLogicals(t),
		Console:  &bytes.Buffer{},
	}

	return &accessFixture{t: t, s: s, vol: vol, ctx: ctx}
}

// split is a spec's directories and NAME.TYP, "DUA0:[A.B]X.DAT".
func split(spec string) ([]string, string) {
	spec = strings.TrimPrefix(spec, "DUA0:")
	i := strings.IndexByte(spec, ']')

	var dirs []string
	if i > 0 {
		dirs = strings.Split(spec[1:i], ".")
	}

	return dirs, spec[i+1:]
}

// file makes spec as SYSTEM, then gives it owner and protection.
func (f *accessFixture) file(spec string, owner security.UIC, protection uint16) {
	f.t.Helper()

	makeFile(f.t, f.s, security.SystemCredentials(), spec)

	dirs, name := split(spec)
	changeHeader(f.t, f.vol, dirs, name, func(h *ondisk.FileHeader) {
		h.Owner, h.FileProtection = diskUIC(owner), protection
	})
}

// protect gives the directory dirs protection.
func (f *accessFixture) protect(dirs []string, protection uint16) {
	f.t.Helper()

	changeHeader(f.t, f.vol, dirs, "", func(h *ondisk.FileHeader) { h.FileProtection = protection })
}

// as makes the services act for creds.
func (f *accessFixture) as(creds security.Credentials) {
	f.ctx.Credentials = func() security.Credentials { return creds }
}

// fab writes a fresh FAB for spec with access fac and options fop.
func (f *accessFixture) fab(spec string, fac byte, fop uint32) {
	f.t.Helper()

	for i := uint32(0); i < 0x50; i++ {
		putByte(f.t, f.ctx, testFabAddr+i, 0)
	}

	newFAB(f.t, f.ctx, spec)
	putByte(f.t, f.ctx, testFabAddr+vmsConst("FAB$B_BID"), byte(vmsConst("FAB$C_BID")))
	putByte(f.t, f.ctx, testFabAddr+vmsConst("FAB$B_BLN"), byte(vmsConst("FAB$C_BLN")))
	putByte(f.t, f.ctx, testFabAddr+fabFAC, fac)
	putLongwordAt(f.t, f.ctx, testFabAddr+fabFOP, fop)
}

// call runs service on the FAB and returns its status and STV, closing
// a file it opened.
func (f *accessFixture) call(service func(*Context, []uint32) (uint32, error)) (uint32, uint32) {
	f.t.Helper()

	r0, err := service(f.ctx, []uint32{testFabAddr})
	if err != nil {
		f.t.Fatal(err)
	}

	stv := readLongword(f.t, f.ctx, testFabAddr+fabSTV)

	if r0&1 != 0 && readWord(f.t, f.ctx, testFabAddr+fabIFI) != 0 {
		if _, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
			f.t.Fatal(err)
		}
	}

	return r0, stv
}

func (f *accessFixture) open(spec string, fac byte) (uint32, uint32) {
	f.fab(spec, fac, 0)

	return f.call(SysOpen)
}

func (f *accessFixture) create(spec string, fop uint32) (uint32, uint32) {
	f.fab(spec, facPut, fop)

	return f.call(SysCreate)
}

func (f *accessFixture) erase(spec string) (uint32, uint32) {
	f.fab(spec, 0, 0)

	return f.call(SysErase)
}

// refused checks that sts and stv are RMS$_PRV and SS$_NOPRIV.
func refused(t *testing.T, what string, sts, stv uint32) {
	t.Helper()

	if sts != rmsPrivilegeViolation || stv != ssNoPriv {
		t.Errorf("%s: %#x, STV %#x; want RMS$_PRV, SS$_NOPRIV", what, sts, stv)
	}
}

// allowed checks that sts is a success.
func allowed(t *testing.T, what string, sts, stv uint32) {
	t.Helper()

	if sts&1 == 0 {
		t.Errorf("%s: %#x, STV %#x; want a success", what, sts, stv)
	}
}

// Protection masks: a set bit denies. Each directory gives everyone what
// its name says ("World" the same as the other three categories).
const (
	protNone uint16 = 0xFFFF
	protAll  uint16 = 0x0000
)

// everyone is the mask giving every category access a (R, W, E, D bits
// of security.Access).
func everyone(a security.Access) uint16 {
	field := uint16(0xF) &^ uint16(a)

	return field | field<<4 | field<<8 | field<<12
}

// A directory's access decides what the services may do in it (A.4,
// "Directories"): a name looked up needs R or E, a create or an erase R
// and W.
func TestDirectoryAccess(t *testing.T) {
	r, w, e := security.Read, security.Write, security.Execute

	cases := []struct {
		dir                  string
		access               security.Access
		lookup, create, kill bool
	}{
		{"DR", r, true, false, false},
		{"DE", e, true, false, false},
		{"DWE", w | e, true, false, false},
		{"DRW", r | w, true, true, true},
		{"DRWE", r | w | e, true, true, true},
		{"DW", w, false, false, false},
		{"DN", 0, false, false, false},
	}

	var names []string
	for _, c := range cases {
		names = append(names, c.dir)
	}

	f := newAccessFixture(t, names...)

	for _, c := range cases {
		f.as(security.SystemCredentials())
		f.file("DUA0:["+c.dir+"]A.DAT", uicA, protAll)
		f.file("DUA0:["+c.dir+"]GONE.DAT", uicA, protAll)
		f.protect([]string{c.dir}, everyone(c.access))

		f.as(account(uicC, 0))

		check := func(what string, want bool, sts, stv uint32) {
			t.Helper()

			if want {
				allowed(t, c.dir+" "+what, sts, stv)
			} else {
				refused(t, c.dir+" "+what, sts, stv)
			}
		}

		sts, stv := f.open("DUA0:["+c.dir+"]A.DAT", facGet)
		check("$OPEN", c.lookup, sts, stv)

		sts, stv = f.create("DUA0:["+c.dir+"]NEW.DAT", 0)
		check("$CREATE", c.create, sts, stv)

		sts, stv = f.erase("DUA0:[" + c.dir + "]GONE.DAT")
		check("$ERASE", c.kill, sts, stv)
	}
}

// Every level above the file's directory is looked up with a check (A.4,
// answer 1): a path an upper level of which gives no access is refused,
// even when the file's own directory gives everything; $PARSE checks the
// levels above the last.
func TestPathLevels(t *testing.T) {
	f := newAccessFixture(t, "P")

	if _, err := f.s.CreateDirectory("DUA0:[P.Q.R]", CreateDirectoryOptions{}); err != nil {
		t.Fatal(err)
	}

	f.file("DUA0:[P.Q.R]X.DAT", uicA, protAll)
	f.protect([]string{"P", "Q"}, protAll)
	f.protect([]string{"P", "Q", "R"}, protAll)
	f.protect([]string{"P"}, protNone)

	f.as(account(uicC, 0))

	sts, stv := f.open("DUA0:[P.Q.R]X.DAT", facGet)
	refused(t, "$OPEN through [P]", sts, stv)

	f.fab("DUA0:[P.Q.R]X.DAT", facGet, 0)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("NAM"), 0x5000)
	putByte(t, f.ctx, 0x5000+namBID, byte(vmsConst("NAM$C_BID")))
	putByte(t, f.ctx, 0x5000+namBLN, byte(vmsConst("NAM$C_BLN")))
	putLongwordAt(t, f.ctx, 0x5000+namESA, 0x5100)
	putByte(t, f.ctx, 0x5000+namESS, 255)

	sts, stv = f.call(SysParse)
	refused(t, "$PARSE through [P]", sts, stv)

	// The last level giving nothing: $PARSE is allowed, $OPEN isn't.
	f.as(security.SystemCredentials())
	f.protect([]string{"P"}, protAll)
	f.protect([]string{"P", "Q", "R"}, protNone)
	f.as(account(uicC, 0))

	if sts, stv = f.call(SysParse); sts&1 == 0 {
		t.Errorf("$PARSE with [P.Q.R] giving nothing: %#x, STV %#x", sts, stv)
	}

	sts, stv = f.open("DUA0:[P.Q.R]X.DAT", facGet)
	refused(t, "$OPEN in [P.Q.R]", sts, stv)

	// SYSTEM without its privileges is in the system category, which the
	// code gives; with them, everything.
	f.as(security.SystemCredentials())

	sts, stv = f.open("DUA0:[P.Q.R]X.DAT", facGet)
	allowed(t, "$OPEN as SYSTEM", sts, stv)
}

// A file's protection decides an open (A.4, "The check"): read needs R,
// write R and W, so a file giving W alone opens for neither.
func TestFileAccess(t *testing.T) {
	f := newAccessFixture(t, "F")

	r, w := security.Read, security.Write
	f.file("DUA0:[F]R.DAT", uicA, everyone(r))
	f.file("DUA0:[F]W.DAT", uicA, everyone(w))
	f.file("DUA0:[F]RW.DAT", uicA, everyone(r|w))

	f.as(account(uicC, 0))

	for _, c := range []struct {
		name     string
		fac      byte
		expected bool
	}{
		{"R.DAT", facGet, true},
		{"R.DAT", facPut, false},
		{"W.DAT", facGet, false},
		{"W.DAT", facPut, false},
		{"RW.DAT", facGet | facPut, true},
	} {
		sts, stv := f.open("DUA0:[F]"+c.name, c.fac)
		if c.expected {
			allowed(t, c.name, sts, stv)
		} else {
			refused(t, c.name, sts, stv)
		}
	}

	// READALL reads, and adds nothing else.
	f.as(account(uicC, security.PrivREADALL))

	if sts, stv := f.open("DUA0:[F]W.DAT", facGet); sts&1 == 0 {
		t.Errorf("READALL reading W.DAT: %#x, %#x", sts, stv)
	}

	sts, stv := f.open("DUA0:[F]R.DAT", facPut)
	refused(t, "READALL writing R.DAT", sts, stv)
}

// A new version needs W on the highest version, and D on each version the
// version limit would take (A.4, answer 3; the probe's V, W, and L files),
// the limit being the name's directory record's, as VMS's is.
func TestNewVersionPastLimit(t *testing.T) {
	f := newAccessFixture(t, "NV")

	r, w, d := security.Read, security.Write, security.Delete
	f.file("DUA0:[NV]V.DAT", uicA, everyone(r))
	f.file("DUA0:[NV]W.DAT", uicA, everyone(r|w))
	f.file("DUA0:[NV]L.DAT", uicA, everyone(r|w))
	f.file("DUA0:[NV]L.DAT", uicA, everyone(r|w|d))

	dir := openEntry(t, f.vol, []string{"NV"}, "")
	nv, err := f.vol.OpenDirectory(dir.Header.Fid)
	if err != nil {
		t.Fatal(err)
	}

	if err := nv.SetNameVersionLimit("L.DAT", 2); err != nil {
		t.Fatal(err)
	}

	f.as(account(uicC, 0))

	sts, stv := f.create("DUA0:[NV]V.DAT", 0)
	refused(t, "over a version giving R", sts, stv)

	sts, stv = f.create("DUA0:[NV]W.DAT", 0)
	allowed(t, "over a version giving RW", sts, stv)

	sts, stv = f.create("DUA0:[NV]L.DAT", 0)
	refused(t, "past the limit, the oldest without D", sts, stv)
}

// FOP SUP replaces a version that gives W and D, the new file the
// creator's with the old file's protection, RMS$_SUPERSEDE; one without
// D is refused (A.4, answer 3).
func TestSupersede(t *testing.T) {
	f := newAccessFixture(t, "S")

	r, w, d := security.Read, security.Write, security.Delete
	f.file("DUA0:[S]NOD.DAT", uicA, everyone(r|w))
	f.file("DUA0:[S]OK.DAT", uicA, everyone(r|w|d))

	f.as(account(uicC, 0))

	sts, stv := f.create("DUA0:[S]NOD.DAT;1", fopSUP)
	refused(t, "a supersede without D", sts, stv)

	if sts, stv = f.create("DUA0:[S]OK.DAT;1", fopSUP); sts != rmsSupersede {
		t.Fatalf("a supersede allowed: %#x, %#x; want RMS$_SUPERSEDE", sts, stv)
	}

	if owner, prot, _ := profileOf(t, f.vol, []string{"S"}, "OK.DAT"); owner != uicC || prot != everyone(r|w|d) {
		t.Errorf("superseding file: owner %v, protection %#x", owner, prot)
	}
}

// A rename needs D on the file and R and W on both directories: RMS$_RMV
// for the old side, RMS$_ENT for the new, each with SS$_NOPRIV, the old
// NAM told the file's name (A.4, answer 2).
func TestRenameProtection(t *testing.T) {
	f := newAccessFixture(t, "RN", "RNX")

	r, w, e, d := security.Read, security.Write, security.Execute, security.Delete
	f.file("DUA0:[RN]NOD.DAT", uicA, everyone(r|w))
	f.file("DUA0:[RN]OK.DAT", uicA, everyone(r|w|d))
	f.file("DUA0:[RN]OUT.DAT", uicA, everyone(r|w|d))
	f.protect([]string{"RNX"}, everyone(r|e))

	f.as(account(uicC, 0))

	for _, c := range []struct {
		from, to string
		sts      uint32
	}{
		{"DUA0:[RN]NOD.DAT", "DUA0:[RN]NOD2.DAT", rmsRemoveFailed},
		{"DUA0:[RN]OUT.DAT", "DUA0:[RNX]OUT2.DAT", rmsEnterFailed},
		{"DUA0:[RN]OK.DAT", "DUA0:[RN]OK2.DAT", rmsNormal},
	} {
		_, sts, stv := renameText(f.ctx, c.from, c.to)

		want := ssNoPriv
		if c.sts == rmsNormal {
			want = 0
		}

		if sts != c.sts || stv != want {
			t.Errorf("%s to %s: %#x, STV %#x; want %#x, %#x", c.from, c.to, sts, stv, c.sts, want)
		}
	}
}

// The ACP checks the caller's credentials as RMS does (A.4, "The check"):
// a lookup needs R or E on the directory, an access R (and W), reading the
// attributes R or C, changing the protection C and the owner the owner
// rule, a delete D and the directory's R and W, a create R and W on it.
func TestACPProtection(t *testing.T) {
	f := newAccessFixture(t, "Q", "QN")

	r, w := security.Read, security.Write
	f.file("DUA0:[Q]NONE.DAT", uicA, protNone)
	f.file("DUA0:[Q]R.DAT", uicA, everyone(r))
	f.file("DUA0:[QN]A.DAT", uicA, protAll)
	f.protect([]string{"QN"}, everyone(w))

	m := f.s.Mounts
	q := fileIDFrom(openEntry(t, f.vol, []string{"Q"}, "").Header.Fid)
	qn := fileIDFrom(openEntry(t, f.vol, []string{"QN"}, "").Header.Fid)

	world, owner := account(uicC, 0), account(uicA, 0)

	if _, _, err := m.ACPLookupAs(world, "DUA0", qn, "A.DAT"); err != ErrNoPrivilege {
		t.Errorf("lookup in a directory giving W: %v", err)
	}

	none, _, err := m.ACPLookupAs(world, "DUA0", q, "NONE.DAT")
	if err != nil {
		t.Fatal(err)
	}

	readable, _, err := m.ACPLookupAs(world, "DUA0", q, "R.DAT")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		what string
		err  error
		want error
	}{
		{"attributes of NONE as the world", func() error { _, err := m.ACPReadAttributesAs(world, "DUA0", none); return err }(), ErrNoPrivilege},
		{"attributes of NONE as its owner", func() error { _, err := m.ACPReadAttributesAs(owner, "DUA0", none); return err }(), nil},
		{"access R.DAT for read", func() error { _, err := m.ACPAccessWith("DUA0", readable, ACPAccessMode{Credentials: &world}); return err }(), nil},
		{"access R.DAT for write", func() error {
			_, err := m.ACPAccessWith("DUA0", readable, ACPAccessMode{Write: true, Credentials: &world})
			return err
		}(), ErrNoPrivilege},
		{"protect R.DAT as the world", m.ACPWriteAttributesAs(world, "DUA0", readable, func(a *ACPAttributes) { a.Protection = 0 }), ErrNoPrivilege},
		{"protect R.DAT as its owner", m.ACPWriteAttributesAs(owner, "DUA0", readable, func(a *ACPAttributes) { a.Protection = everyone(r) }), nil},
		{"give R.DAT away as its owner", m.ACPWriteAttributesAs(owner, "DUA0", readable, func(a *ACPAttributes) { a.Owner = uint32(uicC) }), ErrNoPrivilege},
		{"delete R.DAT as the world", func() error {
			_, err := m.ACPDelete("DUA0", ACPDeleteRequest{Directory: q, Name: "R.DAT", DeleteFile: true, Credentials: &world})
			return err
		}(), ErrNoPrivilege},
		{"create in [QN] as the world", func() error {
			_, err := m.ACPCreate("DUA0", ACPCreateRequest{Directory: qn, Name: "NEW.DAT", Credentials: &world})
			return err
		}(), ErrNoPrivilege},
	} {
		if c.err != c.want {
			t.Errorf("%s: %v, want %v", c.what, c.err, c.want)
		}
	}
}

// SET DEBUG PROTECTION (security.Credentials.Trace) sees every check,
// SYSTEM's too, which BYPASS would otherwise let skip them.
func TestProtectionTrace(t *testing.T) {
	f := newAccessFixture(t, "T")
	f.file("DUA0:[T]X.DAT", uicA, protNone)

	var lines []string

	creds := security.SystemCredentials()
	creds.Trace = func(line string) { lines = append(lines, line) }
	f.as(creds)

	sts, stv := f.open("DUA0:[T]X.DAT", facGet)
	allowed(t, "SYSTEM", sts, stv)

	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"[1,4] wants R of 000000.DIR",
		"[1,4] wants R of X.DAT;1 owner [200,1] (S,O,G,W) ACL 0: granted, has RWEDC (BYPASS",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
}

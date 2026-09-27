package rtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// arena hands out VAX memory for one test's arguments: descriptors,
// buffers, longwords and item lists, each at a fresh address.
type arena struct {
	t    *testing.T
	env  *Environment
	next uint32
}

func newArena(t *testing.T, env *Environment) *arena {
	return &arena{t: t, env: env, next: 0x10000}
}

func (a *arena) alloc(n uint32) uint32 {
	addr := a.next
	a.next += (n + 15) &^ 7

	return addr
}

// desc returns the address of a descriptor for s.
func (a *arena) desc(s string) uint32 {
	d := a.alloc(8)
	putDescriptor(a.t, a.env, d, a.alloc(uint32(len(s))+1), s)

	return d
}

// outDesc returns the address of a descriptor for an n-byte output
// buffer, and the buffer's address.
func (a *arena) outDesc(n uint16) (uint32, uint32) {
	d, buf := a.alloc(8), a.alloc(uint32(n))
	putWord(a.t, a.env, d, n)
	putLongword(a.t, a.env, d+4, buf)

	return d, buf
}

func (a *arena) long(v uint32) uint32 {
	addr := a.alloc(4)
	putLongword(a.t, a.env, addr, v)

	return addr
}

func (a *arena) byteArg(v byte) uint32 {
	addr := a.alloc(1)
	putByte(a.t, a.env, addr, v)

	return addr
}

func (a *arena) str(s string) uint32 {
	addr := a.alloc(uint32(len(s)))
	putBytes(a.t, a.env, addr, []byte(s))

	return addr
}

type item struct {
	code   uint16
	buflen uint16
	buf    uint32
	ret    uint32
}

// items writes an item list (terminated by a zero longword) and returns
// its address.
func (a *arena) items(list ...item) uint32 {
	addr := a.alloc(uint32(12*len(list) + 4))

	for i, it := range list {
		p := addr + uint32(12*i)
		putWord(a.t, a.env, p, it.buflen)
		putWord(a.t, a.env, p+2, it.code)
		putLongword(a.t, a.env, p+4, it.buf)
		putLongword(a.t, a.env, p+8, it.ret)
	}

	putLongword(a.t, a.env, addr+uint32(12*len(list)), 0)

	return addr
}

func (a *arena) readString(addr uint32, n uint16) string {
	return string(readBytes(a.t, a.env, addr, int(n)))
}

func (a *arena) readLong(addr uint32) uint32 {
	v, err := a.env.mem.LoadLongword(a.env.cpu, addr)
	if err != nil {
		a.t.Fatal(err)
	}

	return v
}

func (a *arena) readByte(addr uint32) byte {
	return readBytes(a.t, a.env, addr, 1)[0]
}

func callLNM(t *testing.T, env *Environment, fn ServiceFunc, argv ...uint32) uint32 {
	t.Helper()

	r0, err := fn(env, argv)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	return r0
}

func wantR0(t *testing.T, got, want uint32) {
	t.Helper()

	if got != want {
		t.Errorf("R0 = %#x (%v), want %#x (%v)", got, vmserrors.New(got), want, vmserrors.New(want))
	}
}

func defineLogical(t *testing.T, env *Environment, table, name string, mode lnm.Mode, attrs uint32, values ...string) {
	t.Helper()

	eqv := make([]lnm.Equivalence, len(values))
	for i, v := range values {
		eqv[i] = lnm.Equivalence{Value: v, Attrs: attrs}
	}

	if _, err := env.Logicals.Define(table, name, mode, 0, eqv); err != nil {
		t.Fatalf("Define(%s): %v", name, err)
	}
}

func TestSysTrnlnm_itemCodes(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	defineLogical(t, env, "LNM$SYSTEM", "FIFI", lnm.Executive, lnm.AttrConcealed, "DISK1:[FRED]", "DISK2:")

	str0, len0, str1, ret0, ret1 := a.alloc(64), a.alloc(2), a.alloc(64), a.alloc(2), a.alloc(2)
	attr0, attr1, length1, maxIndex, mode := a.alloc(4), a.alloc(4), a.alloc(4), a.alloc(4), a.alloc(1)
	table, tableLen := a.alloc(32), a.alloc(2)

	list := a.items(
		item{code: lnmString, buflen: 64, buf: str0, ret: ret0},
		item{code: lnmAttributes, buflen: 4, buf: attr0},
		item{code: lnmMaxIndex, buflen: 4, buf: maxIndex},
		item{code: lnmACMode, buflen: 1, buf: mode},
		item{code: lnmTable, buflen: 32, buf: table, ret: tableLen},
		item{code: lnmIndex, buflen: 4, buf: a.long(1)},
		item{code: lnmString, buflen: 64, buf: str1, ret: ret1},
		item{code: lnmLength, buflen: 4, buf: length1, ret: len0},
		item{code: lnmAttributes, buflen: 4, buf: attr1},
	)

	r0 := callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("FIFI"), 0, list)
	wantR0(t, r0, ssNormal)

	if got := a.readString(str0, readWordEnv(t, env, ret0)); got != "DISK1:[FRED]" {
		t.Errorf("index 0 string = %q", got)
	}

	if got := a.readString(str1, readWordEnv(t, env, ret1)); got != "DISK2:" {
		t.Errorf("index 1 string = %q", got)
	}

	if got := a.readLong(length1); got != 6 {
		t.Errorf("index 1 length = %d", got)
	}

	if got := a.readLong(maxIndex); got != 1 {
		t.Errorf("max index = %d", got)
	}

	if got, want := a.readLong(attr0), lnm.AttrConcealed|lnm.AttrExists; got != want {
		t.Errorf("attributes = %#x, want %#x", got, want)
	}

	if got := a.readByte(mode); got != byte(lnm.Executive) {
		t.Errorf("acmode = %d", got)
	}

	if got := a.readString(table, readWordEnv(t, env, tableLen)); got != lnm.SystemTableName {
		t.Errorf("table = %q", got)
	}

	// Past the last index: no EXISTS (and no CONCEALED, which belongs to
	// each equivalence string), and a zero return length.
	missing, missingRet, attr := a.alloc(8), a.alloc(2), a.alloc(4)
	list = a.items(
		item{code: lnmIndex, buflen: 4, buf: a.long(5)},
		item{code: lnmString, buflen: 8, buf: missing, ret: missingRet},
		item{code: lnmAttributes, buflen: 4, buf: attr},
	)
	putWord(t, env, missingRet, 99)

	wantR0(t, callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("FIFI"), 0, list), ssNormal)

	if readWordEnv(t, env, missingRet) != 0 || a.readLong(attr) != 0 {
		t.Errorf("index 5: length %d, attributes %#x", readWordEnv(t, env, missingRet), a.readLong(attr))
	}
}

func TestSysTrnlnm_statuses(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	defineLogical(t, env, "LNM$PROCESS", "MYNAME", lnm.User, 0, "hello")

	short, shortRet := a.alloc(8), a.alloc(2)

	tests := []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"found", []uint32{0, a.desc("LNM$PROCESS_TABLE"), a.desc("MYNAME"), 0, 0}, ssNormal},
		{"case blind", []uint32{a.long(lnm.AttrCaseBlind), a.desc("LNM$FILE_DEV"), a.desc("myname"), 0, 0}, ssNormal},
		{"case matters", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("myname"), 0, 0}, vmserrors.SS_NOLOGNAM},
		{"acmode hides user name", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("MYNAME"), a.byteArg(2), 0}, vmserrors.SS_NOLOGNAM},
		{"no such table", []uint32{0, a.desc("NOSUCHTABLE"), a.desc("MYNAME"), 0, 0}, vmserrors.SS_NOLOGTAB},
		{"no table argument", []uint32{0, 0, a.desc("MYNAME"), 0, 0}, ssBadParam},
		{"empty name", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc(""), 0, 0}, ssIvLogNam},
		{"bad attr", []uint32{a.long(lnm.AttrTerminal), a.desc("LNM$FILE_DEV"), a.desc("MYNAME"), 0, 0}, ssBadParam},
		{"too few args", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("MYNAME")}, ssInsfArg},
		{"buffer overflow", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("MYNAME"), 0,
			a.items(item{code: lnmString, buflen: 3, buf: short, ret: shortRet})}, ssBufferOvf},
		{"bad item code", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("MYNAME"), 0,
			a.items(item{code: 99, buflen: 4, buf: a.alloc(4)})}, ssBadParam},
		{"bad index", []uint32{0, a.desc("LNM$FILE_DEV"), a.desc("MYNAME"), 0,
			a.items(item{code: lnmIndex, buflen: 4, buf: a.long(128)})}, ssBadParam},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantR0(t, callLNM(t, env, serviceSysTrnlnm, tt.argv...), tt.want)
		})
	}

	if got := a.readString(short, readWordEnv(t, env, shortRet)); got != "hel" {
		t.Errorf("overflowed string = %q, want hel", got)
	}
}

func TestSysTrnlnm_chain(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	str, ret := a.alloc(16), a.alloc(2)
	second := a.items(item{code: lnmString, buflen: 16, buf: str, ret: ret})
	first := a.items(item{code: lnmChain, buf: second})

	wantR0(t, callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("TT"), 0, first), ssNormal)

	if got := a.readString(str, readWordEnv(t, env, ret)); got != "_TTA0:" {
		t.Errorf("chained string = %q", got)
	}

	// A chain that loops forever is refused.
	loop := a.alloc(16)
	putWord(t, env, loop, 0)
	putWord(t, env, loop+2, lnmChain)
	putLongword(t, env, loop+4, loop)

	wantR0(t, callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("TT"), 0, loop), ssBadParam)
}

func TestSysTrnlnm_debugTrace(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	var buf bytes.Buffer

	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugLogicals)
	callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("SYS$OUTPUT"), 0, 0)

	if !strings.Contains(buf.String(), `DEBUG: $TRNLNM(LNM$FILE_DEV,SYS$OUTPUT), value="_TTA0:"`) {
		t.Errorf("trace = %q", buf.String())
	}

	buf.Reset()
	env.cpu.SetDebug(0)
	callLNM(t, env, serviceSysTrnlnm, 0, a.desc("LNM$FILE_DEV"), a.desc("SYS$OUTPUT"), 0, 0)

	if buf.Len() != 0 {
		t.Errorf("trace with DEBUG LOGICALS clear = %q", buf.String())
	}
}

func TestSysCrelnm(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	table, tableRet := a.alloc(32), a.alloc(2)
	list := a.items(
		item{code: lnmTable, buflen: 32, buf: table, ret: tableRet},
		item{code: lnmString, buflen: 5, buf: a.str("DUA0:")},
		item{code: lnmAttributes, buflen: 4, buf: a.long(lnm.AttrConcealed | lnm.AttrTerminal)},
		item{code: lnmString, buflen: 5, buf: a.str("DUA1:")},
	)

	r0 := callLNM(t, env, serviceSysCrelnm, a.long(lnm.AttrNoAlias), a.desc("LNM$FILE_DEV"), a.desc("BOTH"), a.byteArg(1), list)
	wantR0(t, r0, ssNormal)

	e, err := env.Logicals.Translate("LNM$FILE_DEV", "BOTH", lnm.User, 0)
	if err != nil {
		t.Fatal(err)
	}

	if e.Table.Name != lnm.ProcessTableName || e.Mode != lnm.Executive || e.Attrs != lnm.AttrNoAlias ||
		len(e.Equivalences) != 2 || e.Equivalences[0] != (lnm.Equivalence{Value: "DUA0:"}) ||
		e.Equivalences[1] != (lnm.Equivalence{Value: "DUA1:", Attrs: lnm.AttrConcealed | lnm.AttrTerminal}) {
		t.Errorf("BOTH = %+v in %s at %s, attrs %#x", e.Equivalences, e.Table.Name, e.Mode, e.Attrs)
	}

	if got := a.readString(table, readWordEnv(t, env, tableRet)); got != lnm.ProcessTableName {
		t.Errorf("LNM$_TABLE = %q", got)
	}

	// Redefining at the same mode supersedes; the default mode is the
	// caller's (kernel, in this fixture).
	one := a.items(item{code: lnmString, buflen: 3, buf: a.str("NEW")})
	wantR0(t, callLNM(t, env, serviceSysCrelnm, 0, a.desc("LNM$PROCESS"), a.desc("BOTH"), a.byteArg(1), one), ssSupersede)
	wantR0(t, callLNM(t, env, serviceSysCrelnm, 0, a.desc("LNM$PROCESS"), a.desc("K"), 0, one), ssNormal)

	if e, _ := env.Logicals.Translate("LNM$PROCESS", "K", lnm.User, 0); e == nil || e.Mode != lnm.Mode(env.cpu.PSL().CurMod()) {
		t.Errorf("K = %+v, want the caller's mode", e)
	}

	for _, tt := range []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"no strings", []uint32{0, a.desc("LNM$PROCESS"), a.desc("X"), 0, a.items()}, ssBadParam},
		{"bad attr", []uint32{a.long(lnm.AttrTerminal), a.desc("LNM$PROCESS"), a.desc("X"), 0, one}, ssBadParam},
		{"no such table", []uint32{0, a.desc("NOSUCH"), a.desc("X"), 0, one}, vmserrors.SS_NOLOGTAB},
		{"too few args", []uint32{0, a.desc("LNM$PROCESS"), a.desc("X"), 0}, ssInsfArg},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wantR0(t, callLNM(t, env, serviceSysCrelnm, tt.argv...), tt.want)
		})
	}
}

func TestSysDellnm(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	defineLogical(t, env, "LNM$PROCESS", "A", lnm.User, 0, "1")
	defineLogical(t, env, "LNM$PROCESS", "A", lnm.Supervisor, 0, "2")
	defineLogical(t, env, "LNM$PROCESS", "B", lnm.User, 0, "3")

	// Deleting at user mode leaves the supervisor-mode A.
	wantR0(t, callLNM(t, env, serviceSysDellnm, a.desc("LNM$FILE_DEV"), a.desc("A"), a.byteArg(3)), ssNormal)

	if e, err := env.Logicals.Translate("LNM$PROCESS", "A", lnm.User, 0); err != nil || e.Mode != lnm.Supervisor {
		t.Errorf("A after user-mode delete = %+v, %v", e, err)
	}

	wantR0(t, callLNM(t, env, serviceSysDellnm, a.desc("LNM$FILE_DEV"), a.desc("NOSUCH"), a.byteArg(3)), vmserrors.SS_NOLOGNAM)

	// No name: every name at the mode or outer in the first table.
	wantR0(t, callLNM(t, env, serviceSysDellnm, a.desc("LNM$PROCESS"), 0, a.byteArg(2)), ssNormal)

	for _, n := range []string{"A", "B"} {
		if _, err := env.Logicals.Translate("LNM$PROCESS", n, lnm.User, 0); err == nil {
			t.Errorf("%s survived", n)
		}
	}

	if _, err := env.Logicals.Translate("LNM$PROCESS", "SYS$OUTPUT", lnm.User, 0); err != nil {
		t.Error("the executive-mode SYS$OUTPUT was deleted")
	}

	wantR0(t, callLNM(t, env, serviceSysDellnm, 0, 0, 0), ssBadParam)
}

func TestSysCrelnt(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	res, resBuf := a.outDesc(31)
	resLen := a.alloc(2)

	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, res, resLen, 0, 0, a.desc("TAX"), a.desc("LNM$PROCESS_DIRECTORY"), a.byteArg(2)),
		ssLnmCreated)

	if got := a.readString(resBuf, readWordEnv(t, env, resLen)); got != "TAX" {
		t.Errorf("resnam = %q", got)
	}

	tables, err := env.Logicals.ResolveTables("TAX", lnm.User)
	if err != nil || tables[0].Mode != lnm.Supervisor || tables[0].Parent != env.Logicals.ProcessDirectory {
		t.Fatalf("TAX = %+v, %v", tables, err)
	}

	// CREATE_IF on an existing table changes nothing.
	wantR0(t, callLNM(t, env, serviceSysCrelnt, a.long(lnm.AttrCreateIf), 0, 0, 0, 0, a.desc("TAX"), a.desc("LNM$PROCESS_DIRECTORY"), a.byteArg(2)),
		ssNormal)

	// Without it, the table is superseded.
	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, 0, 0, 0, 0, a.desc("TAX"), a.desc("LNM$PROCESS_DIRECTORY"), a.byteArg(2)),
		ssSupersede)

	// No tabnam: a unique LNM$xxxx name comes back.
	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, res, resLen, 0, 0, 0, a.desc("LNM$PROCESS_TABLE"), 0), ssLnmCreated)

	if got := a.readString(resBuf, readWordEnv(t, env, resLen)); !strings.HasPrefix(got, "LNM$") {
		t.Errorf("default name = %q", got)
	}

	// A result buffer too small for the name.
	small, _ := a.outDesc(2)
	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, small, 0, 0, 0, a.desc("LONGNAME"), a.desc("LNM$PROCESS_DIRECTORY"), 0), ssResultOvf)

	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, 0, 0, 0, 0, a.desc("T"), a.desc("NOSUCH"), 0), vmserrors.SS_NOLOGTAB)
	wantR0(t, callLNM(t, env, serviceSysCrelnt, 0, 0, 0, 0, 0, a.desc("T")), ssInsfArg)
}

func TestOldStyleServices(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	group := env.Logicals.GroupTableName

	// $CRELOG into each table (by value: tblflg, acmode).
	for _, tt := range []struct {
		tblflg uint32
		name   string
		table  string
	}{
		{0, "SYSNAME", lnm.SystemTableName},
		{1, "GRPNAME", group},
		{2, "PRCNAME", lnm.ProcessTableName},
	} {
		wantR0(t, callLNM(t, env, serviceSysCrelog, tt.tblflg, a.desc(tt.name), a.desc("DUA0:"), 3), ssNormal)

		e, err := env.Logicals.Translate(tt.table, tt.name, lnm.User, 0)
		if err != nil || e.Attrs != lnm.AttrCrelog || e.Mode != lnm.User {
			t.Errorf("%s = %+v, %v", tt.name, e, err)
		}
	}

	wantR0(t, callLNM(t, env, serviceSysCrelog, 2, a.desc("PRCNAME"), a.desc("DUA1:"), 3), ssSupersede)
	wantR0(t, callLNM(t, env, serviceSysCrelog, 7, a.desc("X"), a.desc("Y"), 0), ssIvLogTab)

	// acmode 0 is maximized with the caller's mode.
	psl := env.cpu.PSL()
	psl.SetCurMod(vax.User)
	env.cpu.SetPSL(psl)
	wantR0(t, callLNM(t, env, serviceSysCrelog, 2, a.desc("MAXED"), a.desc("V"), 0), ssNormal)

	if e, _ := env.Logicals.Translate("LNM$PROCESS", "MAXED", lnm.User, 0); e == nil || e.Mode != lnm.User {
		t.Errorf("MAXED = %+v, want user mode", e)
	}

	// $TRNLOG: process before group before system, dsbmsk skipping.
	defineLogical(t, env, lnm.SystemTableName, "SHARED", lnm.Executive, 0, "FROM_SYSTEM")
	defineLogical(t, env, group, "SHARED", lnm.Supervisor, 0, "FROM_GROUP")

	for _, tt := range []struct {
		name      string
		dsbmsk    uint32
		want      string
		wantTable byte
		status    uint32
	}{
		{"PRCNAME", 0, "DUA1:", 2, ssNormal},
		{"SHARED", 0, "FROM_GROUP", 1, ssNormal},
		{"SHARED", 2, "FROM_SYSTEM", 0, ssNormal},
		{"SHARED", 3, "SHARED", 0, ssNoTran},
		{"_DUA0:", 0, "DUA0:", 0, ssNoTran},
		{"NOSUCH", 0, "NOSUCH", 0, ssNoTran},
	} {
		rsl, rslBuf := a.outDesc(32)
		rslLen, table, mode := a.alloc(2), a.byteArg(0xFF), a.byteArg(0xFF)

		wantR0(t, callLNM(t, env, serviceSysTrnlog, a.desc(tt.name), rslLen, rsl, table, mode, tt.dsbmsk), tt.status)

		if got := a.readString(rslBuf, readWordEnv(t, env, rslLen)); got != tt.want {
			t.Errorf("$TRNLOG %s/%d = %q, want %q", tt.name, tt.dsbmsk, got, tt.want)
		}

		if tt.status == ssNormal && a.readByte(table) != tt.wantTable {
			t.Errorf("$TRNLOG %s/%d table = %d, want %d", tt.name, tt.dsbmsk, a.readByte(table), tt.wantTable)
		}
	}

	small, _ := a.outDesc(2)
	wantR0(t, callLNM(t, env, serviceSysTrnlog, a.desc("PRCNAME"), 0, small, 0, 0, 0), ssResultOvf)

	// $DELLOG, by name and by table.
	wantR0(t, callLNM(t, env, serviceSysDellog, 1, a.desc("GRPNAME"), 3), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDellog, 1, a.desc("GRPNAME"), 3), vmserrors.SS_NOLOGNAM)
	wantR0(t, callLNM(t, env, serviceSysDellog, 2, 0, 3), ssNormal)

	if _, err := env.Logicals.Translate("LNM$PROCESS", "PRCNAME", lnm.User, 0); err == nil {
		t.Error("PRCNAME survived $DELLOG of the process table")
	}
}

func TestLogicalServicesRegistered(t *testing.T) {
	env, _ := fixture()

	for _, name := range []string{"SYS$TRNLNM", "SYS$CRELNM", "SYS$DELLNM", "SYS$CRELNT", "SYS$CRELOG", "SYS$DELLOG", "SYS$TRNLOG"} {
		if _, ok := env.services.Lookup(name); !ok {
			t.Errorf("%s isn't registered", name)
		}
	}
}

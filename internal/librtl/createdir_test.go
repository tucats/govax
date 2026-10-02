package librtl

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
)

// createDirFixture is the fixture with a fresh volume mounted on DUA0,
// writable unless readOnly.
func createDirFixture(t *testing.T, readOnly bool) *rtl.Environment {
	t.Helper()

	env := fixture(t)

	path := filepath.Join(t.TempDir(), "libcrd.dsk")
	if err := rms.InitializeContainer(path, 0, "LIBCRD", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := env.Mounts.Mount("DUA0", path, !readOnly); err != nil {
		t.Fatal(err)
	}

	return env
}

// arena hands out addresses for arguments, from 0x10000 up.
type arena struct {
	t    *testing.T
	env  *rtl.Environment
	next uint32
}

func newArena(t *testing.T, env *rtl.Environment) *arena {
	return &arena{t: t, env: env, next: 0x10000}
}

// desc writes a descriptor for s and returns its address.
func (a *arena) desc(s string) uint32 {
	addr := a.next
	a.next += 8 + uint32(len(s)+7)&^7
	putDescriptor(a.t, a.env, addr, addr+8, s)

	return addr
}

// long writes a longword and returns its address.
func (a *arena) long(v uint32) uint32 {
	addr := a.next
	a.next += 8
	putLongword(a.t, a.env, addr, v)

	return addr
}

// word writes a word and returns its address.
func (a *arena) word(v uint16) uint32 {
	addr := a.next
	a.next += 8

	if err := a.env.Memory().StoreWord(a.env.CPU(), addr, v); err != nil {
		a.t.Fatal(err)
	}

	return addr
}

func dirHeader(t *testing.T, env *rtl.Environment, dirs ...string) ondisk.FileHeader {
	t.Helper()

	vol, _ := env.Mounts.Lookup("DUA0")

	d, err := filespec.ResolveDirectory(vol, dirs)
	if err != nil {
		t.Fatalf("ResolveDirectory(%v): %v", dirs, err)
	}

	return d.Header
}

func TestLibCreateDirCreatesAndExists(t *testing.T) {
	env := createDirFixture(t, false)
	a := newArena(t, env)

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[A.B]")); r0 != ssCreated {
		t.Errorf("first call = %#x, want SS$_CREATED", r0)
	}

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[A.B]")); r0 != ssNormal {
		t.Errorf("second call = %#x, want SS$_NORMAL", r0)
	}

	// Defaults: the parent's owner and limit, its protection less delete.
	mfd := dirHeader(t, env)
	b := dirHeader(t, env, "A", "B")

	if b.Owner != mfd.Owner || b.FileProtection != mfd.FileProtection|ondisk.ProtectionNoDeleteAll {
		t.Errorf("[A.B]: owner %v, protection %#x; want the MFD's %v, %#x",
			b.Owner, b.FileProtection, mfd.Owner, mfd.FileProtection|ondisk.ProtectionNoDeleteAll)
	}
}

func TestLibCreateDirArguments(t *testing.T) {
	env := createDirFixture(t, false)
	a := newArena(t, env)

	owner := a.long(0o200<<16 | 0o201)
	enable, value := a.word(0xF000), a.word(0xE000)
	limit, rvn := a.word(3), a.word(1)

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[ARGS]"), owner, enable, value, limit, rvn); r0 != ssCreated {
		t.Fatalf("LIB$CREATE_DIR = %#x, want SS$_CREATED", r0)
	}

	h := dirHeader(t, env, "ARGS")
	mfd := dirHeader(t, env)

	wantProt := uint16(0xE000) | (mfd.FileProtection|ondisk.ProtectionNoDeleteAll)&0x0FFF
	if h.Owner != (ondisk.Uic{Group: 0o200, Member: 0o201}) || h.RecordAttributes.VersionLimit != 3 || h.FileProtection != wantProt {
		t.Errorf("[ARGS]: owner %v, limit %d, protection %#x; want [200,201], 3, %#x",
			h.Owner, h.RecordAttributes.VersionLimit, h.FileProtection, wantProt)
	}

	// Omitted (0) arguments take the defaults: [ARGS.CHILD] gets [ARGS]'s
	// owner and limit; a 0 limit given means none.
	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[ARGS.CHILD]"), 0, 0, 0, 0); r0 != ssCreated {
		t.Fatalf("[ARGS.CHILD] = %#x", r0)
	}

	if c := dirHeader(t, env, "ARGS", "CHILD"); c.Owner != h.Owner || c.RecordAttributes.VersionLimit != 3 {
		t.Errorf("[ARGS.CHILD]: owner %v, limit %d; want [ARGS]'s %v, 3", c.Owner, c.RecordAttributes.VersionLimit, h.Owner)
	}

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[ARGS.NONE]"), 0, 0, 0, a.word(0)); r0 != ssCreated {
		t.Fatalf("[ARGS.NONE] = %#x", r0)
	}

	if got := dirHeader(t, env, "ARGS", "NONE").RecordAttributes.VersionLimit; got != 0 {
		t.Errorf("[ARGS.NONE] limit = %d, want 0", got)
	}
}

// TestLibCreateDirUICFormat: [g,m] names directory GGGMMM, owned by that
// UIC unless owner-UIC says otherwise.
func TestLibCreateDirUICFormat(t *testing.T) {
	env := createDirFixture(t, false)
	a := newArena(t, env)

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[123,321]")); r0 != ssCreated {
		t.Fatalf("LIB$CREATE_DIR([123,321]) = %#x, want SS$_CREATED", r0)
	}

	if got := dirHeader(t, env, "123321").Owner; got != (ondisk.Uic{Group: 0o123, Member: 0o321}) {
		t.Errorf("[123321] owner = %v, want [123,321]", got)
	}

	if r0 := call(t, env, "LIB$CREATE_DIR", a.desc("DUA0:[1,4]"), a.long(0o200<<16|0o201)); r0 != ssCreated {
		t.Fatalf("LIB$CREATE_DIR([1,4]) = %#x, want SS$_CREATED", r0)
	}

	if got := dirHeader(t, env, "001004").Owner; got != (ondisk.Uic{Group: 0o200, Member: 0o201}) {
		t.Errorf("[001004] owner = %v, want owner-UIC's [200,201]", got)
	}
}

func TestLibCreateDirErrors(t *testing.T) {
	env := createDirFixture(t, false)
	a := newArena(t, env)

	tests := []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"no arguments", nil, libInvArg},
		{"no descriptor", []uint32{0}, libInvArg},
		{"too long", []uint32{a.desc("DUA0:[" + strings.Repeat("A", 250) + "]")}, libInvArg},
		{"no directory", []uint32{a.desc("DUA0:")}, libInvFilSpe},
		{"file name", []uint32{a.desc("DUA0:[A]X.DAT")}, libInvFilSpe},
		{"wildcard", []uint32{a.desc("DUA0:[A*]")}, libInvFilSpe},
		{"node", []uint32{a.desc("NODE::DUA0:[A]")}, libInvFilSpe},
		{"no volume default", []uint32{a.desc("[A]")}, libInvFilSpe},
		{"not mounted", []uint32{a.desc("DUB0:[A]")}, rmsDev},
		{"nine levels", []uint32{a.desc("DUA0:[L1.L2.L3.L4.L5.L6.L7.L8.L9]")}, rmsDir},
		{"name too long", []uint32{a.desc("DUA0:[" + strings.Repeat("N", 40) + "]")}, rmsDir},
		{"unreadable owner", []uint32{a.desc("DUA0:[A]"), 0x7FFF0000}, ssAccVio},
	}

	for _, tt := range tests {
		if r0 := call(t, env, "LIB$CREATE_DIR", tt.argv...); r0 != tt.want {
			t.Errorf("%s: %#x, want %#x", tt.name, r0, tt.want)
		}
	}

	vol, _ := env.Mounts.Lookup("DUA0")
	if _, err := filespec.ResolveDirectory(vol, []string{"A"}); err == nil {
		t.Error("[A] was made by a failing call")
	}

	ro := createDirFixture(t, true)
	if r0 := call(t, ro, "LIB$CREATE_DIR", newArena(t, ro).desc("DUA0:[A]")); r0 != ssWritLck {
		t.Errorf("read-only volume: %#x, want SS$_WRITLCK", r0)
	}
}

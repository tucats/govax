package rtl

import (
	"bytes"
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

func dviCode(t *testing.T, name string) uint16 {
	t.Helper()

	code, ok := vmsdef.DVIConstants[name]
	if !ok {
		t.Fatalf("no $DVIDEF code %s", name)
	}

	return uint16(code)
}

// getdvi calls $GETDVIW with an 8-argument list and no AST.
func getdvi(t *testing.T, env *Environment, efn, channel, devnam, itmlst, iosb uint32) uint32 {
	t.Helper()

	return callLNM(t, env, serviceSysGetdvi, efn, channel, devnam, itmlst, iosb, 0, 0, 0)
}

// dviValues asks for each named item of the device a channel or name
// designates, returning the bytes each one returned.
func dviValues(t *testing.T, env *Environment, a *arena, channel, devnam uint32, names ...string) []string {
	t.Helper()

	items := make([]item, len(names))
	for i, n := range names {
		items[i] = item{code: dviCode(t, n), buflen: 64, buf: a.alloc(64), ret: a.alloc(2)}
	}

	wantR0(t, getdvi(t, env, 0, channel, devnam, a.items(items...), 0), ssNormal)

	out := make([]string, len(names))

	for i := range items {
		n, err := env.mem.LoadWord(env.cpu, items[i].ret)
		if err != nil {
			t.Fatal(err)
		}

		out[i] = a.readString(items[i].buf, n)
	}

	return out
}

func long(v uint32) string { return littleEndian(uint64(v), 4) }

func TestGetdvi_items(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	tt := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)
	tt.DevType = 96
	tt.DevBufSize = 80
	tt.DevDepend = 24<<24 | vmsdef.TTConstants["TT$M_LOWER"]
	tt.DevDepend2 = vmsdef.TTConstants["TT2$M_ANSICRT"]
	tt.DevChar = vmsdef.DEVConstants["DEV$M_TRM"] | vmsdef.DEVConstants["DEV$M_REC"]

	ch := assignCall(t, env, a, "TTA0", uint32(vax.User))

	cases := []struct {
		name, want string
	}{
		{"DVI$_DEVCLASS", long(uint32(iodev.DeviceClassTT))},
		{"DVI$_DEVTYPE", long(96)},
		{"DVI$_DEVBUFSIZ", long(80)},
		{"DVI$_DEVCHAR", long(tt.DevChar)},
		{"DVI$_DEVDEPEND", long(tt.DevDepend)},
		{"DVI$_DEVDEPEND2", long(tt.DevDepend2)},
		{"DVI$_UNIT", long(0)},
		{"DVI$_PID", long(env.Process.PID)},
		{"DVI$_OWNUIC", long(env.Process.UIC)},
		{"DVI$_REFCNT", long(1)},
		{"DVI$_DEVNAM", "_TTA0:"},
		{"DVI$_FULLDEVNAM", "_GOVAX$TTA0:"},
		{"DVI$_ALLDEVNAM", "_GOVAX$TTA0:"},
		{"DVI$_TT_PHYDEVNAM", "_TTA0:"},
		{"DVI$_TT_PAGE", long(24)},
		// DEVCHAR bits as Booleans.
		{"DVI$_TRM", long(1)},
		{"DVI$_REC", long(1)},
		{"DVI$_MNT", long(0)},
		// Terminal characteristics from DEVDEPEND and DEVDEPEND2.
		{"DVI$_TT_LOWER", long(1)},
		{"DVI$_TT_NOECHO", long(0)},
		{"DVI$_TT_ANSICRT", long(1)},
		{"DVI$_TT_DECCRT", long(0)},
		{"DVI$_REMOTE_DEVICE", long(0)},
	}

	names := make([]string, len(cases))
	for i, c := range cases {
		names[i] = c.name
	}

	got := dviValues(t, env, a, ch, 0, names...)
	for i, c := range cases {
		if got[i] != c.want {
			t.Errorf("%s = %q, want %q", c.name, got[i], c.want)
		}
	}

	// A non-terminal: its unit number, no physical terminal name.
	defineTestDevice(env, "DUA12", iodev.DeviceClassDisk)

	got = dviValues(t, env, a, 0, a.desc("DUA12:"), "DVI$_UNIT", "DVI$_TT_PHYDEVNAM", "DVI$_DEVNAM")
	if got[0] != long(12) || got[1] != "" || got[2] != "_DUA12:" {
		t.Errorf("DUA12: unit %q, phydevnam %q, devnam %q", got[0], got[1], got[2])
	}

	// The secondary-device flag in an item code is ignored.
	buf := a.alloc(4)
	secondary := dviCode(t, "DVI$_DEVCLASS") | uint16(vmsdef.DVIConstants["DVI$M_SECONDARY"])
	wantR0(t, getdvi(t, env, 0, ch, 0, a.items(item{code: secondary, buflen: 4, buf: buf}), 0), ssNormal)

	if a.readLong(buf) != uint32(iodev.DeviceClassTT) {
		t.Errorf("DVI$_DEVCLASS!DVI$M_SECONDARY = %d", a.readLong(buf))
	}

	// A one-byte buffer gets the low byte (eVAX's $GETDVIW wrote bytes).
	buf, ret := a.alloc(1), a.alloc(2)
	wantR0(t, getdvi(t, env, 0, ch, 0, a.items(item{code: dviCode(t, "DVI$_DEVCLASS"), buflen: 1, buf: buf, ret: ret}), 0), ssNormal)

	if a.readByte(buf) != byte(iodev.DeviceClassTT) {
		t.Errorf("DVI$_DEVCLASS in one byte = %d", a.readByte(buf))
	}
}

// TestGetdvi_registry: every DVI$_TT_ item and DEVCHAR bit is supported,
// and every registered name is a real $DVIDEF code.
func TestGetdvi_registry(t *testing.T) {
	for name := range vmsdef.DVIConstants {
		if strings.HasPrefix(name, "DVI$_TT_") && dviItemsByName[name] == nil {
			t.Errorf("%s has no $TTDEF bit", name)
		}
	}

	for name := range dviItemsByName {
		if _, ok := vmsdef.DVIConstants[name]; !ok {
			t.Errorf("%s isn't a $DVIDEF code", name)
		}
	}
}

func TestGetdvi_completionAndErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	defineTestDevice(env, "TTA0", iodev.DeviceClassTT)
	ch := assignCall(t, env, a, "TTA0", uint32(vax.Kernel))

	good := a.items(item{code: dviCode(t, "DVI$_DEVCLASS"), buflen: 4, buf: a.alloc(4)})

	// Completion: the flag set, the IOSB written, the AST queued.
	iosb := a.alloc(8)
	env.Process.LocalEventFlags[0] = 0
	wantR0(t, callLNM(t, env, serviceSysGetdvi, 4, ch, 0, good, iosb, 0x4000, 7), ssNormal)

	if !flagSet(env, 4) || a.readLong(iosb) != ssNormal || env.PendingASTs() != 1 {
		t.Errorf("flag %v, IOSB %#x, %d ASTs; want set, SS$_NORMAL, 1", flagSet(env, 4), a.readLong(iosb), env.PendingASTs())
	}

	// An unknown item still completes, with SS$_BADPARAM.
	bad := a.items(item{code: 0x7FFE, buflen: 4, buf: a.alloc(4)})
	wantR0(t, getdvi(t, env, 4, ch, 0, bad, iosb), ssBadParam)

	if !flagSet(env, 4) || a.readLong(iosb) != ssBadParam {
		t.Error("SS$_BADPARAM didn't complete the request")
	}

	// Rejected before starting: nothing completes.
	env.Process.ast.queue = nil
	setMode(env, vax.User, vax.User, 0x8000)

	for name, c := range map[string]struct {
		channel, devnam uint32
		want            uint32
	}{
		"a more privileged channel": {ch, 0, ssNoPriv},
		"an unassigned channel":     {0x7F0, 0, ssNoPriv},
		"no device":                 {0, 0, ssIvDevNam},
		"an empty name":             {0, a.desc(""), ssIvLogNam},
		"a long name":               {0, a.desc(strings.Repeat("X", 64)), ssIvLogNam},
		"no such device":            {0, a.desc("XYZ0:"), ssNoSuchDev},
	} {
		putLongword(t, env, iosb, 0xFFFF)
		wantR0(t, callLNM(t, env, serviceSysGetdvi, 4, c.channel, c.devnam, good, iosb, 0x4000, 7), c.want)

		if env.PendingASTs() != 0 {
			t.Errorf("%s: an AST was queued", name)
		}

		if got := a.readLong(iosb); got != 0 {
			t.Errorf("%s: IOSB = %#x, want cleared only", name, got)
		}
	}

	wantR0(t, callLNM(t, env, serviceSysGetdvi, 0, ch, 0), ssInsfArg)
	wantR0(t, getdvi(t, env, 0, 0, a.desc("TTA0"), good, badAddr), ssAccVio)
}

func TestGetdvi_debugTrace(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	defineTestDevice(env, "DKA0", iodev.DeviceClassDisk)

	var trace bytes.Buffer

	env.cpu.SetDebugWriter(&trace)
	env.cpu.SetDebug(vax.DebugDevices)

	dviValues(t, env, a, 0, a.desc("DKA0"), "DVI$_DEVCLASS")

	if !strings.Contains(trace.String(), "DEBUG: SYS$GETDVI looks up device DKA0") {
		t.Errorf("trace = %q", trace.String())
	}
}

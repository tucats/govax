package corevms

import (
	"testing"
	"time"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// setTDF makes the local time zone seconds east of Greenwich.
func setTDF(t *testing.T, env *Environment, seconds string) {
	t.Helper()
	defineLogical(t, env, lnm.SystemTableName, lnm.TimeZoneName, lnm.Executive, 0, seconds)
}

func TestUTCBase(t *testing.T) {
	if got := utcBaseCheck(); got != utcBaseTicks {
		t.Errorf("15-Oct-1582 to 17-Nov-1858 is %d ticks, the constant %d", got, uint64(utcBaseTicks))
	}
}

func TestUTCOctaword(t *testing.T) {
	for _, u := range []utcTime{
		{ticks: 0x0123456789ABCDEF, inacc: utcInfinite, tdf: -300, vers: 1},
		{ticks: 5, inacc: 12345, tdf: 2047, vers: 1},
		{ticks: 5, inacc: 0, tdf: -2048, vers: 15},
	} {
		if got := utcFromBytes(u.bytes()); got != u {
			t.Errorf("%+v comes back %+v", u, got)
		}
	}

	b := utcTime{ticks: 1, inacc: utcInfinite, tdf: -300, vers: 1}.bytes()
	// -300 in 12 bits is 0xED4, under version 1: 0x1ED4.
	if b[14] != 0xD4 || b[15] != 0x1E || b[8] != 0xFF || b[13] != 0xFF {
		t.Errorf("octaword % X", b)
	}
}

func TestUTCServices(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)
	setTDF(t, env, "-18000") // US Eastern, as the simh VAX's

	local := vmsdef.Time(time.Date(1999, time.March, 4, 13, 30, 15, 250_000_000, time.UTC))
	*now = local

	// $GETUTC: five hours later in UTC, the TDF -300, infinite inaccuracy.
	utc := a.alloc(16)
	wantR0(t, callLNM(t, env, serviceSysGetutc, utc), ssNormal)

	u, _ := env.loadUTC(utc)
	if want := local + utcBaseTicks + 300*ticksPerMinute; u.ticks != want || u.tdf != -300 || u.vers != 1 || u.inacc != utcInfinite {
		t.Errorf("$GETUTC: %+v, ticks want %d", u, want)
	}

	// $ASCUTC of it, and of the current time: the local time.
	buf, _ := a.outDesc(23)
	n := a.alloc(2)
	for _, arg := range []uint32{utc, 0} {
		wantR0(t, callLNM(t, env, serviceSysAscutc, n, buf, arg, 0), ssNormal)

		if got := a.readString(a.readLong(buf+4), 23); got != " 4-MAR-1999 13:30:15.25" {
			t.Errorf("$ASCUTC(%#x) = %q", arg, got)
		}
	}

	// $TIMCON both ways.
	smn := a.quad(0)
	wantR0(t, callLNM(t, env, serviceSysTimcon, smn, utc, 0), ssNormal)

	if got, _ := env.loadQuad(smn); got != local {
		t.Errorf("$TIMCON 0: %d, want %d", got, local)
	}

	back := a.alloc(16)
	wantR0(t, callLNM(t, env, serviceSysTimcon, smn, back, 1), ssNormal)

	if got, _ := env.loadUTC(back); got != u {
		t.Errorf("$TIMCON 1: %+v, want %+v", got, u)
	}

	wantR0(t, callLNM(t, env, serviceSysTimcon, smn, back, 2), ssNormal) // as 1, VMS 7.1's
	wantR0(t, callLNM(t, env, serviceSysTimcon, a.quad(-10_000_000), back, 1), ssIvTime)

	// A UTC time in another zone keeps its own: 15 o'clock UTC (18:30 less
	// 3:30) in a zone an hour east reads 16 o'clock.
	other := utcTime{ticks: u.ticks - 3*3600*vmsdef.TicksPerSecond - 30*ticksPerMinute, inacc: 0, tdf: 60, vers: 1}
	a2 := a.alloc(16)
	env.storeUTC(a2, other)
	wantR0(t, callLNM(t, env, serviceSysAscutc, n, buf, a2, 1), ssNormal)

	if got := a.readString(a.readLong(buf+4), 11); got != "16:00:15.25" {
		t.Errorf("$ASCUTC of another zone's = %q", got)
	}

	// $BINUTC: the text is local time.
	wantR0(t, callLNM(t, env, serviceSysBinutc, a.desc(" 4-MAR-1999 13:30:15.25"), back), ssNormal)

	if got, _ := env.loadUTC(back); got != u {
		t.Errorf("$BINUTC: %+v, want %+v", got, u)
	}

	wantR0(t, callLNM(t, env, serviceSysBinutc, a.desc("5 3:00"), back), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysBinutc, a.desc("BAD"), back), ssIvTime)

	// $NUMUTC: the local time, the inaccuracy, the TDF.
	words := a.alloc(26)
	wantR0(t, callLNM(t, env, serviceSysNumutc, words, utc), ssNormal)

	want := []uint16{1999, 3, 4, 13, 30, 15, 25, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFED4}
	for i, w := range want {
		if got, _ := env.mem.LoadWord(env.cpu, words+uint32(2*i)); got != w {
			t.Errorf("$NUMUTC word %d = %#x, want %#x", i, got, w)
		}
	}

	// A known inaccuracy: 1 day, 2:03:04.05.
	other.inacc = ticksPerDay + (2*3600+3*60+4)*vmsdef.TicksPerSecond + 5*ticksPerHundredth
	env.storeUTC(a2, other)
	wantR0(t, callLNM(t, env, serviceSysNumutc, words, a2), ssNormal)

	for i, w := range []uint16{1, 2, 3, 4, 5, 60} {
		if got, _ := env.mem.LoadWord(env.cpu, words+uint32(14+2*i)); got != w {
			t.Errorf("$NUMUTC inaccuracy word %d = %d, want %d", 7+i, got, w)
		}
	}

	// An inaccuracy of 0 is VMS's days 17.
	other.inacc = 0
	env.storeUTC(a2, other)
	wantR0(t, callLNM(t, env, serviceSysNumutc, words, a2), ssNormal)

	for i, w := range []uint16{17, 0, 0, 0, 0} {
		if got, _ := env.mem.LoadWord(env.cpu, words+uint32(14+2*i)); got != w {
			t.Errorf("$NUMUTC zero inaccuracy word %d = %d, want %d", 7+i, got, w)
		}
	}

	// Times a system time can't hold: before 17-Nov-1858, or version 2.
	early := utcTime{ticks: utcBaseTicks - 1, inacc: utcInfinite, vers: 1}
	env.storeUTC(a2, early)
	wantR0(t, callLNM(t, env, serviceSysAscutc, n, buf, a2, 0), ssIvTime)
	wantR0(t, callLNM(t, env, serviceSysTimcon, smn, a2, 0), ssIvTime)

	early.ticks, early.vers = u.ticks, 2
	env.storeUTC(a2, early)
	wantR0(t, callLNM(t, env, serviceSysNumutc, words, a2), ssIvTime)

	// Addresses.
	wantR0(t, callLNM(t, env, serviceSysGetutc, 0), ssAccVio)
	// (A $TIMCON address it can't reach is signaled: testdata/svc113's probe.)
	wantR0(t, callLNM(t, env, serviceSysNumutc, 0, utc), ssAccVio)
}

func TestLocalTDF(t *testing.T) {
	env, _ := fixture()

	for _, c := range []struct {
		seconds string
		want    int
	}{{"-18000", -300}, {"3600", 60}, {"19800", 330}, {"-29", 0}, {"-31", -1}, {"junk", 0}} {
		setTDF(t, env, c.seconds)

		if got := env.localTDF(); got != c.want {
			t.Errorf("%s seconds: TDF %d, want %d", c.seconds, got, c.want)
		}
	}
}

func TestGetchnGetdev(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	tt := defineTestDevice(env, "TTA3", iodev.DeviceClassTT)
	tt.DevType = 96
	tt.DevBufSize = 80
	tt.DevDepend = 24<<24 | 7
	tt.DevDepend2 = 9
	tt.DevChar = vmsdef.Symbols["DEV$M_TRM"] | vmsdef.Symbols["DEV$M_REC"]
	tt.RecSize = 33

	ch := assignCall(t, env, a, "TTA3", 0)

	check := func(what string, buf, length uint32) {
		t.Helper()

		n, _ := env.mem.LoadWord(env.cpu, length)
		if n != dibLength {
			t.Fatalf("%s: length %d, want %d", what, n, dibLength)
		}

		b := []byte(a.readString(a.readLong(buf+4), n))
		word := func(at int) uint32 { return uint32(b[at]) | uint32(b[at+1])<<8 }
		lng := func(at int) uint32 { return word(at) | word(at+2)<<16 }

		if lng(dibDevChar) != tt.DevChar || b[dibDevClass] != byte(iodev.DeviceClassTT) || b[dibDevType] != 96 ||
			word(dibDevBufSiz) != 80 || lng(dibDevDepend) != tt.DevDepend || word(dibUnit) != 3 ||
			lng(dibPID) != env.Process.PID || word(dibRecSiz) != 33 ||
			word(dibVolNamOff) != 0 {
			t.Errorf("%s: block % X", what, b[:44])
		}

		off := word(dibDevNamOff)
		if off != dibDevName || string(b[off+1:off+1+uint32(b[off])]) != "TTA" {
			t.Errorf("%s: name at %d, % X", what, off, b[dibDevName:])
		}
	}

	pri, _ := a.outDesc(200)
	scd, _ := a.outDesc(200)
	plen, slen := a.alloc(2), a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysGetchn, ch, plen, pri, slen, scd), ssNormal)
	check("$GETCHN primary", pri, plen)
	check("$GETCHN secondary", scd, slen)

	wantR0(t, callLNM(t, env, serviceSysGetdev, a.desc("TTA3:"), plen, pri, 0, 0), ssNormal)
	check("$GETDEV", pri, plen)

	// Fewer than five arguments.
	wantR0(t, callLNM(t, env, serviceSysGetdev, a.desc("TTA3:"), plen, pri), ssInsfArg)

	// A short buffer, and the failures.
	short, _ := a.outDesc(10)
	slen2 := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysGetdev, a.desc("TTA3"), slen2, short, 0, 0), ssBufferOvf)
	wantR0(t, callLNM(t, env, serviceSysGetchn, 0, 0, 0, 0, 0), ssIvChan)
	wantR0(t, callLNM(t, env, serviceSysGetchn, 999, plen, pri, 0, 0), ssIvChan)
	wantR0(t, callLNM(t, env, serviceSysGetdev, a.desc("XYZ9"), plen, pri, 0, 0), ssNoSuchDev)
	wantR0(t, callLNM(t, env, serviceSysGetdev, 0, plen, pri, 0, 0), ssIvDevNam)
}

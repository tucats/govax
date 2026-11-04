package rms

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/ondisk"
)

// The indexed file probe's VMS run (docs/PHASE-88 - keyed index files.md,
// testdata/idx88): its files in a save set, and its log.
const idx88Dir = "../../testdata/idx88/vax"

// Where the tests below put their control blocks in memory.
const (
	idxFabAddr  uint32 = 0x1000
	idxNameAddr uint32 = 0x1200
	idxSumAddr  uint32 = 0x1400
	idxKeyAddr  uint32 = 0x1500
	idxAllAddr  uint32 = 0x1600
	idxKnmAddr  uint32 = 0x1700
)

// idxProbeFiles restores every file of the probe's save set onto a fresh
// volume with VMS's 3-block clusters, as DUA1:[000000], and returns a
// Context on it and the files' names by case number (S1, G104, ...; E103 is
// the case's second version).
func idxProbeFiles(t *testing.T) (*Context, map[int]string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(idx88Dir, "idx88.bck"))
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	s := newRestoreSession(t, 20000, 3)
	owner := ondisk.Uic{Group: 1, Member: 4}
	names := map[int]string{}
	digits := regexp.MustCompile(`^\[000000\]([A-Z])(\d+)\.IDX;(\d+)$`)

	for _, f := range set.Files {
		m := digits.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}

		target, err := s.Locate("DUA1:[000000]"+strings.TrimPrefix(f.Name, "[000000]"), false)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.RestoreFile(target, f, RestoreOptions{Owner: &owner}); err != nil {
			t.Fatalf("restoring %s: %v", f.Name, err)
		}

		n, _ := strconv.Atoi(m[2])
		names[n] = m[1] + m[2] + ".IDX"
	}

	ctx := &Context{
		Mem:      vm.NewMemory(1 << 20),
		CPU:      registers.New(),
		Mounts:   s.Mounts,
		Files:    NewFileTable(nil),
		Logicals: newTestLogicals(t),
		Console:  &bytes.Buffer{},
	}

	return ctx, names
}

// idxLog is the probe's log, by case.
func idxLog(t *testing.T) map[int][]string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(idx88Dir, "idx88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[int][]string{}
	head := regexp.MustCompile(`^#(\d+) `)
	n := -1

	for _, line := range strings.Split(string(data), "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			n, _ = strconv.Atoi(m[1])

			continue
		}

		if n >= 0 {
			cases[n] = append(cases[n], line)
		}
	}

	return cases
}

// lastOpen is what the case's last $OPEN that reported the keys printed:
// its FAB, XABSUM, and each key and area.
func lastOpen(lines []string) []string {
	var out []string

	for i, line := range lines {
		if !strings.HasPrefix(line, "=$OPEN r=") || i+2 >= len(lines) || !strings.HasPrefix(lines[i+2], " sum ") {
			continue
		}

		out = []string{line}

		for _, l := range lines[i+1:] {
			if !strings.HasPrefix(l, " fab ") && !strings.HasPrefix(l, " sum ") && !strings.HasPrefix(l, " key ") &&
				!strings.HasPrefix(l, "  pos=") && !strings.HasPrefix(l, " area ") {
				break
			}

			out = append(out, l)
		}
	}

	return out
}

// mustRun runs a service, failing the test on a Go error.
func mustRun(t *testing.T, service func(*Context, []uint32) (uint32, error), ctx *Context, addr uint32) uint32 {
	t.Helper()

	r0, err := service(ctx, []uint32{addr})
	if err != nil {
		t.Fatal(err)
	}

	return r0
}

// openAsProbe does what the probe's OPN did, and prints what it printed:
// $OPEN with a XABSUM, then $DISPLAY of each key with a XABKEY and of each
// area with a XABALL. STV is 0 where VMS's was a channel, which govax's
// isn't (both sides are masked by the caller).
func openAsProbe(t *testing.T, ctx *Context, name string) []string {
	t.Helper()

	for i := range uint32(0x800) {
		putByte(t, ctx, idxFabAddr+i, 0)
	}

	spec := "DUA1:[000000]" + name
	for i := range len(spec) {
		putByte(t, ctx, idxNameAddr+uint32(i), spec[i])
	}

	putByte(t, ctx, idxFabAddr+fabOffset("BID"), byte(vmsConst("FAB$C_BID")))
	putByte(t, ctx, idxFabAddr+fabOffset("BLN"), byte(vmsConst("FAB$C_BLN")))
	putByte(t, ctx, idxFabAddr+fabFAC, facGet)
	putLongwordAt(t, ctx, idxFabAddr+fabFNA, idxNameAddr)
	putByte(t, ctx, idxFabAddr+fabFNS, byte(len(spec)))

	xab := func(addr uint32, cod byte, bln byte) {
		putByte(t, ctx, addr+xabCOD, cod)
		putByte(t, ctx, addr+xabBLN, bln)
	}

	xab(idxSumAddr, xabSUM, byte(vmsConst("XAB$C_SUMLEN")))
	xab(idxKeyAddr, xabKEY, byte(vmsConst("XAB$C_KEYLEN")))
	putLongwordAt(t, ctx, idxKeyAddr+xabKeyKNM, idxKnmAddr)
	xab(idxAllAddr, xabALL, byte(vmsConst("XAB$C_ALLLEN")))

	putLongwordAt(t, ctx, idxFabAddr+fabOffset("XAB"), idxSumAddr)

	r0 := mustRun(t, SysOpen, ctx, idxFabAddr)
	out := []string{fmt.Sprintf("=$OPEN r=%08X", r0)}

	b := func(addr uint32) byte { return readByte(t, ctx, addr) }
	w := func(addr uint32) uint16 { return readWord(t, ctx, addr) }
	l := func(addr uint32) uint32 { return readLongword(t, ctx, addr) }

	f := idxFabAddr
	out = append(out, fmt.Sprintf(" fab org=%02X rfm=%02X rat=%02X mrs=%d bks=%d alq=%d deq=%d fop=%08X sts=%08X stv=%08X",
		b(f+fabORG), b(f+fabRFM), b(f+fabRAT), w(f+fabMRS), b(f+fabOffset("BKS")), l(f+fabOffset("ALQ")),
		w(f+fabOffset("DEQ")), l(f+fabFOP), l(f+fabSTS), l(f+fabSTV)))

	if r0&1 == 0 {
		return out
	}

	out = append(out, fmt.Sprintf(" sum noa=%d nok=%d pvn=%d", b(idxSumAddr+xabNOA), b(idxSumAddr+xabNOK), w(idxSumAddr+xabPVN)))

	display := func(xabAddr uint32) {
		putLongwordAt(t, ctx, f+fabOffset("XAB"), xabAddr)

		if r0 := mustRun(t, SysDisplay, ctx, f); r0&1 == 0 {
			out = append(out, fmt.Sprintf("=$DISPLAY r=%08X", r0))
		}
	}

	for ref := range int(b(idxSumAddr + xabNOK)) {
		k := idxKeyAddr
		putByte(t, ctx, k+xabKeyREF, byte(ref))
		display(k)

		out = append(out, fmt.Sprintf(" key ref=%d dtp=%02X flg=%02X nsg=%d nul=%d tks=%d lvl=%d ian=%d lan=%d dan=%d ibs=%d dbs=%d dfl=%d ifl=%d mrl=%d rvb=%d dvb=%d plg=%d",
			b(k+xabKeyREF), b(k+xabKeyDTP), b(k+xabKeyFLG), b(k+xabKeyNSG), b(k+xabKeyNUL), b(k+xabKeyTKS), b(k+xabKeyLVL),
			b(k+xabKeyIAN), b(k+xabKeyLAN), b(k+xabKeyDAN), b(k+xabKeyIBS), b(k+xabKeyDBS), w(k+xabKeyDFL), w(k+xabKeyIFL),
			w(k+xabKeyMRL), uint32(w(k+xabKeyRVB))|uint32(w(k+xabKeyRVB+2))<<16, l(k+xabKeyDVB), b(k+xabKeyPROLOG)))

		var pos, siz []string
		for i := range keySegments {
			pos = append(pos, strconv.Itoa(int(w(k+xabKeyPOS+uint32(2*i)))))
			siz = append(siz, strconv.Itoa(int(b(k+xabKeySIZ+uint32(i)))))
		}

		var knm strings.Builder
		for i := range keyNameSize {
			c := b(idxKnmAddr + uint32(i))
			if c < ' ' || c > '~' {
				c = '.'
			}

			knm.WriteByte(c)
		}

		out = append(out, fmt.Sprintf("  pos=%s siz=%s knm=|%s|", strings.Join(pos, ","), strings.Join(siz, ","), knm.String()))
	}

	for aid := range int(b(idxSumAddr + xabNOA)) {
		a := idxAllAddr
		putByte(t, ctx, a+xabAllAID, byte(aid))
		display(a)

		out = append(out, fmt.Sprintf(" area aid=%d aln=%d aop=%02X bkz=%d alq=%d deq=%d vol=%d loc=%d",
			b(a+xabAllAID), b(a+xabAllALN), b(a+xabAOP), b(a+xabAllBKZ), l(a+xabALQ), w(a+xabDEQ), w(a+xabAllVOL), l(a+xabAllLOC)))
	}

	putLongwordAt(t, ctx, f+fabOffset("XAB"), 0)
	mustRun(t, SysClose, ctx, f)

	return out
}

// idxNotProlog3 are the probe's files of Prolog 1 and 2, which govax's
// $OPEN refuses with RMS$_PLV (the author's answer 1).
var idxNotProlog3 = map[int]bool{34: true, 59: true, 60: true, 61: true, 92: true}

// TestIndexedOpenAsVMS opens each of VMS's own files that the probe left as
// its last $OPEN of it saw them (the key shapes, areas, and growth cases),
// and $DISPLAYs every key and area as the probe did, holding the lines to
// VMS's: the FAB, the XABSUM, each XABKEY, and each XABALL.
func TestIndexedOpenAsVMS(t *testing.T) {
	ctx, names := idxProbeFiles(t)
	log := idxLog(t)
	channel := regexp.MustCompile(`stv=000000A0$`)
	compared := 0

	for n := 1; n <= 119; n++ {
		if n > 66 && n < 104 {
			continue // the refusals: no file, or no $OPEN
		}

		want := lastOpen(log[n])
		if len(want) == 0 {
			continue
		}

		got := openAsProbe(t, ctx, names[n])

		if idxNotProlog3[n] {
			if !strings.HasPrefix(got[0], fmt.Sprintf("=$OPEN r=%08X", rmsPrologLevel)) {
				t.Errorf("#%d: %s, want RMS$_PLV", n, got[0])
			}

			continue
		}

		for i := range max(len(want), len(got)) {
			var w, g string
			if i < len(want) {
				w = channel.ReplaceAllString(want[i], "stv=00000000")
			}

			if i < len(got) {
				g = got[i]
			}

			if w != g {
				t.Errorf("#%d %s, line %d:\n vms   %s\n govax %s", n, names[n], i, w, g)
			}
		}

		compared++
	}

	t.Logf("compared %d files", compared)

	if compared < 75 {
		t.Errorf("compared only %d files", compared)
	}
}

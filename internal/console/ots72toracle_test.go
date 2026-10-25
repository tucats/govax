package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Phase 72's study of FORRTL's calls (testdata/ots72's ots72t.mar) on
// govax, with VMS's own FORRTL.EXE (docs/PHASE-73 - trace trap.md,
// subtask 6). Each case traces a FOR$CVT_x_Ty call until FORRTL's JSB to
// OTS$$CVT_x_T_R8, and records the registers and FORRTL's frame at the
// JSB and at the RSB. The routine is govax's (librtl/otscvtfor.go) and
// FORRTL is VMS's, so the records are held to VMS's run
// (vax/ots72t-vax.txt) in what both sides decide: FORRTL's arguments to
// the routine and the shape of its frame, and the routine's results by
// its contract. The rest of a record is VMS's own: stack addresses
// (VMS's process has its stack elsewhere), addresses in VMS's images
// (FORRTL is placed elsewhere), the registers VMS's routine leaves (which
// FORRTL never reads), and whatever earlier calls left on the stack.
//
// FORRTL.EXE is licensed and can't be committed, so the test looks for
// it on the local-only volumes of testdata/disks (README.md) and skips
// without one.

// ots72tDir is the study's directory, from this package's.
var ots72tDir = filepath.Join("..", "..", "testdata", "ots72")

// forrtlVolumes are the local volumes that may hold VMS 7.1's FORRTL.EXE,
// and where on each: the simh VAX's system disk first (the one the study
// ran on), then the author's exchange volume.
var forrtlVolumes = []struct{ disk, file string }{
	{"rq0-ra92.dsk", "[VMS$COMMON.SYSLIB]FORRTL.EXE"},
	{"xfer.dsk", "[XFER]FORRTL.EXE"},
}

// ots72tDifferences are the cases whose records or text differ from
// VMS's, by the case's line, with why.
var ots72tDifferences = map[string]string{
	// VMS's H digits drift from the true value's past H's 34 significant
	// digits (a 40-digit E of 3.14159 ends ...117173, the true value
	// ...116973); govax's are exact. Both the text FORRTL prints and the
	// digits in the buffer show it.
	"H_TE 3.14159 50.40 0 0 2 0 R0=00000001": "VMS's H digits drift",
}

// TestOTS72TOracle runs ots72t.mar, linked against VMS's FORRTL.EXE, and
// holds each case to VMS's run: its text, the tracer's state, the value
// at R0, and the two records' fields (ots72tFields).
func TestOTS72TOracle(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "ots72t.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 OTS72`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	src, err := filepath.Abs(filepath.Join(ots72tDir, "ots72t.mar"))
	if err != nil {
		t.Fatal(err)
	}

	opt := filepath.Join(t.TempDir(), "ots72t.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST OTS72T.MAR`)
	dispatch(`COPY "` + opt + `"/HOST OTS72T.OPT`)

	if out := dispatch("MACRO/NOLIST OTS72T"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP OTS72T,OTS72T/OPTIONS"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	got := ots72tCases(strings.Split(dispatch("RUN OTS72T"), "|"))
	if status := strings.TrimSpace(dispatch("SHOW SYMBOL $STATUS")); status != `$STATUS == "%X00000001"` {
		t.Errorf("RUN OTS72T: %s, want SS$_NORMAL", status)
	}

	vax, err := os.ReadFile(filepath.Join(ots72tDir, "vax", "ots72t-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	want := ots72tCases(strings.Split(strings.TrimRight(string(vax), "\n"), "\n"))
	if len(got) != len(want) {
		t.Fatalf("%d cases, VMS's run %d", len(got), len(want))
	}

	seen := map[string]bool{}

	for i, w := range want {
		g := got[i]
		if g.title != w.title {
			t.Fatalf("case %d is %q, VMS's %q", i, g.title, w.title)
		}

		// The self-test's records are the program's own registers and
		// stack; TestTrace73SelfTest checks what it computes.
		if strings.HasPrefix(w.title, "tracer self-test ") {
			continue
		}

		problems := g.compare(w)
		if len(problems) == 0 {
			continue
		}

		if why, ok := ots72tDifferences[w.title]; ok {
			seen[w.title] = true

			t.Logf("%s differs as expected (%s): %s", w.title, why, strings.Join(problems, "; "))

			continue
		}

		t.Errorf("%s:\n\t%s", w.title, strings.Join(problems, "\n\t"))
	}

	for title := range ots72tDifferences {
		if !seen[title] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from ots72tDifferences", title)
		}
	}
}

// findFORRTL returns the path of a local volume holding VMS's FORRTL.EXE
// and the file's name on it, or skips the test.
func findFORRTL(t *testing.T) (disk, file string) {
	t.Helper()

	for _, v := range forrtlVolumes {
		path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "disks", v.disk))
		if err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(path); err == nil {
			return path, v.file
		}
	}

	t.Skip("no local volume with VMS's FORRTL.EXE (testdata/disks/README.md)")

	return "", ""
}

// ots72tCase is one case of the study's output: its line, the lines
// before its records (the text FORRTL wrote and the tracer's state), and
// the two records.
type ots72tCase struct {
	title    string
	text     []string
	value    string // the 16 bytes at R0, as printed
	jsb, rsb ots72tRecord
}

// ots72tRecord is what CAPTURE or RETCAP recorded: R0 to R11, AP, FP,
// and SP, and the 48 longwords from R1-176 to R1+16.
type ots72tRecord struct {
	regs  []uint32
	frame []uint32
}

// ots72tCases splits the study's output into its cases.
func ots72tCases(lines []string) []ots72tCase {
	var cases []ots72tCase

	for _, l := range lines {
		l = strings.TrimRight(l, "\r")

		if l == "" {
			continue
		}

		if !strings.HasPrefix(l, " ") {
			cases = append(cases, ots72tCase{title: l})

			continue
		}

		if len(cases) == 0 {
			continue
		}

		c := &cases[len(cases)-1]
		label, values, _ := strings.Cut(strings.TrimSpace(l), ": ")

		switch {
		case label == "value at R0":
			c.value = values
		case strings.HasPrefix(label, "JSB "):
			c.jsb.add(label, values)
		case strings.HasPrefix(label, "RSB "):
			c.rsb.add(label, values)
		default:
			c.text = append(c.text, l)
		}
	}

	return cases
}

// add puts a record's line into r: the registers, or a line of the frame.
func (r *ots72tRecord) add(label, values string) {
	var words []uint32

	for _, f := range strings.Fields(values) {
		v, err := strconv.ParseUint(f, 16, 32)
		if err != nil {
			return
		}

		words = append(words, uint32(v))
	}

	if strings.HasSuffix(label, "R0-R11 AP FP SP") {
		r.regs = words
	} else {
		r.frame = append(r.frame, words...)
	}
}

// ots72tFrameBase is the frame's first longword's offset from R1.
const ots72tFrameBase = -176

// at is the longword at offset off from R1 (FORRTL's FP) in the frame.
func (r ots72tRecord) at(off int) (uint32, bool) {
	i := (off - ots72tFrameBase) / 4
	if (off-ots72tFrameBase)%4 != 0 || i < 0 || i >= len(r.frame) {
		return 0, false
	}

	return r.frame[i], true
}

// byteAt is the byte at offset off from R1 in the frame.
func (r ots72tRecord) byteAt(off int) (byte, bool) {
	if off < ots72tFrameBase {
		return 0, false
	}

	base := off - (off-ots72tFrameBase)%4
	w, ok := r.at(base)

	return byte(w >> (8 * uint(off-base))), ok
}

// offset is a register's or a frame address's offset from R1.
func (r ots72tRecord) offset(address uint32) int {
	return int(int32(address - r.regs[1]))
}

// digits is the routine's digits from the first one on, as far as the
// buffer, size bytes, holds ASCII digits within the frame and before its
// last byte. (The size is the JSB's R2: VMS's routine changes R2.)
func (r ots72tRecord) digits(size uint32) string {
	buffer, ok1 := r.at(-20)
	first, ok2 := r.at(-32)

	if !ok1 || !ok2 || first >= size {
		return ""
	}

	var b strings.Builder

	for i := first; i+1 < size; i++ {
		d, ok := r.byteAt(r.offset(buffer + i))
		if !ok || d < '0' || d > '9' {
			break
		}

		b.WriteByte(d)
	}

	return b.String()
}

// ots72tFields are the record fields both sides decide, by name, read
// from a case's JSB record (FORRTL's arguments and frame) or RSB record
// (the routine's results).
var ots72tFields = []struct {
	name string
	get  func(c ots72tCase) string
}{
	// R0 is the value's address, in the program's own data.
	{"JSB R0", func(c ots72tCase) string { return ots72tHex(c.jsb.regs[0]) }},
	{"JSB R2 (the buffer's size)", func(c ots72tCase) string { return ots72tHex(c.jsb.regs[2]) }},
	{"JSB AP-FP", func(c ots72tCase) string { return strconv.Itoa(c.jsb.offset(c.jsb.regs[12])) }},
	{"JSB SP-FP", func(c ots72tCase) string { return strconv.Itoa(c.jsb.offset(c.jsb.regs[14])) }},
	{"JSB buffer-FP", func(c ots72tCase) string { return frameOffset(c.jsb, -20) }},
	{"JSB FP-9 (the kind)", func(c ots72tCase) string { return frameByte(c.jsb, -9) }},
	// FP-16 and FP-36 are read for E and F only; for the other kinds
	// they hold what the stack held.
	{"JSB FP-16 (E's digits)", func(c ots72tCase) string { return forKind(c, 0, -16) }},
	{"JSB FP-36 (F's digits)", func(c ots72tCase) string { return forKind(c, 2, -36) }},
	{"RSB SP-FP", func(c ots72tCase) string { return strconv.Itoa(c.rsb.offset(c.rsb.regs[14])) }},
	{"RSB FP-24 (the sign)", func(c ots72tCase) string { return frameWord(c.rsb, -24) }},
	{"RSB FP-28 (the exponent)", exponent},
	{"RSB guard", func(c ots72tCase) string { return guard(c.rsb) }},
}

// compare lists how g, govax's case, differs from w, VMS's.
func (g ots72tCase) compare(w ots72tCase) []string {
	var problems []string

	if gt, wt := strings.Join(g.text, "|"), strings.Join(w.text, "|"); gt != wt {
		problems = append(problems, fmt.Sprintf("text %q, VMS %q", gt, wt))
	}

	// A case the tracer didn't catch has no records to compare.
	if !strings.Contains(strings.Join(w.text, "|"), "tracer state: 00000002") {
		return problems
	}

	if len(g.jsb.regs) != 15 || len(g.rsb.regs) != 15 || len(g.jsb.frame) != 48 || len(g.rsb.frame) != 48 {
		return append(problems, "the records are incomplete")
	}

	if g.value != w.value {
		problems = append(problems, fmt.Sprintf("value at R0 %s, VMS %s", g.value, w.value))
	}

	for _, f := range ots72tFields {
		if gv, wv := f.get(g), f.get(w); gv != wv {
			problems = append(problems, fmt.Sprintf("%s %s, VMS %s", f.name, gv, wv))
		}
	}

	// VMS's buffer holds fewer digits than govax's (its routine works in
	// groups of nine); govax's must start with all of VMS's.
	if gd, wd := g.rsb.digits(g.jsb.regs[2]), w.rsb.digits(w.jsb.regs[2]); wd == "" || !strings.HasPrefix(gd, wd) {
		problems = append(problems, fmt.Sprintf("digits %q, VMS %q", gd, wd))
	}

	return problems
}

// guard is the byte before the first digit at the RSB ("0", the guard
// digit), or "none" when the first digit starts the buffer.
func guard(r ots72tRecord) string {
	buffer, _ := r.at(-20)
	first, _ := r.at(-32)

	if first == 0 {
		return "none"
	}

	b, ok := r.byteAt(r.offset(buffer + first - 1))
	if !ok {
		return "outside the frame"
	}

	return string(rune(b))
}

// forKind is the JSB's longword at off when the case's kind is kind,
// else "unused".
func forKind(c ots72tCase, kind byte, off int) string {
	if k, ok := c.jsb.byteAt(-9); !ok || k != kind {
		return "unused"
	}

	return frameWord(c.jsb, off)
}

// exponent is FP-28 at the RSB, or "unchanged" for a zero (sign 0),
// whose exponent the routine leaves as it was.
func exponent(c ots72tCase) string {
	sign, _ := c.rsb.at(-24)
	before, _ := c.jsb.at(-28)
	after, _ := c.rsb.at(-28)

	if sign == 0 && after == before {
		return "unchanged"
	}

	return frameWord(c.rsb, -28)
}

func ots72tHex(v uint32) string { return fmt.Sprintf("%08X", v) }

// frameWord is the longword at off, in hex.
func frameWord(r ots72tRecord, off int) string {
	v, ok := r.at(off)
	if !ok {
		return "outside the frame"
	}

	return ots72tHex(v)
}

// frameByte is the byte at off, in hex.
func frameByte(r ots72tRecord, off int) string {
	v, ok := r.byteAt(off)
	if !ok {
		return "outside the frame"
	}

	return fmt.Sprintf("%02X", v)
}

// frameOffset is the address in the longword at off as an offset from R1.
func frameOffset(r ots72tRecord, off int) string {
	v, ok := r.at(off)
	if !ok {
		return "outside the frame"
	}

	return strconv.Itoa(r.offset(v))
}

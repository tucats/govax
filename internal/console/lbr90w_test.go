package console

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
)

// lbr90wCases is how many cases LBR90W.MAR has, each run as an image of its
// own (LBR90W n), as HELP90W.COM's part B runs them.
const lbr90wCases = 36

// lbr90wLater are the cases of round 3 whose output depends on what later
// parts of subtask 11 do (docs/PHASE-90 - VMS help.md): the reason for each.
// Their output isn't compared yet.
var lbr90wLater = map[int]string{
	10: "SEARCH (11c)",
	12: "SEARCH (11c)",
	13: "PUT_HISTORY and FLUSH (11c)",
	14: "PUT_HISTORY and GET_HISTORY (11c)",
	15: "the history routines (11c)",
	16: "the history routines (11c)",
	17: "GET_HISTORY and SET_MODULE (11c), LIBRARY's history records (11e)",
	19: "FLUSH (11c)",
	26: "SEARCH and SET_MODULE (11c)",
	27: "binary keys (11d)",
	28: "the VMS V2 format (11d)",
	29: "the create options' defaults (11d)",
	30: "the create options' errors (11d)",
	31: "a second update open: VMS waited past the timer",
	33: "PUT_HISTORY (11c)",
	35: "the NAM block (11d)",
}

// lbr90wLibraries are the libraries the routines made whose blocks are held
// to VMS's (h90w.bck) byte for byte but for times and the librarian's name.
// The rest wait for later parts, as lbr90wLater's cases do, but W12, an
// object library whose module is H90O.OBJ's records: govax's MACRO's object
// isn't VMS's to the byte.
var lbr90wLibraries = []string{
	"W1.TLB;1", "W1.TLB;2", "W2.TLB;1", "W3.TLB;1", "W4.TLB;1", "W9.TLB;1",
	"W10.HLB;1", "W10T.HLB;1", "W11.MLB;1", "W16.TLB;1", "WE2.TLB;1", "WE3.TLB;1",
	"W6.TLB;1", "W7.TLB;1",
}

// TestLBR90WOracle runs round 3 of testdata/help90 (LBR90W.MAR, subtask
// 11) on govax, in both paging modes: part A's libraries made by LIBRARY,
// MACRO, and LINK, then each case an image of its own, its output held to
// VMS 7.1's (vax/round3/help90w.log) line for line but for lbr90wMasked's
// fields and lbr90wLater's cases; then the libraries the routines made are
// held to VMS's (vax/round3/h90w.bck) block for block. With LBR90W_DUMP
// naming a directory, govax's libraries are written there, to compare by
// hand.
func TestLBR90WOracle(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "help90")
	want := lbr90wVMSCases(t, filepath.Join(dir, "vax", "round3", "help90w.log"))
	vmsLibraries := lbr90wSaveSet(t, filepath.Join(dir, "vax", "round3", "h90w.bck"))

	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)
		mountFreshContainerSized(t, c, "DUA3:", 20000)

		for _, name := range []string{"t1.txt", "t2.txt", "h90o.mar", "h90om.mar", "w10.hlp", "lbr90w.mar"} {
			src, err := filepath.Abs(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}

			shareFindRun(t, d, `COPY "`+src+`"/HOST DUA3:[000000]`+strings.ToUpper(name))
		}

		// Part A's options file, for H90OS.EXE's universal symbols.
		opt := filepath.Join(t.TempDir(), "h90o.opt")
		if err := os.WriteFile(opt, []byte("UNIVERSAL=H90O_SYM,H90O_TWO\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY "`+opt+`"/HOST DUA3:[000000]H90O.OPT`)

		for _, cmd := range []string{
			"DEFINE H90DIR DUA3:[000000]",
			"SET DEFAULT DUA3:[000000]",
			"LIBRARY/CREATE/TEXT H90T T1,T2",
			"LIBRARY/REPLACE/TEXT H90T T2",
			"MACRO H90O",
			"MACRO H90OM",
			"LINK/SHAREABLE=H90OS H90O,H90O/OPTIONS",
			"LIBRARY/CREATE/SHARE H90S H90OS",
			"LIBRARY/CREATE/HELP W10L W10",
			"MACRO LBR90W",
			"LINK LBR90W",
			`LBR90W := $DUA3:[000000]LBR90W`,
		} {
			buf.Reset()
			_ = d.Dispatch(cmd)

			if out := buf.String(); out != "" && testing.Verbose() {
				t.Logf("%s:\n%s", cmd, out)
			}
		}

		for n := 1; n <= lbr90wCases; n++ {
			buf.Reset()
			_ = d.Dispatch(fmt.Sprintf("LBR90W %d", n))

			if why, later := lbr90wLater[n]; later {
				t.Logf("case %d not compared: %s", n, why)

				continue
			}

			got := strings.Split(strings.TrimSuffix(strings.ReplaceAll(buf.String(), "\r\n", "\n"), "\n"), "\n")
			lbr90wCompare(t, n, got, want[n])
		}

		for _, name := range lbr90wLibraries {
			data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA3:[000000]" + name})
			if err != nil {
				t.Errorf("%s: %v", name, err)

				continue
			}

			if dir := os.Getenv("LBR90W_DUMP"); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
			}

			lbr90wSameLibrary(t, name, data, vmsLibraries[name])
		}
	})
}

// lbr90wCompare holds case n's output to VMS's.
func lbr90wCompare(t *testing.T, n int, got, want []string) {
	t.Helper()

	got, want = lbr90wMaskedLines(got), lbr90wMaskedLines(want)

	for i := 0; i < max(len(got), len(want)); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			t.Errorf("case %d, line %d differs:\n got: %q\nwant: %q\n\ngot:\n%s\n\nwant:\n%s", n, i+1, g, w,
				strings.Join(got, "\n"), strings.Join(want, "\n"))

			return
		}
	}
}

// lbr90wVMSCases reads VMS's log of part B: each case's lines, by number.
func lbr90wVMSCases(t *testing.T, path string) map[int][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[int][]string{}
	n := 0

	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "$ LBR90W "); ok {
			if n, err = strconv.Atoi(rest); err != nil {
				n = 0
			}

			continue
		}

		if strings.HasPrefix(line, "$ ") {
			n = 0
		}

		if n > 0 {
			cases[n] = append(cases[n], line)
		}
	}

	if len(cases) != lbr90wCases {
		t.Fatalf("the log has %d cases, want %d", len(cases), lbr90wCases)
	}

	return cases
}

// lbr90wModuleHeader is a module header's first line, whose last eight
// bytes are its insertion time, and lbr90wImageTime a shareable image
// library module's second line, whose bytes 1 to 8 are the image's time.
var (
	lbr90wModuleHeader = regexp.MustCompile(`^(  0000: 00 AD (?:[0-9A-F]{2} ){6})(?:[0-9A-F]{2} ?){8}$`)
	lbr90wImageTime    = regexp.MustCompile(`^(  0010: [0-9A-F]{2} )(?:[0-9A-F]{2} ){8}`)
)

// lbr90wMaskedLines masks what may differ from VMS's in a case's lines: the
// header's librarian's name and times (lbr90Masked), and a module header's
// insertion time.
func lbr90wMaskedLines(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		line = lbr90Masked(line)
		if i > 0 && strings.HasPrefix(lines[i-1], "  module header len=") {
			line = lbr90wModuleHeader.ReplaceAllString(line, "${1}(time)")
		}

		if i > 1 && lines[i-2] == "  module header len=49" {
			line = lbr90wImageTime.ReplaceAllString(line, "${1}(time) ")
		}

		out[i] = line
	}

	return out
}

// lbr90wSaveSet reads the libraries in VMS's save set, by name and version.
func lbr90wSaveSet(t *testing.T, path string) map[string][]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	s, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	libs := map[string][]byte{}

	for _, f := range s.Files {
		name := f.Name[strings.LastIndex(f.Name, "]")+1:]
		if _, seen := libs[name]; !seen {
			libs[name] = f.Data()
		}
	}

	return libs
}

// lbr90wSameLibrary holds a library govax's routines made to VMS's, block
// for block, with the header's librarian's name and times and each module
// header's insertion time masked in both.
func lbr90wSameLibrary(t *testing.T, name string, got, want []byte) {
	t.Helper()

	if want == nil {
		t.Errorf("%s: not in VMS's save set", name)

		return
	}

	got, want = lbr90wMaskLibrary(got), lbr90wMaskLibrary(want)

	if len(got) != len(want) {
		t.Errorf("%s: %d blocks, VMS's %d", name, len(got)/512, len(want)/512)
	}

	for off := 0; off < min(len(got), len(want)); off++ {
		if got[off] != want[off] {
			blk := off / 512 * 512
			t.Errorf("%s: block %d differs at byte %d (%02X, VMS's %02X)\n got: % X\nwant: % X", name, off/512+1, off%512,
				got[off], want[off], got[blk:blk+64], want[blk:blk+64])

			return
		}
	}
}

// lbr90wMaskLibrary is a copy of a library with its times and librarian's
// name zeroed: the header's, and each module header's. A module header is a
// record of 16 bytes and the header's user data, starting 00 AD: found at
// every word of every block past the header, its bytes followed into the
// next block of the chain when it runs on past its own.
func lbr90wMaskLibrary(data []byte) []byte {
	out := append([]byte(nil), data...)
	if len(out) < 512 {
		return out
	}

	clear(out[0x0C:0x3C])

	blocks := uint32(len(out) / 512)
	n := 16 + int(out[0x3C])

	// place returns where byte i of a record whose bytes start at off of
	// block vbn is, or -1.
	place := func(vbn uint32, off, i int) int {
		for off+i >= 512 {
			i -= (512 - off + 1) &^ 1
			vbn = binary.LittleEndian.Uint32(out[int(vbn-1)*512+2:])
			off = 6

			if vbn == 0 || vbn > blocks {
				return -1
			}
		}

		return int(vbn-1)*512 + off + i
	}

	for v := uint32(2); v <= blocks; v++ {
		b := int(v-1) * 512

		for off := 6; off <= 510; off += 2 {
			if int(binary.LittleEndian.Uint16(out[b+off:])) != n {
				continue
			}

			body := off + 2
			p0, p1 := place(v, body, 0), place(v, body, 1)

			if p0 < 0 || p1 < 0 || out[p0] != 0 || out[p1] != 0xAD {
				continue
			}

			for i := 8; i < 16; i++ {
				if p := place(v, body, i); p >= 0 {
					out[p] = 0
				}
			}
		}
	}

	return out
}

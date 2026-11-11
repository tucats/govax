package console

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/isam"
)

// The VMS round's answers (testdata/vms88/vax/round2): what VMS 7.1 said of the files govax made
// and of the programs, against what govax says of the same on the same run of VMS88.COM.

// vms88SaveSet is a save set's files by name, with no version, from a host file.
func vms88SaveSet(t *testing.T, path string) map[string]*backup.File {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]*backup.File{}

	for _, f := range set.Files {
		name, _, _ := strings.Cut(f.Name, ";")
		files[name] = f
	}

	return files
}

// vms88Text is a saved text file's lines, from its variable-length records.
func vms88Text(f *backup.File) []string {
	var lines []string

	data := f.Data()

	for at := 0; at+2 <= len(data); {
		n := int(binary.LittleEndian.Uint16(data[at:]))
		if n == 0xFFFF {
			at = (at/512 + 1) * 512

			continue
		}

		at += 2
		if at+n > len(data) {
			break
		}

		lines = append(lines, string(data[at:at+n]))
		at += n + n&1
	}

	return lines
}

// vms88Dates are what a report says of when it ran and when its file was made:
// a day of one digit is blank-padded to two, so the mask takes the pad
// with the day, and a date masks the same on the 8th as on the 18th.
var vms88Dates = regexp.MustCompile(`[ \d]?\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d(\.\d\d)?|File ID: \(\d+,\d+,\d+\)|;2$`)

// vms88LogDifferences are the log's sections that differ from VMS's, with why. Round 2's
// is the log held to (round 1's, vax/vms88.log, had FTNRTL fail to build).
var vms88LogDifferences = map[string]string{
	"END": "ANALYZE/DISK_STRUCTURE, which govax lacks",
}

// vms88ReportDifferences are VMS's reports that aren't govax's, with why.
var vms88ReportDifferences = map[string]string{
	"[RESULTS]G_G117.CK2":   "G117's split, the phase doc's known one",
	"[RESULTS]F_B78.CK2":    "contiguity after VMS's changes extended its copy; govax's extension stays one run",
	"[RESULTS]F_B79.CK2":    "the same",
	"[RESULTS]R_PHONES.FDL": "DATA_FILL on a key with deleted pointers, one point apart, by no rule found",
	"[RESULTS]P_LOAD.CHK":   "the allocation of LOAD.IDX as FTNRTL made it on VMS, its splits G117's class",
	"[RESULTS]P_LOADV.CHK":  "the same, for LOADV.IDX",
}

// vms88FileDifferences are the files VMS's RMS changed or made that govax's doesn't make
// the same bytes of, with why.
var vms88FileDifferences = map[string]string{
	"[M88]G_G117.IDX": "G117's split",
	"[M88]G_G118.IDX": "G118's split, its class",
	"[M88]G_F122.IDX": "a split, G117's class",
	"[M88]G_Q124.IDX": "where a SIDR bucket splits",
	"[M88]G_Q125.IDX": "the same",
	"[M88]G_G108.IDX": "a deleted record's stub kept beside a record put after it, by no rule found",
	"[P88]LOAD.IDX":   "the splits of 2,500 records, G117's class; the same records and walks",
	"[P88]LOADV.IDX":  "the same",
	"[P88]PHONES.IDX": "a bucket's last alternate value with no live pointer: VMS took it out here, where it kept " +
		"one in R_PHONES and the D2 class",
}

func vms88Oracle(t *testing.T, d *Dispatcher, sections map[string]string) {
	t.Helper()

	vax := filepath.Join("testdata", "vms88", "vax", "round2")

	logText, err := os.ReadFile(filepath.Join(vax, "vms88.log"))
	if err != nil {
		t.Fatal(err)
	}

	vmsSections := ftn86Sections(string(logText))
	for name, text := range vmsSections {
		ours := strings.Join(slicesDeleteFunc(strings.Split(sections[name], "\n"), func(l string) bool {
			return strings.HasPrefix(l, "%COPY-S-COPIED")
		}), "\n")
		why, expected := vms88LogDifferences[name]

		switch same := ours == text; {
		case same && expected:
			t.Errorf("%s matches VMS's log: take it out of vms88LogDifferences (%s)", name, why)
		case !same && !expected:
			t.Errorf("%s:\n%s", name, firstDifference(text, ours))
		}
	}

	host := t.TempDir()
	for _, name := range []string{"RESULTS", "CHANGED"} {
		shareFindRun(t, d, "COPY/QUIET/BINARY DUA1:[000000]"+name+`.BCK "`+filepath.Join(host, name+".bck")+`"/HOST`)
	}

	// VMS's reports on govax's files, against govax's.
	want, got := vms88SaveSet(t, filepath.Join(vax, "results.bck")), vms88SaveSet(t, filepath.Join(host, "RESULTS.bck"))
	same, differ := 0, map[string]string{}

	for name, f := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: govax made no such report", name)

			continue
		}

		a, b := vms88Text(f), vms88Text(g)
		line := 0

		for line < len(a) && line < len(b) &&
			vms88Dates.ReplaceAllString(strings.TrimRight(a[line], " "), "") == vms88Dates.ReplaceAllString(strings.TrimRight(b[line], " "), "") {
			line++
		}

		if line == len(a) && line == len(b) {
			same++

			continue
		}

		var x, y string
		if line < len(a) {
			x = a[line]
		}

		if line < len(b) {
			y = b[line]
		}

		differ[name] = "line " + itoa(line+1) + "\n   VMS:   " + x + "\n   govax: " + y
	}

	t.Logf("reports: %d the same, %d not", same, len(differ))

	for name, why := range differ {
		if _, ok := vms88ReportDifferences[name]; !ok {
			t.Errorf("%s: %s", name, why)
		}
	}

	for name := range vms88ReportDifferences {
		if _, ok := differ[name]; !ok {
			t.Errorf("%s matches VMS's: take it out of vms88ReportDifferences", name)
		}
	}

	// The files VMS's RMS changed, against what govax's RMS made of the same changes.
	want, got = vms88SaveSet(t, filepath.Join(vax, "changed.bck")), vms88SaveSet(t, filepath.Join(host, "CHANGED.bck"))
	same = 0
	structure := 0

	var other []string

	for name, f := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: govax made no such file", name)

			continue
		}

		if bytes.Equal(f.Data(), g.Data()) {
			same++

			continue
		}

		a, errA := vms88Structure(f)
		b, errB := vms88Structure(g)

		switch {
		case errA != nil || errB != nil:
			other = append(other, name+" (unreadable)")
		case a == b:
			structure++
		default:
			x, y := strings.Split(a, "\n"), strings.Split(b, "\n")
			i := 0

			for i < len(x) && i < len(y) && x[i] == y[i] {
				i++
			}

			other = append(other, name)

			if i < len(x) && i < len(y) {
				t.Logf("%s, dump line %d:\n   VMS:   %s\n   govax: %s", name, i+1, x[i], y[i])
			}
		}
	}

	t.Logf("changed files: %d byte for byte, %d the same structure, %d not", same, structure, len(other))

	for _, name := range other {
		if _, ok := vms88FileDifferences[name]; !ok {
			t.Errorf("%s: VMS's RMS changed it otherwise", name)
		}
	}

	for name := range vms88FileDifferences {
		if !slicesContains(other, name) {
			t.Errorf("%s is VMS's: take it out of vms88FileDifferences", name)
		}
	}
}

// vms88Blocks is a saved file's bytes as isam reads them.
type vms88Blocks []byte

func (b vms88Blocks) ReadBlock(vbn uint32, block []byte) error {
	copy(block, b[(vbn-1)*512:])

	return nil
}

func (b vms88Blocks) Size() uint32 { return uint32(len(b) / 512) }

// vms88Structure is an indexed file's Dump, which leaves out unused bytes.
func vms88Structure(f *backup.File) (string, error) {
	format := isam.Variable
	if f.RecordAttributes[0]&0x0F == 1 {
		format = isam.Fixed
	}

	m, err := isam.Open(vms88Blocks(f.Data()), isam.Attributes{Format: format, Size: int(binary.LittleEndian.Uint16(f.RecordAttributes[16:]))})
	if err != nil {
		return "", err
	}

	var text strings.Builder
	err = m.Dump(&text)

	return text.String(), err
}

func slicesDeleteFunc(s []string, f func(string) bool) []string {
	var out []string

	for _, x := range s {
		if !f(x) {
			out = append(out, x)
		}
	}

	return out
}

func slicesContains(s []string, x string) bool {
	for _, y := range s {
		if y == x {
			return true
		}
	}

	return false
}

func itoa(n int) string {
	var b strings.Builder

	if n >= 10 {
		b.WriteString(itoa(n / 10))
	}

	b.WriteByte(byte('0' + n%10))

	return b.String()
}

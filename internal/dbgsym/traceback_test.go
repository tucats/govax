package dbgsym

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vmsimage"
)

// tb70VAX holds Phase 70's probe's VMS images (testdata/tb70).
const tb70VAX = "../../testdata/tb70/vax"

func readTB70Image(t *testing.T, name string) *Program {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(tb70VAX, name+".exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	p, err := Read(img, data, 0)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// TestTraceName names the PCs VMS's traceback named in testdata/tb70's
// logs, by each of TraceName's rules: a routine within 400 of the PC
// (compared signed, so a PC in S0 is the last routine's), a psect, or
// nothing; and with /DEBUG's line numbers and its unnamed routine.
func TestTraceName(t *testing.T) {
	long := "TB70_LONG_MODULE_NAME_IS_31_CHR"

	cases := []struct {
		image string
		pc    uint32
		exact bool
		want  TraceName
	}{
		{"tb70", 0x515, false, TraceName{"TB70", "PART_S", 0x88, 0}},
		{"tb70", 0xF46, false, TraceName{long, "TB70_LONG_ROUTINE_NAME_31_CHRSX", 0x2D, 0}},
		{"tb70", 0x8192B447, false, TraceName{long, "TB70_LONG_ROUTINE_NAME_31_CHRSX", 0x8192A52E, 0}},
		{"tb70", 0x898, false, TraceName{"TB70", "CODE", 0x498, 0}},
		{"tb70", 0x2A5, true, TraceName{"TB70", "DATA", 0xA5, 0}},
		{"tb70", 0x3FFF0000, true, TraceName{"", "", 0x3FFF0000, 0}},
		{"tb70b", 0x9C0, false, TraceName{"TB70B", "R3FF", 0x3FF, 0}},
		{"tb70b", 0xDC1, false, TraceName{"TB70B", "CODE", 0x9C1, 0}},
		{"tb70d", 0x515, false, TraceName{"TB70", "PART_S", 0x88, 172}},
		{"tb70d", 0xF46, false, TraceName{"TB70", "", 0xE, 494}},
		{"tb70d", 0x8192B447, false, TraceName{"TB70", "", 0x8192A50F, 0}},
		{"tb70d", 0xEBD, true, TraceName{"TB70", "PART_C", 0x13, 472}},
		{"tb70d", 0xEBD, false, TraceName{"TB70", "PART_C", 0x13, 471}},
	}

	programs := map[string]*Program{}

	for _, c := range cases {
		p := programs[c.image]
		if p == nil {
			p = readTB70Image(t, c.image)
			programs[c.image] = p
		}

		if got := p.TraceName(c.pc, c.exact); got != c.want {
			t.Errorf("%s: TraceName(%08X, %v) = %+v, want %+v", c.image, c.pc, c.exact, got, c.want)
		}
	}

	var none *Program
	if got := none.TraceName(0x400, false); got != (TraceName{Rel: 0x400}) {
		t.Errorf("no program: %+v", got)
	}
}

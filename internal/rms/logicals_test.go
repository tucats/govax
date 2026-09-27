package rms

import (
	"testing"

	"github.com/tucats/govax/internal/lnm"
)

// newTestLogicals returns a logical-name database seeded the way the
// console seeds its own: SYS$INPUT/OUTPUT/ERROR/COMMAND and TT all
// equated to the console terminal, "_TTA0:".
func newTestLogicals(t *testing.T) *lnm.Database {
	t.Helper()

	db := lnm.NewDatabase(1<<16 | 4)
	if err := db.DefineProcessNames("_TTA0:"); err != nil {
		t.Fatalf("DefineProcessNames: %v", err)
	}

	return db
}

// defineTestLogical defines name = value in the process table.
func defineTestLogical(t *testing.T, db *lnm.Database, name, value string) {
	t.Helper()

	if _, err := db.Define("LNM$PROCESS", name, lnm.Supervisor, 0, []lnm.Equivalence{{Value: value}}); err != nil {
		t.Fatalf("Define(%s): %v", name, err)
	}
}

func TestTranslateWholeSpec(t *testing.T) {
	ctx := &Context{Logicals: newTestLogicals(t)}
	defineTestLogical(t, ctx.Logicals, "MYFILE", "DUA0:[X]Y.DAT")

	tests := []struct{ in, want string }{
		{"SYS$OUTPUT", "_TTA0:"},
		{"MYFILE", "DUA0:[X]Y.DAT"},
		{"NOSUCH", "NOSUCH"},
		{"MYFILE:Z.DAT", "MYFILE:Z.DAT"}, // whole-string only until subtask 7
		{"", ""},
	}

	for _, tt := range tests {
		if got := ctx.translateWholeSpec(tt.in); got != tt.want {
			t.Errorf("translateWholeSpec(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	if got := (&Context{}).translateWholeSpec("SYS$OUTPUT"); got != "SYS$OUTPUT" {
		t.Errorf("nil Logicals: got %q", got)
	}
}

func TestNormalizeDeviceName_underscore(t *testing.T) {
	for _, in := range []string{"TTA0", "tta0:", "_TTA0:", "_tta0"} {
		if got := normalizeDeviceName(in); got != "TTA0" {
			t.Errorf("normalizeDeviceName(%q) = %q, want TTA0", in, got)
		}
	}
}

package rms

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
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

// defineTestLogical defines name = values (a search list if more than
// one) in the process table.
func defineTestLogical(t *testing.T, db *lnm.Database, name string, values ...string) {
	t.Helper()

	defineTestLogicalAttrs(t, db, name, 0, values...)
}

func defineTestLogicalAttrs(t *testing.T, db *lnm.Database, name string, attrs uint32, values ...string) {
	t.Helper()

	eqv := make([]lnm.Equivalence, len(values))
	for i, v := range values {
		eqv[i] = lnm.Equivalence{Value: v, Attrs: attrs}
	}

	if _, err := db.Define("LNM$PROCESS", name, lnm.Supervisor, 0, eqv); err != nil {
		t.Fatalf("Define(%s): %v", name, err)
	}
}

// newTwoVolumeSession mounts two writable test volumes, on DUA0 and DUA1,
// and returns a Session over them (with its own logical names) and the
// two volumes.
func newTwoVolumeSession(t *testing.T) (*Session, *volume.Volume, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()

	vols := make([]*volume.Volume, 0, 2)

	for _, dev := range []string{"DUA0", "DUA1"} {
		if err := mounts.Mount(dev, newTestVolumeFile(t, dev+"VOL"), true); err != nil {
			t.Fatalf("Mount(%s): %v", dev, err)
		}

		vol, _ := mounts.Lookup(dev)
		vols = append(vols, vol)
	}

	s := NewSession(mounts)
	s.Logicals = newTestLogicals(t)

	return s, vols[0], vols[1]
}

func TestExpandSpec_precedence(t *testing.T) {
	db := newTestLogicals(t)
	defineTestLogical(t, db, "PAY_FILE", "DISK1:[SALES_STAFF]PAYROLL")
	defineTestLogical(t, db, "FIFI", "DISK1:[FRED]", "DISK2:[GLADYS]", "DISK3:")
	defineTestLogical(t, db, "GETTYSBURG", "[JONES.HISTORY]", "[JONES.WORKFILES]")
	defineTestLogical(t, db, "TREE", "DISK1:[A...]")
	defineTestLogicalAttrs(t, db, "HIDDEN", lnm.AttrConcealed, "DJA3:")
	defineTestLogical(t, db, "SYS$DISK", "DISK9:")

	base := filespec.Spec{Dirs: []string{"MEATBALL", "SUB"}}

	tests := []struct {
		text string
		want []string // Display-device + spec text, one per result
	}{
		// The fields written after the name beat the translation's.
		{"PAY_FILE:*.DAT", []string{"DISK1:[SALES_STAFF]*.DAT"}},
		{"PAY_FILE:", []string{"DISK1:[SALES_STAFF]PAYROLL"}},
		// A search list element's directory beats the default's; one
		// without a directory gets the default's.
		{"FIFI:MEMO.LIS", []string{"DISK1:[FRED]MEMO.LIS", "DISK2:[GLADYS]MEMO.LIS", "DISK3:[MEATBALL.SUB]MEMO.LIS"}},
		// No device: SYS$DISK's.
		{"GETTYSBURG:SPEECH.TXT", []string{"DISK9:[JONES.HISTORY]SPEECH.TXT", "DISK9:[JONES.WORKFILES]SPEECH.TXT"}},
		{"X.DAT", []string{"DISK9:[MEATBALL.SUB]X.DAT"}},
		{"DISK4:X.DAT", []string{"DISK4:[MEATBALL.SUB]X.DAT"}},
		{"_DISK4:X.DAT", []string{"DISK4:[MEATBALL.SUB]X.DAT"}},
		// A concealed device is displayed by its logical name.
		{"HIDDEN:[SAM]X.DAT", []string{"HIDDEN|DJA3:[SAM]X.DAT"}},
	}

	for _, tt := range tests {
		specs, err := expandSpec(db, tt.text, base)
		if err != nil {
			t.Fatalf("expandSpec(%q): %v", tt.text, err)
		}

		var got []string

		for _, r := range specs {
			s := r.Spec.String()
			if r.Display != r.Spec.Device {
				s = r.Display + "|" + s
			}

			got = append(got, s)
		}

		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("expandSpec(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}

	specs, err := expandSpec(db, "TREE:X.DAT", base)
	if err != nil || !specs[0].Spec.Recursive {
		t.Errorf("TREE:X.DAT = %+v, %v; want the [A...] recursion kept", specs, err)
	}
}

func TestExpandSpec_logicalNameError(t *testing.T) {
	db := newTestLogicals(t)
	defineTestLogical(t, db, "A", "B:")
	defineTestLogical(t, db, "B", "A:")

	_, err := expandSpec(db, "A:X.DAT", filespec.Spec{})

	var lne *LogicalNameError
	if !errors.As(err, &lne) || !errors.Is(err, vmserrors.New(vmserrors.SS_TOOMANYLNAM)) {
		t.Errorf("err = %v, want a LogicalNameError wrapping SS$_TOOMANYLNAM", err)
	}

	// The same through a service's status mapping.
	ctx := &Context{Logicals: db}
	if _, status := ctx.resolveFileSpec("A:X.DAT"); status != rmsLogicalNameError {
		t.Errorf("resolveFileSpec status = %#x, want RMS$_LNE", status)
	}

	if _, status := ctx.resolveFileSpec("[UNCLOSED"); status != rmsFileNotFound {
		t.Errorf("resolveFileSpec status = %#x, want RMS$_FNF", status)
	}
}

func TestSession_setDefaultUsesSysDisk(t *testing.T) {
	s, _, _ := newTwoVolumeSession(t)

	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatal(err)
	}

	e, err := s.Logicals.Translate("LNM$PROCESS", "SYS$DISK", lnm.User, 0)
	if err != nil || e.Equivalences[0].Value != "DUA0:" || e.Mode != lnm.Supervisor {
		t.Fatalf("SYS$DISK = %+v, %v", e, err)
	}

	if s.Default.Device != "" {
		t.Errorf("Default.Device = %q, want it kept in SYS$DISK only", s.Default.Device)
	}

	// A plain device logical is translated; a concealed one is kept.
	defineTestLogical(t, s.Logicals, "WORK", "DUA1:[TOM]")
	defineTestLogicalAttrs(t, s.Logicals, "DISK", lnm.AttrConcealed, "DUA0:")

	for _, tt := range []struct{ text, want string }{
		{"WORK:", "DUA1:[TOM]"},
		{"DISK:[SAM.PUP]", "DISK:[SAM.PUP]"},
		{"[.SUB]", "DISK:[SAM.PUP.SUB]"},
	} {
		if err := s.SetDefault(tt.text); err != nil {
			t.Fatal(err)
		}

		if got := s.DefaultString(); got != tt.want {
			t.Errorf("SET DEFAULT %s: SHOW DEFAULT = %q, want %q", tt.text, got, tt.want)
		}
	}
}

// TestSession_setDefaultSearchList is User's Manual §11.7.2's FIFI
// example: SYS$DISK gets the untranslated search list name, the
// directory is unchanged, and SHOW DEFAULT lists each element.
func TestSession_setDefaultSearchList(t *testing.T) {
	s, _, _ := newTwoVolumeSession(t)

	if err := s.SetDefault("DISK2:[MEATBALL.SUB]"); err != nil {
		t.Fatal(err)
	}

	defineTestLogical(t, s.Logicals, "FIFI", "DISK1:[FRED]", "DISK2:[GLADYS]", "DISK3:")

	if err := s.SetDefault("FIFI"); err != nil {
		t.Fatal(err)
	}

	want := "FIFI:[MEATBALL.SUB]\n=   DISK1:[FRED]\n=   DISK2:[GLADYS]\n=   DISK3:[MEATBALL.SUB]"
	if got := s.DefaultString(); got != want {
		t.Errorf("SHOW DEFAULT = %q, want %q", got, want)
	}
}

// TestSession_searchListCommands exercises how each command uses a search
// list over two volumes (User's Manual §11.7 and §11.7.1).
func TestSession_searchListCommands(t *testing.T) {
	s, vol0, vol1 := newTwoVolumeSession(t)
	createTestFile(t, vol0, "SPEECH.TXT", "fourscore")
	createTestFile(t, vol1, "SPEECH.TXT", "and seven")
	createTestFile(t, vol1, "ONLY1.TXT", "second")
	defineTestLogical(t, s.Logicals, "BOTH", "DUA0:", "DUA1:")
	defineTestLogical(t, s.Logicals, "WITHGAP", "DUA7:", "DUA1:")

	// DIRECTORY lists every element.
	text, err := s.Directory("BOTH:SPEECH.TXT", DirectoryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(text, "Directory DUA0:[]") || !strings.Contains(text, "Directory DUA1:[]") ||
		!strings.Contains(text, "Total of 2 file(s).") {
		t.Errorf("DIRECTORY BOTH:SPEECH.TXT =\n%s", text)
	}

	// TYPE takes the first file found, skipping an element that lacks it
	// or whose device isn't mounted.
	for _, tt := range []struct{ spec, want string }{
		{"BOTH:SPEECH.TXT", "fourscore"},
		{"BOTH:ONLY1.TXT", "second"},
		{"WITHGAP:ONLY1.TXT", "second"},
	} {
		got, err := s.Type(tt.spec)
		if err != nil || !strings.HasPrefix(got, tt.want) {
			t.Errorf("TYPE %s = %q, %v; want %q", tt.spec, got, err, tt.want)
		}
	}

	// Not found anywhere: the last element's error.
	var notFound *NotFoundError
	if _, err := s.Type("BOTH:NOSUCH.TXT"); !errors.As(err, &notFound) {
		t.Errorf("TYPE BOTH:NOSUCH.TXT err = %v, want NotFoundError", err)
	}

	// DELETE deletes in every element, and fails only if none matched.
	deleted, err := s.Delete("BOTH:SPEECH.TXT;*")
	if err != nil || len(deleted) != 2 {
		t.Errorf("DELETE BOTH:SPEECH.TXT;* = %v, %v; want two files", deleted, err)
	}

	if _, err := s.Delete("BOTH:SPEECH.TXT;*"); !errors.As(err, &notFound) {
		t.Errorf("second DELETE err = %v, want NotFoundError", err)
	}

	deleted, err = s.Delete("BOTH:ONLY1.TXT;*")
	if err != nil || len(deleted) != 1 {
		t.Errorf("DELETE BOTH:ONLY1.TXT;* = %v, %v; want one file", deleted, err)
	}

	// PURGE runs in each element; an unmounted element is skipped.
	if _, err := s.Purge("WITHGAP:*.*", 1); err != nil {
		t.Errorf("PURGE WITHGAP:*.* = %v", err)
	}

	var notMounted *NotMountedError
	if _, err := s.Directory("DUA7:*.*", DirectoryOptions{}); !errors.As(err, &notMounted) {
		t.Errorf("DIRECTORY DUA7: err = %v, want NotMountedError", err)
	}
}

// TestSession_sysDiskSearchList: with SYS$DISK a search list, a spec with
// no device searches each element.
func TestSession_sysDiskSearchList(t *testing.T) {
	s, _, vol1 := newTwoVolumeSession(t)
	createTestFile(t, vol1, "HERE.TXT", "found it")
	defineTestLogical(t, s.Logicals, "BOTH", "DUA0:", "DUA1:")

	if err := s.SetDefault("BOTH:"); err != nil {
		t.Fatal(err)
	}

	got, err := s.Type("HERE.TXT")
	if err != nil || !strings.HasPrefix(got, "found it") {
		t.Errorf("TYPE HERE.TXT = %q, %v", got, err)
	}
}

func TestPhysicalDevice(t *testing.T) {
	db := newTestLogicals(t)
	defineTestLogical(t, db, "DISK", "DUA3:")
	defineTestLogical(t, db, "LIST", "DUA4:[X]", "DUA5:")

	for _, tt := range []struct{ text, want string }{
		{"DUA0:", "DUA0"},
		{"DUA0", "DUA0"},
		{"_DUA0:", "DUA0"},
		{"DISK:", "DUA3"},
		{"DISK", "DUA3"},
		{"_DISK:", "DISK"},
		{"LIST", "DUA4"},
	} {
		got, err := PhysicalDevice(db, tt.text)
		if err != nil || got != tt.want {
			t.Errorf("PhysicalDevice(%q) = %q, %v; want %q", tt.text, got, err, tt.want)
		}
	}

	defineTestLogical(t, db, "NODEV", "[DIR]")

	if _, err := PhysicalDevice(db, "NODEV"); err == nil {
		t.Error("PhysicalDevice(NODEV) succeeded with no device")
	}
}

func TestNormalizeDeviceName_underscore(t *testing.T) {
	for _, in := range []string{"TTA0", "tta0:", "_TTA0:", "_tta0"} {
		if got := normalizeDeviceName(in); got != "TTA0" {
			t.Errorf("normalizeDeviceName(%q) = %q, want TTA0", in, got)
		}
	}
}

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
		{"_DISK4:X.DAT", []string{"_DISK4|DISK4:[MEATBALL.SUB]X.DAT"}},
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

	want := "FIFI:[MEATBALL.SUB]\n  =   DISK1:[FRED]\n  =   DISK2:[GLADYS]\n  =   DISK3:[MEATBALL.SUB]"
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
	text, _, err := s.Directory("BOTH:SPEECH.TXT", DirectoryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(text, "Directory DUA0:[000000]") || !strings.Contains(text, "Directory DUA1:[000000]") ||
		!strings.Contains(text, "Grand total of 2 directories, 2 files.") {
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
	if _, _, err := s.Directory("DUA7:*.*", DirectoryOptions{}); !errors.As(err, &notMounted) {
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

// A device's full name has its node's name in front ($GETDVI's
// FULLDEVNAM, NAM$T_DVI), or an allocation class: either names the same
// device.
func TestNormalizeDeviceName_prefix(t *testing.T) {
	for _, in := range []string{"_GOVAX$DUA1:", "govax$dua1", "$1$DUA1:", "_SIMVAX$DUA1"} {
		if got := normalizeDeviceName(in); got != "DUA1" {
			t.Errorf("normalizeDeviceName(%q) = %q, want DUA1", in, got)
		}
	}
}

// A concealed name that SYS$DISK holds is the device a name with none is
// shown on, as it is for a name that gives it itself (docs/planning/PHASE-91, A.3;
// VMS 7.1: testdata/lnm91's SET DEFAULT CDISK:[LNM91.ROOT1]).
func TestExpandSpec_concealedSysDisk(t *testing.T) {
	s, _, _ := newTwoVolumeSession(t)
	defineTestLogicalAttrs(t, s.Logicals, "CDISK", lnm.AttrConcealed, "DUA1:")

	if err := s.SetDefault("CDISK:[A]"); err != nil {
		t.Fatal(err)
	}

	specs, err := expandSpec(s.Logicals, "X.DAT", s.Default)
	if err != nil {
		t.Fatal(err)
	}

	if r := specs[0]; r.Display != "CDISK" || r.Spec.Device != "DUA1" {
		t.Errorf("X.DAT is on %s (%s), want CDISK (DUA1)", r.Display, r.Spec.Device)
	}

	if got, err := s.ExpandName("X.DAT"); err != nil || got != "CDISK:[A]X.DAT;" {
		t.Errorf("ExpandName = %q, %v", got, err)
	}
}

// F$PARSE is $PARSE's name processing: the same text for the same name.
func TestParseFollowsNameProcessing(t *testing.T) {
	s, _, _ := newTwoVolumeSession(t)
	defineTestLogicalAttrs(t, s.Logicals, "CDISK", lnm.AttrConcealed, "DUA1:")
	defineTestLogical(t, s.Logicals, "PLAIN", "_DUA1:[A]")

	if err := s.SetDefault("DUA0:[A.B]"); err != nil {
		t.Fatal(err)
	}

	ctx := &Context{Mounts: s.Mounts, Logicals: s.Logicals, Session: s}

	for _, tt := range []struct {
		spec, def string
		noConceal bool
	}{
		{spec: "X.DAT"},
		{spec: "[.C]X"},
		{spec: "[-]X.DAT;3"},
		{spec: "[...]*.*;*"},
		{spec: "[000000]X"},
		{spec: "CDISK:[Z]X.DAT"},
		{spec: "CDISK:[Z]X.DAT", noConceal: true},
		{spec: "PLAIN:X.DAT"},
		{spec: "X", def: "CDISK:[Y].TXT"},
		{spec: "[.K]X", def: "[Y]"},
	} {
		names, sts := ctx.expandName(nameInputs{Primary: tt.spec, Default: tt.def, NoConceal: tt.noConceal})
		if sts != 0 {
			t.Errorf("expandName(%q): status %#x", tt.spec, sts)

			continue
		}

		p, _, err := s.Parse(tt.spec, tt.def, "", true, tt.noConceal)
		if err != nil || p.String() != names[0].String() {
			t.Errorf("Parse(%q, %q) = %q, %v; $PARSE gives %q", tt.spec, tt.def, p, err, names[0])
		}
	}

	// A related file gives F$PARSE only the name and type.
	p, _, err := s.Parse("", "", "DUA1:[R]REL.TXT;4", true, false)
	if err != nil || p.String() != "DUA0:[A.B]REL.TXT;" {
		t.Errorf("related = %q, %v", p, err)
	}

	if _, _, err := s.Parse("A.B.C", "", "", true, false); err == nil {
		t.Error("F$PARSE of A.B.C: no error")
	}
}

// Against stands in for the process defaults, for a name defaulted by the
// name before it in a list or by the file being renamed.
func TestExpandNames_against(t *testing.T) {
	db := newTestLogicals(t)
	defineTestLogical(t, db, "SYS$DISK", "DISK9:")
	defineTestLogical(t, db, "ELSE", "DISK2:[Q]")

	dua1 := translatedName{fileName: fileName{Dev: "DUA1:"}}
	against := defaultName{Disk: dua1, Dirs: []string{"A", "B"}, Name: "OLD", Type: ".TXT"}
	process := filespec.Spec{Dirs: []string{"PROCESS"}}

	for _, tt := range []struct{ text, want string }{
		{"", "DUA1:[A.B]OLD.TXT;"},
		{"NEW", "DUA1:[A.B]NEW.TXT;"},
		{".DAT", "DUA1:[A.B]OLD.DAT;"},
		{"[.C]", "DUA1:[A.B.C]OLD.TXT;"},
		{"[-]", "DUA1:[A]OLD.TXT;"},
		{"[Z]", "DUA1:[Z]OLD.TXT;"},
		{"DUA2:", "DUA2:[A.B]OLD.TXT;"},
		{"ELSE:", "DISK2:[Q]OLD.TXT;"},
	} {
		names, sts := expandNames(db, process, nameInputs{Primary: tt.text, Against: &against})
		if sts != 0 || names[0].String() != tt.want {
			t.Errorf("%q against = %v, %#x; want %s", tt.text, names, sts, tt.want)
		}
	}

	tree := defaultName{Disk: dua1, Dirs: []string{"A"}, Recursive: true}

	names, sts := expandNames(db, process, nameInputs{Primary: "X", Against: &tree})
	if sts != 0 || !names[0].spec().Recursive || names[0].String() != "DUA1:[A...]X.;" {
		t.Errorf("X against a tree = %v, %#x", names, sts)
	}

	// An empty one is no defaults at all: not SYS$DISK, not the process's.
	names, sts = expandNames(db, process, nameInputs{Primary: "X", Against: &defaultName{}})
	if sts != 0 || names[0].Dev != "" || len(names[0].spec().Dirs) != 0 {
		t.Errorf("X against nothing = %v, %#x", names, sts)
	}
}

// TestShownDirectory: a file found through a root is shown relative to it,
// and a root that isn't concealed is shown before the directory (VMS 7.1:
// testdata/lnm91).
func TestShownDirectory(t *testing.T) {
	db := newTestLogicals(t)

	concealed := []lnm.Equivalence{{Value: "_DUA1:[R1.R2.]", Attrs: lnm.AttrConcealed | lnm.AttrTerminal}}
	if _, err := db.Define(lnm.ProcessTableName, "BASE", lnm.Supervisor, 0, concealed); err != nil {
		t.Fatal(err)
	}

	defineTestLogical(t, db, "NOCON", "_DUA1:[R1.R2.]")

	process := filespec.Spec{Dirs: []string{"JONES"}}

	for _, tt := range []struct {
		text string
		dirs []string
		want string
	}{
		{"BASE:[SUB]", []string{"R1", "R2", "SUB"}, "BASE:[SUB]"},
		{"BASE:[*...]", []string{"R1", "R2", "SUB", "K"}, "BASE:[SUB.K]"},
		{"BASE:[000000]", []string{"R1", "R2"}, "BASE:[000000]"},
		{"BASE:[000000...]", []string{"R1", "R2", "SUB"}, "BASE:[000000.SUB]"},
		{"NOCON:[SUB]", []string{"R1", "R2", "SUB"}, "_DUA1:[R1.R2.][SUB]"},
		{"_DUA1:[SUB]", []string{"SUB"}, "_DUA1:[SUB]"},
		{"DUA1:<SUB>", []string{"SUB"}, "DUA1:<SUB>"},
	} {
		specs, err := expandSpec(db, tt.text, process)
		if err != nil {
			t.Errorf("%s: %v", tt.text, err)

			continue
		}

		if got := specs[0].shownDir(tt.dirs); got != tt.want {
			t.Errorf("%s shows %v as %q, want %q", tt.text, tt.dirs, got, tt.want)
		}
	}
}

// A name processing refuses is a *NameError with RMS's status.
func TestExpandSpec_nameError(t *testing.T) {
	db := newTestLogicals(t)

	for _, tt := range []struct {
		text string
		sts  uint32
	}{
		{"[UNCLOSED", rmsSyntaxError},
		{"[A..B]X", rmsDirError},
		{"[-]X", rmsDirError},
		{"A.B.C", rmsSyntaxError},
		{"DEV*:X", rmsDeviceError},
	} {
		_, err := expandSpec(db, tt.text, filespec.Spec{})

		var ne *NameError
		if !errors.As(err, &ne) || ne.Status != tt.sts {
			t.Errorf("expandSpec(%q) = %v, want status %#x", tt.text, err, tt.sts)
		}
	}
}

func TestScanSpec(t *testing.T) {
	spec, sts := scanSpec("_DUA1:[A.B]X.DAT;2")
	if sts != 0 || spec.Device != "_DUA1" || strings.Join(spec.Dirs, ".") != "A.B" ||
		spec.Name != "X" || spec.Type != "DAT" || spec.Version != "2" {
		t.Errorf("scanSpec = %+v, %#x", spec, sts)
	}

	if _, sts := scanSpec("[A"); sts != rmsSyntaxError {
		t.Errorf("scanSpec([A) status %#x", sts)
	}
}

// A device service reads only the device field, whatever RMS would make of
// a name that long (VMS 7.1: testdata/lib58's FTN 64-character device).
func TestPhysicalDevice_longName(t *testing.T) {
	long := strings.Repeat("D", 63)

	if got, err := PhysicalDevice(newTestLogicals(t), long+":"); err != nil || got != long {
		t.Errorf("PhysicalDevice = %q, %v", got, err)
	}
}

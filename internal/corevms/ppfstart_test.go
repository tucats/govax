package corevms

import (
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// permanentName translates name in env's process table and returns its
// equivalence, failing t when it has none.
func permanentName(t *testing.T, env *Environment, name string) (*lnm.Entry, lnm.PermanentFile) {
	t.Helper()

	e, err := env.Logicals.Translate(lnm.ProcessTableName, name, lnm.User, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	pf, ok := lnm.ParsePermanentFile(e.Equivalences[0].Value)
	if !ok {
		t.Fatalf("%s = %q: not a process-permanent file string", name, e.Equivalences[0].Value)
	}

	return e, pf
}

// TestOpenTerminalFiles: process 1's four names lead to two terminal
// entries, as VMS 7.1's did (testdata/ppf71's terminal run): SYS$INPUT
// and SYS$COMMAND one, SYS$OUTPUT and SYS$ERROR the other, executive mode,
// CONCEALED and TERMINAL, each word with PRN; and LIB$PUT_OUTPUT and a
// message go through the output entry, once each.
func TestOpenTerminalFiles(t *testing.T) {
	env, out := fixture()

	if err := env.OpenTerminalFiles(); err != nil {
		t.Fatal(err)
	}

	words := map[string]uint16{
		"SYS$INPUT": 0x8101, "SYS$COMMAND": 0x8101,
		"SYS$OUTPUT": 0x8102, "SYS$ERROR": 0x8102,
	}

	for name, word := range words {
		e, pf := permanentName(t, env, name)

		if pf.IFI != word || pf.Device != "_GOVAX$TTA0:" {
			t.Errorf("%s: IFI %04X, device %q; want %04X, _GOVAX$TTA0:", name, pf.IFI, pf.Device, word)
		}

		if e.Mode != lnm.Executive || e.Equivalences[0].Attrs != lnm.AttrConcealed|lnm.AttrTerminal {
			t.Errorf("%s: mode %v, attributes %#x; want executive, concealed and terminal",
				name, e.Mode, e.Equivalences[0].Attrs)
		}
	}

	input, _ := env.permanentEntry("SYS$INPUT")
	output, _ := env.permanentEntry("SYS$OUTPUT")

	if input == nil || !input.IsTerminal() || input.Access() != fabFACGet {
		t.Errorf("SYS$INPUT's entry %+v; want the terminal, GET", input)
	}

	if output == nil || !output.IsTerminal() || output.Access() != fabFACGet|fabFACPut {
		t.Errorf("SYS$OUTPUT's entry %+v; want the terminal, GET and PUT", output)
	}

	before := env.Process.BufferedIO

	if st, err := env.PutOutput("through the entry"); err != nil || st != ssNormal {
		t.Errorf("PutOutput: %08X, %v; want SS$_NORMAL", st, err)
	}

	if env.Process.BufferedIO != before+1 {
		t.Errorf("BUFIO +%d; want +1", env.Process.BufferedIO-before)
	}

	// SYS$ERROR shares SYS$OUTPUT's entry, so an informational message
	// is written once.
	env.putMessage(3, []string{"%TEST-I-ONCE, one message"})

	if got, want := out.String(), "through the entry\n%TEST-I-ONCE, one message\n"; got != want {
		t.Errorf("terminal output %q; want %q", got, want)
	}

	// An alias leads to the entry its name does.
	if _, err := env.Logicals.Define(lnm.ProcessTableName, "ALIAS", lnm.User, 0,
		[]lnm.Equivalence{{Value: "SYS$OUTPUT"}}); err != nil {
		t.Fatal(err)
	}

	if e, ok := env.permanentEntry("ALIAS"); !ok || e != output {
		t.Errorf("ALIAS's entry %v, %v; want SYS$OUTPUT's", e, ok)
	}
}

// TestOpenTerminalFiles_subprocess: a subprocess's startup opens its own
// terminal entries from its creator's string (the creator's entry isn't
// the subprocess's), not concealed, and defines a mailbox or NL: as given
// (until subprocesses' files are entries too).
func TestOpenTerminalFiles_subprocess(t *testing.T) {
	parent, _ := fixture()
	defineTestDevice(parent, "TTA0", iodev.DeviceClassTT)
	defineTestDevice(parent, "NLA0", iodev.DeviceClassMailbox)

	if err := parent.OpenTerminalFiles(); err != nil {
		t.Fatal(err)
	}

	child := newSubprocess(t, parent)
	terminal := parent.processValue("SYS$INPUT")

	if err := child.openProcessFiles(terminal, "NL:", "NL:", lnm.AttrTerminal, child.terminalDevice); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"SYS$INPUT", "SYS$COMMAND"} {
		e, pf := permanentName(t, child, name)

		if pf.IFI != 0x8101 || pf.Device != "_GOVAX$TTA0:" || e.Equivalences[0].Attrs != lnm.AttrTerminal {
			t.Errorf("the child's %s: IFI %04X, device %q, attributes %#x; want 8101, _GOVAX$TTA0:, terminal",
				name, pf.IFI, pf.Device, e.Equivalences[0].Attrs)
		}
	}

	if _, ok := child.permanentEntry("SYS$INPUT"); !ok {
		t.Error("the child's SYS$INPUT leads to no entry of its own")
	}

	for _, name := range []string{"SYS$OUTPUT", "SYS$ERROR"} {
		if v := child.processValue(name); v != "NL:" {
			t.Errorf("the child's %s = %q; want NL:", name, v)
		}
	}
}

// fakeRecords is a record device holding records to read and taking
// records written.
type fakeRecords struct {
	records [][]byte
	written []string
}

func (f *fakeRecords) Characteristics() uint32 { return 0 }

func (f *fakeRecords) Put(record []byte, _ bool) (uint32, bool, error) {
	f.written = append(f.written, string(record))

	return ssNormal, false, nil
}

func (f *fakeRecords) WaitPut() (uint32, error) { return ssNormal, nil }

func (f *fakeRecords) Get() ([]byte, uint32, error) {
	if len(f.records) == 0 {
		return nil, ssEndOfFile, nil
	}

	r := f.records[0]
	f.records = f.records[1:]

	return r, ssNormal, nil
}

func (f *fakeRecords) Close() {}

// TestPermanentEntry_notTerminal: LIB$GET_INPUT and LIB$PUT_OUTPUT go
// through SYS$INPUT's and SYS$OUTPUT's entries when they aren't the
// terminal: the stream's records, no prompt, the end of the data; the
// original open's access deciding a write.
func TestPermanentEntry_notTerminal(t *testing.T) {
	env, out := fixture()

	define := func(name string, h *rms.FileHandle) {
		t.Helper()

		e, err := env.files.AddPermanent(h, rms.PermanentOptions{Device: "_GOVAX$MBA1:"})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := env.Logicals.Define(lnm.ProcessTableName, name, lnm.Executive, 0,
			[]lnm.Equivalence{{Value: e.EquivalenceString(), Attrs: lnm.AttrTerminal}}); err != nil {
			t.Fatal(err)
		}
	}

	in := &fakeRecords{records: [][]byte{[]byte("first"), []byte("second line")}}
	define("SYS$INPUT", &rms.FileHandle{Device: in, Access: fabFACGet})

	for _, want := range []string{"first", "second"} {
		line, ok, err := env.ReadInputLine("Prompt: ", 6)
		if err != nil || !ok || line != want {
			t.Errorf("ReadInputLine: %q, %v, %v; want %q", line, ok, err, want)
		}
	}

	if _, ok, err := env.ReadInputLine("Prompt: ", 80); ok || err != nil {
		t.Errorf("ReadInputLine at the end: %v, %v; want the end", ok, err)
	}

	if out.Len() != 0 {
		t.Errorf("terminal output %q; want no prompt", out.String())
	}

	// SYS$INPUT's entry was opened to read only: LIB$PUT_OUTPUT to a
	// SYS$OUTPUT leading there is RMS$_FAC.
	if _, err := env.Logicals.Define(lnm.ProcessTableName, "SYS$OUTPUT", lnm.User, 0,
		[]lnm.Equivalence{{Value: "SYS$INPUT"}}); err != nil {
		t.Fatal(err)
	}

	if st, err := env.PutOutput("refused"); err != nil || st != vmsdef.Symbols["RMS$_FAC"] {
		t.Errorf("PutOutput to an input entry: %08X, %v; want RMS$_FAC", st, err)
	}

	sink := &fakeRecords{}
	define("SYS$OUTPUT", &rms.FileHandle{Device: sink, Access: fabFACPut})

	if _, err := env.Logicals.Delete(lnm.ProcessTableName, "SYS$OUTPUT", lnm.User); err != nil {
		t.Fatal(err)
	}

	if st, err := env.PutOutput("a record"); err != nil || st != ssNormal || len(sink.written) != 1 || sink.written[0] != "a record" {
		t.Errorf("PutOutput: %08X, %v, %q; want SS$_NORMAL and the record", st, err, sink.written)
	}
}

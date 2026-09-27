package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// logicalDispatcher returns a dispatcher over the real console grammar and
// the buffer its console writes to.
func logicalDispatcher(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c := newTestDispatcher(t)
	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	return d, c, buf
}

// mustDispatch runs each line, failing t on the first error.
func mustDispatch(t *testing.T, d *Dispatcher, lines ...string) {
	t.Helper()

	for _, line := range lines {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}
}

// wantDispatchStatus runs line and fails t unless it returns code.
func wantDispatchStatus(t *testing.T, d *Dispatcher, line string, code uint32) {
	t.Helper()

	err := d.Dispatch(line)
	if !errors.Is(err, vmserrors.New(code)) {
		t.Errorf("%s: err = %v, want %v", line, err, vmserrors.New(code))
	}
}

// wantOutput runs line and compares everything it printed with want.
func wantOutput(t *testing.T, d *Dispatcher, buf *bytes.Buffer, line, want string) {
	t.Helper()

	buf.Reset()
	mustDispatch(t, d, line)

	if got := buf.String(); got != want {
		t.Errorf("%s: output\n%s\nwant\n%s", line, got, want)
	}
}

func translateOne(t *testing.T, c *Console, table, name string) *lnm.Entry {
	t.Helper()

	e, err := c.Logicals.Translate(table, name, lnm.User, 0)
	if err != nil {
		t.Fatalf("Translate(%s, %s): %v", table, name, err)
	}

	return e
}

func TestShowLogical_manualExamples(t *testing.T) {
	d, _, buf := logicalDispatcher(t)

	wantOutput(t, d, buf, "SHOW LOGICAL SYS$INPUT",
		"  \"SYS$INPUT\" = \"_TTA0:\" (LNM$PROCESS_TABLE)\n")

	mustDispatch(t, d, "DEFINE/SYSTEM WORK4 $255$DUA17:", "DEFINE MYDISK WORK4")
	wantOutput(t, d, buf, "SHOW LOGICAL MYDISK",
		"  \"MYDISK\" = \"WORK4\" (LNM$PROCESS_TABLE)\n"+
			"1 \"WORK4\" = \"$255$DUA17:\" (LNM$SYSTEM_TABLE)\n")

	mustDispatch(t, d, "DEFINE GETTYSBURG [JONES.HISTORY], [JONES.WORKFILES]")
	wantOutput(t, d, buf, "SHOW LOGICAL GETTYSBURG",
		"  \"GETTYSBURG\" = \"[JONES.HISTORY]\" (LNM$PROCESS_TABLE)\n"+
			"        = \"[JONES.WORKFILES]\"\n")

	wantOutput(t, d, buf, "SHOW LOGICAL/FULL SYS$ERROR",
		"  \"SYS$ERROR\" [exec] = \"_TTA0:\" [terminal] (LNM$PROCESS_TABLE)\n")

	wantOutput(t, d, buf, "SHOW LOGICAL NOSUCH",
		"%SHOW-S-NOTRAN, no translation for logical name NOSUCH\n")
}

func TestShowLogical_iterationStops(t *testing.T) {
	d, _, buf := logicalDispatcher(t)

	// A circular pair prints each name once. A TERMINAL equivalence
	// isn't followed, and a trailing colon is dropped before the next
	// lookup.
	mustDispatch(t, d, "DEFINE A B", "DEFINE B A:", "DEFINE/TRANS=TERMINAL T A")
	wantOutput(t, d, buf, "SHOW LOGICAL A",
		"  \"A\" = \"B\" (LNM$PROCESS_TABLE)\n"+
			"1 \"B\" = \"A:\" (LNM$PROCESS_TABLE)\n")
	wantOutput(t, d, buf, "SHOW LOGICAL T",
		"  \"T\" = \"A\" (LNM$PROCESS_TABLE)\n")
}

func TestShowLogical_tableListing(t *testing.T) {
	d, _, buf := logicalDispatcher(t)

	mustDispatch(t, d, "DEFINE/USER_MODE SYS$OUTPUT X.LIS")
	wantOutput(t, d, buf, "SHOW LOGICAL/TABLE=LNM$PROCESS",
		"(LNM$PROCESS_TABLE)\n"+
			"  \"SYS$COMMAND\" = \"_TTA0:\"\n"+
			"  \"SYS$ERROR\" = \"_TTA0:\"\n"+
			"  \"SYS$INPUT\" = \"_TTA0:\"\n"+
			"  \"SYS$OUTPUT\" [user] = \"X.LIS\"\n"+
			"  \"SYS$OUTPUT\" [exec] = \"_TTA0:\"\n"+
			"  \"TT\" = \"_TTA0:\"\n")

	// Wildcards list the matches under every searched table's header.
	mustDispatch(t, d, "DEFINE/SYSTEM SYS$SYSDEVICE DUA0:")
	wantOutput(t, d, buf, "SHOW LOGICAL SYS$S*",
		"(LNM$PROCESS_TABLE)\n\n(LNM$GROUP_000001)\n\n(LNM$SYSTEM_TABLE)\n"+
			"  \"SYS$SYSDEVICE\" = \"DUA0:\"\n")

	// /GROUP selects just the group table.
	wantOutput(t, d, buf, "SHOW LOGICAL/GROUP", "(LNM$GROUP_000001)\n")

	// A directory lists its table names.
	buf.Reset()
	mustDispatch(t, d, "SHOW LOGICAL/TABLE=LNM$PROCESS_DIRECTORY")

	if !strings.Contains(buf.String(), "  \"LNM$PROCESS_TABLE\" [table] = \"\"\n") {
		t.Errorf("directory listing = %q", buf.String())
	}

	wantDispatchStatus(t, d, "SHOW LOGICAL/TABLE=NOSUCH", vmserrors.SS_NOLOGTAB)
}

func TestDefine_andAssign(t *testing.T) {
	d, c, buf := logicalDispatcher(t)

	// DEFINE keeps a trailing colon on the name; ASSIGN removes one.
	mustDispatch(t, d, "DEFINE DISK: DUA1:", "ASSIGN DUA2:, DUA3: DISK:")

	if e := translateOne(t, c, "LNM$PROCESS", "DISK:"); e.Equivalences[0].Value != "DUA1:" {
		t.Errorf("DISK: = %+v", e.Equivalences)
	}

	if e := translateOne(t, c, "LNM$PROCESS", "DISK"); len(e.Equivalences) != 2 || e.Equivalences[1].Value != "DUA3:" {
		t.Errorf("DISK = %+v", e.Equivalences)
	}

	// Redefining at the same mode reports the supersede unless /NOLOG.
	buf.Reset()
	mustDispatch(t, d, "DEFINE DISK DUA4:")

	if got := buf.String(); got != "%DCL-I-SUPERSEDE, previous value of DISK has been superseded\n" {
		t.Errorf("supersede message = %q", got)
	}

	buf.Reset()
	mustDispatch(t, d, "DEFINE/NOLOG DISK DUA5:")

	if buf.Len() != 0 {
		t.Errorf("/NOLOG printed %q", buf.String())
	}

	// Quoted values keep their case; the table and mode qualifiers apply.
	mustDispatch(t, d, `DEFINE/SYSTEM/EXECUTIVE_MODE/TRANSLATION=(CONCEALED,TERMINAL) HOME "dua0:[Tom]"`)

	e := translateOne(t, c, "LNM$SYSTEM", "HOME")
	if e.Mode != lnm.Executive || e.Table.Name != lnm.SystemTableName ||
		e.Equivalences[0] != (lnm.Equivalence{Value: "dua0:[Tom]", Attrs: lnm.AttrConcealed | lnm.AttrTerminal}) {
		t.Errorf("HOME = %+v in %s at %s", e.Equivalences, e.Table.Name, e.Mode)
	}

	mustDispatch(t, d, "DEFINE/GROUP G X", "DEFINE/TABLE=LNM$GROUP_000001 H Y")

	for _, n := range []string{"G", "H"} {
		if e := translateOne(t, c, "LNM$FILE_DEV", n); e.Table.Name != "LNM$GROUP_000001" {
			t.Errorf("%s is in %s", n, e.Table.Name)
		}
	}

	wantDispatchStatus(t, d, "DEFINE/TABLE=NOSUCH X Y", vmserrors.SS_NOLOGTAB)
	wantDispatchStatus(t, d, "DEFINE/PROCESS/SYSTEM X Y", vmserrors.CLI_BADQUALIFIERCOMBO)

	// DEFINE/DEVICE still reaches its own syntax.
	mustDispatch(t, d, "DEFINE/DEVICE DKA9")

	if _, ok := c.Devices.Find("DKA9"); !ok {
		t.Error("DEFINE/DEVICE DKA9 didn't define the device")
	}
}

func TestDeassign(t *testing.T) {
	d, c, _ := logicalDispatcher(t)

	mustDispatch(t, d, "DEFINE DISK DUA1:", "DEFINE WORK DUA2:", "DEFINE/SYSTEM SYSWORK DUA3:")

	// DEASSIGN removes one trailing colon, as ASSIGN does.
	mustDispatch(t, d, "DEASSIGN DISK:")

	if _, err := c.Logicals.Translate("LNM$PROCESS", "DISK", lnm.User, 0); err == nil {
		t.Error("DISK survived DEASSIGN DISK:")
	}

	wantDispatchStatus(t, d, "DEASSIGN DISK", vmserrors.SS_NOLOGNAM)

	// Process-permanent names are executive mode: a default DEASSIGN
	// can't remove them, and DEASSIGN/ALL leaves them behind.
	wantDispatchStatus(t, d, "DEASSIGN SYS$OUTPUT", vmserrors.SS_NOLOGNAM)
	mustDispatch(t, d, "DEASSIGN/ALL")

	if _, err := c.Logicals.Translate("LNM$PROCESS", "WORK", lnm.User, 0); err == nil {
		t.Error("WORK survived DEASSIGN/ALL")
	}

	translateOne(t, c, "LNM$PROCESS", "SYS$OUTPUT")
	translateOne(t, c, "LNM$SYSTEM", "SYSWORK")

	mustDispatch(t, d, "DEASSIGN/SYSTEM SYSWORK")

	wantDispatchStatus(t, d, "DEASSIGN", vmserrors.CLI_MISSINGPARAMETER)
	wantDispatchStatus(t, d, "DEASSIGN/ALL X", vmserrors.CLI_EXTRAPARAMETER)
}

func TestCreateNameTable(t *testing.T) {
	d, c, buf := logicalDispatcher(t)

	// User's Manual §11.10.1's TAX example.
	mustDispatch(t, d, "CREATE/NAME_TABLE TAX", "DEFINE/TABLE=TAX CREDIT [ACCOUNTS.CURRENT]CREDIT.DAT")
	wantOutput(t, d, buf, "SHOW LOGICAL/TABLE=TAX CREDIT",
		"  \"CREDIT\" = \"[ACCOUNTS.CURRENT]CREDIT.DAT\" (TAX)\n")

	tables, err := c.Logicals.ResolveTables("TAX", lnm.User)
	if err != nil {
		t.Fatal(err)
	}

	if tables[0].Parent != c.Logicals.ProcessDirectory || tables[0].Shareable || tables[0].Mode != lnm.Supervisor {
		t.Errorf("TAX: parent %s, shareable %v, mode %s", tables[0].Parent.Name, tables[0].Shareable, tables[0].Mode)
	}

	// §11.10.2: a shareable table under the system directory.
	mustDispatch(t, d, "CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY NEWTAB")
	wantOutput(t, d, buf, "SHOW LOGICAL/STRUCTURE",
		"(LNM$PROCESS_DIRECTORY)\n"+
			"    (LNM$PROCESS_TABLE)\n"+
			"    (TAX)\n"+
			"(LNM$SYSTEM_DIRECTORY)\n"+
			"    (LNM$SYSTEM_TABLE)\n"+
			"    (LNM$GROUP_000001)\n"+
			"    (NEWTAB)\n")

	// Re-creating supersedes (and so empties) the table.
	wantOutput(t, d, buf, "CREATE/NAME_TABLE TAX",
		"%DCL-I-SUPERSEDE, previous value of TAX has been superseded\n")

	// §11.12: DEASSIGN/TABLE=directory deletes a table.
	mustDispatch(t, d, "DEASSIGN/TABLE=LNM$PROCESS_DIRECTORY TAX")

	if _, err := c.Logicals.ResolveTables("TAX", lnm.User); err == nil {
		t.Error("TAX survived DEASSIGN/TABLE=LNM$PROCESS_DIRECTORY TAX")
	}

	wantDispatchStatus(t, d, "CREATE/NAME_TABLE/USER/EXEC T", vmserrors.CLI_BADQUALIFIERCOMBO)
	wantDispatchStatus(t, d, "CREATE X", vmserrors.CLI_EXTRAPARAMETER)
	wantDispatchStatus(t, d, "CREATE", vmserrors.CLI_MISSINGPARAMETER)
}

func TestShowTranslation(t *testing.T) {
	d, _, buf := logicalDispatcher(t)

	mustDispatch(t, d, "DEFINE/SYSTEM WORK4 $255$DUA17:", "DEFINE MYDISK WORK4")

	wantOutput(t, d, buf, "SHOW TRANSLATION MYDISK", "MYDISK = \"WORK4\" (LNM$PROCESS_TABLE)\n")
	wantOutput(t, d, buf, "SHOW TRANSLATION/TABLE=LNM$SYSTEM WORK4", "WORK4 = \"$255$DUA17:\" (LNM$SYSTEM_TABLE)\n")
	wantOutput(t, d, buf, "SHOW TRANSLATION/TABLE=LNM$SYSTEM MYDISK",
		"%SHOW-S-NOTRAN, no translation for logical name MYDISK\n")
	wantDispatchStatus(t, d, "SHOW TRANSLATION/TABLE=NOSUCH X", vmserrors.SS_NOLOGTAB)
}

func TestLogicals_trace(t *testing.T) {
	d, c, buf := logicalDispatcher(t)

	c.CPU.SetDebugWriter(buf)
	c.CPU.SetDebug(vax.DebugLogicals)
	mustDispatch(t, d, "DEFINE FOO BAR")

	if !strings.Contains(buf.String(), "DEBUG: LNM: define FOO [super] in LNM$PROCESS_TABLE") {
		t.Errorf("trace = %q", buf.String())
	}

	buf.Reset()
	c.CPU.SetDebug(0)
	mustDispatch(t, d, "DEFINE/NOLOG FOO BAZ")

	if buf.Len() != 0 {
		t.Errorf("trace with DEBUG LOGICALS clear = %q", buf.String())
	}
}

func TestLogicals_beforeInit(t *testing.T) {
	c := New(&bytes.Buffer{})

	// Logical names don't need INIT; the trace hook must cope with no CPU.
	if err := c.DefineLogicalName("LNM$PROCESS", "FOO", []string{"BAR"}, lnm.Supervisor, 0, true); err != nil {
		t.Fatal(err)
	}
}

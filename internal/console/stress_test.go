package console_test

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 47's subtask 9: stress tests. Three processes at one priority, on
// a short quantum, use one volume at once: appending to one file,
// creating and erasing files in one directory, extending files of their
// own. After each run the volume is dismounted and mounted again, then
// checked (rms.MountTable.VerifyVolume: the bitmap against the headers,
// and every directory entry against its file) and every record read.

// stressFAB is a FAB for a stress program: name, FAC, SHR, RAB$L_ROP,
// and variable-length records.
type stressFAB struct {
	name     string
	fac, shr uint32
	rop      uint32
}

// stressRun boots a machine running each of programs as a process (the
// first is process 1) on quantum, with a fresh 2000-block volume on
// DUA0 and each process's FAB and RAB set up, runs them all to their
// end (1 at data), dismounts and mounts the volume again, and returns
// the console and the processes.
func stressRun(t *testing.T, quantum string, programs []string, fabs []stressFAB) (*console.Console, []*corevms.Environment) {
	t.Helper()

	return stressRunWith(t, quantum, programs, fabs, nil)
}

// stressRunWith is stressRun calling setup, if not nil, before the run.
func stressRunWith(t *testing.T, quantum string, programs []string, fabs []stressFAB,
	setup func(*console.Console, []*corevms.Environment),
) (*console.Console, []*corevms.Environment) {
	t.Helper()

	var codes [][]byte

	for _, src := range programs {
		code, _ := assembleAt(t, rmsSymbols()+src+mbxCommon)
		if codeAddr+len(code) > mbxData {
			t.Fatalf("a program (%d bytes) runs into its data", len(code))
		}

		codes = append(codes, code)
	}

	c, _ := scheduledConsole(t, quantum, codes[0])
	envs := []*corevms.Environment{c.RTL}

	for _, code := range codes[1:] {
		envs = append(envs, handBuiltProcess(t, c, code))
	}

	path := filepath.Join(t.TempDir(), "stress.dsk")
	if err := c.InitializeContainer(path, 2000, "STRESS", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	sym := vmsdef.Symbols

	for i, env := range envs {
		f := fabs[i]
		putFABRAB(t, c, env, f.name, byte(f.fac))

		store := func(addr uint32, b ...byte) {
			if err := c.Mem.StoreIn(c.CPU, env.Space.AddressSpace, addr, b); err != nil {
				t.Fatal(err)
			}
		}

		store(rmsFAB+sym["FAB$B_SHR"], byte(f.shr))
		store(rmsFAB+sym["FAB$B_RFM"], byte(sym["FAB$C_VAR"]))
		store(rmsRAB+sym["RAB$L_ROP"], binary.LittleEndian.AppendUint32(nil, f.rop)...)
	}

	if setup != nil {
		setup(c, envs)
	}

	runUntil(t, c, 20000000, func() bool {
		for _, env := range envs {
			if longwordAt(t, c, env, mbxData) != 1 {
				return false
			}
		}

		return true
	})

	for i, env := range envs {
		if st := longwordAt(t, c, env, mbxData+0x10); st != 0 {
			t.Fatalf("process %d: a service failed: %08X, returning to %08X", i+1, st, longwordAt(t, c, env, mbxData+0x14))
		}
	}

	for _, env := range envs {
		env.CloseFiles()
	}

	if err := c.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	problems, err := c.Mounts.VerifyVolume("DUA0")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range problems {
		t.Errorf("volume: %s", p)
	}

	return c, envs
}

// stressRecords reads every record of DUA0:[000000]name.
func stressRecords(t *testing.T, c *console.Console, name string) [][]byte {
	t.Helper()

	recs, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]" + name}, rms.VariableRecords)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return recs
}

// failOn is each stress program's check of a service's status: called
// (jsb) after the service, it returns if R0 is a success, and otherwise
// stores R0 at data+0x10 and the address after the call at data+0x14,
// and stops.
const failOn = `
chk:	blbs	r0, chkok
	movl	r0, @#data+^X10
	movl	(sp), @#data+^X14
	brw	fin
chkok:	rsb
`

// appendLoop is a stress appender: $OPEN or $CREATE (%[3]s), $CONNECT,
// %[2]d records, each its tag (%[1]d) and a longword count followed by
// count%%32 more bytes, then $CLOSE.
const appendLoop = `
	pushal	@#fab
	calls	#1, @#sys$%[3]s
	jsb	chk
	pushal	@#rab
	calls	#1, @#sys$connect
	jsb	chk
	clrl	r6
	moval	@#data+^X100, @#rab+RAB_L_RBF
next:	movb	#%[1]d, @#data+^X100
	movl	r6, @#data+^X101
	bicl3	#^C31, r6, r0
	addl2	#5, r0
	movw	r0, @#rab+RAB_W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	jsb	chk
	incl	r6
	cmpl	r6, #%[2]d
	blss	next
	pushal	@#fab
	calls	#1, @#sys$close
	jsb	chk
	brw	fin
` + failOn

// checkTagged checks recs as appendLoop's records: the ones tagged tag
// count 0 to n-1 in order, each its length.
func checkTagged(t *testing.T, recs [][]byte, tags []byte, n int) {
	t.Helper()

	next := map[byte]uint32{}

	for i, r := range recs {
		if len(r) < 5 {
			t.Fatalf("record %d is %d bytes", i, len(r))
		}

		tag, k := r[0], binary.LittleEndian.Uint32(r[1:])
		if k != next[tag] || len(r) != 5+int(k%32) {
			t.Fatalf("record %d: tag %c count %d, %d bytes; want count %d", i, tag, k, len(r), next[tag])
		}

		next[tag]++
	}

	for _, tag := range tags {
		if next[tag] != uint32(n) {
			t.Errorf("%c: %d records, want %d", tag, next[tag], n)
		}
	}
}

// TestStress_threeAppenders: three processes append 150 records each to
// one write-shared file, records of varying length so that they cross
// block boundaries anywhere.
func TestStress_threeAppenders(t *testing.T) {
	const n = 150

	sym := vmsdef.Symbols
	shared := stressFAB{
		name: "DUA0:LOG.DAT", fac: sym["FAB$M_PUT"],
		shr: sym["FAB$M_SHRGET"] | sym["FAB$M_SHRPUT"], rop: sym["RAB$M_EOF"],
	}

	// Process 1 creates the file (with the others' sharing), and the
	// others open it; one of them may try before it's made, so they
	// start a little later: each waits for process 1's first record.
	programs := []string{
		fmt.Sprintf(appendLoop, 'A', n, "create"),
		fmt.Sprintf(appendLoop, 'B', n, "open"),
		fmt.Sprintf(appendLoop, 'C', n, "open"),
	}

	c, _ := stressRunAfterCreate(t, programs, []stressFAB{shared, shared, shared})

	recs := stressRecords(t, c, "LOG.DAT")
	checkTagged(t, recs, []byte("ABC"), n)

	switches := 0
	for i := 1; i < len(recs); i++ {
		if recs[i][0] != recs[i-1][0] {
			switches++
		}
	}

	if switches < 10 {
		t.Errorf("the processes' records alternate %d times: they didn't append together", switches)
	}
}

// stressRunAfterCreate is stressRun with the later processes held at
// their first instruction (a $HIBER) until process 1 has created the
// file: process 1 wakes them after its $CONNECT.
func stressRunAfterCreate(t *testing.T, programs []string, fabs []stressFAB) (*console.Console, []*corevms.Environment) {
	t.Helper()

	// Process 1: wake the others (their PIDs at data+4, data+8) once
	// connected; the others: hibernate first.
	programs[0] = insertAfter(programs[0], "calls	#1, @#sys$connect\n\tjsb	chk\n", `
	clrl	-(sp)
	pushal	@#data+4
	calls	#2, @#sys$wake
	clrl	-(sp)
	pushal	@#data+8
	calls	#2, @#sys$wake
`)

	for i := 1; i < len(programs); i++ {
		programs[i] = "\tcalls	#0, @#sys$hiber\n" + programs[i]
	}

	return stressRunWith(t, "40", programs, fabs, func(c *console.Console, envs []*corevms.Environment) {
		setLongword(t, c, envs[0], mbxData+4, envs[1].Process.PID)
		setLongword(t, c, envs[0], mbxData+8, envs[2].Process.PID)
	})
}

// insertAfter inserts text into src after the first occurrence of mark.
func insertAfter(src, mark, text string) string {
	for i := 0; i+len(mark) <= len(src); i++ {
		if src[i:i+len(mark)] == mark {
			return src[:i+len(mark)] + text + src[i+len(mark):]
		}
	}

	panic("no " + mark)
}

// eraseLoop is a stress program making and erasing versions of its own
// file in the shared directory: %[2]d times it $CREATEs the FAB's file
// (a new version), puts one record (its tag, %[1]d, and the turn's
// number), $CLOSEs it, and every other turn $ERASEs it (the highest
// version: the one just made).
const eraseLoop = `
	clrl	r6
	moval	@#data+^X100, @#rab+RAB_L_RBF
	movw	#5, @#rab+RAB_W_RSZ
turn:	pushal	@#fab
	calls	#1, @#sys$create
	jsb	chk
	pushal	@#rab
	calls	#1, @#sys$connect
	jsb	chk
	movb	#%[1]d, @#data+^X100
	movl	r6, @#data+^X101
	pushal	@#rab
	calls	#1, @#sys$put
	jsb	chk
	pushal	@#fab
	calls	#1, @#sys$close
	jsb	chk
	blbc	r6, keep
	pushal	@#fab
	calls	#1, @#sys$erase
	jsb	chk
keep:	incl	r6
	cmpl	r6, #%[2]d
	blss	turn
	brw	fin
` + failOn

// TestStress_createAndErase: three processes create and erase files in
// one directory at once; each ends with the versions it kept, each
// holding its record.
func TestStress_createAndErase(t *testing.T) {
	const turns = 40

	sym := vmsdef.Symbols

	var (
		programs []string
		fabs     []stressFAB
	)

	for _, tag := range "PQR" {
		programs = append(programs, fmt.Sprintf(eraseLoop, tag, turns))
		fabs = append(fabs, stressFAB{name: fmt.Sprintf("DUA0:%c.DAT", tag), fac: sym["FAB$M_PUT"]})
	}

	c, _ := stressRun(t, "40", programs, fabs)

	for _, tag := range "PQR" {
		// Turn k made version k/2+1; the even turns' files are kept.
		for v := 1; v <= turns/2; v++ {
			recs := stressRecords(t, c, fmt.Sprintf("%c.DAT;%d", tag, v))
			if len(recs) != 1 || recs[0][0] != byte(tag) || binary.LittleEndian.Uint32(recs[0][1:]) != uint32(2*(v-1)) {
				t.Errorf("%c.DAT;%d: %v", tag, v, recs)
			}
		}

		if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: fmt.Sprintf("DUA0:[000000]%c.DAT;%d", tag, turns/2+1)}, rms.VariableRecords); err == nil {
			t.Errorf("%c.DAT;%d exists; the last turn's file was erased", tag, turns/2+1)
		}
	}
}

// TestStress_extendTogether: three processes each write a file of their
// own, unshared, at once: their extensions interleave on the volume.
func TestStress_extendTogether(t *testing.T) {
	const n = 200

	sym := vmsdef.Symbols

	var (
		programs []string
		fabs     []stressFAB
	)

	for _, tag := range "XYZ" {
		programs = append(programs, fmt.Sprintf(appendLoop, tag, n, "create"))
		fabs = append(fabs, stressFAB{name: fmt.Sprintf("DUA0:%c.DAT", tag), fac: sym["FAB$M_PUT"]})
	}

	c, _ := stressRun(t, "40", programs, fabs)

	for _, tag := range "XYZ" {
		checkTagged(t, stressRecords(t, c, fmt.Sprintf("%c.DAT", tag)), []byte{byte(tag)}, n)
	}
}

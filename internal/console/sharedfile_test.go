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

// Phase 47's subtask 5: several processes appending to one sequential
// file, write-shared, while the scheduler switches between them.

// appender is each process's program: $OPEN the FAB (FAC=PUT,
// SHR=GET|PUT, set by the test), $CONNECT the RAB (RAB$V_EOF), then put
// %[2]d records, each its tag byte (%[1]d) and a longword count, idling
// a little between them so that the quanta end mid-run; $CLOSE; and 1
// at data. The statuses: $OPEN's at data+8, $CONNECT's at data+12, the
// first failing $PUT's at data+16, $CLOSE's at data+20.
const appender = `
	pushal	@#fab
	calls	#1, @#sys$open
	movl	r0, @#data+8
	pushal	@#rab
	calls	#1, @#sys$connect
	movl	r0, @#data+12
	movl	#%[2]d, r7
	clrl	r6
	moval	@#data+^X100, @#rab+RAB_L_RBF
	movw	#5, @#rab+RAB_W_RSZ
next:	movb	#%[1]d, @#data+^X100
	movl	r6, @#data+^X101
	pushal	@#rab
	calls	#1, @#sys$put
	blbs	r0, ok
	movl	r0, @#data+16
	brb	close
ok:	incl	r6
	movl	#25, r8
idle:	sobgtr	r8, idle
	sobgtr	r7, next
close:	pushal	@#fab
	calls	#1, @#sys$close
	movl	r0, @#data+20
	movl	#1, @#data
done:	brb	done
`

// sharedFAB sets up env's FAB and RAB for the appender: DUA0:LOG.DAT,
// FAC=PUT, SHR=GET|PUT, and RAB$V_EOF.
func sharedFAB(t *testing.T, c *console.Console, env *corevms.Environment) {
	t.Helper()

	sym := vmsdef.Symbols
	putFABRAB(t, c, env, "DUA0:LOG.DAT", byte(sym["FAB$M_PUT"]))

	store := func(addr uint32, b ...byte) {
		if err := c.Mem.StoreIn(c.CPU, env.Space.AddressSpace, addr, b); err != nil {
			t.Fatal(err)
		}
	}

	store(rmsFAB+sym["FAB$B_SHR"], byte(sym["FAB$M_SHRGET"]|sym["FAB$M_SHRPUT"]))

	rop := make([]byte, 4)
	binary.LittleEndian.PutUint32(rop, sym["RAB$M_EOF"])
	store(rmsRAB+sym["RAB$L_ROP"], rop...)
}

// TestSharedFile_twoAppenders: two processes at one priority, with a
// short quantum, append to one file in turns; every record of each is
// there, each process's in its own order, interleaved with the other's,
// and the volume's bitmap agrees with its headers.
func TestSharedFile_twoAppenders(t *testing.T) {
	const perProcess = 60

	sym := vmsdef.Symbols
	codeOne, _ := assembleAt(t, rmsSymbols()+fmt.Sprintf(appender, 'A', perProcess))
	codeTwo, _ := assembleAt(t, rmsSymbols()+fmt.Sprintf(appender, 'B', perProcess))

	c, _ := scheduledConsole(t, "300", codeOne)
	one := c.RTL
	two := handBuiltProcess(t, c, codeTwo)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	loc := rms.FileLocation{Name: "DUA0:[000000]LOG.DAT"}
	if _, err := c.ContainerSession.CreateRecordFile(loc, rms.VariableRecords, nil); err != nil {
		t.Fatal(err)
	}

	sharedFAB(t, c, one)
	sharedFAB(t, c, two)

	runUntil(t, c, 2000000, func() bool {
		return longwordAt(t, c, one, mbxData) == 1 && longwordAt(t, c, two, mbxData) == 1
	})

	normal := sym["RMS$_NORMAL"]

	for _, env := range []*corevms.Environment{one, two} {
		for _, off := range []uint32{8, 12, 16, 20} {
			want := normal
			if off == 16 {
				want = 0
			}

			if got := longwordAt(t, c, env, mbxData+off); got != want {
				t.Errorf("process %08X: status at data+%d = %08X, want %08X", env.Process.PID, off, got, want)
			}
		}
	}

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	next := map[byte]uint32{'A': 0, 'B': 0}
	switches := 0

	for i, r := range records {
		if len(r) != 5 {
			t.Fatalf("record %d is %d bytes", i, len(r))
		}

		tag, n := r[0], binary.LittleEndian.Uint32(r[1:])
		if n != next[tag] {
			t.Fatalf("record %d: %c%d, want %c%d", i, tag, n, tag, next[tag])
		}

		next[tag]++

		if i > 0 && records[i-1][0] != tag {
			switches++
		}
	}

	if next['A'] != perProcess || next['B'] != perProcess {
		t.Errorf("records: %d from A, %d from B; want %d each", next['A'], next['B'], perProcess)
	}

	t.Logf("%d records, %d switches", len(records), switches)

	if switches < 2 {
		t.Errorf("the records alternate between the processes %d times: the test didn't interleave them", switches)
	}

	if err := c.Mounts.Dismount("DUA0"); err != nil {
		t.Fatal(err)
	}
}

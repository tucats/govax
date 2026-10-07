package console_test

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 1: an I/O request that completes for a process
// waiting for it ends the wait at once, with the completion's boost, as
// VMS reports the event to the scheduler when the I/O completes. The
// woken process preempts the one that completed it when its boosted
// priority is at least that one's, rather than waiting for the
// scheduler's next look at its waiters (a quantum end, or a wait).

// mbxReader is process 1's program: create a permanent mailbox named
// PIPE (its channel in the word at ^X%[1]X), $QIOW a read of it, and
// store the $QIOW's R0 at dataAddr; then spin.
const mbxReader = `
	callg	mbx, @#sys$crembx
	movzwl	@#^X%[1]X, qio+8
	callg	qio, @#sys$qiow
	movl	r0, @#^X600
done:	brb	done
mbx:	.long	7, 1, ^X%[1]X, 0, 0, 0, 0, name
qio:	.long	12, 0, 0, %[2]d, ^X%[3]X, 0, 0, ^X%[4]X, 16, 0, 0, 0, 0
name:	.ascid	"PIPE"
`

// mbxWriter is process 2's program: $ASSIGN a channel to PIPE, trying
// again until process 1 has created it, write "hi" to it with IO$M_NOW,
// mark dataAddr+8 after the write, and spin, never waiting.
const mbxWriter = `
again:	callg	asn, @#sys$assign
	blbc	r0, again
	movzwl	@#^X%[1]X, qio+8
	callg	qio, @#sys$qio
	movl	#1, @#^X608		; after the write
spin:	brb	spin
asn:	.long	4, name, ^X%[1]X, 0, 0
qio:	.long	12, 0, 0, %[2]d, 0, 0, 0, msg, 2, 0, 0, 0, 0
name:	.ascid	"PIPE"
msg:	.ascii	"hi"
`

// TestIOCompletion_preempts: process 1 waits for a mailbox read, which
// process 2's write completes. Process 1, at the same base priority as
// process 2 by then, comes out of its wait boosted (PRI$_IOCOM, 2) above process 2, so it
// runs at the next instruction boundary, before process 2 goes on past
// its $QIO: with a quantum far longer than the run, nothing else would
// give it the CPU. It runs at its base priority plus 1: the boost less
// the one the scheduler takes off when it chooses a boosted process. Its read has the message, from process 2's PID.
func TestIOCompletion_preempts(t *testing.T) {
	const (
		chanAddr = dataAddr + 0x10 // the channel (a word)
		iosb     = dataAddr + 0x18 // the read's I/O status block
		buffer   = dataAddr + 0x20 // the message
	)

	reader, _ := assembleAt(t, fmt.Sprintf(mbxReader, chanAddr, vmsdef.Symbols["IO$_READVBLK"], iosb, buffer))
	writer, _ := assembleAt(t, fmt.Sprintf(mbxWriter, chanAddr,
		vmsdef.Symbols["IO$_WRITEVBLK"]|vmsdef.Symbols["IO$M_NOW"]))

	c, _ := scheduledConsole(t, longQuantum, reader)
	one := c.RTL
	two := handBuiltProcess(t, c, writer)

	// Process 2 starts below process 1, so process 1 runs until it
	// waits; then process 2 gets process 1's base priority.
	base := int(one.Process.BasePriority)
	setBasePriority(t, two, base-1)

	sawLEF := false

	for range 20000 {
		if longwordAt(t, c, one, dataAddr) != 0 {
			break
		}

		if stateOf(one) == sched.StateLEF && !sawLEF {
			sawLEF = true

			setBasePriority(t, two, base)
		}

		step(t, c, 1)
	}

	if r0 := longwordAt(t, c, one, dataAddr); r0 != 1 {
		t.Fatalf("process 1's $QIOW returned %08X (never, if 0)", r0)
	}

	if !sawLEF {
		t.Error("process 1 was never seen waiting for its read (LEF)")
	}

	if longwordAt(t, c, two, dataAddr+8) != 0 {
		t.Error("process 2 went on past its write before process 1 ran")
	}

	first := longwordAt(t, c, one, iosb)
	if st, n, from := first&0xFFFF, first>>16, longwordAt(t, c, one, iosb+4); st != 1 || n != 2 || from != two.Process.PID {
		t.Errorf("read IOSB: %04X, %d bytes, from %08X; want SS$_NORMAL, 2, from %08X", st, n, from, two.Process.PID)
	}

	if got := longwordAt(t, c, one, buffer) & 0xFFFF; got != 'h'|'i'<<8 {
		t.Errorf("the read's buffer starts %04X, want \"hi\"", got)
	}

	// Boosted by 2, then one lower as it was chosen to run (the
	// scheduler's decay).
	info, _ := one.Scheduler().Info(sched.Handle(one.Process.PID))
	if info.Priority != info.Base+1 {
		t.Errorf("process 1 runs at priority %d, base %d: want base+1, the boost of 2 less one", info.Priority, info.Base)
	}
}

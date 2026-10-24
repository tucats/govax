package console

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// legacyVectorProgram calls $NUMTIM twice, through its P1 vector entry
// and through its entry in VMS V1's system service vector in S0
// (^X800001B8, as DUNGEON.EXE calls it; docs/PHASE-72 - legacy
// images.md), and leaves 1 in R0 when both worked and gave the same
// date. SYS$NUMTIM's arguments: the 7-word buffer, and 0 for now.
const legacyVectorProgram = `
	.microkernel
	.p1vector

	.entry	main, ^m<>
	clrl	-(sp)
	pushal	@#p1buf
	calls	#2, @#sys$numtim
	blbc	r0, fail
	clrl	-(sp)
	pushal	@#s0buf
	calls	#2, @#^X800001B8
	blbc	r0, fail
	cmpl	@#p1buf, @#s0buf	; the year and the month
	bneq	fail
	cmpw	@#p1buf+4, @#s0buf+4	; the day
	bneq	fail
	movl	#1, r0
	ret
fail:	clrl	r0
	ret

p1buf:	.blkw	7
s0buf:	.blkw	7

	.end	main
`

// TestLegacyVectorService runs a service through the old S0 vector.
func TestLegacyVectorService(t *testing.T) {
	c := newBootableConsole(t)

	path := filepath.Join(t.TempDir(), "legacy.asm")
	if err := os.WriteFile(path, []byte(legacyVectorProgram), 0o600); err != nil {
		t.Fatal(err)
	}

	addr, hasEntry, err := c.Assemble(path)
	if err != nil || !hasEntry {
		t.Fatalf("Assemble: %v (entry %v)", err, hasEntry)
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil || hitCap {
		t.Fatalf("run: %v (step limit reached %v)", runErr, hitCap)
	}

	if got := c.CPU.GPR(registers.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 ($NUMTIM through ^X800001B8 worked as through the P1 vector)", got)
	}
}

package console

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestZZCompare(t *testing.T) {
	name := os.Getenv("P35CMP")
	if name == "" {
		t.Skip()
	}
	filter := os.Getenv("P35FILTER")
	vms, _ := vaxInsn35Records(t)
	ours := runInsn35Probe(t, name)
	diff, n := 0, 0
	for i, v := range vms[name] {
		if filter != "" && !strings.Contains(v.Name, filter) {
			continue
		}
		n++
		g := ours[i]
		cond := func(r insn35Record) uint32 {
			if len(r.Signal) > 0 {
				return r.Signal[0]
			}
			return 0
		}
		if v.Flags != g.Flags || v.PSL&0xFF != g.PSL&0xFF || !bytes.Equal(v.DST, g.DST) || cond(v) != cond(g) || v.Regs != g.Regs {
			diff++
			t.Logf("%3d %-32s VMS fl=%d psl=%02x sig=%x dst=% x\n%40s gov fl=%d psl=%02x sig=%x dst=% x", v.Case, v.Name, v.Flags, v.PSL&0xFF, cond(v), v.DST[:48], "", g.Flags, g.PSL&0xFF, cond(g), g.DST[:48])
			if v.Regs != g.Regs {
				t.Logf("%40s regs VMS %x\n%40s regs gov %x", "", v.Regs, "", g.Regs)
			}
		}
	}
	t.Logf("%d of %d differ", diff, n)
}

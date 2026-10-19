package console

import (
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/registers"
)

// Tests of vax.init's memory sizes, which the vax.memory.* configuration
// items override (read with F$CONFIG).

// memoryLayout is what a boot's INITIALIZE/VAX and VMINIT made, as the
// machine shows it: physical pages, P0, P1, and S0 pages, and the
// executive, supervisor, and interrupt stack pointers (which VMINIT lays
// out one after another, each mode's stack after a guard page, so their
// distances give the stacks' sizes).
type memoryLayout struct {
	physical, p0, p1, s0 uint32
	esp, ssp, isp        uint32
}

// bootLayout boots vax.init with the configuration items values sets
// (and none of the others), and returns the layout it made.
func bootLayout(t *testing.T, values map[string]string) memoryLayout {
	t.Helper()

	for _, key := range []string{
		"vax.memory.size", "vax.memory.p0", "vax.memory.p1", "vax.memory.s0",
		"vax.memory.ksp", "vax.memory.esp", "vax.memory.ssp", "vax.memory.isp",
	} {
		if settings.Exists(key) {
			old := settings.Get(key)

			t.Cleanup(func() { settings.Set(key, old) })
		} else {
			t.Cleanup(func() { _ = settings.Delete(key) })
		}

		if v, ok := values[key]; ok {
			settings.Set(key, v)
		} else {
			_ = settings.Delete(key)
		}
	}

	c := bootedConsole(t)
	p0, p1 := c.VMInitSizes()

	if c.RTL.Space.Entries != p0+p1 {
		t.Errorf("process 1's table area has %d entries, want /P0 + /P1, %d", c.RTL.Space.Entries, p0+p1)
	}

	return memoryLayout{
		physical: c.Mem.Size() >> 9,
		p0:       p0,
		p1:       p1,
		s0:       c.CPU.PR(registers.SLR),
		esp:      c.CPU.PR(registers.ESP),
		ssp:      c.CPU.PR(registers.SSP),
		isp:      c.CPU.PR(registers.ISP),
	}
}

// TestVaxInitMemoryConfig: with no vax.memory.* items, vax.init's own
// sizes; with them, theirs, in decimal or %X.
func TestVaxInitMemoryConfig(t *testing.T) {
	def := bootLayout(t, nil)

	if def.physical != 16384 || def.p0 != 16384 || def.p1 != 8192 {
		t.Errorf("default: %d physical, P0 %d, P1 %d pages, want 16384, 16384, 8192", def.physical, def.p0, def.p1)
	}

	// SSP is past ESP by a guard page and the supervisor stack; the
	// interrupt stack (physical) follows the supervisor stack.
	if got := def.ssp - def.esp; got != 512*(1+8) {
		t.Errorf("default supervisor stack: SSP-ESP %d, want %d", got, 512*(1+8))
	}

	if got := def.isp + 0x80000000 - def.ssp; got != 512*4 {
		t.Errorf("default interrupt stack: %d bytes, want %d", got, 512*4)
	}

	conf := bootLayout(t, map[string]string{
		"vax.memory.size": "%X6000",
		"vax.memory.p0":   "4096",
		"vax.memory.p1":   "2048",
		"vax.memory.s0":   "10000",
		"vax.memory.ksp":  "24",
		"vax.memory.esp":  "10",
		"vax.memory.ssp":  "12",
		"vax.memory.isp":  "6",
	})

	if conf.physical != 0x6000 || conf.p0 != 4096 || conf.p1 != 2048 {
		t.Errorf("configured: %d physical, P0 %d, P1 %d pages, want %d, 4096, 2048", conf.physical, conf.p0, conf.p1, 0x6000)
	}

	// Physical memory the regions leave over is S0's too.
	if want := uint32(0x6000 - 4096 - 2048); conf.s0 != want {
		t.Errorf("configured S0: %d pages, want %d (10000 and what's left over)", conf.s0, want)
	}

	if got := conf.ssp - conf.esp; got != 512*(1+12) {
		t.Errorf("configured supervisor stack: SSP-ESP %d, want %d", got, 512*(1+12))
	}

	if got := conf.isp + 0x80000000 - conf.ssp; got != 512*6 {
		t.Errorf("configured interrupt stack: %d bytes, want %d", got, 512*6)
	}

	// The kernel and executive stacks come first, after the page tables:
	// with the same regions, ESP moves by their growth.
	same := bootLayout(t, map[string]string{
		"vax.memory.p0":  "4096",
		"vax.memory.p1":  "2048",
		"vax.memory.s0":  "10000",
		"vax.memory.ksp": "24",
		"vax.memory.esp": "10",
	})
	base := bootLayout(t, map[string]string{
		"vax.memory.p0": "4096",
		"vax.memory.p1": "2048",
		"vax.memory.s0": "10000",
	})

	if got, want := same.esp-base.esp, uint32(512*((24-20)+(10-8))); got != want {
		t.Errorf("kernel and executive stacks: ESP moved %d, want %d", got, want)
	}
}

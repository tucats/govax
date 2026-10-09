package console_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 44's subtask 3: the scheduler switching processes by itself. Like
// handswitch_test.go, these boot as cmd/govax does and build a second
// process by hand, but turn the scheduler on (vax.process.scheduler) and
// let the engine's hook do the switching.

// setSetting sets a configuration key for the length of the test.
func setSetting(t testing.TB, key, value string) {
	t.Helper()

	old, had := settings.Get(key), settings.Exists(key)

	t.Cleanup(func() {
		if had {
			settings.Set(key, old)
		} else {
			_ = settings.Delete(key)
		}
	})

	settings.Set(key, value)
}

// incrAbs is INCL @#addr.
func incrAbs(addr uint32) []byte {
	return []byte{0xD6, 0x9F, byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24)}
}

// brbBack is BRB back over n bytes of code before it: to the start of a
// loop whose body is n bytes long.
func brbBack(n int) []byte {
	return []byte{0x11, byte(-(n + 2))}
}

// counter is a two-instruction loop at codeAddr: INCL @#dataAddr, BRB
// back to it.
func counter() []byte {
	inc := incrAbs(dataAddr)

	return append(inc, brbBack(len(inc))...)
}

// scheduledConsole boots a console with the scheduler on at the given
// quantum, and puts process 1 in user mode at IPL 0, about to run code at
// codeAddr in its P0.
func scheduledConsole(t testing.TB, quantum string, code []byte) (*console.Console, *bytes.Buffer) {
	t.Helper()

	setSetting(t, "vax.process.scheduler", "true")
	setSetting(t, "vax.process.quantum", quantum)

	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v\n%s", err, out.String())
	}

	if !c.RTL.ProcessSettings.Scheduler {
		t.Fatal("the scheduler isn't on")
	}

	if err := c.Mem.StoreIn(c.CPU, c.RTL.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	c.Engine.SetModeStack(vax.User, false)

	psl := c.CPU.PSL()
	psl.SetPrvMod(vax.User)
	psl.SetIPL(0)
	c.CPU.SetPSL(psl)
	c.CPU.SetPR(vax.IPL, 0)
	c.CPU.SetGPR(vax.PC, codeAddr)

	return c, &out
}

// handBuiltProcess adds a process to c's system with its own address
// space, stacks, and PCB, starting in user mode at codeAddr in its P0,
// where code is put.
func handBuiltProcess(t testing.TB, c *console.Console, code []byte) *corevms.Environment {
	t.Helper()

	sys := c.RTL.System

	env, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	pid := env.Process.PID

	if env.Space, err = sys.BuildAddressSpace(pid, 64, corevms.MinP1Pages); err != nil {
		t.Fatal(err)
	}

	if env.Stacks, err = sys.BuildStacks(pid, 4, 2, 2); err != nil {
		t.Fatal(err)
	}

	if err := c.Mem.StoreIn(c.CPU, env.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	var userPSL vax.PSL

	userPSL.SetCurMod(vax.User)
	userPSL.SetPrvMod(vax.User)

	start := corevms.InitialPCB(env.Space, env.Stacks, codeAddr, userPSL)
	if err := cpu.WritePCB(c.Mem, env.Stacks.PCBB, &start); err != nil {
		t.Fatal(err)
	}

	return env
}

// countOf reads env's counter, at dataAddr in its own P0.
func countOf(t *testing.T, c *console.Console, env *corevms.Environment) uint32 {
	t.Helper()

	v, err := c.Mem.LoadLongwordIn(c.CPU, env.Space.AddressSpace, dataAddr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// TestScheduler_roundRobin: two processes at the same priority, each
// counting in its own P0 at the same address, take turns a quantum
// each, and each is charged its own instructions.
//
// Process 2, made computable at process 1's priority, preempts it at the
// hook's next call (a newcomer at an equal priority preempts; see
// internal/sched), so process 2 runs first. Process 1 used some of its
// quantum booting (vax.init runs the microkernel), and keeps the rest
// for its first turn.
func TestScheduler_roundRobin(t *testing.T) {
	const quantum = 100

	c, out := scheduledConsole(t, "100", counter())
	one := c.RTL
	two := handBuiltProcess(t, c, counter())
	sys := one.System

	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugProcess)

	// The hook's next call: process 2 preempts, and runs its first
	// instruction.
	c.Engine.RequestReschedule()
	step(t, c, 1)

	if c.RTL.Current() != two {
		t.Fatal("process 2 didn't preempt process 1")
	}

	info, _ := sys.Scheduler().Info(sched.Handle(one.Process.PID))
	cpuBefore := sys.CPUInstructions(one)

	// Each turn: who runs, for how many instructions. A counter's count
	// after n instructions is n/2 rounded up (INCL comes first).
	turns := []struct {
		runs *corevms.Environment
		n    int
	}{
		{two, quantum - 1},
		{one, info.QuantumLeft},
		{two, quantum},
		{one, quantum},
	}

	ran := map[*corevms.Environment]int{two: 1}

	for i, tt := range turns {
		step(t, c, tt.n)
		ran[tt.runs] += tt.n

		if got := c.RTL.Current(); got != tt.runs {
			t.Fatalf("turn %d: process %08X current, want %08X", i, got.Process.PID, tt.runs.Process.PID)
		}

		for _, env := range []*corevms.Environment{one, two} {
			if got, want := countOf(t, c, env), uint32(ran[env]+1)/2; got != want {
				t.Fatalf("turn %d: process %08X counted %d, want %d", i, env.Process.PID, got, want)
			}
		}
	}

	// Instructions are charged when the hook is next called, so the
	// current turn's aren't yet: process 1 has been charged its first
	// turn, process 2 both of its.
	if a, b := sys.CPUInstructions(one)-cpuBefore, sys.CPUInstructions(two); a != uint64(info.QuantumLeft) || b != 2*quantum {
		t.Errorf("CPU instructions %d and %d, want %d and %d", a, b, info.QuantumLeft, 2*quantum)
	}

	// Four switches, each traced.
	if n := strings.Count(out.String(), "DEBUG(PROCESS): SWITCH"); n != 4 {
		t.Errorf("%d switches traced, want 4:\n%s", n, out.String())
	}

	if !strings.Contains(out.String(), "SWITCH 00000301 -> 00000302, PC=00000400") {
		t.Errorf("no trace of the first switch:\n%s", out.String())
	}

	// Process 1's PCB describes its address space: its memory-management
	// longwords were saved with its registers.
	pcb, err := cpu.ReadPCB(c.Mem, one.Stacks.PCBB)
	if err != nil {
		t.Fatal(err)
	}

	if pcb.AddressSpace() != one.Space.AddressSpace {
		t.Errorf("process 1's PCB space %+v, want %+v", pcb.AddressSpace(), one.Space.AddressSpace)
	}
}

// TestScheduler_servicesReachCurrent: a system service called by process
// 2 runs for process 2. Process 2 names itself with $SETPRN; process 1's
// name stays as it was.
func TestScheduler_servicesReachCurrent(t *testing.T) {
	const (
		descAddr = dataAddr + 0x10 // a string descriptor for "TWO"
		textAddr = dataAddr + 0x20 // "TWO"
		statAddr = dataAddr + 0x04 // where process 2 stores $SETPRN's status
	)

	var setprn uint32

	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$SETPRN" {
			setprn = e.Addr
		}
	}

	if setprn == 0 {
		t.Fatal("no SYS$SETPRN in the P1 vector")
	}

	c, _ := scheduledConsole(t, "50", counter())
	one := c.RTL
	oneName := one.Process.Name

	// PUSHAL @#desc; CALLS #1, @#SYS$SETPRN; MOVL R0, @#stat; then count.
	prefix := bytes.Join([][]byte{
		append([]byte{0xDF, 0x9F}, binary.LittleEndian.AppendUint32(nil, descAddr)...),
		append([]byte{0xFB, 0x01, 0x9F}, binary.LittleEndian.AppendUint32(nil, setprn)...),
		movlToAbs(0, statAddr),
	}, nil)
	two := handBuiltProcess(t, c, append(prefix, counter()...))

	// The descriptor: length 3, DSC$K_DTYPE_T (14), DSC$K_CLASS_S (1),
	// then the text's address.
	desc := binary.LittleEndian.AppendUint32([]byte{3, 0, 14, 1}, textAddr)
	if err := c.Mem.StoreIn(c.CPU, two.Space.AddressSpace, descAddr, desc); err != nil {
		t.Fatal(err)
	}

	if err := c.Mem.StoreIn(c.CPU, two.Space.AddressSpace, textAddr, []byte("TWO")); err != nil {
		t.Fatal(err)
	}

	step(t, c, 400)

	stat, err := c.Mem.LoadLongwordIn(c.CPU, two.Space.AddressSpace, statAddr)
	if err != nil {
		t.Fatal(err)
	}

	if stat != 1 {
		t.Fatalf("$SETPRN's status %08X, want SS$_NORMAL", stat)
	}

	if two.Process.Name != "TWO" || one.Process.Name != oneName {
		t.Errorf("names %q and %q, want %q and %q", one.Process.Name, two.Process.Name, oneName, "TWO")
	}

	if countOf(t, c, one) == 0 || countOf(t, c, two) == 0 {
		t.Errorf("counts %d and %d: both should have run", countOf(t, c, one), countOf(t, c, two))
	}
}

// BenchmarkContextSwitch measures what a context switch costs
// (docs/PERFORMANCE.md, "Check: context switches"): two processes, each
// counting in its own P0, share the CPU at the quantum each sub-benchmark
// names. At 10 instructions a quantum the CPU switches every 10
// instructions; at the default, 20,000, almost never. The difference in
// time per instruction, times the quantum, is a switch's cost. Each op
// is one instruction.
func BenchmarkContextSwitch(b *testing.B) {
	for _, quantum := range []string{"10", "20000"} {
		b.Run("quantum="+quantum, func(b *testing.B) {
			c, _ := scheduledConsole(b, quantum, counter())
			handBuiltProcess(b, c, counter())
			c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugProcess)

			b.ResetTimer()
			step(b, c, b.N)
		})
	}
}

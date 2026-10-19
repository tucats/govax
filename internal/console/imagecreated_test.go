package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vm"
)

// Phase 66's subtask 6 (docs/PHASE-66 - mapped image activation.md):
// the image of a process $CREPRC created, and the images a subprocess's
// DCL runs, are mapped as RUN maps process 1's. Both reach the same
// activation (Machine.ActivateCreatedImage, then
// ImageProcess.ActivateImage), which maps the image into the process's
// own P0 through its own page tables, whichever process is current;
// these tests check that the mapping lands there, not in process 1's,
// that the process holds the image file while the image runs, and that
// the image's end (and, for a created process, the process's deletion)
// lets everything go.

// childPTE is the PTE of env's page at va, read through env's own page
// tables.
func childPTE(t *testing.T, c *console.Console, env *corevms.Environment, va uint32) vm.PTE {
	t.Helper()

	pte, err := c.Mem.LookupPTEIn(c.CPU, env.Space.AddressSpace, va)
	if err != nil {
		t.Fatalf("PTE of %08X in PID %08X: %v", va, env.Process.PID, err)
	}

	return pte
}

// runChildTo runs the machine until env is current and has reached
// checkpoint n of imageMapSource (its R6), then steps it into the
// checkpoint's loop.
func runChildTo(t *testing.T, c *console.Console, env *corevms.Environment, n uint32) {
	t.Helper()

	runUntil(t, c, 400_000, func() bool { return c.RTL.Current() == env && c.CPU.GPR(6) == n })
	step(t, c, 4)

	if c.RTL.Current() != env || c.CPU.GPR(6) != n {
		t.Fatalf("PID %08X left checkpoint %d", env.Process.PID, n)
	}
}

// releaseChild lets env, current and waiting at a checkpoint, go on past
// it (its R10, as imageMapSource's checkpoints are released).
func releaseChild(t *testing.T, c *console.Console, env *corevms.Environment, n uint32) {
	t.Helper()

	if c.RTL.Current() != env {
		t.Fatalf("PID %08X isn't current", env.Process.PID)
	}

	c.CPU.SetGPR(10, n)
}

// checkChildMapped checks env's image, stopped at checkpoint 1: each
// private ISD a section of the image file in env (and none in process
// 1), its untouched code still in its section form, the sections'
// protections and the fixup page's UREW in env's own page tables, and
// the file held, so its volume can't be dismounted. It returns the
// image's sections.
func checkChildMapped(t *testing.T, c *console.Console, env *corevms.Environment) int {
	t.Helper()

	main := c.ImageState(env).FindMainICB()
	if main == nil {
		t.Fatalf("PID %08X has no main image", env.Process.PID)
	}

	if c.FindMainICB() != nil {
		t.Error("process 1 has a main image")
	}

	sections, sawFixup := 0, false

	for _, isd := range main.ISDList {
		if isd.Flags>>24 == mapISDUsrStack || isd.Flags&mapISDGBL != 0 {
			continue
		}

		if isd.Flags&mapISDDZRO == 0 {
			sections++
		}

		want := vm.ProtUR

		switch {
		case isd.Flags&mapISDFIXUPVEC != 0:
			want, sawFixup = vm.ProtUREW, true
		case isd.Flags&mapISDWRT != 0:
			want = vm.ProtUW
		}

		for n := range uint32(isd.Pages) {
			va := main.Base + (isd.VPN+n)<<9
			pte := childPTE(t, c, env, va)

			if got := pte.Protection(); got != want {
				t.Errorf("page %08X (ISD flags %08X): PROT %s, want %s", va, isd.Flags, got, want)
			}

			if isd.Flags&mapISDDZRO != 0 && !pte.IsDemandZero() {
				t.Errorf("demand-zero page %08X has PTE %08X, want demand-zero", va, uint32(pte))
			}
		}
	}

	if !sawFixup {
		t.Error("the image has no fixup section")
	}

	// The code never run (R8's 4 pages) is still mapped to the file.
	unrun := c.CPU.GPR(8)
	for n := range uint32(4) {
		if va := unrun + n*512; !isSectionPTE(childPTE(t, c, env, va)) {
			t.Errorf("code page %08X never run has PTE %08X, want its section form", va, uint32(childPTE(t, c, env, va)))
		}
	}

	if got := env.ProcessSections(); got != sections {
		t.Errorf("PID %08X has %d process sections, want the image's %d", env.Process.PID, got, sections)
	}

	if got := c.ProcessOne().ProcessSections(); got != 0 {
		t.Errorf("process 1 has %d process sections, want none", got)
	}

	if err := c.Mounts.Dismount("DUA0"); err == nil {
		t.Fatal("the image's volume was dismounted while a process ran the image")
	}

	return sections
}

// awaitGone runs the machine until env has been deleted and the CPU has
// left it: a process deleted while the CPU runs in it keeps its memory
// until then (corevms's DeleteProcess).
func awaitGone(t *testing.T, c *console.Console, env *corevms.Environment, limit int) {
	t.Helper()

	runUntil(t, c, limit, func() bool { return env.Deleted && c.RTL.Current() != env })
}

// spinProcessOne puts process 1 back in scheduledConsole's spin loop, in
// user mode at IPL 0, after a DCL command: process 1's DCL lends the CPU
// between commands and leaves it at no instruction of its own.
func spinProcessOne(t *testing.T, c *console.Console) {
	t.Helper()

	if c.RTL.Current() != c.ProcessOne() {
		t.Fatal("process 1 isn't current after the command")
	}

	c.Engine.SetModeStack(registers.User, false)

	psl := c.CPU.PSL()
	psl.SetCurMod(registers.User)
	psl.SetPrvMod(registers.User)
	psl.SetIPL(0)
	c.CPU.SetPSL(psl)
	c.CPU.SetPR(registers.IPL, 0)
	c.CPU.SetGPR(registers.PC, codeAddr)
}

// checkNoChildFrames checks that no frame is tied to a P0 page of pid.
func checkNoChildFrames(t *testing.T, c *console.Console, pid uint32) {
	t.Helper()

	db := c.Mem.Paging()

	for pfn := range db.Frames() {
		e, err := db.Entry(paging.PFN(pfn))
		if err != nil {
			t.Fatal(err)
		}

		if e.Tied && e.Owner.Handle == pid && e.Owner.VA < 0x40000000 && e.Type == paging.ProcessPage {
			t.Errorf("frame %d is still tied to PID %08X's P0 page %08X: %+v", pfn, pid, e.Owner.VA, e)
		}
	}
}

// TestMappedCreatedImage: the image of a process $CREPRC creates is
// mapped into that process's P0, the file held while it runs; the image
// runs to its end (its G^ call says hello), and the process's deletion
// leaves no section, frame, or hold on the file behind.
func TestMappedCreatedImage(t *testing.T) {
	c, _, name, out := mappedImageConsole(t, "1")
	one := c.RTL

	child, st := one.CreateProcess(corevms.CreateRequest{
		Image: name, Output: "TTA0:", Name: "MAPKID", BasePriority: one.Process.BasePriority,
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()
	runChildTo(t, c, child, 1)
	checkChildMapped(t, c, child)

	// Checkpoint 2: the 40 array pages written, past the 30-page working
	// set, so the image's pages fault in and out of the child's set.
	releaseChild(t, c, child, 1)
	runChildTo(t, c, child, 2)
	checkWorkingSets(t, c)

	releaseChild(t, c, child, 2)
	awaitGone(t, c, child, 400_000)

	if !strings.Contains(out.String(), "mapped hello") {
		t.Errorf("the child wrote %q, want its hello", out.String())
	}

	if child.Process.ExitStatus != 1 {
		t.Errorf("the child's exit status %08X, want 1", child.Process.ExitStatus)
	}

	if got := child.ProcessSections(); got != 0 {
		t.Errorf("%d process sections after the child's deletion, want none", got)
	}

	checkNoChildFrames(t, c, child.Process.PID)
	checkWorkingSets(t, c)

	if err := c.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after the child: %v", err)
	}
}

// TestMappedSpawnedImage: a subprocess's DCL runs the image twice, each
// RUN mapping it into the subprocess's P0. The first image's end lets
// its sections go before the second is activated, so the second has
// only its own; after the second the subprocess holds nothing of the
// image while it is still there, and the file is free once it has gone.
func TestMappedSpawnedImage(t *testing.T) {
	c, d, name, out := mappedImageConsole(t, "1")

	proc := filepath.Join(t.TempDir(), "twice.com")
	text := "$ RUN " + name + "\n$ RUN " + name + "\n"

	if err := os.WriteFile(proc, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`SPAWN/NOWAIT/PROCESS=MAPSUB @"` + proc + `"`); err != nil {
		t.Fatalf("SPAWN: %v", err)
	}

	spinProcessOne(t, c)

	// Process 1's spin has a quantum of 10 million instructions; below
	// the subprocess's priority it runs only while the subprocess waits.
	if err := c.RTL.Scheduler().SetBasePriority(sched.Handle(c.RTL.Process.PID), 1); err != nil {
		t.Fatal(err)
	}

	c.Engine.RequestReschedule()

	sub, ok := c.RTL.FindProcessName(c.RTL.Process.UICGroup(), "MAPSUB")
	if !ok {
		t.Fatal("no subprocess MAPSUB")
	}

	pid := sub.Process.PID

	for run := 1; run <= 2; run++ {
		runChildTo(t, c, sub, 1)

		if got := checkChildMapped(t, c, sub); got == 0 {
			t.Fatalf("run %d: no sections", run)
		}

		main := c.ImageState(sub).FindMainICB()

		releaseChild(t, c, sub, 1)
		runChildTo(t, c, sub, 2)
		releaseChild(t, c, sub, 2)

		// The image's end: its rundown takes its sections. The ICB list
		// is still this run's (the next activation resets it), so it was
		// the rundown, not the next RUN, that let them go.
		runUntil(t, c, 400_000, func() bool {
			return sub.Deleted || strings.Count(out.String(), "mapped hello") == run && sub.ProcessSections() == 0
		})

		// After the second run the procedure ends, and the subprocess
		// with it, perhaps already.
		if sub.Deleted {
			if run == 1 {
				t.Fatal("the subprocess ended after its first run")
			}

			break
		}

		if c.ImageState(sub).FindMainICB() != main {
			t.Errorf("run %d: the next image was activated before the sections went", run)
		}

		checkNoChildFrames(t, c, pid)
		checkWorkingSets(t, c)
	}

	awaitGone(t, c, sub, 2_000_000)
	checkNoChildFrames(t, c, pid)
	checkWorkingSets(t, c)

	if err := c.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after the subprocess: %v", err)
	}
}

// TestMilestone_paging runs the multiprocessing milestone with paging
// on, so that MSPARENT.EXE, and MSCHILD.EXE in the process it starts by
// $CREPRC or LIB$SPAWN, are both mapped from the volume: the output and
// files are TestMilestone's, no process is left, and the volume, its
// image files no longer held, dismounts and verifies.
func TestMilestone_paging(t *testing.T) {
	for _, how := range []struct{ name, mode, want string }{
		{"creprc", "CREPRC", "$CREPRC"},
		{"spawn", "SPAWN", "LIB$SPAWN"},
	} {
		t.Run(how.name, func(t *testing.T) {
			setSetting(t, machine.PagingSetting, "true")

			c, out, path := milestoneMachine(t, "500")
			if c.Mem.Paging() == nil {
				t.Fatal("vax.init's VMINIT left paging off")
			}

			opts := machine.RunOptions{CommandLine: how.mode + " DUA0:[000000]MSCHILD.EXE"}
			if err := c.Run("DUA0:[000000]MSPARENT.EXE", opts); err != nil {
				t.Fatalf("RUN: %v\n%s", err, out.String())
			}

			got := programLines(out.String())
			if want := milestoneWant(how.want); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}

			if n := len(c.RTL.Processes()); n != 1 {
				t.Errorf("%d processes left, want only process 1", n)
			}

			checkMilestoneFiles(t, c)
			checkWorkingSets(t, c)
			verifyMilestoneVolume(t, c, path)
		})
	}
}

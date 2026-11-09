package console

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/sysvol"
	"github.com/tucats/govax/internal/vmserrors"
)

// ASM and LOAD on volume files (docs/PHASE-87 - system volume.md,
// subtask 6; internal/machine/consolefile.go).

// volumeFileConsole is a console with a system volume on DUA0, govax's
// built-in files behind its resolver, the console's grammar, and a
// machine to assemble into.
func volumeFileConsole(t *testing.T) (*Console, *Dispatcher, *bytes.Buffer) {
	t.Helper()

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "govax-system.dsk")})
	if !r.Mounted || !r.Writable {
		t.Fatalf("result = %+v\n%s", r, buf)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	mustDispatch(t, d, "INIT/VAX ^D4096", "INITIALIZE/MEMORY")
	buf.Reset()

	return c, d, buf
}

// writeText puts lines on DUA0 at spec as a text file.
func writeText(t *testing.T, c *Console, spec string, lines ...string) {
	t.Helper()

	writeVolumeFile(t, c, spec, sysvol.Canonical([]byte(strings.Join(lines, "\n")+"\n"), sysvol.Text), sysvol.Text)
}

// machineSymbol is the machine symbol name's value, failing t if it has none.
func machineSymbol(t *testing.T, c *Console, name string) uint32 {
	t.Helper()

	v, ok := c.Symbols.Get(name)
	if !ok {
		t.Fatalf("no symbol %s", name)
	}

	return v
}

// TestASMVolumeFile checks ASM of a file named by a VMS specification:
// it's read from its volume, and its .INCLUDE of a bare name from beside
// it there, though no host or built-in file has that name.
func TestASMVolumeFile(t *testing.T) {
	c, d, buf := volumeFileConsole(t)

	writeText(t, c, "[SYSTEM.SOURCES]TINY.ASM",
		"tiny$start:     .long   5",
		`                .include "tinydefs.asm"`)
	writeText(t, c, "[SYSTEM.SOURCES]TINYDEFS.ASM", ".set /perm TINY$VALUE ^d42")

	mustDispatch(t, d, "ASM SYS$SYSDEVICE:[SYSTEM.SOURCES]TINY.ASM")

	if v := machineSymbol(t, c, "TINY$VALUE"); v != 42 {
		t.Errorf("TINY$VALUE = %d, want 42\n%s", v, buf)
	}

	machineSymbol(t, c, "TINY$START")
}

// TestASMBareName checks a bare name with SET DEFAULT on a volume: one in
// the default directory is read from there, and one that isn't is
// looked for among the host's and govax's built-in files, as ASM did
// before it read volume files (STARTUP.COM's "kernel.asm" with no system
// volume but a work volume).
func TestASMBareName(t *testing.T) {
	c, d, buf := volumeFileConsole(t)

	writeText(t, c, "[SYSTEM]HERE.ASM", "here$label:     .long   7")
	mustDispatch(t, d, "SET DEFAULT DUA0:[SYSTEM]", "ASM HERE.ASM")
	machineSymbol(t, c, "HERE$LABEL")

	// ssdef.asm is only a built-in file (and SSDEF.ASM on the volume,
	// but not in the default directory).
	mustDispatch(t, d, `ASM "ssdef.asm"`)

	if v := machineSymbol(t, c, "SS$_ACCVIO"); v != 12 {
		t.Errorf("SS$_ACCVIO = %d\n%s", v, buf)
	}
}

// TestASMVolumeFileMissing checks an explicit volume file that isn't
// there: NOSUCHFILE with its full name, and no search of the host.
func TestASMVolumeFileMissing(t *testing.T) {
	_, d, _ := volumeFileConsole(t)

	err := d.Dispatch("ASM DUA0:[SYSTEM.SOURCES]NONE.ASM")
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Fatalf("err = %v, want NOSUCHFILE", err)
	}

	if !strings.Contains(err.Error(), "DUA0:[SYSTEM.SOURCES]NONE.ASM") {
		t.Errorf("err = %v, doesn't name the file", err)
	}

	err = d.Dispatch("ASM DUB0:[X]NONE.ASM")
	if !errors.Is(err, vmserrors.New(vmserrors.SS_DEVNOTMOUNT)) {
		t.Errorf("err = %v, want DEVNOTMOUNT", err)
	}
}

// TestLoadVolumeFiles checks LOAD/ROM of the system volume's DEFAULT.ROM,
// named in SHOW ROM by its full name; /NOERROR's quiet failure for a
// volume file that isn't there; and LOAD/NVRAM of a missing volume file.
func TestLoadVolumeFiles(t *testing.T) {
	c, d, buf := volumeFileConsole(t)

	mustDispatch(t, d, "LOAD/ROM SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM", "SHOW ROM")

	if want := `ROM FILE="SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM;1"`; !strings.Contains(buf.String(), want) {
		t.Errorf("SHOW ROM lacks %s:\n%s", want, buf)
	}

	// The same bytes as the built-in ROM.
	builtinROM := c.Engine.Memory().ROM

	c2, d2, _ := volumeFileConsole(t)
	mustDispatch(t, d2, "LOAD/ROM/NOERROR")

	if !bytes.Equal(builtinROM, c2.Engine.Memory().ROM) {
		t.Error("the volume's ROM differs from the built-in one")
	}

	mustDispatch(t, d, "LOAD/ROM/NOERROR DUA0:[SYSTEM.SYSEXE]NONE.ROM")

	if err := d.Dispatch("LOAD/NVRAM DUA0:[SYSTEM.SYSEXE]NONE.NVRAM"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("LOAD/NVRAM of a missing file: err = %v", err)
	}
}

// TestBootFromEditedKernel checks STARTUP.COM's kernel on the system
// volume: an edited KERNEL.ASM is the one that boots, with its .INCLUDE
// of a file beside it, and the ROM is the volume's.
func TestBootFromEditedKernel(t *testing.T) {
	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "govax-system.dsk")})
	if !r.Mounted || !r.Writable {
		t.Fatalf("result = %+v", r)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	kernel, _ := sysvol.Lookup(sysvol.Sources + "KERNEL.ASM")
	text, _ := kernel.Contents()

	const (
		include = `.include "ssdef.asm"`
		message = `"%SYSTEM-S-INIT, Kernel initialized"`
	)

	edited := string(text)
	if !strings.Contains(edited, include) || !strings.Contains(edited, message) {
		t.Fatal("kernel.asm isn't as this test expects")
	}

	edited = strings.Replace(edited, include, `.include "mydefs.asm"`, 1)
	edited = strings.Replace(edited, message, `"%SYSTEM-S-INIT, Edited kernel initialized"`, 1)
	writeVolumeFile(t, c, kernel.Spec(), []byte(edited), sysvol.Text)

	defs, _ := sysvol.Lookup(sysvol.Sources + "SSDEF.ASM")
	defsText, _ := defs.Contents()
	writeVolumeFile(t, c, sysvol.Sources+"MYDEFS.ASM", defsText, sysvol.Text)

	buf.Reset()

	if err := c.RunStartup(d.Dispatch); err != nil {
		t.Fatalf("RunStartup: %v\n%s", err, buf)
	}

	if !c.KernelPlaced() || c.InDebugger() {
		t.Errorf("kernel placed %v, in the debugger %v\n%s", c.KernelPlaced(), c.InDebugger(), buf)
	}

	if !strings.Contains(buf.String(), "%SYSTEM-S-INIT, Edited kernel initialized\n") {
		t.Errorf("the edited kernel didn't boot:\n%s", buf)
	}

	if got, want := c.Engine.Memory().ROMFile, "SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;1"; got != want {
		t.Errorf("ROM file = %q, want %q", got, want)
	}
}

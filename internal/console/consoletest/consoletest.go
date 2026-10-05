// Package consoletest is test support for packages that drive a
// console.Console from the outside: internal/debugger's tests, and any
// later package built on the console. It exports what internal/console's
// own tests keep private -- a console ready to run programs, the paths of
// the kernel and the fixtures, and the grammars -- so that each package
// doesn't copy them.
//
// It is for tests only. Nothing in a govax build imports it.
package consoletest

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/respath"
)

// root returns the repository's root directory, found from this file's
// own location, so tests work from any package's directory.
func root(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	// This file is <root>/internal/console/consoletest/consoletest.go.
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// RepoPath returns the path of a file in the repository, given as its
// elements below the repository's root: RepoPath(t, "testdata", "dbg").
func RepoPath(t testing.TB, elements ...string) string {
	t.Helper()

	return filepath.Join(append([]string{root(t)}, elements...)...)
}

// BootFile returns the path of one of the files govax embeds for its own
// start-up (internal/bootdata/files): the grammars, the help files,
// kernel.asm, and so on.
func BootFile(t testing.TB, name string) string {
	t.Helper()

	return RepoPath(t, "internal", "bootdata", "files", name)
}

// KernelPath returns the path of kernel.asm, the microkernel that an
// image's run needs assembled first.
func KernelPath(t testing.TB) string {
	t.Helper()

	return BootFile(t, "kernel.asm")
}

// DebugImagePath returns the path of one of Phase 41's debugger probe
// images in testdata/dbg/vax, such as "dbgdis.exe".
func DebugImagePath(t testing.TB, name string) string {
	t.Helper()

	return RepoPath(t, "testdata", "dbg", "vax", name)
}

// ConsoleGrammar loads the console's grammar (console.dcl) as govax
// parses it at start-up.
func ConsoleGrammar(t testing.TB) *dcl.Grammar {
	t.Helper()

	return loadGrammar(t, "console.dcl")
}

// DebugGrammar loads the debugger's grammar (debug.dcl).
func DebugGrammar(t testing.TB) *dcl.Grammar {
	t.Helper()

	return loadGrammar(t, "debug.dcl")
}

func loadGrammar(t testing.TB, name string) *dcl.Grammar {
	t.Helper()

	g, err := dcl.LoadGrammarFile(BootFile(t, name))
	if err != nil {
		t.Fatalf("loading %s: %v", name, err)
	}

	return g
}

// ParseHelp reads one of the help files (vax.help, debug.help).
func ParseHelp(t testing.TB, name string) *console.Help {
	t.Helper()

	h, err := console.LoadHelpFile(BootFile(t, name))
	if err != nil {
		t.Fatalf("loading %s: %v", name, err)
	}

	return h
}

// New returns a console with a machine ready to run programs: 2 MB of
// memory, with VMINIT's page tables and stacks, as vax.init builds them
// but without assembling the microkernel (a test that runs an image
// assembles it with KernelPath). Its output goes to the returned buffer.
// Its file search path ends in govax's embedded files, as cmd/govax's.
func New(t testing.TB) (*console.Console, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)

	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2000, 200, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	return c, &out
}

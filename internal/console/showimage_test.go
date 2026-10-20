package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
)

// The debugger's SHOW IMAGE/FULL shows how an image's sections are laid
// out in memory: each section's addresses, size, protection, and backing
// store, and what its pages are mapped to now. With paging on, the
// image is mapped from its file, so its pages move from "not yet read"
// into memory, and out to the free and modified lists, as it runs; with
// paging off, it's copied.

// TestShowImageFull_mapped: at the image's first stop (pfc 1, so only
// the pages touched are valid) its never-run code is still in its
// section; at its second, after 40 array pages through a 30-page working
// set, the array's pages are valid and in transition on the modified
// list, all backed by the page file.
func TestShowImageFull_mapped(t *testing.T) {
	c, d, name, out := mappedImageConsole(t, "1")

	if err := d.Dispatch("RUN/DEBUG " + name); err != nil {
		t.Fatalf("RUN/DEBUG: %v", err)
	}

	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)

	show := func() string {
		t.Helper()
		out.Reset()

		if err := d.Dispatch("SHOW IMAGE/FULL"); err != nil {
			t.Fatalf("SHOW IMAGE/FULL: %v", err)
		}

		return out.String()
	}

	first := show()

	for _, want := range []string{
		"    file DUA0:[000000]IMAP.EXE;1, mapped\n",
		"    section       start     end       pages  prot  backing store\n",
		"    demand zero   00000400  000053FF     40  ALL   page file\n",
		"                  pages: 40 demand zero\n",
		"    read-only     00005400  00005DFF      5  UR    image file from VBN 3 (section 1)\n",
		"    fixup         00005E00  00005FFF      1  UREW  image file from VBN 8, then the page file (section 2)\n",
		"    user stack                           20        the process's own stack, not mapped\n",
		"    global        LIBRTL_001",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("at the first stop, SHOW IMAGE/FULL lacks %q:\n%s", want, first)
		}
	}

	// The read-only section: the code run so far is valid, the 4 pages
	// never run (R8's) not yet read.
	if !strings.Contains(first, ", 4 not yet read\n") {
		t.Errorf("at the first stop, the code never run isn't shown as not yet read:\n%s", first)
	}

	toCheckpoint2(t, c)

	second := show()

	for _, want := range []string{
		"                  pages: 28 valid, 12 in transition (on the modified list)\n",
		"                  in memory: 40 backed by the page file\n",
	} {
		if !strings.Contains(second, want) {
			t.Errorf("at the second stop, SHOW IMAGE/FULL lacks %q:\n%s", want, second)
		}
	}
}

// TestShowImageFull_copied: with paging off the image is copied: every
// page is valid, with no backing store.
func TestShowImageFull_copied(t *testing.T) {
	setSetting(t, machine.PagingSetting, "false")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	host := buildImage(t, c, "imap", imageMapSource)

	if err := d.Dispatch(`RUN/DEBUG "` + host + `"`); err != nil {
		t.Fatalf("RUN/DEBUG: %v", err)
	}

	out.Reset()

	if err := d.Dispatch("SHOW IMAGE/FULL"); err != nil {
		t.Fatalf("SHOW IMAGE/FULL: %v", err)
	}

	got := out.String()

	for _, want := range []string{
		", copied into memory\n",
		"    demand zero   00000400  000053FF     40  ALL   none (zeroed in memory)\n",
		"    read-only     00005400  00005DFF      5  UR    none (copied from VBN 3)\n",
		"                  pages: 5 valid\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SHOW IMAGE/FULL lacks %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "in memory:") {
		t.Errorf("SHOW IMAGE/FULL shows backing store with paging off:\n%s", got)
	}
}

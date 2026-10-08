package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 5: global sections, through the services on a
// booted machine's process 1. The sections shared by two processes are
// in gblpair_test.go.

// Scratch addresses for the section tests: a name descriptor (with its
// text after it), an ident quadword, and P0 pages for the mappings.
const (
	secName  = 0x8100
	secIdent = 0x8200
	secPages = 0x50000
)

// The flags and statuses the section tests use.
var (
	secGBL    = vmsdef.LibrarySymbols["SEC$M_GBL"]
	secWRT    = vmsdef.LibrarySymbols["SEC$M_WRT"]
	secPERM   = vmsdef.LibrarySymbols["SEC$M_PERM"]
	secSYSGBL = vmsdef.LibrarySymbols["SEC$M_SYSGBL"]
	secPAGFIL = vmsdef.LibrarySymbols["SEC$M_PAGFIL"]
	secEXPREG = vmsdef.LibrarySymbols["SEC$M_EXPREG"]
	secDZRO   = vmsdef.LibrarySymbols["SEC$M_DZRO"]
	secCRF    = vmsdef.LibrarySymbols["SEC$M_CRF"]

	ssCreated    = vmsdef.Symbols["SS$_CREATED"]
	ssNoSuchSec  = vmsdef.Symbols["SS$_NOSUCHSEC"]
	ssIvSecFlg   = vmsdef.Symbols["SS$_IVSECFLG"]
	ssIvSecIdCtl = vmsdef.Symbols["SS$_IVSECIDCTL"]
	ssGsdFull    = vmsdef.Symbols["SS$_GSDFULL"]
	ssGptFull    = vmsdef.Symbols["SS$_GPTFULL"]
	ssIvLogNam   = vmsdef.Symbols["SS$_IVLOGNAM"]
	ssUnsupport  = vmsdef.Symbols["SS$_UNSUPPORTED"]
)

// pageFile is the flags of a writable page-file global section.
var pageFile = secGBL | secPAGFIL | secDZRO | secWRT

// putName writes a string descriptor for name at secName.
func putName(t *testing.T, c *Console, name string) uint32 {
	t.Helper()

	if err := c.storeLong(secName, uint32(len(name))); err != nil {
		t.Fatal(err)
	}

	if err := c.storeLong(secName+4, secName+8); err != nil {
		t.Fatal(err)
	}

	if err := c.storeBytes(secName+8, []byte(name)); err != nil {
		t.Fatal(err)
	}

	return secName
}

// putIdent writes an ident quadword (match control, version) at
// secIdent.
func putIdent(t *testing.T, c *Console, control, version uint32) uint32 {
	t.Helper()

	putRange(t, c, secIdent, control, version)

	return secIdent
}

// crmpsc calls $CRMPSC for a section name of pagcnt pages, mapped at
// the pages from first to last, with flags and ident (an address, or 0).
func crmpsc(t *testing.T, c *Console, name string, flags, first, last, pagcnt, ident uint32) uint32 {
	t.Helper()

	putRange(t, c, vaInadr, first, last)

	return callService(t, c, "SYS$CRMPSC", vaInadr, vaRetadr, 0, flags, putName(t, c, name), ident, 0, 0, pagcnt, 0, 0, 0)
}

// mgblsc calls $MGBLSC for section name, from page relpag, at the pages
// from first to last.
func mgblsc(t *testing.T, c *Console, name string, flags, first, last, relpag, ident uint32) uint32 {
	t.Helper()

	putRange(t, c, vaInadr, first, last)

	return callService(t, c, "SYS$MGBLSC", vaInadr, vaRetadr, 0, flags, putName(t, c, name), ident, relpag)
}

// section returns the section named name process 1's system has, or
// nil.
func section(c *Console, name string) *corevms.GlobalSection {
	for _, s := range c.RTL.Sections.Sections() {
		if s.Name == name {
			return s
		}
	}

	return nil
}

// TestCrmpsc_createAndMap: $CRMPSC creates a section and maps it (only
// its own two pages of a three-page range); a second $CRMPSC of the name
// maps the same physical pages elsewhere (SS$_NORMAL); a read-only
// $MGBLSC from page 1 maps one page, read-only; what is stored through
// one mapping is read through the others.
func TestCrmpsc_createAndMap(t *testing.T) {
	c := newServiceConsole(t)

	if got := crmpsc(t, c, "SHARE", pageFile, secPages, secPages+0x400, 2, 0); got != ssCreated {
		t.Fatalf("$CRMPSC = %#x, want SS$_CREATED", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != secPages || b != secPages+0x3FF {
		t.Errorf("retadr %#x-%#x, want %#x-%#x", a, b, secPages, secPages+0x3FF)
	}

	s := section(c, "SHARE")
	if s == nil || len(s.Frames) != 2 || s.Refs != 2 {
		t.Fatalf("section %+v: want 2 frames, 2 references", s)
	}

	pte := pteAt(t, c, secPages)
	if !pte.Valid() || pte.PFN() != s.Frames[0] {
		t.Errorf("mapped PTE %#x: want valid, PFN %#x", uint32(pte), s.Frames[0])
	}

	if pte := pteAt(t, c, secPages+0x400); pte.Valid() {
		t.Errorf("the third page was mapped: %#x", uint32(pte))
	}

	// The same name again: mapped, not created.
	if got := crmpsc(t, c, "SHARE", pageFile, secPages+0x1000, secPages+0x13FF, 2, 0); got != ssNormal {
		t.Fatalf("second $CRMPSC = %#x, want SS$_NORMAL", got)
	}

	if pteAt(t, c, secPages+0x1200).PFN() != s.Frames[1] || s.Refs != 4 {
		t.Errorf("second mapping: PFN %#x, %d references", pteAt(t, c, secPages+0x1200).PFN(), s.Refs)
	}

	// Read-only, from the section's second page, into a two-page range.
	if got := mgblsc(t, c, "SHARE", 0, secPages+0x2000, secPages+0x23FF, 1, 0); got != ssNormal {
		t.Fatalf("$MGBLSC = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != secPages+0x2000 || b != secPages+0x21FF {
		t.Errorf("$MGBLSC retadr %#x-%#x, want one page", a, b)
	}

	ro := pteAt(t, c, secPages+0x2000)
	if ro.PFN() != s.Frames[1] || ro.Protection() == pte.Protection() {
		t.Errorf("read-only PTE %#x: want PFN %#x, protection other than %d", uint32(ro), s.Frames[1], pte.Protection())
	}

	if err := c.storeLong(secPages+0x204, 0x12345678); err != nil {
		t.Fatal(err)
	}

	for _, addr := range []uint32{secPages + 0x1204, secPages + 0x2004} {
		if v, err := c.Mem.LoadLongword(c.CPU, addr); err != nil || v != 0x12345678 {
			t.Errorf("at %#x: %#x, %v; want the stored value", addr, v, err)
		}
	}
}

// TestCrmpsc_errors: the flag checks, the arguments, and writable
// mapping of a read-only section.
func TestCrmpsc_errors(t *testing.T) {
	c := newServiceConsole(t)

	cases := []struct {
		what   string
		name   string
		flags  uint32
		pagcnt uint32
		want   uint32
	}{
		{"an unknown flag", "X", pageFile | 1<<30, 1, ssIvSecFlg},
		{"page file without GBL", "X", secPAGFIL, 1, ssIvSecFlg},
		{"PERM without GBL", "X", secPERM, 1, ssIvSecFlg},
		{"copy on reference in the page file", "X", pageFile | secCRF, 1, ssIvSecFlg},
		{"a file section", "X", secGBL | secWRT, 1, ssUnsupport},
		{"a private section", "X", secWRT, 1, ssUnsupport},
		{"no pages", "X", pageFile, 0, ssIllPagCnt},
		{"no name", "", pageFile, 1, ssIvLogNam},
		{"a long name", strings.Repeat("N", 44), pageFile, 1, ssIvLogNam},
	}

	for _, tc := range cases {
		if got := crmpsc(t, c, tc.name, tc.flags, secPages, secPages, tc.pagcnt, 0); got != tc.want {
			t.Errorf("%s: %#x, want %#x", tc.what, got, tc.want)
		}
	}

	if got := mgblsc(t, c, "NONE", 0, secPages, secPages, 0, 0); got != ssNoSuchSec {
		t.Errorf("$MGBLSC of no section: %#x, want SS$_NOSUCHSEC", got)
	}

	if got := mgblsc(t, c, "NONE", secPAGFIL, secPages, secPages, 0, 0); got != ssIvSecFlg {
		t.Errorf("$MGBLSC with SEC$M_PAGFIL: %#x, want SS$_IVSECFLG", got)
	}

	// A read-only section, kept mapped so it lasts.
	if got := crmpsc(t, c, "RO", secGBL|secPAGFIL, secPages, secPages, 1, 0); got != ssCreated {
		t.Fatalf("$CRMPSC RO = %#x", got)
	}

	if got := mgblsc(t, c, "RO", secWRT, secPages+0x200, secPages+0x200, 0, 0); got != ssNoPriv {
		t.Errorf("writable mapping of a read-only section: %#x, want SS$_NOPRIV", got)
	}

	if got := mgblsc(t, c, "RO", 0, secPages+0x200, secPages+0x200, 1, 0); got != ssBadParam {
		t.Errorf("relpag past the section: %#x, want SS$_BADPARAM", got)
	}

	// A group section isn't found as a system one.
	if got := mgblsc(t, c, "RO", secSYSGBL, secPages+0x200, secPages+0x200, 0, 0); got != ssNoSuchSec {
		t.Errorf("$MGBLSC/SYSGBL of a group section: %#x, want SS$_NOSUCHSEC", got)
	}

	// Without the privileges, no system or permanent section.
	p := c.RTL.Process
	saved := p.CurrentPrivileges
	p.CurrentPrivileges = 0

	t.Cleanup(func() { p.CurrentPrivileges = saved })

	for _, flags := range []uint32{pageFile | secSYSGBL, pageFile | secPERM} {
		if got := crmpsc(t, c, "PRIV", flags, secPages+0x400, secPages+0x400, 1, 0); got != ssNoPriv {
			t.Errorf("flags %#x without privileges: %#x, want SS$_NOPRIV", flags, got)
		}
	}
}

// TestCrmpsc_protection: a section's protection mask decides who may map
// it: with the owner's access denied, process 1 (the owner, but in a
// system group, so in the SYSTEM category too) still may, until it's
// moved out of the system groups.
func TestCrmpsc_protection(t *testing.T) {
	c := newServiceConsole(t)

	putRange(t, c, vaInadr, secPages, secPages)

	// prot: SYSTEM, OWNER, GROUP, and WORLD all denied write.
	prot := uint32(0x2222)
	if got := callService(t, c, "SYS$CRMPSC", vaInadr, 0, 0, pageFile, putName(t, c, "PROT"), 0, 0, 0, 1, 0, prot, 0); got != ssCreated {
		t.Fatalf("$CRMPSC = %#x", got)
	}

	p := c.RTL.Process
	saved := p.CurrentPrivileges
	p.CurrentPrivileges = 0

	t.Cleanup(func() { p.CurrentPrivileges = saved })

	if got := mgblsc(t, c, "PROT", 0, secPages+0x200, secPages+0x200, 0, 0); got != ssNormal {
		t.Errorf("read-only mapping: %#x, want SS$_NORMAL", got)
	}

	if got := mgblsc(t, c, "PROT", secWRT, secPages+0x400, secPages+0x400, 0, 0); got != ssNoPriv {
		t.Errorf("writable mapping with write denied: %#x, want SS$_NOPRIV", got)
	}
}

// TestCrmpsc_ident: version idents, created with one and matched by
// each match control.
func TestCrmpsc_ident(t *testing.T) {
	c := newServiceConsole(t)

	const version = 0x01000005 // major 1, minor 5

	if got := crmpsc(t, c, "VER", pageFile, secPages, secPages, 1, putIdent(t, c, 1, version)); got != ssCreated {
		t.Fatalf("$CRMPSC = %#x", got)
	}

	cases := []struct {
		control, version, want uint32
	}{
		{0, 0x07000000, ssNormal},    // MATALL: any
		{1, version, ssNormal},       // MATEQU: the same
		{1, 0x01000004, ssNoSuchSec}, // MATEQU: another minor
		{2, 0x01000003, ssNormal},    // MATLEQ: an earlier minor
		{2, 0x01000007, ssNoSuchSec}, // MATLEQ: a later minor
		{2, 0x02000001, ssNoSuchSec}, // MATLEQ: another major
		{3, version, ssIvSecIdCtl},   // no such match control
		{1, version, ssNormal},       // still there
	}

	for i, tc := range cases {
		addr := secPages + uint32(i+1)*0x200
		if got := mgblsc(t, c, "VER", 0, addr, addr, 0, putIdent(t, c, tc.control, tc.version)); got != tc.want {
			t.Errorf("control %d, version %08X: %#x, want %#x", tc.control, tc.version, got, tc.want)
		}
	}

	// A $CRMPSC asking for another version creates a second section of
	// the name.
	if got := crmpsc(t, c, "VER", pageFile, secPages+0x2000, secPages+0x2000, 1, putIdent(t, c, 1, 0x02000000)); got != ssCreated {
		t.Errorf("$CRMPSC of another version: %#x, want SS$_CREATED", got)
	}
}

// TestCrmpsc_lifetime: a temporary section goes with its last mapping,
// its physical pages freed; a permanent one stays unmapped until
// $DGBLSC, which deletes its name at once (a new section may take it)
// and the section with its last mapping; image rundown unmaps.
func TestCrmpsc_lifetime(t *testing.T) {
	c := newServiceConsole(t)

	// The scratch page the arguments go in is touched first, so that
	// it's counted in before.
	putName(t, c, "")
	putRange(t, c, vaInadr, 0, 0)

	before := c.Mem.MappedPages()

	// Temporary: two mappings, deleted one after the other.
	crmpsc(t, c, "TEMP", pageFile, secPages, secPages+0x200, 2, 0)
	mgblsc(t, c, "TEMP", secWRT, secPages+0x1000, secPages+0x1200, 0, 0)

	putRange(t, c, vaInadr, secPages, secPages+0x200)
	callService(t, c, "SYS$DELTVA", vaInadr, 0, 0)

	if s := section(c, "TEMP"); s == nil || s.Refs != 2 {
		t.Fatalf("after one mapping's deletion: %+v, want 2 references", s)
	}

	putRange(t, c, vaInadr, secPages+0x1000, secPages+0x1200)
	callService(t, c, "SYS$DELTVA", vaInadr, 0, 0)

	if s := section(c, "TEMP"); s != nil {
		t.Errorf("TEMP outlived its mappings: %+v", s)
	}

	if after := c.Mem.MappedPages(); after != before {
		t.Errorf("%d physical pages in use, %d before: the section's weren't freed", after, before)
	}

	// Permanent: stays with no mapping.
	putRange(t, c, vaInadr, secPages, secPages)
	callService(t, c, "SYS$DELTVA", vaInadr, 0, 0) // nothing there
	crmpsc(t, c, "PERM", pageFile|secPERM, secPages, secPages, 1, 0)

	if err := c.storeLong(secPages, 0xFEEDF00D); err != nil {
		t.Fatal(err)
	}

	callService(t, c, "SYS$DELTVA", vaInadr, 0, 0)

	if s := section(c, "PERM"); s == nil || s.Refs != 0 {
		t.Fatalf("permanent section with no mapping: %+v", s)
	}

	// Mapped again, its contents are still there.
	mgblsc(t, c, "PERM", 0, secPages+0x200, secPages+0x200, 0, 0)

	if v, _ := c.Mem.LoadLongword(c.CPU, secPages+0x200); v != 0xFEEDF00D {
		t.Errorf("remapped permanent section reads %#x", v)
	}

	// $DGBLSC: the name goes, the section stays while mapped.
	old := section(c, "PERM")

	if got := callService(t, c, "SYS$DGBLSC", 0, putName(t, c, "PERM"), 0); got != ssNormal {
		t.Fatalf("$DGBLSC = %#x", got)
	}

	if got := mgblsc(t, c, "PERM", 0, secPages+0x400, secPages+0x400, 0, 0); got != ssNoSuchSec {
		t.Errorf("$MGBLSC after $DGBLSC: %#x, want SS$_NOSUCHSEC", got)
	}

	if got := crmpsc(t, c, "PERM", pageFile, secPages+0x400, secPages+0x400, 1, 0); got != ssCreated {
		t.Errorf("a new PERM: %#x, want SS$_CREATED", got)
	}

	if v, _ := c.Mem.LoadLongword(c.CPU, secPages+0x200); v != 0xFEEDF00D || !old.DeletePending || len(old.Frames) != 1 {
		t.Errorf("deleted section while mapped: reads %#x, %+v", v, old)
	}

	if got := callService(t, c, "SYS$DGBLSC", 0, putName(t, c, "GONE"), 0); got != ssNoSuchSec {
		t.Errorf("$DGBLSC of no section: %#x, want SS$_NOSUCHSEC", got)
	}

	// Image rundown unmaps both: the old PERM goes, and so does the new
	// (temporary) one; the pages are demand-zero pages again.
	c.RTL.ImageRundown()

	if len(old.Frames) != 0 || section(c, "PERM") != nil {
		t.Errorf("after image rundown: old %+v, new %+v", old, section(c, "PERM"))
	}

	if pte := pteAt(t, c, secPages+0x200); pte != corevms.ProcessPTE(false, (secPages+0x200)/512) {
		t.Errorf("unmapped PTE %#x, want a new process's", uint32(pte))
	}

	if after := c.Mem.MappedPages(); after != before {
		t.Errorf("%d physical pages in use, %d before", after, before)
	}
}

// TestCrmpsc_limits: GBLSECTIONS and GBLPAGES, and SEC$M_EXPREG placing
// a section above P0's high-water mark and moving it.
func TestCrmpsc_limits(t *testing.T) {
	c := newServiceConsole(t)
	g := c.RTL.Sections

	crmpsc(t, c, "ONE", pageFile, secPages, secPages+0x200, 2, 0)

	g.SectionLimit = 1
	if got := crmpsc(t, c, "TWO", pageFile, secPages+0x400, secPages+0x400, 1, 0); got != ssGsdFull {
		t.Errorf("past GBLSECTIONS: %#x, want SS$_GSDFULL", got)
	}

	g.SectionLimit, g.PageLimit = 10, 3
	if got := crmpsc(t, c, "TWO", pageFile, secPages+0x400, secPages+0x600, 2, 0); got != ssGptFull {
		t.Errorf("past GBLPAGES: %#x, want SS$_GPTFULL", got)
	}

	// EXPREG: at the first page above the high-water mark.
	mark := c.RTL.RegionSize[0]
	want := (mark + 0x1FF) &^ 0x1FF

	if got := callService(t, c, "SYS$CRMPSC", 0, vaRetadr, 0, pageFile|secEXPREG, putName(t, c, "TWO"), 0, 0, 0, 1, 0, 0, 0); got != ssCreated {
		t.Fatalf("$CRMPSC/EXPREG = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != want || b != want+0x1FF {
		t.Errorf("EXPREG mapped %#x-%#x, want %#x-%#x", a, b, want, want+0x1FF)
	}

	if c.RTL.RegionSize[0] != want+0x200 {
		t.Errorf("high-water mark %#x, want %#x", c.RTL.RegionSize[0], want+0x200)
	}

	if pte := pteAt(t, c, want); !pte.Valid() || pte.PFN() != section(c, "TWO").Frames[0] {
		t.Errorf("EXPREG PTE %#x", uint32(pte))
	}

}

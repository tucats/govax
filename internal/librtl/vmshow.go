package librtl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmheap"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIBRTL2's zone routines, for debugging a program's use of zones (5.0
// LIB$ manual, chapter 6, and LIB-163, LIB-359, LIB-464):
// LIB$FIND_VM_ZONE walks the image's zones, LIB$SHOW_VM_ZONE describes
// one, and LIB$VERIFY_VM_ZONE checks one. What a zone holds and how it's
// checked are corevms's (vmshow.go): the zones' bookkeeping is in Go, so
// the checks are of what the program can damage, a zone's control block
// and the free fill in its free space.
//
// LIB$SHOW_VM_ZONE's report is laid out as VMS 7.1 lays it out
// (testdata/lib59, which showed every section and level); the link
// analysis of a damaged block shows govax's own links, which the program
// can't see, so its addresses aren't VMS's.

var libBadZone = vmsdef.LibrarySymbols["LIB$_BADZONE"]

// zoneFailure maps corevms's zone errors to the debugging routines'
// conditions: LIB$_BADZONE for an ID that names no zone and for a zone
// found damaged.
func zoneFailure(env *corevms.Environment, err error) (uint32, error) {
	switch {
	case errors.Is(err, corevms.ErrBadZone), errors.Is(err, corevms.ErrZoneDamaged):
		return libBadZone, nil
	case errors.Is(err, vmheap.ErrNoMemory):
		return libInsVirMem, nil
	}

	return memoryFault(env, err)
}

// libFindVMZone is LIB$FIND_VM_ZONE:
//
//	LIB$FIND_VM_ZONE context.mlu.r ,zone-id.wlu.r
//
// Each call writes the ID of the next of the image's zones to zone-id:
// the default zone, the string zone, then the rest, user-defined zones
// too, in VMS 7.1's order (corevms.Environment.NextZone). context is the
// scan's: 0 starts it, and it then holds where the zone last found is
// (govax's form; the manual leaves it the RTL's). A context of 0 checks
// every zone's control block first, and a damaged one is LIB$_BADZONE,
// nothing written. After the last zone it's LIB$_NOTFOU (an alternate success),
// zone-id and context left alone. Not exactly two arguments is
// LIB$_WRONUMARG.
func libFindVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 {
		return libWroNumArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	context, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if context == 0 {
		if err := env.VerifyZones(); err != nil {
			return zoneFailure(env, err)
		}
	}

	id, next, ok, err := env.NextZone(context)

	switch {
	case err != nil:
		return zoneFailure(env, err)
	case !ok:
		return libNotFou, nil
	}

	if err := mem.StoreLongword(cpu, arg(argv, 1), id); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, arg(argv, 0), next); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libVerifyVMZone is LIB$VERIFY_VM_ZONE:
//
//	LIB$VERIFY_VM_ZONE zone-id.rlu.r
//
// It checks the zone zone-id names (0, the default zone):
// corevms.Environment.VerifyZone. SS$_NORMAL for a sound zone,
// LIB$_BADZONE for one damaged or an ID that names no zone; zone-id
// omitted is LIB$_INVARG ("invalid or null argument"), and not exactly
// one argument LIB$_WRONUMARG.
func libVerifyVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 1 {
		return libWroNumArg, nil
	}

	if arg(argv, 0) == 0 {
		return libInvArg, nil
	}

	id, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if err := env.VerifyZone(id); err != nil {
		return zoneFailure(env, err)
	}

	return ssNormal, nil
}

// libShowVMZone is LIB$SHOW_VM_ZONE:
//
//	LIB$SHOW_VM_ZONE zone-id.rlu.r [,detail-level.rl.r]
//	    [,user-action-procedure.szem.v] [,user-arg.rz.v]
//
// It describes the zone zone-id names (0, the default zone) at
// detail-level 0 to 3 (0 when omitted; any other is 3, as VMS 7.1 takes
// it, testdata/lib59): the lines of zoneReport. Each line goes to
// SYS$OUTPUT as LIB$PUT_OUTPUT writes it, or, with an action routine, to
// the routine, called with a descriptor of the line and user-arg (0 when
// omitted); a failure status from either ends the report and is the
// routine's ("success codes are ignored"). A report that finds the zone
// damaged (a free block without its free fill, a lookaside list linked
// to itself) shows the damage and is LIB$_BADZONE. An ID that names no
// zone is LIB$_BADZONE; zone-id omitted LIB$_INVARG; no arguments, or
// more than four, LIB$_WRONUMARG.
//
// The action routine is called back once a line, so a call with one
// runs on a goroutine of its own (corevms.Environment.RunCallingBack),
// having made the report and changed nothing.
func libShowVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 || len(argv) > 4 {
		return libWroNumArg, nil
	}

	if arg(argv, 0) == 0 {
		return libInvArg, nil
	}

	id, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	detail, err := optionalLongword(env, arg(argv, 1), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	detail = min(detail, 3)

	r, err := env.ZoneReportOf(id)
	if err != nil {
		return zoneFailure(env, err)
	}

	action := arg(argv, 2)
	if action != 0 && !env.CallingBack() {
		return env.RunCallingBack("LIB$SHOW_VM_ZONE", libShowVMZone, argv)
	}

	lines, damaged, err := zoneReport(env, r, int(detail))
	if err != nil {
		return memoryFault(env, err)
	}

	for _, line := range lines {
		status, err := showZoneLine(env, action, arg(argv, 3), line)
		if err != nil || status&1 == 0 {
			return status, err
		}
	}

	if damaged {
		return libBadZone, nil
	}

	return ssNormal, nil
}

// zoneLine is one line of LIB$SHOW_VM_ZONE's report. A plain line is
// one with nothing in it but its text (a blank line, a heading), which
// VMS 7.1 gives an action routine as a class S text descriptor; any
// other comes from $FAO, its descriptor's type and class 0
// (testdata/lib59).
type zoneLine struct {
	text  string
	plain bool
}

// showZoneLine writes one of LIB$SHOW_VM_ZONE's lines: to SYS$OUTPUT, or
// to the action routine (with value, the user argument), its descriptor
// below SP for the call.
func showZoneLine(env *corevms.Environment, action, value uint32, line zoneLine) (uint32, error) {
	if action == 0 {
		return env.PutOutput(line.text)
	}

	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)

	defer cpu.SetGPR(registers.SP, sp)

	desc, err := stackString(env, line.text)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	if !line.plain {
		if err := env.Memory().StoreWord(cpu, desc+2, 0); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	status, err := env.Callback(action, desc, value)
	if err != nil {
		return memoryFault(env, err)
	}

	return status, nil
}

// zoneAlgorithms are the algorithms' names, by vmheap's Algorithm.
var zoneAlgorithms = []string{
	vmheap.FirstFit:      "LIB$K_VM_FIRST_FIT",
	vmheap.QuickFit:      "LIB$K_VM_QUICK_FIT",
	vmheap.FrequentSizes: "LIB$K_VM_FREQ_SIZES",
	vmheap.FixedSize:     "LIB$K_VM_FIXED",
}

// zoneListTitles are the lookaside list summaries' titles, by algorithm
// (a fixed size zone has none).
var zoneListTitles = map[vmheap.Algorithm]string{
	vmheap.QuickFit:      "Quick Fit Lookaside List Summary:",
	vmheap.FrequentSizes: "Frequent Sizes Lookaside List Summary:",
}

// zoneFlag is one of the flags LIB$SHOW_VM_ZONE shows, its name, and
// whether a zone's settings have it. VMS 7.1 shows bits 0 to 5 only,
// a GET_FILL1 or FREE_FILL1 given with its FILL0 left out (testdata/
// lib59).
type zoneFlag struct {
	name string
	bit  uint32
	set  func(corevms.ZoneSettings) bool
}

// zoneFlags are the flags in bit order, as the report lists them.
var zoneFlags = []zoneFlag{
	{"LIB$M_VM_BOUNDARY_TAGS", vmBoundaryTags, func(s corevms.ZoneSettings) bool { return s.BoundaryTags }},
	{"LIB$M_VM_GET_FILL0", vmGetFill0, func(s corevms.ZoneSettings) bool { return s.GetFill == corevms.FillZero }},
	{"LIB$M_VM_GET_FILL1", vmGetFill1, func(s corevms.ZoneSettings) bool { return s.GetFill == corevms.FillOnes }},
	{"LIB$M_VM_FREE_FILL0", vmFreeFill0, func(s corevms.ZoneSettings) bool { return s.FreeFill == corevms.FillZero }},
	{"LIB$M_VM_FREE_FILL1", vmFreeFill1, func(s corevms.ZoneSettings) bool { return s.FreeFill == corevms.FillOnes }},
	{"LIB$M_VM_EXTEND_AREA", vmExtendArea, func(s corevms.ZoneSettings) bool { return s.ExtendArea }},
}

// zoneReporter builds a report's lines.
type zoneReporter struct {
	lines []zoneLine
}

// plain adds a line of text alone; line adds a formatted one, after a
// tab (as all but the first line are, VMS 7.1's layout).
func (z *zoneReporter) plain(text string) {
	z.lines = append(z.lines, zoneLine{text, true})
}

func (z *zoneReporter) line(format string, args ...any) {
	z.lines = append(z.lines, zoneLine{"\t" + fmt.Sprintf(format, args...), false})
}

func (z *zoneReporter) blank() {
	z.plain("")
}

// zoneReport returns LIB$SHOW_VM_ZONE's lines for the zone r describes
// at detail level detail, laid out as VMS 7.1 lays them out
// (testdata/lib59), and whether it found the zone damaged:
//
//   - 0: the zone's ID and name;
//   - 1: its algorithm, flags, and sizes, the bytes freed and not yet
//     allocated again, and the bytes of its control blocks (a
//     user-defined zone shows its user argument instead);
//   - 2: a lookaside list summary (quick fit and frequent sizes: the
//     lists that hold blocks), and the area summary;
//   - 3: the scan of the lookaside lists and of each area's free list,
//     with each area's count of free blocks, and the damage found: a
//     list linked to itself, a free block without its free fill.
//
// The error is memory that can't be read.
func zoneReport(env *corevms.Environment, r corevms.ZoneReport, detail int) ([]zoneLine, bool, error) {
	z := &zoneReporter{}
	z.lines = []zoneLine{{fmt.Sprintf("Zone Id = %08X,  Zone name = %q", r.ID, r.Settings.Name), false}}

	if detail == 0 {
		return z.lines, false, nil
	}

	z.blank()

	if r.User != nil {
		z.line("Algorithm = (user zone)   USER_ARG = %d", r.User.Argument)

		return z.lines, false, nil
	}

	s := r.Settings
	algorithmLines(z, r)
	z.blank()

	var flags uint32

	for _, f := range zoneFlags {
		if f.set(s) {
			flags |= f.bit
		}
	}

	z.line("Flags = %08X", flags)

	for _, f := range zoneFlags {
		if f.set(s) {
			z.plain("\t\t     " + f.name)
		}
	}

	z.blank()

	areas := "areas"
	if len(r.Areas) == 1 {
		areas = "area"
	}

	limit := "None"
	if s.PageLimit > 0 {
		limit = fmt.Sprintf("%d pages", s.PageLimit)
	}

	z.line("Initial size = %7d pages     Current size = %d pages in %d %s", s.InitialShown(), r.Pages, len(r.Areas), areas)
	z.line("Extend size  = %7d pages     Page limit   = %s", s.ExtendPages, limit)
	z.blank()
	z.line("Requests are rounded up to a multiple of %d bytes,", max(r.Quantum, r.Alignment))
	z.line("naturally aligned on %d byte boundaries", max(r.Quantum, r.Alignment))
	z.blank()
	z.line("%d bytes have been freed and not yet reallocated", r.FreeBytes())
	z.blank()

	// The overhead is the control blocks' share of themselves and the
	// zone's pages, cut (not rounded) to a tenth of a percent.
	control := r.ControlBytes()
	tenths := uint64(control) * 1000 / (uint64(control) + uint64(r.Pages)*vmheap.PageSize)
	z.line("%d bytes are used for zone and area control blocks, or %d.%d%% overhead", control, tenths/10, tenths%10)
	z.blank()

	if detail == 1 {
		return z.lines, false, nil
	}

	// VMS 7.1 shows no more of a zone that has no areas yet.
	if len(r.Areas) == 0 {
		return z.lines, false, nil
	}

	listSummary(z, r)
	areaSummary(z, r)

	if detail == 2 {
		return z.lines, false, nil
	}

	damaged, err := scanLines(env, z, r)
	z.blank()

	return z.lines, damaged, err
}

// algorithmLines are the report's algorithm line, with what a lookaside
// algorithm's lists are.
func algorithmLines(z *zoneReporter, r corevms.ZoneReport) {
	s := r.Settings
	name := "Algorithm = " + zoneAlgorithms[s.Algorithm]

	switch s.Algorithm {
	case vmheap.QuickFit:
		z.line("%s  with %d Lookaside Lists ranging from", name, len(r.Lists))
		z.line("            a minimum blocksize of %d, to a maximum blocksize of %d",
			r.Smallest, r.Smallest+uint32(len(r.Lists)-1)*r.Quantum)

	case vmheap.FrequentSizes:
		z.line("%s  with %d Lookaside Lists", name, len(r.Lists))

	case vmheap.FixedSize:
		z.line("%s  with block size of %d", name, s.FixedSize)

	default:
		z.plain("\t" + name)
	}
}

// listSummary is the lookaside lists' summary, for quick fit and
// frequent sizes: each list that holds blocks, its number (from 1), its
// blocks' size, and how many it holds ("*****" for a list linked to
// itself, whose blocks can't be counted).
func listSummary(z *zoneReporter, r corevms.ZoneReport) {
	title, ok := zoneListTitles[r.Settings.Algorithm]
	if !ok {
		return
	}

	z.plain("\t" + title)
	z.blank()
	z.plain("\t     List        Block   Number of")
	z.plain("\t    number        size     blocks")
	z.plain("\t    ------  ----------  ----------")

	for i, l := range r.Lists {
		switch {
		case l.Loop != 0:
			z.line("%10d  %10d  %10s", i+1, l.Size, "*****")
		case len(l.Blocks) > 0:
			z.line("%10d  %10d  %10d", i+1, l.Size, len(l.Blocks))
		}
	}

	z.blank()
}

// areaSummary is the areas' summary: each area's first and last
// address, its pages, and the bytes of its unallocated end.
func areaSummary(z *zoneReporter, r corevms.ZoneReport) {
	z.plain("\tArea Summary:")
	z.blank()
	z.plain("\t      First     Last       Pages    Bytes not yet")
	z.plain("\t     address   address    assigned    allocated")
	z.plain("\t    --------  --------  ----------  -------------")

	for _, a := range r.Areas {
		z.line("    %08X  %08X  %10d  %13d", a.Base, a.End-1, (a.End-a.Base)/vmheap.PageSize, a.Unallocated())
	}

	z.blank()
}

// scanLines are detail level 3's scans: the lookaside lists (for the
// algorithms that have them), then each area's free list, with the
// number of free blocks and their smallest and largest sizes. A list
// linked to itself is shown as VMS 7.1 shows it, a warning and its
// block; a free block without the zone's free fill as an error and its
// block, the rest of its area's free list not counted. It reports
// whether it found damage.
func scanLines(env *corevms.Environment, z *zoneReporter, r corevms.ZoneReport) (bool, error) {
	damaged := false

	if r.Settings.Algorithm != vmheap.FirstFit {
		z.plain("\tScanning Lookaside Lists in Zone Control Block")

		for _, l := range r.Lists {
			if l.Loop == 0 {
				continue
			}

			damaged = true

			if err := loopedList(env, z, r, l); err != nil {
				return damaged, err
			}
		}

		for _, l := range r.Lists {
			for _, b := range l.Blocks {
				bad, err := env.FreeFillMismatch(r.ID, b)
				if err != nil {
					return damaged, err
				}

				if bad {
					damaged = true

					if err := fillMismatch(env, z, r, b); err != nil {
						return damaged, err
					}
				}
			}
		}
	}

	for _, a := range r.Areas {
		z.line("Scanning Free List for Area at %08X", a.Base)

		bad := false

		for _, b := range a.Free {
			mismatch, err := env.FreeFillMismatch(r.ID, b)
			if err != nil {
				return damaged, err
			}

			if mismatch {
				bad, damaged = true, true

				if err := fillMismatch(env, z, r, b); err != nil {
					return damaged, err
				}

				break
			}
		}

		if bad || len(a.Free) == 0 {
			continue
		}

		smallest, largest := a.Free[0].Size(), a.Free[0].Size()
		for _, s := range a.Free {
			smallest, largest = min(smallest, s.Size()), max(largest, s.Size())
		}

		z.line("  Number of blocks = %d, Min blocksize = %d, Max blocksize = %d", len(a.Free), smallest, largest)
		z.blank()
	}

	return damaged, nil
}

// zoneListHead is where govax says a zone's lookaside lists' heads are
// in its control block, for the link analysis (VMS's are its own).
const zoneListHead = 0x50

// loopedList shows a lookaside list linked to itself as VMS 7.1 does:
// a warning, the link analysis of its block (the list's head before it,
// and itself after it), and the block.
func loopedList(env *corevms.Environment, z *zoneReporter, r corevms.ZoneReport, l vmheap.ListReport) error {
	head, cur := r.ID+zoneListHead, l.Loop

	z.blank()
	z.plain("\t**** WARNING -- forward-link of current block may not be valid ****")
	z.blank()
	linkHeader(z)
	z.line("  Block adr      :  %08X  %08X  %08X", head, cur, cur)
	z.line("  Forw link (rel):  %08X  %08X  %08X", cur-head, 0, 0)
	z.line("  Back link (rel):  %08X  %08X  %08X", cur-head, head-cur, head-cur)
	z.blank()
	z.line("  Forw link (abs):  %08X  %08X  %08X", cur, cur, cur)
	z.line("  Back link (abs):  %08X  %08X  %08X", cur, head, head)
	z.blank()
	z.line("Block size = %d", l.Size)

	return blockDump(env, z, vmheap.Span{Start: cur, End: cur + l.Size})
}

// fillMismatch shows a free block without its zone's free fill as VMS
// 7.1 does: an error, the link analysis of the block (the free list's
// head before and after it), and the block.
func fillMismatch(env *corevms.Environment, z *zoneReporter, r corevms.ZoneReport, b vmheap.Span) error {
	head := r.ID + zoneListHead

	z.blank()
	z.plain("\t**** ERROR -- free-fill mismatch ****")
	z.blank()
	linkHeader(z)
	z.line("  Block adr      :  %08X  %08X  %08X", head, b.Start, head)
	z.blank()
	z.line("  Forw link (abs):  %08X  %08X  %08X", b.Start, head, b.Start)
	z.blank()
	z.line("Block size = %8d", b.Size())

	return blockDump(env, z, b)
}

// linkHeader is a link analysis's title and column headings.
func linkHeader(z *zoneReporter) {
	z.plain("\tLink Analysis for Current Block:")
	z.blank()
	z.plain("\t                    Previous   Current      Next")
	z.plain("\t                    --------  --------  --------")
}

// blockDump is a block's contents as VMS 7.1 shows them: 16 bytes a
// line, as four longwords from the highest address down, the bytes as
// text (a period for one that isn't printable in the DEC Multinational
// set), the line's offset in the block, and its address; then two blank
// lines.
func blockDump(env *corevms.Environment, z *zoneReporter, b vmheap.Span) error {
	mem, cpu := env.Memory(), env.CPU()

	z.blank()
	z.plain("\tBlock contents:")
	z.blank()

	for line := b.Start; line < b.End; line += 16 {
		var longs, text strings.Builder

		n := min(16, b.End-line)

		for i := uint32(0); i < n; i++ {
			c, err := mem.LoadByte(cpu, line+i)
			if err != nil {
				return err
			}

			if c < 0x20 || c > 0x7E && c < 0xA1 || c == 0xFF {
				c = '.'
			}

			text.WriteByte(c)
		}

		for k := 3; k >= 0; k-- {
			at := line + uint32(4*k)
			if at+4 > line+n {
				longs.WriteString("         ")

				continue
			}

			v, err := mem.LoadLongword(cpu, at)
			if err != nil {
				return err
			}

			fmt.Fprintf(&longs, "%08X ", v)
		}

		z.line("  %s%-16s %05X %08X", longs.String(), text.String(), line-b.Start, line)
	}

	z.blank()
	z.blank()

	return nil
}

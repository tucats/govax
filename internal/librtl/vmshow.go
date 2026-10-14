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
// LIB$SHOW_VM_ZONE's report follows the VSI manual's two samples (detail
// levels 1 and 3), which agree with the 5.0 manual's; every line the
// samples don't show, and the samples' own spacing (they're indented
// differently), are govax's choices (unconfirmed: probe cases).

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
// the default zone, the string zone, then the rest in the order they
// were made (user-defined zones too). context is the scan's: 0 starts
// it, and it then holds the number of the zone last found (govax's form;
// the manual leaves it the RTL's). A context of 0 verifies every zone
// first (LIB$VERIFY_VM_ZONE), and a damaged one is LIB$_BADZONE, nothing
// written. After the last zone it's LIB$_NOTFOU (an alternate success),
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

	id, seq, ok, err := env.NextZone(context)

	switch {
	case err != nil:
		return zoneFailure(env, err)
	case !ok:
		return libNotFou, nil
	}

	if err := mem.StoreLongword(cpu, arg(argv, 1), id); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, arg(argv, 0), seq); err != nil {
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
// It describes the zone zone-id names (0 or omitted, the default zone)
// at detail-level 0 to 3 (0 when omitted; another, LIB$_INVARG): the
// lines of zoneReport. Each line goes to SYS$OUTPUT as LIB$PUT_OUTPUT
// writes it, or, with an action routine, to the routine, called with a
// class S descriptor of the line and user-arg (0 when omitted); a
// failure status from either ends the report and is the routine's
// ("success codes are ignored"). An ID that names no zone is
// LIB$_BADZONE; no arguments, or more than four, LIB$_WRONUMARG.
//
// The action routine is called back once a line, so a call with one
// runs on a goroutine of its own (corevms.Environment.RunCallingBack),
// having made the report and changed nothing.
func libShowVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 || len(argv) > 4 {
		return libWroNumArg, nil
	}

	id, err := optionalLongword(env, arg(argv, 0), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	detail, err := optionalLongword(env, arg(argv, 1), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(detail) < 0 || detail > 3 {
		return libInvArg, nil
	}

	r, err := env.ZoneReportOf(id)
	if err != nil {
		return zoneFailure(env, err)
	}

	action := arg(argv, 2)
	if action != 0 && !env.CallingBack() {
		return env.RunCallingBack("LIB$SHOW_VM_ZONE", libShowVMZone, argv)
	}

	lines, err := zoneReport(env, r, int(detail))
	if err != nil {
		return memoryFault(env, err)
	}

	for _, line := range lines {
		status, err := showZoneLine(env, action, arg(argv, 3), line)
		if err != nil || status&1 == 0 {
			return status, err
		}
	}

	return ssNormal, nil
}

// showZoneLine writes one of LIB$SHOW_VM_ZONE's lines: to SYS$OUTPUT, or
// to the action routine (with value, the user argument), its descriptor
// below SP for the call.
func showZoneLine(env *corevms.Environment, action, value uint32, line string) (uint32, error) {
	if action == 0 {
		return env.PutOutput(line)
	}

	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)

	defer cpu.SetGPR(registers.SP, sp)

	desc, err := stackString(env, line)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	status, err := env.Callback(action, desc, value)
	if err != nil {
		return memoryFault(env, err)
	}

	return status, nil
}

// The report's indentations: the lines after the first, and a flag's
// name under the flags.
const (
	zoneIndent = "    "
	flagIndent = zoneIndent + "             "
)

// zoneAlgorithms are the algorithms' names, by vmheap's Algorithm.
var zoneAlgorithms = []string{
	vmheap.FirstFit:      "LIB$K_VM_FIRST_FIT",
	vmheap.QuickFit:      "LIB$K_VM_QUICK_FIT",
	vmheap.FrequentSizes: "LIB$K_VM_FREQ_SIZES",
	vmheap.FixedSize:     "LIB$K_VM_FIXED",
}

// zoneListTitles are the lookaside list summaries' titles, by
// algorithm (Quick Fit's is the manual's).
var zoneListTitles = []string{
	vmheap.QuickFit:      "Quick Fit Lookaside List Summary:",
	vmheap.FrequentSizes: "Frequent Sizes Lookaside List Summary:",
	vmheap.FixedSize:     "Fixed Size Lookaside List Summary:",
}

// zoneFlag is one of the flags LIB$CREATE_VM_ZONE takes, its name, and
// whether a zone's settings have it.
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
	{"LIB$M_VM_NO_EXTEND", vmNoExtend, func(s corevms.ZoneSettings) bool { return s.NoExtend }},
	{"LIB$M_VM_TAIL_LARGE", vmTailLarge, func(s corevms.ZoneSettings) bool { return s.TailLarge }},
}

// zoneReport returns LIB$SHOW_VM_ZONE's lines for the zone r describes
// at detail level detail:
//
//   - 0: the zone's ID and name (a user-defined zone shows only this, at
//     every level; unconfirmed);
//   - 1: its algorithm, flags, and sizes, the bytes freed and not yet
//     allocated again, and the bytes of its control blocks;
//   - 2: a lookaside list's summary (the lists that hold blocks), and the
//     area summary;
//   - 3: the scan of the lookaside lists and of each area's free list:
//     with a free fill, each free block that doesn't hold it is shown as
//     an error, with its contents.
//
// The error is memory that can't be read.
func zoneReport(env *corevms.Environment, r corevms.ZoneReport, detail int) ([]string, error) {
	lines := []string{fmt.Sprintf("Zone Id = %08X,  Zone name = %q", r.ID, r.Settings.Name)}
	if detail == 0 || r.User != nil {
		return lines, nil
	}

	add := func(format string, args ...any) {
		lines = append(lines, zoneIndent+fmt.Sprintf(format, args...))
	}

	s := r.Settings
	lines = append(lines, algorithmLines(r)...)

	var flags uint32

	for _, f := range zoneFlags {
		if f.set(s) {
			flags |= f.bit
		}
	}

	add("Flags = %08X", flags)

	for _, f := range zoneFlags {
		if f.set(s) {
			lines = append(lines, flagIndent+f.name)
		}
	}

	areas := "areas"
	if len(r.Areas) == 1 {
		areas = "area"
	}

	limit := "None"
	if s.PageLimit > 0 {
		limit = fmt.Sprintf("%d pages", s.PageLimit)
	}

	add("Initial size = %7d pages     Current size = %d pages in %d %s", s.InitialPages, r.Pages, len(r.Areas), areas)
	add("Extend size  = %7d pages     Page limit   = %s", s.ExtendPages, limit)
	add("Requests are rounded up to a multiple of %d bytes,", r.Quantum)

	if r.Alignment == r.Quantum {
		add("naturally aligned on %d byte boundaries", r.Alignment)
	} else {
		add("aligned on %d byte boundaries", r.Alignment)
	}

	add("%d bytes have been freed and not yet reallocated", r.FreeBytes())

	control := r.ControlBytes()
	add("%d bytes are used for zone and area control blocks, or %.1f%% overhead",
		control, 100*float64(control)/float64(uint64(control)+uint64(r.Pages)*vmheap.PageSize))

	if detail == 1 {
		return lines, nil
	}

	lines = append(lines, listSummary(r)...)
	lines = append(lines, areaSummary(r)...)

	if detail == 2 {
		return lines, nil
	}

	scan, err := scanLines(env, r)

	return append(lines, scan...), err
}

// algorithmLines are the report's algorithm line, with what a lookaside
// algorithm's lists are (Quick Fit's wording is the manual's, the
// others' govax's).
func algorithmLines(r corevms.ZoneReport) []string {
	s := r.Settings
	name := zoneIndent + "Algorithm = " + zoneAlgorithms[s.Algorithm]

	switch s.Algorithm {
	case vmheap.QuickFit:
		smallest, largest := r.Lists[0].Size, r.Lists[len(r.Lists)-1].Size

		return []string{
			fmt.Sprintf("%s  with %d Lookaside Lists ranging from", name, len(r.Lists)),
			fmt.Sprintf("%s            a minimum blocksize of %d, to a maximum blocksize of %d", zoneIndent, smallest, largest),
		}

	case vmheap.FrequentSizes:
		return []string{fmt.Sprintf("%s  with %d Lookaside Lists", name, len(r.Lists))}

	case vmheap.FixedSize:
		return []string{fmt.Sprintf("%s  with a blocksize of %d", name, s.FixedSize)}
	}

	return []string{name}
}

// listSummary is the lookaside lists' summary, for the algorithms that
// have them: each list that holds blocks, its number (from 1), its
// blocks' size, and how many it holds. The manual's samples leave out
// the first list, which can have held no blocks, so a list without any
// isn't shown (unconfirmed).
func listSummary(r corevms.ZoneReport) []string {
	if r.Settings.Algorithm == vmheap.FirstFit {
		return nil
	}

	lines := []string{
		zoneIndent + zoneListTitles[r.Settings.Algorithm],
		"         List        Block   Number of",
		"        number        size     blocks",
		"        ------  ----------  ----------",
	}

	for i, l := range r.Lists {
		if len(l.Blocks) > 0 {
			lines = append(lines, fmt.Sprintf("        %6d  %10d  %10d", i+1, l.Size, len(l.Blocks)))
		}
	}

	return lines
}

// areaSummary is the areas' summary: each area's first and last
// address, its pages, and the bytes of its unallocated end.
func areaSummary(r corevms.ZoneReport) []string {
	lines := []string{
		zoneIndent + "Area Summary:",
		"          First     Last       Pages    Bytes not yet",
		"         address   address    assigned    allocated",
		"        --------  --------  ----------  -------------",
	}

	for _, a := range r.Areas {
		lines = append(lines, fmt.Sprintf("        %08X  %08X  %10d  %13d",
			a.Base, a.End-1, (a.End-a.Base)/vmheap.PageSize, a.Unallocated()))
	}

	return lines
}

// scanLines are detail level 3's scans: the lookaside lists (for the
// algorithms that have them), then each area's free list, with the
// number of free blocks and their smallest and largest sizes. With a
// free fill, a free block that doesn't hold it is shown, before the
// counts, as an error and a dump of its contents.
func scanLines(env *corevms.Environment, r corevms.ZoneReport) ([]string, error) {
	var lines []string

	fill := r.Settings.FreeFill

	check := func(blocks []vmheap.Span) error {
		for _, b := range blocks {
			bad, err := env.FillMismatch(b, fill)
			if err != nil {
				return err
			}

			if bad {
				dump, err := blockDump(env, b)
				if err != nil {
					return err
				}

				lines = append(lines, zoneIndent+"**** ERROR -- free-fill mismatch ****")
				lines = append(lines, dump...)
			}
		}

		return nil
	}

	if r.Settings.Algorithm != vmheap.FirstFit {
		lines = append(lines, zoneIndent+"Scanning Lookaside Lists in Zone Control Block")

		for _, l := range r.Lists {
			if err := check(l.Blocks); err != nil {
				return lines, err
			}
		}
	}

	for _, a := range r.Areas {
		lines = append(lines, fmt.Sprintf("%sScanning Free List for Area at %08X", zoneIndent, a.Base))

		if err := check(a.Free); err != nil {
			return lines, err
		}

		if len(a.Free) == 0 {
			lines = append(lines, zoneIndent+"  Number of blocks = 0")

			continue
		}

		smallest, largest := a.Free[0].Size(), a.Free[0].Size()
		for _, s := range a.Free {
			smallest, largest = min(smallest, s.Size()), max(largest, s.Size())
		}

		lines = append(lines, fmt.Sprintf("%s  Number of blocks = %d, Min blocksize = %d, Max blocksize = %d",
			zoneIndent, len(a.Free), smallest, largest))
	}

	return lines, nil
}

// blockDump is a free block's size and contents as the manual's sample
// error display shows them: 16 bytes a line, as four longwords from the
// highest address down, the bytes as text (a period for one that isn't
// printable), the line's offset in the block, and its address. A last
// line of fewer longwords is padded on the left.
func blockDump(env *corevms.Environment, b vmheap.Span) ([]string, error) {
	mem, cpu := env.Memory(), env.CPU()
	lines := []string{
		fmt.Sprintf("%sBlock size = %d", zoneIndent, b.Size()),
		zoneIndent + "Block contents:",
	}

	for line := b.Start; line < b.End; line += 16 {
		var longs, text strings.Builder

		n := min(16, b.End-line)

		for i := uint32(0); i < n; i++ {
			c, err := mem.LoadByte(cpu, line+i)
			if err != nil {
				return lines, err
			}

			if c < 0x20 || c > 0x7E {
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
				return lines, err
			}

			fmt.Fprintf(&longs, "%08X ", v)
		}

		lines = append(lines, fmt.Sprintf("%s  %s %-16s %05X %08X", zoneIndent, longs.String(), text.String(), line-b.Start, line))
	}

	return lines, nil
}

package dbgsym

// TraceName is how the traceback facility's symbolic stack dump names a
// PC (docs/PHASE-70 - traceback.md, Part A, "Naming a PC"): the module
// and routine columns, the PC relative to what names it, and the
// listing line. An empty Routine with a Module is a psect or a routine
// with no name; both empty, the PC is in nothing the image describes,
// and Rel is the PC itself. Line is 0 when the PC has none.
type TraceName struct {
	Module  string
	Routine string
	Rel     uint32
	Line    int
}

// tracebackRoutineLimit is how far past a routine's start a PC can be
// and still be named by it, whatever the routine's size: VMS 7.1's
// traceback named routine+3FF by the routine and routine+400 by its
// psect (testdata/tb70's round 2).
const tracebackRoutineLimit = 0x400

// TraceName names pc as the traceback facility does. exact says pc is
// where a fault happened, so its line is pc's own; any other PC (a
// trap's, a call's return address) is past the instruction it belongs
// to, so its line is the one holding the byte before it.
//
// The rules, from VMS's output:
//
//  1. The routine, of any module, with the greatest address at or below
//     pc names it if pc less that address, compared as a signed number,
//     is below 400 (hex). The comparison being signed, a PC far above
//     the image (in S0, say) is named by the image's last routine, with
//     a "negative" Rel, as VMS names one.
//  2. Otherwise the psect contribution holding pc names it: the psect's
//     name in the routine column, Rel from the contribution's start.
//  3. Otherwise nothing does.
//
// The line comes from the module's line-number table (an image linked
// /DEBUG has one) whichever rule named the PC.
func (p *Program) TraceName(pc uint32, exact bool) TraceName {
	name := TraceName{Rel: pc}

	if p == nil {
		return name
	}

	lineAt := pc
	if !exact {
		lineAt--
	}

	if l, _, ok := p.LineAt(lineAt); ok {
		name.Line = l.Line
	}

	// Rule 1: the routine at or below pc nearest to it, in any module.
	var (
		routine *Routine
		module  *Module
	)

	for _, m := range p.Modules {
		for _, r := range m.Routines {
			if r.Address <= pc && (routine == nil || r.Address > routine.Address) {
				routine, module = r, m
			}
		}
	}

	if routine != nil && int32(pc-routine.Address) < tracebackRoutineLimit {
		name.Module, name.Routine, name.Rel = module.Name, routine.Name, pc-routine.Address

		return name
	}

	// Rule 2: the psect contribution holding pc.
	for _, m := range p.Modules {
		for _, ps := range m.Psects {
			if pc >= ps.Address && pc < ps.End() {
				name.Module, name.Routine, name.Rel = m.Name, ps.Name, pc-ps.Address

				return name
			}
		}
	}

	return name
}

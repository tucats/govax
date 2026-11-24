package link

// LIB$INITIALIZE (docs/planning/PHASE-113 - Fortran errata.md, subtask 6). A
// program asks for routines to run before its main program by putting their
// addresses in the program section LIB$INITIALIZE and referring to the symbol
// LIB$INITIALIZE, which a library module defines (VMS's in STARLET.OLB, govax's
// own otherwise: internal/bootdata's libinit.mar, by the Linker manual's
// account and VAX/VMS Internals and Data Structures' 21.1.3). LINK then puts
// LIB$INITIALIZE's address in the image's transfer address array, after
// SYS$IMGSTA's (or the debugger's) and before the program's own (the book's
// Figure 21-8: "Nonzero Contribution to PSECT LIB$INITIALIZE"), so that it is
// called first and calls each routine in turn.

// initializeName is both the program section that lists initialization
// routines and the routine that calls them.
const initializeName = "LIB$INITIALIZE"

// initializeTransfer is the transfer address LIB$INITIALIZE adds to the image's
// array, and true: the routine's address, when the image has a contribution of
// some length to the program section LIB$INITIALIZE and the symbol is defined.
// A shareable image's too, whose array has no program's address after it
// (whether VMS's image activator calls it is the probe round's question).
func (l *linker) initializeTransfer() (uint32, bool) {
	g := l.symbols[initializeName]
	if g == nil || !g.defined {
		return 0, false
	}

	for _, p := range l.order {
		if p.name == initializeName && p.length > 0 {
			return g.value, true
		}
	}

	return 0, false
}

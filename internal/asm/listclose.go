package asm

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file lays out a MACRO listing's closing pages, which follow the
// source pages (listpage.go), as real VAX MACRO lays them out
// (docs/PHASE-29.md, subtask 4):
//
//   - the symbol table, on a page of its own: each symbol the module
//     defined or referred to, in name order, with its value and
//     attributes;
//   - the psect synopsis: each psect's size, number, and attributes;
//   - the performance indicators: the time each phase of the assembly
//     took, and how many source lines and object records there were;
//   - the macro library statistics: how many macros each library
//     defined;
//   - the summary of errors and warnings, and the command line.
//
// The closing pages go on from page to page as the source pages do. A
// page's second heading line names the section the page starts in:
// "Symbol table", "Psect synopsis", or, from the performance indicators
// on, "VAX-11 Macro Run Statistics".
//
// Real MACRO's statistics also measure its own memory use: the page
// faults in each phase, its working set limit, the virtual memory its
// intermediate code, symbol table, and macros took. govax has no such
// figures, and leaves them out rather than showing a made-up value
// (the phase's Decision 2).

// The second heading line's name for each part of the closing pages.
const (
	labelSymbols    = "Symbol table"
	labelSynopsis   = "Psect synopsis"
	labelStatistics = "VAX-11 Macro Run Statistics"
)

// PhaseTime is how long a phase of an assembly took: the process's CPU
// time and the elapsed (wall clock) time.
type PhaseTime struct {
	CPU, Elapsed time.Duration
}

// add returns the sum of t and u.
func (t PhaseTime) add(u PhaseTime) PhaseTime {
	return PhaseTime{CPU: t.CPU + u.CPU, Elapsed: t.Elapsed + u.Elapsed}
}

// PhaseStart is the moment a phase began, for measuring it.
type PhaseStart struct {
	cpu  time.Duration
	wall time.Time
}

// StartPhase marks the start of a phase.
func StartPhase() PhaseStart {
	return PhaseStart{cpu: processCPU(), wall: time.Now()}
}

// Elapsed returns the time since s.
func (s PhaseStart) Elapsed() PhaseTime {
	return PhaseTime{CPU: processCPU() - s.cpu, Elapsed: time.Since(s.wall)}
}

// The phases of an assembly the performance indicators list, in their
// order. The assembler measures all but the first two, which happen
// before it's called (see ListingOptions).
const (
	phaseInitialization = iota
	phaseCommand
	phasePass1
	phaseSymbolSort
	phasePass2
	phaseSymbolOutput
	phaseSynopsisOutput
	phaseCrossReference
	phaseCount
)

// phaseNames are the phases' names in the performance indicators.
var phaseNames = [phaseCount]string{
	"Initialization",
	"Command processing",
	"Pass 1",
	"Symbol table sort",
	"Pass 2",
	"Symbol table output",
	"Psect synopsis output",
	"Cross-reference output",
}

// closingPages appends the closing pages to p.
func (a *Assembler) closingPages(p *listPager, opts ListingOptions) {
	a.phases[phaseInitialization] = opts.Initialization
	a.phases[phaseCommand] = opts.CommandProcessing

	// The symbol table always starts a page of its own.
	p.label = labelSymbols
	p.breakPage()

	phase := StartPhase()
	symbols, width := a.listedSymbols()
	a.phases[phaseSymbolSort] = phase.Elapsed()

	phase = StartPhase()

	for _, s := range symbols {
		p.add(symbolTableLine(s, width))
	}

	a.phases[phaseSymbolOutput] = phase.Elapsed()

	p.label = labelSynopsis
	phase = StartPhase()
	a.psectSynopsis(p)
	a.phases[phaseSynopsisOutput] = phase.Elapsed()

	p.label = labelStatistics
	a.performance(p)
	a.libraryStatistics(p)
	a.summary(p)

	p.add("")
	p.add(opts.Command)
}

// listedSymbols returns the symbols the symbol table lists, in name
// order (ASCII order, so "$" and "." sort before letters), and the width
// of the table's name column.
//
// Every symbol the module defined or referred to is listed but a local
// label ("10$") and a symbol defined under .ENABLE SUPPRESSION that
// nothing referred to, which is how real MACRO leaves most of a $xxxDEF
// macro's symbols out of its table. The name column is 15 wide, or 31
// (the longest a symbol can be) if any symbol's name is longer than 15,
// listed or not: real MACRO's tables for programs that use $FABDEF are
// 31 wide though no symbol they list is that long.
func (a *Assembler) listedSymbols() ([]*symbol, int) {
	width := 15

	var out []*symbol

	for name, s := range a.symbols.byName {
		if s.flags&(SymBuiltin|SymLocalLabel) != 0 {
			continue
		}

		if len(name) > width {
			width = 31
		}

		if s.suppressed && !s.referenced {
			continue
		}

		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })

	return out, width
}

// symbolTableLine returns s's line in the symbol table, its name in a column
// width wide:
//
//	NAME             00000000 RG    02
//	LIMIT          = 00000064  G
//	MAYBE            ********W GX   00
//
// After the name comes "=" for a symbol given its value by a direct
// assignment, then the value ("********" if it's undefined, which in a
// finished assembly means external), then four flags: W (weak), R
// (relocatable), G (global), and X (external, or undefined). Last is
// the psect number, in hex: the psect a relocatable symbol is in, or for
// an external one, the psect that was current where the module first
// named it. An absolute symbol has none, even a label in an absolute
// psect.
func symbolTableLine(s *symbol, width int) string {
	defined := s.defined()

	assign := ' '
	if defined && s.flags&(SymLabel|SymEntry) == 0 {
		assign = '='
	}

	value := "********"
	if defined {
		value = fmt.Sprintf("%08X", s.value)
	}

	flags := []byte("    ")

	if s.flags&SymWeak != 0 {
		flags[0] = 'W'
	}

	if s.sect != nil && defined {
		flags[1] = 'R'
	}

	// .EXTERNAL makes a symbol global in the object, as .GLOBAL does,
	// but real MACRO's table shows G for an external symbol only if
	// .GLOBAL (or .WEAK) named it.
	if s.flags&SymGlobal != 0 && (defined || s.flags&SymExtern == 0) {
		flags[2] = 'G'
	}

	psect := "  "

	switch {
	case !defined:
		flags[3] = 'X'

		if s.firstSect != nil {
			psect = fmt.Sprintf("%02X", s.firstSect.index)
		} else {
			psect = "00"
		}

	case s.sect != nil:
		psect = fmt.Sprintf("%02X", s.sect.index)
	}

	// The line ends in blanks: six after a 15-column name, eight after a
	// 31-column one, as real MACRO's do.
	trail := 6
	if width > 15 {
		trail = 8
	}

	return fmt.Sprintf("%-*s%c %s%s   %s%s", width, s.name, assign, value, flags, psect, strings.Repeat(" ", trail))
}

// boxed returns a section's boxed title, indented by indent columns:
//
//	+----------------+
//	! Psect synopsis !
//	+----------------+
func boxed(title string, indent int) []string {
	pad := strings.Repeat(" ", indent)
	rule := pad + "+" + strings.Repeat("-", len(title)+2) + "+"

	return []string{rule, pad + "! " + title + " !", rule}
}

// addSection appends a closing section's opening to p: a blank line, its
// boxed title, and another blank line.
func addSection(p *listPager, title string, indent int) {
	p.add("")

	for _, line := range boxed(title, indent) {
		p.add(line)
	}

	p.add("")
}

// psectSynopsis appends the psect synopsis: each psect, in psect number
// order, with its allocation (its size, in hex and decimal), its number
// (in hex and decimal), and its attributes.
func (a *Assembler) psectSynopsis(p *listPager) {
	addSection(p, "Psect synopsis", 48)
	p.add("PSECT name                      Allocation          PSECT No.  Attributes     ")
	p.add("----------                      ----------          ---------  ----------     ")

	for _, s := range a.sections {
		p.add(fmt.Sprintf("%-32s%08X  (%5d.)  %02X (%3d.)  %s",
			s.name, s.hi, s.hi, s.index, s.index, psectAttributeText(s)))
	}
}

// psectAttributeText returns a psect's attributes as the synopsis shows
// them, each in a column five wide and followed by a blank:
//
//	NOPIC   USR   CON   REL   LCL NOSHR   EXE   RD  NOWRT NOVEC LONG
func psectAttributeText(s *section) string {
	columns := []struct {
		flag     uint32
		set, not string
	}{
		{gpsPIC, "PIC", "NOPIC"},
		{gpsLIB, "LIB", "USR"},
		{gpsOVR, "OVR", "CON"},
		{gpsREL, "REL", "ABS"},
		{gpsGBL, "GBL", "LCL"},
		{gpsSHR, "SHR", "NOSHR"},
		{gpsEXE, "EXE", "NOEXE"},
		{gpsRD, "RD ", "NORD "},
		{gpsWRT, "WRT", "NOWRT"},
		{gpsVEC, "VEC", "NOVEC"},
	}

	var b strings.Builder

	for _, c := range columns {
		text := c.not
		if s.flags&c.flag != 0 {
			text = c.set
		}

		fmt.Fprintf(&b, "%5s ", text)
	}

	fmt.Fprintf(&b, "%-5s ", alignmentName(s.align))

	return b.String()
}

// alignmentName returns the name of an alignment, a power of two, as the
// synopsis shows it. No real listing yet shows an alignment without a
// keyword (.PSECT's numeric form); govax shows its number.
func alignmentName(align uint32) string {
	for name, n := range alignKeywords {
		if n == align {
			return name
		}
	}

	return fmt.Sprint(align)
}

// performance appends the performance indicators: the time each phase of
// the assembly took, and the counts of source lines and object records.
// Real MACRO's page-fault column, and its lines about its own memory
// (its working set limit, and the memory its intermediate code, symbol
// table, and macros used) are left out.
func (a *Assembler) performance(p *listPager) {
	addSection(p, "Performance indicators", 45)
	p.add("Phase                    CPU Time       Elapsed Time   ")
	p.add("-----                    --------       ------------   ")

	var total PhaseTime

	for i, name := range phaseNames {
		p.add(phaseLine(name, a.phases[i]))
		total = total.add(a.phases[i])
	}

	p.add(phaseLine("Assembler run totals", total))
	p.add("")

	lines := 0

	for _, l := range a.listLines {
		if l.depth == 0 {
			lines++
		}
	}

	p.add(fmt.Sprintf("%d source lines were read in Pass 1, producing %d object records in Pass 2.", lines, a.objectRecords))
}

// phaseLine returns a phase's line in the performance indicators.
func phaseLine(name string, t PhaseTime) string {
	return fmt.Sprintf("%-25s%s    %s", name, durationText(t.CPU), durationText(t.Elapsed))
}

// durationText returns d as hh:mm:ss.cc, as VMS shows a time interval.
func durationText(d time.Duration) string {
	cs := d.Milliseconds() / 10

	return fmt.Sprintf("%02d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

// libraryStatistics appends the macro library statistics: each library
// searched, in search order, with the number of macros the assembly
// defined from it (and their total, with more than one library), and the
// number of records (GETs) read to define them.
func (a *Assembler) libraryStatistics(p *listPager) {
	addSection(p, "Macro library statistics", 44)
	p.add("Macro library name                           Macros defined      ")
	p.add("------------------                           --------------      ")

	libs := a.searchOrder()
	macros, gets := 0, 0

	for _, use := range libs {
		p.add(libraryLine(use.name(), use.macros))
		macros += use.macros
		gets += use.gets
	}

	if len(libs) > 1 {
		p.add(libraryLine("TOTALS (all libraries)", macros))
	}

	p.add("")

	// No real listing yet shows one macro defined; govax writes it
	// singular, as real MACRO's "1 page of virtual memory was used to
	// define 1 macro." is.
	noun := "macros"
	if macros == 1 {
		noun = "macro"
	}

	p.add(fmt.Sprintf("%d GETS were required to define %d %s.", gets, macros, noun))
}

// libraryLine returns a line of the macro library statistics.
func libraryLine(name string, macros int) string {
	return fmt.Sprintf("%-45s%12d        ", name, macros)
}

// summary appends the count of errors and warnings, with the lines they
// were on, five to a row, each with its file number.
func (a *Assembler) summary(p *listPager) {
	p.add("")

	errs, warnings := len(a.errs), len(a.warnings)
	if errs == 0 && warnings == 0 {
		p.add("There were no errors, warnings or information messages.")

		return
	}

	p.add(fmt.Sprintf("There were %d errors, %d warnings and 0 information messages, on lines:", errs, warnings))

	var (
		row   strings.Builder
		count int
		last  int
		line  int
	)

	for _, l := range a.listLines {
		// A message from a macro expansion or repeat block is counted
		// on the program's line that called it.
		if l.depth == 0 {
			line = l.line
		}

		if len(l.errs) == 0 && len(l.warnings) == 0 || line == last {
			continue
		}

		last = line

		fmt.Fprintf(&row, "%5d (1)     ", line)

		if count++; count%5 == 0 {
			p.add(row.String())
			row.Reset()
		}
	}

	if row.Len() > 0 {
		p.add(row.String())
	}
}

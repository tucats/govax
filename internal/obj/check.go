package obj

import (
	"fmt"
	"regexp"
)

// Problem is one thing Check found wrong with a module. Record is the
// 1-based record number, or 0 for a problem with the module as a whole.
type Problem struct {
	Record  int
	Message string
}

func (p Problem) String() string {
	if p.Record == 0 {
		return p.Message
	}

	return fmt.Sprintf("record %d: %s", p.Record, p.Message)
}

// linkerStackDepth is the least stack space the linker promises TIR
// commands, in longwords (section 7.4).
const linkerStackDepth = 25

// createdRE matches the MHD creation time's fixed "dd-mmm-yyyy hh:mm" form.
var createdRE = regexp.MustCompile(`^[ 0-9][0-9]-[A-Z]{3}-[0-9]{4} [0-9]{2}:[0-9]{2}$`)

// Check validates a module's structure against the rules of the VAX object
// language (chapter 7 of the VMS 5.0 Linker Utility Manual) and the checks
// ANALYZE/OBJECT makes, returning every problem it finds. It checks:
//
//   - record order: MHD first, LNM second, the other headers next, and
//     exactly one EOM or EOMW, last (GSD and TIR records may interleave,
//     as real MACRO writes them);
//   - record sizes against the MHD maximum and OBJ$C_MAXRECSIZ;
//   - the MHD's structure level, name, version, and creation time;
//   - names (1 to 31 characters) and psect alignments;
//   - psect indexes, in symbol definitions, TIR commands, and the EOM
//     transfer address, against the psects the GSD defines;
//   - the linker's stack: no command pops an empty stack, the depth stays
//     within the linker's guaranteed 25 longwords, and the stack is empty
//     at the end of the module;
//   - that each global a TIR command names is in the GSD;
//   - the EOM severity is not a reserved value.
func Check(m *Module) []Problem {
	c := checker{m: m, globals: map[string]bool{}}
	c.run()

	return c.problems
}

type checker struct {
	m        *Module
	problems []Problem

	rec      int // the current record, 1-based
	psects   int
	globals  map[string]bool
	maxSize  int
	depth    int
	seenText bool
	seenEOM  bool
}

func (c *checker) report(format string, args ...any) {
	c.problems = append(c.problems, Problem{Record: c.rec, Message: fmt.Sprintf(format, args...)})
}

func (c *checker) name(what, name string) {
	if len(name) == 0 || len(name) > MaxNameLength {
		c.report("%s name %q must be 1 to %d characters", what, name, MaxNameLength)
	}
}

func (c *checker) psect(what string, index uint16) {
	if int(index) >= c.psects {
		c.report("%s refers to psect %d, but only %d psects are defined", what, index, c.psects)
	}
}

func (c *checker) run() {
	recs := c.m.Records
	if len(recs) < 3 {
		c.report("a module needs at least a main header, a language header, and an end of module record")
	}

	c.maxSize = MaxRecordSize

	// Psects are numbered across the whole module in the order their GSD
	// subrecords appear, and a symbol or TIR command may refer to one
	// defined later in the GSD, so count them all first.
	for _, rec := range recs {
		if g, ok := rec.(*GSD); ok {
			for _, s := range g.Subrecords {
				if _, ok := s.(*Psect); ok {
					c.psects++
				}
			}
		}
	}

	seenGSD := false

	for i, rec := range recs {
		c.rec = i + 1

		if c.seenEOM {
			c.report("%s record after the end of module record", rec.RecordType())
		}

		if b, err := EncodeRecord(rec); err != nil {
			c.report("%v", err)
		} else if len(b) > c.maxSize {
			c.report("record is %d bytes, more than the maximum of %d", len(b), c.maxSize)
		}

		switch rec := rec.(type) {
		case *MainHeader:
			if i != 0 {
				c.report("main header record is not the first record")
			}

			c.mainHeader(rec)

		case *TextHeader:
			switch {
			case i == 0:
				c.report("the first record is %s, not the main header", rec.Type)
			case i == 1 && rec.Type != HdrLNM:
				c.report("the second record is %s, not the language processor header", rec.Type)
			case rec.Type == HdrLNM && i != 1:
				c.report("language processor header is not the second record")
			}

			if seenGSD || c.seenText {
				c.report("header record after the global symbol directory or text records")
			}

		case *GSD:
			// GSD and TIR records may interleave: real MACRO defines each
			// psect just before its first text, and each symbol near its
			// definition (see docs/PHASE-27.md).
			seenGSD = true
			c.gsd(rec)

		case *TIR:
			c.seenText = true
			c.tir(rec)

		case *EOM:
			c.seenEOM = true
			c.eom(rec, i == len(recs)-1)

		case *LNK:
			c.name("link option file", rec.Name)

		case *Unknown:
			c.report("unknown record type %d", byte(rec.Type))
		}

		if i < 2 && !isHeader(rec) {
			c.report("%s record where a header record belongs", rec.RecordType())
		}
	}

	c.rec = 0

	if !seenGSD {
		c.report("module has no global symbol directory record")
	}

	if !c.seenEOM {
		c.report("module has no end of module record")
	}
}

func isHeader(rec Record) bool { return rec.RecordType() == RecHDR }

func (c *checker) mainHeader(h *MainHeader) {
	if h.StructureLevel != StructureLevel {
		c.report("structure level is %d, not %d", h.StructureLevel, StructureLevel)
	}

	if int(h.MaxRecordSize) > MaxRecordSize || h.MaxRecordSize == 0 {
		c.report("maximum record size %d must be 1 to %d", h.MaxRecordSize, MaxRecordSize)
	} else {
		c.maxSize = int(h.MaxRecordSize)
	}

	c.name("module", h.Name)

	if len(h.Version) == 0 || len(h.Version) > MaxNameLength {
		c.report("module version %q must be 1 to %d characters", h.Version, MaxNameLength)
	}

	if !createdRE.MatchString(h.Created) {
		c.report("creation time %q is not in the form dd-mmm-yyyy hh:mm", h.Created)
	}
}

func (c *checker) gsd(g *GSD) {
	for _, s := range g.Subrecords {
		switch s := s.(type) {
		case *Psect:
			c.name("psect", s.Name)

			if s.Align > MaxPsectAlignment {
				c.report("psect %s alignment 2**%d is more than page alignment", s.Name, s.Align)
			}

		case *Symbol:
			c.name("symbol", s.Name)
			c.globals[s.Name] = true

			if s.Defined() && s.Flags&SymREL != 0 {
				c.psect("symbol "+s.Name, s.Psect)
			}

			if s.Formals != nil && s.Formals.Min > s.Formals.Max {
				c.report("procedure %s: minimum argument count %d is more than the maximum %d", s.Name, s.Formals.Min, s.Formals.Max)
			}

		case *IdentCheck:
			c.name("ident check entity", s.Name)

		case *Environment:
			c.name("environment", s.Name)
		}
	}
}

func (c *checker) tir(t *TIR) {
	for _, cmd := range t.Commands {
		switch ops[cmd.Op].format {
		case opPsectB, opPsectW, opPsectL, opWPsectB, opWPsectW, opWPsectL:
			c.psect(cmd.Op.String(), cmd.Psect)
		}

		if (cmd.Op == OpStackGlobal || cmd.Op == opsByName["STA_EPM"]) && !c.globals[cmd.Name] {
			c.report("%s names %s, which the global symbol directory doesn't", cmd.Op, cmd.Name)
		}

		if cmd.Op == OpStoreImmediate {
			continue
		}

		pop, push := cmd.Op.StackEffect()
		if c.depth < pop {
			c.report("%s needs %d stack longwords, but the stack has %d", cmd.Op, pop, c.depth)
			c.depth = 0
		} else {
			c.depth -= pop
		}

		c.depth += push
		if c.depth > linkerStackDepth {
			c.report("%s leaves %d longwords on the stack, more than the linker's guaranteed %d", cmd.Op, c.depth, linkerStackDepth)
		}
	}
}

func (c *checker) eom(e *EOM, last bool) {
	if !last {
		c.report("end of module record is not the last record")
	}

	if e.Severity > SeverityAbort && e.Severity <= 10 {
		c.report("severity %d is reserved", e.Severity)
	}

	if e.HasTransfer {
		c.psect("transfer address", e.Psect)
	}

	if e.HasFlags && e.Flags&^EOMWeakTransfer != 0 {
		c.report("reserved transfer flag bits %#x are set", e.Flags&^EOMWeakTransfer)
	}

	if c.depth != 0 {
		c.report("the linker's stack holds %d longwords at the end of the module", c.depth)
	}
}

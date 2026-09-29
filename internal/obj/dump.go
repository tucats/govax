package obj

import (
	"fmt"
	"io"
	"strings"
)

// Dump writes a readable description of every record in the module, in
// the spirit of ANALYZE/OBJECT: one heading line per record, then one
// indented line per GSD subrecord or TIR command. Psects are numbered as
// the linker numbers them, in order of definition.
func Dump(w io.Writer, m *Module) error {
	d := dumper{w: w}

	for i, rec := range m.Records {
		d.record(i+1, rec)
	}

	return d.err
}

type dumper struct {
	w      io.Writer
	err    error
	psects int
}

func (d *dumper) line(indent int, format string, args ...any) {
	if d.err != nil {
		return
	}

	_, d.err = fmt.Fprintf(d.w, "%s%s\n", strings.Repeat("    ", indent), fmt.Sprintf(format, args...))
}

func (d *dumper) record(n int, rec Record) {
	switch rec := rec.(type) {
	case *MainHeader:
		d.line(0, "%d. HDR MHD: module %q, version %q", n, rec.Name, rec.Version)
		d.line(1, "created %q, structure level %d, maximum record size %d", rec.Created, rec.StructureLevel, rec.MaxRecordSize)

		if strings.Trim(rec.Patched, " \x00") != "" {
			d.line(1, "patched %q", rec.Patched)
		}

	case *TextHeader:
		d.line(0, "%d. HDR %s: %q", n, rec.Type, rec.Text)

	case *GSD:
		d.line(0, "%d. GSD", n)

		for _, s := range rec.Subrecords {
			d.subrecord(s)
		}

	case *TIR:
		d.line(0, "%d. %s", n, rec.Type)

		for _, c := range rec.Commands {
			d.line(1, "%s", FormatCommand(c))
		}

	case *EOM:
		name := "EOM"
		if rec.Word {
			name = "EOMW"
		}

		text := fmt.Sprintf("%d. %s: severity %s", n, name, severityName(rec.Severity))
		if rec.HasTransfer {
			text += fmt.Sprintf(", transfer address psect %d offset %#x", rec.Psect, rec.Transfer)
		}

		if rec.HasFlags {
			text += fmt.Sprintf(", flags %#x", rec.Flags)
		}

		d.line(0, "%s", text)

	case *LNK:
		d.line(0, "%d. LNK type %d, flags %#x: %q", n, rec.Type, rec.Flags, rec.Name)

	case *Unknown:
		d.line(0, "%d. %s: %d bytes % x", n, rec.Type, len(rec.Data), rec.Data)
	}
}

func (d *dumper) subrecord(s Subrecord) {
	switch s := s.(type) {
	case *Psect:
		text := fmt.Sprintf("%s %d: %q, alignment %s, %s, %d bytes", s.GSDType(), d.psects, s.Name,
			alignmentName(s.Align), PsectFlagNames(s.Flags), s.Alloc)
		if s.Shared {
			text += fmt.Sprintf(", base %#x", s.Base)
		}

		d.line(1, "%s", text)
		d.psects++

	case *Symbol:
		text := fmt.Sprintf("%s %q", s.Type, s.Name)

		if !s.Defined() {
			text += " (reference)"
		} else {
			text += fmt.Sprintf(" = %#x", s.Value)
			if s.Flags&SymREL != 0 {
				text += fmt.Sprintf(" in psect %d", s.Psect)
			}
		}

		if flags := SymbolFlagNames(s.Flags); flags != "" {
			text += ", " + flags
		}

		if symbolLayouts[s.Type].entry && s.Defined() {
			text += fmt.Sprintf(", mask %#04x", s.Mask)
		}

		if symbolLayouts[s.Type].local {
			text += fmt.Sprintf(", environment %d", s.Env)
		}

		if s.Extra != 0 {
			text += fmt.Sprintf(", extra %#x", s.Extra)
		}

		if s.DataType != 0 {
			text += fmt.Sprintf(", data type %d", s.DataType)
		}

		if s.Formals != nil {
			text += fmt.Sprintf(", %d to %d arguments", s.Formals.Min, s.Formals.Max)
		}

		d.line(1, "%s", text)

	case *IdentCheck:
		d.line(1, "IDC %q ident % x object %q, flags %#x", s.Name, s.Ident, s.Object, s.Flags)

	case *Environment:
		d.line(1, "ENV %q, parent %d, flags %#x", s.Name, s.Parent, s.Flags)
	}
}

// FormatCommand describes one TIR command on a line.
func FormatCommand(c Command) string {
	info := ops[c.Op]

	switch {
	case c.Op == OpStoreImmediate:
		return fmt.Sprintf("STO_IMM %d bytes: % x", len(c.Data), c.Data)
	case c.Op == opsByName["STA_CKARG"]:
		return fmt.Sprintf("%s %q argument %d, descriptor % x", c.Op, c.Name, c.Index, c.Data)
	}

	switch info.format {
	case opName:
		return fmt.Sprintf("%s %q", c.Op, c.Name)
	case opByte, opWord, opLong:
		return fmt.Sprintf("%s %#x", c.Op, c.StackedValue())
	case opPsectB, opPsectW, opPsectL, opWPsectB, opWPsectW, opWPsectL:
		return fmt.Sprintf("%s psect %d offset %#x", c.Op, c.Psect, c.StackedValue())
	case opEnvName:
		return fmt.Sprintf("%s environment %d %q", c.Op, c.Env, c.Name)
	case opIndex:
		return fmt.Sprintf("%s literal %d", c.Op, c.Index)
	case opField:
		return fmt.Sprintf("%s bits %d to %d", c.Op, c.Pos, int(c.Pos)+int(c.Size)-1)
	case opBytes:
		return fmt.Sprintf("%s % x", c.Op, c.Data)
	}

	return c.Op.String()
}

// PsectFlagNames spells out a psect's flags the way MACRO's .PSECT
// attributes do: "NOPIC,CON,REL,LCL,NOSHR,EXE,RD,WRT,NOVEC".
func PsectFlagNames(flags uint16) string {
	var names []string

	for _, f := range psectFlagNames {
		switch {
		case flags&*f.bit != 0:
			names = append(names, f.set)
		case f.clear != "":
			names = append(names, f.clear)
		}
	}

	return strings.Join(names, ",")
}

// SymbolFlagNames lists the symbol flags that are set: "DEF,REL".
func SymbolFlagNames(flags uint16) string {
	var names []string

	for _, f := range symFlagNames {
		if flags&*f.bit != 0 {
			names = append(names, f.name)
		}
	}

	return strings.Join(names, ",")
}

func alignmentName(a byte) string {
	switch a {
	case 0:
		return "BYTE"
	case 1:
		return "WORD"
	case 2:
		return "LONG"
	case 3:
		return "QUAD"
	case 4:
		return "OCTA"
	case 9:
		return "PAGE"
	}

	return fmt.Sprintf("2**%d", a)
}

func severityName(s byte) string {
	switch s {
	case SeveritySuccess:
		return "SUCCESS"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityAbort:
		return "ABORT"
	}

	return fmt.Sprintf("%d", s)
}

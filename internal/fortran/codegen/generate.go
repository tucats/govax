package codegen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// The program sections a FORTRAN module's routines contribute to
// (Decision 10 of docs/PHASE-74 - FORTRAN plan.md): the instructions,
// read-only data (constants, descriptors, I/O control blocks), and the
// static storage of variables and the compiler's cells.
const (
	codePsect  = "$CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG"
	pdataPsect = "$PDATA,PIC,CON,REL,LCL,SHR,NOEXE,RD,NOWRT,LONG"
	localPsect = "$LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,LONG"
)

// The flags of a class A descriptor (the OpenVMS Calling Standard's
// DSC$B_AFLAGS): FORTRAN's arrays are stored by columns, and the
// descriptor holds the multipliers and the bounds.
const (
	flagColumn = 1 << 5 // DSC$V_FL_COLUMN
	flagCoeff  = 1 << 6 // DSC$V_FL_COEFF
	flagBounds = 1 << 7 // DSC$V_FL_BOUNDS
)

// Generate writes the MACRO-32 code for a source file's routines to w,
// after the header the caller has written: the definitions the file
// uses, then each routine (its storage map, its storage, its read-only
// data, its code, with the source lines as comments before the
// instructions they became), the source's last lines, and .END, naming
// the main program's entry as the image's transfer address if the file
// has a main program. lines are the source's lines, as the source pass
// read them.
func Generate(w *Writer, routines []*ir.Routine, lines []*source.Line) {
	src := &sourceLines{lines: lines}
	names := &namer{}

	writeDefinitions(w, routines)

	main := ""

	for _, r := range routines {
		g := newRoutineGen(r, names, src)
		g.generate()
		g.write(w)

		if r.Kind == ir.Main {
			main = r.Name
		}
	}

	src.rest(w)
	w.Blank()
	w.Statement("", ".END", main, "")
}

// The descriptor codes the file may use, with what each means, in the
// order they are defined at the top of the file. govax's STARLET has no
// $DSCDEF, so the file defines the few it uses itself.
var definitions = []struct {
	name    string
	value   int
	comment string
}{
	{"DSC$K_DTYPE_LU", int(ir.DTypeLU), "longword logical"},
	{"DSC$K_DTYPE_L", int(ir.DTypeL), "longword integer"},
	{"DSC$K_DTYPE_F", int(ir.DTypeF), "F_floating"},
	{"DSC$K_DTYPE_T", int(ir.DTypeT), "text"},
	{"DSC$K_CLASS_S", int(ir.ClassS), "scalar or fixed-length string"},
	{"DSC$K_CLASS_A", int(ir.ClassA), "array"},
}

// writeDefinitions defines the descriptor codes the routines'
// descriptors use.
func writeDefinitions(w *Writer, routines []*ir.Routine) {
	used := map[string]bool{}

	note := func(d *ir.Descriptor) {
		used["DSC$K_DTYPE_"+d.DType.String()] = true
		used["DSC$K_CLASS_"+d.Class.String()] = true
	}

	for _, r := range routines {
		for _, d := range r.Data {
			if d.Kind == ir.DescriptorDatum {
				note(d.Desc)
			}
		}

		for _, c := range r.Cells {
			if c.Kind == ir.DescriptorCell {
				note(c.Desc)
			}
		}
	}

	if len(used) == 0 {
		return
	}

	w.Comment("Descriptor codes (the OpenVMS Calling Standard's DSC$K_ values)")

	for _, d := range definitions {
		if used[d.name] {
			w.Assignment(d.name, strconv.Itoa(d.value), d.comment)
		}
	}

	w.Comment("")
}

// write writes the routine: its storage map, the definitions of its
// dummy arguments' offsets, its $LOCAL storage, its $PDATA data, and its
// code.
func (g *routineGen) write(w *Writer) {
	r := g.r

	w.Blank()
	g.writeMap(w)

	for _, p := range r.Params {
		w.Assignment(g.names.symbol(p.Name), strconv.Itoa(4*p.Index),
			fmt.Sprintf("%s is argument %d", p.Name, p.Index))
	}

	if len(g.cells()) > 0 {
		w.Blank()
		w.Statement("", ".PSECT", localPsect, "")
		g.writeLocal(w)
	}

	if len(r.Data)+len(g.extra) > 0 {
		w.Blank()
		w.Statement("", ".PSECT", pdataPsect, "")
		g.writeData(w)
	}

	w.Blank()
	w.Statement("", ".PSECT", codePsect, "")
	w.Append(&g.pre)
	w.Statement("", ".ENTRY", r.Name+","+g.entryMask(), "")
	w.Append(&g.code)
}

// what a routine is, for its storage map's heading.
func (g *routineGen) what() string {
	switch g.r.Kind {
	case ir.Function:
		article := "a "
		if strings.ContainsAny(g.r.Unit.Result.Type.String()[:1], "AEIOU") {
			article = "an "
		}

		return article + g.r.Unit.Result.Type.String() + " function"
	case ir.Subroutine:
		return "a subroutine"
	}

	return "the main program"
}

// writeMap writes the routine's storage map: where each of its
// variables, arrays, and dummy arguments is, and its symbol in the
// MACRO file; then the numbered forms of names too long for MACRO.
func (g *routineGen) writeMap(w *Writer) {
	type row struct{ name, typ, where, symbol string }

	var rows []row

	for _, p := range g.r.Params {
		rows = append(rows, row{p.Name, typeText(p.Sym),
			"argument " + strconv.Itoa(p.Index), "@" + g.names.symbol(p.Name) + "(AP)"})
	}

	for _, c := range g.r.Cells {
		if c.Sym != nil {
			rows = append(rows, row{c.Sym.Name, typeText(c.Sym),
				"$LOCAL+" + strconv.Itoa(c.Offset), g.names.symbol(c.Name)})
		}
	}

	w.Comment("Storage map for " + g.r.Name + ", " + g.what())

	if len(rows) == 0 {
		w.Comment("  (no variables)")
	}

	widths := [3]int{4, 4, 5}

	for _, r := range rows {
		widths[0] = max(widths[0], len(r.name))
		widths[1] = max(widths[1], len(r.typ))
		widths[2] = max(widths[2], len(r.where))
	}

	for _, r := range rows {
		w.Comment(fmt.Sprintf("  %-*s  %-*s  %-*s  %s", widths[0], r.name, widths[1], r.typ, widths[2], r.where, r.symbol))
	}

	for _, name := range g.names.long {
		w.Comment("  " + g.names.symbol(name) + " is " + g.r.Name + "." + name)
	}

	w.Comment("")
}

// typeText is a variable's type, with an array's bounds: INTEGER,
// REAL (0:9), INTEGER (2,3), INTEGER (*).
func typeText(s *sema.Symbol) string {
	text := s.Type.String()

	if len(s.Dims) == 0 {
		return text
	}

	bounds := make([]string, len(s.Dims))

	for i, d := range s.Dims {
		switch {
		case d.Assumed && d.Lower == 1:
			bounds[i] = "*"
		case d.Assumed:
			bounds[i] = strconv.Itoa(d.Lower) + ":*"
		case d.Lower == 1:
			bounds[i] = strconv.Itoa(d.Upper)
		default:
			bounds[i] = strconv.Itoa(d.Lower) + ":" + strconv.Itoa(d.Upper)
		}
	}

	return text + " (" + strings.Join(bounds, ",") + ")"
}

// cells are the routine's $LOCAL cells in order: lowering's, then the
// code generator's own.
func (g *routineGen) cells() []*ir.Cell {
	cells := append(append([]*ir.Cell(nil), g.r.Cells...), g.spills...)
	sort.SliceStable(cells, func(i, j int) bool { return cells[i].Offset < cells[j].Offset })

	return cells
}

// writeLocal writes the routine's static storage: a block of longwords
// for each variable, array, and cell, and the descriptors whose
// addresses the code stores (their other fields set here).
func (g *routineGen) writeLocal(w *Writer) {
	at := 0

	for _, c := range g.cells() {
		if c.Offset > at {
			w.Statement("", ".BLKB", strconv.Itoa(c.Offset-at), "")
		}

		label := g.names.symbol(c.Name)

		if c.Kind == ir.DescriptorCell {
			w.Label(label)
			writeDescriptor(w, c.Desc, "0", "0", "set when the descriptor is used")
		} else {
			count, unit := c.Size/4, ".BLKL"
			if c.Size%4 != 0 {
				count, unit = c.Size, ".BLKB"
			}

			w.Statement(label, unit, strconv.Itoa(count), g.cellNote(c))
		}

		at = c.Offset + c.Size
	}

	if end := g.localSize(); end > at {
		w.Statement("", ".BLKB", strconv.Itoa(end-at), "")
	}
}

// cellNote says what a cell holds, for its comment.
func (g *routineGen) cellNote(c *ir.Cell) string {
	switch c.Kind {
	case ir.VariableCell:
		return c.Sym.Name
	case ir.ArrayCell:
		return c.Sym.Name + ", " + typeText(c.Sym)
	case ir.ResultCell:
		return c.Sym.Name + ", the function's value"
	case ir.CountCell:
		return fmt.Sprintf("DO %d's count", c.Loop)
	case ir.StepCell:
		return fmt.Sprintf("DO %d's step", c.Loop)
	}

	if strings.HasPrefix(c.Name, "$X") {
		return "a part of an expression, set aside"
	}

	return "temporary"
}

// writeData writes the routine's read-only data: the longword data
// first (constants, descriptors, control blocks), then the texts, whose
// bytes would leave what follows them unaligned.
func (g *routineGen) writeData(w *Writer) {
	data := append(append([]*ir.Datum(nil), g.r.Data...), g.extra...)

	for _, d := range data {
		if d.Kind != ir.TextDatum {
			g.writeDatum(w, d)
		}
	}

	for _, d := range data {
		if d.Kind == ir.TextDatum {
			g.writeDatum(w, d)
		}
	}
}

// writeDatum writes one datum.
func (g *routineGen) writeDatum(w *Writer, d *ir.Datum) {
	label := g.names.symbol(d.Name)

	switch d.Kind {
	case ir.ConstantDatum:
		if d.Value.T == ir.Float {
			w.Statement(label, ".LONG", fmt.Sprintf("^X%08X", uint32(d.Value.Bits)), ir.ExprText(d.Value))
		} else {
			w.Statement(label, ".LONG", strconv.Itoa(int(d.Value.Int())), "")
		}

	case ir.TextDatum:
		pieces := asciiPieces(d.Text)

		w.Statement(label, ".ASCII", pieces[0], "")

		for _, p := range pieces[1:] {
			w.Statement("", ".ASCII", p, "")
		}

	case ir.DescriptorDatum:
		w.Label(label)

		pointer := g.target(d.Desc.Target, 0)
		a0 := ""

		if d.Desc.Class == ir.ClassA {
			a0 = g.target(d.Desc.Target, -firstOffset(d.Desc))
		}

		writeDescriptor(w, d.Desc, pointer, a0, "")

	case ir.ControlDatum:
		g.writeControl(w, label, d.Control)
	}
}

// target is the address expression for the place m, moved by delta
// bytes: SQUARES.I, CALLS.M+4, FILL.$K1.
func (g *routineGen) target(m *ir.Mem, delta int) string {
	var name string

	if m.Cell != nil {
		name = m.Cell.Name
	} else {
		name = m.Datum.Name
	}

	return withOffset(g.names.symbol(name), m.Offset+delta)
}

// firstOffset is how far an array's first element is from the element
// whose subscripts are all zero (where A0 points), in bytes.
func firstOffset(d *ir.Descriptor) int {
	first, multiplier := 0, 1

	for _, b := range d.Dims {
		first += b.Lower * multiplier
		multiplier *= b.Upper - b.Lower + 1
	}

	return first * d.Length
}

// writeDescriptor writes a descriptor's fields (the OpenVMS Calling
// Standard, chapter 7): the length, the data type and class, and the
// pointer; for an array (class A) then the scale and digits (0), the
// flags, the dimension count, the array's size in bytes, A0, a
// multiplier for each dimension (its extent, the array being stored by
// columns), and each dimension's bounds. pointer and a0 are the address
// fields' values; note is the pointer's comment.
func writeDescriptor(w *Writer, d *ir.Descriptor, pointer, a0, note string) {
	what := "DSC$K_DTYPE_" + d.DType.String() + ",DSC$K_CLASS_" + d.Class.String()
	directive := ".ADDRESS"

	if pointer == "0" {
		directive = ".LONG"
	}

	w.Statement("", ".WORD", strconv.Itoa(d.Length), "length")
	w.Statement("", ".BYTE", what, "")
	w.Statement("", directive, pointer, note)

	if d.Class != ir.ClassA {
		return
	}

	w.Statement("", ".BYTE", fmt.Sprintf("0,0,^X%02X,%d", flagColumn|flagCoeff|flagBounds, len(d.Dims)),
		"scale, digits, flags, dimensions")
	w.Statement("", ".LONG", strconv.Itoa(d.Elements()*d.Length), "the array's size in bytes")
	w.Statement("", directive, a0, "A0, where the element with zero subscripts would be")

	multipliers := make([]string, len(d.Dims))
	bounds := make([]string, 0, 2*len(d.Dims))

	for i, b := range d.Dims {
		multipliers[i] = strconv.Itoa(b.Upper - b.Lower + 1)
		bounds = append(bounds, strconv.Itoa(b.Lower), strconv.Itoa(b.Upper))
	}

	w.Statement("", ".LONG", strings.Join(multipliers, ","), "multipliers")
	w.Statement("", ".LONG", strings.Join(bounds, ","), "bounds")
}

// The I/O control block's layout (Decision 12 of docs/PHASE-74 -
// FORTRAN plan.md; FTNRTL reads it, mar/ftn_listout.mar): a word of
// its size in bytes, a word of flags saying which of the statement's
// optional parts it has (none yet), and the unit number, -1 for *.
const controlSize = 8

// writeControl writes an I/O statement's control block.
func (g *routineGen) writeControl(w *Writer, label string, c *ir.Control) {
	what := "PRINT *"
	if c.Write {
		what = "WRITE (*,*)"
	}

	unit := strconv.Itoa(c.Unit)
	note := "unit " + unit

	if c.Unit == ir.DefaultUnit {
		note = "unit *"
	}

	w.Statement(label, ".WORD", strconv.Itoa(controlSize), what+"'s control block: its size")
	w.Statement("", ".WORD", "0", "flags")
	w.Statement("", ".LONG", unit, note)
}

// asciiPieces is a text as .ASCII operands, each short enough for a
// line: runs of printing characters between delimiters (a character the
// run doesn't hold), and any other character as its code in angle
// brackets: /IT'S/<9>/DONE/.
func asciiPieces(text string) []string {
	const perLine = 64

	var (
		pieces []string
		b      strings.Builder
		run    []byte
	)

	flushRun := func() {
		if len(run) == 0 {
			return
		}

		if d := delimiter(string(run)); d != "" {
			b.WriteString(d + string(run) + d)
		} else {
			for _, c := range run {
				b.WriteString("<" + strconv.Itoa(int(c)) + ">")
			}
		}

		run = run[:0]
	}

	for i := 0; i < len(text); i++ {
		c := text[i]

		if c >= ' ' && c <= '~' {
			run = append(run, c)
		} else {
			flushRun()
			b.WriteString("<" + strconv.Itoa(int(c)) + ">")
		}

		if len(run)+b.Len() >= perLine {
			flushRun()
			pieces = append(pieces, b.String())
			b.Reset()
		}
	}

	flushRun()

	if b.Len() > 0 || len(pieces) == 0 {
		pieces = append(pieces, b.String())
	}

	return pieces
}

// delimiter is a delimiter for a .ASCII string of the text: the first
// of MACRO's usual ones the text doesn't hold, or "" if it holds them
// all.
func delimiter(text string) string {
	for _, d := range []string{"/", "\"", "|", "!", "#", "%", "&", "*", "+", "?", "@", "^", "~", "`", "{", "}"} {
		if !strings.Contains(text, d) {
			return d
		}
	}

	return ""
}

// withOffset is sym+offset as MACRO writes it: SQUARES.A, SQUARES.A+8,
// SQUARES.A-4.
func withOffset(sym string, offset int) string {
	switch {
	case offset > 0:
		return sym + "+" + strconv.Itoa(offset)
	case offset < 0:
		return sym + "-" + strconv.Itoa(-offset)
	}

	return sym
}

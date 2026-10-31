package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
)

// Dump writes routines as the golden tests compare them. For each
// routine: a line naming it; its dummy arguments, cells, and data; then
// its blocks in layout order, each statement on a line of its own, and
// a "; line n" line wherever the FORTRAN statement the code comes from
// changes. Expressions are written as calls, the operator with its
// type's letter (add.L, mul.F, cvt.LF for a conversion from L to F), a
// place as its base with its offset and index, the way MACRO would
// write it (TABLE-4[I]), a dummy argument's with @ (@N).
//
//	routine SQUARES main
//	  cell  I       L  $LOCAL+0   variable I
//	  cell  $C1     L  $LOCAL+4   DO 10's count
//	  datum $IO1    control PRINT *
//	  datum $D1     descriptor L S 4 I
//	B1:
//	  ; line 3
//	  assign $C1 = 5
//	  assign I = 1
//	  jump B2
//	B2:  ; DO 10's body
//	  ; line 4
//	  call FTN$PRINT_LIST(ref $IO1)
//	  ...
func Dump(routines []*Routine) string {
	var b strings.Builder

	for _, r := range routines {
		dumpRoutine(&b, r)
	}

	return b.String()
}

// dumpRoutine writes one routine.
func dumpRoutine(b *strings.Builder, r *Routine) {
	fmt.Fprintf(b, "routine %s %s", r.Name, r.Kind)

	if r.Result != None {
		fmt.Fprintf(b, " %s", r.Result)
	}

	b.WriteString("\n")

	for _, p := range r.Params {
		if p.Through != nil {
			fmt.Fprintf(b, "  param %-7s %-2s through %s\n", p.Name, p.T, p.Through.Name)
		} else {
			fmt.Fprintf(b, "  param %-7s %-2s argument %d\n", p.Name, p.T, p.Index)
		}
	}

	for _, e := range r.Entries {
		fmt.Fprintf(b, "  entry %d %s", e.Number, e.Name)

		for _, a := range e.Args {
			fmt.Fprintf(b, " %s=%d", a.Param.Name, a.Index)
		}

		if e.First != nil {
			fmt.Fprintf(b, " at %s", e.First.Name)
		}

		b.WriteString("\n")
	}

	for _, c := range r.Cells {
		section := "$LOCAL"
		if c.Common != nil {
			section = c.Common.Psect
		}

		fmt.Fprintf(b, "  cell  %-7s %-2s %s+%-4d %s\n", c.Name, c.T, section, c.Offset, cellNote(c))
	}

	fmt.Fprintf(b, "  $LOCAL is %d bytes\n", r.LocalSize)

	for _, in := range r.Initials {
		section := "$LOCAL"
		if in.Sym.Common != nil {
			section = in.Sym.Common.Psect
		}

		fmt.Fprintf(b, "  init  %s+%d %s = % X\n", section, in.Offset, in.Item, in.Bytes)
	}

	for _, name := range r.Externals {
		fmt.Fprintf(b, "  external %s\n", name)
	}

	for _, d := range r.Data {
		fmt.Fprintf(b, "  datum %-7s %s\n", d.Name, datumText(d))
	}

	var line ast.Stmt

	for _, blk := range r.Blocks {
		head := blk.Name + ":"
		if blk.Label != 0 {
			head += " L" + strconv.Itoa(blk.Label)
		}

		if blk.Note != "" {
			head += "  ; " + blk.Note
		}

		b.WriteString(head + "\n")

		for _, s := range blk.Stmts {
			line = lineMark(b, line, s.Source())
			b.WriteString("  " + StmtText(s) + "\n")
		}

		if blk.End != nil {
			line = lineMark(b, line, blk.End.Source())
			b.WriteString("  " + TerminatorText(blk.End) + "\n")
		}
	}
}

// lineMark writes a "; line n" line if s isn't the statement the code
// before it came from, and returns s.
func lineMark(b *strings.Builder, last, s ast.Stmt) ast.Stmt {
	if s != nil && s != last && (last == nil || s.Position().Line != last.Position().Line) {
		fmt.Fprintf(b, "  ; line %d\n", s.Position().Line)
	}

	if s == nil {
		return last
	}

	return s
}

// cellNote says what a cell holds.
func cellNote(c *Cell) string {
	switch c.Kind {
	case VariableCell:
		return "variable " + c.Sym.Name
	case ArrayCell:
		return fmt.Sprintf("array %s, %d bytes", c.Sym.Name, c.Size)
	case ResultCell:
		return "result " + c.Sym.Name
	case CountCell:
		return c.Loop + "'s count"
	case StepCell:
		return c.Loop + "'s step"
	case DescriptorCell:
		return "descriptor " + descriptorText(c.Desc)
	case ArgCell:
		return "address of " + c.Dummy
	case SelectorCell:
		return "the entry point used"
	}

	return "temporary"
}

// datumText is what a datum holds.
func datumText(d *Datum) string {
	switch d.Kind {
	case ConstantDatum:
		return d.Value.T.String() + " " + ExprText(d.Value)

	case TextDatum:
		return "text '" + strings.ReplaceAll(d.Text, "'", "''") + "'"

	case DescriptorDatum:
		return "descriptor " + descriptorText(d.Desc)
	}

	if d.Control.Write {
		return "control WRITE (*,*)"
	}

	return "control PRINT *"
}

// descriptorText is a descriptor's type, class, length, bounds, and
// target: L S 4 I, F A 4 (0:9) V, T S 3 $K1.
func descriptorText(d *Descriptor) string {
	text := fmt.Sprintf("%s %s %d", d.DType, d.Class, d.Length)

	if len(d.Dims) > 0 {
		bounds := make([]string, len(d.Dims))
		for i, b := range d.Dims {
			bounds[i] = fmt.Sprintf("%d:%d", b.Lower, b.Upper)
			if b.Set {
				bounds[i] = "?"
			}
		}

		text += " (" + strings.Join(bounds, ",") + ")"
	}

	if d.Target != nil {
		text += " " + MemText(d.Target)
	}

	return text
}

// StmtText is a statement as the dump writes it.
func StmtText(s Stmt) string {
	switch s := s.(type) {
	case *Assign:
		return "assign " + MemText(s.Dst) + " = " + ExprText(s.Src)

	case *CharMove:
		return "move " + TextText(s.Dst) + " = " + TextText(s.Src)

	case *Call:
		args := make([]string, len(s.Args))

		for i, a := range s.Args {
			switch {
			case a.Routine != "":
				args[i] = "routine " + a.Routine
			case a.Ref != nil:
				args[i] = "ref " + MemText(a.Ref)
			default:
				args[i] = "val " + ExprText(a.Value)
			}
		}

		text := "call " + s.Routine + "(" + strings.Join(args, ", ") + ")"
		if s.Result != nil {
			text += " -> " + MemText(s.Result)
		}

		return text
	}

	return fmt.Sprintf("%T", s)
}

// TerminatorText is a terminator as the dump writes it.
func TerminatorText(t Terminator) string {
	switch t := t.(type) {
	case *Jump:
		return "jump " + t.To.Name
	case *Branch:
		return "branch " + ExprText(t.Cond) + " " + t.True.Name + " " + t.False.Name
	case *Return:
		if t.Value == nil {
			return "return"
		}

		return "return " + ExprText(t.Value)
	case *Switch:
		return "switch " + ExprText(t.Index) + " [" + blockNames(t.Targets) + "] " + t.Else.Name
	case *Sign:
		return "sign " + ExprText(t.Value) + " " + t.Neg.Name + " " + t.Zero.Name + " " + t.Pos.Name
	case *IndirectJump:
		return "jump @" + MemText(t.Through) + " [" + blockNames(t.Targets) + "]"
	case *NoReturn:
		return "noreturn"
	}

	return fmt.Sprintf("%T", t)
}

// blockNames is the names of blocks, separated by blanks.
func blockNames(list []*Block) string {
	names := make([]string, len(list))
	for i, b := range list {
		names[i] = b.Name
	}

	return strings.Join(names, " ")
}

// MemText is a place as the dump writes it: I, $T1, TABLE-4[I], @N, and
// for a place at an address computed, @(the address): @(arg S) is the
// descriptor a CHARACTER dummy S's argument points at, and @(@(arg S)+4) its
// text.
func MemText(m *Mem) string {
	var text string

	switch {
	case m.Cell != nil:
		text = m.Cell.Name
	case m.Datum != nil:
		text = m.Datum.Name
	case m.Param != nil:
		text = "@" + m.Param.Name
	case m.Ptr != nil:
		text = "@(" + ExprText(m.Ptr) + ")"
	}

	if m.Offset != 0 {
		text += fmt.Sprintf("%+d", m.Offset)
	}

	if m.Index != nil {
		text += "[" + ExprText(m.Index) + "]"
	}

	return text
}

// TextText is a CHARACTER value's place and length as the dump writes it:
// S(8), S+1[J](3), $K1(5).
func TextText(t Text) string {
	return MemText(t.Mem) + "(" + ExprText(t.Len) + ")"
}

// ExprText is an expression as the dump writes it.
func ExprText(e Expr) string {
	switch e := e.(type) {
	case *Const:
		if e.T == Quad || e.T == Octa {
			re, im := e.Parts()

			return "(" + floatText(re) + ", " + floatText(im) + ")"
		}

		if e.T.Floating() {
			return floatText(e)
		}

		return strconv.Itoa(int(e.Int()))

	case *Load:
		return MemText(e.Mem)

	case *Addr:
		return "addr(" + MemText(e.Mem) + ")"

	case *LabelAddr:
		return "label(" + e.Block.Name + ")"

	case *CallValue:
		return "R0"

	case *ArgPointer:
		return "arg " + e.Param.Name

	case *Op:
		if e.Op == Cvt || e.Op == Zext || e.Op == Round || e.Op == Low {
			return e.Op.String() + "." + e.X.Type().String() + e.T.String() + "(" + ExprText(e.X) + ")"
		}

		text := e.Op.String() + "." + e.T.String() + "(" + ExprText(e.X)
		if e.Y != nil {
			text += ", " + ExprText(e.Y)
		}

		return text + ")"

	case *Cmp:
		return e.Op.String() + "." + e.X.Type().String() + "(" + ExprText(e.X) + ", " + ExprText(e.Y) + ")"

	case *CharCmp:
		return e.Op.String() + ".T(" + TextText(e.X) + ", " + TextText(e.Y) + ")"

	case *Field:
		return "field(" + ExprText(e.X) + ", " + ExprText(e.Pos) + ", " + ExprText(e.Size) + ")"

	case *CharIndex:
		return "index(" + TextText(e.X) + ", " + TextText(e.Y) + ", " + ExprText(e.Base) + ")"
	}

	return fmt.Sprintf("%T", e)
}

// floatText is a floating constant with the fewest digits that give its
// value again, as FORTRAN writes it: an F_floating one always with a period
// or an exponent (1.0, not 1), a D or G one with a D exponent (0.1D0,
// 1.5D-10), and an H one with a Q exponent.
func floatText(c *Const) string {
	if c.T == Float {
		text := strings.ToUpper(strconv.FormatFloat(c.Real().Float64(), 'g', -1, 32))
		if !strings.ContainsAny(text, ".E") {
			text += ".0"
		}

		return text
	}

	letter := "D"
	if c.T == HFloat {
		letter = "Q"
	}

	text := strings.ToUpper(c.Real().Text('g', -1))
	if i := strings.IndexByte(text, 'E'); i >= 0 {
		return text[:i] + letter + strings.TrimPrefix(text[i+1:], "+")
	}

	return text + letter + "0"
}

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
		fmt.Fprintf(b, "  param %-7s %-2s argument %d\n", p.Name, p.T, p.Index)
	}

	for _, c := range r.Cells {
		fmt.Fprintf(b, "  cell  %-7s %-2s $LOCAL+%-4d %s\n", c.Name, c.T, c.Offset, cellNote(c))
	}

	fmt.Fprintf(b, "  $LOCAL is %d bytes\n", r.LocalSize)

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
		return fmt.Sprintf("DO %d's count", c.Loop)
	case StepCell:
		return fmt.Sprintf("DO %d's step", c.Loop)
	case DescriptorCell:
		return "descriptor " + descriptorText(c.Desc)
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

	case *Call:
		args := make([]string, len(s.Args))

		for i, a := range s.Args {
			if a.Ref != nil {
				args[i] = "ref " + MemText(a.Ref)
			} else {
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
	case *NoReturn:
		return "noreturn"
	}

	return fmt.Sprintf("%T", t)
}

// MemText is a place as the dump writes it: I, $T1, TABLE-4[I], @N.
func MemText(m *Mem) string {
	var text string

	switch {
	case m.Cell != nil:
		text = m.Cell.Name
	case m.Datum != nil:
		text = m.Datum.Name
	case m.Param != nil:
		text = "@" + m.Param.Name
	}

	if m.Offset != 0 {
		text += fmt.Sprintf("%+d", m.Offset)
	}

	if m.Index != nil {
		text += "[" + ExprText(m.Index) + "]"
	}

	return text
}

// ExprText is an expression as the dump writes it.
func ExprText(e Expr) string {
	switch e := e.(type) {
	case *Const:
		if e.T == Float {
			return floatText(e)
		}

		return strconv.Itoa(int(e.Int()))

	case *Load:
		return MemText(e.Mem)

	case *Addr:
		return "addr(" + MemText(e.Mem) + ")"

	case *Op:
		if e.Op == Cvt {
			return "cvt." + e.X.Type().String() + e.T.String() + "(" + ExprText(e.X) + ")"
		}

		text := e.Op.String() + "." + e.T.String() + "(" + ExprText(e.X)
		if e.Y != nil {
			text += ", " + ExprText(e.Y)
		}

		return text + ")"

	case *Cmp:
		return e.Op.String() + "." + e.X.Type().String() + "(" + ExprText(e.X) + ", " + ExprText(e.Y) + ")"
	}

	return fmt.Sprintf("%T", e)
}

// floatText is an F_floating constant with the fewest digits that give
// its value again, always with a period or an exponent (1.0, not 1).
func floatText(c *Const) string {
	text := strings.ToUpper(strconv.FormatFloat(c.Real().Float64(), 'g', -1, 32))
	if !strings.ContainsAny(text, ".E") {
		text += ".0"
	}

	return text
}

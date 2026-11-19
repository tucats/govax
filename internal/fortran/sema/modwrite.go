package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// A module's text (finding 7; docs/planning/PHASE-106 - Fortran 90 modules.md,
// the author's answer 1). When a module compiles, what its users may reach is
// written as Fortran 90: the module's specification part as declarations, with
// every type and kind written out, every constant's value folded, every bound
// a number, then (subtask 3) CONTAINS and each module procedure's header and
// declarations without its body, which is what an interface is. What Fortran
// can't say is in comment lines that start !GOVAX$, which the compiler reads as
// facts about the declaration after them (modread.go):
//
//	!GOVAX$ MODULE FILE 1
//	!GOVAX$ COMPILER GOVAX FORTRAN V1.0-140
//	!GOVAX$ SOURCE DUA1:[WORK]CONSTS.F90;1
//	!GOVAX$ FLOATING D
//	MODULE CONSTS
//	!GOVAX$ PSECT $MOD$CONSTS SIZE 44 ALIGN 0
//	!GOVAX$ TYPE POINT SIZE 8 BOUNDARY 4 NATURAL
//	  TYPE POINT
//	    REAL(KIND=4) :: X
//	    REAL(KIND=4) :: Y
//	  END TYPE POINT
//	  REAL(KIND=4), PARAMETER :: PI = 3.14159274E+00_4
//	!GOVAX$ AT CALLS 0 4
//	  INTEGER(KIND=4) :: CALLS
//	!GOVAX$ AT TABLE 4 40
//	  REAL(KIND=4) :: TABLE(1:10)
//	END MODULE CONSTS
//
// So a reader learns what a module offers without its source, and the
// compiler reads it with its own source, lexer, parser, and semantic analysis.
// A variable's offset is the AT line's, the layout its module computed, so the
// module's /ALIGN means nothing to a user (finding 10); a type's layout is the
// TYPE line's, checked against what the reader computes from the components.
// Initial values aren't written: they are in the module's object only.
//
// The text's lines are free form, at most 132 characters; a long declaration
// goes on in lines that start with &, which continue it exactly where the line
// before ended (the standard's 3.3.1.3).

// moduleFormat is the module text's form's number, its first line's.
const moduleFormat = "1"

// moduleWriter makes a module's text.
type moduleWriter struct {
	u     *Unit
	lines []string

	// failed are the entities the text can't describe yet, each reported
	// NOTIMPL; a module with one makes no text.
	failed []failedEntity

	// written are the structures written so far, each once, before any that
	// mentions it.
	written map[*Structure]bool
}

// failedEntity is an entity of a module its text can't describe.
type failedEntity struct {
	pos  diag.Position
	what string
}

// finishModule makes the text of the module u, analyzed without an error, and
// keeps it for the USE statements after it and the module file. What the text
// can't describe yet is reported NOTIMPL, and then there is no text.
func (p *Program) finishModule(u *Unit, opts Options, messages *diag.List) {
	w := &moduleWriter{u: u, written: map[*Structure]bool{}}
	w.module(opts)

	if len(w.failed) > 0 {
		for _, f := range w.failed {
			messages.Report("NOTIMPL", f.pos, f.what)
		}

		return
	}

	mt := &ModuleText{Name: u.Name, Lines: w.lines, Digest: moduleDigest(w.lines)}
	p.Modules = append(p.Modules, mt)
	p.modules.compiled[u.Name] = mt
}

// moduleDigest is the SHA-256 of a module's text, in hexadecimal, but for its
// SOURCE line: two compilations of one source with nothing changed give one
// digest, whatever the files' times (finding 9).
func moduleDigest(lines []string) string {
	h := sha256.New()

	for _, line := range lines {
		if strings.HasPrefix(line, "!GOVAX$ SOURCE ") {
			continue
		}

		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}

	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// fact writes a !GOVAX$ line.
func (w *moduleWriter) fact(words ...string) {
	w.lines = append(w.lines, "!GOVAX$ "+strings.Join(words, " "))
}

// put writes a statement, indented by depth, in lines of at most 132
// characters: past that, each line ends with & and the next starts with one,
// which continues a token, even a character constant, exactly.
func (w *moduleWriter) put(depth int, text string) {
	const width = 100

	text = strings.Repeat("  ", depth) + text

	for len(text) > width+2 {
		w.lines = append(w.lines, text[:width]+"&")
		text = "&" + text[width:]
	}

	w.lines = append(w.lines, text)
}

// fail notes an entity the text can't describe.
func (w *moduleWriter) fail(pos diag.Position, what string) {
	w.failed = append(w.failed, failedEntity{pos, what})
}

// module writes the whole text.
func (w *moduleWriter) module(opts Options) {
	u := w.u

	w.fact("MODULE", "FILE", moduleFormat)
	w.fact("COMPILER", "GOVAX", "FORTRAN", "V"+opts.Version)

	if opts.Source != "" {
		w.fact("SOURCE", opts.Source)
	}

	floating := "D"
	if u.GFloating {
		floating = "G"
	}

	w.fact("FLOATING", floating)
	w.put(0, "MODULE "+u.Name)

	for _, b := range u.Commons {
		if b.Module != "" {
			w.fact("PSECT", b.Psect, "SIZE", strconv.Itoa(b.Size), "ALIGN", strconv.Itoa(b.LayoutAlign))
		}
	}

	for _, st := range u.Structures {
		w.structure(st)
	}

	for _, s := range u.Symbols {
		if s.Class == Constant && s.Use == nil {
			w.constant(s)
		}
	}

	w.variables()
	w.namelists()
	w.put(0, "END MODULE "+u.Name)
}

// typeSpec is the type t as a Fortran 90 type specification with its kind or
// length written out: INTEGER(KIND=2), REAL(KIND=8), CHARACTER(LEN=10),
// TYPE(POINT). A VAX structure's record has no such form (RECORD /S/ is a
// statement of its own), and gives "".
func typeSpec(t Type) string {
	switch t.Kind {
	case Integer, Real, Logical, Complex:
		name := map[Kind]string{Integer: "INTEGER", Real: "REAL", Logical: "LOGICAL", Complex: "COMPLEX"}[t.Kind]

		return name + "(KIND=" + strconv.Itoa(t.KindNumber()) + ")"

	case Character:
		if t.Size == 0 {
			return "CHARACTER(LEN=*)"
		}

		return "CHARACTER(LEN=" + strconv.Itoa(t.Size) + ")"

	case Record:
		if t.Struct != nil && t.Struct.Derived {
			return "TYPE(" + t.Struct.Name + ")"
		}
	}

	return ""
}

// dimsText is an array's dimensions as a declaration writes them: (1:10, 0:3),
// a deferred or assumed shape's colons, an assumed size's *.
func dimsText(dims []Dim) string {
	if len(dims) == 0 {
		return ""
	}

	parts := make([]string, len(dims))

	for i, d := range dims {
		switch {
		case d.Deferred:
			parts[i] = ":"
		case d.AssumedShape && d.LowerExpr != nil:
			parts[i] = fortranText(d.LowerExpr) + ":"
		case d.AssumedShape:
			parts[i] = strconv.Itoa(d.Lower) + ":"
		default:
			lower := strconv.Itoa(d.Lower)
			if d.LowerExpr != nil {
				lower = fortranText(d.LowerExpr)
			}

			upper := strconv.Itoa(d.Upper)

			switch {
			case d.Assumed:
				upper = "*"
			case d.UpperExpr != nil:
				upper = fortranText(d.UpperExpr)
			}

			parts[i] = lower + ":" + upper
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// structure writes a structure or derived type, after those its fields name,
// with its layout's fact.
func (w *moduleWriter) structure(st *Structure) {
	if w.written[st] {
		return
	}

	w.written[st] = true

	if st.Name == "" {
		w.fail(st.Pos, "an unnamed structure in a module")

		return
	}

	for _, f := range st.Fields {
		if f.Struct != nil {
			w.structure(f.Struct)
		}
	}

	layout := "PACKED"
	if st.Natural {
		layout = "NATURAL"
	}

	w.fact("TYPE", st.Name, "SIZE", strconv.Itoa(st.Size), "BOUNDARY", strconv.Itoa(st.Boundary), layout)

	if st.Derived {
		w.put(1, "TYPE "+st.Name)

		if st.Sequence {
			w.put(2, "SEQUENCE")
		}

		for _, f := range st.Fields {
			spec := typeSpec(f.Type)
			if f.Struct != nil {
				spec = typeSpec(recordType(f.Struct))
			}

			w.put(2, spec+" :: "+f.Name+dimsText(f.Dims))
		}

		w.put(1, "END TYPE "+st.Name)

		return
	}

	if len(st.Initials) > 0 {
		w.fail(st.Pos, "a structure with initial values in a module")

		return
	}

	w.put(1, "STRUCTURE /"+st.Name+"/")

	var union *Union

	mapNo := 0

	for _, f := range st.Fields {
		if f.Union != union {
			if union != nil {
				w.put(3, "END MAP")
				w.put(2, "END UNION")
			}

			union, mapNo = f.Union, 0

			if union != nil {
				w.put(2, "UNION")
			}
		}

		if union != nil && f.Map != mapNo {
			if mapNo != 0 {
				w.put(3, "END MAP")
			}

			mapNo = f.Map
			w.put(3, "MAP")
		}

		depth := 2
		if union != nil {
			depth = 4
		}

		switch {
		case f.Name == "%FILL":
			w.put(depth, "BYTE %FILL("+strconv.Itoa(f.Size())+")")
		case f.Struct != nil:
			w.put(depth, "RECORD /"+f.Struct.Name+"/ "+f.Name+dimsText(f.Dims))
		default:
			w.put(depth, oldTypeText(f.Type)+" "+f.Name+dimsText(f.Dims))
		}
	}

	if union != nil {
		w.put(3, "END MAP")
		w.put(2, "END UNION")
	}

	w.put(1, "END STRUCTURE")
}

// oldTypeText is the type t as a VAX FORTRAN type statement writes it, with
// its length: INTEGER*4, CHARACTER*10, a structure's field's form.
func oldTypeText(t Type) string {
	switch t.Kind {
	case Integer:
		return "INTEGER*" + strconv.Itoa(t.Size)
	case Real:
		return "REAL*" + strconv.Itoa(t.Size)
	case Logical:
		return "LOGICAL*" + strconv.Itoa(t.Size)
	case Complex:
		return "COMPLEX*" + strconv.Itoa(t.Size)
	case Character:
		return "CHARACTER*" + strconv.Itoa(t.Size)
	}

	return t.String()
}

// constant writes a named constant with its value.
func (w *moduleWriter) constant(s *Symbol) {
	var value string

	switch {
	case s.ArrayValue != nil:
		v, ok := arrayConstText(s.ArrayValue, s.Type, s.Dims)
		if !ok {
			w.fail(s.Pos, "the value of the constant "+s.Name+" in a module's text")

			return
		}

		value = v

	case s.Value != nil:
		v, ok := constText(s.Value, s.Type)
		if !ok {
			w.fail(s.Pos, "the value of the constant "+s.Name+" in a module's text")

			return
		}

		value = v

	default:
		return
	}

	spec := typeSpec(s.Type)
	if spec == "" {
		w.fail(s.Pos, "a constant of a structure in a module")

		return
	}

	// A REAL or COMPLEX value computed, not written, is its rounded value,
	// whose digits the reader must not take for a literal's.
	if c := s.Value; c != nil && (s.Type.Kind == Real || s.Type.Kind == Complex) && !c.literal {
		w.fact("FOLDED", s.Name)
	}

	w.put(1, spec+", PARAMETER :: "+s.Name+dimsText(s.Dims)+" = "+value)
}

// variables writes the module's variables, each after its AT fact, the
// module's own in its section's order, then each COMMON block's with its
// COMMON statement.
func (w *moduleWriter) variables() {
	u := w.u

	for _, b := range u.Commons {
		if b.Module == "" {
			continue
		}

		for _, s := range u.Symbols {
			if s.Common == b {
				w.variable(s)
			}
		}
	}

	for _, b := range u.Commons {
		if b.Module != "" {
			continue
		}

		w.fact("COMMON", b.Psect, "SIZE", strconv.Itoa(b.Size), "ALIGN", strconv.Itoa(b.LayoutAlign))

		for _, s := range u.Symbols {
			if s.Common == b {
				w.variable(s)
			}
		}

		names := make([]string, len(b.Members))
		for i, s := range b.Members {
			names[i] = s.Name
		}

		w.put(1, "COMMON /"+b.Name+"/ "+strings.Join(names, ", "))
	}
}

// variable writes one variable's AT fact and declaration.
func (w *moduleWriter) variable(s *Symbol) {
	w.fact("AT", s.Name, strconv.Itoa(s.Offset), strconv.Itoa(s.Size()))

	if s.Struct != nil && !s.Struct.Derived {
		w.put(1, "RECORD /"+s.Struct.Name+"/ "+s.Name+dimsText(s.Dims))

		if s.Volatile {
			w.put(1, "VOLATILE "+s.Name)
		}

		return
	}

	spec := typeSpec(s.Type)
	if s.LengthExpr != nil {
		w.fail(s.Pos, "a CHARACTER variable of a length not constant in a module")

		return
	}

	if s.Allocatable {
		spec += ", ALLOCATABLE"
	}

	if s.Volatile {
		spec += ", VOLATILE"
	}

	if s.Target {
		spec += ", TARGET"
	}

	w.put(1, spec+" :: "+s.Name+dimsText(s.Dims))
}

// namelists writes the module's NAMELIST groups.
func (w *moduleWriter) namelists() {
	for _, n := range w.u.Namelists {
		names := make([]string, len(n.Items))
		for i, s := range n.Items {
			names[i] = s.Name
		}

		w.put(1, "NAMELIST /"+n.Name+"/ "+strings.Join(names, ", "))
	}
}

// constText is the constant c, of type t, as Fortran 90 writes it so that
// reading it back gives the same value: an INTEGER or LOGICAL with its kind, a
// REAL in as many digits as bring back its bits exactly, a COMPLEX as its two
// parts, a CHARACTER constant between apostrophes, with CHAR(n) for a
// character that can't be in one, a derived type's by its constructor. It
// reports false for a value it can't write (a typeless constant's bits that
// are no number).
func constText(c *Const, t Type) (string, bool) {
	kind := "_" + strconv.Itoa(t.KindNumber())

	switch t.Kind {
	case Integer:
		if c.Int == -2147483648 {
			return "(-2147483647_4-1)", true
		}

		return strconv.Itoa(int(c.Int)) + kind, true

	case Logical:
		if c.Int&1 != 0 {
			return ".TRUE." + kind, true
		}

		return ".FALSE." + kind, true

	case Real:
		if c.Raw != nil && !rawMatches(c.Raw, c.Real, t) {
			return "", false
		}

		if c.literal {
			return literalText(c.exactRe, t), true
		}

		return realText(c.Real, t)

	case Complex:
		part := Type{Kind: Real, Size: t.Size / 2}

		if c.Raw != nil {
			return "", false
		}

		re, ok1 := realText(c.Real, part)
		im, ok2 := realText(c.Imag, part)

		if c.literal {
			re, im, ok1, ok2 = literalText(c.exactRe, part), literalText(c.exactIm, part), true, true
		}

		return "(" + re + ", " + im + ")", ok1 && ok2

	case Character:
		return characterText(c.Text), true

	case Record:
		st := t.Struct
		if st == nil || !st.Derived || len(c.Parts) != len(st.Fields) {
			return "", false
		}

		parts := make([]string, len(c.Parts))

		for i, p := range c.Parts {
			f := st.Fields[i]
			ft := f.Type

			if f.Struct != nil {
				ft = recordType(f.Struct)
			}

			var (
				text string
				ok   bool
			)

			switch p := p.(type) {
			case *Const:
				text, ok = constText(p, ft)
			case *ArrayConst:
				text, ok = arrayConstText(p, ft, f.Dims)
			}

			if !ok {
				return "", false
			}

			parts[i] = text
		}

		return st.Name + "(" + strings.Join(parts, ", ") + ")", true
	}

	return "", false
}

// rawMatches reports whether a REAL constant's bytes are its value's.
func rawMatches(raw []byte, v vaxfloat.Value, t Type) bool {
	f := realFormatOf(t)

	return string(floatBytes(f, v)) == string(raw)
}

// realFormatOf is the floating format of a REAL type of the text: REAL(8) is
// written in decimal, which the reader rounds to its module's format, so D
// here is only a stand-in for 8 bytes' digits; realText checks the module's
// own format.
func realFormatOf(t Type) vaxfloat.Format {
	switch t.Size {
	case 8:
		return vaxfloat.D
	case 16:
		return vaxfloat.H
	}

	return vaxfloat.F
}

// realText is a REAL value of type t in decimal, with its kind: the fewest
// digits that round back to the same value in every format of its size (D and
// G for REAL(8)), so the reader, which rounds to its module's format, gets the
// same bits.
func realText(v vaxfloat.Value, t Type) (string, bool) {
	formats := []vaxfloat.Format{realFormatOf(t)}
	if t.Size == 8 {
		formats = []vaxfloat.Format{vaxfloat.D, vaxfloat.G}
	}

	kind := "_" + strconv.Itoa(t.Size)

	for prec := 1; prec <= 40; prec++ {
		text := v.Text('e', prec)

		back, err := vaxfloat.Parse(text)
		if err != nil {
			return "", false
		}

		same := true

		for _, f := range formats {
			want, err1 := vaxfloat.Round(f, v)
			got, err2 := vaxfloat.Round(f, back)

			if err1 == nil && (err2 != nil || got.Cmp(want) != 0) {
				same = false
			}
		}

		if same {
			return strings.ToUpper(text) + kind, true
		}
	}

	return "", false
}

// literalText is a REAL literal's exact value as it was written, in the fewest
// digits that read back as the same exact value, with its kind: a literal keeps
// its digits' extra precision in an operation of a higher one (section
// 2.3.1.2), so the text gives its digits, not its rounded value's.
func literalText(exact vaxfloat.Value, t Type) string {
	return strings.ToUpper(exact.Text('e', -1)) + "_" + strconv.Itoa(t.Size)
}

// characterText is a character constant's text: between apostrophes, each
// apostrophe doubled, a character outside printing ASCII written CHAR(n) and
// joined by //.
func characterText(s string) string {
	if s == "" {
		return "''"
	}

	var parts []string

	var run strings.Builder

	flush := func() {
		if run.Len() > 0 {
			parts = append(parts, "'"+run.String()+"'")
			run.Reset()
		}
	}

	for i := range len(s) {
		c := s[i]

		switch {
		case c == '\'':
			run.WriteString("''")
		case c >= ' ' && c <= '~':
			run.WriteByte(c)
		default:
			flush()
			parts = append(parts, "CHAR("+strconv.Itoa(int(c))+")")
		}
	}

	flush()

	return strings.Join(parts, "//")
}

// arrayConstText is an array constant of element type t and dimensions dims as
// a constructor, under RESHAPE for more than one dimension.
func arrayConstText(c *ArrayConst, t Type, dims []Dim) (string, bool) {
	elems := make([]string, len(c.Elems))

	for i, e := range c.Elems {
		text, ok := constText(e, t)
		if !ok {
			return "", false
		}

		elems[i] = text
	}

	// An empty constructor would need its type, which Fortran 90 can't
	// write.
	if len(elems) == 0 {
		return "", false
	}

	text := "(/ " + strings.Join(elems, ", ") + " /)"

	if len(dims) <= 1 {
		return text, true
	}

	extents := make([]string, len(dims))
	for i, d := range dims {
		extents[i] = strconv.Itoa(d.Extent())
	}

	return "RESHAPE(" + text + ", (/ " + strings.Join(extents, ", ") + " /))", true
}

// fortranText is a specification expression written back as Fortran from the
// syntax tree (finding 7): an adjustable bound or a length a module
// procedure's dummy arguments use. Every operation is parenthesized, so the
// text means what the tree does whatever the operators' precedence.
func fortranText(e ast.Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *ast.IntConst:
		if e.Radix == 8 {
			return `"` + e.Text
		}

		return e.Text
	case *ast.RealConst:
		return e.Text
	case *ast.LogicalConst:
		if e.Value {
			return ".TRUE."
		}

		return ".FALSE."
	case *ast.CharConst:
		return characterText(e.Value)
	case *ast.KindConst:
		return fortranText(e.X) + "_" + e.Kind
	case *ast.ComplexConst:
		return "(" + fortranText(e.Re) + ", " + fortranText(e.Im) + ")"
	case *ast.Name:
		return e.Name
	case *ast.Apply:
		return e.Name + "(" + fortranListText(e.Args) + ")"
	case *ast.Unary:
		return "(" + e.Op.String() + fortranText(e.X) + ")"
	case *ast.Binary:
		return "(" + fortranText(e.X) + e.Op.String() + fortranText(e.Y) + ")"
	case *ast.DefinedOp:
		if e.Y == nil {
			return "(" + e.Op + " " + fortranText(e.X) + ")"
		}

		return "(" + fortranText(e.X) + " " + e.Op + " " + fortranText(e.Y) + ")"
	case *ast.Paren:
		return "(" + fortranText(e.X) + ")"
	case *ast.Substring:
		return fortranText(e.X) + "(" + fortranText(e.Lower) + ":" + fortranText(e.Upper) + ")"
	case *ast.FieldRef:
		text := fortranText(e.X) + "%" + e.Field
		if e.Args != nil {
			text += "(" + fortranListText(e.Args) + ")"
		}

		return text
	case *ast.KeywordArg:
		return e.Name + "=" + fortranText(e.X)
	case *ast.Triplet:
		text := fortranText(e.Lower) + ":" + fortranText(e.Upper)
		if e.Stride != nil {
			text += ":" + fortranText(e.Stride)
		}

		return text
	case *ast.ArrayConstructor:
		return "(/ " + fortranListText(e.Values) + " /)"
	}

	return ast.DumpExpr(e)
}

// fortranListText is a list of expressions, separated by commas.
func fortranListText(list []ast.Expr) string {
	parts := make([]string, len(list))
	for i, e := range list {
		parts[i] = fortranText(e)
	}

	return strings.Join(parts, ", ")
}

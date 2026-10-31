package ast

import "github.com/tucats/govax/internal/fortran/diag"

// The statements: a node for every statement of VAX FORTRAN (the 1988
// manual's Table D-2), grouped as Figure 1-1 groups them. Where several
// statements share a grammar they share a node, and the StmtBase's Kind
// says which statement it was: DIMENSION and VIRTUAL are DimensionStmts;
// SAVE, EXTERNAL, INTRINSIC, VOLATILE, AUTOMATIC, and STATIC AttrStmts;
// PRINT, TYPE, ACCEPT, and READ f, list PrintStmts; READ, WRITE, and
// REWRITE with a control list TransferStmts; OPEN, CLOSE, INQUIRE, and
// the positioning statements AuxIOStmts.

// Program units' headers (sections 4.12, 6.2.2, 6.2.3, 4.1, 6.2.4).
type (
	// ProgramStmt is PROGRAM name.
	ProgramStmt struct {
		StmtBase
		Name string
	}

	// FunctionStmt is [type] FUNCTION name[*n] [([p[,p]...])]. Type is
	// nil when the statement names no type, and the function's type comes
	// from the declarations or the name's first letter; a length after
	// the name (CHARACTER FUNCTION F*10) is in Type's Len.
	FunctionStmt struct {
		StmtBase
		Type   *TypeSpec
		Name   string
		Params []*Name
	}

	// SubroutineStmt is SUBROUTINE name [([p[,p]...])]. A dummy written *
	// (an alternate return's place, section 6.1.1.5) is a Name whose Name
	// is "*".
	SubroutineStmt struct {
		StmtBase
		Name   string
		Params []*Name
	}

	// BlockDataStmt is BLOCK DATA [name]; Name is "" without one.
	BlockDataStmt struct {
		StmtBase
		Name string
	}

	// EntryStmt is ENTRY name [([p[,p]...])], an alternate entry point
	// in a subprogram; its * dummies are as SubroutineStmt's.
	EntryStmt struct {
		StmtBase
		Name   string
		Params []*Name
	}
)

// Specification statements (chapter 4).
type (
	// TypeStmt is a type declaration: INTEGER I, A(10); CHARACTER*8
	// NAME, LINE*80 (sections 4.4.1, 4.4.2). It is also a field
	// declaration in a structure.
	TypeStmt struct {
		StmtBase
		Type     TypeSpec
		Entities []*Declarator
	}

	// DimensionStmt is DIMENSION a(d)[,a(d)]... (section 4.5), and
	// VIRTUAL, which is the same statement under another name.
	DimensionStmt struct {
		StmtBase
		Entities []*Declarator
	}

	// ImplicitStmt is IMPLICIT typ(a[,a]...)[,typ(a[,a]...)]..., or
	// IMPLICIT NONE (section 4.8).
	ImplicitStmt struct {
		StmtBase
		None  bool
		Rules []*ImplicitRule
	}

	// ParameterStmt is PARAMETER (p=c[,p=c]...) (section 4.11), or
	// appendix A.4's form without parentheses, where Old is set: its
	// constants take their type from their values, not their names.
	ParameterStmt struct {
		StmtBase
		Old  bool
		Defs []*ParamDef
	}

	// CommonStmt is COMMON [/[cb]/]nlist[[,]/[cb]/nlist]... (section
	// 4.2), each block in the order written; a block named twice is
	// listed twice, as written.
	CommonStmt struct {
		StmtBase
		Blocks []*CommonBlock
	}

	// EquivalenceStmt is EQUIVALENCE (nlist)[,(nlist)]... (section 4.6):
	// each set's names, elements, and substrings share a place.
	EquivalenceStmt struct {
		StmtBase
		Sets [][]Expr
	}

	// DataStmt is DATA nlist/clist/[[,]nlist/clist/]... (section 4.3).
	DataStmt struct {
		StmtBase
		Sets []*DataSet
	}

	// AttrStmt is a statement that gives each name in its list an
	// attribute, which its Kind says: SAVE [a[,a]...] (section 4.14),
	// EXTERNAL v[,v]... (4.7), INTRINSIC v[,v]... (4.9), VOLATILE nlist
	// (4.16), and AUTOMATIC and STATIC (not in the 1988 manual: Phase
	// 76's survey). SAVE with no list has Items nil: everything.
	AttrStmt struct {
		StmtBase
		Items []*AttrItem
	}

	// NamelistStmt is NAMELIST /group/ list[[,]/group/ list]... (section
	// 4.10).
	NamelistStmt struct {
		StmtBase
		Groups []*NamelistGroup
	}

	// RecordStmt is RECORD /structure/ list[,/structure/ list]...
	// (section 4.13).
	RecordStmt struct {
		StmtBase
		Groups []*RecordGroup
	}

	// StructureStmt is a structure declaration (section 4.15.1):
	// STRUCTURE [/name/] [fields], its field declarations (type
	// statements, RECORD, PARAMETER, nested structures, and unions), and
	// END STRUCTURE, the last statement of Body (missing if the parser
	// reported it missing). Fields is a nested structure's field
	// namelist.
	StructureStmt struct {
		StmtBase
		Name   string
		Fields []*Declarator
		Body   []Stmt
	}

	// UnionStmt is a union declaration (section 4.15.3): UNION, its map
	// declarations, and END UNION, the last statement of Body.
	UnionStmt struct {
		StmtBase
		Body []Stmt
	}

	// MapStmt is a map declaration within a union: MAP, its field
	// declarations, and END MAP, the last statement of Body.
	MapStmt struct {
		StmtBase
		Body []Stmt
	}

	// EndBlockStmt is END STRUCTURE, END UNION, or END MAP, by its
	// Kind.
	EndBlockStmt struct {
		StmtBase
	}

	// DictionaryStmt is DICTIONARY 'cdd-path[/[NO]LIST]' (section 1.4.1),
	// which reads the Common Data Dictionary: Path is the string.
	DictionaryStmt struct {
		StmtBase
		Path string
	}
)

// ImplicitRule is one type and its letters in an IMPLICIT statement:
// REAL*8 (A-H, O-Z).
type ImplicitRule struct {
	Type    TypeSpec
	Letters []LetterRange
}

// LetterRange is a letter, or a range of them, in an IMPLICIT statement:
// A, or A-H, where First and Last are the same for a single letter.
type LetterRange struct {
	First, Last byte
	Pos         diag.Position
}

// ParamDef is one p=c of a PARAMETER statement.
type ParamDef struct {
	Name  *Name
	Value Expr
}

// CommonBlock is one block's part of a COMMON statement: its name ("" for
// blank common) and the entities placed in it.
type CommonBlock struct {
	Name     string
	Pos      diag.Position
	Entities []*Declarator
}

// DataSet is one nlist/clist/ of a DATA statement: the names, array
// elements, substrings, and implied-DO lists, and the constants they
// get.
type DataSet struct {
	Names  []Expr
	Values []*DataValue
}

// DataValue is one item of a constant list, in a DATA statement or a
// type statement's initial values: c, or r*c, where r is a repeat count
// (an unsigned integer constant or a constant's name; nil without one)
// and c a constant, possibly signed, or a constant's name.
type DataValue struct {
	Repeat Expr
	Value  Expr
	Pos    diag.Position
}

// AttrItem is a name in an AttrStmt's list, or a common block's name
// between slashes (SAVE /BLK/), where Common is set.
type AttrItem struct {
	Name   string
	Common bool
	Pos    diag.Position
}

// NamelistGroup is one group of a NAMELIST statement.
type NamelistGroup struct {
	Name  string
	Pos   diag.Position
	Items []*Name
}

// RecordGroup is one /structure/ list of a RECORD statement.
type RecordGroup struct {
	Structure string
	Pos       diag.Position
	Entities  []*Declarator
}

// Compilation control: OPTIONS (section 1.4.3), the CDEC$ directives
// (section 10.3), and FORMAT (chapter 8), which describes data rather
// than doing anything.
type (
	// OptionsStmt is OPTIONS /qualifier[/qualifier]...: each qualifier
	// as written, NOCHECK or CHECK=(BOUNDS,NOOVERFLOW).
	OptionsStmt struct {
		StmtBase
		Qualifiers []*Qualifier
	}

	// DirectiveStmt is a CDEC$ directive: Name is IDENT, TITLE,
	// SUBTITLE, or PSECT. IDENT, TITLE, and SUBTITLE have a string, Text
	// (HasText says whether one was given); PSECT a common block (Common,
	// "" for blank common) and its attributes (LCL, GBL, [NO]WRT,
	// [NO]SHR, ALIGN=n), each a Qualifier.
	DirectiveStmt struct {
		StmtBase
		Name    string
		Text    string
		HasText bool
		Common  string
		Attrs   []*Qualifier
	}

	// FormatStmt is FORMAT (spec) (chapter 8). Text is the specification
	// as written, parentheses and all: FTNRTL interprets formats, and
	// semantic analysis checks them (internal/fortran/format). Offset is
	// where Text starts in the statement's source text, so a message can
	// point into it. VFEs are its variable format expressions, parsed, in
	// the order they come in the text (section 8.3.2); the parser leaves a
	// format that isn't valid with none, for semantic analysis to report.
	FormatStmt struct {
		StmtBase
		Text   string
		Offset int
		VFEs   []Expr
	}
)

// Qualifier is one qualifier of an OPTIONS statement, or one attribute of
// a CDEC$ PSECT directive: its name as written (NOCHECK, G_FLOATING,
// NOWRT) and its values, if it has any (CHECK=(BOUNDS,OVERFLOW) has two,
// ALIGN=2 one).
type Qualifier struct {
	Name   string
	Values []string
	Pos    diag.Position
}

// Assignment and control statements (chapters 3 and 5).
type (
	// AssignStmt is an assignment, Target = Value (chapter 3). Target is
	// a *Name, an *Apply (an array element, or, if the name isn't an
	// array, a statement function's definition), a *Substring, or a
	// *FieldRef.
	AssignStmt struct {
		StmtBase
		Target Expr
		Value  Expr
	}

	// AssignLabelStmt is ASSIGN s TO v (section 3.5).
	AssignLabelStmt struct {
		StmtBase
		Target LabelRef
		Var    *Name
	}

	// CallStmt is CALL name [([a][,[a]]...)] (section 5.1). A nil
	// argument is a null one: CALL S(A,,B). An alternate return
	// specifier, *s or &s, is an *AltReturn.
	CallStmt struct {
		StmtBase
		Name    string
		NamePos diag.Position
		Args    []Expr
	}

	// GotoStmt is the unconditional GO TO s (section 5.6.1).
	GotoStmt struct {
		StmtBase
		Target LabelRef
	}

	// ComputedGotoStmt is GO TO (s[,s]...)[,] e (section 5.6.2).
	ComputedGotoStmt struct {
		StmtBase
		Targets []LabelRef
		Index   Expr
	}

	// AssignedGotoStmt is GO TO v [[,](s[,s]...)] (section 5.6.3);
	// Targets is nil without the list.
	AssignedGotoStmt struct {
		StmtBase
		Var     *Name
		Targets []LabelRef
	}

	// IfStmt is the logical IF, IF (Cond) Then (section 5.7.2). Then is
	// the statement executed when Cond is true; it has no label of its
	// own.
	IfStmt struct {
		StmtBase
		Cond Expr
		Then Stmt
	}

	// ArithIfStmt is the arithmetic IF, IF (Cond) s1, s2, s3 (section
	// 5.7.1): the labels for a negative, a zero, and a positive value.
	ArithIfStmt struct {
		StmtBase
		Cond    Expr
		Targets [3]LabelRef
	}

	// IfThenStmt is IF (Cond) THEN, which starts an IF block (section
	// 5.7.3).
	IfThenStmt struct {
		StmtBase
		Cond Expr
	}

	// ElseIfStmt is ELSE IF (Cond) THEN.
	ElseIfStmt struct {
		StmtBase
		Cond Expr
	}

	// ElseStmt is ELSE.
	ElseStmt struct {
		StmtBase
	}

	// EndIfStmt is END IF.
	EndIfStmt struct {
		StmtBase
	}

	// DoStmt is DO [s[,]] v = e1, e2 [, e3] (section 5.3.1). Target's
	// Label is 0 for a loop without a label, which END DO ends (5.4).
	// Step is nil when e3 is left out (it is 1).
	DoStmt struct {
		StmtBase
		Target LabelRef
		Var    *Name
		Start  Expr
		Limit  Expr
		Step   Expr
	}

	// DoWhileStmt is DO [s[,]] WHILE (e) (section 5.3.2); Target's Label
	// is 0 without a label.
	DoWhileStmt struct {
		StmtBase
		Target LabelRef
		Cond   Expr
	}

	// EndDoStmt is END DO (section 5.4).
	EndDoStmt struct {
		StmtBase
	}

	// ContinueStmt is CONTINUE (section 5.2).
	ContinueStmt struct {
		StmtBase
	}

	// StopStmt is STOP [disp] (section 5.10), and PAUSE [disp] (5.8),
	// by its Kind. Display is nil, a *CharConst, or an *IntConst of 1 to
	// 5 digits.
	StopStmt struct {
		StmtBase
		Display Expr
	}

	// ReturnStmt is RETURN [i] (section 5.9). Alt is the alternate
	// return's number, or nil.
	ReturnStmt struct {
		StmtBase
		Alt Expr
	}

	// EndStmt is END (section 5.5), every unit's last statement.
	EndStmt struct {
		StmtBase
	}
)

// Input and output (chapters 7 and 9, appendix A).
type (
	// PrintStmt is an I/O statement without a control list, keyword f
	// [, iolist], by its Kind: PRINT and TYPE (section 7.6), ACCEPT
	// (7.5), and READ f, list (7.2.1). A name as the format may be a
	// namelist group's (PRINT NL), which the declarations decide.
	PrintStmt struct {
		StmtBase
		Format FormatSpec
		Items  []Expr
	}

	// TransferStmt is READ, WRITE, or REWRITE with a control list
	// (sections 7.1.1, 7.2 to 7.4), by its Kind. The control list's
	// unit, format, namelist, and record are taken out of it whether
	// given by position or by keyword (WRITE (6, 10), WRITE (FMT=10,
	// UNIT=6), READ (1'5), READ (UNIT=1, REC=5)); Specs are the rest
	// (ERR=, END=, IOSTAT=, and the keyed READ's KEY= forms and KEYID=),
	// in order. Format is nil for unformatted I/O, Namelist nil unless
	// NML= gave one (a namelist given by position is the Format), Rec nil
	// for sequential access.
	TransferStmt struct {
		StmtBase
		Unit     UnitSpec
		Format   *FormatSpec
		Namelist *Name
		Rec      Expr
		Specs    []*IOSpec
		Items    []Expr
	}

	// CodeStmt is ENCODE or DECODE (c, f, b[,IOSTAT=ios][,ERR=s]) [list]
	// (appendix A.1), by its Kind: Count characters of Buffer under
	// Format.
	CodeStmt struct {
		StmtBase
		Count  Expr
		Format FormatSpec
		Buffer Expr
		Specs  []*IOSpec
		Items  []Expr
	}

	// AuxIOStmt is OPEN, CLOSE, INQUIRE (chapter 9), REWIND, BACKSPACE,
	// ENDFILE, UNLOCK, DELETE, or FIND (appendix A.3), by its Kind. Unit
	// is the unit, given by position or by UNIT= (or written alone, as
	// in REWIND 5), nil if none was (INQUIRE (FILE=...)); Rec the record
	// of DELETE's and FIND's u'r or REC=; Specs the other specifiers in
	// order.
	AuxIOStmt struct {
		StmtBase
		Unit  *UnitSpec
		Rec   Expr
		Specs []*IOSpec
	}

	// DefineFileStmt is DEFINE FILE u(m,n,U,v)[,u(m,n,U,v)]... (appendix
	// A.2).
	DefineFileStmt struct {
		StmtBase
		Files []*FileDef
	}
)

// IOSpec is one keyword=value specifier of a control list or an OPEN,
// CLOSE, or INQUIRE statement: Keyword is the keyword as written (ERR,
// IOSTAT, FILE, ...). A label's specifier (ERR=, END=) has Label set; one
// with no value (OPEN's READONLY, SHARED, NOSPANBLOCKS) has neither; the
// rest have a Value (OPEN's KEY= a *KeyList).
type IOSpec struct {
	Keyword string
	Value   Expr
	Label   *LabelRef
	Pos     diag.Position
}

// FileDef is one file of a DEFINE FILE statement: its unit, number of
// records, record length in 16-bit words, form (always U, unformatted),
// and associated variable.
type FileDef struct {
	Unit    Expr
	Records Expr
	Length  Expr
	Var     *Name
	Pos     diag.Position
}

// BadStmt is a statement with an error the parser or the lexer reported.
// It keeps the statement's label and kind, so that a jump to it isn't
// reported again, and an END IF with an error still closes its block.
type BadStmt struct {
	StmtBase
}

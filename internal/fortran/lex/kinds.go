package lex

import "sort"

// Kind is what kind of statement a statement is: the lexer's answer to
// "which statement is this?", which the parser uses to choose the
// function that parses it.
//
// Every statement of VAX FORTRAN has a kind, not only those the compiler
// can compile yet, so that a statement it can't is reported by name ("the
// READ statement is not implemented yet") rather than as a syntax error.
type Kind int

// The kinds of statement.
const (
	// Unknown is text the lexer can't take for any statement.
	Unknown Kind = iota

	// Empty is a statement with no text: a label alone on a line.
	Empty

	// Assignment is v = e: an arithmetic, logical, or character
	// assignment, or a statement function's definition, which looks the
	// same (F(X) = X*X) until the declarations say whether F is an array.
	Assignment

	// Directive is a CDEC$ compiler directive (chapter 10).
	Directive

	// The IF statements, told apart by what follows the condition.
	LogicalIf    // IF (e) st
	ArithmeticIf // IF (e) s1, s2, s3
	IfThen       // IF (e) THEN
	ElseIf       // ELSE IF (e) THEN

	// The keyword statements, each named by its keyword (keywords).
	Accept
	Assign
	Automatic
	Backspace
	BlockData
	Byte
	Call
	Character
	Close
	Common
	Complex
	Continue
	Data
	Decode
	DefineFile
	Delete
	Dictionary
	Dimension
	Do
	DoWhile
	DoubleComplex
	DoublePrecision
	Else
	Encode
	End
	EndDo
	EndFile
	EndIf
	EndMap
	EndStructure
	EndUnion
	Entry
	Equivalence
	External
	Find
	Format
	Function
	GoTo
	Implicit
	Include
	Inquire
	Integer
	Intrinsic
	Logical
	Map
	Namelist
	Open
	Options
	Parameter
	Pause
	Print
	Program
	Read
	Real
	Record
	Return
	Rewind
	Rewrite
	Save
	Static
	Stop
	Structure
	Subroutine
	Type
	Union
	Unlock
	Virtual
	Volatile
	Write

	numKinds
)

// kindInfo is what the compiler knows about a kind of statement.
type kindInfo struct {
	// name is the statement's name in messages: its keyword with the
	// blanks the manual writes (GO TO, END IF), or a description.
	name string

	// keyword is the keyword that starts the statement, without blanks,
	// as the lexer sees it after removing them (GOTO, ENDIF); empty for a
	// kind not found by its keyword.
	keyword string

	// executable is true for an executable statement (Figure 1-1's
	// list): only such a statement's label may be the target of a jump.
	executable bool

	// typeKeyword is true for a data type keyword (INTEGER, REAL, ...),
	// which may start a FUNCTION statement: INTEGER FUNCTION F(X).
	typeKeyword bool
}

// kinds describes each kind.
var kinds = [numKinds]kindInfo{
	Unknown:      {name: "unrecognized statement"},
	Empty:        {name: "empty statement"},
	Assignment:   {name: "assignment", executable: true},
	Directive:    {name: "CDEC$ directive"},
	LogicalIf:    {name: "logical IF", executable: true},
	ArithmeticIf: {name: "arithmetic IF", executable: true},
	IfThen:       {name: "IF THEN", executable: true},
	ElseIf:       {name: "ELSE IF", executable: true},

	Accept:          {name: "ACCEPT", keyword: "ACCEPT", executable: true},
	Assign:          {name: "ASSIGN", keyword: "ASSIGN", executable: true},
	Automatic:       {name: "AUTOMATIC", keyword: "AUTOMATIC"},
	Backspace:       {name: "BACKSPACE", keyword: "BACKSPACE", executable: true},
	BlockData:       {name: "BLOCK DATA", keyword: "BLOCKDATA"},
	Byte:            {name: "BYTE", keyword: "BYTE", typeKeyword: true},
	Call:            {name: "CALL", keyword: "CALL", executable: true},
	Character:       {name: "CHARACTER", keyword: "CHARACTER", typeKeyword: true},
	Close:           {name: "CLOSE", keyword: "CLOSE", executable: true},
	Common:          {name: "COMMON", keyword: "COMMON"},
	Complex:         {name: "COMPLEX", keyword: "COMPLEX", typeKeyword: true},
	Continue:        {name: "CONTINUE", keyword: "CONTINUE", executable: true},
	Data:            {name: "DATA", keyword: "DATA"},
	Decode:          {name: "DECODE", keyword: "DECODE", executable: true},
	DefineFile:      {name: "DEFINE FILE", keyword: "DEFINEFILE"},
	Delete:          {name: "DELETE", keyword: "DELETE", executable: true},
	Dictionary:      {name: "DICTIONARY", keyword: "DICTIONARY"},
	Dimension:       {name: "DIMENSION", keyword: "DIMENSION"},
	Do:              {name: "DO", keyword: "DO", executable: true},
	DoWhile:         {name: "DO WHILE", keyword: "DO", executable: true},
	DoubleComplex:   {name: "DOUBLE COMPLEX", keyword: "DOUBLECOMPLEX", typeKeyword: true},
	DoublePrecision: {name: "DOUBLE PRECISION", keyword: "DOUBLEPRECISION", typeKeyword: true},
	Else:            {name: "ELSE", keyword: "ELSE", executable: true},
	Encode:          {name: "ENCODE", keyword: "ENCODE", executable: true},
	End:             {name: "END", keyword: "END", executable: true},
	EndDo:           {name: "END DO", keyword: "ENDDO", executable: true},
	EndFile:         {name: "ENDFILE", keyword: "ENDFILE", executable: true},
	EndIf:           {name: "END IF", keyword: "ENDIF", executable: true},
	EndMap:          {name: "END MAP", keyword: "ENDMAP"},
	EndStructure:    {name: "END STRUCTURE", keyword: "ENDSTRUCTURE"},
	EndUnion:        {name: "END UNION", keyword: "ENDUNION"},
	Entry:           {name: "ENTRY", keyword: "ENTRY"},
	Equivalence:     {name: "EQUIVALENCE", keyword: "EQUIVALENCE"},
	External:        {name: "EXTERNAL", keyword: "EXTERNAL"},
	Find:            {name: "FIND", keyword: "FIND", executable: true},
	Format:          {name: "FORMAT", keyword: "FORMAT"},
	Function:        {name: "FUNCTION", keyword: "FUNCTION"},
	GoTo:            {name: "GO TO", keyword: "GOTO", executable: true},
	Implicit:        {name: "IMPLICIT", keyword: "IMPLICIT"},
	Include:         {name: "INCLUDE", keyword: "INCLUDE"},
	Inquire:         {name: "INQUIRE", keyword: "INQUIRE", executable: true},
	Integer:         {name: "INTEGER", keyword: "INTEGER", typeKeyword: true},
	Intrinsic:       {name: "INTRINSIC", keyword: "INTRINSIC"},
	Logical:         {name: "LOGICAL", keyword: "LOGICAL", typeKeyword: true},
	Map:             {name: "MAP", keyword: "MAP"},
	Namelist:        {name: "NAMELIST", keyword: "NAMELIST"},
	Open:            {name: "OPEN", keyword: "OPEN", executable: true},
	Options:         {name: "OPTIONS", keyword: "OPTIONS"},
	Parameter:       {name: "PARAMETER", keyword: "PARAMETER"},
	Pause:           {name: "PAUSE", keyword: "PAUSE", executable: true},
	Print:           {name: "PRINT", keyword: "PRINT", executable: true},
	Program:         {name: "PROGRAM", keyword: "PROGRAM"},
	Read:            {name: "READ", keyword: "READ", executable: true},
	Real:            {name: "REAL", keyword: "REAL", typeKeyword: true},
	Record:          {name: "RECORD", keyword: "RECORD"},
	Return:          {name: "RETURN", keyword: "RETURN", executable: true},
	Rewind:          {name: "REWIND", keyword: "REWIND", executable: true},
	Rewrite:         {name: "REWRITE", keyword: "REWRITE", executable: true},
	Save:            {name: "SAVE", keyword: "SAVE"},
	Static:          {name: "STATIC", keyword: "STATIC"},
	Stop:            {name: "STOP", keyword: "STOP", executable: true},
	Structure:       {name: "STRUCTURE", keyword: "STRUCTURE"},
	Subroutine:      {name: "SUBROUTINE", keyword: "SUBROUTINE"},
	Type:            {name: "TYPE", keyword: "TYPE", executable: true},
	Union:           {name: "UNION", keyword: "UNION"},
	Unlock:          {name: "UNLOCK", keyword: "UNLOCK", executable: true},
	Virtual:         {name: "VIRTUAL", keyword: "VIRTUAL"},
	Volatile:        {name: "VOLATILE", keyword: "VOLATILE"},
	Write:           {name: "WRITE", keyword: "WRITE", executable: true},
}

// String is the statement's name, as messages show it: "GO TO".
func (k Kind) String() string {
	if k < 0 || k >= numKinds {
		return "unknown kind"
	}

	return kinds[k].name
}

// Executable reports whether statements of this kind are executable, so
// that a jump may go to their labels.
func (k Kind) Executable() bool {
	return k >= 0 && k < numKinds && kinds[k].executable
}

// TypeKeyword reports whether the kind is a data type declaration
// (INTEGER, REAL, ...).
func (k Kind) TypeKeyword() bool {
	return k >= 0 && k < numKinds && kinds[k].typeKeyword
}

// byKeyword is the keyword statements, longest keyword first, which is
// the order the lexer tries them in: ENDIF must be tried before END, or
// END IF would be END followed by IF. The IF statements, DO and DO WHILE
// (DO's tests come before this table's), and kinds with no keyword are
// left out.
var byKeyword = func() []Kind {
	var list []Kind

	for k := Kind(0); k < numKinds; k++ {
		switch {
		case kinds[k].keyword == "", k == Do, k == DoWhile:
			continue
		}

		list = append(list, k)
	}

	sort.SliceStable(list, func(i, j int) bool {
		return len(kinds[list[i]].keyword) > len(kinds[list[j]].keyword)
	})

	return list
}()

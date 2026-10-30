package diag

// kind is one entry of the message table: a message's severity and the
// format of its text (fmt's verbs, filled in from Report's arguments).
type kind struct {
	Severity Severity
	Format   string
}

// messages is every message the compiler can report, by identifier. The
// identifiers and wording are govax's own (docs/PHASE-74 - FORTRAN
// plan.md, Decision 19): they follow the style of VAX FORTRAN's (a short
// identifier, lowercase text) without copying them. Each phase adds the
// messages its passes need.
var messages = map[string]kind{
	// NOTIMPL is a statement or feature the compiler recognizes but can't
	// compile yet. It is an error, so a program using it never runs with
	// the feature silently missing (Decision 1).
	"NOTIMPL": {Error, "%s is not implemented yet"},

	// The source form's messages (internal/fortran/source).

	// NOINITIAL is a continuation line with no statement before it to
	// continue: the first line of a file, or a line after a directive.
	"NOINITIAL": {Error, "continuation line has no statement to continue"},

	// CONTLABEL is a continuation line with something in its label
	// field, which must be blank (section 1.3.3).
	"CONTLABEL": {Error, "the label field of a continuation line must be blank"},

	// TOOMANYCONT is a statement with more continuation lines than
	// /CONTINUATIONS allows; it is reported at the first line too many.
	"TOOMANYCONT": {Error, "statement has more than %d continuation lines; use /CONTINUATIONS to allow more"},

	// BADLABEL is a label field that holds something other than one to
	// five digits.
	"BADLABEL": {Error, "statement label field holds %q, which is not a label of 1 to 5 digits"},

	// ZEROLABEL is a label of all zeros, which the manual forbids.
	"ZEROLABEL": {Error, "statement label 0 is not allowed"},

	// LINETOOLONG is text past column 132 with /EXTEND_SOURCE, which the
	// manual makes a fatal error (section 1.3.5).
	"LINETOOLONG": {Fatal, "line has text past column %d"},

	// BADINCLUDE is an INCLUDE statement not of the form INCLUDE 'name'.
	"BADINCLUDE": {Error, "an INCLUDE statement must be INCLUDE 'file' or INCLUDE '(module)'"},

	// INCLOPEN is an INCLUDE file that can't be read: fatal, since the
	// statements in it are missing.
	"INCLOPEN": {Fatal, "can't read the INCLUDE file '%s': %s"},

	// INCLDEPTH is INCLUDE files nested too deeply, most likely a file
	// that includes itself.
	"INCLDEPTH": {Fatal, "the INCLUDE files are nested more than %d deep"},

	// The lexer's messages (internal/fortran/lex).

	// NOQUOTE is a character constant with no closing apostrophe before
	// the statement ends.
	"NOQUOTE": {Error, "character constant has no closing apostrophe"},

	// HOLLEND is a Hollerith constant whose count runs past the end of
	// the statement.
	"HOLLEND": {Error, "this Hollerith constant should have %d characters, but the statement ends after %d"},

	// UNRECSTMT is a statement the lexer can't take for any FORTRAN
	// statement: no keyword it knows starts it, and it isn't an
	// assignment.
	"UNRECSTMT": {Error, "statement not recognized"},

	// BADCHAR is a character FORTRAN doesn't use, outside a constant.
	"BADCHAR": {Error, "the character %q is not allowed here"},

	// The parser's messages (internal/fortran/parse).

	// SYNTAX is a statement whose tokens don't follow its grammar: the
	// first argument says what the parser expected, the second what it
	// found instead.
	"SYNTAX": {Error, "expected %s, but found %s"},

	// EXTRATEXT is text after a statement's grammar is complete.
	"EXTRATEXT": {Error, "the statement should end before %s"},

	// NOEND is a program unit the file ends, or the next unit's header
	// starts, without an END statement. It is reported at the end of the
	// unit's last statement.
	"NOEND": {Error, "%s has no END statement"},

	// BADLABELREF is a reference to a statement label that can't be one:
	// more than five digits, or all zeros.
	"BADLABELREF": {Error, "%s is not a statement label, which is 1 to 5 digits, not all zeros"},

	// EMPTYSTMT is a statement label with no statement after it.
	"EMPTYSTMT": {Error, "statement label %d is on no statement"},

	// BADIFSTMT is a logical IF whose statement is one a logical IF may
	// not execute (section 5.7.2): a block IF, a DO, another logical IF,
	// END, or a statement that isn't executable.
	"BADIFSTMT": {Error, "a logical IF can't execute %s"},

	// BADSPECIFIER is a keyword specifier the statement doesn't take:
	// WRITE (6, END=10).
	"BADSPECIFIER": {Error, "%s= is not a specifier of the %s statement"},

	// DUPSPECIFIER is a specifier given twice in one statement.
	"DUPSPECIFIER": {Error, "the %s specifier is given twice"},

	// BADLETTERS is an IMPLICIT statement's range of letters out of
	// alphabetical order (section 4.8): Z-A.
	"BADLETTERS": {Error, "%c-%c is not a range of letters in alphabetical order"},

	// NOF77 is the EXTERNAL statement's *name form, which only /NOF77
	// allows (appendix A.6), and govax has no /NOF77.
	"NOF77": {Error, "the form EXTERNAL *%s is appendix A.6's, for /NOF77, which govax doesn't have"},

	// BADDIRECTIVE is a CDEC$ line naming no general directive (section
	// 10.3).
	"BADDIRECTIVE": {Error, "there is no CDEC$ %s directive; the directives are IDENT, PSECT, TITLE, and SUBTITLE"},

	// Structure declarations (section 4.15).

	// NOBLOCKEND is a STRUCTURE, UNION, or MAP declaration with no END
	// STRUCTURE, END UNION, or END MAP before the unit's END (or before
	// the end of the declaration around it).
	"NOBLOCKEND": {Error, "this %s declaration has no %s statement"},

	// NOBLOCK is END STRUCTURE, END UNION, or END MAP with no declaration
	// of its kind open.
	"NOBLOCK": {Error, "this %s statement ends no %s declaration"},

	// NOTINBLOCK is a declaration, or a field declaration, where it can't
	// be: a MAP outside a UNION, a field in a UNION but no MAP.
	"NOTINBLOCK": {Error, "%s can only be in %s"},

	// FIELDSTMT is a statement inside a structure declaration that
	// declares no field: only type statements, RECORD, PARAMETER, and
	// nested STRUCTURE and UNION declarations may be there.
	"FIELDSTMT": {Error, "%s can't be in a structure declaration, which holds only field declarations"},

	// FILLNAME is %FILL, an unnamed field, outside a structure
	// declaration.
	"FILLNAME": {Error, "%%FILL names an unnamed field, so it can only be in a structure declaration"},

	// Semantic analysis's messages (internal/fortran/sema).

	// Program units.

	// DUPUNIT is a second program unit with a name the file has already
	// given one.
	"DUPUNIT": {Error, "there is already a program unit named %s, at line %d"},

	// TWOMAINS is a second main program in a file.
	"TWOMAINS": {Error, "a file can hold only one main program, and there is one at line %d"},

	// DUPDUMMY is a name twice among a subprogram's dummy arguments.
	"DUPDUMMY": {Error, "%s is already a dummy argument of this %s"},

	// Declarations.

	// DUPDECL is a second type declaration of a name.
	"DUPDECL": {Error, "%s's type is already declared, at line %d"},

	// DUPDIM is a second array declarator for a name.
	"DUPDIM": {Error, "%s's dimensions are already declared, at line %d"},

	// LATEDECL is a declaration after the unit's first executable
	// statement (Figure 1-1).
	"LATEDECL": {Error, "a declaration must come before the executable statements, the first of which is at line %d"},

	// ORDER is a statement out of Figure 1-1's order other than a
	// declaration after an executable statement (LATEDECL): IMPLICIT
	// after a type statement, a type statement after a statement
	// function's definition.
	"ORDER": {Error, "this %s must come before the %s at line %d"},

	// BADLENGTH is a length a type doesn't have: INTEGER*3.
	"BADLENGTH": {Error, "%s*%d is not a data type; %s's lengths are %s"},

	// NOLENGTH is a length after a type keyword that names one: BYTE,
	// DOUBLE PRECISION, DOUBLE COMPLEX.
	"NOLENGTH": {Error, "%s can't be given a length"},

	// NUMLENGTH is an expression or (*) as a numeric type's length.
	"NUMLENGTH": {Error, "a length in parentheses is a CHARACTER length, and this function is %s"},

	// BADCHARLEN is a CHARACTER length that isn't an integer constant
	// expression from 1 to 65535 (section 4.4.2).
	"BADCHARLEN": {Error, "a CHARACTER length must be an integer constant expression from 1 to 65535, or (*)"},

	// PASSEDLEN is a passed length, CHARACTER*(*), on a name that can't
	// take one.
	"PASSEDLEN": {Error, "%s can't have the length (*): only a dummy argument, a function, or a PARAMETER constant can"},

	// IMPLNONE is IMPLICIT NONE in a unit with another IMPLICIT
	// statement (section 4.8).
	"IMPLNONE": {Error, "a program unit with an IMPLICIT NONE statement can't have another IMPLICIT statement, and this one has the %s statement at line %d"},

	// DUPIMPLICIT is a letter given a type by two IMPLICIT rules.
	"DUPIMPLICIT": {Error, "the letter %s already has a type, from the IMPLICIT statement at line %d"},

	// NOTYPE is a name with no type under IMPLICIT NONE.
	"NOTYPE": {Error, "%s has no type: with IMPLICIT NONE, every name must be declared"},

	// DUPSTMTFUNC is a second definition of a statement function.
	"DUPSTMTFUNC": {Error, "the statement function %s is already defined, at line %d"},

	// BADBOUND is an array bound that isn't an arithmetic expression.
	"BADBOUND": {Error, "an array's bound must be an INTEGER or REAL expression"},

	// ADJNOTDUMMY is a bound that isn't constant on an array that isn't
	// a dummy argument: only those can be adjustable.
	"ADJNOTDUMMY": {Error, "the bounds of %s must be constants: only a dummy argument can be an adjustable array"},

	// BOUNDVAR is a name in an adjustable array's bound that isn't a
	// dummy argument or in a common block.
	"BOUNDVAR": {Error, "%s can't be used in an array's bounds: it isn't a dummy argument or in a common block"},

	// BOUNDREF is an array or a function in an array's bound.
	"BOUNDREF": {Error, "%s can't be used in an array's bounds: arrays and functions aren't allowed there"},

	// BOUNDTYPE is a type statement changing the type of a name already
	// used in an array's bounds.
	"BOUNDTYPE": {Error, "the type of %s can't change: it is used in an array's bounds, at line %d"},

	// TOOMANYDIMS is an array declarator of more than seven dimensions.
	"TOOMANYDIMS": {Error, "an array can have at most 7 dimensions, not %d"},

	// ARRAYSIZE is an array too large for the VAX to address.
	"ARRAYSIZE": {Error, "the array %s is too large: it takes more than 2147483647 bytes"},

	// BOUNDORDER is an upper bound less than its lower bound.
	"BOUNDORDER": {Error, "the upper bound %d is less than the lower bound %d"},

	// ASSUMEDSIZE is an upper bound of * anywhere but a dummy array's last
	// dimension.
	"ASSUMEDSIZE": {Error, "only a dummy array's last upper bound can be *"},

	// COMMON and EQUIVALENCE (sections 4.2, 4.6).

	// COMMONDUMMY is a dummy argument in a common block.
	"COMMONDUMMY": {Error, "%s is a dummy argument, so it can't be in a common block"},

	// DUPCOMMON is a name listed in a common block twice, or in two.
	"DUPCOMMON": {Error, "%s is already in %s"},

	// COMMONNAME is a common block with the name of a routine or the main
	// program.
	"COMMONNAME": {Error, "the common block %s can't have the name of %s"},

	// COMMONSIZE is a common block too large for the VAX to address.
	"COMMONSIZE": {Error, "the common block %s is too large: it takes more than 2147483647 bytes"},

	// EQUIVONE is an EQUIVALENCE list of one item.
	"EQUIVONE": {Error, "an EQUIVALENCE list must have at least two items"},

	// EQUIVFORM is an item that isn't a variable, an array, an array
	// element, or a substring of one: a record's field.
	"EQUIVFORM": {Error, "an EQUIVALENCE list can hold only variables, arrays, array elements, and substrings"},

	// EQUIVDUMMY is a dummy argument in an EQUIVALENCE list.
	"EQUIVDUMMY": {Error, "%s is a dummy argument, so it can't be in an EQUIVALENCE list"},

	// EQUIVCONST is a subscript or substring bound that isn't an integer
	// constant expression.
	"EQUIVCONST": {Error, "a subscript or substring bound in an EQUIVALENCE list must be an integer constant expression"},

	// EQUIVRANGE is an element outside its array.
	"EQUIVRANGE": {Error, "this element is outside the array %s"},

	// EQUIVSUBSTR is a substring of a name that isn't CHARACTER.
	"EQUIVSUBSTR": {Error, "%s is %s, so it can't have a substring"},

	// EQUIVSUBRANGE is a substring outside its variable or element.
	"EQUIVSUBRANGE": {Error, "the substring (%d:%d) isn't within %s's %d characters"},

	// EQUIVCONFLICT is a name the lists put in two places: two elements of
	// one array associated, or two substrings of one variable starting at
	// different characters.
	"EQUIVCONFLICT": {Error, "this EQUIVALENCE puts %s in two places at once"},

	// EQUIVBLOCKS is two names in different common blocks associated.
	"EQUIVBLOCKS": {Error, "this EQUIVALENCE ties %s in %s to %s in %s, but they can't share storage"},

	// EQUIVCOMMON is two names of one block associated at places the
	// block's order disagrees with.
	"EQUIVCOMMON": {Error, "this EQUIVALENCE puts %s and %s in different places from those %s gives them"},

	// EQUIVBEFORE is EQUIVALENCE extending a common block before its first
	// element (section 4.6.3).
	"EQUIVBEFORE": {Error, "this EQUIVALENCE puts %s before the start of %s"},

	// Names and expressions.

	// BADUSE is a name used as something it can't be: a variable called
	// as a subroutine, a subroutine's own name given a type.
	"BADUSE": {Error, "%s is %s, so it can't %s"},

	// The DATA statement's messages, and a type statement's initial
	// values' (section 4.3, internal/fortran/sema/data.go). DATACOUNT is
	// lists that don't come out even.
	"DATACOUNT": {Error, "this list has %s"},

	// DATAREPEAT is a repeat count that isn't a positive integer constant.
	"DATAREPEAT": {Error, "a repeat count must be a positive integer constant, or a name of one"},

	// DATAVALUE is a value that isn't a constant; the argument says why.
	"DATAVALUE": {Error, "an initial value must be a constant, and %s"},

	// DATAFORM is an item a list can't hold: an implied-DO list holds
	// array elements, substrings, and other implied-DO lists, no bare names.
	"DATAFORM": {Error, "a DATA list can hold only variables, arrays, array elements, substrings, and implied-DO lists, and an implied-DO list no variable or array by itself"},

	// DATACONST is a subscript or bound that isn't constant.
	"DATACONST": {Error, "a subscript, substring bound, or implied-DO limit in a DATA list must be an integer constant expression"},

	// DATARANGE is an element outside its array.
	"DATARANGE": {Error, "this element is outside the array %s"},

	// DATASUBSTR is a substring of a name not of type CHARACTER, and
	// DATASUBRANGE one that runs outside its variable.
	"DATASUBSTR":   {Error, "%s is %s, so it can't have a substring"},
	"DATASUBRANGE": {Error, "the substring (%d:%d) isn't within %s's %d characters"},

	// DATASIZE is a whole array whose size isn't known.
	"DATASIZE": {Error, "the array %s's size isn't known, so it can't be given initial values"},

	// DATADOVAR is an implied-DO variable that can't be one; DATADOSTEP a
	// step of zero.
	"DATADOVAR":  {Error, "the implied-DO variable %s %s"},
	"DATADOSTEP": {Error, "an implied-DO list's step can't be zero"},

	// DATADUMMY is a dummy argument given an initial value, DATABLANK a
	// name in blank common, and DATABLOCK a name a block data subprogram
	// has no named block for.
	"DATADUMMY": {Error, "%s is a dummy argument, so it can't be given an initial value"},
	"DATABLANK": {Error, "%s is in blank common, which can't be given initial values"},
	"DATABLOCK": {Error, "%s isn't in a named common block, and a block data subprogram initializes only those"},

	// DATATYPE is a constant of a type its item can't be given, and
	// DATACHAR section 4.3's rule for a number given to a CHARACTER item.
	"DATATYPE": {Error, "%s constant can't initialize %s item"},
	"DATACHAR": {Error, "only an integer, octal, or hexadecimal constant from 0 to 255 can initialize a CHARACTER item, and only one of length 1"},

	// DATATWICE is two initial values for one byte.
	"DATATWICE": {Error, "%s has an initial value already, from line %d"},

	// SAVE's messages (section 4.14).
	"SAVEDUMMY":  {Error, "%s is a dummy argument, so it can't be in a SAVE statement"},
	"SAVECOMMON": {Error, "%s is in %s: a SAVE statement names the block, not its names"},
	"SAVEBLANK":  {Error, "blank common can't be in a SAVE statement"},
	"SAVEBLOCK":  {Error, "this unit has no common block /%s/"},

	// EXTERNAL's and INTRINSIC's messages (sections 4.7, 4.9).
	"DUPATTR":      {Error, "%s is %s already"},
	"BADINTRINSIC": {Error, "%s is not one of FORTRAN's intrinsic functions"},

	// BLOCKSTMT is a statement section 4.1 doesn't allow in a block data
	// subprogram.
	"BLOCKSTMT": {Error, "%s can't be in a block data subprogram"},

	// RECURSIVE is a subprogram referring to itself (section 6.2).
	"RECURSIVE": {Error, "%s can't refer to itself: a FORTRAN subprogram is not recursive"},

	// NOTARRAY is subscripts on a name that isn't an array.
	"NOTARRAY": {Error, "%s is not an array, so it can't have subscripts"},

	// NEEDSUBSCRIPT is a whole array where only an element or a scalar
	// may be (section 2.2.3.5).
	"NEEDSUBSCRIPT": {Error, "the array %s can't be used without subscripts here"},

	// SUBSCRIPTS is an array element with the wrong number of subscripts.
	"SUBSCRIPTS": {Error, "the array %s takes %s, not %d"},

	// NOSIZE is an assumed-size array as a whole in an I/O list.
	"NOSIZE": {Error, "the assumed-size array %s has no size, so it can't be written as a whole"},

	// BADOPERAND is an operator applied to a type it doesn't take:
	// .AND. on a REAL.
	"BADOPERAND": {Error, "%s can't be applied to a %s value"},

	// NOTLOGICAL is an IF's condition that is neither LOGICAL nor
	// INTEGER.
	"NOTLOGICAL": {Error, "a condition must be LOGICAL or INTEGER, and this one is %s"},

	// BADTARGET is an assignment to something that can't hold a value.
	"BADTARGET": {Error, "can't assign a value to %s"},

	// INTRANGE is an integer constant, or a constant expression's value,
	// outside its INTEGER type's range.
	"INTRANGE": {Error, "%s is outside %s's range, %d to %d"},

	// REALRANGE is a real constant, or a constant expression's value,
	// too large or too small for its REAL type's format.
	"REALRANGE": {Error, "%s is outside %s's range, about %s"},

	// DIVZERO is a constant expression that divides by zero.
	"DIVZERO": {Error, "this constant expression divides by zero"},

	// UNDEFPOW is a constant expression raising zero to a power of zero
	// or less, which has no value.
	"UNDEFPOW": {Error, "this constant expression raises zero to the power %d, which has no value"},

	// Constants and constant expressions (section 4.11, appendix A.4).

	// NOTCONST is a PARAMETER value that isn't a constant expression;
	// the argument says why.
	"NOTCONST": {Error, "a PARAMETER constant's value must be a constant expression, and %s"},

	// DUPPARAM is a second definition of a constant.
	"DUPPARAM": {Error, "%s is already a constant, defined at line %d"},

	// TYPEAFTER is a type statement for a constant after the PARAMETER
	// statement that defined it.
	"TYPEAFTER": {Error, "%s's type must be declared before the PARAMETER statement that defines it, at line %d"},

	// OLDPARAMTYPE is a declared type for a constant appendix A.4's
	// PARAMETER defines, which takes its value's type.
	"OLDPARAMTYPE": {Error, "%s can't be given a type: PARAMETER without parentheses gives a constant its value's type (the %s is at line %d)"},

	// PARAMTYPE is a PARAMETER value that can't be converted to its
	// constant's type.
	"PARAMTYPE": {Error, "the constant %s is %s, and can't take the value of %s expression"},

	// BADCOMPLEX is a part of a COMPLEX constant that isn't a constant or
	// a constant's name (section 2.2.1.3).
	"BADCOMPLEX": {Error, "each part of a COMPLEX constant must be an INTEGER or REAL constant, or the name of one"},

	// INTRINARGS is an intrinsic function given the wrong number of
	// arguments.
	"INTRINARGS": {Error, "the intrinsic function %s takes %s, not %d"},

	// BADARG is an intrinsic function's argument of a type or value it
	// doesn't take.
	"BADARG": {Error, "the intrinsic function %s's arguments must be %s, and this one is %s"},

	// BADTYPELESS is an octal or hexadecimal constant whose bits are a
	// reserved operand in the floating type it is given.
	"BADTYPELESS": {Error, "this constant's bits are a reserved operand, not a %s value"},

	// Statements and blocks.

	// MAINRETURN is RETURN in a main program (section 5.9).
	"MAINRETURN": {Error, "a main program can't have a RETURN statement; use STOP or END"},

	// NOIF is ELSE IF, ELSE, or END IF outside any IF block.
	"NOIF": {Error, "this %s statement is not in an IF block"},

	// NOENDIF is an IF block the unit's END reaches before its END IF.
	"NOENDIF": {Error, "this IF block has no END IF statement"},

	// ELSEORDER is ELSE IF or ELSE after the block's ELSE, which only END
	// IF may follow (section 5.7.3).
	"ELSEORDER": {Error, "this %s statement comes after the IF block's ELSE statement, at line %d"},

	// NODO is a DO loop whose terminal statement never comes after it.
	"NODO": {Error, "no statement after this DO statement has the label %d"},

	// DOTERM is a statement that may not end a DO loop (section 5.3.1).
	"DOTERM": {Error, "a DO loop can't end with %s"},

	// DONEST is a DO loop or IF block that doesn't nest wholly inside
	// the construct around it (sections 5.3.1.2 and 5.7.3.1).
	"DONEST": {Error, "the %s at line %d must end before %s"},

	// DOVAR is a DO variable that isn't an INTEGER or REAL variable.
	"DOVAR": {Error, "a DO loop's variable must be an INTEGER or REAL variable, and %s is %s"},

	// DOVARSET is a change to an active DO loop's control variable.
	"DOVARSET": {Error, "%s is the control variable of the DO loop at line %d, which can't change it"},

	// DOSTEP0 is a DO loop whose increment is the constant zero.
	"DOSTEP0": {Error, "a DO loop's increment can't be zero"},

	// Labels.

	// DUPLABEL is a statement label defined twice in a unit.
	"DUPLABEL": {Error, "statement label %d is already on the statement at line %d"},

	// UNDEFLBL is a jump to a label no statement in the unit has.
	"UNDEFLBL": {Error, "statement label %d is never defined"},

	// NOTEXEC is a jump to a statement that can't be jumped to.
	"NOTEXEC": {Error, "statement label %d is on %s, which a jump can't go to"},

	// INTOBLOCK is a jump into an IF block from outside it, or from one
	// of its blocks to another (section 5.7.3.1).
	"INTOBLOCK": {Error, "this jump to label %d goes into an IF block from outside it"},

	// SHAREDTERM is a jump to the terminal statement nested DO loops
	// share from outside the innermost of them (section 5.3.1.3).
	"SHAREDTERM": {Error, "statement label %d ends more than one DO loop, so only the innermost loop can jump to it"},

	// References between the file's units.

	// FUNCTYPE is a reference to a function of the file whose type it
	// doesn't agree with.
	"FUNCTYPE": {Error, "the function %s is %s (line %d), but is used here as %s"},

	// ARGCOUNT is a reference to a routine of the file with a different
	// number of arguments from its dummy arguments.
	"ARGCOUNT": {Error, "%s takes %s (line %d), but is given %d here"},
}

// Known reports whether ident is a message in the table, and its
// severity.
func Known(ident string) (Severity, bool) {
	k, ok := messages[ident]

	return k.Severity, ok
}

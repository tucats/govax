package diag

// kind is one entry of the message table: a message's severity and the
// format of its text (fmt's verbs, filled in from Report's arguments).
type kind struct {
	Severity Severity
	Format   string
}

// messages is every message the compiler can report, by identifier. The
// identifiers and wording are govax's own (docs/planning/PHASE-74 - FORTRAN
// plan.md, Decision 19): they follow the style of VAX FORTRAN's (a short
// identifier, lowercase text) without copying them. Each phase adds the
// messages its passes need.
var messages = map[string]kind{
	// NOTIMPL is a statement or feature the compiler recognizes but can't
	// compile yet. It is an error, so a program using it never runs with
	// the feature silently missing (Decision 1).
	"NOTIMPL": {Error, "%s is not implemented yet"},

	// NOLISTING is a qualifier that only says what a listing holds, on a command
	// that doesn't ask for one.
	"NOLISTING": {Informational, "%s has no effect without /LIST"},

	// BADCHECK is an OPTIONS statement's /CHECK with a keyword that isn't one.
	"BADCHECK": {Error, "%s"},

	// SUBSCRIPTRANGE is a constant subscript outside its dimension.
	"SUBSCRIPTRANGE": {Warning, "subscript %d of %s is past its %s bound, %d"},

	// What /WARNINGS's keywords ask for (diag.Warnings; sema/warnings.go).
	"UNDECLARED":  {Warning, "%s is used but not typed, and its first letter makes it %s"},
	"UNUSEDNAME":  {Warning, "%s is declared but never used"},
	"UNUSEDDUMMY": {Warning, "the dummy argument %s is never used"},
	"UNUSEDLABEL": {Warning, "statement label %s is never used"},

	// EXTENSION is something in the program that FORTRAN-77 doesn't have, as
	// /STANDARD asks (sema/standard.go): a remark, so a program flagged still compiles.
	"EXTENSION": {Informational, "extension to FORTRAN-77: %s"},

	// DEBUGOPTIMIZE is /DEBUG with /OPTIMIZE, which is allowed but may surprise.
	"DEBUGOPTIMIZE": {Informational, "under /OPTIMIZE the debugger may find a variable out of date, its value in a register; use /NOOPTIMIZE to debug"},

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

	// Free source form's (source/free.go; the standard's 3.3.1).

	// LONEAMP is a line holding only an &, which 3.3.1.3 forbids.
	"LONEAMP": {Error, "a line may not hold only an &"},

	// MISSINGAMP is a character constant continued onto a line that doesn't
	// start with the & that says where it goes on (3.3.1.3.2).
	"MISSINGAMP": {Error, "a character constant continued from the line before must go on after an & on this line"},

	// CONTNOLINE is a statement continued by an & on a file's last statement line.
	"CONTNOLINE": {Error, "the & continues the statement, but the file has no line after it"},

	// EMPTYSEMI is a ; with no statement before it on its line (3.3.1.2).
	"EMPTYSEMI": {Error, "a ; must follow a statement"},

	// LABELBLANK is a free-form label run into what follows it (3.3.1).
	"LABELBLANK": {Error, "a blank must follow a statement label"},

	// DIRINSTMT is a !DEC$ directive among a continued statement's lines.
	"DIRINSTMT": {Error, "a directive may not come between the lines of a continued statement"},

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

	// NOQUOTE is a character constant with no closing apostrophe (or, under
	// /F90, quotation mark) before
	// the statement ends.
	"NOQUOTE": {Error, "character constant has no closing %s"},

	// HOLLEND is a Hollerith constant whose count runs past the end of
	// the statement.
	"HOLLEND": {Error, "this Hollerith constant should have %d characters, but the statement ends after %d"},

	// RAD50END is a RADIX-50 constant whose count runs past the end of the
	// statement.
	"RAD50END": {Error, "this RADIX-50 constant should have %d characters, but the statement ends after %d"},

	// UNRECSTMT is a statement the lexer can't take for any FORTRAN
	// statement: no keyword it knows starts it, and it isn't an
	// assignment.
	"UNRECSTMT": {Error, "statement not recognized"},

	// BADCHAR is a character FORTRAN doesn't use, outside a constant.
	"BADCHAR": {Error, "the character %q is not allowed here"},

	// BLANKINNAME is a blank inside a name, keyword, constant, or operator of
	// several characters in free source form, where blanks are significant
	// (the standard's 3.3.1; docs/planning/PHASE-99 - Fortran 90.md, Decision 3).
	"BLANKINNAME": {Error, "a blank may not stand inside %s in free source form"},

	// NEEDBLANK is two names, keywords, constants, or labels written together
	// in free source form, where a blank must separate them (3.3.1).
	"NEEDBLANK": {Error, "a blank must separate %s from %s in free source form"},

	// BADKIND is a kind parameter after an underscore that is neither digits
	// nor a name (R404: 1.5_8, 2_K).
	"BADKIND": {Error, "a kind parameter after _ must be digits or a name"},

	// NOTCONSTRUCT is a construct name (NAME:) before a statement that
	// starts no construct: only DO, DO WHILE, a block IF, and SELECT CASE
	// may have one (the standard's 8.1).
	"NOTCONSTRUCT": {Error, "only a DO, block IF, or SELECT CASE statement may have a construct name, not this %s statement"},

	// The parser's messages (internal/fortran/parse).

	// ENDKIND is an END statement naming a kind of unit (END FUNCTION) other
	// than the one it ends (the standard's R1103, R1218, R1222).
	"ENDKIND": {Error, "the END %s statement can't end %s"},

	// ENDNAME is an END statement whose name isn't its unit's.
	"ENDNAME": {Error, "the END statement names %s, but it ends %s"},

	// ENDNONAME is an END statement with a name, ending a unit that has none:
	// a main program without a PROGRAM statement, or a BLOCK DATA without a
	// name.
	"ENDNONAME": {Error, "the END statement names %s, but %s has no name"},

	// ENDNOKIND is the plain END of a procedure inside another, which must say
	// END FUNCTION or END SUBROUTINE (the standard's 12.5.2.2 and 12.5.2.3).
	"ENDNOKIND": {Error, "the END of %s, which is inside another unit, must be END %s"},

	// Fortran 90's units inside units (parse/units.go; docs/planning/PHASE-103 -
	// Fortran 90 procedures and interfaces.md, finding 1).

	// EMPTYCONTAINS is CONTAINS with no procedure after it (R210, R212).
	"EMPTYCONTAINS": {Error, "a CONTAINS statement must be followed by at least one procedure"},

	// NESTEDCONTAINS is CONTAINS in an internal procedure (12.1.2.2).
	"NESTEDCONTAINS": {Error, "%s is an internal procedure, which can't have a CONTAINS statement of its own"},

	// ENTRYINTERNAL is ENTRY in an internal procedure (12.1.2.2).
	"ENTRYINTERNAL": {Error, "%s is an internal procedure, which can't have an ENTRY statement"},

	// AFTERCONTAINS is a statement between CONTAINS and the unit's END that
	// isn't an internal procedure's.
	"AFTERCONTAINS": {Error, "%s can't follow CONTAINS: only procedures and the unit's END can"},

	// BADINBODY is a statement an interface body can't hold (R1204).
	"BADINBODY": {Error, "%s can't be in an interface body, which holds only a procedure's declarations"},

	// INTERFACESTMT is a statement in an interface block outside its bodies.
	"INTERFACESTMT": {Error, "%s can't be in an interface block outside an interface body"},

	// NOINTERFACEEND is an interface block its unit's END reaches.
	"NOINTERFACEEND": {Error, "this interface block has no END INTERFACE statement"},

	// NOINTERFACE is END INTERFACE with no interface block open.
	"NOINTERFACE": {Error, "this END INTERFACE statement ends no interface block"},

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

	// BADWHERESTMT is a WHERE statement whose statement isn't an assignment
	// (Fortran 90's R740: only an assignment may be masked).
	"BADWHERESTMT": {Error, "a WHERE statement can execute only an assignment, not %s"},

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
	"NOF77": {Error, "the form EXTERNAL *%s is appendix A.6's, which only /NOF77 has"},

	// BADDIRECTIVE is a CDEC$ line naming no general directive (section
	// 10.3).
	"BADDIRECTIVE": {Error, "there is no CDEC$ %s directive; the directives are IDENT, PSECT, TITLE, SUBTITLE, OPTIONS, and END OPTIONS"},

	// DIRSTRING is a string for CDEC$ IDENT, TITLE, or SUBTITLE that is too
	// long or holds a character that can't be printed (section 10.3).
	"DIRSTRING": {Error, "the string of a CDEC$ %s directive must be at most %d printable characters"},

	// PSECTATTR is a word CDEC$ PSECT doesn't take (section 10.3.2).
	"PSECTATTR": {Error, "%s is not an attribute of CDEC$ PSECT, which takes LCL, GBL, [NO]WRT, [NO]SHR, and ALIGN=n"},

	// PSECTVALUE is CDEC$ PSECT's attribute with a value it doesn't take.
	"PSECTVALUE": {Error, "the CDEC$ PSECT attribute %s takes no value"},

	// PSECTALIGN is CDEC$ PSECT's ALIGN= without a number from 0 to 9.
	"PSECTALIGN": {Error, "the ALIGN= attribute of CDEC$ PSECT takes one number from 0 to 9"},

	// PSECTCLASH is one unit choosing two ways for one attribute of a
	// block: LCL with GBL (section 10.3.2), NOWRT with WRT.
	"PSECTCLASH": {Error, "the CDEC$ PSECT attribute %s can't go with %s for %s"},

	// PSECTBLOCK is CDEC$ PSECT naming a block the unit doesn't declare.
	"PSECTBLOCK": {Error, "%s is not a common block of this program unit, so CDEC$ PSECT can't change it"},

	// OPTIONSFIRST is an OPTIONS statement inside a unit (section 1.4.3).
	"OPTIONSFIRST": {Error, "an OPTIONS statement must come before the statements of its program unit"},

	// OPTIONSBAD is an option of CDEC$ OPTIONS that isn't one (the HP manual's
	// 14.14).
	"OPTIONSBAD": {Error, "in CDEC$ OPTIONS: %s"},

	// OPTIONSPLACE is a CDEC$ OPTIONS or END OPTIONS before a unit's SUBROUTINE,
	// FUNCTION, or BLOCK DATA statement, or after the file's last unit.
	"OPTIONSPLACE": {Error, "a CDEC$ OPTIONS or END OPTIONS directive must come after the %s statement"},

	// OPTIONSLATE is a CDEC$ OPTIONS or END OPTIONS after the unit's first
	// executable statement.
	"OPTIONSLATE": {Error, "a CDEC$ OPTIONS or END OPTIONS directive must come before the executable statements"},

	// OPTIONSSTRUCT is a group beginning or ending inside a structure
	// declaration.
	"OPTIONSSTRUCT": {Error, "a CDEC$ OPTIONS or END OPTIONS directive can't be inside a structure declaration"},

	// OPTIONSOPEN is a CDEC$ OPTIONS with no END OPTIONS before its unit's END.
	"OPTIONSOPEN": {Error, "this CDEC$ OPTIONS has no END OPTIONS in its program unit"},

	// NOOPTIONS is a CDEC$ END OPTIONS with no group to end.
	"NOOPTIONS": {Error, "this CDEC$ END OPTIONS has no CDEC$ OPTIONS"},

	// OPTIONSDEPTH is groups nested too deep.
	"OPTIONSDEPTH": {Error, "groups of CDEC$ OPTIONS can nest only %d levels"},

	// PSECTSMALL is a CDEC$ PSECT ALIGN= smaller than the block's layout needs.
	"PSECTSMALL": {Error, "%s needs ALIGN=%d for its aligned names, but CDEC$ PSECT gives it ALIGN=%d"},

	// ALIGNFIELD and ALIGNCOMMON are /WARN=ALIGNMENT's remarks.
	"ALIGNFIELD":  {Informational, "the field %s, at offset %d, is not on its natural boundary of %d"},
	"ALIGNCOMMON": {Informational, "%s, at offset %d in %s, is not on its natural boundary of %d"},

	// COMMONLAYOUT is a block two units declare alike under different alignment
	// settings, so its names don't match up.
	"COMMONLAYOUT": {Warning, "%s is declared here as at line %d, but aligned differently, so their names are at different places"},

	// PSECTDIFFER is a block with other attributes than another unit of
	// the file gives it, which section 10.3.2 forbids.
	"PSECTDIFFER": {Error, "the attributes of %s here are %s, but at line %d they are %s; every unit must change them alike"},

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

	// Structure declarations' fields and layout (sections 4.15.1 to
	// 4.15.3; sema/structure.go).

	// STRUCTNAME is an outermost structure declaration without a name,
	// which only a nested one may leave out.
	"STRUCTNAME": {Error, "this structure declaration needs a name: STRUCTURE /name/"},

	// STRUCTFIELDS is a field list where there can't be one, or none
	// where there must be: the text says which.
	"STRUCTFIELDS": {Error, "%s"},

	// DUPSTRUCT is a second structure of one name in a unit.
	"DUPSTRUCT": {Error, "a structure named %s is already declared, at line %d"},

	// STRUCTEMPTY is a structure declaration with no fields.
	"STRUCTEMPTY": {Error, "this structure declaration declares no fields"},

	// NOSTRUCT is a RECORD statement naming a structure not declared
	// before it.
	"NOSTRUCT": {Error, "there is no structure named %s declared before this statement"},

	// RECURSTRUCT is a RECORD statement inside a structure's declaration
	// naming that structure, or one it is nested in.
	"RECURSTRUCT": {Error, "the structure %s can't contain itself"},

	// DUPFIELD is a second field of one name at one level of a
	// structure (a union's maps' fields are at the structure's level).
	"DUPFIELD": {Error, "this structure already has a field named %s, declared at line %d"},

	// FIELDDIM is a field's array bound that isn't a constant expression:
	// fields can't be adjustable or assumed-size arrays.
	"FIELDDIM": {Error, "the bounds of the field %s must be constants"},

	// FIELDLEN is a CHARACTER*(*) field: a field's length must be known.
	"FIELDLEN": {Error, "the field %s can't have a passed length, CHARACTER*(*)"},

	// FIELDOP is a field named like an operator or logical constant,
	// which a reference can't reach: R.EQ.X is a comparison (section
	// 2.2.5.2's note).
	"FIELDOP": {Warning, "a field named %s can't be referenced, since .%[1]s. is an operator or constant"},

	// FILLINIT is %FILL with initial values: an unnamed field is always
	// undefined (section 4.15.1).
	"FILLINIT": {Error, "%%FILL can't be given initial values"},

	// STRUCTSIZE is a structure too long for a longword to measure.
	"STRUCTSIZE": {Error, "the structure %s is too large: it takes more than 2147483647 bytes"},

	// RECORDTYPED is a RECORD statement naming a name a type statement
	// has given a type (section 4.13: a record's form is its structure).
	"RECORDTYPED": {Error, "%s's type is declared at line %d, so it can't be a record"},

	// DUPRECORD is a name two RECORD statements name.
	"DUPRECORD": {Error, "%s is already a record, declared at line %d"},

	// NOTRECORD is a field reference to a name that isn't a record.
	"NOTRECORD": {Error, "%s is %s, not a record, so it has no field %s"},

	// NOFIELD is a field reference naming a field its structure doesn't
	// have.
	"NOFIELD": {Error, "the structure %s has no field named %s"},

	// FIELDNOTRECORD is a field reference that goes on past a field that
	// isn't a substructure.
	"FIELDNOTRECORD": {Error, "the field %s is %s, not a substructure, so it has no field %s"},

	// FIELDSUBS is a field reference whose subscripts don't fit the
	// field's dimensions: too many or too few, or any for a field that
	// isn't an array.
	"FIELDSUBS": {Error, "the field %s takes %s, not %d"},

	// FIELDWHOLE is an array field without subscripts where a single
	// item must be.
	"FIELDWHOLE": {Error, "the field %s is an array, and can't be used without subscripts here"},

	// AGGREGATE is a record or substructure as a whole where a value must
	// be: section 2.2.5.2 has no operations on them but assignment.
	"AGGREGATE": {Error, "%s is a record as a whole, which only an assignment, an argument, or an unformatted I/O list can use"},

	// RECORDASSIGN is an assignment to a record of anything but a record of
	// the same structure declaration (section 3.4).
	"RECORDASSIGN": {Error, "%s can't be assigned to %s, which is a record of the structure %s: both must be of one structure"},

	// AGGREGATEIO is a record in a formatted, list-directed, or NAMELIST I/O
	// list (section 2.2.5.2).
	"AGGREGATEIO": {Error, "%s is a record as a whole, which only an unformatted I/O list can hold"},

	// RECORDIOSIZE is a record in an I/O list too long for its descriptor's
	// length, a word.
	"RECORDIOSIZE": {Error, "%s takes %d bytes, too long for an I/O list item, which can take %d"},

	// RECORDARG is an argument and a dummy argument of the file's routine
	// that aren't both records of one form (section 6.1.1).
	"RECORDARG": {Error, "argument %d of %s is %s, but its dummy argument %s (line %d) is %s: a record goes only to a record of the same form"},

	// Program units.

	// DUPUNIT is a second program unit or entry point with a name the file
	// has already given one.
	"DUPUNIT": {Error, "there is already a program unit or entry point named %s, at line %d"},

	// TWOMAINS is a second main program in a file.
	"TWOMAINS": {Error, "a file can hold only one main program, and there is one at line %d"},

	// DUPDUMMY is a name twice among a subprogram's dummy arguments.
	"DUPDUMMY": {Error, "%s is already a dummy argument of this %s"},

	// ENTRYUNIT is an ENTRY statement in a main program (section 6.2.4).
	"ENTRYUNIT": {Error, "an ENTRY statement can only be in a FUNCTION or SUBROUTINE subprogram"},

	// ENTRYBLOCK is an ENTRY statement inside a block IF or a DO loop
	// (section 6.2.4).
	"ENTRYBLOCK": {Error, "an ENTRY statement can't be inside a block, and this one is inside the %s at line %d"},

	// ENTRYRETURN is a * among the dummy arguments of a function's ENTRY
	// statement (section 6.2.4).
	"ENTRYRETURN": {Error, "alternate returns can only be in a subroutine's ENTRY statements"},

	// FUNCRETURN is a * among a FUNCTION statement's dummy arguments, or
	// RETURN i in a function (sections 5.9 and 6.1.1.5).
	"FUNCRETURN": {Error, "a function can't have alternate returns"},

	// ENTRYLATE is an entry point's name or a dummy argument used in an
	// executable statement before the ENTRY statement that first names it
	// (section 6.2.4).
	"ENTRYLATE": {Error, "%s can't be used before the ENTRY statement that names it, at line %d"},

	// ENTRYTYPE is a function's entry point of a type that can't share its
	// value's place with the function's (section 6.2.4.1).
	"ENTRYTYPE": {Error, "the entry point %s, %s, can't go with the function %s, %s: they must all be up to 8 bytes long and not CHARACTER, or all REAL*16 and COMPLEX*16, or all CHARACTER of one length"},

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

	// TWOLENGTHS is a CHARACTER function with a length after CHARACTER and
	// another after its name (section 6.2.2.2).
	"TWOLENGTHS": {Error, "the CHARACTER function %s has its length already"},

	// Fortran 90's declarations (docs/planning/PHASE-100 - Fortran 90
	// declarations and kinds.md, subtask 3). BADATTR is a word in a type
	// declaration's attribute list that is no attribute; SLASHINIT DEC's
	// initial values in slashes after a ::, where only = may give one (the HP
	// manual's rule, and R504's). ATTRTWICE is an attribute a list gives
	// twice, ATTRPAIR two a list gives that can't go together, and
	// ATTRCONFLICT an attribute a name can't have with one it has already,
	// all by the standard's 5.1 (sema/attrtable.go's table). ATTRAGAIN is
	// INTENT, OPTIONAL, or TARGET given a name twice, ATTRDUMMY INTENT or
	// OPTIONAL on a name that isn't a dummy argument; NOTINMODULE
	// PUBLIC or PRIVATE outside a module (R510's constraint). PARAMNOVALUE is
	// a PARAMETER attribute's name with no = value; INITTYPE a value given
	// with = that an assignment couldn't store in the name (7.5.1.4); and
	// FIELDATTR attributes on a structure's field declaration.
	"BADATTR":      {Error, "%s is not an attribute a declaration can give"},
	"SLASHINIT":    {Error, "after ::, an initial value is written = value, not in slashes"},
	"ATTRTWICE":    {Error, "the %s attribute is in this list twice"},
	"ATTRPAIR":     {Error, "the %s and %s attributes can't go together"},
	"ATTRCONFLICT": {Error, "%s has the %s attribute, so it can't have the %s attribute too"},
	"ATTRAGAIN":    {Error, "%s has the %s attribute already"},
	"ATTRDUMMY":    {Error, "%s isn't a dummy argument, so it can't have the %s attribute"},
	"NOTINMODULE":  {Error, "%s is allowed only in a module"},
	"PARAMNOVALUE": {Error, "the constant %s needs a value: = and a constant expression"},
	"INITTYPE":     {Error, "%s value can't initialize %s item, which an assignment couldn't store it in"},
	"FIELDATTR":    {Error, "a structure's field declaration can't give attributes"},

	// Fortran 90's kinds (docs/planning/PHASE-100 - Fortran 90 declarations
	// and kinds.md, subtask 4; sema/kinds.go). NOSUCHKIND is a kind its type
	// doesn't have, NOKIND a selector on a type keyword that names one type
	// (DOUBLE PRECISION), KINDNOTCONST a selector's kind that isn't a
	// constant integer expression, at least 0 (R505). KINDNAME is a constant's
	// kind parameter that names no INTEGER constant (R404), KINDEXPONENT one
	// after a real constant with a D or Q exponent, which names its type
	// already (R413), and KINDCONST one after a constant that can't have one.
	"NOSUCHKIND":   {Error, "%s has no kind %d; its kinds are %s"},
	"NOKIND":       {Error, "%s can't be given a kind"},
	"KINDNOTCONST": {Error, "a kind must be a constant integer expression, 0 or more"},
	"KINDNAME":     {Error, "the kind parameter %s must be digits or the name of an INTEGER constant"},
	"KINDEXPONENT": {Error, "%s has a D or Q exponent, which gives it its type, so it can't have a kind parameter"},
	"KINDCONST":    {Error, "a constant of this form can't have a kind parameter"},

	// BADCHARLEN is a CHARACTER length that isn't an integer constant
	// expression from 1 to 65535 (section 4.4.2).
	"BADCHARLEN": {Error, "a CHARACTER length must be an integer constant expression from 1 to 65535, or (*)"},

	// PASSEDLEN is a passed length, CHARACTER*(*), on a name that can't
	// take one.
	"PASSEDLEN": {Error, "%s can't have the length (*): only a dummy argument, a function's own name, or a PARAMETER constant can"},

	// STMTFUNCLEN is a statement function's dummy argument named for a
	// passed-length name.
	"STMTFUNCLEN": {Error, "the statement function's dummy argument %s can't have the length (*)"},

	// STMTFUNCLATE is a reference to a statement function in a definition
	// before its own (section 6.2.1).
	"STMTFUNCLATE": {Error, "the statement function %s isn't defined until line %d, and a statement function can use only those defined before it"},

	// STMTFUNCARG is a statement function's actual argument of another type
	// from its dummy argument (section 6.2.1).
	"STMTFUNCARG": {Error, "argument %d of the statement function %s is %s, but its dummy argument %s is %s"},

	// NULLINTRINARG and NULLSTMTARG are an argument left out of a reference to
	// an intrinsic function or a statement function, neither of which is
	// called with an argument list a routine could look at.
	"NULLINTRINARG": {Error, "an argument of the intrinsic function %s can't be left out"},

	// Keyword arguments to an intrinsic function (docs/planning/PHASE-100 -
	// Fortran 90 declarations and kinds.md, subtask 6; sema/intrinsickeys.go):
	// a keyword the function hasn't, a dummy given twice, an argument without
	// a keyword after one with, and a required argument left out.
	//
	// Since Phase 103 (sema/optional.go) the same four are said of a reference
	// through an explicit interface, whose first argument names the procedure
	// rather than "the intrinsic function X".
	"KEYWORDNAME":  {Error, "%s has no argument %s; its arguments are %s"},
	"KEYWORDTWICE": {Error, "the argument %s of %s is given twice"},
	"KEYWORDORDER": {Error, "an argument of %s without a keyword can't follow one with a keyword"},
	"MISSINGARG":   {Error, "%s needs its argument %s"},
	"NULLSTMTARG":   {Error, "an argument of the statement function %s can't be left out"},

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
	"BADBOUND": {Error, "%s must be an INTEGER or REAL expression"},

	// ADJNOTDUMMY is a bound that isn't constant on an array that isn't
	// a dummy argument: only those can be adjustable.
	"ADJNOTDUMMY": {Error, "the bounds of %s must be constants: only a dummy argument can be an adjustable array"},

	// AUTOATTR is an automatic array (a subprogram's own array whose bounds
	// aren't constants, docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
	// finding 7) in COMMON, EQUIVALENCE, DATA, NAMELIST, SAVE, or STATIC: its
	// storage is made at each entry, so none of those can hold it (the Fortran
	// 90 standard's 5.1.2.4.1 and 5.2.4).
	"AUTOATTR": {Error, "%s is an automatic array, whose bounds aren't constants, so it can't %s"},

	// AUTOLENATTR is AUTOATTR for a subprogram's own CHARACTER name whose
	// length isn't constant (CHARACTER(LEN=N) :: S; finding 7), which is an
	// automatic object too.
	"AUTOLENATTR": {Error, "%s is an automatic object, whose CHARACTER length isn't constant, so it can't %s"},

	// LENNOTDUMMY is a CHARACTER length that isn't constant in a main program
	// or a block data subprogram, which can have no automatic objects (the
	// Fortran 90 standard's 5.1.1.5).
	"LENNOTDUMMY": {Error, "the CHARACTER length of %s must be constant: only a subprogram's names can have a length that isn't"},

	// BOUNDVAR is a name in an adjustable array's bound that isn't a
	// dummy argument or in a common block.
	"BOUNDVAR": {Error, "%s can't be used in %s: it isn't a dummy argument or in a common block"},

	// BOUNDREF is an array or a function in an array's bound.
	"BOUNDREF": {Error, "%s can't be used in %s: arrays and functions aren't allowed there"},

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

	// VOLATILE's messages (section 4.16, the HP manual's 5.19).
	"VOLATILEBLANK": {Error, "blank common can't be in a VOLATILE statement"},
	"VOLATILEGROUP": {Error, "%s is a NAMELIST group, which can't be in a VOLATILE statement"},

	// AUTOMATIC's and STATIC's messages (the HP manual's 5.3). A name in
	// either statement but once; none is a dummy argument; and what is static
	// by its nature (in a common block, in a SAVE statement, or with an initial
	// value) can't be AUTOMATIC, which the name's EQUIVALENCE group shares. A
	// NAMELIST group's block holds its names' addresses in read-only data, which
	// an AUTOMATIC variable's is not known in time for (ours).
	"DUPSTORAGE":     {Error, "%s is named in a %s statement already (line %d)"},
	"STORAGEDUMMY":   {Error, "%s is a dummy argument, so it can't be in a %s statement"},
	"AUTOCOMMON":     {Error, "%s is in %s, so it can't be AUTOMATIC"},
	"AUTOSAVE":       {Error, "%s is in a SAVE statement, so it can't be AUTOMATIC"},
	"AUTOINIT":       {Error, "%s has an initial value, so it can't be AUTOMATIC"},
	"AUTOEQUIVALENT": {Error, "%s is AUTOMATIC, and EQUIVALENCE ties it to %s, which is named in a STATIC statement"},
	"ASSOCFRAME":     {Warning, "the associated variable %s is in the frame, which a RETURN gives up while the file stays open"},

	// EXTERNAL's and INTRINSIC's messages (sections 4.7, 4.9).
	"DUPATTR":      {Error, "%s is %s already"},
	"BADINTRINSIC": {Error, "%s is not one of FORTRAN's intrinsic functions"},

	// BLOCKSTMT is a statement section 4.1 doesn't allow in a block data
	// subprogram.
	"BLOCKSTMT": {Error, "%s can't be in a block data subprogram"},

	// RECURSIVE is a subprogram referring to itself (section 6.2), which only
	// a RECURSIVE one may do (the HP Fortran manual's 8.5.1.1), and a statement
	// function never.
	"RECURSIVE": {Error, "%s can't refer to itself: it isn't a RECURSIVE subprogram"},

	// RESULT's messages (the HP Fortran manual's 8.5.2.1 and 8.11; docs/planning/PHASE-89 -
	// reentrant FORTRAN.md). RESULTSUB is RESULT where there is no function,
	// RESULTSAME a result named for its own function or entry point, RESULTNAME
	// one that is another name the unit gives meaning to already, and RESULTDECL
	// a declaration of a function's name when RESULT names its value.
	"RESULTSUB":  {Error, "a subroutine has no result: RESULT is for a FUNCTION statement or a function's ENTRY"},
	"RESULTSAME": {Error, "the result of %s must be named for something else"},
	"RESULTNAME": {Error, "%s can't be a result's name: it is %s"},
	"RESULTDECL": {Error, "%s can't be declared: its RESULT, %s, has its type and its value"},

	// NOTARRAY is subscripts on a name that isn't an array.
	"NOTARRAY": {Error, "%s is not an array, so it can't have subscripts"},

	// NEEDSUBSCRIPT is a whole array where only an element or a scalar
	// may be (section 2.2.3.5).
	"NEEDSUBSCRIPT": {Error, "the array %s can't be used without subscripts here"},

	// Fortran 90's array expressions (docs/planning/PHASE-102 - Fortran 90
	// array expressions.md). NOTSCALAR is an array-valued expression where
	// a scalar must be; NOTCONFORM two arrays of different shapes in one
	// operation or assignment (the standard's 7.1.5); ZEROSTRIDE a
	// triplet's stride of zero; SECTIONEND a section of an assumed-size
	// array whose last dimension has no upper bound (6.2.2.3).
	"NOTSCALAR":  {Error, "an array is used where a scalar is required"},
	"NOTCONFORM": {Error, "the shapes %s and %s don't conform"},
	"ZEROSTRIDE": {Error, "a subscript triplet's stride can't be zero"},
	"NOSHAPE":    {Error, "the assumed-size array %s has no shape, so it can't be used as a whole here"},
	"SECTIONEND": {Error, "a section of the assumed-size array %s needs an upper bound in its last dimension"},
	"VECTORRANK": {Error, "a vector subscript must be an array of rank one, not rank %d"},

	// Array constructors and constants (Phase 102, subtask 7).
	"CONSTRUCTTYPE": {Error, "an array constructor's values must have one type, and this one is %[2]s where the first is %[1]s"},
	"CONSTRUCTLEN":  {Error, "an array constructor's CHARACTER values must have one length, and this one has %[2]d where the first has %[1]d"},
	"CONSTRUCTSIZE": {Error, "an array constant can't have more than %d elements"},
	"CTORDOBOUND":   {Error, "an implied-DO list's bounds and step must be INTEGER"},
	"CTORDOVAR":     {Error, "an array constructor's implied-DO variable %s must be INTEGER"},
	"BADRESHAPE":    {Error, "this RESHAPE can't be computed: %s"},
	"ARRAYARG":      {Error, "%s's %s must be an array"},
	"DIMRANGE":      {Error, "%s's DIM, %s, isn't a dimension of an array of rank %d"},
	"SHAPEVALUE":    {Error, "%s has %d elements, and its value %d"},
	"MERGEARGS":     {Error, "the TSOURCE and FSOURCE of MERGE must have one type and length, and they are %s and %s"},
	"ARRAYRANK":     {Error, "%s's %s must be of rank %s, not %d"},
	"MANYONE":       {Error, "the vector subscript of %s names element %d more than once, so the section can't be given values"},

	// SUBSCRIPTS is an array element with the wrong number of subscripts.
	"SUBSCRIPTS": {Error, "the array %s takes %s, not %d"},

	// NULLSUBSCRIPT is a subscript left out, A(1,,2): a list after a name may
	// have a null only if the name is a function's.
	"NULLSUBSCRIPT": {Error, "a subscript can't be left out"},

	// NOSIZE is an assumed-size array as a whole in an I/O list.
	"NOSIZE": {Error, "the assumed-size array %s has no size, so it can't be written as a whole"},

	// BADOPERAND is an operator applied to a type it doesn't take:
	// .AND. on a REAL.
	"BADOPERAND": {Error, "%s can't be applied to %s value"},

	// CHARNUMBER is a CHARACTER value where a number or a logical value
	// must be: FORTRAN converts no character value to one.
	"CHARNUMBER": {Error, "%s value can't be used as a number or a logical value"},

	// COMPARETYPE is a CHARACTER value compared with a value of another
	// type: a character value can be compared only with another.
	"COMPARETYPE": {Error, "%s value can't be compared with %s value"},

	// ASSIGNTYPE is an assignment of a CHARACTER value to a numeric or
	// logical place, or of another value to a CHARACTER one (section 3.3).
	"ASSIGNTYPE": {Error, "%s value can't be assigned to %s, which is %s"},

	// SUBSTR is a substring of something that isn't a CHARACTER variable
	// or array element.
	"SUBSTR": {Error, "%s is %s, so it can't have a substring"},

	// SUBRANGE is a substring whose constant bounds aren't within its
	// variable (section 2.2.4: 1 <= first <= last <= the length), and
	// SUBBOUND one of whose bounds is constant and outside it.
	"SUBRANGE": {Error, "the substring (%d:%d) isn't within %s's %d characters"},
	"SUBBOUND": {Error, "the substring's bound %d isn't within %s's %d characters"},

	// SUBLOW and SUBORDER are a substring of a passed-length dummy argument
	// whose constant bounds can't be right whatever its length is: a bound
	// less than 1, or a first character after the last.
	"SUBLOW":   {Error, "the substring's bound %d is less than 1"},
	"SUBORDER": {Error, "the substring (%d:%d) ends before it begins"},

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

	// NOTPASSABLE is an intrinsic function's name given as an actual
	// argument that has no routine to pass: SIZEOF, whose value is known
	// only when compiling.
	"NOTPASSABLE": {Error, "the intrinsic function %s can't be an actual argument"},

	// BADTYPELESS is an octal or hexadecimal constant whose bits are a
	// reserved operand in the floating type it is given.
	"BADTYPELESS": {Error, "this constant's bits are a reserved operand, not a %s value"},

	// BADDIGITS is an octal or hexadecimal constant with a digit its base
	// doesn't have, or none at all (section 2.2.1.4).
	"BADDIGITS": {Error, "this constant isn't made of %s digits"},

	// TYPELESSLONG is an octal or hexadecimal constant of more than 128 bits
	// (section 2.2.1.4).
	"TYPELESSLONG": {Error, "this constant has more than 128 bits"},

	// TYPELESSCUT is an octal or hexadecimal constant whose nonzero digits
	// don't fit the type it takes (section 2.2.1.4).
	"TYPELESSCUT": {Error, "this constant's nonzero digits are too many for %s, %d bytes long"},

	// HOLLCUT is a Hollerith constant with characters other than blanks past
	// the type it takes (section 2.2.1.7), or a character constant taken for
	// one.
	"HOLLCUT": {Error, "this constant's characters are too many for %s, %d bytes long"},

	// NOCHARVALUE is a Hollerith, octal, or hexadecimal constant where a
	// CHARACTER value is wanted: it can't take that type (section 2.2.1.7; for
	// an octal or hexadecimal one, ours).
	"NOCHARVALUE": {Error, "%s can't be a CHARACTER value"},

	// BADRAD50 is a character the RADIX-50 set doesn't have (Table B-2).
	"BADRAD50": {Error, "the character %q is not in the RADIX-50 character set"},

	// RAD50LONG is a RADIX-50 constant of more than 12 characters (appendix
	// B.3).
	"RAD50LONG": {Error, "this RADIX-50 constant has %d characters, and may have at most 12"},

	// RAD50PLACE is a RADIX-50 constant outside a DATA statement or an initial
	// value (appendix B.3).
	"RAD50PLACE": {Error, "a RADIX-50 constant may be only in a DATA statement or an initial value"},

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

	// NOENDDO is a DO or DO WHILE loop without a label, which the unit's
	// END reaches before its END DO (section 5.4).
	"NOENDDO": {Error, "this DO loop has no END DO statement"},

	// NOLOOP is END DO with no DO loop open to end.
	"NOLOOP": {Error, "this END DO statement is not in a DO loop"},

	// ENDDOLABEL is END DO in a loop that ends at a label of its own,
	// which an END DO can end only if it has that label (section 5.4).
	"ENDDOLABEL": {Error, "the DO loop at line %d ends at statement label %d, so this END DO statement can't end it"},

	// SHAREDENDDO is an END DO that more than one loop ends at: nested
	// loops may share a labeled terminal statement, but not an END DO
	// (section 5.3.1.2).
	"SHAREDENDDO": {Error, "the DO loop at line %d ends at this END DO statement too, and DO loops can't share an END DO"},

	// Fortran 90's construct names, EXIT, and CYCLE (the standard's 8.1 and
	// 14.1.2; sema/constructs.go).

	// WRONGNAME is a construct name on ELSE IF, ELSE, END IF, or END DO
	// that isn't the name of the construct the statement belongs to.
	"WRONGNAME": {Error, "this %s statement names %s, but the %s at line %d %s"},

	// MISSINGNAME is END IF or END DO without the name its construct was
	// given: a named construct must be named at its end too.
	"MISSINGNAME": {Error, "the %s at line %d is named %s, so this %s statement must name it too"},

	// NAMEDEND is a named DO loop ending at a labeled statement other than
	// END DO: a named loop must end at an END DO naming it (R825).
	"NAMEDEND": {Error, "the DO loop at line %d is named %s, so it must end at an END DO statement naming it, not at %s"},

	// DUPCONSTRUCT is a construct name given to two constructs of a unit.
	"DUPCONSTRUCT": {Error, "%s is already the name of the construct at line %d"},

	// NAMECLASH is a construct name that also names something else of the
	// unit's: construct names are local names like variables' (14.1.2).
	"NAMECLASH": {Error, "%s, the name of the construct at line %d, can't also be the name of %s"},

	// NOTINLOOP is EXIT or CYCLE outside every DO loop.
	"NOTINLOOP": {Error, "this %s statement is not in a DO loop"},

	// NOLOOPNAME is EXIT or CYCLE naming no construct open around it.
	"NOLOOPNAME": {Error, "this %s statement names %s, but it isn't in a DO loop of that name"},

	// NOTALOOP is EXIT or CYCLE naming a construct that isn't a DO loop.
	"NOTALOOP": {Error, "this %s statement names %s, the %s at line %d, which isn't a DO loop"},

	// Fortran 90's CASE construct (the standard's 8.1.3; sema/selectcase.go).

	// NOSELECT is CASE or END SELECT outside any CASE construct.
	"NOSELECT": {Error, "this %s statement is not in a SELECT CASE construct"},

	// NOENDSELECT is a CASE construct the unit's END reaches before its END
	// SELECT.
	"NOENDSELECT": {Error, "this SELECT CASE construct has no END SELECT statement"},

	// Fortran 90's WHERE (the standard's 7.5.3; sema/where.go). NOWHERE is
	// ELSEWHERE or END WHERE outside any WHERE construct; NOENDWHERE a WHERE
	// construct the unit's END reaches; WHEREBODY a statement in one that
	// isn't an assignment (R742); WHEREMASK a mask that isn't a LOGICAL
	// array; WHERETARGET an assignment in a WHERE to a scalar;
	// ELSEWHEREORDER a second ELSEWHERE; F95ONLY a form Fortran 95 added
	// (a WHERE inside a WHERE, ELSEWHERE with a mask), Phase 112's.
	"NOWHERE":        {Error, "this %s statement is not in a WHERE construct"},
	"NOENDWHERE":     {Error, "this WHERE construct has no END WHERE statement"},
	"WHEREBODY":      {Error, "only assignments may be in a WHERE construct, not %s"},
	"WHEREMASK":      {Error, "a WHERE's mask must be a LOGICAL array, and this one is %s"},
	"WHERETARGET":    {Error, "an assignment in a WHERE must be to an array of the mask's shape, not to a scalar"},
	"ELSEWHEREORDER": {Error, "this WHERE construct has an ELSEWHERE statement already, at line %d"},
	"F95ONLY":        {Error, "%s is Fortran 95's, not Fortran 90's, and isn't compiled yet (Phase 112)"},

	// CASEFIRST is a statement between SELECT CASE and the construct's first
	// CASE, where the standard's R808 has no place for one.
	"CASEFIRST": {Error, "a SELECT CASE statement must be followed by a CASE statement, not %s"},

	// CASEEXPR is a SELECT CASE expression that isn't INTEGER, CHARACTER, or
	// LOGICAL (R812).
	"CASEEXPR": {Error, "a SELECT CASE expression must be INTEGER, CHARACTER, or LOGICAL, and this one is %s"},

	// CASEVALUE is a case value that isn't a constant expression (R815).
	"CASEVALUE": {Error, "a case value must be a constant expression, and %s"},

	// CASETYPE is a case value of another type than the SELECT CASE
	// expression's (R814's constraint; INTEGER values of any kind may select
	// an INTEGER expression, the author's answer 2).
	"CASETYPE": {Error, "this case value is %s, but the SELECT CASE expression at line %d is %s"},

	// CASEBYTE is an INTEGER case value for a LOGICAL*1 (BYTE) expression,
	// which selects only by .TRUE. and .FALSE. (the author's answer 1).
	"CASEBYTE": {Error, "the SELECT CASE expression at line %d is LOGICAL*1 (BYTE), which a number can't select; declare it INTEGER(1) to select it by number"},

	// CASERANGE is a range of values (with a colon) selecting a LOGICAL
	// expression (R814's constraint).
	"CASERANGE": {Error, "a LOGICAL SELECT CASE expression is selected by .TRUE. or .FALSE., not by a range of values"},

	// TWODEFAULT is a second CASE DEFAULT in one construct.
	"TWODEFAULT": {Error, "the SELECT CASE construct already has a CASE DEFAULT, at line %d"},

	// CASEOVERLAP is a case value range that some value would match beside
	// one of an earlier CASE's (8.1.3.2: no value may match more than one).
	"CASEOVERLAP": {Error, "this case value overlaps one at line %d: a value of the SELECT CASE expression would match both"},

	// CASENEVER is a case value range no value of the expression can match:
	// its low end above its high end, or past what the expression's INTEGER
	// kind holds (the author's answer 3).
	"CASENEVER": {Warning, "this case value can never match, since %s"},

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

	// ASSIGNVAR is the variable of an ASSIGN or an assigned GO TO that
	// isn't INTEGER*4: it holds a statement's address (section 3.5; the
	// author's answer 1 in docs/planning/PHASE-79).
	"ASSIGNVAR": {Error, "%s is %s, but a statement label is ASSIGNed only to an INTEGER*4 variable"},

	// ARITHIF is an arithmetic IF whose expression isn't INTEGER or REAL
	// (section 5.7.1).
	"ARITHIF": {Error, "an arithmetic IF's expression must be INTEGER or REAL, and this one is %s"},

	// DUPLABEL is a statement label defined twice in a unit.
	"DUPLABEL": {Error, "statement label %d is already on the statement at line %d"},

	// UNDEFLBL is a jump to a label no statement in the unit has.
	"UNDEFLBL": {Error, "statement label %d is never defined"},

	// NOTEXEC is a jump to a statement that can't be jumped to.
	"NOTEXEC": {Error, "statement label %d is on %s, which a jump can't go to"},

	// INTOBLOCK is a jump into an IF block or a CASE construct's block from
	// outside it, or from one of its blocks to another (section 5.7.3.1;
	// the standard's 8.1.1).
	"INTOBLOCK": {Error, "this jump to label %d goes into %s from outside it"},

	// SHAREDTERM is a jump to the terminal statement nested DO loops
	// share from outside the innermost of them (section 5.3.1.3).
	"SHAREDTERM": {Error, "statement label %d ends more than one DO loop, so only the innermost loop can jump to it"},

	// References between the file's units.

	// FUNCTYPE is a reference to a function of the file whose type it
	// doesn't agree with.
	"FUNCTYPE": {Error, "the function %s is %s (line %d), but is used here as %s"},

	// BUILTINPLACE is %VAL, %REF, or %DESCR anywhere but around an actual
	// argument of a CALL or a function reference (section 6.1.2.1).
	"BUILTINPLACE": {Error, "%%%s can only say how an actual argument of a CALL or a function reference is passed"},

	// BUILTINARG is a cell of Table 6-1 that says No.
	"BUILTINARG": {Error, "%%%s can't pass %s"},

	// LOCARG is %LOC of what has no address to give (section 6.1.2.2).
	"LOCARG": {Error, "%%LOC needs a variable, an array, an array element, a substring, or a procedure"},

	// HOLLCHAR is a Hollerith, octal, or hexadecimal constant passed to a
	// CHARACTER dummy argument of a routine of the file (section 6.1.1.4).
	"HOLLCHAR": {Error, "%s can't be passed to %s's CHARACTER dummy argument %s (line %d)"},

	// ARGCOUNT is a reference to a routine of the file with a different
	// number of arguments from its dummy arguments.
	"ARGCOUNT": {Error, "%s takes %s (line %d), but is given %d here"},

	// Internal procedures (sema/internal.go; docs/planning/PHASE-103 - Fortran 90
	// procedures and interfaces.md, findings 2, 3, and 6).

	// DUPNAME is an internal procedure with the name of its host or of another
	// of its host's internal procedures, or of something else its host declares.
	"DUPNAME": {Error, "%s is already %s in this unit, at line %d"},

	// DUPINTERFACE is a type statement, EXTERNAL, or INTRINSIC in a host for one
	// of its internal procedures, whose interface is explicit already (12.3.2.1).
	"DUPINTERFACE": {Error, "%s is an internal procedure (line %d), whose interface its own statements give; it can't be declared here"},

	// INTERNALARG is an internal procedure as an actual argument (R1213's constraint).
	"INTERNALARG": {Error, "the internal procedure %s can't be an actual argument"},

	// ARGTYPE, ARGRANK, ARGLENGTH, and NOTDEFINABLE are an actual argument that
	// doesn't match its dummy in an explicit interface (12.4.1).
	"ARGTYPE":      {Error, "argument %d of %s is %s, but its dummy argument %s is %s (line %d)"},
	"ARGRANK":      {Error, "argument %d of %s is %s, but its dummy argument %s is %s (line %d)"},
	"ARGLENGTH":    {Error, "argument %d of %s is %d characters long, but its dummy argument %s is %d (line %d)"},
	"NOTDEFINABLE": {Error, "argument %d of %s must be a variable, since its dummy argument %s is INTENT(%s) (line %d)"},

	// Interface blocks (sema/interface.go; PHASE-103's finding 7).

	// INTERFACESELF is an interface body for the subprogram holding it, or one
	// of its entry points (R1201's constraint).
	"INTERFACESELF": {Error, "%s is this subprogram's own procedure, so an interface body here can't describe it"},

	// INTERFACEDECL is an interface body for a name the unit declares
	// otherwise, which would give the procedure a second interface (12.3.2.1).
	"INTERFACEDECL": {Error, "%s is already %s in this unit (line %d), so an interface body can't declare it"},

	// INTERFACEDIFF is an interface body that disagrees with the definition of
	// its procedure in the file (12.3.2.1's "consistent").
	"INTERFACEDIFF": {Error, "this interface body for %s doesn't agree with its definition (line %d): %s"},

	// EXPLICITREQ is a reference through an implicit interface to a routine of
	// the file with an OPTIONAL dummy argument (12.3.1.1).
	"EXPLICITREQ": {Error, "%s has the OPTIONAL dummy argument %s (line %d), so a reference to it needs an explicit interface"},

	// Keyword arguments, OPTIONAL, PRESENT, and INTENT (sema/optional.go and
	// intent.go; PHASE-103's findings 8 to 10).

	// KEYWORDNOINTERFACE is a keyword argument in a reference to a procedure
	// with no explicit interface (12.4.1).
	"KEYWORDNOINTERFACE": {Error, "%s has no explicit interface here, so its arguments can't be given by keyword"},

	// NOTOPTIONAL is PRESENT of anything but an OPTIONAL dummy argument of
	// the unit or its host (13.13.80).
	"NOTOPTIONAL": {Error, "the argument of PRESENT must be the name of an OPTIONAL dummy argument"},

	// INTENTIN is an INTENT(IN) dummy argument given a value, or passed where
	// an explicit interface may give it one (5.1.2.3).
	"INTENTIN": {Error, "%s is an INTENT(IN) dummy argument, which can't be given a value"},

	// INTENTOUTREF is an INTENT(OUT) dummy argument used before every
	// statement that gives it a value, in the order of the statements: it is
	// undefined when the routine starts (5.1.2.3).
	"INTENTOUTREF": {Warning, "%s is an INTENT(OUT) dummy argument, used here before any statement gives it a value"},

	// Generic interfaces (sema/generic.go; PHASE-103's finding 11).

	// GENERICDECL is a generic name the unit declares otherwise.
	"GENERICDECL": {Error, "%s is already %s in this unit (line %d), so it can't be a generic name"},

	// GENERICMIX is a generic name's specific procedures, a function and a
	// subroutine (14.1.2.3).
	"GENERICMIX": {Error, "%s and %s, specific procedures of the generic name %s, must both be functions or both subroutines"},

	// GENERICAMBIG is two specific procedures of a generic name whose
	// arguments can't tell them apart (14.1.2.3).
	"GENERICAMBIG": {Error, "the specific procedures %s (line %d) and %s of the generic name %s can't be told apart by their arguments"},

	// GENERICINTRINSIC is a specific procedure of a generic name that is an
	// intrinsic function's, taking what the intrinsic takes (14.1.2.3).
	"GENERICINTRINSIC": {Error, "the specific procedure %s takes arguments the intrinsic function %s already takes"},

	// GENERICNONE is a reference by a generic name that no specific procedure
	// fits (14.1.2.4.1).
	"GENERICNONE": {Error, "no specific procedure of the generic name %s takes %s"},

	// GENERICARG is a generic name as an actual argument (12.4.1.2).
	"GENERICARG": {Error, "the generic name %s can't be an actual argument; pass one of its specific procedures"},

	// Defined operators and assignment (sema/operators.go; PHASE-103's findings
	// 12 and 13).

	// OPUNDEFINED is a defined operator no interface block in the unit or its
	// hosts gives a meaning (12.3.2.1.1).
	"OPUNDEFINED": {Error, "%s is no operator here: no interface block gives it a meaning"},

	// DEFOPNONE is a defined operation whose operands no function of its
	// operator's interface blocks takes.
	"DEFOPNONE": {Error, "no function for the operator %s takes %s"},

	// DEFOPFUNC, DEFOPDUMMY, and DEFOPCOUNT are an operator's procedure that
	// isn't a function, whose dummies aren't one or two INTENT(IN) data objects,
	// not OPTIONAL, or whose number doesn't fit the intrinsic operator it
	// extends (12.3.2.1.1).
	"DEFOPFUNC":  {Error, "%s is a subroutine, but a procedure for %s must be a function"},
	"DEFOPDUMMY": {Error, "%s, a function for %s, must have one or two dummy arguments, each data, INTENT(IN), and not OPTIONAL"},
	"DEFOPCOUNT": {Error, "%s, a function for %s, takes %d operands, which that operator never has"},

	// DEFOPINTR is a function for an intrinsic operator taking operands Fortran
	// gives the operator a meaning for already (7.1.3, 14.1.2.3).
	"DEFOPINTR": {Error, "%s takes operands the operator %s already has a meaning for"},

	// DEFASSIGNSUB, DEFASSIGNDUMMY, and DEFASSIGNINTR are ASSIGNMENT(=)'s
	// procedures: subroutines of two dummies, the first INTENT(OUT) or
	// INTENT(INOUT), the second INTENT(IN), for an assignment Fortran gives no
	// meaning (12.3.2.1.2, 7.5.1.3).
	"DEFASSIGNSUB":   {Error, "%s is a function, but a procedure for ASSIGNMENT(=) must be a subroutine"},
	"DEFASSIGNDUMMY": {Error, "%s, a subroutine for ASSIGNMENT(=), must have two dummy arguments, not OPTIONAL, the first INTENT(OUT) or INTENT(INOUT) and the second INTENT(IN)"},
	"DEFASSIGNINTR":  {Error, "%s defines assigning %s %s, which Fortran defines already"},

	// OPAMBIG is two procedures for one operator, or for assignment, that no
	// operand's type, kind, or rank tells apart (14.1.2.3).
	"OPAMBIG": {Error, "the procedures %s (line %d) and %s for %s can't be told apart by their operands"},

	// ARGTYPEW and ARGRANKW are ARGTYPE and ARGRANK through an implicit
	// interface, under /WARNINGS=ARGUMENT_CHECKING.
	"ARGTYPEW": {Warning, "argument %d of %s is %s, but its dummy argument %s is %s (line %d)"},
	"ARGRANKW": {Warning, "argument %d of %s is %s, but its dummy argument %s is %s (line %d)"},

	// ALTPLACE is a CALL of a routine of the file whose alternate return
	// specifiers aren't in the places of the routine's *s (section 6.1.1.5).
	"ALTPLACE": {Error, "the alternate return specifiers in this call of %s aren't in the places of its dummy argument list's *s (line %d)"},

	// The I/O statements' messages (chapters 7 and 8, appendix A.1;
	// Phase 80).

	// FMTSYNTAX is a FORMAT statement whose specification breaks chapter
	// 8's rules (internal/fortran/format says what's wrong), pointing at
	// the character that does.
	"FMTSYNTAX": {Error, "the format is not valid: %s"},

	// BADVFE is a variable format expression that isn't a number (section
	// 8.3.2).
	"BADVFE": {Error, "a variable format expression must be a number, not %s"},

	// FMTLABEL is a FORMAT statement without a label, which nothing could
	// use (section 8.1).
	"FMTLABEL": {Error, "a FORMAT statement must have a label"},

	// NOTFORMAT is an I/O statement's format label on a statement other
	// than a FORMAT.
	"NOTFORMAT": {Error, "statement label %d is on %s, not a FORMAT statement"},

	// BADFORMAT is a format that is none of section 7.1.1.2's: a FORMAT
	// statement's label, an INTEGER*4 variable a FORMAT's label was
	// ASSIGNed to, a CHARACTER value, or an array or an array element
	// holding a run-time format.
	"BADFORMAT": {Error, "a format must be a FORMAT statement's label, a CHARACTER value, an array or an array element, or an INTEGER*4 variable ASSIGNed a FORMAT statement's label, and this is %s"},

	// BADUNIT is a unit that is neither an INTEGER expression nor an
	// internal file: a CHARACTER variable, array element, substring, or
	// array (section 7.1.1.1).
	"BADUNIT": {Error, "a unit must be an INTEGER expression or a CHARACTER variable, array, array element, or substring, and this is %s"},

	// BADINPUT is an input list item that can't be read into: only a
	// variable, an array element, a substring, a whole array, or an
	// implied-DO list can (section 7.1.2).
	"BADINPUT": {Error, "%s can't be read into: an input list holds variables, array elements, substrings, arrays, and implied-DO lists"},

	// BADIOSTAT is an IOSTAT= that doesn't name an INTEGER variable or
	// array element (section 7.1.1.9).
	"BADIOSTAT": {Error, "an IOSTAT= specifier must name an INTEGER variable or array element"},

	// KEYTWICE is a file statement's keyword given twice, under one of its
	// names or the other (sections 9.1, 9.2).
	"KEYTWICE": {Error, "%s repeats %s in this statement"},

	// KEYNUMBER is a keyword that takes a number given something else
	// (section 9.1).
	"KEYNUMBER": {Error, "%s= must be a number, not %s"},

	// KEYTEXT is a keyword that takes a character value given something
	// else (section 9.1).
	"KEYTEXT": {Error, "%s= must be a character value, not %s"},

	// KEYCHOICE is a keyword's constant value that isn't one of its list
	// (section 9.1), which VMS's run-time library would call FOR$_KEYVALERR.
	"KEYCHOICE": {Error, "'%s' isn't a value of %s="},

	// OPENCONFLICT is an OPEN whose keywords VMS's run-time library would
	// refuse together (FOR$_INCOPECLO).
	"OPENCONFLICT": {Error, "%s= can't be given with %s"},

	// NEEDRECL is an OPEN of a new file of fixed-length records with no
	// RECL= (Table 9-3).
	"NEEDRECL": {Error, "%s needs RECL= for a new file"},

	// INQUIREBOTH and INQUIRENONE are an INQUIRE that doesn't ask by a unit
	// or by a file, the two forms of section 9.3.
	"INQUIREBOTH": {Error, "an INQUIRE asks by a unit or by FILE=, not both"},
	"INQUIRENONE": {Error, "an INQUIRE needs a unit, FILE=, or DEFAULTFILE="},

	// KEYPLACE is an INQUIRE specifier that doesn't name a place of the
	// type its answer has.
	"KEYPLACE": {Error, "%s= must name a place to store %s value"},

	// BADCOUNT is an ENCODE or DECODE count that isn't an INTEGER
	// expression (appendix A.1).
	"BADCOUNT": {Error, "%s's count of characters must be an INTEGER expression"},

	// CODEFORMAT is ENCODE or DECODE with a list-directed or namelist
	// format, which they don't have (appendix A.1).
	"CODEFORMAT": {Error, "%s needs a FORMAT statement's label or a run-time format"},

	// BADBUFFER is ENCODE's or DECODE's buffer when it isn't a variable,
	// an array, an array element, or a substring.
	"BADBUFFER": {Error, "%s's buffer must be a variable, an array, an array element, or a substring"},

	// NMLINTERNAL is namelist I/O to an internal file, which section 7.1.1.3
	// doesn't allow.
	"NMLINTERNAL": {Error, "namelist I/O can't use an internal file"},

	// UNFUNIT is unformatted I/O to an internal file or the default unit
	// (*), which are formatted (section 7.1.1.1).
	"UNFUNIT": {Error, "unformatted I/O needs a unit number, not an internal file or *"},

	// DIRECTUNIT is direct access I/O (REC= or u'r) to an internal file or
	// the default unit (*), which have no records by number (section
	// 7.1.1.1).
	"DIRECTUNIT": {Error, "direct access I/O needs a unit number, not an internal file or *"},

	// DIRECTFORM is direct access I/O that is list-directed or namelist,
	// where it is formatted or unformatted (sections 7.2.2 and 7.3.2).
	"DIRECTFORM": {Error, "direct access I/O is formatted or unformatted, not %s"},

	// DIRECTEND is END= in a direct access READ, which has no end of file
	// to reach (section 7.2.2).
	"DIRECTEND": {Error, "a direct access READ can't have END="},

	// BADREC is a record number that isn't an INTEGER or REAL expression
	// (section 7.1.1.4).
	"BADREC": {Error, "a record number must be an INTEGER expression, not %s"},

	// ASSOCVAR is an associated variable (OPEN's ASSOCIATEVARIABLE=, DEFINE
	// FILE's last) that isn't an INTEGER variable, or is a dummy argument
	// (section 9.1.2, appendix A.2).
	"ASSOCVAR": {Error, "the associated variable %s must be an INTEGER*2 or INTEGER*4 variable, and not a dummy argument"},

	// USEROPENNAME is a USEROPEN= that isn't the name of a procedure
	// declared EXTERNAL (section 9.1.28).
	"USEROPENNAME": {Error, "the routine USEROPEN= names must be a procedure declared EXTERNAL"},

	// RECORDUNIT is a REWRITE or a keyed READ of an internal file or the
	// default unit (*), which have no current record or keys (sections 7.2.3
	// and 7.4).
	"RECORDUNIT": {Error, "%s needs a unit number, not an internal file or *"},

	// RECORDFORM is a REWRITE or a keyed READ that is list-directed or
	// namelist, where it is formatted or unformatted (sections 7.2.3 and 7.4).
	"RECORDFORM": {Error, "%s is formatted or unformatted, not %s"},

	// KEYEDREC is a READ with both a key (KEY=, KEYEQ=, ...) and a record
	// number, two ways of choosing its record.
	"KEYEDREC": {Error, "a READ can't have both a key and a record number"},

	// KEYEDVALUE is a key that isn't an INTEGER or CHARACTER value, or a
	// KEYID= that isn't an INTEGER (sections 7.1.1.7 and 7.1.1.8).
	"KEYEDVALUE": {Error, "%s= must be %s, not %s"},

	// KEYIDALONE is a READ with KEYID= and no key to look for (section
	// 7.1.1.8: the key of reference "accompanies" a key's value).
	"KEYIDALONE": {Error, "a READ with KEYID= needs a key to look for, such as KEY="},

	// KEYEDEND is a keyed READ with END=, which its form doesn't have (section
	// 7.2.3).
	"KEYEDEND": {Error, "a keyed READ can't have END="},

	// KEYLISTLONG is an OPEN listing more keys than it can (section 9.1.15).
	"KEYLISTLONG": {Error, "an OPEN statement can list at most %d keys in KEY=, not %d; make the file with FDL"},

	// KEYFIELDTYPE is a key whose type isn't one of section 9.1.15's.
	"KEYFIELDTYPE": {Error, "a key's type is INTEGER or CHARACTER, not %s"},

	// KEYFIELDDIR is a key whose direction isn't one of section 9.1.15's.
	"KEYFIELDDIR": {Error, "a key's direction is ASCENDING or DESCENDING, not %s"},

	// KEYBOUNDS is a key whose positions break section 9.1.15's rule, 1 <= e1
	// <= e2.
	"KEYBOUNDS": {Error, "the key %d:%d can't be: its first byte must be 1 or later, and not after its last"},

	// KEYLENGTH is a key too long (section 9.1.15).
	"KEYLENGTH": {Error, "the key %d:%d is %d bytes long, and a key can be at most 255"},

	// KEYINTEGER is an INTEGER key of another length (section 9.1.15).
	"KEYINTEGER": {Error, "the INTEGER key %d:%d is %d bytes long, and an INTEGER key must be 2 or 4"},

	// FINDREC is a FIND with no record number (appendix A.3).
	"FINDREC": {Error, "a FIND needs a record number, u'r or REC="},

	// DEFINEUNIT is a DEFINE FILE unit that isn't an INTEGER constant or
	// variable, or a record count or length that isn't an INTEGER (appendix
	// A.2).
	"DEFINEUNIT": {Error, "the %s of a DEFINE FILE must be an INTEGER constant or variable"},

	// NMLTWICE is a NAMELIST group defined twice in a unit.
	"NMLTWICE": {Error, "the NAMELIST group %s is defined already (line %d)"},

	// Allocatable arrays (docs/planning/PHASE-104 - Fortran 90 dynamic
	// arrays.md). DEFERREDNOTALLOC is a deferred shape, (:), on an array that
	// isn't ALLOCATABLE (or a dummy's assumed shape); DEFERREDMIX bounds some
	// of which are colons; ALLOCSHAPE an ALLOCATABLE name without a deferred
	// shape (the standard's 5.1's constraints); ALLOCDUMMY one that is a dummy
	// argument or a function's value, ALLOCATTR one in COMMON, EQUIVALENCE,
	// DATA, or NAMELIST, or given a value (5.2.6, the HP manual's 6.3).
	// NOTALLOCATABLE is ALLOCATE, DEALLOCATE, or ALLOCATED of a name that isn't
	// an allocatable array; ALLOCBOUNDS an ALLOCATE object with the wrong
	// number of bounds, or a bound of * or :; ALLOCSELF a bound that uses an
	// object of its own statement (6.3.1's constraint); ALLOCOBJECT the same
	// object twice in one statement. BADSTAT is a STAT= that doesn't name an
	// INTEGER variable or array element, and STATSELF one that uses an object
	// of its own statement (6.3.1's and 6.3.3's constraints).
	"DEFERREDNOTALLOC": {Error, "%s has a deferred shape, (:), so it must be ALLOCATABLE"},
	"DEFERREDMIX":      {Error, "the bounds of %s must be all colons or none"},
	"ALLOCSHAPE":       {Error, "%s is ALLOCATABLE, so it must be an array whose bounds are all colons, (:)"},
	"ALLOCDUMMY":       {Error, "%s can't be ALLOCATABLE: it is %s"},
	"ALLOCATTR":        {Error, "the allocatable array %s can't %s"},
	"NOTALLOCATABLE":   {Error, "%s is not an allocatable array"},
	"ALLOCBOUNDS":      {Error, "in ALLOCATE, %s needs %s, each a value or two separated by a colon"},
	"ALLOCSELF":        {Error, "a bound in ALLOCATE can't use %s, which the same statement allocates or deallocates"},
	"ALLOCOBJECT":      {Error, "%s is named twice in one %s statement"},
	"BADSTAT":          {Error, "a STAT= specifier must name an INTEGER variable or array element"},
	"STATSELF":         {Error, "the STAT= specifier can't use %s, which the same statement allocates or deallocates"},

	// NMLDUMMY is a dummy argument in a NAMELIST group (section 4.10).
	"NMLDUMMY": {Error, "the dummy argument %s can't be in a NAMELIST group"},

	// NMLITEM is a NAMELIST group's name that is neither a variable nor
	// an array.
	"NMLITEM": {Error, "a NAMELIST group holds variables and arrays, and %s is %s"},

	// NOTNAMELIST is NML= naming no NAMELIST group of the unit.
	"NOTNAMELIST": {Error, "%s is not a NAMELIST group"},

	// ERRSNSARG is an argument of ERRSNS that isn't an INTEGER*2 or
	// INTEGER*4 variable or array element, or a null argument (D.4.3).
	"ERRSNSARG": {Error, "an argument of ERRSNS must be an INTEGER*2 or INTEGER*4 variable or array element, or null"},

	// ERRSNSLEN is a call of ERRSNS whose arguments are of both lengths,
	// which D.4 doesn't allow: all INTEGER*2 or all INTEGER*4.
	"ERRSNSLEN": {Error, "the arguments of ERRSNS must be all INTEGER*2 or all INTEGER*4"},

	// ERRSNSCOUNT is a call of ERRSNS with more than its five arguments.
	"ERRSNSCOUNT": {Error, "the system subroutine ERRSNS takes at most 5 arguments, and is given %d"},

	// SYSCOUNT is a system subroutine or function given the wrong number of
	// arguments (D.4, D.5.3): its name, how many it takes, and how many it has.
	"SYSCOUNT": {Error, "the system routine %s takes %s, and is given %d"},

	// SYSARG is an argument of a system subroutine or function of the wrong
	// kind: the routine's name, then what the argument must be.
	"SYSARG": {Error, "an argument of %s must be %s"},

	// SYSLEN is a system subroutine's arguments of both integer lengths.
	"SYSLEN": {Error, "the arguments of %s must be all INTEGER*2 or all INTEGER*4"},
}

// Known reports whether ident is a message in the table, and its
// severity.
func Known(ident string) (Severity, bool) {
	k, ok := messages[ident]

	return k.Severity, ok
}

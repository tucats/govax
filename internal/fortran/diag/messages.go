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
}

// Known reports whether ident is a message in the table, and its
// severity.
func Known(ident string) (Severity, bool) {
	k, ok := messages[ident]

	return k.Severity, ok
}

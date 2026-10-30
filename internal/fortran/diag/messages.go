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
}

// Known reports whether ident is a message in the table, and its
// severity.
func Known(ident string) (Severity, bool) {
	k, ok := messages[ident]

	return k.Severity, ok
}

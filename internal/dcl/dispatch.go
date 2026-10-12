package dcl

import "github.com/tucats/govax/internal/vmserrors"

// Dispatch calls the Handler bound (via Bind) to r's active entry, passing
// the active entry's ID. (The ID can be used to help the handler disambiguate
// what part of the DCL grammar invoked it).) The handler is expected to use
// the Result to read any parameters or qualifiers that were parsed from the
// command line. If no handler is bound to the active entry, an error is
// returned.
func (g *Grammar) Dispatch(r *Result) error {
	h, ok := g.handlers[r.Active]
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOHANDLER, r.Active)
	}

	return h(r.ActiveID, r)
}

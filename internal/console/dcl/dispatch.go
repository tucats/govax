package dcl

import "fmt"

// Dispatch calls the Handler bound (via Bind) to r's active entry, passing
// the active entry's ID — the Go equivalent of DCLdispatch, which always
// calls whichever verb/syntax ended up DCL_PRESENT after a parse.
func (g *Grammar) Dispatch(r *Result) error {
	h, ok := g.handlers[r.Active]
	if !ok {
		return fmt.Errorf("dcl: no handler bound for %s", r.Active)
	}
	return h(r.ActiveID, r)
}

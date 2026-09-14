package rtl

// ShimFunc implements one LIB$/CRTL shim routine (a librtl_*.c function),
// matching shim.c's own CALLV signature: given the argument list already
// resolved to native longwords, return the value shim() leaves in R0.
type ShimFunc func(env *Environment, argv []uint32) (uint32, error)

// shimEntry pairs a ShimFunc with the name shim.c's rtl_entry_list records
// it under, purely for diagnostics (matching shim_dump's own listing).
type shimEntry struct {
	name string
	fn   ShimFunc
}

// ShimTable is a numeric-code-keyed registry of ShimFuncs, the Go equivalent
// of shim.c's rtl_entry_list — see doc.go's design note on why this is a
// registry rather than a switch.
type ShimTable struct {
	entries map[uint32]shimEntry
}

// NewShimTable returns an empty ShimTable.
func NewShimTable() *ShimTable {
	return &ShimTable{entries: map[uint32]shimEntry{}}
}

// Register adds fn under code, matching shim_declare(code, entry). A code
// already registered is silently replaced — this project's own registries
// (e.g. internal/cpu's Table.SetHandler) don't guard against re-registration
// either, and doing so would only matter for a programming mistake caught
// immediately by this package's own tests.
func (t *ShimTable) Register(code uint32, name string, fn ShimFunc) {
	t.entries[code] = shimEntry{name: name, fn: fn}
}

// Lookup returns the ShimFunc registered under code, matching shim()'s own
// rtl_entry_list[code].addr != 0 check.
func (t *ShimTable) Lookup(code uint32) (ShimFunc, bool) {
	e, ok := t.entries[code]
	if !ok {
		return nil, false
	}
	return e.fn, true
}

// registerShims installs every implemented shim routine into t. Each
// register* function lives alongside the routines it registers (math.go,
// time.go, ...), matching shim_init's own per-routine shim_declare calls.
func registerShims(t *ShimTable) {
	registerMathShims(t)
	registerTimeShims(t)
}

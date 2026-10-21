package corevms

// ShimFunc implements one LIB$/CRTL shim routine: given the argument list already
// resolved to native longwords, return the value shim() leaves in R0.
type ShimFunc func(env *Environment, argv []uint32) (uint32, error)

// JSBShimFunc implements a routine reached by JSB rather than CALL
// (docs/PHASE-54, "The JSB entry points"): its arguments are in R0, R1,
// R2, ..., and it leaves its results in registers. It gets the registers
// R0 to R6 in regs and changes the ones the routine returns; the XFC
// puts all seven back. A JSB routine has no call frame of its own, so a
// condition it signals (Environment.SignalFailure) is its caller's, and
// SS$_CONTINUE from a handler returns to the caller with R0 the status.
type JSBShimFunc func(env *Environment, regs *JSBRegisters) error

// JSBRegisters are R0 to R6, the registers a JSB routine may use for its
// arguments and results.
type JSBRegisters [7]uint32

// shimEntry pairs a ShimFunc (or, for a routine reached by JSB, a
// JSBShimFunc) with the name rtl_entry_list records it under, purely for
// diagnostics (matching shim_dump's own listing).
type shimEntry struct {
	name string
	fn   ShimFunc
	jsb  JSBShimFunc

	// callsBack is true for a routine that may call back into the
	// program (RegisterCallback): it runs on a goroutine of its own.
	callsBack bool
}

// ShimTable is a numeric-code-keyed registry of ShimFuncs.
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

// RegisterCallback adds fn under code as Register does, for a routine
// that may call back into the program with Environment.Callback (a
// success or error routine the program passed it; callback.go).
func (t *ShimTable) RegisterCallback(code uint32, name string, fn ShimFunc) {
	t.entries[code] = shimEntry{name: name, fn: fn, callsBack: true}
}

// CallsBack reports whether the routine under code was registered with
// RegisterCallback.
func (t *ShimTable) CallsBack(code uint32) bool {
	return t.entries[code].callsBack
}

// RegisterJSB adds the JSB routine fn under code. CALL and JSB routines
// share one space of codes.
func (t *ShimTable) RegisterJSB(code uint32, name string, fn JSBShimFunc) {
	t.entries[code] = shimEntry{name: name, jsb: fn}
}

// Lookup returns the ShimFunc registered under code, matching shim()'s own
// rtl_entry_list[code].addr != 0 check. A JSB routine's code isn't one.
func (t *ShimTable) Lookup(code uint32) (ShimFunc, bool) {
	e, ok := t.entries[code]
	if !ok || e.fn == nil {
		return nil, false
	}

	return e.fn, true
}

// Has reports whether a routine, reached by CALL or by JSB, is
// registered under code.
func (t *ShimTable) Has(code uint32) bool {
	_, ok := t.entries[code]

	return ok
}

// LookupJSB returns the JSB routine registered under code.
func (t *ShimTable) LookupJSB(code uint32) (JSBShimFunc, bool) {
	e, ok := t.entries[code]
	if !ok || e.jsb == nil {
		return nil, false
	}

	return e.jsb, true
}

// registerShims installs the shim routines rtl implements itself into t:
// the C library's (DECC$) and govax's own (EXE$). LIBRTL.EXE's routines are
// internal/librtl's, which the console registers alongside. Each
// register* function lives alongside the routines it registers (math.go,
// time.go, ...), matching shim_init's own per-routine shim_declare calls.
func registerShims(t *ShimTable) {
	registerTimeShims(t)
	registerStringShims(t)
	registerMemoryShims(t)
	registerPrintShims(t)
	registerFileShims(t)
	registerInputShims(t)
	registerTracebackShims(t)
}

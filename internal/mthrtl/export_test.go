package mthrtl

// What the external tests (package mthrtl_test, which can import the
// console, which imports this package) use of this package's insides.
var (
	CodeOf          = codeOf
	SignalCondition = signalCondition
	Fail            = fail
	LoadArg         = loadArg
	LoadRegisters   = loadRegisters
	StoreResult     = storeResult
	StoreRegisters  = storeRegisters
	ReturnFloat     = returnFloat
)

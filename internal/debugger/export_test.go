package debugger

// SetTakeTraces makes d take a trace pending when a STEP begins, as its
// own, instead of giving it to the program (traced.go's programGetsTrace):
// the other answer Decision 5 leaves to the probe, for the tests.
func SetTakeTraces(d *Debugger, on bool) { d.takeTraces = on }

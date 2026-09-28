package cpu

// Attention keys and CTRL/C / CTRL/Y ASTs (docs/PHASE-26.md subtask 27).
//
// When the user types CTRL/C at the host terminal while a VAX program
// runs, govax's console is told (Engine.Attention, from main.go's
// keyboard and signal handling), and by default the next Step stops the
// machine with ErrAttention, returning to the console prompt.
//
// On VMS, though, a program can ask to handle CTRL/C (or CTRL/Y) itself:
// a terminal $QIO with IO$_SETMODE!IO$M_CTRLCAST names an AST routine to
// call when the key is typed, instead of the command interpreter
// interrupting the program. So before stopping, Step offers the key to the
// system services' AttentionHandler, if they have one. If the handler
// takes it — it has queued the program's AST, which Step's AST check then
// delivers — the key is consumed and the program keeps running.

// The attention keys: the control characters CTRL/C and CTRL/Y.
const (
	AttentionCtrlC byte = 0x03
	AttentionCtrlY byte = 0x19
)

// AttentionHandler is the optional half of SystemServices that can take
// an attention key. SetSystemServices checks for it.
type AttentionHandler interface {
	// HandleAttention is called, on the goroutine running Step, when an
	// attention key is pending. It returns true if it has dealt with the
	// key (queued an AST for it), false to let the machine stop.
	HandleAttention(key byte) bool
}

// checkAttention is Step's attention check: nil to go on, or
// ErrAttention to stop. A pending key the handler takes is cleared. The
// key is only cleared if it hasn't changed meanwhile (another typed on
// the host goroutine), so a second key typed at that moment is kept.
func (e *Engine) checkAttention() error {
	key := e.attentionKey.Load()
	if key == 0 {
		return nil
	}

	if e.attentionHandler != nil && e.attentionHandler.HandleAttention(byte(key)) {
		e.attentionKey.CompareAndSwap(key, 0)

		return nil
	}

	return ErrAttention
}

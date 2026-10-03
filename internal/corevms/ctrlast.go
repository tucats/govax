package corevms

// CTRL/C and CTRL/Y ASTs (docs/PHASE-26.md subtask 27).
//
// # What they're for
//
// When a VMS user types CTRL/Y at the terminal, the command interpreter
// interrupts whatever program is running ("*INTERRUPT*"); CTRL/C does the
// same unless something asks for it. A program that wants to handle these
// keys itself — to stop a long listing cleanly, say — asks the terminal
// driver for an AST when one is typed:
//
//	$QIOW chan=..., func=IO$_SETMODE!IO$M_CTRLCAST,
//	      p1=astadr, p2=astprm, p3=acmode
//
// (IO$M_CTRLYAST for CTRL/Y). The AST is *one-shot*: once delivered, the
// program must ask again to catch the next key. p1 of 0 cancels the
// request. VMS's rules for a typed key are:
//
//   - CTRL/C: every CTRL/C AST enabled on the terminal is delivered. If
//     there are none, CTRL/C acts as CTRL/Y.
//   - CTRL/Y: every CTRL/Y AST is delivered. If there are none, the
//     command interpreter interrupts the program.
//
// A channel's requests are cancelled by $CANCEL and $DASSGN on it (and
// so by image rundown, which deassigns user-mode channels).
//
// # How govax does it
//
// The terminal driver's IO$_SETMODE (ttdriver.go) records requests with
// armAttentionAST. The host's CTRL/C reaches the engine as an attention
// key (internal/cpu/attention.go); before stopping the machine, the
// engine asks Attention — through the console — whether the program
// takes it. Attention queues the ASTs as the rules above say and reports
// whether it did. The AST is delivered at the next instruction boundary
// like any other.
//
// Every terminal is the console, so the requests are the Environment's,
// not a device's.

// attentionAST is one CTRL/C or CTRL/Y AST request.
type attentionAST struct {
	key     byte   // AttentionCtrlC or AttentionCtrlY
	channel uint16 // the channel it was requested on
	routine uint32
	param   uint32
	mode    uint32
}

// The attention keys, as the engine reports them (the control
// characters). They equal cpu.AttentionCtrlC and cpu.AttentionCtrlY; this
// package doesn't import internal/cpu, and a console test checks they
// agree.
const (
	AttentionCtrlC = 0x03
	AttentionCtrlY = 0x19
)

// armAttentionAST records an AST request for key on channel: routine
// with param, delivered in mode. It replaces the channel's earlier
// request for the same key. routine 0 just cancels that request.
func (env *Environment) armAttentionAST(key byte, channel uint16, routine, param, mode uint32) {
	env.disarmAttentionASTs(func(a attentionAST) bool { return a.key == key && a.channel == channel })

	if routine != 0 {
		env.attentionASTs = append(env.attentionASTs, attentionAST{key: key, channel: channel, routine: routine, param: param, mode: mode})
	}
}

// disarmAttentionASTs removes every request match accepts.
func (env *Environment) disarmAttentionASTs(match func(attentionAST) bool) {
	kept := env.attentionASTs[:0]

	for _, a := range env.attentionASTs {
		if !match(a) {
			kept = append(kept, a)
		}
	}

	env.attentionASTs = kept
}

// disarmChannel removes channel's requests: $CANCEL and $DASSGN.
func (env *Environment) disarmChannel(channel uint16) {
	env.disarmAttentionASTs(func(a attentionAST) bool { return a.channel == channel })
}

// Attention is the engine's question when the user types key (CTRL/C or
// CTRL/Y) at the terminal: does the program take it? If it has ASTs
// enabled for the key — or, for CTRL/C with none, for CTRL/Y — they are
// queued and disabled (they're one-shot), and Attention returns true.
// Otherwise it returns false and the machine stops, as it always has.
func (env *Environment) Attention(key byte) bool {
	if key == AttentionCtrlC && env.deliverAttentionASTs(AttentionCtrlC) {
		return true
	}

	return env.deliverAttentionASTs(AttentionCtrlY)
}

// deliverAttentionASTs queues and removes every request for key,
// reporting whether there were any.
func (env *Environment) deliverAttentionASTs(key byte) bool {
	found := false

	for _, a := range env.attentionASTs {
		if a.key == key {
			env.queueAST(a.routine, a.param, a.mode)

			found = true
		}
	}

	if found {
		env.disarmAttentionASTs(func(a attentionAST) bool { return a.key == key })
	}

	return found
}

// AttentionASTs reports how many CTRL/C and CTRL/Y AST requests are
// enabled.
func (env *Environment) AttentionASTs() int { return len(env.attentionASTs) }

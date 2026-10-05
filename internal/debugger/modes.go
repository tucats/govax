package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is SET MODE, CANCEL MODE, SHOW MODE, and SHOW RADIX
// (docs/PHASE-42.md, subtask 11, Decisions 4 and 8).
//
// "Mode" means two unrelated things here:
//
//   - The VMS debugger's *display modes*: whether names are symbolic,
//     whether stepping goes by source line, whether operands are
//     explained, which floating format is shown, and so on.
//     SET MODE SYMBOLIC and SET MODE NOOPERANDS are of this kind.
//   - govax's *access mode*: which of the VAX's four privilege levels
//     (kernel, executive, supervisor, user) the CPU runs in, which also
//     picks the stack pointer in use. The console's old SET MODE KERNEL
//     was of this kind, and the debugger keeps it.
//
// One SET MODE accepts both sets of keywords, since they don't collide,
// except for INTERRUPT, which the VMS debugger uses as a display mode
// (the debugger sees interrupt-level code too). govax's switch to the
// interrupt stack, which used that word in the console, is ISP here.

// displayModes are the SET MODE settings the debugger holds besides
// symbolic and operands (which have fields of their own because
// EXAMINE/INSTRUCTION reads them). govax records them and shows them in
// SHOW MODE, as VMS does, but doesn't act on them yet: it has no
// screen mode and shows no floating-point values (docs/PHASE-42.md lists
// this as unconfirmed).
type displayModes struct {
	// line says whether the debugger works by source line (the VMS
	// default) rather than by address. gFloat chooses G_FLOAT over
	// D_FLOAT as the floating-point format to show. scroll, dynamic, and
	// interrupt are the VMS debugger's other on/off settings.
	line, gFloat, scroll, dynamic, interrupt bool
}

// defaultDisplayModes are SET MODE's settings at start-up and after
// CANCEL MODE, as the probe shows them (testdata/dbgcmd/vax/exam.dlg).
func defaultDisplayModes() displayModes {
	return displayModes{line: true, scroll: true, dynamic: true, interrupt: true}
}

// modeKeyword is one word SET MODE accepts. apply makes the change; its
// argument is the text after an "=" ("" when there was none).
type modeKeyword struct {
	name  string
	apply func(d *Debugger, value string) error
}

// flag returns an apply function that sets *target to on, for a keyword
// that takes no value.
func flag(target func(d *Debugger) *bool, on bool) func(*Debugger, string) error {
	return func(d *Debugger, value string) error {
		if value != "" {
			return vmserrors.New(vmserrors.DBG_SYNTAX, "="+value)
		}

		*target(d) = on

		return nil
	}
}

// unavailable is the apply function of a VMS mode govax doesn't have
// (screen mode and its relatives, which are out of scope): the keyword is
// known, so that an abbreviation is ambiguous exactly as VMS's is, but
// using it is a syntax error.
func unavailable(d *Debugger, value string) error {
	return vmserrors.New(vmserrors.DBG_SYNTAX, "mode")
}

// accessMode returns an apply function that switches the CPU to a mode
// (govax's SET MODE KERNEL and the like).
func accessMode(mode vax.AccessMode, interruptStack bool) func(*Debugger, string) error {
	return func(d *Debugger, value string) error {
		if value != "" {
			return vmserrors.New(vmserrors.DBG_SYNTAX, "="+value)
		}

		return d.Console.SetAccessMode(mode, interruptStack)
	}
}

// modeKeywords lists every word SET MODE accepts. A word may be
// abbreviated to anything that picks out one of them (setMode).
var modeKeywords = []modeKeyword{
	{"SYMBOLIC", flag(func(d *Debugger) *bool { return &d.symbolic }, true)},
	{"NOSYMBOLIC", flag(func(d *Debugger) *bool { return &d.symbolic }, false)},
	{"LINE", flag(func(d *Debugger) *bool { return &d.modes.line }, true)},
	{"NOLINE", flag(func(d *Debugger) *bool { return &d.modes.line }, false)},
	{"D_FLOAT", flag(func(d *Debugger) *bool { return &d.modes.gFloat }, false)},
	{"G_FLOAT", flag(func(d *Debugger) *bool { return &d.modes.gFloat }, true)},
	{"SCROLL", flag(func(d *Debugger) *bool { return &d.modes.scroll }, true)},
	{"NOSCROLL", flag(func(d *Debugger) *bool { return &d.modes.scroll }, false)},
	{"DYNAMIC", flag(func(d *Debugger) *bool { return &d.modes.dynamic }, true)},
	{"NODYNAMIC", flag(func(d *Debugger) *bool { return &d.modes.dynamic }, false)},
	{"INTERRUPT", flag(func(d *Debugger) *bool { return &d.modes.interrupt }, true)},
	{"NOINTERRUPT", flag(func(d *Debugger) *bool { return &d.modes.interrupt }, false)},
	{"OPERANDS", func(d *Debugger, value string) error { return d.setOperands(value) }},
	{"NOOPERANDS", func(d *Debugger, value string) error {
		if value != "" {
			return vmserrors.New(vmserrors.DBG_SYNTAX, "="+value)
		}

		d.operands = console.OperandsOff

		return nil
	}},

	// Screen mode is out of scope for this phase.
	{"SCREEN", unavailable},
	{"NOSCREEN", unavailable},
	{"KEYPAD", unavailable},
	{"NOKEYPAD", unavailable},
	{"SEPARATE", unavailable},
	{"NOSEPARATE", unavailable},

	// govax's access modes.
	{"KERNEL", accessMode(vax.Kernel, false)},
	{"EXECUTIVE", accessMode(vax.Executive, false)},
	{"SUPERVISOR", accessMode(vax.Supervisor, false)},
	{"USER", accessMode(vax.User, false)},
	{"ISP", accessMode(vax.Kernel, true)},
}

// setOperands is SET MODE OPERANDS[=FULL|BRIEF].
func (d *Debugger) setOperands(value string) error {
	switch {
	case value == "":
		d.operands = console.OperandsBrief
	case strings.HasPrefix("FULL", value):
		d.operands = console.OperandsFull
	case strings.HasPrefix("BRIEF", value):
		d.operands = console.OperandsBrief
	default:
		return vmserrors.New(vmserrors.DBG_SYNTAX, value)
	}

	return nil
}

// lookupMode finds the SET MODE keyword a word names: the keyword it
// spells exactly, or else the only one it is an abbreviation of. A word
// that abbreviates several (S) is ambiguous, as DCL's own are; one that
// names none is a syntax error.
func lookupMode(word string) (modeKeyword, error) {
	var matches []modeKeyword

	for _, k := range modeKeywords {
		if k.name == word {
			return k, nil
		}

		if word != "" && strings.HasPrefix(k.name, word) {
			matches = append(matches, k)
		}
	}

	switch len(matches) {
	case 0:
		return modeKeyword{}, vmserrors.New(vmserrors.DBG_SYNTAX, word)
	case 1:
		return matches[0], nil
	}

	return modeKeyword{}, vmserrors.New(vmserrors.CLI_AMBIGUOUS, "keyword", word)
}

// setMode runs SET MODE keyword[,keyword...]: each is a display mode
// ([NO]SYMBOLIC, [NO]OPERANDS[=FULL|BRIEF], ...) or one of govax's access
// modes (KERNEL, EXECUTIVE, SUPERVISOR, USER, ISP). The words before a
// bad one have taken effect, as a series of separate SET MODEs would.
func (d *Debugger) setMode(words string) error {
	for _, word := range strings.Split(words, ",") {
		word = strings.ToUpper(strings.TrimSpace(word))
		name, value, _ := strings.Cut(word, "=")

		k, err := lookupMode(name)
		if err != nil {
			return err
		}

		if err := k.apply(d, value); err != nil {
			return err
		}
	}

	return nil
}

// cancelMode runs CANCEL MODE: every display mode goes back to its
// start-up setting. The radixes and govax's access mode are left alone.
func (d *Debugger) cancelMode() error {
	d.symbolic = console.SymbolicDefault()
	d.operands = console.OperandsOff
	d.modes = defaultDisplayModes()

	return nil
}

// radixWord is the name SHOW MODE and SHOW RADIX give a radix.
func radixWord(radix int) string {
	for _, r := range radixNames {
		if r.radix == radix {
			return strings.ToLower(r.name)
		}
	}

	return fmt.Sprintf("radix %d", radix)
}

// modesLine is SHOW MODE's first line, as the VMS debugger words it:
//
//	modes: symbolic, line, d_float, noscreen, scroll, nokeypad, dynamic, interrupt, no separate window
//
// A mode that is off has NO in front, as it does in SET MODE. The
// operands setting appears only when on, as "brief operands" or "full
// operands".
func (d *Debugger) modesLine() string {
	word := func(on bool, name string) string {
		if on {
			return name
		}

		return "no" + name
	}

	float := "d_float"
	if d.modes.gFloat {
		float = "g_float"
	}

	parts := []string{word(d.symbolic, "symbolic"), word(d.modes.line, "line"), float}

	switch d.operands {
	case console.OperandsBrief:
		parts = append(parts, "brief operands")
	case console.OperandsFull:
		parts = append(parts, "full operands")
	}

	// Screen mode and the keypad don't exist here, so they're always off.
	parts = append(parts, "noscreen", word(d.modes.scroll, "scroll"), "nokeypad",
		word(d.modes.dynamic, "dynamic"), word(d.modes.interrupt, "interrupt"), "no separate window")

	return "modes: " + strings.Join(parts, ", ")
}

// ShowMode runs SHOW MODE: the display modes, the two radixes, and then
// govax's access mode (the VMS debugger has no such line, so it comes
// last, where a log comparison can set it aside).
func (d *Debugger) ShowMode() error {
	c := d.Console

	c.Printf("%s\n", d.modesLine())

	if err := d.ShowRadix(); err != nil {
		return err
	}

	if name := c.AccessModeName(); name != "" {
		c.Printf("access mode: %s\n", name)
	}

	return nil
}

// ShowRadix runs SHOW RADIX: the radix numbers are typed in and the one
// they are shown in, in the VMS debugger's two lines.
func (d *Debugger) ShowRadix() error {
	d.Console.Printf("input radix : %s\n", radixWord(d.inputRadix))
	d.Console.Printf("output radix: %s\n", radixWord(d.outputRadix))

	return nil
}

package console

import (
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// IF, THEN, ELSE, and ENDIF (docs/PHASE-50 - DCL command procedures.md,
// subtask 12), as the OpenVMS User's Manual describes them (13.5.2 and
// 14.16). IF has two forms. On one line, it runs one command when its
// expression is true:
//
//	$ IF P1 .EQS. "" THEN GOTO NO_FILE
//
// Without THEN, it starts a block, whose first command must be THEN:
//
//	$ IF COUNT .GT. 10
//	$ THEN
//	$     WRITE SYS$OUTPUT "Too many"
//	$     EXIT
//	$ ELSE
//	$     COUNT = COUNT + 1
//	$ ENDIF
//
// The commands after THEN, up to ELSE or ENDIF, run when the expression
// is true; those after ELSE, up to ENDIF, when it's false. THEN and ELSE
// may each have a command on their own line too. Blocks nest, at most 15
// deep at one command level (14.16.4).
//
// The expression is DCL's (dclexpr.go), and it is true when its value is
// odd: an odd integer, a string whose number is odd, or a string that
// starts with T or Y, in either case (dclValue.Int).
//
// Each command level keeps the blocks open at it (flowState.blocks). DCL
// passes over the lines of a branch that doesn't run without running or
// substituting them, counting the blocks inside them by their THEN and
// ENDIF lines (blockSkip, Console.skipLine), so the same rule works for a
// procedure and for lines typed at the terminal. A GOTO out of a block
// closes it (dcllabel.go), and RETURN puts back the blocks GOSUB left
// (dclcall.go). The blocks still open when a procedure ends go with it.
//
// IF, THEN, ELSE, and ENDIF leave $STATUS alone (13.15 names IF; the
// others are unconfirmed), so a block can test the status of the command
// before it; a command run after THEN or ELSE sets it as it would alone.

// maxIfBlocks is how deeply IF blocks nest at one command level (the
// User's Manual's 14.16.4).
const maxIfBlocks = 15

// ifBlock is one open IF-THEN-ELSE block.
type ifBlock struct {
	// number identifies the block among those of its level, so that a
	// label knows which blocks it was in (labelPlace).
	number int

	// value is the IF's expression's truth: whether the THEN branch
	// runs.
	value bool

	// state is how far the block has got.
	state blockState
}

// blockState is where an IF block is.
type blockState int

const (
	// awaitingThen: after IF; the next command must be THEN.
	awaitingThen blockState = iota

	// inThen: after THEN, in the THEN branch (run or passed over).
	inThen

	// inElse: after ELSE, in the ELSE branch (run or passed over).
	inElse
)

// blockSkip says DCL is passing over lines: the rest of a block's branch
// that doesn't run.
type blockSkip struct {
	// active is set while lines are passed over.
	active bool

	// toElse says the branch passed over is THEN's, so the block's ELSE
	// ends it (and the ELSE branch runs); otherwise only its ENDIF does.
	toElse bool

	// depth is how many blocks, started inside the lines passed over,
	// are still open.
	depth int
}

// top returns the innermost open block at f's level, nil for none.
func (f *flowState) top() *ifBlock {
	if len(f.blocks) == 0 {
		return nil
	}

	return &f.blocks[len(f.blocks)-1]
}

// blockNumbers returns the numbers of f's open blocks, outermost first.
func (f *flowState) blockNumbers() []int {
	numbers := make([]int, len(f.blocks))
	for i, b := range f.blocks {
		numbers[i] = b.number
	}

	return numbers
}

// closeBlocks closes the blocks open at f's level past the first depth
// of them, as going to a label outside them does, and stops any passing
// over of lines.
func (f *flowState) closeBlocks(depth int) {
	f.blocks = f.blocks[:min(depth, len(f.blocks))]
	f.skip = blockSkip{}
}

// popBlock closes the innermost open block.
func (f *flowState) popBlock() {
	f.blocks = f.blocks[:len(f.blocks)-1]
}

// skipLine passes over line, a command line read from the current level,
// if the level is passing over the lines of a branch that doesn't run
// (run false). It counts the blocks inside them, by their THEN and ENDIF
// lines, and stops at the branch's end: the block's ENDIF, which closes
// it, or, for a THEN branch, its ELSE, whose branch then runs. ELSE's
// own command, if it has one, is returned to be run (run true).
func (c *Console) skipLine(line string) (rest string, run bool) {
	f := c.flow()
	if !f.skip.active {
		return line, true
	}

	_, afterLabel, _ := splitLabel(line)
	verb, rest := flowVerb(afterLabel)

	switch verb {
	case "THEN":
		f.skip.depth++

	case "ENDIF":
		if f.skip.depth > 0 {
			f.skip.depth--

			break
		}

		f.skip = blockSkip{}
		if f.top() != nil {
			f.popBlock()
		}

	case "ELSE":
		if f.skip.depth > 0 || !f.skip.toElse {
			break
		}

		f.skip = blockSkip{}
		if b := f.top(); b != nil {
			b.state = inElse
		}

		return rest, rest != ""
	}

	return "", false
}

// awaitThen checks the command after a block's IF: it must be THEN. Any
// other closes the block, and isn't run: CLI$_NOTHEN (unconfirmed).
func (c *Console) awaitThen(line string) error {
	f := c.flow()

	b := f.top()
	if b == nil || b.state != awaitingThen {
		return nil
	}

	if verb, _ := flowVerb(line); verb == "THEN" {
		return nil
	}

	f.popBlock()

	return vmserrors.New(vmserrors.CLI_NOTHEN)
}

// ifCommand is IF expression [THEN [$] command]. With THEN, the command
// runs when the expression is true (it comes back to dispatchCommand,
// so its apostrophes aren't substituted again). Without THEN, the IF
// starts a block (openBlock). A false IF, or one that starts a block,
// leaves $STATUS alone (13.15); a command after THEN sets it.
func (d *Dispatcher) ifCommand(id int64, r *dcl.Result) error {
	text := strings.TrimSpace(r.String("TEXT"))
	if text == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	value, command, then, err := evaluateDCLCondition(text, &d.Console.dclSymbols, d.Console)
	if err != nil {
		return err
	}

	if !then {
		return d.Console.openBlock(value)
	}

	command = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), "$"))
	if command == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	if !value {
		d.Console.keepStatus()

		return nil
	}

	return d.dispatchCommand(command)
}

// evaluateDCLCondition evaluates the expression at the start of text, an
// IF's, and reports its truth (its value's low bit), the text after a
// THEN that follows it, and whether there is one. Anything else after
// the expression is CLI$_NOTHEN.
func evaluateDCLCondition(text string, symbols *dclSymbolTable, c *Console) (value bool, command string, then bool, err error) {
	e := &dclExpression{text: text, symbols: symbols, console: c}

	if e.skipBlanks(); e.atEnd() {
		return false, "", false, vmserrors.New(vmserrors.CLI_EXPSYN)
	}

	v, err := e.binary(precOr)
	if err != nil {
		return false, "", false, err
	}

	value = v.Int()&1 != 0

	if e.skipBlanks(); e.atEnd() {
		return value, "", false, nil
	}

	end := e.pos
	for end < len(text) && isDCLNameChar(text[end]) {
		end++
	}

	if !strings.EqualFold(text[e.pos:end], "THEN") {
		return false, "", false, vmserrors.New(vmserrors.CLI_NOTHEN)
	}

	return value, text[end:], true, nil
}

// openBlock starts an IF block whose expression's truth is value: the
// next command must be THEN (awaitThen).
func (c *Console) openBlock(value bool) error {
	f := c.flow()
	if len(f.blocks) >= maxIfBlocks {
		return vmserrors.New(vmserrors.CLI_INVIFNEST)
	}

	f.nextBlock++
	f.blocks = append(f.blocks, ifBlock{number: f.nextBlock, value: value})
	c.keepStatus()

	return nil
}

// thenCommand is THEN [[$] command], the first command of an IF block:
// when the IF was true, the THEN branch runs, starting with THEN's own
// command; when it was false, DCL passes over the branch to the block's
// ELSE or ENDIF. A THEN that doesn't follow a block's IF is
// CLI$_INVIFNEST (unconfirmed).
func (d *Dispatcher) thenCommand(id int64, r *dcl.Result) error {
	f := d.Console.flow()

	b := f.top()
	if b == nil || b.state != awaitingThen {
		return vmserrors.New(vmserrors.CLI_INVIFNEST)
	}

	b.state = inThen

	command := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r.String("COMMAND")), "$"))
	if !b.value {
		f.skip = blockSkip{active: true, toElse: true}
	}

	if !b.value || command == "" {
		d.Console.keepStatus()

		return nil
	}

	return d.dispatchCommand(command)
}

// elseCommand is ELSE [[$] command], reached at the end of a THEN branch
// that ran: DCL passes over the ELSE branch to the block's ENDIF. (When
// the THEN branch didn't run, skipLine starts the ELSE branch instead.)
// An ELSE outside a THEN branch is CLI$_INVIFNEST.
func (d *Dispatcher) elseCommand(id int64, r *dcl.Result) error {
	f := d.Console.flow()

	b := f.top()
	if b == nil || b.state != inThen {
		return vmserrors.New(vmserrors.CLI_INVIFNEST)
	}

	b.state = inElse
	f.skip = blockSkip{active: true}
	d.Console.keepStatus()

	return nil
}

// endifCommand is ENDIF, which closes the innermost block. One with no
// block open, or in a block still waiting for its THEN, is
// CLI$_INVIFNEST.
func (d *Dispatcher) endifCommand(id int64, r *dcl.Result) error {
	f := d.Console.flow()

	b := f.top()
	if b == nil || b.state == awaitingThen {
		return vmserrors.New(vmserrors.CLI_INVIFNEST)
	}

	f.popBlock()
	d.Console.keepStatus()

	return nil
}

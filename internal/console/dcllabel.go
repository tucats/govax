package console

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// Labels and GOTO (docs/PHASE-50 - DCL command procedures.md, subtask
// 11), as the OpenVMS User's Manual describes them (13.2 and 14.16.5).
//
// A label marks a line of a command procedure: a name, ended by a colon,
// at the start of a command line, alone or before a command:
//
//	$ LOOP:
//	$ COUNT = COUNT + 1
//	$ IF COUNT .LT. 10 THEN GOTO LOOP
//
// As DCL runs a procedure's lines, it enters each label it reaches in the
// level's own table of labels (a "special section of the local symbol
// table", 13.2.1; SHOW SYMBOL doesn't show them). A label used again
// replaces the place recorded for it. GOTO looks there first, so of
// duplicate labels it goes to the one DCL processed last (13.2.2). A
// label not reached yet is searched for forward from the GOTO, and the
// first one found is taken (the labels the search passes are recorded
// too). When neither finds it, the search has used
// up the file: DCL shows CLI$_USGOTO and the procedure ends there, at its
// end of file, unless an ON action moves it somewhere else first.
//
// A GOTO's target can't be inside an IF-THEN-ELSE block, or a CALL
// subroutine, that the GOTO isn't in (14.16.5): the forward search
// follows the blocks it passes (searchLabel), and a recorded label
// counts only while the blocks it was in are still open. Going out of a
// block to a label after its ENDIF closes the block. A label is local to
// its command level: a CALL subroutine's level has its own.
//
// At the terminal (command level 0) a label means nothing: DCL ignores
// it with CLI$_NOLBLS, and GOTO has nothing to search.

// flowState is a command level's place in its text and what the commands
// that move it keep: the labels, GOTO, IF blocks (dclif.go), GOSUB, and
// CALL (dclcall.go). Each procedure level has one (commandLevel.flow), and
// so does the terminal (Console.terminalFlow), where only IF blocks mean
// anything.
type flowState struct {
	// source is the level's procedure text; nil at the terminal.
	source *procedureSource

	// labels are the labels DCL has processed at this level, by name
	// (uppercase), each where it was last seen.
	labels map[string]labelPlace

	// blocks are the IF-THEN-ELSE blocks open at this level, innermost
	// last, and nextBlock the number the next one opened gets (dclif.go).
	blocks    []ifBlock
	nextBlock int

	// skip is set while the lines of a block's branch that doesn't run
	// are passed over (dclif.go).
	skip blockSkip

	// gosubs are GOSUB's return places, the latest last (dclcall.go).
	gosubs []gosubFrame
}

// labelPlace is where a label is: the index of the first record of its
// line, and the IF blocks that were open there, by number, outermost
// first.
type labelPlace struct {
	pos    int
	blocks []int
}

// flow returns the current command level's flowState: the innermost
// procedure's, or the terminal's.
func (c *Console) flow() *flowState {
	if level := c.currentLevel(); level != nil {
		return &level.flow
	}

	return &c.terminalFlow
}

// splitLabel splits a label from the start of a command line: a name of
// letters, digits, "$", and "_", starting with a letter, "$", or "_",
// ended by a colon (not ":=", an assignment). It returns the label,
// uppercase, and the rest of the line, the command after it if there is
// one.
func splitLabel(line string) (label, rest string, ok bool) {
	i := 0
	for i < len(line) && isDCLLabelChar(line[i]) {
		i++
	}

	if i == 0 || !isDCLSymbolStart(line[0]) || i >= len(line) || line[i] != ':' {
		return "", line, false
	}

	if i+1 < len(line) && line[i+1] == '=' {
		return "", line, false
	}

	return strings.ToUpper(line[:i]), strings.TrimSpace(line[i+1:]), true
}

// isDCLLabelChar reports whether ch can be in a label: a letter, digit,
// "$", or "_".
func isDCLLabelChar(ch byte) bool {
	return isDCLSymbolStart(ch) || (ch >= '0' && ch <= '9')
}

// takeLabel handles a label at the start of line, a command line DCL has
// read from the current level: in a procedure it records where the label
// is, and at the terminal it shows CLI$_NOLBLS. It returns the rest of
// the line, the command to run; when there is none, the line leaves
// $STATUS alone (a label isn't a command).
func (c *Console) takeLabel(line string) (string, error) {
	label, rest, ok := splitLabel(line)
	if !ok {
		return line, nil
	}

	f := c.flow()

	if f.source == nil {
		err := vmserrors.New(vmserrors.CLI_NOLBLS)
		if rest == "" {
			return "", err
		}

		c.Printf("%%%s\n", err.Error())

		return rest, nil
	}

	if f.labels == nil {
		f.labels = map[string]labelPlace{}
	}

	f.labels[label] = labelPlace{pos: f.source.lineStart, blocks: f.blockNumbers()}

	if rest == "" {
		c.keepStatus()
	}

	return rest, nil
}

// labelTarget is where a GOTO, GOSUB, or CALL goes: the first record of
// the label's line, how many of the level's open IF blocks are still
// open there (the rest are closed by going there), and the label's line
// after the label.
type labelTarget struct {
	pos   int
	depth int
	rest  string
}

// findLabel finds label for a GOTO, GOSUB, or CALL at f's level: where
// DCL last processed it, if the IF blocks it was in are still open, or
// else the first one a forward search from the cursor finds. notFound
// is the error for a label that isn't there; a search that ran into a
// SUBROUTINE with no ENDSUBROUTINE is CLI$_MSNGENDS instead.
func (f *flowState) findLabel(label string, notFound error) (labelTarget, error) {
	open := f.blockNumbers()

	if place, ok := f.labels[label]; ok && len(place.blocks) <= len(open) && slices.Equal(place.blocks, open[:len(place.blocks)]) {
		line, _, _, _, _ := f.source.lineAt(place.pos)
		_, rest, _ := splitLabel(line)

		return labelTarget{pos: place.pos, depth: len(place.blocks), rest: rest}, nil
	}

	// The labels the search passes are processed as it passes them
	// (unconfirmed: the manual says DCL enters labels "as [it]
	// encounters" them), so a later GOTO finds them where they are.
	seen := func(name string, pos, depth int) {
		if f.labels == nil {
			f.labels = map[string]labelPlace{}
		}

		f.labels[name] = labelPlace{pos: pos, blocks: open[:depth]}
	}

	target, found, unended := f.source.searchLabel(label, f.source.Position(), len(open), seen)

	switch {
	case found:
		return target, nil
	case unended:
		return target, vmserrors.New(vmserrors.CLI_MSNGENDS)
	}

	return target, notFound
}

// jump moves f's level to target: its cursor to the label's line, which
// is read next (and the label recorded again), and its IF blocks to
// those still open there.
func (f *flowState) jump(target labelTarget) {
	f.source.Seek(target.pos)
	f.closeBlocks(target.depth)
}

// searchLabel searches p's lines from record pos on for the first line
// with label at the level the search starts at, as DCL scans forward for
// a GOTO's target, following the blocks it passes: a THEN line starts
// an IF block, ELSE switches to its other branch, and ENDIF ends it; a
// SUBROUTINE line starts a CALL subroutine, and ENDSUBROUTINE ends it.
// open is how many IF blocks are open where the search starts. A label
// inside a block the search entered, or in the other branch of a block
// that was open, isn't a target. Leaving an open block by its ENDIF is
// allowed, and the target's depth says how many are left.
//
// Each label the search passes that could be a target is given to seen,
// if it isn't nil, with its line's first record and its depth.
//
// unended reports a search that ended inside a SUBROUTINE with no
// ENDSUBROUTINE (DCL's CLI$_MSNGENDS).
func (p *procedureSource) searchLabel(label string, pos, open int, seen func(name string, pos, depth int)) (target labelTarget, found, unended bool) {
	// entered are the blocks the search went into, innermost last: 'i'
	// for an IF block (or a branch of an open one the search isn't in),
	// 's' for a subroutine.
	var entered []byte

	for {
		line, data, start, next, ok := p.lineAt(pos)
		if !ok {
			return target, false, slices.Contains(entered, 's')
		}

		pos = next

		if data {
			continue
		}

		name, rest, labelled := splitLabel(line)
		if labelled && len(entered) == 0 {
			if name == label {
				return labelTarget{pos: start, depth: open, rest: rest}, true, false
			}

			if seen != nil {
				seen(name, start, open)
			}
		}

		verb, _ := flowVerb(rest)

		switch verb {
		case "THEN":
			entered = append(entered, 'i')
		case "SUBROUTINE":
			entered = append(entered, 's')
		case "ELSE":
			// The other branch of a block that was open: the search has
			// left the branch it was in, and is in one it isn't in.
			if len(entered) == 0 && open > 0 {
				open--
				entered = append(entered, 'i')
			}
		case "ENDIF", "ENDSUBROUTINE":
			switch {
			case len(entered) > 0:
				entered = entered[:len(entered)-1]
			case verb == "ENDIF" && open > 0:
				open--
			}
		}
	}
}

// flowVerbs are the verbs whose lines change a level's block structure:
// a scan of a procedure's text, or DCL passing over the lines of a
// block, looks for them without running anything (flowVerb).
var flowVerbs = []string{"THEN", "ELSE", "ENDIF", "SUBROUTINE", "ENDSUBROUTINE"}

// flowVerb returns which of flowVerbs line's verb is, "" for none, and
// the rest of the line after it. A verb may be shortened to four
// characters, as DCL's verbs may (unconfirmed for these).
func flowVerb(line string) (verb, rest string) {
	word, after := readCommandVerb(line)
	word = strings.ToUpper(word)

	if len(word) >= 4 {
		for _, v := range flowVerbs {
			if strings.HasPrefix(v, word) {
				return v, strings.TrimSpace(after)
			}
		}
	}

	return "", line
}

// labelParameter reads the label a GOTO, GOSUB, or CALL names: one word
// (dclWord's rules), uppercase. A second word is CLI$_MAXPARM, and none
// CLI$_INSFPRM.
func labelParameter(text string) (string, error) {
	label, i, ok := dclWord(text, 0, false)
	if !ok || label == "" {
		return "", vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	if _, _, more := dclWord(text, i, false); more {
		return "", vmserrors.New(vmserrors.CLI_MAXPARM)
	}

	return label, nil
}

// gotoCommand is GOTO label (Console.Goto).
func (d *Dispatcher) gotoCommand(id int64, r *dcl.Result) error {
	label, err := labelParameter(r.String("LABEL"))
	if err != nil {
		return err
	}

	return d.Console.Goto(label)
}

// Goto is GOTO label: the current procedure goes on at the label's line
// (findLabel). A label that isn't there leaves the procedure at its end
// of file, with CLI$_USGOTO. At the terminal there's nothing to search,
// and GOTO is CLI$_USGOTO. GOTO leaves $STATUS alone (13.15).
func (c *Console) Goto(label string) error {
	f := c.flow()
	if f.source == nil {
		return vmserrors.New(vmserrors.CLI_USGOTO)
	}

	target, err := f.findLabel(strings.ToUpper(label), vmserrors.New(vmserrors.CLI_USGOTO))
	if err != nil {
		f.source.Seek(f.source.end)

		return err
	}

	f.jump(target)
	c.keepStatus()

	return nil
}

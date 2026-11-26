package dclcmd

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sumslp"
)

// MACRO/UPDATE (docs/planning/PHASE-117 - SUMSLP.md, subtask 8): a source
// file with /UPDATE is SUMSLP's input, and what the assembler is given is
// SUMSLP's output; the source and the update files aren't changed. The
// update files are named as EDIT/SUM names them (sumUpdates: the first the
// source's name with the type .UPD, each later one in the directory of the
// one before), a missing one is %SUM-I-OPENIN and the assembly goes on
// (testdata/sum117's M07, M16), several are merged into one list (M03), and
// SUMSLP's warnings go to the terminal and the listing (M13). The listing's
// side is internal/asm's update.go.
//
// Unconfirmed: a source none of whose update files could be read is
// assembled as it is, its listing a plain one (M07 can't tell: SUMB's lines
// fit either way); an input record past SUMSLP's 255 bytes is EDIT/SUM's
// READERR, and the lines before it are assembled.

// macroUpdate applies the update files specs to a source file's lines, found
// at found, and returns the lines to assemble and how SUMSLP numbered them,
// or the lines as they were and nil if no update file could be read.
func (c *Context) macroUpdate(lines [][]byte, found rms.FileLocation, specs []string) ([][]byte, *asm.Update, error) {
	updates, err := c.sumUpdates(specs, c.macroSourceName(found))
	if err != nil {
		return nil, nil, err
	}

	if len(updates) == 0 {
		return lines, nil, nil
	}

	r := sumslp.Apply(lines, updates)

	for _, m := range r.Messages() {
		c.Printf("%s\n", m)
	}

	if r.Long > 0 {
		c.Printf("%%SUM-E-READERR, %s\n%s\n", sumReadText(found.Name), rtbLine(r.Long))
	}

	return r.Output, sumUpdate(r, len(lines)), nil
}

// macroSourceName is a source file's name as the update files' defaults
// take it.
func (c *Context) macroSourceName(found rms.FileLocation) rms.ParsedName {
	if p, _, err := c.ContainerSession.Parse(found.Name, "", "", true, false); err == nil {
		return p
	}

	// A host file: its name alone, the update files beside it.
	dir, file := filepath.Split(found.Name)
	name := strings.TrimSuffix(file, filepath.Ext(file))

	return rms.ParsedName{Directory: dir, Name: name}
}

// sumUpdate turns SUMSLP's listing lines into the assembler's account of
// them: a line for each output record, in order (the listing's numbered
// lines are the output's records), with the warnings before it; inputs is
// the input's line count.
func sumUpdate(r *sumslp.Result, inputs int) *asm.Update {
	u := &asm.Update{Inputs: inputs}

	var (
		notes []asm.UpdateNote
		input int // the last input line listed
	)

	for _, l := range r.Lines {
		if l.Number == "" {
			// A warning's first line starts with "%"; the lines after it
			// name its edits.
			if strings.HasPrefix(string(l.Text), "%") || len(notes) == 0 {
				notes = append(notes, asm.UpdateNote{})
			}

			n := &notes[len(notes)-1]
			n.Lines = append(n.Lines, string(l.Text))

			continue
		}

		line := asm.UpdatedLine{Field: l.Field, Before: notes}
		notes = nil

		if rest, ok := strings.CutPrefix(l.Number, "."); ok {
			line.Line, line.Insert = input, "."+rest
		} else {
			input, _ = strconv.Atoi(l.Number)
			line.Line = input
		}

		u.Lines = append(u.Lines, line)
	}

	u.After = notes

	return u
}

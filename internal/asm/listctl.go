package asm

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the listing controls (docs/PHASE-29.md, subtask 7):
// the directives that decide which source lines a MACRO listing shows,
// and where its pages break.
//
//   - .SHOW and .NOSHOW turn listing options on and off. Each option says
//     whether one kind of line is listed: BINARY (MEB) the lines of a macro
//     expansion that store bytes, CALLS (MC) macro calls, CONDITIONALS
//     (CND) conditional directives and the lines they leave out,
//     DEFINITIONS (MD) macro definitions, and EXPANSIONS (ME) every line
//     of a macro expansion. The options aren't counted: .SHOW EXPANSIONS
//     twice and .NOSHOW EXPANSIONS once leaves expansions off
//     (lctl.lis's "Counted options").
//   - .LIST and .NLIST with arguments are .SHOW and .NOSHOW. Without
//     arguments (and so are .SHOW and .NOSHOW without them) they raise and
//     lower the listing level: below 0 nothing is listed, at 0 the options
//     decide, and above 0 every line is listed. The directive's own line
//     is listed only when the level it leaves is above 0.
//   - .PAGE starts a new page, unless the page has nothing on it yet; its
//     own line isn't listed.
//   - .SBTTL (.SUBTITLE) names the part of the program that follows: the
//     heading of each later page shows it, and the table of contents
//     lists it.
//
// MACRO's /SHOW= and /NOSHOW= qualifiers set options for the whole
// assembly (SetListingShow): the source's directives don't change an
// option the command line named (lctlshow.lis and lctlnosh.lis).
//
// Each recorded line keeps the options and level in force when it began
// (listLine.show); the listing decides from them which lines to show
// (listShown).

// listOptions is a set of listing options, one bit each.
type listOptions uint8

const (
	showBinary      listOptions = 1 << iota // BINARY, MEB
	showCalls                               // CALLS, MC
	showConditional                         // CONDITIONALS, CND
	showDefinitions                         // DEFINITIONS, MD
	showExpansions                          // EXPANSIONS, ME

	// defaultShow is what real MACRO starts with: calls, conditionals,
	// and definitions listed; expansions not.
	defaultShow = showCalls | showConditional | showDefinitions
)

// listOptionNames maps each option's names, full and abbreviated, to its
// bit.
var listOptionNames = map[string]listOptions{
	"BINARY":       showBinary,
	"MEB":          showBinary,
	"CALLS":        showCalls,
	"MC":           showCalls,
	"CONDITIONALS": showConditional,
	"CND":          showConditional,
	"DEFINITIONS":  showDefinitions,
	"MD":           showDefinitions,
	"EXPANSIONS":   showExpansions,
	"ME":           showExpansions,
}

// listShow is the listing state in force: the options, and the listing
// level (.LIST and .NLIST without arguments).
type listShow struct {
	opts  listOptions
	level int
}

// has reports whether option o is on.
func (s listShow) has(o listOptions) bool { return s.opts&o != 0 }

// SetListingShow sets listing options for the next Assemble, as MACRO's
// /SHOW=(show) and /NOSHOW=(noshow) qualifiers do: each list holds option
// names (BINARY or MEB, CALLS or MC, CONDITIONALS or CND, DEFINITIONS or
// MD, EXPANSIONS or ME), turned on or off for the whole assembly, whatever
// the source's .SHOW and .NOSHOW say.
func (a *Assembler) SetListingShow(show, noshow []string) error {
	on, err := parseListOptions(".SHOW", show)
	if err != nil {
		return err
	}

	off, err := parseListOptions(".NOSHOW", noshow)
	if err != nil {
		return err
	}

	a.showOn, a.showOff = on, off

	return nil
}

// parseListOptions returns the options names name, for directive (the
// directive or qualifier, for an error).
func parseListOptions(directive string, names []string) (listOptions, error) {
	var opts listOptions

	for _, name := range names {
		o, ok := listOptionNames[strings.ToUpper(strings.TrimSpace(name))]
		if !ok {
			return 0, vmserrors.New(vmserrors.VAX_BADKEYWORD, directive, name)
		}

		opts |= o
	}

	return opts, nil
}

// startShow returns the listing state an assembly starts with.
func (a *Assembler) startShow() listShow {
	return listShow{opts: (defaultShow | a.showOn) &^ a.showOff}
}

// pseudoShow assembles .SHOW and .NOSHOW (on says which), and .LIST and
// .NLIST, which are the same directives: with a list of options, it
// turns them on or off; with none, it raises or lowers the listing level.
func (a *Assembler) pseudoShow(c *cursor, on bool) error {
	var names []string

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.atEnd() {
			break
		}

		name := scanName(c)
		if name == "" {
			return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
		}

		names = append(names, name)
	}

	if len(names) == 0 {
		if on {
			a.show.level++
		} else {
			a.show.level--
		}

		if a.listCur != nil {
			a.listCur.levelChange = true
			a.listCur.levelAfter = a.show.level
		}

		return nil
	}

	directive := ".SHOW"
	if !on {
		directive = ".NOSHOW"
	}

	opts, err := parseListOptions(directive, names)
	if err != nil {
		return err
	}

	if on {
		a.show.opts |= opts
	} else {
		a.show.opts &^= opts
	}

	// An option the command line set stays set.
	a.show.opts = (a.show.opts | a.showOn) &^ a.showOff

	return nil
}

// pseudoPage assembles .PAGE: the listing starts a new page.
func (a *Assembler) pseudoPage(c *cursor) error {
	c.pos = len(c.s)

	if a.listCur != nil {
		a.listCur.page = true
	}

	return nil
}

// pseudoSubtitle assembles .SBTTL (.SUBTITLE) text: the text, as written,
// is the subtitle of the listing's later pages, and an entry in its table
// of contents.
func (a *Assembler) pseudoSubtitle(c *cursor) error {
	c.skipBlanks()
	text := strings.TrimRight(c.rest(), " \t")
	c.pos = len(c.s)

	if a.listCur != nil {
		a.listCur.subtitle = text
		a.listCur.hasSubtitle = true
	}

	return nil
}

// conditionalDirectives are the conditional assembly directives, whose
// lines .NOSHOW CONDITIONALS leaves out of the listing.
var conditionalDirectives = map[string]bool{
	"IF": true, "IF_FALSE": true, "IFF": true, "IF_TRUE": true, "IFT": true,
	"IF_TRUE_FALSE": true, "IFTF": true, "ENDC": true, "IIF": true,
}

// listConditional records that the line is a conditional directive.
func (a *Assembler) listConditional() {
	if a.listCur != nil {
		a.listCur.conditional = true
	}
}

// listCall records that the line is a macro call.
func (a *Assembler) listCall() {
	if a.listCur != nil {
		a.listCur.call = true
	}
}

// listShown reports whether the listing shows l, by the listing state in
// force when it began (see this file's comment). These rules are what
// real MACRO's listings of the probe's lctl.mar show (docs/PHASE-29.md,
// subtask 7):
//
//   - A line of a macro expansion or repeat block is listed under
//     EXPANSIONS by the same rules as the program's own lines; under
//     BINARY alone, only if it stored bytes.
//   - A macro definition, from .MACRO to .ENDM, is listed under
//     DEFINITIONS.
//   - A repeat block's first line is listed under DEFINITIONS. The lines
//     it repeats are listed under DEFINITIONS too, or when the
//     repetitions themselves aren't listed (neither EXPANSIONS nor
//     BINARY). Its .ENDR is always listed.
//   - A conditional directive, and a line a conditional left out, are
//     listed under CONDITIONALS; .IIF is a conditional directive even
//     when the statement it guards stores bytes.
//   - A macro call is listed under CALLS.
func listShown(l *listLine) bool {
	s := l.show

	switch {
	case l.levelChange:
		return l.levelAfter > 0
	case l.page && !l.skipped && !l.collected:
		return false
	case s.level < 0:
		return false
	case s.level > 0:
		return true
	}

	if l.depth > 0 {
		switch {
		case s.has(showExpansions):
		case s.has(showBinary):
			return !l.skipped && !l.collected && len(l.fields) > 0
		default:
			return false
		}
	}

	switch {
	case l.def == defMacro:
		return s.has(showDefinitions)
	case l.def == defRepeat && l.defStart:
		return s.has(showDefinitions)
	case l.def == defRepeat && !l.defEnd:
		return s.has(showDefinitions) || !s.has(showExpansions|showBinary)
	case l.def == defRepeat:
		return true
	case l.conditional || l.skipped:
		return s.has(showConditional)
	case l.call:
		return s.has(showCalls)
	}

	return true
}

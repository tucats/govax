package asm

import (
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Program section attributes, as the object language's GPS$M_ flag bits
// (a PSC record's flags word). USR is LIB's absence, and each NO form is
// the flag's absence.
var (
	gpsPIC = vmsdef.OBJConstants["GPS$M_PIC"]
	gpsLIB = vmsdef.OBJConstants["GPS$M_LIB"]
	gpsOVR = vmsdef.OBJConstants["GPS$M_OVR"]
	gpsREL = vmsdef.OBJConstants["GPS$M_REL"]
	gpsGBL = vmsdef.OBJConstants["GPS$M_GBL"]
	gpsSHR = vmsdef.OBJConstants["GPS$M_SHR"]
	gpsEXE = vmsdef.OBJConstants["GPS$M_EXE"]
	gpsRD  = vmsdef.OBJConstants["GPS$M_RD"]
	gpsWRT = vmsdef.OBJConstants["GPS$M_WRT"]
	gpsVEC = vmsdef.OBJConstants["GPS$M_VEC"]
)

// psectAttribute is one .PSECT attribute keyword: the flag it sets or
// clears.
type psectAttribute struct {
	flag uint32
	set  bool
}

// psectAttributes is every .PSECT attribute keyword (the MACRO manual,
// Table 6-6), with its opposite.
var psectAttributes = map[string]psectAttribute{
	"PIC": {gpsPIC, true}, "NOPIC": {gpsPIC, false},
	"LIB": {gpsLIB, true}, "USR": {gpsLIB, false},
	"OVR": {gpsOVR, true}, "CON": {gpsOVR, false},
	"REL": {gpsREL, true}, "ABS": {gpsREL, false},
	"GBL": {gpsGBL, true}, "LCL": {gpsGBL, false},
	"SHR": {gpsSHR, true}, "NOSHR": {gpsSHR, false},
	"EXE": {gpsEXE, true}, "NOEXE": {gpsEXE, false},
	"RD": {gpsRD, true}, "NORD": {gpsRD, false},
	"WRT": {gpsWRT, true}, "NOWRT": {gpsWRT, false},
	"VEC": {gpsVEC, true}, "NOVEC": {gpsVEC, false},
}

// Default psect attributes (the MACRO manual, .PSECT, Table 6-7 and note
// 2): a named psect, and . BLANK ., are CON, EXE, LCL, NOPIC, NOSHR, RD,
// REL, USR, WRT, and NOVEC. . ABS . has none of the flags (NOPIC, USR,
// CON, ABS, LCL, NOSHR, NOEXE, NORD, NOWRT, NOVEC). All are BYTE aligned.
var defaultPsectFlags = gpsREL | gpsEXE | gpsRD | gpsWRT

// alignKeywords are the alignment keywords .PSECT and .ALIGN take, as
// powers of two.
var alignKeywords = map[string]uint32{"BYTE": 0, "WORD": 1, "LONG": 2, "QUAD": 3, "PAGE": 9}

// maxAlign is the largest alignment, as a power of two: PAGE.
const maxAlign = 9

// maxUserPsects is the most psects a module can define besides the two
// default ones (the MACRO manual, .PSECT), which keeps every psect index
// in the byte the object language's short forms have for it.
const maxUserPsects = 254

// psectContext is one .SAVE_PSECT entry: the psect and its location,
// whether that was the implicit . ABS . that code moves out of, and, with
// LOCAL_BLOCK, the local label block.
type psectContext struct {
	sect        *section
	loc         uint32
	implicitAbs bool
	withBlock   bool
	block       int
	used        bool
}

// maxPsectStack is how many contexts .SAVE_PSECT can hold.
const maxPsectStack = 31

// useBlankPsect moves a MACRO-dialect assembly that hasn't chosen a psect
// yet from . ABS . into . BLANK ., defining it, now that code, data, or a
// label needs a relocatable location (the MACRO manual, .PSECT). Only
// symbol definitions go in . ABS . until then.
func (a *Assembler) useBlankPsect() {
	if !a.implicitAbs {
		return
	}

	a.implicitAbs = false
	a.enterSection(a.psect(blankPsect, defaultPsectFlags, 0))

	if n := len(a.events); n > 0 {
		a.events[n-1].implicit = true
	}
}

// psect returns the psect named name, defining it with flags and align
// if it's new.
func (a *Assembler) psect(name string, flags, align uint32) *section {
	if s := a.findSection(name); s != nil {
		return s
	}

	s := a.newSection(name, flags&gpsREL != 0, newImage(), 0)
	s.flags, s.align = flags, align

	return s
}

// pseudoPsect assembles .PSECT [name[,attribute...]]: continues the
// program section name, defining it the first time with the default
// attributes as the list changes them, or . BLANK . when no name is
// given. A continuation may repeat attributes, but can't change them.
func (a *Assembler) pseudoPsect(c *cursor) error {
	c.skipBlanks()

	name := scanPsectName(c)
	if name == "" {
		name = blankPsect
	}

	flags, align := defaultPsectFlags, uint32(0)

	// Which attributes the list named, so a continuation checks only
	// those.
	var named uint32

	alignNamed := false

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()
			c.skipBlanks()
		}

		if c.atEnd() {
			break
		}

		word := scanName(c)
		if word == "" {
			return vmserrors.New(vmserrors.VAX_PSECTATTR, c.rest())
		}

		if attr, ok := psectAttributes[word]; ok {
			named |= attr.flag

			if attr.set {
				flags |= attr.flag
			} else {
				flags &^= attr.flag
			}

			continue
		}

		n, ok := alignmentValue(word)
		if !ok {
			return vmserrors.New(vmserrors.VAX_PSECTATTR, word)
		}

		align, alignNamed = n, true
	}

	if err := a.endLocalBlock(); err != nil {
		return err
	}

	a.implicitAbs = false

	if s := a.findSection(name); s != nil {
		if diff := (s.flags ^ flags) & named; diff != 0 {
			return vmserrors.New(vmserrors.VAX_PSECTCONFLICT, attributeName(diff, flags), name)
		}

		if alignNamed && align != s.align {
			return vmserrors.New(vmserrors.VAX_PSECTCONFLICT, "alignment", name)
		}

		a.enterSection(s)

		return nil
	}

	user := len(a.sections) - 1 // all but . ABS .
	if a.findSection(blankPsect) != nil {
		user--
	}

	if name != blankPsect && user >= maxUserPsects {
		return vmserrors.New(vmserrors.VAX_TOOMANYPSECTS, maxUserPsects)
	}

	a.enterSection(a.psect(name, flags, align))

	return nil
}

// scanPsectName reads a psect name: like a symbol, but it may also
// contain "." (the MACRO manual, .PSECT).
func scanPsectName(c *cursor) string {
	start := c.pos

	for isSymbolChar(c.peek()) || c.peek() == '.' {
		c.pos++
	}

	return c.s[start:c.pos]
}

// attributeName names the attribute in diff (one or more flags) as flags
// has it, for an error message.
func attributeName(diff, flags uint32) string {
	for name, attr := range psectAttributes {
		if diff&attr.flag != 0 && attr.set == (flags&attr.flag != 0) {
			return name
		}
	}

	return "attribute"
}

// alignmentValue reads an alignment, a keyword or an integer from 0 to 9,
// as a power of two.
func alignmentValue(word string) (uint32, bool) {
	if n, ok := alignKeywords[word]; ok {
		return n, true
	}

	if len(word) == 1 && isDigit(word[0]) {
		return uint32(word[0] - '0'), true
	}

	return 0, false
}

// pseudoSavePsect assembles .SAVE_PSECT [LOCAL_BLOCK]: pushes the current
// psect and location, and with LOCAL_BLOCK the local label block, on the
// psect context stack.
func (a *Assembler) pseudoSavePsect(c *cursor) error {
	c.skipBlanks()

	ctx := psectContext{sect: a.cur, loc: a.cur.loc, implicitAbs: a.implicitAbs}

	if !c.atEnd() {
		if word := scanName(c); word != "LOCAL_BLOCK" {
			return vmserrors.New(vmserrors.VAX_BADKEYWORD, ".SAVE_PSECT", word)
		}

		ctx.withBlock, ctx.block, ctx.used = true, a.localBlock, a.localUsed
	}

	if len(a.psectStack) >= maxPsectStack {
		return vmserrors.New(vmserrors.VAX_PSECTSTACK, "full")
	}

	a.psectStack = append(a.psectStack, ctx)

	return nil
}

// pseudoRestorePsect assembles .RESTORE_PSECT: returns to the psect and
// location on top of the psect context stack, and to its local label
// block if it saved one.
func (a *Assembler) pseudoRestorePsect(*cursor) error {
	n := len(a.psectStack)
	if n == 0 {
		return vmserrors.New(vmserrors.VAX_PSECTSTACK, "empty")
	}

	ctx := a.psectStack[n-1]
	a.psectStack = a.psectStack[:n-1]

	ctx.sect.loc = ctx.loc
	a.enterSection(ctx.sect)
	a.implicitAbs = ctx.implicitAbs

	if ctx.withBlock {
		if err := a.checkLocalBlock(); err != nil {
			return err
		}

		a.localBlock, a.localUsed = ctx.block, ctx.used
	}

	return nil
}

// blockSaved reports whether .SAVE_PSECT LOCAL_BLOCK saved local label
// block n.
func (a *Assembler) blockSaved(n int) bool {
	for _, ctx := range a.psectStack {
		if ctx.withBlock && ctx.block == n {
			return true
		}
	}

	return false
}

// pseudoAlignMACRO assembles MACRO-32's .ALIGN integer|keyword[,fill]:
// moves the location up to a multiple of 2^integer (or the keyword's
// size), which can't be more than the psect's own alignment. With a fill
// expression, the bytes skipped are set to it; otherwise they are a gap.
func (a *Assembler) pseudoAlignMACRO(c *cursor) error {
	c.skipBlanks()

	save := c.pos

	power, ok := alignmentValue(scanName(c))
	if !ok {
		c.pos = save

		n, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		if n > maxAlign {
			return vmserrors.New(vmserrors.VAX_DATARANGE, ".ALIGN", n)
		}

		power = n
	}

	fill, hasFill := uint32(0), false

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()

		v, err := a.exprNoForward(c)
		if err != nil {
			return err
		}

		fill, hasFill = v, true
	}

	a.useBlankPsect()

	if power > a.cur.align {
		return vmserrors.New(vmserrors.VAX_ALIGNPSECT, power, a.cur.name)
	}

	size := uint32(1) << power
	pad := (size - a.cur.loc%size) % size

	if !hasFill {
		a.advance(pad)

		return nil
	}

	for ; pad > 0; pad-- {
		if err := a.emitByte(byte(fill)); err != nil {
			return err
		}
	}

	return nil
}

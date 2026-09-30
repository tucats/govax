// Package link is govax's VAX linker: it builds a VMS executable image from
// object modules, as the VMS LINK command does (docs/PHASE-30.md). It reads
// modules through internal/obj, so it links govax's objects and real VAX
// objects alike.
//
// Linking follows the VMS 5.0 Linker Utility Manual's chapter 6:
//
//   - Pass 1 reads each module's global symbol directory: its program
//     sections (psects), which modules contribute to by name, and the global
//     symbols it defines and refers to.
//   - Allocation groups the psects into image sections by their writability,
//     executability, and vector attributes, and gives each psect and image
//     section an address, starting at 0x200.
//   - Pass 2 runs each module's text information and relocation (TIR)
//     commands on the linker's stack machine, which stores the module's
//     bytes, relocated, into the image sections.
//   - Last, writable pages nothing was stored in become demand-zero, the
//     fixup section is added, and the image header is written.
//
// Traceback and debugger records are read and skipped: the image has no
// debug symbol table. A symbol the modules refer to but don't define comes
// from the symbol sources (source.go): an absolute value, or a routine in
// a shareable image, which a general mode (G^) operand reaches through a
// cell in the fixup section that the image activator fills in.
package link

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// Input is one object module to link, and the file it came from (for
// messages).
type Input struct {
	File   string
	Module *obj.Module
}

// Options are the parts of an image that don't come from its modules.
type Options struct {
	// ImageName is the image's name (the executable file's name, without
	// its type).
	ImageName string
	// Time is the link time; the zero time means now.
	Time time.Time
	// LinkerID identifies the linker in the image header, as real LINK
	// writes its version ("V11-39"). At most 15 characters; "" means
	// "govax".
	LinkerID string
	// Traceback makes SYS$IMGSTA the image's first transfer address, as
	// LINK/TRACEBACK (the default) does.
	Traceback bool
	// StackPages is the user stack's size; 0 means 20 pages, LINK's
	// default.
	StackPages int
	// Sources define the symbols the modules refer to but don't, searched
	// in order.
	Sources []SymbolSource
}

// Image is a linked executable image.
type Image struct {
	// Bytes is the image file's contents, a whole number of blocks.
	Bytes []byte
	// Psects are the image's program sections, by address.
	Psects []PsectInfo
	// Transfer is the user transfer address, if the image has one.
	Transfer    uint32
	HasTransfer bool
}

// PsectInfo describes one program section of the image, for a map.
type PsectInfo struct {
	Name   string
	Base   uint32
	Length uint32
	Align  byte
	Flags  uint16
}

// sysImgsta is SYS$IMGSTA's address in the P1 vector: the image activator
// calls it first in an image linked with traceback.
var sysImgsta = func() uint32 {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$IMGSTA" {
			return e.Addr
		}
	}

	panic("link: no SYS$IMGSTA in the P1 vector table")
}()

// imageBase is where an executable image's first cluster starts.
const imageBase = 0x200

// Link links inputs into an executable image.
func Link(inputs []Input, opts Options) (*Image, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("link: no object modules")
	}

	if opts.Time.IsZero() {
		opts.Time = time.Now()
	}

	if opts.LinkerID == "" {
		opts.LinkerID = "govax"
	}

	if opts.StackPages == 0 {
		opts.StackPages = defaultStackPages
	}

	l := &linker{opts: opts, psects: map[string]*psect{}, symbols: map[string]*global{}}

	for i := range inputs {
		if err := l.pass1(&inputs[i]); err != nil {
			return nil, err
		}
	}

	if err := l.checkUndefined(); err != nil {
		return nil, err
	}

	l.allocate()

	for _, m := range l.modules {
		if err := l.pass2(m); err != nil {
			return nil, err
		}
	}

	return l.image()
}

// linker holds one link's state.
type linker struct {
	opts Options

	modules []*module

	// psects are the image's program sections by name, and order the
	// order they were first seen in.
	psects map[string]*psect
	order  []*psect

	symbols map[string]*global

	// sections are the image sections, by address.
	sections []*section

	// shared are the shareable images the image refers to, in the order
	// first referred to, and gRefs the general mode references to them.
	shared []*sharedRef
	gRefs  []gRef

	transfer        uint32
	transferSet     bool
	transferWk      bool
	pendingTransfer pendingTransfer
	imageID         string
}

// module is one input module during the link.
type module struct {
	input *Input
	name  string
	ident string
	// contribs are the module's psect contributions, by its own psect
	// index.
	contribs []*contribution
}

// psect is one program section of the image, which modules contribute to.
type psect struct {
	name     string
	flags    uint16
	align    byte
	contribs []*contribution
	base     uint32
	length   uint32
	section  *section
}

// contribution is one module's part of a psect.
type contribution struct {
	psect  *psect
	size   uint32
	align  byte
	offset uint32 // from the psect's base
}

// base is the contribution's address.
func (c *contribution) base() uint32 { return c.psect.base + c.offset }

// global is a global symbol.
type global struct {
	name    string
	defined bool
	weak    bool
	// value is the symbol's address or absolute value, once allocation
	// has placed its psect.
	value   uint32
	contrib *contribution // nil for an absolute symbol
	offset  uint32
	mask    uint16 // an entry point's register save mask
	entry   bool
	// image is the shareable image a symbol from a source is in; its
	// value is its offset there.
	image string
	refs    []string // modules that refer to it

	// module and psectIndex are where a relocatable definition is, until
	// the module's psects are all known: real MACRO writes its global
	// symbols before its psect definitions.
	module     *module
	psectIndex int
	rel        bool
}

// pass1 reads a module's global symbol directory and end of module
// record.
func (l *linker) pass1(in *Input) error {
	m := &module{input: in}
	l.modules = append(l.modules, m)

	for _, rec := range in.Module.Records {
		switch r := rec.(type) {
		case *obj.MainHeader:
			m.name, m.ident = r.Name, r.Version

		case *obj.GSD:
			for _, sub := range r.Subrecords {
				if err := l.gsdEntry(m, sub); err != nil {
					return fmt.Errorf("link: %s: %w", in.File, err)
				}
			}

		case *obj.EOM:
			if r.HasTransfer {
				if err := l.noteTransfer(m, r); err != nil {
					return err
				}
			}
		}
	}

	for _, g := range l.symbols {
		if g.module != m || !g.rel {
			continue
		}

		if g.psectIndex >= len(m.contribs) {
			return fmt.Errorf("link: %s: %s is in psect %d, which the module doesn't define", in.File, g.name, g.psectIndex)
		}

		g.contrib = m.contribs[g.psectIndex]
	}

	if l.imageID == "" && !l.transferSet {
		l.imageID = m.ident
	}

	return nil
}

// gsdEntry records one GSD subrecord.
func (l *linker) gsdEntry(m *module, sub obj.Subrecord) error {
	switch e := sub.(type) {
	case *obj.Psect:
		if e.Shared {
			return fmt.Errorf("shareable image psect %s isn't supported yet", e.Name)
		}

		p, ok := l.psects[e.Name]
		if !ok {
			p = &psect{name: e.Name, flags: e.Flags}
			l.psects[e.Name] = p
			l.order = append(l.order, p)
		}

		c := &contribution{psect: p, size: e.Alloc, align: e.Align}
		p.contribs = append(p.contribs, c)
		m.contribs = append(m.contribs, c)

		if e.Align > p.align {
			p.align = e.Align
		}

	case *obj.Symbol:
		return l.symbol(m, e)
	}

	return nil
}

// symbol records a symbol definition or reference.
func (l *linker) symbol(m *module, s *obj.Symbol) error {
	g, ok := l.symbols[s.Name]
	if !ok {
		g = &global{name: s.Name}
		l.symbols[s.Name] = g
	}

	if !s.Defined() {
		g.refs = append(g.refs, m.name)

		return nil
	}

	weak := s.Flags&obj.SymWEAK != 0

	switch {
	case g.defined && weak:
		return nil // a weak definition never replaces one
	case g.defined && !g.weak:
		return fmt.Errorf("%s is defined more than once", s.Name)
	}

	g.defined, g.weak = true, weak
	g.offset, g.contrib = s.Value, nil
	g.entry = s.GSDType() == obj.GSDEntry
	g.mask = s.Mask
	g.module, g.psectIndex, g.rel = m, int(s.Psect), s.Flags&obj.SymREL != 0

	return nil
}

// noteTransfer records a module's transfer address. A strong transfer
// address takes over from a weak one; a second strong one is an error. The
// image's identification is the ident of the module with the transfer
// address (the Linker manual, IDENTIFICATION=).
func (l *linker) noteTransfer(m *module, eom *obj.EOM) error {
	weak := eom.HasFlags && eom.Flags&1 != 0

	switch {
	case l.transferSet && weak:
		return nil
	case l.transferSet && !l.transferWk:
		return fmt.Errorf("link: %s: a second transfer address", m.input.File)
	}

	if int(eom.Psect) >= len(m.contribs) {
		return fmt.Errorf("link: %s: transfer address in psect %d, which the module hasn't defined", m.input.File, eom.Psect)
	}

	l.transferSet, l.transferWk = true, weak
	l.pendingTransfer = pendingTransfer{contrib: m.contribs[eom.Psect], offset: eom.Transfer}
	l.imageID = m.ident

	return nil
}

// checkUndefined looks each symbol the modules refer to but don't define
// up in the symbol sources, and reports the ones none of them defines.
func (l *linker) checkUndefined() error {
	var undefined []string

	for name, g := range l.symbols {
		if g.defined {
			continue
		}

		if d, ok := l.lookup(name); ok {
			g.defined, g.image, g.offset = true, d.Image, d.Value

			continue
		}

		undefined = append(undefined, name)
	}

	if len(undefined) == 0 {
		return nil
	}

	sort.Strings(undefined)

	return fmt.Errorf("link: undefined symbols: %s", strings.Join(undefined, ", "))
}

// lookup finds a symbol in the first source that defines it.
func (l *linker) lookup(name string) (Definition, bool) {
	for _, src := range l.opts.Sources {
		if d, ok := src.Lookup(name); ok {
			return d, true
		}
	}

	return Definition{}, false
}

// sharedImage returns what the first source that knows the shareable
// image name says about it. An image no source describes is recorded by
// name only, with a match control that accepts any ident.
func (l *linker) sharedImage(name string) SharedImage {
	for _, src := range l.opts.Sources {
		if i, ok := src.Image(name); ok {
			i.Name = name

			return i
		}
	}

	return SharedImage{Name: name, Match: MatchAlways}
}

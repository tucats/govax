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
// With traceback (Options.Traceback, LINK's default), pass 2 also runs
// each module's traceback (TBT) records, whose bytes go into the image's
// debug symbol table (DST) instead of its sections: the modules' DST
// records, in link order, with their addresses resolved. The DST follows
// the image's other blocks, and the header's IHS block points at it (the
// Linker manual, 7.7 and 7.8). A module's debugger (DBG) records are
// skipped, unless the link is LINK/DEBUG (Options.Debug): then they go
// into the DST too, with the TBT records, in record order (docs/
// PHASE-29.md, subtasks 16 to 20). A symbol the modules refer to but
// don't define comes
// from the symbol sources (source.go): an absolute value, or a routine in
// a shareable image, which a general mode (G^) operand reaches through a
// cell in the fixup section that the image activator fills in.
package link

import (
	"fmt"
	"sort"
	"time"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

const platformName = "govax"

// Input is one object module to link, and the file it came from (for
// messages).
type Input struct {
	File   string
	Module *obj.Module
	// Selective marks a module from an object library that is searched
	// selectively (LIBRARY/INSERT/SELECTIVE_SEARCH): the link takes only
	// its definitions of symbols already referred to, as STARLET.OLB's
	// modules are taken.
	Selective bool
	// System marks a module from a system library (STARLET.OLB), which a
	// default map leaves out; a module from the user's own library is
	// listed.
	System bool
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
	// Debug is LINK/DEBUG: the modules' debugger (DBG) records go into
	// the debug symbol table with their traceback records, and the image
	// header asks for the debugger (IHD$V_LNKDEBUG). It turns Traceback
	// on, as the Linker manual says /DEBUG does even with /NOTRACEBACK.
	Debug bool
	// StackPages is the user stack's size; 0 means 20 pages, LINK's
	// default.
	StackPages int
	// Sources define the symbols the modules refer to but don't, searched
	// in order.
	Sources []SymbolSource
	// Ident is the image's identification (IDENTIFICATION=); "" means the
	// ident of the module with the transfer address, or of the first.
	Ident string
	// Symbols are absolute global symbols an options file defines
	// (SYMBOL=), which take precedence over the modules' definitions.
	Symbols []Symbol
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

	// Messages are the warnings and information the link reported, as
	// real LINK reports them: undefined symbols, and each reference to
	// one. A link with warnings still makes an image, as real LINK does.
	Messages []Message

	// l is the link that made the image, for its map (mapfile.go).
	l *linker
}

// PsectInfo describes one program section of the image, for a map.
type PsectInfo struct {
	Name   string
	Base   uint32
	Length uint32
	Align  byte
	Flags  uint16
}

// Message is a message a link reports, in real LINK's words
// (linker/lis/linkmsg.msg).
type Message struct {
	Severity byte // 'W' for a warning, 'I' for information
	Ident    string
	// Text is the message's text, which may run to more lines, each
	// after a newline.
	Text string
}

// String formats the message as VMS does: %LINK-W-IDENT, text.
func (m Message) String() string {
	return fmt.Sprintf("%%LINK-%c-%s, %s", m.Severity, m.Ident, m.Text)
}

// imgstaName is the routine the image activator calls first in an image
// linked with traceback, and sysImgsta its address in the P1 vector.
const imgstaName = "SYS$IMGSTA"

var sysImgsta = func() uint32 {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == imgstaName {
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
		opts.LinkerID = platformName
	}

	if opts.StackPages == 0 {
		opts.StackPages = defaultStackPages
	}

	if opts.Debug {
		opts.Traceback = true
	}

	l := &linker{opts: opts, psects: map[string]*psect{}, symbols: map[string]*global{}}

	for _, s := range opts.Symbols {
		g := l.refer(s.Name, "")
		g.defined, g.option, g.offset = true, true, s.Value
	}

	for i := range inputs {
		if _, err := l.pass1(&inputs[i], false); err != nil {
			return nil, err
		}
	}

	// With traceback, the image refers to SYS$IMGSTA, which real LINK
	// finds in STARLET.OLB like any other symbol.
	if opts.Traceback {
		l.refer(imgstaName, "").strongRef = true
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
	// addressFixups counts the .ADDRESS longwords that hold offsets in
	// shareable images.
	addressFixups int

	transfer        uint32
	transferSet     bool
	transferWk      bool
	pendingTransfer pendingTransfer
	imageID         string

	// undefined are the symbols referred to (not only weakly) that
	// nothing defines, and messages the link's messages that belong to no
	// module.
	undefined []string
	messages  []Message

	// What image() laid out, for the map: the image sections, and the
	// fixup section's address and size.
	isdCount    int
	fixupVA     uint32
	fixupLength uint32
	imageBlocks uint32

	// dst is the debug symbol table pass 2 builds from the modules'
	// traceback records, and dstVBN its first block in the image file
	// (0 if there's none).
	dst    []byte
	dstVBN uint32
}

// module is one input module during the link.
type module struct {
	input *Input
	name  string
	ident string
	// library says an object library supplied the module, rather than
	// the command; a default map lists only the command's modules.
	library bool
	// created and language are its creation time and language processor
	// (MHD and LNM), for the map.
	created  string
	language string
	// messages are the messages about the module, such as each
	// reference it makes to an undefined symbol.
	messages []Message
	// deferred are a selectively searched module's definitions of symbols
	// nothing referred to when it was read, in case something does later.
	deferred map[string]*obj.Symbol
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
	name string
	seq  int // the order it was first seen in
	// defined and weak say whether and how it's defined; fromSource, that
	// a symbol source defined it.
	defined    bool
	weak       bool
	fromSource bool
	// option says an options file defined it (SYMBOL=), which the
	// modules' definitions don't replace.
	option bool
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
	refs  []string // modules that refer to it
	// strongRef says something refers to it other than weakly. A symbol
	// only referred to weakly is 0, silently, if nothing defines it, and
	// isn't looked for in the symbol sources.
	strongRef bool

	// module and psectIndex are where a relocatable definition is, until
	// the module's psects are all known: real MACRO writes its global
	// symbols before its psect definitions.
	module     *module
	psectIndex int
	rel        bool
}

// pass1 reads a module's global symbol directory and end of module
// record. library says an object library supplied it.
func (l *linker) pass1(in *Input, library bool) (*module, error) {
	m := &module{input: in, library: library}
	l.modules = append(l.modules, m)

	for _, rec := range in.Module.Records {
		switch r := rec.(type) {
		case *obj.MainHeader:
			m.name, m.ident, m.created = r.Name, r.Version, r.Created

		case *obj.TextHeader:
			if r.Type == obj.HdrLNM {
				m.language = r.Text
			}

		case *obj.GSD:
			for _, sub := range r.Subrecords {
				if err := l.gsdEntry(m, sub); err != nil {
					return nil, fmt.Errorf("link: %s: %w", in.File, err)
				}
			}

		case *obj.EOM:
			if r.HasTransfer {
				if err := l.noteTransfer(m, r); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := l.resolvePsects(m); err != nil {
		return nil, err
	}

	if l.imageID == "" && !l.transferSet {
		l.imageID = m.ident
	}

	return m, nil
}

// resolvePsects gives each relocatable symbol the module defines its
// psect contribution, once the module's psects are all known.
func (l *linker) resolvePsects(m *module) error {
	for _, g := range l.symbols {
		if g.module != m || !g.rel || g.contrib != nil {
			continue
		}

		if g.psectIndex >= len(m.contribs) {
			return fmt.Errorf("link: %s: %s is in psect %d, which the module doesn't define", m.input.File, g.name, g.psectIndex)
		}

		g.contrib = m.contribs[g.psectIndex]
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

// refer records a reference to the global symbol name from the module
// named by, and returns the symbol.
func (l *linker) refer(name, by string) *global {
	g, ok := l.symbols[name]
	if !ok {
		g = &global{name: name, seq: len(l.symbols)}
		l.symbols[name] = g
	}

	if by != "" {
		g.refs = append(g.refs, by)
	}

	return g
}

// symbol records a symbol definition or reference.
func (l *linker) symbol(m *module, s *obj.Symbol) error {
	if !s.Defined() {
		if g := l.refer(s.Name, m.name); s.Flags&obj.SymWEAK == 0 {
			g.strongRef = true
		}

		return nil
	}

	if m.input.Selective && l.symbols[s.Name] == nil {
		if m.deferred == nil {
			m.deferred = map[string]*obj.Symbol{}
		}

		m.deferred[s.Name] = s

		return nil
	}

	g := l.refer(s.Name, "")
	if g.option {
		return nil
	}

	weak := s.Flags&obj.SymWEAK != 0

	switch {
	case g.fromSource:
		// A module added from a library defines what a later source
		// defined first; the module's definition is the one to use.
	case g.defined && weak:
		return nil // a weak definition never replaces one
	case g.defined && !g.weak:
		return fmt.Errorf("%s is defined more than once", s.Name)
	}

	g.defined, g.fromSource, g.image, g.weak = true, false, "", weak
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
// up in the symbol sources, and reports the ones none of them defines. A
// module from an object library is added to the link (pass 1 reads it),
// which defines the symbol and may refer to more, so the search goes on
// until nothing new is added. Symbols are looked up in the order they were
// first referred to, so library modules are added in a fixed order.
func (l *linker) checkUndefined() error {
	added := map[*Input]string{}
	modules := map[*Input]*module{}

	for {
		progress := false

		for _, g := range l.bySequence() {
			if g.defined || !g.strongRef {
				continue
			}

			d, ok, err := l.lookup(g.name)
			if err != nil {
				return fmt.Errorf("link: %s: %w", g.name, err)
			}

			if !ok {
				continue
			}

			if d.Module == nil {
				g.defined, g.fromSource, g.image, g.offset = true, true, d.Image, d.Value

				continue
			}

			if first, ok := added[d.Module]; ok {
				// A selectively searched module already added may have
				// passed over the definition, as nothing referred to it
				// then.
				m := modules[d.Module]

				s := m.deferred[g.name]
				if s == nil {
					return fmt.Errorf("link: %s was added for %s, but doesn't define %s", d.Module.File, first, g.name)
				}

				delete(m.deferred, g.name)

				if err := l.symbol(m, s); err != nil {
					return fmt.Errorf("link: %s: %w", d.Module.File, err)
				}

				if err := l.resolvePsects(m); err != nil {
					return err
				}

				progress = true

				continue
			}

			added[d.Module] = g.name

			m, err := l.pass1(d.Module, true)
			if err != nil {
				return err
			}

			modules[d.Module] = m
			progress = true
		}

		if !progress {
			break
		}
	}

	// With no STARLET.OLB, and no other source, SYS$IMGSTA is where the
	// P1 vector has it.
	if g := l.symbols[imgstaName]; g != nil && !g.defined && l.opts.Traceback {
		g.defined, g.fromSource, g.offset = true, true, sysImgsta
	}

	// A symbol still undefined is 0, and each reference to it is a
	// warning (pass 2), but the link goes on, as real LINK's does.
	for name, g := range l.symbols {
		if !g.defined && g.strongRef {
			l.undefined = append(l.undefined, name)
		}
	}

	if len(l.undefined) == 0 {
		return nil
	}

	sort.Strings(l.undefined)

	plural := "s"
	if len(l.undefined) == 1 {
		plural = ""
	}

	l.messages = append(l.messages, Message{'W', "NUDFSYMS", fmt.Sprintf("%d undefined symbol%s:", len(l.undefined), plural)})

	for _, name := range l.undefined {
		l.messages = append(l.messages, Message{'I', "UDFSYM", "\t" + name + " "})
	}

	return nil
}

// bySequence returns the global symbols in the order they were first seen.
func (l *linker) bySequence() []*global {
	all := make([]*global, 0, len(l.symbols))
	for _, g := range l.symbols {
		all = append(all, g)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].seq < all[j].seq })

	return all
}

// lookup finds a symbol in the first source that defines it.
func (l *linker) lookup(name string) (Definition, bool, error) {
	for _, src := range l.opts.Sources {
		d, ok, err := src.Lookup(name)
		if err != nil || ok {
			return d, ok, err
		}
	}

	return Definition{}, false, nil
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

package link

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/tucats/govax/internal/obj"
)

// section is one image section: psects with the same significant
// attributes, together, at a page-aligned address.
type section struct {
	key    int
	psects []*psect
	base   uint32
	pages  uint32
	data   []byte
	// stored marks the pages something nonzero was stored in. A
	// writable page with nothing stored in it can be demand-zero.
	stored []bool
}

// writable reports whether the section's psects are writable.
func (s *section) writable() bool { return s.key&keyWRT != 0 }

// Significant attributes of an executable image's psects, as bits of an
// image section's key. The keys' numeric order is the order of image
// sections in a cluster (the Linker manual, Table 6-1): NOWRT before WRT,
// NOEXE before EXE, NOVEC before VEC, VEC the most significant.
const (
	keyWRT = 1 << iota
	keyEXE
	keyVEC
)

func sectionKey(flags uint16) int {
	key := 0

	if flags&obj.PsectWRT != 0 {
		key |= keyWRT
	}

	if flags&obj.PsectEXE != 0 {
		key |= keyEXE
	}

	if flags&obj.PsectVEC != 0 {
		key |= keyVEC
	}

	return key
}

// pendingTransfer is a transfer address waiting for its psect's address.
type pendingTransfer struct {
	contrib *contribution
	offset  uint32
}

// alignUp rounds v up to a multiple of 2^align.
func alignUp(v uint32, align byte) uint32 {
	mask := uint32(1)<<align - 1

	return (v + mask) &^ mask
}

// pageUp rounds v up to a page boundary.
func pageUp(v uint32) uint32 { return alignUp(v, 9) }

// allocate lays out each psect's contributions, groups the relocatable
// psects into image sections, and gives every psect, symbol, and the
// transfer address its address (the Linker manual, §6.3.4). Contributions
// to a concatenated psect follow one another, each at its own alignment;
// those to an overlaid psect all start at its base. Within an image
// section, psects are in alphabetical order.
func (l *linker) allocate() {
	groups := map[int]*section{}

	for _, p := range l.order {
		var end uint32

		for _, c := range p.contribs {
			if p.flags&obj.PsectOVR != 0 {
				c.offset = 0
				end = max(end, c.size)

				continue
			}

			c.offset = alignUp(end, c.align)
			end = c.offset + c.size
		}

		p.length = end

		// An absolute psect's contributions are only offsets, from 0.
		if p.flags&obj.PsectREL == 0 {
			continue
		}

		key := sectionKey(p.flags)
		if groups[key] == nil {
			groups[key] = &section{key: key}
		}

		groups[key].psects = append(groups[key].psects, p)
	}

	keys := make([]int, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	va := uint32(imageBase)

	for _, k := range keys {
		s := groups[k]

		sort.SliceStable(s.psects, func(i, j int) bool { return s.psects[i].name < s.psects[j].name })

		va = pageUp(va)
		s.base = va

		for _, p := range s.psects {
			va = alignUp(va, p.align)
			p.base, p.section = va, s
			va += p.length
		}

		if va == s.base {
			continue // nothing in it
		}

		s.pages = (pageUp(va) - s.base) / blockSize
		s.data = make([]byte, s.pages*blockSize)
		s.stored = make([]bool, s.pages)
		l.sections = append(l.sections, s)
	}

	for _, g := range l.symbols {
		g.value = g.offset
		if g.contrib != nil {
			g.value += g.contrib.base()
		}
	}

	if l.transferSet {
		l.transfer = l.pendingTransfer.contrib.base() + l.pendingTransfer.offset
	}
}

// value is a value on the linker's stack, and whether it's an address in
// the image (which moves with the image), an offset in a shareable image
// (img names it), or an absolute value.
type value struct {
	v   uint32
	rel bool
	img string
}

// store is how a store command writes the value it pops.
type store struct {
	size int
	// disp makes it a displacement from the end of the field.
	disp bool
	// signed limits the value to the signed range.
	signed bool
}

var stores = map[string]store{
	"STO_B": {size: 1}, "STO_W": {size: 2}, "STO_L": {size: 4},
	"STO_SB": {size: 1, signed: true}, "STO_SW": {size: 2, signed: true},
	"STO_BD": {size: 1, disp: true}, "STO_WD": {size: 2, disp: true}, "STO_LD": {size: 4, disp: true},
	// In an executable image, a position-independent data reference is
	// the address itself.
	"STO_PIDR": {size: 4},
}

// binaryOps are the arithmetic commands.
var binaryOps = map[string]func(a, b uint32) uint32{
	"OPR_ADD": func(a, b uint32) uint32 { return a + b },
	"OPR_SUB": func(a, b uint32) uint32 { return a - b },
	"OPR_MUL": func(a, b uint32) uint32 { return a * b },
	"OPR_DIV": func(a, b uint32) uint32 {
		if b == 0 {
			return 0
		}

		return uint32(int32(a) / int32(b))
	},
	"OPR_AND": func(a, b uint32) uint32 { return a & b },
	"OPR_IOR": func(a, b uint32) uint32 { return a | b },
	"OPR_EOR": func(a, b uint32) uint32 { return a ^ b },
	"OPR_ASH": func(a, b uint32) uint32 {
		switch n := int32(b); {
		case n >= 32:
			return 0
		case n >= 0:
			return a << uint(n)
		case n <= -32:
			return uint32(int32(a) >> 31)
		default:
			return uint32(int32(a) >> uint(-n))
		}
	},
}

// machine runs one module's TIR commands.
type machine struct {
	l     *linker
	m     *module
	stack []value
	loc   uint32
}

func (x *machine) push(v value) { x.stack = append(x.stack, v) }

func (x *machine) pop() (value, error) {
	if len(x.stack) == 0 {
		return value{}, fmt.Errorf("the linker's stack is empty")
	}

	v := x.stack[len(x.stack)-1]
	x.stack = x.stack[:len(x.stack)-1]

	return v, nil
}

// pass2 runs a module's TIR records, storing its contents into the image
// sections. Debugger and traceback records are skipped.
func (l *linker) pass2(m *module) error {
	x := &machine{l: l, m: m}

	for _, rec := range m.input.Module.Records {
		switch r := rec.(type) {
		case *obj.TIR:
			if r.Type != obj.RecTIR {
				continue
			}

			for _, c := range r.Commands {
				if err := x.command(c); err != nil {
					return fmt.Errorf("link: %s: %s: %w", m.input.File, c.Op, err)
				}
			}

		case *obj.EOM:
			if len(x.stack) != 0 {
				return fmt.Errorf("link: %s: %d values left on the linker's stack", m.input.File, len(x.stack))
			}
		}
	}

	return nil
}

// command runs one TIR command.
func (x *machine) command(c obj.Command) error {
	if c.Op == obj.OpStoreImmediate {
		if err := x.l.write(x.loc, c.Data); err != nil {
			return err
		}

		x.loc += uint32(len(c.Data))

		return nil
	}

	name := c.Op.String()

	switch name {
	case "STA_UB", "STA_UW", "STA_LW", "STA_SB", "STA_SW":
		x.push(value{v: c.StackedValue()})

	case "STA_PB", "STA_PW", "STA_PL":
		if int(c.Psect) >= len(x.m.contribs) {
			return fmt.Errorf("psect %d isn't defined in the module", c.Psect)
		}

		contrib := x.m.contribs[c.Psect]
		x.push(value{v: contrib.base() + c.StackedValue(), rel: contrib.psect.flags&obj.PsectREL != 0})

	case "STA_GBL":
		g := x.l.symbols[c.Name]
		if g == nil || !g.defined {
			return fmt.Errorf("%s is undefined", c.Name)
		}

		x.push(value{v: g.value, rel: g.contrib != nil, img: g.image})

	case "STA_EPM":
		g := x.l.symbols[c.Name]
		if g == nil || !g.defined || !g.entry {
			return fmt.Errorf("%s isn't an entry point", c.Name)
		}

		x.push(value{v: uint32(g.mask)})

	case "OPR_NEG", "OPR_COM":
		a, err := x.pop()
		if err != nil {
			return err
		}

		if name == "OPR_NEG" {
			a.v = -a.v
		} else {
			a.v = ^a.v
		}

		x.push(a)

	case "CTL_SETRB":
		a, err := x.pop()
		if err != nil {
			return err
		}

		x.loc = a.v

	case "CTL_AUGRB":
		x.loc += c.StackedValue()

	case "STO_PICR":
		return x.storePICR()

	default:
		if op, ok := binaryOps[name]; ok {
			return x.binary(name, op)
		}

		if st, ok := stores[name]; ok {
			return x.store(st)
		}

		return fmt.Errorf("the command isn't supported yet")
	}

	return nil
}

// binary runs an arithmetic command. The difference of two addresses in
// the image is absolute; anything else involving one is an address.
func (x *machine) binary(name string, op func(a, b uint32) uint32) error {
	b, err := x.pop()
	if err != nil {
		return err
	}

	a, err := x.pop()
	if err != nil {
		return err
	}

	// An offset in a shareable image can only be moved by a constant.
	img := a.img
	if a.img != "" || b.img != "" {
		if b.img != "" || b.rel || a.rel || (name != "OPR_ADD" && name != "OPR_SUB") {
			return fmt.Errorf("arithmetic on an address in a shareable image isn't supported")
		}
	}

	rel := a.rel || b.rel
	if name == "OPR_SUB" && a.rel && b.rel {
		rel = false
	}

	x.push(value{v: op(a.v, b.v), rel: rel, img: img})

	return nil
}

// store runs a store command.
func (x *machine) store(st store) error {
	a, err := x.pop()
	if err != nil {
		return err
	}

	if a.img != "" {
		return fmt.Errorf("a reference to shareable image %s must be general mode (G^)", a.img)
	}

	v := int64(int32(a.v))
	if st.disp {
		v = int64(int32(a.v - (x.loc + uint32(st.size))))
	}

	if st.size < 4 {
		bits := uint(8 * st.size)
		lo, hi := int64(-1)<<(bits-1), int64(1)<<bits-1

		if st.signed || st.disp {
			hi = int64(1)<<(bits-1) - 1
		}

		if v < lo || v > hi {
			return fmt.Errorf("%d doesn't fit in %d bytes at %08X", v, st.size, x.loc)
		}
	}

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))

	if err := x.l.write(x.loc, b[:st.size]); err != nil {
		return err
	}

	x.loc += uint32(st.size)

	return nil
}

// storePICR stores a general mode (G^) operand: five bytes, starting at
// the addressing mode byte. An address in the image becomes relative mode
// (EF and a longword displacement), and an absolute one absolute mode (9F
// and the address). An address in a shareable image becomes longword
// relative deferred mode (FF) through a cell in the fixup section, which
// the image activator fills in with the address (the Linker manual,
// §6.3.6.2); its displacement is stored once the fixup section is laid
// out (see patchGRefs).
func (x *machine) storePICR() error {
	a, err := x.pop()
	if err != nil {
		return err
	}

	b := make([]byte, 5)

	switch {
	case a.img != "":
		b[0] = 0xFF
		x.l.referShared(a.img, a.v, x.loc+1)

	case a.rel:
		b[0] = 0xEF
		binary.LittleEndian.PutUint32(b[1:], a.v-(x.loc+5))
	default:
		b[0] = 0x9F
		binary.LittleEndian.PutUint32(b[1:], a.v)
	}

	if err := x.l.write(x.loc, b); err != nil {
		return err
	}

	x.loc += 5

	return nil
}

// write stores data at address addr, which must be in an image section.
func (l *linker) write(addr uint32, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	for _, s := range l.sections {
		end := s.base + s.pages*blockSize
		if addr < s.base || addr+uint32(len(data)) > end {
			continue
		}

		off := addr - s.base
		copy(s.data[off:], data)

		for i, b := range data {
			if b != 0 {
				s.stored[(off+uint32(i))/blockSize] = true
			}
		}

		return nil
	}

	return fmt.Errorf("%d bytes stored at %08X, outside every image section", len(data), addr)
}

// dzroMin is the fewest uninitialized pages the linker makes a separate
// demand-zero section of within a writable section (DZRO_MIN's default,
// the Linker manual §6.3.6.1). A writable section with nothing stored in
// it is demand-zero whatever its size.
const dzroMin = 5

// image lays out the image file: the header block, each image section's
// pages (writable pages with nothing stored in them become demand-zero),
// the fixup section, and the user stack.
func (l *linker) image() (*Image, error) {
	var (
		isds  []isd
		pages [][]byte
		end   uint32
	)

	vbn := uint32(2)

	// The fixup section follows the last section. Laying it out gives
	// each general mode operand's cell, whose displacement goes into the
	// code before the sections' pages are written out.
	for _, s := range l.sections {
		end = s.base + s.pages*blockSize
	}

	if end == 0 {
		end = imageBase
	}

	fixupVA := pageUp(end)
	layout := l.layoutFixups()

	if err := l.patchGRefs(layout, fixupVA); err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	fixup := l.fixupSection(layout, fixupVA)

	for _, s := range l.sections {
		for _, run := range s.runs() {
			vpn := (s.base >> 9) + run.first

			if run.demandZero {
				isds = append(isds, isd{pages: run.count, vpn: vpn, flags: isdLASTCLU | isdDZRO | isdWRT})

				continue
			}

			flags := uint32(isdLASTCLU)
			if s.writable() {
				flags |= isdWRT | isdCRF
			}

			isds = append(isds, isd{pages: run.count, vpn: vpn, flags: flags, vbn: vbn})
			pages = append(pages, s.data[run.first*blockSize:(run.first+run.count)*blockSize])
			vbn += run.count
		}
	}

	isds = append(isds,
		isd{pages: uint32(len(fixup)) / blockSize, vpn: fixupVA >> 9, flags: isdFIXUPVEC | isdWRT | isdCRF, vbn: vbn},
		isd{pages: uint32(l.opts.StackPages), vpn: 1<<22 - uint32(l.opts.StackPages), flags: isdTypeUserStack | isdLASTCLU | isdDZRO | isdWRT},
	)
	pages = append(pages, fixup)

	global := make([][]byte, 0, len(l.shared))

	for _, r := range l.shared {
		g, err := globalSectionISD(r.image)
		if err != nil {
			return nil, fmt.Errorf("link: %w", err)
		}

		global = append(global, g)
	}

	if l.opts.ImageName == "" {
		l.opts.ImageName = l.modules[0].name
	}

	header, err := l.header(isds, global, fixupVA)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	img := &Image{Transfer: l.transfer, HasTransfer: l.transferSet}
	img.Bytes = header

	for _, p := range pages {
		img.Bytes = append(img.Bytes, p...)
	}

	for _, p := range l.order {
		if p.flags&obj.PsectREL != 0 {
			img.Psects = append(img.Psects, PsectInfo{Name: p.name, Base: p.base, Length: p.length, Align: p.align, Flags: p.flags})
		}
	}

	sort.SliceStable(img.Psects, func(i, j int) bool { return img.Psects[i].Base < img.Psects[j].Base })

	return img, nil
}

// pageRun is a run of a section's pages that become one image section.
type pageRun struct {
	first, count uint32
	demandZero   bool
}

// runs splits a section into its image sections. A read-only section is
// one. A writable one with nothing stored in it is demand-zero; otherwise
// each run of at least dzroMin pages with nothing stored in them is split
// off as demand-zero.
func (s *section) runs() []pageRun {
	if !s.writable() {
		return []pageRun{{0, s.pages, false}}
	}

	var out []pageRun

	for p := uint32(0); p < s.pages; {
		q := p
		for q < s.pages && s.stored[q] == s.stored[p] {
			q++
		}

		zero := !s.stored[p] && (q-p >= dzroMin || (p == 0 && q == s.pages))

		if n := len(out); n > 0 && !zero && !out[n-1].demandZero {
			out[n-1].count += q - p
		} else {
			out = append(out, pageRun{p, q - p, zero})
		}

		p = q
	}

	return out
}

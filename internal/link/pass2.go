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
	key     int
	cluster *cluster
	psects  []*psect
	base    uint32
	pages   uint32
	data    []byte
	// fixedUp marks a read-only section of a shareable image that the
	// image activator stores .ADDRESS fixups in (shareable.go's
	// referSelf): it is writable and copy on reference until then.
	fixedUp bool
	// stored marks the pages something nonzero was stored in. A
	// writable page with nothing stored in it can be demand-zero.
	stored []bool
}

// writable reports whether the section's psects are writable.
func (s *section) writable() bool { return s.key&keyWRT != 0 }

// Significant attributes of a psect, as bits of an image section's key.
// The keys' numeric order is the order of image sections in a cluster
// (the Linker manual, Table 6-1): NOWRT before WRT, NOEXE before EXE, and
// NOVEC before VEC, VEC the most significant. In an executable image only
// those three count. A shareable image's add SHR and PIC: SHR NOPIC
// (SHRFXD), NOSHR NOPIC (PRVFXD), SHR PIC (SHRPIC), then NOSHR PIC
// (PRVPIC), each with the other attributes' four combinations in order
// (VSI's Linker manual, Table 7.3, and testdata/link68's SHR1 and SHR4).
const (
	keyWRT = 1 << iota
	keyEXE
	keyNOSHR
	keyPIC
	keyVEC
)

func (l *linker) sectionKey(flags uint16) int {
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

	if l.opts.Shareable {
		if flags&obj.PsectSHR == 0 {
			key |= keyNOSHR
		}

		if flags&obj.PsectPIC != 0 {
			key |= keyPIC
		}
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

// allocate lays out each psect's contributions, then each user cluster's
// image sections (cluster.go), gives the clusters their addresses, and
// then every psect, symbol, and the transfer address theirs (the Linker
// manual, 6.3.4). Contributions to a concatenated psect follow one
// another, each at its own alignment; those to an overlaid psect all start
// at its base.
func (l *linker) allocate() error {
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
	}

	for _, c := range l.clusters {
		if c.user() {
			l.layoutCluster(c)
		}
	}

	if err := l.placeClusters(); err != nil {
		return err
	}

	// A symbol in a shareable image the link placed is an address now,
	// which needs no fixup.
	for _, g := range l.symbols {
		g.value = g.offset

		switch {
		case g.contrib != nil:
			g.value += g.contrib.base()
		case g.image != "":
			if c := l.placedImage(g.image); c != nil {
				g.value += c.base
				g.placed = true
			}
		}
	}

	if l.transferSet {
		l.transfer = l.pendingTransfer.contrib.base() + l.pendingTransfer.offset
	}

	return nil
}

// value is a value on the linker's stack, and whether it's an address in
// the image (which moves with the image), an offset in a shareable image
// (img names it), or an absolute value.
type value struct {
	v   uint32
	rel bool
	img string
	// sym is the global symbol an offset in a shareable image is the
	// value of, untouched by arithmetic, or "" (see sharedRef.names).
	sym string
}

// store is how a store command writes the value it pops.
type store struct {
	size int
	// disp makes it a displacement from the end of the field.
	disp bool
	// signed limits the value to the signed range.
	signed bool
	// address marks a .ADDRESS longword (STO_PIDR), which may hold an
	// address in a shareable image, fixed up by the image activator.
	address bool
}

var stores = map[string]store{
	"STO_B": {size: 1}, "STO_W": {size: 2}, "STO_L": {size: 4},
	"STO_SB": {size: 1, signed: true}, "STO_SW": {size: 2, signed: true},
	"STO_BD": {size: 1, disp: true}, "STO_WD": {size: 2, disp: true}, "STO_LD": {size: 4, disp: true},
	// In an executable image, a position-independent data reference is
	// the address itself.
	"STO_PIDR": {size: 4, address: true},
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

// machine runs one module's TIR commands, or its traceback commands:
// with debug set, what it stores goes into the debug symbol table
// (linker.dst), loc being an offset in it, and not into the image.
type machine struct {
	l     *linker
	m     *module
	stack []value
	loc   uint32
	debug bool
}

// write stores data at the machine's location.
func (x *machine) write(data []byte) error {
	if x.debug {
		x.l.writeDST(x.loc, data)

		return nil
	}

	return x.l.write(x.loc, data)
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
// sections, and, with traceback, its traceback records, adding its DST
// records to the debug symbol table after the modules' before it (each
// TBT record stores where the last left off). Debugger records are
// skipped, but for LINK/DEBUG, which runs them with the traceback
// records, in the order they come, as one stream.
func (l *linker) pass2(m *module) error {
	x := &machine{l: l, m: m}
	tb := &machine{l: l, m: m, debug: true, loc: uint32(len(l.dst))}
	m.dstStart = tb.loc

	defer func() { m.dstEnd = uint32(len(l.dst)) }()

	for _, rec := range m.input.Module.Records {
		switch r := rec.(type) {
		case *obj.TIR:
			run := x

			switch {
			case r.Type == obj.RecTBT && l.opts.Traceback,
				r.Type == obj.RecDBG && l.opts.Debug:
				run = tb
			case r.Type != obj.RecTIR:
				continue
			}

			for _, c := range r.Commands {
				if err := run.command(c); err != nil {
					return fmt.Errorf("link: %s: %s: %w", m.input.File, c.Op, err)
				}
			}

		case *obj.EOM:
			if n := len(x.stack) + len(tb.stack); n != 0 {
				return fmt.Errorf("link: %s: %d values left on the linker's stack", m.input.File, n)
			}
		}
	}

	return nil
}

// writeDST stores data at offset off in the debug symbol table, which
// grows to hold it.
func (l *linker) writeDST(off uint32, data []byte) {
	if end := int(off) + len(data); end > len(l.dst) {
		l.dst = append(l.dst, make([]byte, end-len(l.dst))...)
	}

	copy(l.dst[off:], data)
}

// command runs one TIR command.
func (x *machine) command(c obj.Command) error {
	if c.Op == obj.OpStoreImmediate {
		if err := x.write(c.Data); err != nil {
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
		if g == nil {
			return fmt.Errorf("%s isn't in the module's symbol directory", c.Name)
		}

		if !g.defined && g.strongRef && !x.debug {
			x.undefinedReference(g)
		}

		img := g.image
		if g.placed {
			img = ""
		}

		x.push(value{v: g.value, rel: g.contrib != nil, img: img, sym: c.Name})

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

	case "OPR_REDEF":
		// .TRANSFER: the symbol's universal value in a shareable image
		// is the location here, its transfer vector entry (the Linker
		// manual, 7.4). It touches neither the stack nor the location.
		x.redefine(c.Name)

	case "STO_PICR":
		if x.debug {
			return fmt.Errorf("a general mode operand in a traceback record")
		}

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

// undefinedReference reports a reference to an undefined symbol, whose
// value is 0, at the operand being stored (real LINK's USEUNDEF).
func (x *machine) undefinedReference(g *global) {
	where := fmt.Sprintf("address %%X%08X", x.loc)

	for _, p := range x.l.order {
		if p.flags&obj.PsectREL != 0 && x.loc >= p.base && x.loc < p.base+p.length {
			where = fmt.Sprintf("psect %s offset %%X%08X", p.name, x.loc-p.base)

			break
		}
	}

	x.m.messages = append(x.m.messages, Message{'W', "USEUNDEF", fmt.Sprintf(
		"undefined symbol %s referenced\n\tin %s\n\tin module %s file %s", g.name, where, x.m.name, x.m.input.File)})
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

	switch {
	case a.img != "" && x.debug:
		return fmt.Errorf("an address in shareable image %s in a traceback record", a.img)
	case a.img != "" && st.address:
		x.l.referAddress(a.img, a.v, a.sym, x.loc)
		x.m.relocations++
	case a.img != "":
		return fmt.Errorf("a reference to shareable image %s must be general mode (G^) or .ADDRESS", a.img)
	case a.rel && x.debug && x.l.opts.Shareable && st.size == 4 && !x.l.basedAt(a.v):
		x.l.dstFixups = append(x.l.dstFixups, dstFixup{offset: x.loc, address: a.v})
		x.m.relocations++
	case a.rel && st.address && !x.debug && x.l.opts.Shareable:
		if x.l.referSelf(x.loc, a.v) {
			x.m.relocations++
		}
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

	if err := x.write(b[:st.size]); err != nil {
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

		x.l.referShared(a.img, a.v, a.sym, x.loc+1)

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
// it is demand-zero whatever its size. isdMax is ISD_MAX's default, the
// image sections past which the linker makes no more demand-zero ones:
// "approximately 96" (3.3); govax takes it as exactly that.
const (
	dzroMin = 5
	isdMax  = 96
)

// image lays out the image file: the header block, each image section's
// pages (writable pages with nothing stored in them become demand-zero),
// the fixup section, and the user stack. The image sections the link
// placed are in address order, and the shareable images the image
// activator places come last (testdata/link68's EXE1 and MAIN3).
func (l *linker) image() (*Image, error) {
	// A shareable image the image uses only through another, such as
	// SHR1's LIBRTL in SHR2, is in the list with no references.
	for _, c := range l.clusters {
		if !c.user() && l.sharedRef(c.image) == nil {
			l.addShared(c.shared)
		}
	}

	l.imageOrder()

	// The fixup section follows the last section of the user cluster at
	// the highest address (the Linker manual, 6.3.6.2). Laying it out
	// gives each general mode operand's cell, whose displacement goes into
	// the code before the sections' pages are written out.
	var end uint32

	for _, s := range l.sections {
		end = max(end, s.base+s.pages*blockSize)
	}

	if end == 0 {
		end = l.defaultCluster.base
		if !l.defaultCluster.based {
			end = l.origin()
		}
	}

	fixupVA := pageUp(end)
	layout := l.layoutFixups()

	if err := l.patchGRefs(layout, fixupVA); err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	fixup := l.fixupSection(layout, fixupVA)

	// A full map shows each symbol in a shareable image at its cell.
	l.cells = map[string]map[uint32]uint32{}

	for _, r := range l.shared {
		for i, o := range r.offsets {
			if l.cells[r.image.Name] == nil {
				l.cells[r.image.Name] = map[uint32]uint32{}
			}

			l.cells[r.image.Name][o] = fixupVA + layout.cells[r][i]
		}
	}

	placed, global, err := l.globalSections()
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	// The image's own sections, each split into its runs, with as many
	// demand-zero runs as ISD_MAX allows, from the first cluster to the
	// last; the fixup section; the user stack.
	minPages, limit := uint32(dzroMin), isdMax
	if l.opts.DZROMin != nil {
		minPages = *l.opts.DZROMin
	}

	if l.opts.ISDMax != nil {
		limit = int(*l.opts.ISDMax)
	}

	// A shareable image's sections aren't split: SHR1's eight untouched
	// pages were written into the file, even with DZRO_MIN=1 (SHR4).
	if l.opts.Shareable {
		minPages = ^uint32(0)
	}

	budget := limit - (len(l.sections) + len(placed) + len(global) + 2)

	var isds []isd

	for _, c := range l.clusters {
		flags := uint32(0)
		if c.last && !l.opts.Shareable {
			flags |= isdLASTCLU
		}

		if c.protect {
			flags |= isdPROTECT
		}

		for _, s := range c.sections {
			for _, run := range s.runs(minPages, &budget) {
				d := isd{pages: run.count, vpn: (s.base >> 9) + run.first, pfc: c.pfc, flags: flags | l.sectionType(s)}

				switch {
				case run.demandZero:
					d.flags |= isdDZRO | isdWRT
				case s.writable() || s.fixedUp:
					d.flags |= isdWRT

					if l.copyOnReference(s) || s.fixedUp {
						d.flags |= isdCRF
					}
				}

				if !run.demandZero {
					d.data = s.data[run.first*blockSize : (run.first+run.count)*blockSize]
				}

				isds = append(isds, d)
			}
		}
	}

	isds = append(isds, placed...)

	// The fixup section, and an executable's user stack. A shareable
	// image has a fixup section only when it has something to fix up,
	// and no stack (shareable.go).
	if l.hasFixupSection() {
		isds = append(isds, isd{pages: uint32(len(fixup)) / blockSize, vpn: fixupVA >> 9, flags: l.fixupSectionFlags(), data: fixup})
	} else {
		fixupVA, fixup = 0, nil
	}

	if !l.opts.Shareable {
		stackFlags := uint32(isdTypeUserStack | isdDZRO | isdWRT)
		if l.defaultCluster.last {
			stackFlags |= isdLASTCLU
		}

		isds = append(isds, isd{pages: uint32(l.opts.StackPages), vpn: 1<<22 - uint32(l.opts.StackPages), flags: stackFlags})
	}

	sort.SliceStable(isds, func(i, j int) bool { return isds[i].vpn < isds[j].vpn })

	// The sections' contents follow the header block, in their ISDs'
	// order.
	vbn := uint32(2)

	var pages [][]byte

	l.vmLow, l.vmHigh = ^uint32(0), 0

	for i := range isds {
		d := &isds[i]

		if d.data != nil {
			d.vbn = vbn
			pages = append(pages, d.data)
			vbn += d.pages
		}

		if d.flags&isdTypeMask != isdTypeUserStack {
			l.vmLow = min(l.vmLow, d.vpn<<9)
			l.vmHigh = max(l.vmHigh, (d.vpn+d.pages)<<9-1)
		}
	}

	isds = append(isds, global...)

	// The debug symbol table follows everything else, in whole blocks,
	// and, linked /DEBUG, the debug module table after it. dst gathers
	// these debug tables, which start at block vbn, each at a block of
	// its own.
	var dst []byte

	add := func(table []byte) uint32 {
		at := vbn + uint32(len(dst))/blockSize
		dst = append(dst, table...)
		dst = append(dst, make([]byte, int(pageUp(uint32(len(dst))))-len(dst))...)

		return at
	}

	if len(l.dst) > 0 {
		l.dst = l.withDSTFixups(l.dst)
		l.dstVBN = add(l.dst)

		if l.opts.Debug {
			l.dmt = l.debugModuleTable()
			l.dmtVBN = add(l.dmt)
		}
	}

	if l.opts.ImageName == "" {
		l.opts.ImageName = l.modules[0].name
	}

	if l.opts.Ident != "" {
		l.imageID = l.opts.Ident
	} else if id, ok := l.firstImageIdent(); ok && !l.transferSet {
		l.imageID = id
	}

	// Linked /DEBUG, or in a shareable image, the global symbol table
	// comes last. Real LINK ends the file at its last record, mid-block;
	// govax pads it to a whole block, as images are whole blocks
	// everywhere in govax (rms, and ods2's fixed-length records). Readers find the GST by the IHS's
	// VBN and record count, so the padding should be harmless; it's a
	// known difference to revisit if VMS objects (docs/PHASE-29.md,
	// Decision 7).
	if l.opts.Debug || l.opts.Shareable {
		gst, records, err := l.globalSymbolTable()
		if err != nil {
			return nil, fmt.Errorf("link: the global symbol table: %w", err)
		}

		l.gst, l.gstRecords = gst, records
		l.gstVBN = add(gst)
	}

	header, err := l.header(isds, fixupVA)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	// The map counts the image's own sections; each shareable image's
	// count separately (mapCounts).
	l.isdCount = len(isds) - len(placed) - len(global)
	l.fixupVA, l.fixupLength = fixupVA, uint32(len(fixup))
	l.ownISDs = nil

	for _, d := range isds {
		if d.flags&(isdGBL|isdBASED) == 0 {
			l.ownISDs = append(l.ownISDs, d)
		}
	}

	img := &Image{Transfer: l.transfer, HasTransfer: l.transferSet, l: l}
	img.Messages = append(img.Messages, l.messages...)

	for _, m := range l.modules {
		img.Messages = append(img.Messages, m.messages...)
	}

	img.Bytes = header

	for _, p := range pages {
		img.Bytes = append(img.Bytes, p...)
	}

	l.imageBlocks = uint32(len(img.Bytes)/blockSize) - 1

	// The map's count of a shareable image's binary blocks leaves out the
	// sections that hold the image's own .ADDRESS longwords: that fits
	// all eight of testdata/link68's shareable images (SHR1's 13 blocks
	// are 4, its 9-page data section left out; SHR6's 3 are 2, its
	// read-only section left out; SHR2, with no such longwords, keeps
	// its 3), though why is unconfirmed. An executable's are all its
	// blocks.
	uncounted := map[*section]bool{}

	for _, a := range l.selfAddresses {
		if s := l.sectionAt(a); s != nil && !uncounted[s] {
			uncounted[s] = true
			l.imageBlocks -= s.pages
		}
	}
	img.Bytes = append(img.Bytes, dst...)

	for _, p := range l.order {
		if p.flags&obj.PsectREL != 0 {
			img.Psects = append(img.Psects, PsectInfo{Name: p.name, Base: p.base, Length: p.length, Align: p.align, Flags: p.flags})
		}
	}

	sort.SliceStable(img.Psects, func(i, j int) bool { return img.Psects[i].Base < img.Psects[j].Base })

	return img, nil
}

// globalSections returns the global section ISDs that map the shareable
// images the image refers to: placed, a section each for those the link
// placed (BASED, at their addresses), and global, one each for those the
// image activator places, the size of the image's first section.
func (l *linker) globalSections() (placed, global []isd, err error) {
	for _, r := range l.shared {
		c := l.placedImage(r.image.Name)
		if c == nil {
			d, err := globalSectionISD(r.image)
			if err != nil {
				return nil, nil, err
			}

			global = append(global, d)

			continue
		}

		flags := uint32(isdBASED)
		if c.last {
			flags |= isdLASTCLU
		}

		for i, sec := range c.shared.Layout {
			d := globalISD(c.shared, i+1, sec.Pages, flags|sec.Flags)
			d.vpn = (c.base + sec.Offset) >> 9
			placed = append(placed, d)
		}
	}

	return placed, global, nil
}

// pageRun is a run of a section's pages that become one image section.
type pageRun struct {
	first, count uint32
	demandZero   bool
}

// runs splits a section into its image sections. A read-only section is
// one. A writable one with nothing stored in it is demand-zero; otherwise
// each run of at least minPages pages with nothing stored in them is split
// off as demand-zero, while budget, the image sections ISD_MAX leaves,
// allows: a run at the section's start or end takes one more, and one in
// its middle two (the run, and the pages after it).
func (s *section) runs(minPages uint32, budget *int) []pageRun {
	if !s.writable() {
		return []pageRun{{0, s.pages, false}}
	}

	var out []pageRun

	for p := uint32(0); p < s.pages; {
		q := p
		for q < s.pages && s.stored[q] == s.stored[p] {
			q++
		}

		whole := p == 0 && q == s.pages
		zero := !s.stored[p] && (whole || q-p >= minPages)

		if zero && !whole {
			cost := 2
			if p == 0 || q == s.pages {
				cost = 1
			}

			if *budget < cost {
				zero = false
			} else {
				*budget -= cost
			}
		}

		if n := len(out); n > 0 && !zero && !out[n-1].demandZero {
			out[n-1].count += q - p
		} else {
			out = append(out, pageRun{p, q - p, zero})
		}

		p = q
	}

	return out
}

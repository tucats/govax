package link

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tucats/govax/internal/obj"
)

// This file writes a link map, as LINK/MAP does, from the VAX linker's own
// map routines (vmssrc_archive/v73/linker/lis/lnkmaprtn.lis) and the cross
// reference facility that lays out its symbol table (crf/lis/cref.lis).
// Real LINK's maps of the Phase 27 fixtures (testdata/mar/vax/*.map) are
// what it's checked against.
//
// A map is pages of at most 58 lines (LIB$LP_LINES' 66, less 8), each
// starting with a form feed and a heading. The default map has:
//
//   - the object module synopsis: each module the command named, with its
//     size, file, creation date, and language processor;
//   - the program section synopsis: each nonempty relocatable psect, and
//     the named modules' contributions to it;
//   - the global symbols the named modules define, by name, and a key to
//     the symbol table's flags;
//   - on a new page, the image synopsis: the image's layout and counts.
//
// /BRIEF leaves out the psects and symbols. /FULL and /CROSS_REFERENCE
// aren't written yet, and neither is real LINK's last section, its run
// statistics (page faults and CPU time), which say nothing about the
// image.

// MapOptions are what a link map records that the link doesn't know.
type MapOptions struct {
	// ImageFile is the image file's full name; "" means no image was
	// written (/NOEXECUTABLE).
	ImageFile string
	// ImageText is the image file name as the command gave it
	// (/EXECUTABLE=), which the object module synopsis's headings show;
	// real LINK creates the image file, and knows its full name, only
	// after reading the modules.
	ImageText string
	// MapFile is the map file's name, which the image synopsis records.
	MapFile string
	// Brief is /BRIEF: only the object module and image synopses.
	Brief bool
}

// Map lengths and widths.
const (
	mapLineWidth    = 132    // LEN$C_MAPLINE
	mapLinesPerPage = 66 - 8 // LIB$LP_LINES, less 8
	mapShortName    = 15     // SYM$C_SHORTNAME
	mapLongName     = 31     // SYM$C_MAXLNG
	mapFileWidth    = 36     // the object module synopsis's file name
	mapLanguage     = 30     // LEN$C_LNGNAM
	mapNoImage      = "No image file created"
)

// Map returns the image's link map, a line per record.
func (img *Image) Map(opts MapOptions) []string {
	l := img.l
	w := &mapWriter{
		heading: opts.ImageText,
		date:    mapDate(l.opts.Time),
		linker:  "Linker " + l.opts.LinkerID,
		page:    1,
	}

	if opts.ImageFile == "" {
		w.heading = mapNoImage
	}

	w.newPage()
	w.box("Object Module Synopsis")

	// Messages that belong to no module come first, as real LINK lists
	// them there; each module's follow its line.
	for _, m := range l.messages {
		w.message(m)
	}

	w.subheading(
		"Module Name     Ident              Bytes      File                                Creation Date      Creator",
		"-----------     -----              -----      -----                               -------------      -------")

	for _, m := range l.modules {
		if !m.input.System {
			w.module(m)

			for _, msg := range m.messages {
				w.message(msg)
			}
		}
	}

	// Real LINK creates the image file after reading the modules, and
	// the pages after that name it in full (EXTERNU.MAP's page 2).
	if opts.ImageFile != "" {
		w.heading = opts.ImageFile
	}

	if !opts.Brief {
		w.psects(l)
		w.symbols(l)
	}

	// Writing the image, real LINK notes in the map that it has no
	// transfer address.
	if opts.ImageFile != "" && !l.transferSet {
		w.message(NoTransferMessage(opts.ImageFile))
	}

	w.synopsis(l, opts)

	return w.lines
}

// NoTransferMessage is real LINK's warning that the image in file has no
// user transfer address (USRTFR).
func NoTransferMessage(file string) Message {
	return Message{'W', "USRTFR", fmt.Sprintf("image %s has no user transfer address", file)}
}

// message writes a message, a line for each of its lines.
func (w *mapWriter) message(m Message) {
	for _, line := range strings.Split(m.String(), "\n") {
		w.out(line)
	}
}

// mapDate is a time as the map shows it, "dd-MMM-yyyy hh:mm", with a
// space before a one-digit day, as $ASCTIM writes it.
func mapDate(t time.Time) string {
	return fmt.Sprintf("%2d-%s-%04d %02d:%02d", t.Day(), strings.ToUpper(t.Month().String()[:3]), t.Year(), t.Hour(), t.Minute())
}

// mapWriter writes a map's lines, starting a new page when one is full.
type mapWriter struct {
	lines []string
	// count is the lines on the current page, and page the next page's
	// number.
	count int
	page  int
	// heading, date, and linker make each page's heading line.
	heading, date, linker string
	// subhead is the sub-heading to repeat on a new page, and psectLine
	// the psect whose contributions are being listed, whose line a new
	// page repeats too.
	subhead   []string
	psectLine string
}

// out writes a line, on a new page if this one is full.
func (w *mapWriter) out(line string) {
	if w.count >= mapLinesPerPage {
		w.newPage()
	}

	w.lines = append(w.lines, line)
	w.count++
}

// newPage starts a page: a form feed, the heading, a blank line, and the
// sub-heading and psect line being continued, if any.
func (w *mapWriter) newPage() {
	w.count = 0
	w.out("\f")
	w.out(fmt.Sprintf("%-64.64s%-25s%-33sPage%5d", w.heading, w.date, w.linker, w.page))

	w.page++

	w.out("")

	if w.subhead != nil {
		w.subheading(w.subhead...)
	}

	if w.psectLine != "" {
		w.out(w.psectLine)
	}
}

// box writes a title in a box, centered as real LINK centers it.
func (w *mapWriter) box(title string) {
	title = "! " + title + " !"
	indent := strings.Repeat(" ", (mapLineWidth-len(title))/2-8)
	edge := indent + "+" + strings.Repeat("-", len(title)-2) + "+"

	w.out(edge)
	w.out(indent + title)
	w.out(edge)
}

// subheading writes a blank line and a table's column headings, which a
// new page repeats until the next section.
func (w *mapWriter) subheading(lines ...string) {
	w.out("")

	for _, line := range lines {
		w.out(line)
	}

	w.subhead = lines
}

// module writes a module's line of the object module synopsis. A name or
// ident too long for its column goes on a line of its own.
func (w *mapWriter) module(m *module) {
	var size uint32
	for _, c := range m.contribs {
		size += c.size
	}

	rest := fmt.Sprintf("%10d %-37s%-19s%s", size, trimFileName(m.input.File), m.created, m.language[:min(len(m.language), mapLanguage)])

	switch {
	case len(m.name) <= mapShortName && len(m.ident) <= mapShortName:
		w.out(fmt.Sprintf("%-16s%-15s%s", m.name, m.ident, rest))
	case len(m.ident) <= mapShortName:
		w.out(m.name)
		w.out(fmt.Sprintf("%-16s%-15s%s", "", m.ident, rest))
	default:
		if len(m.name) > mapShortName {
			w.out(m.name)
		}

		w.out(fmt.Sprintf("%-31s%s", m.ident, rest))
	}
}

// trimFileName shortens a file name to the synopsis's column, as
// LIB$TRIM_FILESPEC does, by leaving out its directory.
func trimFileName(name string) string {
	if len(name) <= mapFileWidth {
		return name
	}

	if i := strings.LastIndexAny(name, "]>:"); i >= 0 && !strings.ContainsRune(name, '/') {
		name = name[i+1:]
	} else {
		name = filepath.Base(name)
	}

	return name[:min(len(name), mapFileWidth)]
}

// psects writes the program section synopsis: each nonempty relocatable
// psect, by address, and each nonempty contribution the command's modules
// make to it.
func (w *mapWriter) psects(l *linker) {
	w.subhead, w.psectLine = nil, ""
	w.out("")
	w.box("Program Section Synopsis")
	w.subheading(
		"Psect Name      Module Name       Base     End           Length            Align                 Attributes",
		"----------      -----------       ----     ---           ------            -----                 ----------")

	var list []*psect

	for _, p := range l.order {
		if p.flags&obj.PsectREL != 0 && p.length > 0 {
			list = append(list, p)
		}
	}

	sort.SliceStable(list, func(i, j int) bool { return list[i].base < list[j].base })

	owners := map[*contribution]*module{}

	for _, m := range l.modules {
		for _, c := range m.contribs {
			owners[c] = m
		}
	}

	for _, p := range list {
		numbers := mapExtent(p.base, p.length, p.align)
		attrs := psectAttributes(p.flags)

		var line string
		if len(p.name) <= mapShortName {
			line = fmt.Sprintf("%-16s%-15s%s %s", p.name, "", numbers, attrs)
		} else {
			line = fmt.Sprintf("%-31s%s %s", p.name, numbers, attrs)
		}

		w.out(line)
		w.psectLine = fmt.Sprintf("%-*s", mapLineWidth, line)

		for _, c := range p.contribs {
			m := owners[c]
			if c.size == 0 || m == nil || m.input.System {
				continue
			}

			numbers := mapExtent(c.base(), c.size, c.align)

			if len(m.name) > mapShortName {
				w.out(fmt.Sprintf("%-16s%-39s", "", m.name))
				w.out(fmt.Sprintf("%-31s%s", "", numbers))

				continue
			}

			w.out(fmt.Sprintf("%-16s%-15s%s", "", m.name, numbers))
		}

		w.psectLine = ""
	}
}

// mapExtent is a psect's or contribution's base, end, length (in hex and
// decimal), and alignment, as the program section synopsis shows them.
func mapExtent(base, length uint32, align byte) string {
	end := base + length
	if length > 0 {
		end--
	}

	return fmt.Sprintf("%9s%9s%9s (%11d.) %-5s%2d", hex8(base), hex8(end), hex8(length), length, alignName(align), align)
}

func hex8(v uint32) string { return fmt.Sprintf("%08X", v) }

// alignName names an alignment as the map does.
func alignName(align byte) string {
	switch align {
	case 0:
		return "BYTE"
	case 1:
		return "WORD"
	case 2:
		return "LONG"
	case 3:
		return "QUAD"
	case 4:
		return "OCTA"
	case 9:
		return "PAGE"
	}

	return "2 **"
}

// psectAttributes lists a psect's attributes, each in a fixed width.
func psectAttributes(flags uint16) string {
	pick := func(bit uint16, setValue, clearValue string) string {
		if flags&bit != 0 {
			return setValue
		}

		return clearValue
	}

	return pick(obj.PsectPIC, "  PIC,", "NOPIC,") +
		pick(obj.PsectLIB, "LIB,", "USR,") +
		pick(obj.PsectOVR, "OVR,", "CON,") +
		pick(obj.PsectREL, "REL,", "ABS,") +
		pick(obj.PsectGBL, "GBL,", "LCL,") +
		pick(obj.PsectSHR, "  SHR,", "NOSHR,") +
		pick(obj.PsectEXE, "  EXE,", "NOEXE,") +
		pick(obj.PsectRD, "  RD,", "NORD,") +
		pick(obj.PsectWRT, "  WRT,", "NOWRT,") +
		pick(obj.PsectVEC, "  VEC", "NOVEC")
}

// symbols writes the global symbols the command's modules define, by
// name, in columns filled down each page as the cross reference facility
// fills them, then the key to their flags.
func (w *mapWriter) symbols(l *linker) {
	type entry struct {
		name   string
		value  uint32
		suffix string
	}

	var list []entry

	for _, g := range l.symbols {
		switch {
		case !g.defined && g.strongRef:
			list = append(list, entry{g.name, 0, "-*"})
		case !g.defined:
			// Only referred to weakly: 0, with no flag.
			list = append(list, entry{g.name, 0, ""})
		case g.option:
			list = append(list, entry{g.name, g.value, ""})
		case g.fromSource || g.module == nil || g.module.input.System:
			continue
		case g.rel:
			list = append(list, entry{g.name, g.value, "-R"})
		default:
			list = append(list, entry{g.name, g.value, ""})
		}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })

	nameWidth := mapShortName
	heading := []string{
		"Symbol          Value              Symbol          Value              Symbol          Value              Symbol          Value",
		"------          -----              ------          -----              ------          -----              ------          -----",
	}

	for _, e := range list {
		if len(e.name) > mapShortName {
			nameWidth = mapLongName
			heading = []string{
				"Symbol                          Value       Symbol                          Value       Symbol                          Value",
				"------                          -----       ------                          -----       ------                          -----",
			}

			break
		}
	}

	w.subhead, w.psectLine = nil, ""
	w.out("")
	w.box("Symbols By Name")
	w.subheading(heading...)

	// An entry is the name, a space, the value, and a 4-character suffix.
	// As many fit on a line as leave room for a blank between each.
	width := nameWidth + 1 + 8 + 4
	perLine, left := mapLineWidth/width, mapLineWidth%width

	perLine--

	blanks := left / perLine
	if blanks == 0 {
		perLine--
		blanks = (left + width) / perLine
	}

	perLine++

	lines := mapLinesPerPage - w.count
	if lines <= 0 {
		lines = mapLinesPerPage - 6
	}

	for first := 0; first < len(list); {
		rows := min(lines, len(list)-first)

		for r := range rows {
			var b strings.Builder

			for col := range perLine {
				i := first + col*lines + r
				if i >= len(list) {
					break
				}

				// Every column but the last is followed by the blanks
				// between columns.
				e := list[i]
				fmt.Fprintf(&b, "%-*.*s %08X%-4s", nameWidth, nameWidth, e.name, e.value, e.suffix)

				if col < perLine-1 {
					b.WriteString(strings.Repeat(" ", blanks))
				}
			}

			w.out(b.String())
		}

		first += perLine * lines
		lines = mapLinesPerPage - 6
	}

	if w.count+10 > mapLinesPerPage {
		w.count = mapLinesPerPage
		w.out("")
	}

	for range 3 {
		w.out("")
	}

	w.out("\t  Key for special characters above:")
	w.out("\t\t+--------------------+")

	for _, key := range []string{
		" *  - Undefined     ", " A  - Alias Name    ", " I  - Internal Name ", " U  - Universal     ",
		" R  - Relocatable   ", " X  - External      ", " WK - Weak          ",
	} {
		w.out("\t\t!" + key + "!")
	}

	w.out("\t\t+--------------------+")
}

// synopsis writes the image synopsis, on a new page.
func (w *mapWriter) synopsis(l *linker, opts MapOptions) {
	w.subhead, w.psectLine = nil, ""
	w.newPage()
	w.box("Image Synopsis")
	w.out("")

	plural := func(n uint32) string {
		if n == 1 {
			return ""
		}

		return "s"
	}

	line := func(label, format string, args ...any) {
		w.out(fmt.Sprintf("%-50s"+format, append([]any{label}, args...)...))
	}

	low := uint32(imageBase)
	if len(l.sections) > 0 {
		low = l.sections[0].base
	}

	high := l.fixupVA + l.fixupLength - 1
	size := high - low + 1
	pages := (size + blockSize - 1) / blockSize

	w.out(fmt.Sprintf("%-49s%9s%9s%9s (%d. byte%s, %d. page%s)", "Virtual memory allocated:",
		hex8(low), hex8(high), hex8(size), size, plural(size), pages, plural(pages)))

	stack := uint32(l.opts.StackPages)
	line("Stack size:", "%8d. page%s", stack, plural(stack))
	line("Image header virtual block limits:", "%8d.%9d. (%5d. block)", 1, 1, 1)

	var first, last uint32
	if l.imageBlocks > 0 {
		first, last = 2, 1+l.imageBlocks
	}

	line("Image binary virtual block limits:", "%8d.%9d. (%5d. block%s)", first, last, l.imageBlocks, plural(l.imageBlocks))
	line("Image name and identification:", "%s %s", l.opts.ImageName, l.imageID)

	counts := l.mapCounts()
	line("Number of files:", "%8d.", counts.files)
	line("Number of modules:", "%8d.", counts.modules)
	line("Number of program sections:", "%8d.", counts.psects)
	line("Number of global symbols:", "%8d.", counts.symbols)

	if len(l.undefined) > 0 {
		line("Including undefined count of:", "%8d.", len(l.undefined))
	}

	// Each SYMBOL= option is a cross reference in a map that lists
	// symbols (lnkoption.lis), and not a global symbol.
	if !opts.Brief && counts.options > 0 {
		line("Number of cross references:", "%8d.", counts.options)
	}
	
	line("Number of image sections:", "%8d.", counts.sections)

	if l.transferSet {
		line("User transfer address:", "%08X", l.transfer)
	}

	if l.opts.Traceback {
		line("Debugger transfer address:", "%08X", l.symbols[imgstaName].value)
	}

	if l.addressFixups > 0 {
		line("Number of address fixups:", "%8d.", l.addressFixups)
	}

	// Each target in a shareable image has a cell, whether general mode
	// operands or .ADDRESS longwords reach it: ADDR's map counts one code
	// reference, though only a .ADDRESS reaches LIB$PUT_OUTPUT.
	cells := 0
	for _, r := range l.shared {
		cells += len(r.offsets)
	}

	if cells > 0 {
		line("Number of code references to shareable images:", "%8d.", cells)
	}

	line("Image type:", "EXECUTABLE.")

	format, estimate := "DEFAULT", 7+counts.modules/4
	if opts.Brief {
		format = "BRIEF"
	} else {
		estimate += counts.psects + counts.symbols/16
	}

	line("Map format:", "%s in file %s", format, opts.MapFile)
	line("Estimated map length:", "%d. blocks", estimate)
}

// mapCounts are the link's files, modules, psects, global symbols, and
// image sections, as the image synopsis counts them.
type mapCounts struct {
	files, modules, psects, symbols, sections int
	// options are the symbols SYMBOL= options define.
	options int
}

// mapCounts counts what the link read and made: the files (each object
// file, and each file a symbol source read); the modules (each object
// module, and each shareable image the image uses); the psects and global
// symbols, including every one in the global symbol table of each
// shareable image used, which real LINK reads in whole; and the image
// sections, the image's own (its global section ISDs aside) and each
// shareable image's.
func (l *linker) mapCounts() mapCounts {
	var c mapCounts

	files := map[string]bool{}

	for _, m := range l.modules {
		if !m.library {
			files[m.input.File] = true
		}
	}

	c.files = len(files)

	for _, src := range l.opts.Sources {
		if fc, ok := src.(FileCounter); ok {
			c.files += fc.Files()
		}
	}

	used := map[string]int{} // each shareable image used, and its symbols in the link

	for _, g := range l.symbols {
		if g.image != "" {
			used[g.image]++
		}
	}

	for _, r := range l.shared {
		if _, ok := used[r.image.Name]; !ok {
			used[r.image.Name] = 0
		}
	}

	c.modules = len(l.modules) + len(used)
	c.psects, c.symbols, c.sections = len(l.order), len(l.symbols), l.isdCount

	for _, g := range l.symbols {
		if g.option {
			c.options++
			c.symbols--
		}
	}

	for name, n := range used {
		img := l.sharedImage(name)
		c.psects += img.Psects
		c.symbols += max(img.Symbols-n, 0)
		c.sections += max(img.Sections, 1)
	}

	return c
}

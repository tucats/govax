package link

import (
	"fmt"
	"sort"
	"strings"
)

// This file is what a full map (/FULL) adds to the default one
// (mapfile.go), from the maps of testdata/link68, all of them /FULL.

// mapModules are the modules the object module synopsis lists. A default
// map lists the command's modules. A full map lists every module and each
// shareable image used, cluster by cluster: a shareable image an options
// file names, and those it uses, in its place in the cluster list, and one
// a library supplied after the modules (SHR1's LIBRTL, after STARLET's
// SYS$P1_VECTOR; MAIN1L's SHR1, found in MYSHR.OLB). A shareable image is
// a module of no bytes, with its global symbol table's ident, creation
// time, and creator.
func (l *linker) mapModules(full bool) []*module {
	var list []*module

	if !full {
		for _, m := range l.modules {
			if !m.input.System {
				list = append(list, m)
			}
		}

		return list
	}

	for _, c := range l.clusters {
		if !c.user() {
			img := l.sharedImage(c.image)
			list = append(list, &module{
				input: &Input{File: img.File}, name: c.image, ident: img.Ident,
				created: img.Created, language: img.Creator,
			})

			continue
		}

		for _, m := range l.modules {
			if m.cluster == c {
				list = append(list, m)
			}
		}
	}

	return list
}

// relocations writes the module relocatable reference synopsis: each
// module that has longwords the image activator or the debugger relocates
// (module.relocations), three to a line, in the object module synopsis's
// order. A map with none has no such section (SHR3, based; MAIN2). How
// real LINK fills more than one line is unconfirmed: govax fills each
// line across.
func (w *mapWriter) relocations(l *linker) {
	var entries []string

	for _, m := range l.mapModules(true) {
		if m.relocations > 0 {
			entries = append(entries, fmt.Sprintf("%-35s%6d  ", m.name, m.relocations))
		}
	}

	if len(entries) == 0 {
		return
	}

	w.subhead, w.psectLine = nil, ""
	w.out("")
	w.box("Module Relocatable Reference Synopsis")
	w.subheading(
		"Module Name                        Number  Module Name                        Number  Module Name                        Number",
		"-----------                        ------  -----------                        ------  -----------                        ------")

	for len(entries) > 0 {
		n := min(3, len(entries))
		w.out(strings.Join(entries[:n], ""))
		entries = entries[n:]
	}
}

// imageSections writes the image section synopsis, on a page of its own:
// for each cluster, in the cluster list's order, after a blank line, its
// image sections, its name on the first. A user cluster's are the image's
// own sections in it (the fixup section with the cluster at the highest
// address, the user stack with the default cluster). A shareable image's
// are its own sections, each a global section of the image's name and its
// number, with the ident the image was linked with.
func (w *mapWriter) imageSections(l *linker) {
	w.subhead, w.psectLine = nil, ""
	w.newPage()
	w.box("Image Section Synopsis")
	w.subheading(
		"   Cluster      Type Pages   Base Addr  Disk VBN PFC Protection and Paging      Global Sec. Name   Match     Majorid   Minorid",
		"   -------      ---- -----   ---------  -------- --- ---------------------      ----------------   -----     -------   -------")

	// The fixup section follows the cluster at the highest address.
	var last *cluster

	var end uint32

	for _, s := range l.sections {
		if e := s.base + s.pages*blockSize; e > end {
			end, last = e, s.cluster
		}
	}

	if last == nil {
		last = l.defaultCluster
	}

	clusterOf := func(d isd) (*cluster, *section) {
		switch {
		case d.flags&isdTypeMask == isdTypeUserStack:
			return l.defaultCluster, nil
		case d.flags&isdFIXUPVEC != 0:
			return last, nil
		}

		if s := l.sectionAt(d.vpn << 9); s != nil {
			return s.cluster, s
		}

		return l.defaultCluster, nil
	}

	for _, c := range l.clusters {
		var lines []string

		if c.user() {
			for _, d := range l.ownISDs {
				owner, s := clusterOf(d)
				if owner != c {
					continue
				}

				lines = append(lines, imageSectionLine(d.flags, d.pages, d.vpn<<9, l.opts.Shareable && !c.based,
					d.vbn, d.pfc, mapProtection(d.flags, s != nil && s.fixedUp)))
			}
		} else {
			lines = w.sharedSections(l, c)
		}

		if len(lines) == 0 {
			continue
		}

		w.out("")

		name := c.name
		if len(name) > mapShortName {
			w.out(name)
			name = ""
		}

		for _, line := range lines {
			w.out(fmt.Sprintf("%-16s%s", name, line))
			name = ""
		}
	}

	w.key([]string{" R  - Relocatable   ", " P  - Protected     "})
}

// sharedSections are a shareable image's lines of the image section
// synopsis: each of its own sections (from its header), at its offset in
// the image, relocatable, or at its address if the link placed the image
// or it is based (EXE1's LIBRTL, MAIN3's SHR3). An image whose sections
// aren't known is one shareable section of its pages.
func (w *mapWriter) sharedSections(l *linker, c *cluster) []string {
	img := l.sharedImage(c.image)

	layout := img.Layout
	if layout == nil {
		layout = []SharedSection{{Pages: img.Pages, Flags: isdTypeSharedPIC}}
	}

	var lines []string

	for i, sec := range layout {
		base := sec.Offset
		if c.placed {
			base += c.base
		}

		flags := sec.Flags
		if sec.Fixups {
			flags |= isdFIXUPVEC
		}

		prot := fmt.Sprintf("%-27s%-17s%-10s%9d%10d", mapProtection(flags, false),
			fmt.Sprintf("%s_%03d", img.Name, i+1), matchName(img.Match), img.MajorID, img.MinorID)

		lines = append(lines, imageSectionLine(flags, sec.Pages, base, !c.placed && !img.Based, 0, 0, prot))
	}

	return lines
}

// imageSectionLine is an image section's line of the image section
// synopsis, after the cluster column: its type, pages, address (P- if
// protected, -R if relocatable), first block in the file (0 for none),
// page fault cluster, and protection.
func imageSectionLine(flags, pages, base uint32, rel bool, vbn, pfc uint32, prot string) string {
	protect, relocatable := "  ", "  "
	if flags&isdPROTECT != 0 {
		protect = "P-"
	}

	if rel {
		relocatable = "-R"
	}

	return fmt.Sprintf("%4d%6d  %s%08X%s%9d%4d %s", flags>>24, pages, protect, base, relocatable, vbn, pfc, prot)
}

// mapProtection is an image section's protection and paging, as the image
// section synopsis words them. A shareable image's read-only section that
// holds .ADDRESS fixups (fixedUp) is NON-SHAREABLE ADDRESS DATA in its
// own map (SHR6), though an image linked against it shows it COPY ON REF
// (MAIN6).
func mapProtection(flags uint32, fixedUp bool) string {
	if flags&isdWRT == 0 {
		return "READ ONLY     "
	}

	paging := ""

	switch {
	case flags&isdFIXUPVEC != 0:
		paging = "FIXUP VECTORS"
	case flags&isdDZRO != 0:
		paging = "DEMAND ZERO  "
	case fixedUp:
		paging = "NON-SHAREABLE ADDRESS DATA"
	case flags&isdCRF != 0:
		paging = "COPY ON REF"
	}

	return "READ WRITE   " + paging
}

// matchName is a match control as the map words it.
func matchName(m Match) string {
	switch m {
	case MatchAlways:
		return "ALWAYS"
	case MatchEqual:
		return "EQUAL"
	case MatchLEQ:
		return "LESS/EQUAL"
	}

	return "NEVER"
}

// mapSymbol is a global symbol as the map's symbol tables show it: its
// value, and its flags there (U universal, R relocatable, X external, *
// undefined). byValue are its flags in the symbols by value, where a
// .TRANSFER'd symbol's value is its routine's, not the universal one, so
// it isn't U (SHR1's "R-SHR1_ADD").
type mapSymbol struct {
	name           string
	value          uint32
	flags, byValue string
}

// mapSymbols are the global symbols the map's symbol tables list: those
// the command's modules define, the undefined ones, and SYMBOL='s. A full
// map adds those that libraries and shareable images define and something
// refers to, a symbol in a shareable image at its cell in the fixup
// section (X), unless the link placed the image, when it is its address.
func (l *linker) mapSymbols(full bool) []mapSymbol {
	var list []mapSymbol

	for _, g := range l.symbols {
		e := mapSymbol{name: g.name, value: g.value}

		switch {
		case !g.defined && g.strongRef:
			e.value, e.flags = 0, "*"
		case !g.defined:
			// Only referred to weakly: 0, with no flag.
			e.value = 0
		case g.option:
		case g.fromSource || g.module == nil || g.module.input.System:
			if !full || !g.strongRef && len(g.refs) == 0 {
				continue
			}

			if g.image != "" && !g.placed {
				e.flags = "RX"

				if cell, ok := l.cells[g.image][g.value]; ok {
					e.value = cell
				}
			} else if g.rel {
				e.flags = "R"
			}
		case g.rel:
			e.flags = "R"
		}

		e.byValue = e.flags

		if g.defined && g.universal {
			e.flags += "U"

			if !g.vectored {
				e.byValue += "U"
			}
		}

		list = append(list, e)
	}

	return list
}

// symbolsByValue writes the symbols by value, on a page of their own, and
// the key to the symbol tables' flags: each value, and the symbols with
// it, by name, each its flags and a dash, right-aligned in four columns,
// then its name, as wide as the longest name. How many go on a line, and
// how a value with more continues, is unconfirmed (no fixture has more
// than two at a value).
func (w *mapWriter) symbolsByValue(l *linker) {
	list := l.mapSymbols(true)

	sort.Slice(list, func(i, j int) bool {
		if list[i].value != list[j].value {
			return list[i].value < list[j].value
		}

		return list[i].name < list[j].name
	})

	width := 0
	for _, e := range list {
		width = max(width, len(e.name))
	}

	w.section(true)
	w.box("Symbols By Value")
	w.subheading(
		"Value                                   Symbols...",
		"-----                                   ----------")

	perLine := max((mapLineWidth-12+1)/(width+5), 1)

	for len(list) > 0 {
		n := 1
		for n < len(list) && n < perLine && list[n].value == list[0].value {
			n++
		}

		entries := make([]string, n)

		for i, e := range list[:n] {
			flags := "    "
			if e.byValue != "" {
				flags = fmt.Sprintf("%3s-", e.byValue)
			}

			entries[i] = flags + fmt.Sprintf("%-*s", width, e.name)
		}

		w.out(fmt.Sprintf("%08X    %s", list[0].value, strings.Join(entries, " ")))
		list = list[n:]
	}

	w.symbolKey()
}

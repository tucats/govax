package lbr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// This file is LIBRARY/LIST's listing (librar/lis/listlib.lis): the
// library header's summary, then a line for each module, in index order,
// with /FULL's details and /NAMES's global symbols. The lines are
// LIBRARIAN's own FAO formats, reproduced here field for field, with
// trailing blanks dropped as output_listline drops them.

// ListOptions choose what List writes, as LIBRARY/LIST's qualifiers do.
type ListOptions struct {
	// Name is the library's file name, and Now the time of the listing,
	// a VMS time.
	Name string
	Now  uint64
	// Full is /FULL: each module's ident, insertion time, and symbol
	// count. Names is /NAMES: an object library module's global symbols.
	Full  bool
	Names bool
	// Width is the line width /NAMES fills; 0 is 80, and the most is 132
	// (LIB$C_LISRECLNG).
	Width int
}

// typeNames are LIB$AL_TYPNAMES.
var typeNames = []string{"UNKNOWN", "OBJECT", "MACRO", "HELP", "TEXT", "SHAREABLE IMAGE SYMBOL TABLE", "UNKNOWN", "ALPHA OBJECT", "ALPHA SHAREABLE IMAGE SYMBOL TABLE"}

const listRecordLength = 132 // LIB$C_LISRECLNG

// List returns the listing's lines.
func (l *Library) List(opts ListOptions) ([]string, error) {
	width := opts.Width
	if width <= 0 {
		width = 80
	}

	width = min(width, listRecordLength)

	typeName := "USER DEFINED"
	if int(l.Type) < len(typeNames) {
		typeName = typeNames[l.Type]
	}

	out := []string{
		fmt.Sprintf("Directory of %s library %s on %s", typeName, opts.Name, listDate(opts.Now)),
		"Creation date:  " + listDate(l.Created) + spaces(7) + field("Creator:  ", 10) + l.Librarian,
		"Revision date:  " + listDate(l.Updated) + spaces(7) + field("Library format:  ", 18) + fmt.Sprintf("%d.%d", l.MajorID, l.MinorID),
		field("Number of modules:  ", 20) + number(l.Modules, 5) + spaces(17) + field("Max. key length:  ", 18) + strconv.Itoa(l.KeySize()),
		field("Other entries:  ", 20) + number(l.IndexEntries-l.Modules, 5) + spaces(17) + field("Preallocated index blocks:  ", 28) + number(l.Preallocated, 5),
		field("Recoverable deleted blocks:  ", 29) + number(l.DeletedBlocks, 5) + spaces(8) + field("Total index blocks used:  ", 28) + number(l.IndexBlocks, 5),
		field("Max. Number history records:  ", 31) + number(uint32(l.History), 5) + spaces(6) + field("Library history records:  ", 26) + number(uint32(l.HistoryRecords), 7),
	}

	if l.DataReduced() {
		out = append(out, "Library is in DCX data reduced format")
	}

	out = append(out, "")

	if len(l.Indexes) == 0 {
		return trimLines(out), nil
	}

	object := l.Type == TypeObject || l.Type == TypeShareable

	// Each module's global symbols, in index 2's order.
	symbols := map[RFA][]string{}

	if object && opts.Names && len(l.Indexes) > 1 {
		for _, k := range l.Indexes[1].Keys {
			symbols[k.RFA] = append(symbols[k.RFA], k.Name)
		}
	}

	for _, k := range l.Indexes[0].Keys {
		var (
			h   ModuleHeader
			err error
		)

		if opts.Full || (object && opts.Names) {
			if h, err = l.Header(k.RFA); err != nil {
				return nil, err
			}
		}

		if !object {
			switch {
			case !opts.Full:
				out = append(out, k.Name)
			case len(k.Name) > 15:
				out = append(out, k.Name+" inserted "+listDate(h.Inserted))
			default:
				out = append(out, field(k.Name, 16)+" inserted "+listDate(h.Inserted))
			}

			continue
		}

		count := int(h.RefCount) - 1

		prefix := ""
		if opts.Names {
			prefix = "Module "
		}

		if opts.Full {
			ident := h.ObjectIdent()
			if l.Type == TypeShareable {
				ident = gsmatch(h)
			}

			plural := "s"
			if count == 1 {
				plural = ""
			}

			name, id := field(k.Name, 16), field(ident, 16)
			if len(k.Name) > 15 || len(ident) > 15 {
				name, id = k.Name, ident
			}

			out = append(out, fmt.Sprintf("%s%s Ident %s Inserted %s %d symbol%s", prefix, name, id, listDate(h.Inserted), count, plural))

			if h.SelectiveSearch() {
				out = append(out, "     Selectively searched")
			}
		} else {
			out = append(out, prefix+k.Name)
		}

		if !opts.Names || count <= 0 {
			continue
		}

		// The symbols, in columns a key and two blanks wide.
		column := l.KeySize() + 2
		line := ""

		for _, s := range symbols[k.RFA][:min(count, len(symbols[k.RFA]))] {
			if len(line)+column > width {
				out = append(out, line)
				line = ""
			}

			line += field(s, column)
		}

		if line != "" {
			out = append(out, line)
		}

		out = append(out, "")
	}

	return trimLines(out), nil
}

// listDate is FAO's !20<!%D!>: $ASCTIM's absolute time, day padded with a
// blank, cut to the second.
func listDate(v uint64) string {
	t := vmsdef.GoTime(v)
	months := "JANFEBMARAPRMAYJUNJULAUGSEPOCTNOVDEC"
	m := int(t.Month()-1) * 3

	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d", t.Day(), months[m:m+3], t.Year(), t.Hour(), t.Minute(), t.Second())
}

// gsmatch is a shareable image library module's ident: its GSMATCH
// criteria and major and minor IDs, as !2XL,!6XL shows them.
func gsmatch(h ModuleHeader) string {
	if len(h.UserData) < 6 {
		return ""
	}

	v := le32(h.UserData, 2)

	return fmt.Sprintf("%02X,%06X", v>>24, v&0xFFFFFF)
}

// field is FAO's !n<...!>: s in a field n wide, padded with blanks or cut.
func field(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}

	return s + spaces(n-len(s))
}

// number is FAO's !nUL: v right-justified in n columns, or asterisks if it
// doesn't fit.
func number(v uint32, n int) string {
	s := strconv.FormatUint(uint64(v), 10)
	if len(s) > n {
		return strings.Repeat("*", n)
	}

	return spaces(n-len(s)) + s
}

func spaces(n int) string { return strings.Repeat(" ", n) }

func trimLines(lines []string) []string {
	for i, s := range lines {
		lines[i] = strings.TrimRight(s, " ")
	}

	return lines
}

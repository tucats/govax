package link

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// This file reads a LINK options file (the Linker manual, 1.8 and chapter
// 10: an input file with /OPTIONS). Each line is either input files,
// separated by commas, each with its own qualifiers:
//
//	file[/LIBRARY][/INCLUDE=(module[,...])][/SELECTIVE_SEARCH][/SHAREABLE]
//
// or an option, keyword=value, one of VAX LINK's:
//
//	BASE=address
//	CLUSTER=cluster-name,[base-address],[pfc][,file-spec[,...]]
//	COLLECT=cluster-name,psect-name[,...]
//	DZRO_MIN=pages
//	GSMATCH=keyword,major-id,minor-id     (ALWAYS, EQUAL, LEQUAL)
//	IDENTIFICATION=ident
//	IOSEGMENT=pages[,[NO]P0BUFS]
//	ISD_MAX=image-sections
//	NAME=image-name
//	PROTECT=YES|NO
//	PSECT_ATTR=psect-name,attribute[,...]
//	STACK=pages
//	SYMBOL=name,value
//	UNIVERSAL=symbol-name[,...]
//	UNSUPPORTED=value
//
// An exclamation point starts a comment, and a hyphen at the end of a line
// continues it on the next. Keywords and qualifiers may be abbreviated to
// any unique prefix, and numbers may be decimal or have a radix prefix
// (%X, %D, %O).
//
// LINK reads an options file a line at a time (OptionsParser), and stops
// at the first line in error: VMS's LINK showed a message after the line
// it read last (testdata/link68's log, where verification shows each line
// as LINK reads it). An error is an OptionError, which words its message
// as VMS's LINK does, with the part of the line in error between
// apostrophes:
//
//	%LINK-F-OPTSYNERR, syntax error in options file SYS$INPUT:.;
//	-LINK-E-OPTLIN, options line in error
//		GSMATCH='SOMETIMES',1,2
//
// The probe settled OPTSYNERR for an unknown keyword (UNIVERSALS=) and an
// unknown GSMATCH keyword (NEVER and SOMETIMES), OPTVALERR for a major ID
// past 255, and that a second GSMATCH= replaces the first. The other
// checks and their messages are govax's choices (unconfirmed): a value
// that isn't a number or a keyword, a name that's missing or too long,
// or an input file with a qualifier LINK doesn't know, is OPTSYNERR with
// that value or file quoted; a number out of range is OPTVALERR, with the
// ranges below. PROTECT= holds to the end of its options file.

// InputFile is one input file of a link, as the command line or an options
// file names it, with its qualifiers.
type InputFile struct {
	Name string
	// Library is /LIBRARY: a library searched for symbols the modules
	// refer to but don't define, before the system libraries.
	Library bool
	// Include is /INCLUDE=(...): modules of a library added to the link
	// whether or not anything refers to them.
	Include []string
	// Selective is /SELECTIVE_SEARCH: an object file whose definitions are
	// taken only for symbols already referred to.
	Selective bool
	// Shareable is /SHAREABLE (options files only): a shareable image
	// whose symbols the link uses.
	Shareable bool
	// Options is /OPTIONS (the command line only): an options file.
	Options bool
	// Cluster is the cluster a CLUSTER= option put the file in; "" for
	// the default cluster.
	Cluster string
}

// OptionsFile is what an options file says.
type OptionsFile struct {
	// Files are its input files, in order, those CLUSTER= names among
	// them (with their Cluster).
	Files []InputFile
	// Stack is STACK=, the user stack's pages; 0 if not given.
	Stack int
	// Ident is IDENTIFICATION=, and Name is NAME=; "" if not given.
	Ident, Name string
	// Symbols are SYMBOL= definitions, in order.
	Symbols []Symbol

	// Universals are the symbols UNIVERSAL= names, in order.
	Universals []string
	// GSMatch is GSMATCH=: the last one given, or nil.
	GSMatch *GSMatch
	// Clusters are CLUSTER= options, and Collects COLLECT= options, in
	// order.
	Clusters []Cluster
	Collects []Collect
	// PsectAttrs are PSECT_ATTR= options, in order.
	PsectAttrs []PsectAttr
	// Base is BASE=, DZROMin DZRO_MIN=, ISDMax ISD_MAX=, IOSegment
	// IOSEGMENT=, and Unsupported UNSUPPORTED=; nil if not given.
	Base        *uint32
	DZROMin     *uint32
	ISDMax      *uint32
	IOSegment   *IOSegment
	Unsupported *uint32
}

// Symbol is a global symbol an options file defines (SYMBOL=), which takes
// precedence over any definition of it in the modules.
type Symbol struct {
	Name  string
	Value uint32
}

// GSMatch is GSMATCH=: a shareable image's match control and its global
// section idents, which the image activator compares with those an image
// linked against it recorded.
type GSMatch struct {
	Match Match
	Major uint8
	Minor uint32 // 24 bits
}

// Cluster is a CLUSTER= option: a cluster, its base address if it has
// one, its page fault cluster (0 for the system's default), and whether a
// PROTECT=YES before it protects its image sections. Its files are
// among OptionsFile.Files.
//
// A shareable image an options file names is a cluster of its own, in
// its place in the list (the Linker manual, 6.3.1.2): an entry with
// Shareable set, and Image the image's name once LINK has read it
// (OptionsFile.NameImage).
type Cluster struct {
	Name    string
	Base    *uint32
	PFC     uint32
	Protect bool

	Shareable bool
	Image     string
}

// NameImage names the first shareable image cluster not yet named: the
// image LINK has just read, the next one the options files name.
func (o *OptionsFile) NameImage(name string) {
	for i := range o.Clusters {
		if c := &o.Clusters[i]; c.Shareable && c.Image == "" {
			c.Image = name

			return
		}
	}
}

// addFiles adds input files to the options, and a cluster for each
// shareable image among them.
func (o *OptionsFile) addFiles(files ...InputFile) {
	for _, f := range files {
		o.Files = append(o.Files, f)

		if f.Shareable {
			o.Clusters = append(o.Clusters, Cluster{Shareable: true})
		}
	}
}

// Collect is a COLLECT= option: psects moved into a cluster, which a
// PROTECT=YES before it protects.
type Collect struct {
	Cluster string
	Psects  []string
	Protect bool
}

// PsectAttr is a PSECT_ATTR= option: a psect's attributes to set, as the
// keywords that set them (PIC, NOSHR, ...), and its alignment, a power of
// 2, if one was given.
type PsectAttr struct {
	Name       string
	Attributes []string
	Align      *uint32
}

// IOSegment is IOSEGMENT=: the image I/O segment's pages, and whether RMS
// may take more in P0 (P0BUFS).
type IOSegment struct {
	Pages  uint32
	P0Bufs bool
}

// Unimplemented returns the options o gives that govax's LINK doesn't act
// on yet, by keyword, so that a link refuses them rather than ignoring
// them.
func (o *OptionsFile) Unimplemented() []string {
	var names []string

	add := func(given bool, name string) {
		if given {
			names = append(names, name)
		}
	}

	add(o.Unsupported != nil, "UNSUPPORTED=")

	return names
}

// Merge takes a later options file's options into o: a value replaces an
// earlier one, and lists add up. Files are left alone: the link reads
// each options file's files as it comes to them.
func (o *OptionsFile) Merge(later *OptionsFile) {
	if later.Stack != 0 {
		o.Stack = later.Stack
	}

	if later.Ident != "" {
		o.Ident = later.Ident
	}

	if later.Name != "" {
		o.Name = later.Name
	}

	o.Symbols = append(o.Symbols, later.Symbols...)
	o.Universals = append(o.Universals, later.Universals...)
	o.Clusters = append(o.Clusters, later.Clusters...)
	o.Collects = append(o.Collects, later.Collects...)
	o.PsectAttrs = append(o.PsectAttrs, later.PsectAttrs...)

	replace(&o.GSMatch, later.GSMatch)
	replace(&o.Base, later.Base)
	replace(&o.DZROMin, later.DZROMin)
	replace(&o.ISDMax, later.ISDMax)
	replace(&o.IOSegment, later.IOSegment)
	replace(&o.Unsupported, later.Unsupported)
}

// replace sets *p to v when v was given.
func replace[T any](p **T, v *T) {
	if v != nil {
		*p = v
	}
}

// OptionError is a line of an options file in error, as VMS's LINK
// reports it: OPTSYNERR for its syntax, or OPTVALERR for a number out of
// range.
type OptionError struct {
	// Ident is the message's: OPTSYNERR or OPTVALERR.
	Ident string
	// Line is the line in error, with the part in error between
	// apostrophes.
	Line string
	// Option, Value, Low, and High are OPTVALERR's: the option's keyword,
	// the value given, and the range it must be in.
	Option           string
	Value, Low, High uint32
}

// LINK's condition values for the two errors (VMS's statuses less
// STS$M_INHIBIT_MSG, which LINK sets once it has shown the message:
// testdata/link68's %X106483B4 and %X106483BC).
const (
	statusOptSynErr = 0x006483B4
	statusOptValErr = 0x006483BC
)

// Status is the error's condition value: its $STATUS, but for the
// inhibit bit.
func (e *OptionError) Status() uint32 {
	if e.Ident == "OPTVALERR" {
		return statusOptValErr
	}

	return statusOptSynErr
}

// Message is the error's message, without its leading "%", for an options
// file named file (as RMS names it: "SYS$INPUT:.;" for SYS$INPUT).
func (e *OptionError) Message(file string) string {
	if e.Ident == "OPTVALERR" {
		return fmt.Sprintf("LINK-F-OPTVALERR, %s option value %d. is out of range (%d. to %d.)\n\tin options file %s\n-LINK-E-OPTLIN, options line in error\n\t%s",
			e.Option, e.Value, e.Low, e.High, file, e.Line)
	}

	return fmt.Sprintf("LINK-F-OPTSYNERR, syntax error in options file %s\n-LINK-E-OPTLIN, options line in error\n\t%s", file, e.Line)
}

func (e *OptionError) Error() string {
	if e.Ident == "OPTVALERR" {
		return fmt.Sprintf("%s option value %d. is out of range (%d. to %d.): %s", e.Option, e.Value, e.Low, e.High, e.Line)
	}

	return "syntax error in options line " + e.Line
}

// OptionsParser reads an options file a line at a time.
type OptionsParser struct {
	o OptionsFile
	// statement holds a statement continued from earlier lines.
	statement strings.Builder
	// protect is PROTECT='s setting, for the clusters after it.
	protect bool
}

// Line reads one line of the file. An error is an *OptionError for the
// statement the line ends, or another error for an input file govax
// can't use.
func (p *OptionsParser) Line(line string) error {
	line = strings.TrimSpace(stripComment(line))

	if cont, ok := strings.CutSuffix(line, "-"); ok {
		p.statement.WriteString(cont)

		return nil
	}

	p.statement.WriteString(line)

	return p.finish()
}

// End ends the file, reading any statement its last line continued, and
// returns what it said.
func (p *OptionsParser) End() (*OptionsFile, error) {
	if err := p.finish(); err != nil {
		return nil, err
	}

	o := p.o

	return &o, nil
}

// finish reads the statement gathered so far, if any.
func (p *OptionsParser) finish() error {
	text := strings.TrimSpace(p.statement.String())
	p.statement.Reset()

	if text == "" {
		return nil
	}

	return p.read(text)
}

// ParseOptions reads an options file's lines.
func ParseOptions(lines []string) (*OptionsFile, error) {
	var p OptionsParser

	for n, line := range lines {
		if err := p.Line(line); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}

	return p.End()
}

// field is one comma-separated part of a statement, trimmed, and where it
// starts in the statement's text.
type field struct {
	text  string
	start int
}

// statement is an option's statement: its keyword and the values after
// the "=".
type statement struct {
	text    string
	keyword field
	values  []field
}

// syntax is OPTSYNERR for the statement, f quoted.
func (s *statement) syntax(f field) error {
	return &OptionError{Ident: "OPTSYNERR", Line: s.quote(f)}
}

// valueRange is OPTVALERR for f, a number that isn't in low to high.
func (s *statement) valueRange(f field, option string, value, low, high uint32) error {
	return &OptionError{Ident: "OPTVALERR", Line: s.quote(f), Option: option, Value: value, Low: low, High: high}
}

// quote returns the statement's text with f between apostrophes.
func (s *statement) quote(f field) string {
	end := f.start + len(f.text)

	return s.text[:f.start] + "'" + f.text + "'" + s.text[end:]
}

// count checks the statement has from low to high values (high 0 for any
// number): too few is the last value in error, too many the first one
// past high.
func (s *statement) count(low, high int) error {
	switch {
	case len(s.values) < low:
		return s.syntax(s.values[len(s.values)-1])
	case high > 0 && len(s.values) > high:
		return s.syntax(s.values[high])
	}

	return nil
}

// number reads value i, a number from low to high.
func (s *statement) number(i int, option string, low, high uint32) (uint32, error) {
	f := s.values[i]

	n, err := optionNumber(f.text)
	if err != nil {
		return 0, s.syntax(f)
	}

	if n < low || n > high {
		return 0, s.valueRange(f, option, n, low, high)
	}

	return n, nil
}

// optionalNumber reads value i, a number from low to high, or nothing.
func (s *statement) optionalNumber(i int, option string, low, high uint32) (*uint32, error) {
	if s.values[i].text == "" {
		return nil, nil
	}

	n, err := s.number(i, option, low, high)
	if err != nil {
		return nil, err
	}

	return &n, nil
}

// name reads value i, a name of 1 to 31 characters, in uppercase.
func (s *statement) name(i int) (string, error) {
	f := s.values[i]
	if f.text == "" || len(f.text) > 31 || strings.ContainsAny(f.text, " \t\"") {
		return "", s.syntax(f)
	}

	return strings.ToUpper(f.text), nil
}

// names reads values from i on, names.
func (s *statement) names(i int) ([]string, error) {
	var out []string

	for j := i; j < len(s.values); j++ {
		n, err := s.name(j)
		if err != nil {
			return nil, err
		}

		out = append(out, n)
	}

	return out, nil
}

// keywordValue reads value i, one of keywords or a unique abbreviation of
// one.
func keywordValue[V any](s *statement, i int, keywords map[string]V) (string, error) {
	f := s.values[i]

	k, err := matchKeyword(strings.ToUpper(f.text), keywords)
	if err != nil {
		return "", s.syntax(f)
	}

	return k, nil
}

// whole returns the statement's value as one string: everything after the
// "=", as one field.
func (s *statement) whole() field {
	first, last := s.values[0], s.values[len(s.values)-1]

	return field{text: s.text[first.start : last.start+len(last.text)], start: first.start}
}

// Ranges an option's numbers must be in. GSMATCH='s are the image
// header's fields (and the probe's OPTVALERR); the rest are govax's.
const (
	maxMajorID   = 0xFF
	maxMinorID   = 0xFFFFFF
	maxStack     = 1 << 21
	maxPFC       = 127
	maxAlignment = 9 // a page
	maxNumber    = 1<<32 - 1
)

// gsmatchKeywords are GSMATCH='s match controls. NEVER is a match
// control too (ISD$K_MATNEV), but LINK refused it (the probe).
var gsmatchKeywords = map[string]Match{
	"ALWAYS": MatchAlways,
	"EQUAL":  MatchEqual,
	"LEQUAL": MatchLEQ,
}

// yesNo are PROTECT='s values.
var yesNo = map[string]bool{"YES": true, "NO": false}

// p0Bufs are IOSEGMENT='s second values.
var p0Bufs = map[string]bool{"P0BUFS": true, "NOP0BUFS": false}

// psectAttrKeywords are the attributes PSECT_ATTR= may set (the Linker
// manual's table of program section attributes), and alignments the
// powers of 2 named by keyword.
var psectAttrKeywords = map[string]bool{
	"PIC": true, "NOPIC": true,
	"SHR": true, "NOSHR": true,
	"WRT": true, "NOWRT": true,
	"EXE": true, "NOEXE": true,
	"RD": true, "NORD": true,
	"GBL": true, "LCL": true,
	"OVR": true, "CON": true,
	"REL": true, "ABS": true,
	"VEC": true, "NOVEC": true,
	"LIB": true, "USR": true,
	"BYTE": true, "WORD": true, "LONG": true, "QUAD": true, "OCTA": true, "PAGE": true,
}

var alignments = map[string]uint32{"BYTE": 0, "WORD": 1, "LONG": 2, "QUAD": 3, "OCTA": 4, "PAGE": 9}

// linkOptions are the options an options file may give, each a function
// that records its value. PSECT_ATTRIBUTE is VSI's name for PSECT_ATTR,
// which abbreviates it.
var linkOptions = map[string]func(p *OptionsParser, s *statement) error{
	"BASE": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		n, err := s.number(0, "BASE", 0, maxNumber)
		p.o.Base = &n

		return err
	},
	"CLUSTER": func(p *OptionsParser, s *statement) error {
		name, err := s.name(0)
		if err != nil {
			return err
		}

		c := Cluster{Name: name, Protect: p.protect}

		if len(s.values) > 1 {
			if c.Base, err = s.optionalNumber(1, "CLUSTER", 0, maxNumber); err != nil {
				return err
			}
		}

		if len(s.values) > 2 {
			pfc, err := s.optionalNumber(2, "CLUSTER", 0, maxPFC)
			if err != nil {
				return err
			}

			if pfc != nil {
				c.PFC = *pfc
			}
		}

		var files []InputFile

		for _, f := range s.values[min(3, len(s.values)):] {
			file, err := parseInputFile(f.text)
			if err != nil {
				return s.fileError(f, err)
			}

			file.Cluster = name
			files = append(files, file)
		}

		p.o.Clusters = append(p.o.Clusters, c)
		p.o.addFiles(files...)

		return nil
	},
	"COLLECT": func(p *OptionsParser, s *statement) error {
		if err := s.count(2, 0); err != nil {
			return err
		}

		names, err := s.names(0)
		if err != nil {
			return err
		}

		p.o.Collects = append(p.o.Collects, Collect{Cluster: names[0], Psects: names[1:], Protect: p.protect})

		return nil
	},
	"DZRO_MIN": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		n, err := s.number(0, "DZRO_MIN", 1, maxNumber)
		p.o.DZROMin = &n

		return err
	},
	"GSMATCH": func(p *OptionsParser, s *statement) error {
		if err := s.count(3, 3); err != nil {
			return err
		}

		k, err := keywordValue(s, 0, gsmatchKeywords)
		if err != nil {
			return err
		}

		major, err := s.number(1, "GSMATCH", 0, maxMajorID)
		if err != nil {
			return err
		}

		minor, err := s.number(2, "GSMATCH", 0, maxMinorID)
		if err != nil {
			return err
		}

		p.o.GSMatch = &GSMatch{Match: gsmatchKeywords[k], Major: uint8(major), Minor: minor}

		return nil
	},
	"IDENTIFICATION": func(p *OptionsParser, s *statement) error {
		f := s.whole()
		if ident := unquote(f.text); len(ident) <= 15 {
			p.o.Ident = ident

			return nil
		}

		return s.syntax(f)
	},
	"IOSEGMENT": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 2); err != nil {
			return err
		}

		n, err := s.number(0, "IOSEGMENT", 0, maxNumber)
		if err != nil {
			return err
		}

		// P0BUFS is the default (the 5.0 manual; VSI's says NOP0BUFS).
		seg := &IOSegment{Pages: n, P0Bufs: true}

		if len(s.values) > 1 {
			k, err := keywordValue(s, 1, p0Bufs)
			if err != nil {
				return err
			}

			seg.P0Bufs = p0Bufs[k]
		}

		p.o.IOSegment = seg

		return nil
	},
	"ISD_MAX": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		n, err := s.number(0, "ISD_MAX", 1, maxNumber)
		p.o.ISDMax = &n

		return err
	},
	"NAME": func(p *OptionsParser, s *statement) error {
		f := s.whole()
		if name := unquote(f.text); name != "" && len(name) <= 39 {
			p.o.Name = name

			return nil
		}

		return s.syntax(f)
	},
	"PROTECT": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		k, err := keywordValue(s, 0, yesNo)
		p.protect = yesNo[k]

		return err
	},
	"PSECT_ATTRIBUTE": func(p *OptionsParser, s *statement) error {
		if err := s.count(2, 0); err != nil {
			return err
		}

		name, err := s.name(0)
		if err != nil {
			return err
		}

		a := PsectAttr{Name: name}

		for i := 1; i < len(s.values); i++ {
			if s.values[i].text != "" && s.values[i].text[0] >= '0' && s.values[i].text[0] <= '9' {
				n, err := s.number(i, "PSECT_ATTR", 0, maxAlignment)
				if err != nil {
					return err
				}

				a.Align = &n

				continue
			}

			k, err := keywordValue(s, i, psectAttrKeywords)
			if err != nil {
				return err
			}

			if n, ok := alignments[k]; ok {
				a.Align = &n
			} else {
				a.Attributes = append(a.Attributes, k)
			}
		}

		p.o.PsectAttrs = append(p.o.PsectAttrs, a)

		return nil
	},
	"STACK": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		n, err := s.number(0, "STACK", 1, maxStack)
		p.o.Stack = int(n)

		return err
	},
	"SYMBOL": func(p *OptionsParser, s *statement) error {
		if err := s.count(2, 2); err != nil {
			return err
		}

		name, err := s.name(0)
		if err != nil {
			return err
		}

		n, err := s.number(1, "SYMBOL", 0, maxNumber)
		if err != nil {
			return err
		}

		p.o.Symbols = append(p.o.Symbols, Symbol{Name: name, Value: n})

		return nil
	},
	"UNIVERSAL": func(p *OptionsParser, s *statement) error {
		names, err := s.names(0)
		p.o.Universals = append(p.o.Universals, names...)

		return err
	},
	"UNSUPPORTED": func(p *OptionsParser, s *statement) error {
		if err := s.count(1, 1); err != nil {
			return err
		}

		n, err := s.number(0, "UNSUPPORTED", 0, maxNumber)
		p.o.Unsupported = &n

		return err
	},
}

// inputQualifiers are the qualifiers an input file may have in an options
// file, each a function that records it.
var inputQualifiers = map[string]func(f *InputFile, value string, hasValue bool) error{
	"LIBRARY": func(f *InputFile, _ string, hasValue bool) error {
		f.Library = true

		return noValue("LIBRARY", hasValue)
	},
	"INCLUDE": func(f *InputFile, value string, hasValue bool) error {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
		for _, m := range splitOutside(value, ',') {
			if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
				f.Include = append(f.Include, m)
			}
		}

		if !hasValue || len(f.Include) == 0 {
			return fmt.Errorf("/INCLUDE needs module names")
		}

		return nil
	},
	"SELECTIVE_SEARCH": func(f *InputFile, _ string, hasValue bool) error {
		f.Selective = true

		return noValue("SELECTIVE_SEARCH", hasValue)
	},
	"SHAREABLE": func(f *InputFile, value string, hasValue bool) error {
		f.Shareable = true

		// /SHAREABLE=COPY copies the image into the link, which isn't
		// supported; NOCOPY is the default.
		if hasValue && !strings.HasPrefix("NOCOPY", strings.ToUpper(value)) {
			return unsupportedError{fmt.Errorf("/SHAREABLE=%s isn't supported", value)}
		}

		return nil
	},
}

// unsupportedError is an input file's qualifier VMS's LINK takes and
// govax's doesn't: not a syntax error.
type unsupportedError struct{ error }

func (e unsupportedError) Unwrap() error { return e.error }

// fileError is the error for f, an input file spec parseInputFile
// refused with err: OPTSYNERR with f quoted, unless govax refused
// something VMS takes.
func (s *statement) fileError(f field, err error) error {
	var unsupported unsupportedError
	if errors.As(err, &unsupported) {
		return err
	}

	return s.syntax(f)
}

func noValue(name string, hasValue bool) error {
	if hasValue {
		return fmt.Errorf("/%s takes no value", name)
	}

	return nil
}

// read reads one statement: an option or a line of input files.
func (p *OptionsParser) read(text string) error {
	eq := indexOutside(text, '=')
	if eq >= 0 && !strings.ContainsAny(text[:eq], "/:[]<>.;\"") {
		s := &statement{text: text, keyword: trimmed(text[:eq], 0)}
		s.values = fields(text[eq+1:], eq+1)

		name, err := matchKeyword(strings.ToUpper(s.keyword.text), linkOptions)
		if err != nil {
			return s.syntax(s.keyword)
		}

		return linkOptions[name](p, s)
	}

	s := &statement{text: text, values: fields(text, 0)}

	for _, f := range s.values {
		file, err := parseInputFile(f.text)
		if err != nil {
			return s.fileError(f, err)
		}

		p.o.addFiles(file)
	}

	return nil
}

// parseInputFile reads one input file and its qualifiers.
func parseInputFile(spec string) (InputFile, error) {
	parts := splitOutside(spec, '/')

	f := InputFile{Name: unquote(strings.TrimSpace(parts[0]))}
	if f.Name == "" {
		return f, fmt.Errorf("%q names no file", spec)
	}

	for _, q := range parts[1:] {
		name, value, hasValue := strings.Cut(q, "=")

		key, err := matchKeyword(strings.ToUpper(strings.TrimSpace(name)), inputQualifiers)
		if err != nil {
			return f, fmt.Errorf("%s: /%s: %w", f.Name, strings.TrimSpace(name), err)
		}

		if err := inputQualifiers[key](&f, strings.TrimSpace(value), hasValue); err != nil {
			return f, fmt.Errorf("%s: %w", f.Name, err)
		}
	}

	return f, nil
}

// matchKeyword finds the keyword word abbreviates, uniquely.
func matchKeyword[V any](word string, keywords map[string]V) (string, error) {
	if _, ok := keywords[word]; ok {
		return word, nil
	}

	var found []string

	for k := range keywords {
		if word != "" && strings.HasPrefix(k, word) {
			found = append(found, k)
		}
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("isn't supported")
	}

	return "", fmt.Errorf("is ambiguous")
}

// optionNumber reads a number: decimal, or with a radix prefix.
func optionNumber(s string) (uint32, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	base := 10

	switch {
	case strings.HasPrefix(s, "%X"):
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "%O"):
		base, s = 8, s[2:]
	case strings.HasPrefix(s, "%D"):
		s = s[2:]
	}

	n, err := strconv.ParseUint(s, base, 32)

	return uint32(n), err
}

// stripComment removes a comment: from an exclamation point that isn't in
// quotes.
func stripComment(line string) string {
	quoted := false

	for i, c := range line {
		switch {
		case c == '"':
			quoted = !quoted
		case c == '!' && !quoted:
			return line[:i]
		}
	}

	return line
}

// fields splits s at each comma that isn't in quotes or parentheses into
// trimmed fields, each with where it starts, offset by start (where s
// starts in its statement).
func fields(s string, start int) []field {
	var out []field

	at := 0
	for _, part := range splitOutside(s, ',') {
		out = append(out, trimmed(part, start+at))
		at += len(part) + 1
	}

	return out
}

// trimmed is s, which starts at start, as a field without its blanks.
func trimmed(s string, start int) field {
	text := strings.TrimLeft(s, " \t")
	start += len(s) - len(text)

	return field{text: strings.TrimRight(text, " \t"), start: start}
}

// indexOutside returns the index of the first sep in s that isn't in
// quotes or parentheses, or -1.
func indexOutside(s string, sep rune) int {
	if parts := splitOutside(s, sep); len(parts) > 1 {
		return len(parts[0])
	}

	return -1
}

// splitOutside splits s at each sep that isn't in quotes or parentheses.
func splitOutside(s string, sep rune) []string {
	var (
		out    []string
		depth  int
		quoted bool
		start  int
	)

	for i, c := range s {
		switch {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}

	return append(out, s[start:])
}

// unquote removes the quotes around a quoted string.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	return s
}

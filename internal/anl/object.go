package anl

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// ObjectOptions are the ANALYZE/OBJECT choices that change a report.
type ObjectOptions struct {
	// Select, when not empty, limits the records described to these
	// types (/MHD, /GSD, /TIR, ...): every record is still read and
	// checked, and counted in the summary, but only these are shown.
	// HDR selects every header record.
	Select []obj.RecordType
}

// ObjectReport is the result of analyzing an object file.
type ObjectReport struct {
	Lines  []Line
	Errors int
}

// The record types the summary counts, in the order it lists them.
var summaryTypes = []obj.RecordType{
	obj.RecHDR, obj.RecGSD, obj.RecTIR, obj.RecEOM,
	obj.RecDBG, obj.RecTBT, obj.RecLNK, obj.RecEOMW,
}

// AnalyzeObject describes every record of an object file, given as its
// records (each starting with its record type byte), as ANALYZE/OBJECT
// does. A file may hold several modules, one after another, each ending
// with its end of module record.
func AnalyzeObject(records [][]byte, opts ObjectOptions) ObjectReport {
	a := &objectAnalyzer{opts: opts, counts: map[obj.RecordType]int{}, sizes: map[obj.RecordType]int{}}
	a.run(records)

	return ObjectReport{Lines: a.lines, Errors: a.errors}
}

// objectAnalyzer is one ANALYZE/OBJECT in progress.
type objectAnalyzer struct {
	report

	opts   ObjectOptions
	errors int

	// counts and sizes are the summary: how many records of each type,
	// and their total size in bytes.
	counts map[obj.RecordType]int
	sizes  map[obj.RecordType]int

	// The state of the module being read: whether its end of module
	// record has been seen, how many psects it has defined so far, and
	// the depth of the linker's stack, which TIR, DBG, and TBT records
	// share.
	inModule bool
	psects   int
	depth    int

	// records are the file's records; modulePsects is how many psects
	// the current module defines in all, and maxSize its main header's
	// maximum record size.
	records      [][]byte
	modulePsects int
	maxSize      int
}

// How many lines each kind of heading needs left on a page to be written
// there (Line.Keep). Real ANALYZE's rule, reconstructed from the page
// breaks of the 54 fixtures (docs/PHASE-38.md): a record's heading needs
// 5, a GSD subrecord's or a TIR command's 3, a flags list's label 3, a
// STORE IMMEDIATE's 2, and a hex dump's 4. A subrecord's could be 2: the
// fixtures allow 2 or 3, and 3 matches a TIR command's.
const (
	keepRecord    = 5
	keepItem      = 3
	keepFlags     = 3
	keepImmediate = 2
	keepDump      = 4
)

// recordTitles are each record type's heading, by type; header records
// are titled by their header type instead (headerTitles).
var recordTitles = map[obj.RecordType]string{
	obj.RecGSD:  "GLOBAL SYMBOL DIRECTORY",
	obj.RecTIR:  "TEXT INFORMATION/RELOCATION",
	obj.RecEOM:  "END OF MODULE",
	obj.RecDBG:  "DEBUGGER INFORMATION",
	obj.RecTBT:  "TRACEBACK INFORMATION",
	obj.RecLNK:  "LINK OPTION SPECIFICATION",
	obj.RecEOMW: "END OF MODULE WITH WORD PSECT",
}

// headerTitles are each header record's heading, by header type.
var headerTitles = map[obj.HeaderType]string{
	obj.HdrMHD: "MODULE HEADER",
	obj.HdrLNM: "LANGUAGE PROCESSOR HEADER",
	obj.HdrSRC: "SOURCE FILE HEADER",
	obj.HdrTTL: "TITLE HEADER",
	obj.HdrCPR: "COPYRIGHT HEADER",
	obj.HdrMTC: "MAINTENANCE STATUS HEADER",
	obj.HdrGTX: "GENERAL TEXT HEADER",
}

func (a *objectAnalyzer) run(records [][]byte) {
	a.records = records

	a.line("This is an OpenVMS VAX object file")
	a.blank()

	for i, raw := range records {
		a.record(i+1, raw)
	}

	if a.inModule {
		a.moduleEnd(true)
	}

	a.summary(len(records))
}

// selected reports whether records of type t are shown.
func (a *objectAnalyzer) selected(t obj.RecordType) bool {
	if len(a.opts.Select) == 0 {
		return true
	}

	for _, s := range a.opts.Select {
		if s == t {
			return true
		}
	}

	return false
}

// fail reports an error where it's found.
func (a *objectAnalyzer) fail(format string, args ...any) {
	a.errors++
	a.line("***  " + fmt.Sprintf(format, args...))
}

// moduleEnd checks what must be true when a module ends: its stack is
// empty, and, when the module ends without one, that it had an end of
// module record.
func (a *objectAnalyzer) moduleEnd(missingEOM bool) {
	if missingEOM {
		a.fail("End of module record is missing from previous module.")
	}

	if a.depth != 0 {
		a.fail("The stack still contains %d longword%s.", a.depth, plural(a.depth))
	}

	a.inModule, a.psects, a.depth = false, 0, 0
}

func (a *objectAnalyzer) record(n int, raw []byte) {
	if len(raw) == 0 {
		a.fail("Record %d is empty.", n)

		return
	}

	t := obj.RecordType(raw[0])
	a.counts[t]++
	a.sizes[t] += len(raw)

	mainHeader := t == obj.RecHDR && len(raw) > 1 && obj.HeaderType(raw[1]) == obj.HdrMHD

	switch {
	case mainHeader && a.inModule:
		a.moduleEnd(true)
		a.moduleStart(n - 1)
	case mainHeader:
		a.moduleStart(n - 1)
	case !a.inModule:
		a.moduleStart(n - 1)
		a.fail("The module header record is missing.")
	}

	// A main header's own size is checked against the maximum it gives.
	if mainHeader && len(raw) >= 5 {
		a.maxSize = int(binary.LittleEndian.Uint16(raw[3:5]))
	}

	show := a.selected(t)
	if show {
		a.heading(n, t, raw)
	}

	if a.maxSize > 0 && len(raw) > a.maxSize {
		a.fail("The record is longer than the maximum record size, %d bytes.", a.maxSize)
	}

	ended := false

	switch t {
	case obj.RecGSD:
		a.gsdRecord(show, raw[1:])

	case obj.RecTIR, obj.RecDBG, obj.RecTBT:
		a.tirRecord(show, raw[1:])

	default:
		ended = a.otherRecord(show, raw)
	}

	if show {
		a.spill()
		a.spill()
	}

	if ended {
		a.moduleEnd(false)
	}
}

// moduleStart starts a module whose first record is records[first]: it
// counts the psects the module defines, since a TIR command or symbol may
// refer to one a later GSD record defines.
func (a *objectAnalyzer) moduleStart(first int) {
	a.inModule, a.psects, a.depth, a.maxSize = true, 0, 0, 0
	a.modulePsects = 0

	for i, raw := range a.records[first:] {
		if len(raw) == 0 {
			continue
		}

		t := obj.RecordType(raw[0])

		if i > 0 && t == obj.RecHDR && len(raw) > 1 && obj.HeaderType(raw[1]) == obj.HdrMHD {
			return
		}

		if t == obj.RecEOM || t == obj.RecEOMW {
			return
		}

		if t != obj.RecGSD {
			continue
		}

		for b := raw[1:]; len(b) > 0; {
			sub, used, err := obj.DecodeSubrecord(b)
			if err != nil {
				break
			}

			if _, ok := sub.(*obj.Psect); ok {
				a.modulePsects++
			}

			b = b[used:]
		}
	}
}

// gsdRecord describes a GSD record's subrecords, up to one that's
// malformed.
func (a *objectAnalyzer) gsdRecord(show bool, b []byte) {
	for k := 1; len(b) > 0; k++ {
		sub, used, err := obj.DecodeSubrecord(b)
		if err != nil {
			a.fail("GSD subrecord %d is malformed: %v.", k, unwrap(err))

			return
		}

		if show && k > 1 {
			a.blank()
		}

		a.subrecord(show, k, sub)
		a.checkSubrecord(sub)

		b = b[used:]
	}
}

// tirRecord describes a TIR, DBG, or TBT record's commands, up to one
// that's malformed.
func (a *objectAnalyzer) tirRecord(show bool, b []byte) {
	for k := 1; len(b) > 0; k++ {
		c, used, err := obj.DecodeCommand(b)
		if err != nil {
			a.fail("Command %d is malformed: %v.", k, unwrap(err))

			return
		}

		if show && k > 1 {
			a.blank()
		}

		underflow := a.command(show, k, c)
		a.checkCommand(c, underflow)

		b = b[used:]
	}
}

// otherRecord describes a header, end of module, or link option record,
// and reports whether it ended the module.
func (a *objectAnalyzer) otherRecord(show bool, raw []byte) bool {
	m, err := obj.Decode([][]byte{raw})
	if err != nil {
		a.fail("The record is malformed: %v.", unwrap(err))

		return false
	}

	switch rec := m.Records[0].(type) {
	case *obj.MainHeader:
		if show {
			a.mainHeader(rec)
		}

		a.checkName("module", rec.Name)

	case *obj.TextHeader:
		if show {
			a.line("\tTextual information:")

			for _, part := range headerText(rec.Text) {
				a.line("\t" + quote(part))
			}
		}

	case *obj.EOM:
		if show {
			a.endOfModule(rec)
		}

		if rec.Severity > obj.SeverityAbort {
			a.fail("Severity %d is undefined.", rec.Severity)
		}

		if rec.HasTransfer {
			a.checkPsect(rec.Psect)
		}

		return true

	case *obj.LNK:
		if show {
			a.linkOption(rec)
		}

	case *obj.Unknown:
		a.fail("Record type %d is undefined.", byte(rec.Type))
	}

	return false
}

// heading adds a record's heading line and the blank line after it.
func (a *objectAnalyzer) heading(n int, t obj.RecordType, raw []byte) {
	title, code := recordTitles[t], "OBJ$C_"+t.String()

	if t == obj.RecHDR && len(raw) > 1 {
		h := obj.HeaderType(raw[1])
		title = headerTitles[h]
		code += "_" + h.String()
	}

	if title == "" {
		title = "UNKNOWN RECORD TYPE"
	}

	a.keep(keepRecord, fmt.Sprintf("%d.  %s (%s), %d byte%s", n, title, code, len(raw), plural(len(raw))))
	a.blank()
}

func (a *objectAnalyzer) mainHeader(h *obj.MainHeader) {
	a.line(fmt.Sprintf("\tstructure level: %d", h.StructureLevel))
	a.line(fmt.Sprintf("\tmaximum record size: %d", h.MaxRecordSize))
	a.line("\tmodule name: " + quote(h.Name))
	a.line("\tmodule version: " + quote(h.Version))
	a.line("\tcreation   date/time: " + h.Created)

	// The patch time is shown only when there is one; real MACRO never
	// writes one, so its line is unconfirmed. Its label is the one
	// "creation   date/time" is padded to match.
	if strings.Trim(h.Patched, " \x00") != "" {
		a.line("\tlast patch date/time: " + h.Patched)
	}
}

func (a *objectAnalyzer) endOfModule(e *obj.EOM) {
	a.line(fmt.Sprintf("\tseverity: %s (%d)", severityName(e.Severity), e.Severity))

	if e.HasTransfer {
		a.line(fmt.Sprintf("\tpsect: %d", e.Psect))
		a.line("\tvalue: " + signedValue(e.Transfer))
	}

	if e.HasFlags {
		a.line(fmt.Sprintf("\ttransfer flags: %d (%%X'%02X')", e.Flags, e.Flags))
	}
}

// headerTextWidth is how many characters of a text header's text ANALYZE
// shows on a line.
const headerTextWidth = 65

// headerText is a text header's text as ANALYZE shows it: in pieces of
// 65 characters, one to a line, with each unprintable character a
// period, as in a hex dump.
func headerText(text string) []string {
	b := []byte(text)
	for i, c := range b {
		b[i] = dumpChar(c)
	}

	var parts []string

	for len(b) > headerTextWidth {
		parts = append(parts, string(b[:headerTextWidth]))
		b = b[headerTextWidth:]
	}

	return append(parts, string(b))
}

// severityName is an EOM completion code's description. VMS's word for
// 2 is "errors" (testdata/mar/dst/vax/dstsym.anl); 4's is govax's
// (unconfirmed).
func severityName(s byte) string {
	switch s {
	case obj.SeveritySuccess:
		return "successful"
	case obj.SeverityWarning:
		return "warning"
	case obj.SeverityError:
		return "errors"
	case obj.SeverityAbort:
		return "abort"
	}

	return "unknown"
}

func (a *objectAnalyzer) summary(total int) {
	a.page("SUMMARY STATISTICS:")
	a.blank()
	a.line("Record Type\tCount\tTotal Bytes")
	a.blank()

	bytes := 0

	for _, t := range summaryTypes {
		a.line(fmt.Sprintf("OBJ$C_%s\t%5d\t%6d", t, a.counts[t], a.sizes[t]))
		bytes += a.sizes[t]
	}

	a.blank()
	a.line(fmt.Sprintf("Totals\t\t%5d\t%6d", total, bytes))
	a.blank()
	a.blank()

	if a.errors == 0 {
		a.line("The analysis uncovered NO errors.")
	} else {
		a.line(fmt.Sprintf("The analysis uncovered %d error%s.", a.errors, plural(a.errors)))
	}

	a.blank()
	a.blank()
}

// signedValue shows a longword as ANALYZE does: as a signed decimal
// number, then in hex.
func signedValue(v uint32) string {
	return fmt.Sprintf("%d (%%X'%08X')", int32(v), v)
}

// unsignedValue shows a longword as an unsigned decimal number, then in
// hex.
func unsignedValue(v uint32) string {
	return fmt.Sprintf("%d (%%X'%08X')", v, v)
}

// plural is "s" unless n is 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}

// flagBit is one named bit of a flags word.
type flagBit struct {
	bit  int
	name string
}

// flagBits returns the bits named by every symbol with prefix in
// vmsdef.Symbols ("GPS$V_"), in bit order.
func flagBits(prefix string) []flagBit {
	var bits []flagBit

	for name, v := range vmsdef.Symbols {
		if strings.HasPrefix(name, prefix) {
			bits = append(bits, flagBit{int(v), name})
		}
	}

	sort.Slice(bits, func(i, j int) bool { return bits[i].bit < bits[j].bit })

	return bits
}

// flagLines adds a line for every named bit of flags, with its value.
func (a *objectAnalyzer) flagLines(bits []flagBit, flags uint16) {
	for _, b := range bits {
		a.line(fmt.Sprintf("\t\t\t%-5s%-17s%d", fmt.Sprintf("(%d)", b.bit), b.name, flags>>b.bit&1))
	}
}

package anl

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/ods2/vmstime"
)

// ANALYZE/RMS_FILE's reports (docs/PHASE-88 - keyed index files.md, subtask 10d):
// /CHECK's integrity report, /STATISTICS, /SUMMARY, and /FDL. isam.Inspector reads
// the file; this file lays out what it found, in the order it finds it, so a
// report's "***" lines come where VMS's do: a prolog's sums before FIXED PROLOG, a
// key's walk right after its descriptor. VMS 7.1's reports in testdata/idx88 are the
// layout's only source, and TestIdx88ReportsOracle holds them to it line for line.

// RMSMode is which report to write.
type RMSMode int

// The reports, in the order VMS prefers them when a command names several.
const (
	RMSCheck RMSMode = iota
	RMSSummary
	RMSStatistics
	RMSFDL
)

// The statuses ANALYZE/RMS_FILE leaves when it found nothing wrong.
const (
	// RMSStatusReported is a report on SYS$OUTPUT with its ERRORS message.
	RMSStatusReported = 0x103B8103

	// RMSStatusWritten is any other report.
	RMSStatusWritten = 0x103B8009
)

// RMSOptions is one report's settings.
type RMSOptions struct {
	Mode    RMSMode
	Command string

	// Message says the report carries the %ANLRMS-I-ERRORS line.
	Message bool

	// OwnerName writes an owner's UIC.
	OwnerName func(uint32) string

	// Now is the clock; time.Now when nil.
	Now func() time.Time
}

// RMSReport is a finished report.
type RMSReport struct {
	Text []byte

	// Errors counts the findings, and Status is the first with a status of its own, 0
	// for none: VMS's $STATUS.
	Errors int
	Status uint32
}

const (
	rmsPageBody   = 54 // lines of a report below a page's heading
	rmsTitleWidth = 45 // the column a page heading's date starts at
)

// rmsWriter lays a report out. /FDL has no pages: the title is empty.
type rmsWriter struct {
	b     strings.Builder
	title string
	file  string
	now   func() time.Time
	page  int
	count int

	// paged is every report but FDL's.
	paged bool
}

func (w *rmsWriter) newPage() {
	w.page++
	w.count = 0

	fmt.Fprintf(&w.b, "\f\n%-*s%s   Page %d\n%s\n\n\n", rmsTitleWidth, w.title, timeText(w.now(), true), w.page, w.file)
}

// line writes one line, starting a page when the page is full.
func (w *rmsWriter) line(text string) {
	if w.paged && (w.page == 0 || w.count >= rmsPageBody) {
		w.newPage()
	}

	w.count++

	w.b.WriteString(text)
	w.b.WriteByte('\n')
}

func (w *rmsWriter) linef(format string, args ...any) {
	w.line(fmt.Sprintf(format, args...))
}

// heading writes a section's heading and the blank after it, on a new page if
// fewer than that many lines and one more are left on this one.
func (w *rmsWriter) heading(text string) {
	if w.paged && w.count+rmsHeadingRoom > rmsPageBody {
		w.newPage()
	}

	w.line(text)
	w.line("")
}

// rmsHeadingRoom is how many lines a heading wants to have below it, itself and its
// blank line among them.
const rmsHeadingRoom = 3

// timeText writes a time as $ASCTIM does, with its hundredths if asked.
func timeText(t time.Time, hundredths bool) string {
	text := fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d", t.Day(), strings.ToUpper(t.Month().String()[:3]), t.Year(), t.Hour(), t.Minute(), t.Second())
	if hundredths {
		text += fmt.Sprintf(".%02d", t.Nanosecond()/10_000_000)
	}

	return text
}

func vmsDate(t uint64) string { return timeText(vmstime.VMSTime(int64(t)).Time(), true) }

// protectionLetters is one category's access: a set bit in a mask denies.
func protectionLetters(mask uint16) string {
	var b strings.Builder

	for i, letter := range "RWED" {
		if mask&(1<<i) == 0 {
			b.WriteRune(letter)
		}
	}

	return b.String()
}

// rmsAnalysis is one report in progress.
type rmsAnalysis struct {
	w    *rmsWriter
	opts RMSOptions
	file *rms.AnalyzedFile
	in   *isam.Inspector
	rep  RMSReport
}

// AnalyzeRMSFile writes one file's report.
func AnalyzeRMSFile(file *rms.AnalyzedFile, opts RMSOptions) RMSReport {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	titles := map[RMSMode]string{RMSCheck: "Check RMS File Integrity", RMSStatistics: "RMS File Statistics", RMSSummary: "RMS File Summary"}

	a := &rmsAnalysis{
		w:    &rmsWriter{title: titles[opts.Mode], file: file.Attr.Name, now: now, paged: opts.Mode != RMSFDL},
		opts: opts,
		file: file,
	}

	if file.Blocks != nil && file.Indexed() {
		a.in = isam.NewInspector(file.Blocks, file.Isam, file.Attr.Allocated, a.finding)
	}

	if opts.Mode == RMSFDL {
		a.fdl(now())
	} else {
		a.report()
	}

	a.rep.Text = []byte(a.w.b.String())

	return a.rep
}

// finding writes a finding's line and counts it.
func (a *rmsAnalysis) finding(f isam.Finding) {
	a.rep.Errors++

	if a.rep.Status == 0 {
		a.rep.Status = f.Status
	}

	if f.VBN == 0 {
		a.w.linef("***  %s", f.Text)
	} else {
		a.w.linef("***  VBN %d:  %s", f.VBN, f.Text)
	}
}

// unrecoverable writes the line a walk's end gets, if it ended.
func (a *rmsAnalysis) unrecoverable() bool {
	if a.in == nil || !a.in.Stopped {
		return false
	}

	a.w.line("Unrecoverable error encountered in structure of file.")

	return true
}

func (a *rmsAnalysis) report() {
	a.fileHeader()
	a.attributes()

	switch {
	case a.in != nil:
		a.indexed()
	case a.file.Blocks != nil && a.file.Relative():
		a.relative()
	}

	a.closing()
}

func (a *rmsAnalysis) closing() {
	w := a.w
	w.line("")
	w.line("")

	switch a.rep.Errors {
	case 0:
		w.line("The analysis uncovered NO errors.")
	case 1:
		w.line("The analysis uncovered 1 error.")
	default:
		w.linef("The analysis uncovered %d errors.", a.rep.Errors)
	}

	if a.opts.Message {
		w.line(ErrorsMessage(a.file.Attr.Name, a.rep.Errors))
	}

	w.line("")
	w.line("")
	w.line(a.opts.Command)
}

// ErrorsMessage is the message ending a report on SYS$OUTPUT, or all /NOOUTPUT shows.
func ErrorsMessage(name string, errors int) string {
	noun := "errors"
	if errors == 1 {
		noun = "error"
	}

	return fmt.Sprintf("%%ANLRMS-I-ERRORS, <%s> %d %s", name, errors, noun)
}

func (a *rmsAnalysis) owner() string {
	if a.opts.OwnerName != nil {
		return a.opts.OwnerName(a.file.Attr.Owner)
	}

	return fmt.Sprintf("[%o,%o]", a.file.Attr.Owner>>16, a.file.Attr.Owner&0xFFFF)
}

func (a *rmsAnalysis) fileHeader() {
	w, attr := a.w, a.file.Attr

	w.line("FILE HEADER")
	w.line("")
	w.linef("\tFile Spec: %s", attr.Name)
	w.linef("\tFile ID: %s", attr.FID)
	w.linef("\tOwner UIC: %s", a.owner())

	p := attr.Protection
	w.linef("\tProtection:  System: %s, Owner: %s, Group: %s, World: %s",
		protectionLetters(p), protectionLetters(p>>4), protectionLetters(p>>8), protectionLetters(p>>12))
	w.linef("\tCreation Date:   %s", vmsDate(attr.Created))
	w.linef("\tRevision Date:   %s, Number: %d", vmsDate(attr.Revised), attr.Revision)

	if attr.Expires == 0 {
		w.line("\tExpiration Date: none specified")
	} else {
		w.linef("\tExpiration Date: %s", vmsDate(attr.Expires))
	}

	if attr.Backup == 0 {
		w.line("\tBackup Date:     none posted")
	} else {
		w.linef("\tBackup Date:     %s", vmsDate(attr.Backup))
	}

	contiguity := "none"

	switch {
	case attr.Contiguous():
		contiguity = "contiguous"
	case attr.ContiguousBestTry():
		contiguity = "contiguous-best-try"
	}

	w.linef("\tContiguity Options:  %s", contiguity)
	w.line("\tPerformance Options: none")
	w.linef("\tReliability Options: %s", reliabilityText(attr))
	w.line("\tJournaling Enabled:  none")
}

// reliabilityText names the file's checks. VMS's own words for them are unconfirmed:
// no file of the probe had one.
func reliabilityText(attr rms.FileAttributes) string {
	var names []string
	if attr.ReadCheck() {
		names = append(names, "read-check")
	}

	if attr.WriteCheck() {
		names = append(names, "write-check")
	}

	if len(names) == 0 {
		return "none"
	}

	return strings.Join(names, ", ")
}

var rmsOrganizations = []string{"sequential", "relative", "indexed"}

var rmsFormats = []string{"undefined", "fixed", "variable", "variable-with-fixed-control", "stream", "stream-LF", "stream-CR"}

// FDL's words for the same formats.
var fdlFormats = []string{"undefined", "fixed", "variable", "vfc", "stream", "stream_lf", "stream_cr"}

// The record attribute bits, FAB$M_FTN and the rest.
const (
	ratFortran = 0x01
	ratCR      = 0x02
	ratPrint   = 0x04
	ratNoSpan  = 0x08
)

func nameOf(names []string, i int) string {
	if i < len(names) {
		return names[i]
	}

	return "unknown"
}

func (a *rmsAnalysis) attributes() {
	w, attr := a.w, a.file.Attr

	w.line("")
	w.line("")
	w.line("RMS FILE ATTRIBUTES")
	w.line("")
	w.linef("\tFile Organization: %s", nameOf(rmsOrganizations, int(attr.Organization)))
	w.linef("\tRecord Format: %s", nameOf(rmsFormats, int(attr.Format)))

	span, control := "", ""
	if attr.RecordAttributes&ratNoSpan != 0 {
		span = "no-span"
	}

	switch {
	case attr.RecordAttributes&ratFortran != 0:
		control = "fortran"
	case attr.RecordAttributes&ratCR != 0:
		control = "carriage-return"
	case attr.RecordAttributes&ratPrint != 0:
		control = "print"
	}

	w.linef("\tRecord Attributes: %s %s ", span, control)
	w.linef("\tMaximum Record Size: %d", attr.RecordSize)

	sequential := attr.Organization == 0
	// A variable relative file's isn't shown (C65.REL, whose header has 200), and
	// any fixed format file's is (C2H.DAT, G109.IDX).
	if sequential || attr.Format == 1 {
		w.linef("\tLongest Record: %d", attr.LongestRecord)
	}

	if attr.Format == 3 {
		w.linef("\tFixed Control Size: %d", attr.FixedControl)
	}

	w.linef("\tBlocks Allocated: %d, Default Extend Size: %d", attr.Allocated, attr.Extend)

	if sequential {
		w.linef("\tEnd-of-File VBN: %d, Offset: %%X'%04X'", attr.EndBlock, attr.FirstFreeByte)
	} else {
		w.linef("\tBucket Size: %d", attr.BucketSize)
	}

	w.line("\tFile Monitoring: disabled")
	w.linef("\tGlobal Buffer Count: %d", attr.GlobalBuffers)
}

func (a *rmsAnalysis) relative() {
	w := a.w
	p := isam.ReadFixedProlog(a.file.Blocks)

	w.line("")
	w.line("")
	w.line("FIXED PROLOG")
	w.line("")
	w.line("\tProlog Flags:")

	noExtend := 0
	if p.NoExtend {
		noExtend = 1
	}

	w.linef("\t\t(0)  PLG$V_NOEXTEND   %d", noExtend)
	w.linef("\tFirst Data Bucket VBN: %d", p.DataVBN)
	w.linef("\tMaximum Record Number: %d", p.MaxRecord)
	w.linef("\tEnd-of-File VBN: %d", p.EOF)
	w.linef("\tProlog Version: %d", p.Version)
}

func (a *rmsAnalysis) indexed() {
	w, in := a.w, a.in

	in.CheckSums()

	w.line("")
	w.line("")
	w.line("FIXED PROLOG")
	w.line("")
	w.linef("\tNumber of Areas: %d, VBN of First Descriptor: %d", in.Prolog.AreaCount, in.Prolog.AreaVBN)
	w.linef("\tProlog Version: %d", in.Prolog.Version)

	in.CheckAreasLocated()

	if a.unrecoverable() {
		return
	}

	for i := range in.Areas {
		a.area(i)
	}

	for _, k := range in.Keys {
		a.key(k)

		if a.opts.Mode == RMSSummary {
			continue
		}

		a.statistics(k, in.Walk(k))

		if a.unrecoverable() {
			return
		}
	}
}

func (a *rmsAnalysis) area(i int) {
	w, area := a.w, a.in.Areas[i]

	const perBlock = isam.BlockSize / 64

	w.line("")
	w.heading(fmt.Sprintf("AREA DESCRIPTOR #%d (VBN %d, offset %%X'%04X')", i, a.in.Prolog.AreaVBN+uint32(i/perBlock), i%perBlock*64))
	w.linef("\tBucket Size: %d", area.BucketSize)
	w.linef("\tReclaimed Bucket VBN: %d", area.Available)
	w.linef("\tCurrent Extent Start: %d, Blocks: %d, Used: %d, Next: %d", area.ExtentVBN, area.ExtentSize, area.Used, area.NextVBN)
	w.linef("\tDefault Extend Quantity: %d", area.ExtendSize)
	w.linef("\tTotal Allocation: %d", area.TotalAllocation)

	if a.opts.Mode == RMSStatistics {
		w.line("")
		w.heading(fmt.Sprintf("STATISTICS FOR AREA #%d", i))
		w.line("\tCount of Reclaimed Blocks:              0")
	}
}

func flagValue(flags, bit uint8) int {
	if flags&bit != 0 {
		return 1
	}

	return 0
}

func dataTypeName(t isam.Type) string {
	names := map[isam.Type]string{
		isam.String: "string", isam.Int2: "signed word", isam.Bin2: "unsigned word", isam.Int4: "signed longword",
		isam.Bin4: "unsigned longword", isam.Packed: "packed decimal", isam.Int8: "signed quadword",
		isam.Bin8: "unsigned quadword", isam.Collated: "collated",
	}

	name, ok := names[t.Kind()]
	if !ok {
		name = "unknown"
	}

	if t.IsDescending() {
		return "descending " + name
	}

	return name
}

func (a *rmsAnalysis) key(k *isam.Key) {
	w := a.w

	w.line("")
	w.heading(fmt.Sprintf("KEY DESCRIPTOR #%d (VBN %d, offset %%X'%04X')", k.Ref, k.DescriptorVBN, k.DescriptorOffset))

	if k.NextDescriptorVBN != 0 {
		w.linef("\tNext Key Descriptor VBN: %d, Offset: %%X'%04X'", k.NextDescriptorVBN, k.NextDescriptorOffset)
	}

	w.linef("\tIndex Area: %d, Level 1 Index Area: %d, Data Area: %d", k.IndexArea, k.Level1Area, k.DataArea)
	w.linef("\tRoot Level: %d", k.RootLevel)
	w.linef("\tIndex Bucket Size: %d, Data Bucket Size: %d", k.IndexBucketSize, k.DataBucketSize)

	initialized := k.Flags&isam.FlagNoIndexYet == 0
	if initialized {
		w.linef("\tRoot VBN: %d", k.RootVBN)
	}

	w.line("\tKey Flags:")
	w.linef("\t\t(0)  KEY$V_DUPKEYS    %d", flagValue(k.Flags, isam.FlagDuplicates))

	if k.Ref != 0 {
		w.linef("\t\t(1)  KEY$V_CHGKEYS    %d", flagValue(k.Flags, isam.FlagChanges))
		w.linef("\t\t(2)  KEY$V_NULKEYS    %d", flagValue(k.Flags, isam.FlagNull))
	}

	w.linef("\t\t(3)  KEY$V_IDX_COMPR  %d", flagValue(k.Flags, isam.FlagIndexCompress))
	w.linef("\t\t(4)  KEY$V_INITIDX    %d", flagValue(k.Flags, isam.FlagNoIndexYet))
	w.linef("\t\t(6)  KEY$V_KEY_COMPR  %d", flagValue(k.Flags, isam.FlagKeyCompress))

	if k.Ref == 0 {
		w.linef("\t\t(7)  KEY$V_REC_COMPR  %d", flagValue(k.Flags, isam.FlagRecordCompress))
	}

	w.linef("\tKey Segments: %d", len(k.Segments))

	if k.Flags&isam.FlagNull != 0 {
		w.linef("\tNull Character: %%X'%02X'", k.Null)
	}

	w.linef("\tKey Size: %d", k.Size)
	w.linef("\tMinimum Record Size: %d", k.MinRecord)
	w.linef("\tIndex Fill Quantity: %d, Data Fill Quantity: %d", k.IndexFill, k.DataFill)

	var positions, sizes strings.Builder
	for _, s := range k.Segments {
		fmt.Fprintf(&positions, "%6d", s.Position)
		fmt.Fprintf(&sizes, "%6d", s.Size)
	}

	w.linef("\tSegment Positions:  %s", positions.String())
	w.linef("\tSegment Sizes:      %s", sizes.String())
	w.linef("\tData Type: %s", dataTypeName(k.Type))
	w.linef("\tName: %q", k.Name)

	if initialized {
		w.linef("\tFirst Data Bucket VBN: %d", k.FirstData)
	}
}

// statLine is a statistic: its value ends in column 41.
func (w *rmsWriter) statLine(label string, value int, suffix string) {
	w.linef("\t%s%*d%s", label, 41-len(label), value, suffix)
}

func (a *rmsAnalysis) statistics(k *isam.Key, s *isam.KeyStats) {
	if a.opts.Mode != RMSStatistics || a.in.Stopped {
		return
	}

	w := a.w

	w.line("")
	w.heading(fmt.Sprintf("STATISTICS FOR KEY #%d", k.Ref))

	if !s.Initialized {
		w.line("\t! This index is uninitialized - there are no records.")

		return
	}

	w.statLine("Number of Index Levels:", s.Depth, "")
	w.statLine("Count of Level 1 Records:", s.Level1Records, "")
	w.statLine("Mean Length of Index Entry:", s.MeanIndexLength(), "")
	w.statLine("Count of Index Blocks:", s.IndexBlocks, "")
	w.statLine("Mean Index Bucket Fill:", s.IndexFill(), "%")
	w.statLine("Mean Index Entry Compression:", s.IndexCompression(), "%")
	w.line("")

	if k.Ref == 0 {
		w.statLine("Count of Data Records:", s.Records, "")
	} else {
		w.statLine("Count of SIDR Arrays:", s.Records, "")
		w.statLine("Mean Duplicates per Data Record:", s.MeanDuplicates(), "")
	}

	w.statLine("Mean Length of Data Record:", s.MeanDataLength(), "")
	w.statLine("Count of Data Blocks:", s.DataBlocks, "")
	w.statLine("Mean Data Bucket Fill:", s.DataFill(), "%")
	w.statLine("Mean Data Key Compression:", s.KeyCompression(), "%")

	if k.Ref == 0 {
		w.statLine("Mean Data Record Compression:", s.RecordCompression(), "%")
	}

	w.line("")

	if k.Ref == 0 {
		w.statLine("Overall Space Efficiency:", s.RecordBytes()*100/max(int(a.file.Attr.Allocated)*isam.BlockSize, 1), "%")
	}
}

// fdlLine is a secondary attribute: its value starts in column 25.
func (w *rmsWriter) fdlLine(name string, value any) {
	w.linef("\t%-24s%v", name, value)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}

	return "no"
}

// fillPercent is a key's fill in bytes as FDL's percentage of its bucket: a byte more
// than it holds, truncated. That gives VMS's 80 for C37's 1638 of 2048 and its 58 for
// G119's 300 of 512, which neither rounding nor truncation alone does.
func fillPercent(fill, blocks int) int {
	bytes := blocks * isam.BlockSize
	if bytes == 0 {
		return 0
	}

	return min((fill+1)*100/bytes, 100)
}

var fdlKeyTypeNames = map[isam.Type]string{
	isam.String: "string", isam.Int2: "int2", isam.Bin2: "bin2", isam.Int4: "int4", isam.Bin4: "bin4",
	isam.Packed: "decimal", isam.Int8: "int8", isam.Bin8: "bin8", isam.Collated: "collated",
}

func (a *rmsAnalysis) fdl(now time.Time) {
	w, attr := a.w, a.file.Attr

	w.linef("IDENT\t\"%s\tOpenVMS ANALYZE/RMS_FILE Utility\"", timeText(now, false))
	w.line("")
	w.line("SYSTEM")
	w.fdlLine("SOURCE", "OpenVMS")
	w.line("")
	w.line("FILE")
	w.fdlLine("ALLOCATION", attr.Allocated)
	w.fdlLine("BEST_TRY_CONTIGUOUS", yesNo(attr.ContiguousBestTry()))

	if attr.Organization != 0 {
		w.fdlLine("BUCKET_SIZE", attr.BucketSize)
	}

	w.fdlLine("CLUSTER_SIZE", a.file.Cluster)
	w.fdlLine("CONTIGUOUS", yesNo(attr.Contiguous()))
	w.fdlLine("EXTENSION", attr.Extend)
	w.line("\tFILE_MONITORING         no")
	w.fdlLine("GLOBAL_BUFFER_COUNT", attr.GlobalBuffers)

	if a.file.Relative() && a.file.Blocks != nil {
		w.fdlLine("MAX_RECORD_NUMBER", isam.ReadFixedProlog(a.file.Blocks).MaxRecord)
	}

	w.fdlLine("NAME", strconv.Quote(attr.Name))
	w.fdlLine("ORGANIZATION", nameOf(rmsOrganizations, int(attr.Organization)))
	w.fdlLine("OWNER", a.owner())

	p := attr.Protection
	w.fdlLine("PROTECTION", fmt.Sprintf("(system:%s, owner:%s, group:%s, world:%s)",
		protectionLetters(p), protectionLetters(p>>4), protectionLetters(p>>8), protectionLetters(p>>12)))
	w.line("")
	w.line("RECORD")
	w.fdlLine("BLOCK_SPAN", yesNo(attr.RecordAttributes&ratNoSpan == 0))

	control := "none"

	switch {
	case attr.RecordAttributes&ratFortran != 0:
		control = "fortran"
	case attr.RecordAttributes&ratCR != 0:
		control = "carriage_return"
	case attr.RecordAttributes&ratPrint != 0:
		control = "print"
	}

	w.fdlLine("CARRIAGE_CONTROL", control)

	if attr.Format == 3 {
		w.fdlLine("CONTROL_FIELD_SIZE", attr.FixedControl)
	}

	w.fdlLine("FORMAT", nameOf(fdlFormats, int(attr.Format)))
	w.fdlLine("SIZE", attr.RecordSize)

	if a.in != nil {
		a.fdlIndexed()
	}
}

func (a *rmsAnalysis) fdlIndexed() {
	w, in := a.w, a.in

	in.CheckSums()

	if in.Prolog.Version != 3 || in.Prolog.AreaVBN == 0 {
		in.CheckAreasReadable()
		a.unrecoverable()

		return
	}

	for _, area := range in.Areas {
		w.line("")
		w.linef("AREA %d", area.ID)
		w.fdlLine("ALLOCATION", area.TotalAllocation)

		if area.Options&isam.AreaBestTry != 0 {
			w.fdlLine("BEST_TRY_CONTIGUOUS", "yes")
		}

		w.fdlLine("BUCKET_SIZE", area.BucketSize)

		if area.Options&isam.AreaContiguous != 0 {
			w.fdlLine("CONTIGUOUS", "yes")
		}

		w.fdlLine("EXTENSION", area.ExtendSize)
	}

	for _, k := range in.Keys {
		a.fdlKey(k)
	}

	for _, area := range in.Areas {
		w.line("")
		w.linef("ANALYSIS_OF_AREA %d", area.ID)
		w.line("\tRECLAIMED_SPACE         0")
	}

	for _, k := range in.Keys {
		s := in.Walk(k)

		if a.unrecoverable() {
			return
		}

		w.line("")
		w.linef("ANALYSIS_OF_KEY %d", k.Ref)

		if !s.Initialized {
			w.line("\t! This index is uninitialized - there are no records.")

			continue
		}

		w.fdlLine("DATA_FILL", s.DataFill())
		w.fdlLine("DATA_KEY_COMPRESSION", s.KeyCompression())

		if k.Ref == 0 {
			w.fdlLine("DATA_RECORD_COMPRESSION", s.RecordCompression())
		}

		w.fdlLine("DATA_RECORD_COUNT", s.Records)
		w.fdlLine("DATA_SPACE_OCCUPIED", s.DataBlocks)
		w.fdlLine("DEPTH", s.Depth)

		if k.Ref != 0 {
			w.fdlLine("DUPLICATES_PER_SIDR", s.DuplicatesPerSIDR())
		}

		w.fdlLine("INDEX_COMPRESSION", s.IndexCompression())
		w.fdlLine("INDEX_FILL", s.IndexFill())
		w.fdlLine("INDEX_SPACE_OCCUPIED", s.IndexBlocks)
		w.fdlLine("LEVEL1_RECORD_COUNT", s.Level1Records)
		w.fdlLine("MEAN_DATA_LENGTH", s.MeanDataLength())
		w.fdlLine("MEAN_INDEX_LENGTH", s.MeanIndexLength())
	}
}

func (a *rmsAnalysis) fdlKey(k *isam.Key) {
	w := a.w

	w.line("")
	w.linef("KEY %d", k.Ref)
	w.fdlLine("CHANGES", yesNo(k.Flags&isam.FlagChanges != 0))
	w.fdlLine("DATA_KEY_COMPRESSION", yesNo(k.Flags&isam.FlagKeyCompress != 0))

	if k.Ref == 0 {
		w.fdlLine("DATA_RECORD_COMPRESSION", yesNo(k.Flags&isam.FlagRecordCompress != 0))
	}

	w.fdlLine("DATA_AREA", k.DataArea)
	w.fdlLine("DATA_FILL", fillPercent(k.DataFill, k.DataBucketSize))
	w.fdlLine("DUPLICATES", yesNo(k.Flags&isam.FlagDuplicates != 0))
	w.fdlLine("INDEX_AREA", k.IndexArea)
	w.fdlLine("INDEX_COMPRESSION", yesNo(k.Flags&isam.FlagIndexCompress != 0))
	w.fdlLine("INDEX_FILL", fillPercent(k.IndexFill, k.IndexBucketSize))
	w.fdlLine("LEVEL1_INDEX_AREA", k.Level1Area)
	w.fdlLine("NAME", strconv.Quote(k.Name))
	w.fdlLine("NULL_KEY", yesNo(k.Flags&isam.FlagNull != 0))

	if k.Flags&isam.FlagNull != 0 {
		w.fdlLine("NULL_VALUE", k.Null)
	}

	if k.Ref == 0 {
		w.fdlLine("PROLOG", a.in.Prolog.Version)
	}

	for i, s := range k.Segments {
		w.fdlLine(fmt.Sprintf("SEG%d_LENGTH", i), s.Size)
		w.fdlLine(fmt.Sprintf("SEG%d_POSITION", i), s.Position)
	}

	name := fdlKeyTypeNames[k.Type.Kind()]
	if k.Type.IsDescending() {
		name = "d" + name
	}

	w.fdlLine("TYPE", name)
}

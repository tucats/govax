package dclcmd

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// BACKUP/LIST (docs/PHASE-69 - backup save sets.md, subtask 7): a save
// set's summary and its files, as VMS V7.1's BACKUP V6.2 lays them out
// (testdata/backup69's log, text.lis, and b69full.lis).
//
// A listing is
//
//	Listing of save set(s)
//	(blank)
//	the summary: "Save set:          B69.BCK", "Written by:", ... (blank)
//	a line per file (/BRIEF), or a block per file and a blank (/FULL)
//	(blank)
//	Total of 25 files, 354 blocks
//	End of save set
//
// "End of save set" ends with a carriage return and a line feed of its
// own, as $FAO's !/ writes them: VMS's log and its /LIST file both show
// "End of save set", a CR, and then an empty line.
//
// What VMS V7.1 showed that the manual doesn't say: the summary leaves
// out the save set's /COMMENT, though the save set holds it (B69COM); a
// listing ignores /SELECT and /EXCLUDE on its input save set, listing
// every file; a save set truncated after a block (B69CUT) is listed to
// its end with no message, its last file at its saved size; and a block
// rebuilt from its redundancy group is reported by XORERRS after the
// files, before the total (B69HDRD). Where several groups had damage,
// VMS's placing of XORERRS is unconfirmed: govax reports them once, after
// the files, with the count. What VMS says in a listing of a block that
// can't be rebuilt is unconfirmed too; govax says nothing.

// listSaveSet is BACKUP/LIST: the listing of the input save set, on
// SYS$OUTPUT or into /LIST's file. The heading goes out before the save
// set is opened, so a save set that can't be read is reported after it,
// as VMS's OPENIN and NOTSAVESET are.
func (c *Context) listSaveSet(cmd BackupCommand) error {
	l := &backupListing{c: c, toFile: cmd.ListFile != ""}

	l.line("Listing of save set(s)")
	l.line("")

	in, err := c.ReadSaveSet(cmd.Input, backup.Options{CheckCRC: cmd.CheckCRC})
	if err != nil {
		if flushErr := l.flush(cmd.ListFile); flushErr != nil {
			return flushErr
		}

		return backupOpenFailure(err)
	}

	set := in.Set

	writeSaveSetSummary(l, set.Summary)

	var blocks uint32

	for _, f := range set.Files {
		if cmd.Full {
			writeFullListingEntry(l, f)
		} else {
			l.line(briefListingLine(f))
		}

		blocks += f.Used()
	}

	if n := len(set.Recovered); n > 0 {
		l.message(BackupLine(vmsdef.BackupXorErrs, n, in.Name))
	}

	l.line("")
	l.line(fmt.Sprintf("Total of %d file%s, %d block%s", len(set.Files), plural(len(set.Files)), blocks, plural(blocks)))
	l.line("End of save set\r\n")

	if err := l.flush(cmd.ListFile); err != nil {
		return err
	}

	// BACKUP's status for a listing that worked (testdata/backup69's
	// log: %X10000001).
	if c.SetStatus != nil {
		c.SetStatus(ssNormal | stsInhibitMsg)
	}

	return nil
}

// backupListing collects a listing's lines. With toFile, they're kept
// for /LIST's file and the messages among them go to SYS$OUTPUT as they
// come; without, everything goes to SYS$OUTPUT in order.
type backupListing struct {
	c      *Context
	toFile bool
	text   strings.Builder
}

// line adds one line to the listing.
func (l *backupListing) line(s string) {
	l.text.WriteString(s)
	l.text.WriteByte('\n')
}

// message is one of BACKUP's messages among the listing's lines.
func (l *backupListing) message(s string) {
	if l.toFile {
		l.c.Printf("%s\n", s)

		return
	}

	l.line(s)
}

// flush writes the listing: to SYS$OUTPUT, or as the text file name
// (its type LIS unless it gives one, as for DIRECTORY/OUTPUT).
func (l *backupListing) flush(name string) error {
	text := l.text.String()
	l.text.Reset()

	if !l.toFile {
		l.c.Printf("%s", text)

		return nil
	}

	return l.c.writeDirectoryListing(name, text)
}

// writeSaveSetSummary writes the summary's lines and the blank after
// them. Each label is padded to 19 columns. "Group size" is left out for
// a save set with no redundancy groups (B69G0); the comment is never
// shown (B69COM).
func writeSaveSetSummary(l *backupListing, s backup.Summary) {
	field := func(label, value string) {
		l.line(fmt.Sprintf("%-19s%s", label+":", value))
	}

	field("Save set", s.Name)
	field("Written by", s.UserName)
	field("UIC", listingUIC(s.UserUIC))
	field("Date", listingTime(s.Date, false))
	field("Command", s.Command)
	field("Operating system", operatingSystemName(s.OpSys)+" version "+s.SystemVersion)
	field("BACKUP version", s.BackupVersion)
	// A CPU ID of 0, govax's, isn't shown (VMS V7.1's listings of
	// govax's save sets, round 2).
	if s.CPUID != 0 {
		field("CPU ID register", fmt.Sprintf("%08X", s.CPUID))
	}

	field("Written on", s.Device)
	field("Block size", fmt.Sprint(s.BlockSize))

	if s.GroupSize != 0 {
		field("Group size", fmt.Sprint(s.GroupSize))
	}

	field("Buffer count", fmt.Sprint(s.Buffers))
	l.line("")
}

// operatingSystemName is the summary's operating system code as the
// listing names it. VMS V7.1 showed only its own ("OpenVMS VAX"); the
// others' names are unconfirmed.
func operatingSystemName(code uint16) string {
	switch code {
	case backup.OpSysVAX:
		return "OpenVMS VAX"
	case backup.OpSysAlpha:
		return "OpenVMS Alpha"
	case backup.OpSysI64:
		return "OpenVMS I64"
	}

	return fmt.Sprintf("operating system %%X%04X", code)
}

// briefListingLine is a file's /BRIEF line: its name, its used blocks
// ending in column 61, and its creation time to the minute
// ("[B69]ACL.TXT;1 ... 1  20-OCT-2026 11:55"). A name too long for its
// 52 columns pushes the rest along (unconfirmed: the probe's names are
// all shorter).
func briefListingLine(f *backup.File) string {
	return fmt.Sprintf("%-52s%9d  %s", f.Name, f.Used(), listingTime(f.Created, true))
}

// writeFullListingEntry writes a file's /FULL block and the blank after
// it: three columns of 22, 24, and the rest for the name, sizes, owner,
// and file ID beside the dates, then the attributes. A name longer than
// 21 characters goes on a line of its own ("[B69.SUB1]MEDIUM.DAT;1",
// 22 characters, did; a name of exactly 21 is unconfirmed).
//
// This isn't DIRECTORY/FULL's layout, and its words differ too:
// "Contiguous-best-try", "Carriage return", "Print file format", the
// version limit and the NOBACKUP and directory characteristics not shown
// at all.
func writeFullListingEntry(l *backupListing, f *backup.File) {
	name := f.Name
	if len(name) > 21 {
		l.line(name)
		name = ""
	}

	dated := func(t uint64, none string) string {
		if t == 0 {
			return none
		}

		return listingTime(t, true)
	}

	sizes := fmt.Sprintf("Size: %7d/%d", f.Used(), f.Allocated)

	l.line(fmt.Sprintf("%-22s%-24sCreated: %s", name, sizes, listingTime(f.Created, true)))
	l.line(fmt.Sprintf("%-22s%-24sRevised: %s (%d)", "", "Owner: "+listingUIC(f.Owner),
		listingTime(f.Revised, true), f.Revision))
	l.line(fmt.Sprintf("%-22s%-24sExpires: %s", "", "File ID: "+listingFileID(f.FID),
		dated(f.Expires, "<None specified>")))
	l.line(fmt.Sprintf("%-46sBackup:  %s", "", dated(f.Backup, "<No backup done>")))
	l.line("  File protection:    " + listingProtection(f.Protection))
	l.line("  File organization:  " + listingOrganization(f.Organization()))

	alloc := fmt.Sprintf("Allocation = %d, Extend = %d", f.Allocated, f.Extend())
	if f.Organization() != 0 {
		alloc += fmt.Sprintf(", maximum bucket size = %d", f.BucketSize())
	}

	l.line("  File attributes:    " + alloc)
	l.line(fmt.Sprintf("%22sGlobal Buffer Count = %d%s", "", f.GlobalBuffers(), listingCharacteristics(f.Characteristics)))
	l.line("  Record format:      " + listingRecordFormat(f.Attributes))
	l.line("  Record attributes:  " + listingRecordAttributes(f.RecordAttr()))
	l.line("")
}

// listingTime is a VMS time as the listing writes it, $ASCTIM's
// "dd-MMM-yyyy hh:mm:ss.cc" with a blank before a one-digit day
// (" 1-JAN-2030 00:00"); toMinute stops at the minute, as the file lines
// do.
func listingTime(t uint64, toMinute bool) string {
	text, ok := corevms.FormatTime(t, false)
	if !ok {
		return "<invalid time>"
	}

	if toMinute {
		return text[:17]
	}

	return text
}

// listingUIC is a UIC as the listing writes it, both halves in six octal
// digits: "[000001,000004]".
func listingUIC(uic uint32) string {
	return fmt.Sprintf("[%06o,%06o]", uic>>16, uic&0xFFFF)
}

// listingFileID is a saved file ID as the listing writes it, with its
// relative volume number: "(36,1,1)", where DIRECTORY shows the file
// "(36,1,0)". The third word holds the volume number in its low byte and
// the file number's high byte (FID$B_NMX) in its high byte.
func listingFileID(id [3]uint16) string {
	number := uint32(id[0]) | uint32(id[2]>>8)<<16

	return fmt.Sprintf("(%d,%d,%d)", number, id[1], id[2]&0xFF)
}

// listingProtection is a protection mask as the listing writes it:
// "System:RWED, Owner:RWED, Group:RE, World:". Each category has four
// bits, read, write, execute, and delete, a bit set denying.
func listingProtection(mask uint16) string {
	categories := []string{"System", "Owner", "Group", "World"}
	parts := make([]string, len(categories))

	for i, category := range categories {
		bits := mask >> (4 * i)
		access := ""

		for j, letter := range "RWED" {
			if bits&(1<<j) == 0 {
				access += string(letter)
			}
		}

		parts[i] = category + ":" + access
	}

	return strings.Join(parts, ", ")
}

// listingOrganization is the file organization's name ("Relative" is
// unconfirmed).
func listingOrganization(org uint8) string {
	switch org {
	case 1:
		return "Relative"
	case 2:
		return "Indexed"
	}

	return "Sequential"
}

// listingCharacteristics is what the Global Buffer Count line adds for a
// file's characteristics: ", Contiguous" or ", Contiguous-best-try".
// VMS V7.1 showed nothing for NOBACKUP or a directory, and the other
// characteristics' words are unconfirmed, so they show nothing.
func listingCharacteristics(fch uint32) string {
	switch {
	case fch&backup.CharContiguous != 0:
		return ", Contiguous"
	case fch&backup.CharContiguousBestTry != 0:
		return ", Contiguous-best-try"
	}

	return ""
}

// listingRecordFormat is the record format line: "Fixed length 80 byte
// records", "Variable length", "VFC, 2 byte header", "Stream_LF", with
// ", maximum 80 bytes" when the file has a record size limit (seen for
// variable-length records; the other formats' and the words for
// Stream, Stream_CR, and Undefined are unconfirmed).
func listingRecordFormat(f backup.Attributes) string {
	var text string

	switch f.RecordFormat() {
	case 1:
		return fmt.Sprintf("Fixed length %d byte records", f.RecordSize())
	case 2:
		text = "Variable length"
	case 3:
		text = fmt.Sprintf("VFC, %d byte header", f.VFCSize())
	case 4:
		text = "Stream"
	case 5:
		text = "Stream_LF"
	case 6:
		text = "Stream_CR"
	default:
		text = "Undefined"
	}

	if limit := f.MaxRecord(); limit != 0 {
		text += fmt.Sprintf(", maximum %d bytes", limit)
	}

	return text
}

// Record attribute bits (FAB$M_FTN, CR, PRN, BLK).
const (
	ratFortran   = 0x01
	ratCarriage  = 0x02
	ratPrint     = 0x04
	ratNoSpanned = 0x08
)

// listingRecordAttributes is the record attributes line: "Carriage
// return", "Print file format", "Non-spanned" (a directory), or "None".
// "Fortran carriage control" and the words joined by commas are
// unconfirmed.
func listingRecordAttributes(rat uint8) string {
	var words []string

	switch {
	case rat&ratFortran != 0:
		words = append(words, "Fortran carriage control")
	case rat&ratCarriage != 0:
		words = append(words, "Carriage return")
	case rat&ratPrint != 0:
		words = append(words, "Print file format")
	}

	if rat&ratNoSpanned != 0 {
		words = append(words, "Non-spanned")
	}

	if len(words) == 0 {
		return "None"
	}

	return strings.Join(words, ", ")
}

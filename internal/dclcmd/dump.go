package dclcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// DUMP (Phase 51): a file's contents in hexadecimal, as VMS's DUMP
// shows them, a virtual block at a time (/BLOCK, the default) or a record
// at a time (/RECORD), laid out as VMS 7.3's DUMP laid them out
// (testdata/dcl50's rounds 4 and 6), after a line holding a form feed:
//
//	Dump of file DUA1:[000000]PROBE50AV.DAT;1 on 12-OCT-2026 04:36:20.27
//	File ID (36,1,0)   End of file block 1 / Allocated 3
//
//	Virtual block number 1 (00000001), 512 (0200) bytes
//
//	 00000000 00000000 FFFF0068 67666564 8D010007 8D010002 00636261 8D010005 ....abc.........defgh........... 000000
//
// Each line is 32 bytes: as longwords in hexadecimal, the highest first
// (so each reads as a number), then the bytes as text, lowest first, a
// "." for one that isn't printable, then the line's offset. A record's
// last line has only its bytes, the hexadecimal right-aligned and the
// text padded with ".":
//
//	Record number 1 (00000001), 15 (000F) bytes, RFA(0001,0000,0000)
//
//	                                       313220 646E6120 37203A74 73726966 first: 7 and 21................. 000000
//
// A file on the host has no blocks or records to show, and a device
// isn't dumped (unconfirmed against VMS: the default range, every block
// up to the end-of-file block, and the blank line between blocks).

// dumpLineBytes is how many bytes each line shows.
const dumpLineBytes = 32

// DumpRange is /BLOCK's or /RECORD's (START=n, END=n, COUNT=n): the
// first block or record to show (from 1), and the last, 0 for the end.
type DumpRange struct {
	Start, End uint32
}

// ParseDumpRange reads /BLOCK's or /RECORD's value: "", or keywords
// START=n, END=n, and COUNT=n, separated by commas, in parentheses or
// not. COUNT=n ends the range n after its start. A keyword it doesn't
// have is CLI_IVKEYW, a value that isn't a number CLI_IVVALU.
func ParseDumpRange(value string) (DumpRange, error) {
	r := DumpRange{Start: 1}

	var count uint32

	for _, item := range strings.Split(strings.Trim(strings.TrimSpace(value), "()"), ",") {
		item = strings.ToUpper(strings.TrimSpace(item))
		if item == "" || item == "ALL" {
			continue
		}

		key, text, _ := strings.Cut(item, "=")

		n, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
		if err != nil {
			return r, vmserrors.NewSegment(vmserrors.CLI_IVVALU, text)
		}

		switch strings.TrimSpace(key) {
		case "START":
			r.Start = max(uint32(n), 1)
		case "END":
			r.End = uint32(n)
		case "COUNT":
			count = uint32(n)
		default:
			return r, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
		}
	}

	if count > 0 {
		r.End = r.Start + count - 1
	}

	return r, nil
}

// Dump shows the file spec names, a block at a time, or a record at a
// time with records, over the range span (see this file's opening
// comment).
func (c *Context) Dump(spec string, records bool, span DumpRange) error {
	loc, err := c.ContainerSession.Locate(spec, false)
	if err != nil {
		return FileFailure(err, spec)
	}

	if loc.Host {
		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, errors.New("DUMP shows only a file on a mounted volume"), spec)
	}

	file, err := c.ContainerSession.OpenDump(spec)
	if err != nil {
		var fe *rms.DCLFileError
		if errors.As(err, &fe) {
			text := "DUMP-E-OPENIN, error opening " + fe.Name + " as input\n" + ConditionLine(fe.Status)
			if fe.STV != 0 {
				text += "\n" + ConditionLine(fe.STV)
			}

			return vmserrors.ConditionError{Status: fe.Status | stsInhibitMsg, Text: text}
		}

		return FileFailure(err, spec)
	}

	now, _ := corevms.FormatTime(c.now(), false)

	c.Printf("\f\nDump of file %s on %s\n", file.Name, now)
	c.Printf("File ID %s   End of file block %d / Allocated %d\n", file.FID, file.EndBlock, file.Allocated)

	if records {
		return c.dumpRecords(file, span)
	}

	return c.dumpBlocks(file, span)
}

// now is the system time, in VMS's format.
func (c *Context) now() uint64 {
	if c.RTL != nil && c.RTL.Clock != nil {
		return c.RTL.Clock()
	}

	return 0
}

// dumpBlocks shows the file's virtual blocks in span, up to its
// end-of-file block when span has no end, each after a blank line.
func (c *Context) dumpBlocks(file *rms.DumpFile, span DumpRange) error {
	last := span.End
	if last == 0 {
		last = file.EndBlock
	}

	for vbn := span.Start; vbn <= last && vbn <= file.Allocated; vbn++ {
		block, err := file.Block(vbn)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, file.Name)
		}

		c.Printf("\nVirtual block number %d (%08X), %d (%04X) bytes\n\n", vbn, vbn, len(block), len(block))
		c.dumpBytes(block)
	}

	return nil
}

// dumpRecords shows the file's records in span, each after a blank line.
func (c *Context) dumpRecords(file *rms.DumpFile, span DumpRange) error {
	records, err := file.Records()
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, file.Name)
	}

	for i, rec := range records {
		n := uint32(i + 1)
		if n < span.Start || (span.End != 0 && n > span.End) {
			continue
		}

		c.Printf("\nRecord number %d (%08X), %d (%04X) bytes, RFA(%04X,%04X,%04X)\n\n",
			n, n, len(rec.Data), len(rec.Data), rec.VBN&0xFFFF, rec.VBN>>16, rec.Offset)
		c.dumpBytes(rec.Data)
	}

	return nil
}

// dumpBytes shows data, dumpLineBytes to a line (see this file's opening
// comment).
func (c *Context) dumpBytes(data []byte) {
	for off := 0; off < len(data); off += dumpLineBytes {
		c.Printf("%s\n", dumpLine(data[off:min(off+dumpLineBytes, len(data))], off))
	}
}

// dumpLine is one line of a dump: line's bytes, at offset off.
func dumpLine(line []byte, off int) string {
	var hex strings.Builder

	for start := (len(line) - 1) / 4 * 4; start >= 0; start -= 4 {
		hex.WriteString(" ")

		for i := min(start+4, len(line)) - 1; i >= start; i-- {
			fmt.Fprintf(&hex, "%02X", line[i])
		}
	}

	text := make([]byte, dumpLineBytes)
	for i := range text {
		text[i] = '.'

		if i < len(line) && line[i] >= ' ' && line[i] < 0x7F {
			text[i] = line[i]
		}
	}

	return fmt.Sprintf("%*s %s %06X", dumpLineBytes/4*9, hex.String(), text, off)
}

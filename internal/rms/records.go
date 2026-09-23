package rms

import (
	"io"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file implements the record-format-aware "render a file's content as
// plain text" logic docs/PHASE-23.md's subtask 8 (TYPE) calls for, written
// so subtask 9 (COPY's own container-to-host and container-to-container
// text paths, copy.go) can reuse it unchanged rather than re-deriving the
// same VFC-decoding/line-ending logic a second time -- see that subtask's
// own design note "whichever of TYPE/COPY lands first implements it, the
// other reuses it". Its behavioral
// reference is the sibling ods2 module's own cmd/ods2/internal/session/
// type.go (writeRecords/typeFile), reproduced fresh here since that
// package's own internal/ visibility rules this module out of importing it
// directly -- see docs/PHASE-23.md's "Why this phase looks different from
// most others".
//
// # A note for a reader new to VMS record formats
//
// An ODS-2 file isn't necessarily just "a stream of bytes" the way a Unix
// file is -- it can be stored as a sequence of discrete "records" (Fixed:
// every record the same fixed size; Variable: each record prefixed with its
// own length; VFC: like Variable, but each record also carries two leading
// "carriage control" bytes that say how many blank lines/form feeds to emit
// before and after it -- FORTRAN's classic "carriage control" convention)
// or, like an ordinary Unix/Windows text file, as a byte stream delimited
// by a line-ending sequence (the three Stream formats). Rendering any of
// these as plain, readable text means reconstructing line breaks the same
// way real VMS's own TYPE/COPY commands would: honor VFC's own embedded
// carriage control when present, otherwise insert an ordinary line ending
// after every record.

// lfLineEnding and crlfLineEnding are the two line-ending byte sequences
// writeRecords can be asked to use for non-VFC records -- plain '\n'
// (Session.Type's own default, and Session.Copy's own default too absent
// a future /CRLF qualifier) or '\r\n' (/CRLF, once subtask 10 adds it).
var (
	lfLineEnding   = []byte{'\n'}
	crlfLineEnding = []byte{'\r', '\n'}
)

// writeRecords writes f's entire content to w as text, honoring its record
// format: VFC records have their carriage control expanded (odsrms.
// FormatVFCRecord) and lineEnding is not used for them at all, since a VFC
// record's own trailing control byte already determines what follows it;
// every other format gets lineEnding appended after each record,
// reconstructing ordinary line-oriented text regardless of whether the
// original framing was a length prefix (Fixed/Variable) or a stream
// delimiter odsrms.Reader has already stripped off (Stream).
func writeRecords(w io.Writer, f *volume.File, lineEnding []byte) error {
	r, err := odsrms.NewReader(f)
	if err != nil {
		return err
	}

	attr := f.Header.RecordAttributes
	isVFC := attr.Format == ondisk.RecordFormatVFC
	vfcSize := int(attr.VfcSize)

	for {
		rec, err := r.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		if isVFC && len(rec) >= vfcSize {
			if _, err := w.Write(odsrms.FormatVFCRecord(rec[:vfcSize], rec[vfcSize:])); err != nil {
				return err
			}

			continue
		}

		if _, err := w.Write(rec); err != nil {
			return err
		}

		if _, err := w.Write(lineEnding); err != nil {
			return err
		}
	}
}

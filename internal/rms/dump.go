package rms

import (
	"errors"
	"fmt"
	"github.com/tucats/govax/internal/isam"
	"io"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// What DUMP shows of a file (Phase 51): its blocks and its records, read
// from the volume as they are, for the console's DUMP command
// (internal/dclcmd/dump.go) to lay out.

// DumpRecord is one of a file's records: its data (a VFC record's fixed
// control left out, as DUMP/RECORD shows it) and its record's file
// address, the virtual block holding its first byte and that byte's
// offset in the block.
type DumpRecord struct {
	Data   []byte
	VBN    uint32
	Offset uint16
}

// DumpFile is one file to dump: its full name, file ID, end-of-file
// block, and allocation, and what reads its blocks and records.
type DumpFile struct {
	Name      string
	FID       string
	EndBlock  uint32
	Allocated uint32

	file *volume.File
}

// Block reads the file's virtual block vbn (from 1), whole.
func (d *DumpFile) Block(vbn uint32) ([]byte, error) {
	buf := make([]byte, ondisk.BlockSize)

	if err := d.file.ReadBlock(vbn, buf); err != nil {
		return nil, err
	}

	return buf, nil
}

// Records reads the file's records, in order, each with its address.
func (d *DumpFile) Records() ([]DumpRecord, error) {
	if isIndexed(d.file) {
		return d.indexedRecords()
	}

	r, err := odsrms.NewReader(d.file)
	if err != nil {
		return nil, err
	}

	control := 0
	if d.file.Header.RecordAttributes.Format == ondisk.RecordFormatVFC {
		control = int(d.file.Header.RecordAttributes.VfcSize)
	}

	var records []DumpRecord

	for {
		data, err := r.Next()
		if errors.Is(err, io.EOF) {
			return records, nil
		}

		if err != nil {
			return records, err
		}

		off := r.RecordOffset()
		records = append(records, DumpRecord{
			Data:   data[min(control, len(data)):],
			VBN:    uint32(off/ondisk.BlockSize) + 1,
			Offset: uint16(off % ondisk.BlockSize),
		})
	}
}

// indexedRecords is Records for an indexed file: its records by the primary
// key, each with its RFA, its bucket's VBN and its ID in the offset's place,
// as RMS's RAB$W_RFA has them (Phase 88, subtask 9; unconfirmed until round 4's
// run).
func (d *DumpFile) indexedRecords() ([]DumpRecord, error) {
	file, err := openIndexed(d.file, false)
	if err != nil {
		return nil, err
	}

	var (
		records []DumpRecord
		failed  error
	)

	err = file.Walk(0, func(e isam.Entry) bool {
		data, err := file.Record(e.RFA)
		if err != nil {
			failed = err

			return false
		}

		records = append(records, DumpRecord{Data: data, VBN: e.RFA.VBN, Offset: e.RFA.ID})

		return true
	})
	if err == nil {
		err = failed
	}

	return records, err
}

// lastUsedBlock is the "End of file block" DUMP shows for f. An indexed
// file's header ends at the block after its allocation, byte 0, and VMS 7.1's
// DUMP/RECORD showed that as the allocation, "End of file block 27 / Allocated
// 27" for a header's 28 (testdata/idx88's round 4); its DUMP of a relative file
// so ended showed the header's 4 of 3 (testdata/for81). Which of the file's
// organization and /RECORD made the difference is unconfirmed: here it's the
// organization.
func lastUsedBlock(f *volume.File) uint32 {
	ra := f.Header.RecordAttributes
	if isIndexed(f) && ra.FirstFreeByte == 0 && ra.EndOfFileBlock > 0 {
		return ra.EndOfFileBlock - 1
	}

	return ra.EndOfFileBlock
}

// OpenDump finds the one volume file spec names, for DUMP: the newest
// version unless spec gives one. A name with wildcards is a *DCLFileError
// with RMS$_WLD, and one that finds no file is NotFound's.
func (s *Session) OpenDump(spec string) (*DumpFile, error) {
	if strings.ContainsAny(spec, "*%") || strings.Contains(spec, "...") {
		name, err := s.ExpandName(spec)
		if err != nil {
			name = strings.ToUpper(spec)
		}

		return nil, newDCLFileError(name, rmsWildcardError)
	}

	var dump *DumpFile

	err := s.firstSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil || len(matches) == 0 {
			return &NotFoundError{Spec: spec}
		}

		if len(matches) > 1 {
			return &AmbiguousError{Spec: spec, Count: len(matches), Verb: "DUMP"}
		}

		m := matches[0]

		f, err := vol.OpenFID(m.Fid)
		if err != nil {
			return err
		}

		dump = &DumpFile{
			Name:      r.shownFile(m),
			FID:       m.Fid.String(),
			EndBlock:  lastUsedBlock(f),
			Allocated: f.Header.RecordAttributes.HighestBlock,
			file:      f,
		}

		return nil
	})

	var notFound *NotFoundError

	switch {
	case dump != nil:
		return dump, nil
	case errors.As(err, &notFound):
		return nil, s.NotFound(spec)
	case err != nil:
		return nil, fmt.Errorf("dump: %w", err)
	}

	return nil, s.NotFound(spec)
}

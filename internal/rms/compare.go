package rms

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/ods2/ondisk"
)

// This file is docs/PHASE-69 - backup save sets.md's subtask 10: holding
// one file that a BACKUP save set holds (a backup.File) up against a file
// on a mounted volume or on the host, for BACKUP/COMPARE. The command
// (internal/dclcmd) decides which files to compare and what each is
// called; CompareFile compares one of them.
//
// "Because BACKUP processes files by blocks" (the System Management
// Utilities Reference Manual's /COMPARE), a compare is of virtual blocks:
// each block the save set holds against the same block of the file, whole,
// so a record appended after the save shows as a difference in the block
// it went into (VMS's "verification error for block 1 of
// DUA1:[CMP]TEXT.TXT;1"). Only the data is compared: whether VMS also
// compares the header's attributes is unconfirmed (the probe changed none
// of them).

// CompareResult is what CompareFile found.
type CompareResult struct {
	// Name is the file compared, "DUA1:[CMP]TEXT.TXT;1", or a host path.
	Name string

	// Differences are the virtual block numbers whose contents differ,
	// in order.
	Differences []uint32
}

// CompareFile compares file, saved in a save set, with the file target
// names: on a mounted volume, the one file target names (with no
// wildcards), or on the host, target's path.
//
// On a volume each block the save set holds is compared with the file's
// block of the same number; a block the file doesn't have (it's shorter
// than the saved file) is a difference. Blocks the save set lacks
// (backup.File.Missing) aren't compared. A file saved without its data
// (HeaderOnly) has nothing to compare, and matches, as VMS found
// NOBACK.TXT; so does a saved directory file, whose blocks hold file IDs
// that a restore changes (unconfirmed: the probe compared no directory).
//
// On the host the file is compared with what a restore by COPY's rule
// would write there (govax's; VMS has no host), in 512-byte pieces, a
// piece numbered as a block is.
//
// A file that isn't there is a *DCLFileError (RMS$_FNF, or RMS$_DNF for a
// missing directory), for BACKUP's OPENIN.
func (s *Session) CompareFile(target FileLocation, file *backup.File) (CompareResult, error) {
	if target.Host {
		return compareWithHost(target.Name, file)
	}

	d, err := s.OpenDump(target.Name)
	if err != nil {
		return CompareResult{}, err
	}

	result := CompareResult{Name: d.Name}
	if file.HeaderOnly() || file.Directory() {
		return result, nil
	}

	data := file.Data()
	missing := file.Missing()

	for vbn := uint32(1); vbn <= uint32(len(data)/ondisk.BlockSize); vbn++ {
		if inRanges(missing, vbn) {
			continue
		}

		start := int(vbn-1) * ondisk.BlockSize
		saved := data[start : start+ondisk.BlockSize]

		block, err := d.Block(vbn)
		if err != nil || !bytes.Equal(block, saved) {
			result.Differences = append(result.Differences, vbn)
		}
	}

	return result, nil
}

// inRanges reports whether vbn is in one of ranges.
func inRanges(ranges []backup.Range, vbn uint32) bool {
	for _, r := range ranges {
		if vbn >= r.First && vbn <= r.Last {
			return true
		}
	}

	return false
}

// compareWithHost is CompareFile for a host target, path.
func compareWithHost(path string, file *backup.File) (CompareResult, error) {
	result := CompareResult{Name: path}

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return result, newDCLFileError(path, rmsFileNotFound)
	}

	if err != nil {
		return result, fmt.Errorf("compare: %w", err)
	}

	if file.Directory() || file.HeaderOnly() {
		return result, nil
	}

	if info.IsDir() {
		return result, fmt.Errorf("compare: %s is a directory", path)
	}

	saved, err := ondisk.DecodeRecAttr(file.RecordAttributes[:])
	if err != nil {
		return result, fmt.Errorf("compare: %w", err)
	}

	want, err := hostContent(path, file, saved)
	if err != nil {
		return result, fmt.Errorf("compare: %s: %w", path, err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("compare: %w", err)
	}

	pieces := (max(len(want), len(got)) + ondisk.BlockSize - 1) / ondisk.BlockSize
	for i := range pieces {
		if !bytes.Equal(piece(want, i), piece(got, i)) {
			result.Differences = append(result.Differences, uint32(i+1))
		}
	}

	return result, nil
}

// piece is data's 512-byte piece i (from 0), short or empty at its end.
func piece(data []byte, i int) []byte {
	start := min(i*ondisk.BlockSize, len(data))
	end := min(start+ondisk.BlockSize, len(data))

	return data[start:end]
}

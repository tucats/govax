// Package backup reads and writes VMS BACKUP save sets (Phase 69; the
// writer, writer.go, is subtask 11). A save set is what `BACKUP files
// saveset/SAVE_SET` writes: a sequence of save set blocks, each a few
// thousand to 65535 bytes, that BACKUP alone was meant to interpret.
// The layout here comes from save sets real VMS (V7.1, BACKUP V6.2)
// wrote, from the public save set readers' names for its fields, and
// from the probe in testdata/backup69 (docs/PHASE-69 - backup save
// sets.md, Part A):
//
//   - Every block starts with a 256-byte block header (block.go): the
//     block's size, its kind (a data block, or a redundancy block that
//     is the XOR of its group), its number, flags, the save set's name,
//     and the file being saved when the block was written. A CRC-16 of
//     the header guards it, and a CRC-32 of the whole block guards the
//     data when the save set was written with /CRC (crc.go).
//   - Records follow the header up to the block's end (record.go), each
//     a 16-byte record header and a body: the save set summary (block 1
//     only), a file's attributes, a run of a file's virtual blocks, and
//     a null record filling what's left.
//   - The summary and a file's attributes are lists of typed values
//     (attributes.go), decoded into Summary and Attributes.
//   - Every group of data blocks (ten by default) is followed by a
//     redundancy ("XOR") block, so one damaged block in a group can be
//     rebuilt from the others (reader.go).
//
// Read takes any io.ReaderAt, so a save set may be a file on a mounted
// volume or a host file holding the same bytes. The package knows
// nothing of volumes, RMS, or DCL; times are VMS quadword times (100 ns
// units since 17-NOV-1858), as internal/rms keeps them.
package backup

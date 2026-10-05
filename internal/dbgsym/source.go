package dbgsym

import (
	"fmt"
	"sort"
)

// SourceFile is a source file a module's source correlation records
// declare (docs/DEBUG-RECORDS.md, 14.4): its full file specification,
// and the attributes the debugger checks to be sure it has the right
// version of it.
type SourceFile struct {
	ID        uint16
	Spec      string
	Created   uint64 // a VMS time
	EOFBlock  uint32
	FirstFree uint16
	Format    byte   // RMS record format and file organization
	LibModule string // the module's name, for a file in a source library
}

// sourceRange maps count listing lines from line, one to one, to the
// records of a source file from record.
type sourceRange struct {
	line, count int
	file        uint16
	record      int
}

// The source correlation commands (section 14.3).
const (
	srcDeclFile   = 1
	srcSetFile    = 2
	srcSetRecL    = 3
	srcSetRecW    = 4
	srcSetLnumL   = 5
	srcSetLnumW   = 6
	srcIncrLnumB  = 7
	srcDefLinesW  = 10
	srcDefLinesB  = 11
	srcFormFeed   = 16
	declFileFixed = 19 // a DECLFILE's bytes after its length byte, up to the file name's count
)

// sourceTable runs a module's source correlation commands (section
// 14.2) into its files and line ranges.
func sourceTable(cmds []byte) ([]SourceFile, []sourceRange, error) {
	var (
		files  []SourceFile
		ranges []sourceRange
		line   = 1
		file   uint16
		record int
		// recordOf remembers each file's record position, which
		// SETFILE returns to.
		recordOf = map[uint16]int{}
	)

	for i := 0; i < len(cmds); {
		cmd := cmds[i]
		i++

		operand := func(size int) (int, error) {
			if i+size > len(cmds) {
				return 0, fmt.Errorf("source command %d is cut short", cmd)
			}

			v := 0
			for j := size - 1; j >= 0; j-- {
				v = v<<8 | int(cmds[i+j])
			}

			i += size

			return v, nil
		}

		var (
			n   int
			err error
		)

		switch cmd {
		case srcDeclFile:
			var f SourceFile

			f, i, err = declFile(cmds, i)
			files = append(files, f)

		case srcSetFile:
			recordOf[file] = record

			n, err = operand(2)
			file, record = uint16(n), recordOf[uint16(n)]

		case srcSetRecL:
			record, err = operand(4)

		case srcSetRecW:
			record, err = operand(2)

		case srcSetLnumL:
			line, err = operand(4)

		case srcSetLnumW:
			line, err = operand(2)

		case srcIncrLnumB:
			n, err = operand(1)
			line += n

		case srcDefLinesW, srcDefLinesB:
			size := 2
			if cmd == srcDefLinesB {
				size = 1
			}

			n, err = operand(size)
			ranges = append(ranges, sourceRange{line: line, count: n, file: file, record: record})
			line += n
			record += n

		case srcFormFeed:
			// Form-feed-only records count as lines: nothing to keep,
			// since records are counted here, not read.

		default:
			return nil, nil, fmt.Errorf("source command %d is undefined", cmd)
		}

		if err != nil {
			return nil, nil, err
		}
	}

	return files, ranges, nil
}

// declFile decodes a DECLFILE command's operand at cmds[at:] (section
// 14.4): its length, flags, file ID, the file's creation time, end of
// file block, first free byte, and record format, then the counted file
// specification and the counted library module name. It returns the
// offset after it.
func declFile(cmds []byte, at int) (SourceFile, int, error) {
	if at >= len(cmds) {
		return SourceFile{}, 0, errShort
	}

	end := at + 1 + int(cmds[at])
	if end > len(cmds) || int(cmds[at]) < declFileFixed {
		return SourceFile{}, 0, errShort
	}

	b := cmds[at+1 : end]
	f := SourceFile{
		ID:        le.Uint16(b[1:]),
		Created:   le.Uint64(b[3:]),
		EOFBlock:  le.Uint32(b[11:]),
		FirstFree: le.Uint16(b[15:]),
		Format:    b[17],
	}

	spec, next, err := counted(b, 18)
	if err != nil {
		return SourceFile{}, 0, err
	}

	f.Spec = spec

	if next < len(b) {
		f.LibModule, _, err = counted(b, next)
		if err != nil {
			return SourceFile{}, 0, err
		}
	}

	return f, end, nil
}

// SourceOf returns the source file and record (from 1) that listing line
// n comes from.
func (m *Module) SourceOf(n int) (*SourceFile, int, bool) {
	i := sort.Search(len(m.sources), func(i int) bool { return m.sources[i].line+m.sources[i].count > n })
	if i == len(m.sources) || n < m.sources[i].line {
		return nil, 0, false
	}

	r := m.sources[i]

	for j := range m.Files {
		if m.Files[j].ID == r.file {
			return &m.Files[j], r.record + n - r.line, true
		}
	}

	return nil, 0, false
}

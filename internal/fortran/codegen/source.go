package codegen

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/source"
)

// sourceLines copies the FORTRAN source into the MACRO file as comments,
// a stretch at a time: before the code for a statement, every line not
// yet written up to the statement's last line, so that comments and
// declarations, which make no code, appear where they are in the source,
// and each statement's lines come just before its instructions.
type sourceLines struct {
	lines []*source.Line

	// next is the index in lines of the next line to write.
	next int

	// files is the stack of files being included: files[d] is the file
	// at include depth d, whose lines are being written.
	files []*source.File
}

// through writes the lines up to and including the last line of the
// statement s (nothing if they are written already, or s is nil).
func (s *sourceLines) through(w *Writer, stmt ast.Stmt) {
	if stmt == nil || stmt.Base().Source == nil {
		return
	}

	s.upTo(w, stmt.Base().Source.Last)
}

// rest writes the lines not yet written: those after the last
// statement that made code, the last unit's END among them.
func (s *sourceLines) rest(w *Writer) {
	s.upTo(w, len(s.lines)-1)

	for len(s.files) > 1 {
		w.Comment("end of " + s.files[len(s.files)-1].Spec)
		s.files = s.files[:len(s.files)-1]
	}
}

// upTo writes the lines from the next one to lines[last], each
// included file's lines between a comment naming the file and one
// marking its end.
func (s *sourceLines) upTo(w *Writer, last int) {
	for ; s.next <= last && s.next < len(s.lines); s.next++ {
		line := s.lines[s.next]

		for len(s.files) > line.Depth+1 || (len(s.files) == line.Depth+1 && s.files[line.Depth] != line.File) {
			w.Comment("end of " + s.files[len(s.files)-1].Spec)
			s.files = s.files[:len(s.files)-1]
		}

		if len(s.files) == line.Depth {
			if line.Depth > 0 {
				w.Comment("INCLUDE file " + line.File.Spec + ":")
			}

			s.files = append(s.files, line.File)
		}

		w.SourceLineAt(line.Depth, line.Number, line.Text)
	}
}

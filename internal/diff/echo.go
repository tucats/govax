package diff

// The command echoed at a listing's end, as VMS 7.1 writes it
// (testdata/dif116's N37, N38, N60, N61, and N64):
//
//	DIFFERENCES /IGNORE=()/MERGED=1-
//	    DUA1:[DIF116.IN]A02.A;1-
//	    DUA1:[DIF116.IN]A02.B;1
//
// "DIFFERENCES " and the qualifiers' pieces (Echo's pieces: "/IGNORE=()", or
// a list's "/IGNORE=(SPACING," and "CASE)") go on a line while each fits in
// one less than the width; one that doesn't starts a new line, the old
// ending in "-" and the new starting with four blanks. Each file starts a
// line of its own the same way. Under all of that, a line is written as
// Wrap writes one: past one less than the width, a "-" and the rest on the
// next line, from its first column. So at /WIDTH=12, VMS's narrowest, the
// echo is
//
//	DIFFERENCES-
//	 /IGNORE=()-
//	    /WIDTH=-
//	12/MERGED=1-
//	    DUA1:[D-
//	IF116.IN]A0-
//	2.A;1-
//	...

// wrapper writes text in lines no longer than limit, then "-" to say a line
// goes on. Its column is the current line's length.
type wrapper struct {
	limit int
	lines []string
	cur   []byte
}

func newWrapper(width int) *wrapper {
	return &wrapper{limit: max(width, MinWidth) - 1}
}

// write adds text, starting a line after "-" whenever the line is full and
// another character comes.
func (w *wrapper) write(text string) {
	for i := 0; i < len(text); i++ {
		if len(w.cur) == w.limit {
			w.end()
		}

		w.cur = append(w.cur, text[i])
	}
}

// end ends the current line with "-".
func (w *wrapper) end() {
	w.lines = append(w.lines, string(w.cur)+"-")
	w.cur = w.cur[:0]
}

// fits says whether piece fits on the current line.
func (w *wrapper) fits(piece string) bool { return len(w.cur)+len(piece) <= w.limit }

// finish ends the last line, without a "-", and returns them all.
func (w *wrapper) finish() []string {
	return append(w.lines, string(w.cur))
}

// Echo writes the command's echo at the width: the qualifiers' pieces in
// order, then each file, its name with any qualifiers of its own.
func Echo(l *Listing, pieces, files []string) {
	w := newWrapper(l.Width)
	w.write("DIFFERENCES ")

	for _, piece := range pieces {
		if !w.fits(piece) {
			w.end()
			w.write("    ")
		}

		w.write(piece)
	}

	for _, file := range files {
		w.end()
		w.write("    " + file)
	}

	l.Lines = append(l.Lines, w.finish()...)
}

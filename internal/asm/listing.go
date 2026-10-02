package asm

// This file records what a MACRO listing (MACRO/LIST, docs/PHASE-29.md)
// shows for each source line, as the line is assembled. A later step lays
// the recorded lines out as the listing's pages.
//
// The assembler makes one pass, and finishes a forward reference only
// once its symbol is defined (often lines later), or at the end of the
// assembly. So the listing can't be written as the source is read: a
// line's bytes aren't final yet. Instead each source line gets a
// listLine, which records where the line was assembled and which fields
// it stored, and the bytes and their marks are worked out from those
// once the assembly is over (listFields).
//
// Every line is recorded, whatever it is: lines of the program, of each
// macro expansion and repeat block (the source stack's frames, see
// source.go), blank and comment lines, the lines of a macro definition
// being collected, and lines a conditional leaves out. Which of them a
// listing shows is the listing's choice (.SHOW, .NOSHOW, .LIST, .NLIST),
// not the recording's. A macro library's definitions (sourceLibrary) are
// the one exception: they aren't the program's lines, and real MACRO
// never lists them.
//
// Recording costs nothing unless SetListing turned it on.

// listLine is one source line, as the listing needs it.
type listLine struct {
	// text is the line as written. For a line of a macro expansion it's
	// the line with the call's arguments (and string operators) already
	// substituted, which is what an expansion lists.
	text string
	// kind is the kind of source the line came from, and depth how many
	// sources are below it on the source stack: 0 for the program's own
	// lines, 1 for a macro the program called, and so on.
	kind  sourceKind
	depth int
	// line is the line's number in its own source: the program's line
	// number, for a line of the program.
	line int

	// sect and loc are where the line was assembled: the section, and
	// the location in it, when the line began. endSect and endLoc are
	// where it left the location counter.
	sect    *section
	loc     uint32
	endSect *section
	endLoc  uint32

	// stmt is the number of the statement the line assembled (see
	// Assembler.stmt), or 0 if it assembled none: a blank or comment
	// line, a line being collected, or the first lines of a continued
	// statement.
	stmt int
	// op is the statement's directive (".PSECT"), instruction ("MOVL"),
	// or macro name, or "=" for a direct assignment, or "" if it had
	// none.
	op string
	// value is the value a direct assignment gave its symbol, or ". ="
	// gave the location counter (hasValue says there is one). valueSect
	// is the section a relocatable value is an offset in, or nil.
	value     uint32
	hasValue  bool
	valueSect *section

	// fields are the fields the statement stored, in the order stored.
	fields []listField

	// errs, warnings, and messages are what the statement reported:
	// errors and warnings as the statement raised them (without the
	// source stack's location, which the listing shows by placing them
	// after the line), and .PRINT's messages.
	errs     []error
	warnings []error
	messages []string

	// collected says the line was taken into a macro definition or
	// repeat block being collected, not assembled; skipped says a
	// conditional left it out; and continued says the statement goes on
	// on the next line.
	collected bool
	skipped   bool
	continued bool
}

// listField is one field a statement stored: size bytes at offset in
// sect, as a single store (a byte, word, longword, ...) put them there.
type listField struct {
	sect   *section
	offset uint32
	size   uint32
	// group is the part of an instruction the field belongs to: 0 for
	// the opcode, n for the nth operand specifier. It's 0 for data.
	group int
	// stack says the field was stored through the linker's stack as a
	// constant (.ASCID's first longword), so the object, and the
	// listing, show value, not the bytes the image ends up with.
	stack bool
	// patch says the field was stored back over an earlier one of the
	// same statement (.ASCIC's count), with value.
	patch bool
	value uint32
}

// listMark is how the listing marks a field's value.
type listMark int

const (
	// markNone: the value is final.
	markNone listMark = iota
	// markReloc: the linker stores the field (the listing writes a '
	// after it). The value shown is what the field holds before the
	// linker adds a psect's base or an external symbol's value, or, for
	// a constant stored through the linker's stack, the constant.
	markReloc
	// markGeneral: the addressing mode byte of a general mode (G^)
	// operand, whose mode the linker chooses (the listing writes G in
	// place of the mode's high digit).
	markGeneral
)

// listBytes is a field of a recorded line as the listing shows it: its
// final bytes, in address order, and its mark.
type listBytes struct {
	offset uint32
	data   []byte
	group  int
	mark   listMark
}

// SetListing turns the recording of listing lines on or off for the
// next Assemble.
func (a *Assembler) SetListing(on bool) { a.listing = on }

// listBegin starts the record of the line raw of source f, line number
// line, and makes it the line being recorded. It returns nil (recording
// nothing) when no listing was asked for, or the line is a macro
// library's.
func (a *Assembler) listBegin(f *sourceFrame, line int, raw string) *listLine {
	if !a.listing || f.kind == sourceLibrary || a.inLibrary() {
		a.listCur = nil

		return nil
	}

	l := &listLine{
		text:  raw,
		kind:  f.kind,
		depth: len(a.sources) - 1,
		line:  line,
		sect:  a.cur,
		loc:   a.cur.loc,
	}

	a.listLines = append(a.listLines, l)
	a.listCur = l

	return l
}

// inLibrary reports whether a macro library's definition is on the
// source stack: a line run from inside one isn't the program's.
func (a *Assembler) inLibrary() bool {
	return a.count(sourceLibrary) > 0
}

// listEnd finishes the record of l, which the line has left the
// location counter after.
func (a *Assembler) listEnd(l *listLine) {
	if l == nil {
		return
	}

	l.endSect = a.cur
	l.endLoc = a.cur.loc
}

// listOp records the statement's directive, instruction, or macro name.
func (a *Assembler) listOp(name string) {
	if a.listCur != nil {
		a.listCur.op = name
	}
}

// listValue records the value a direct assignment gave, in sect (nil for
// an absolute value).
func (a *Assembler) listValue(sect *section, v uint32) {
	if a.listCur != nil {
		a.listCur.value = v
		a.listCur.valueSect = sect
		a.listCur.hasValue = true
	}
}

// listGroup sets the instruction part the fields stored next belong to
// (see listField.group).
func (a *Assembler) listGroup(group int) { a.group = group }

// listData records that the statement stored n bytes at offset in the
// current section.
func (a *Assembler) listData(offset, n uint32) {
	if a.listCur != nil && n > 0 {
		a.listCur.fields = append(a.listCur.fields, listField{sect: a.cur, offset: offset, size: n, group: a.group})
	}
}

// listConst records that the statement stored the n-byte constant value
// at offset in the current section through the linker's stack.
func (a *Assembler) listConst(offset, n, value uint32) {
	if a.listCur != nil {
		a.listCur.fields = append(a.listCur.fields, listField{sect: a.cur, offset: offset, size: n, group: a.group, stack: true, value: value})
	}
}

// listPatch records that the statement went back and stored the n-byte
// value at offset in the current section, as .ASCIC stores its count once
// the string is counted. The listing shows it as a field of its own,
// after the others.
func (a *Assembler) listPatch(offset, n, value uint32) {
	if a.listCur != nil {
		a.listCur.fields = append(a.listCur.fields, listField{sect: a.cur, offset: offset, size: n, group: a.group, patch: true, value: value})
	}
}

// listError records an error the statement raised.
func (a *Assembler) listError(err error) {
	if a.listCur != nil {
		a.listCur.errs = append(a.listCur.errs, err)
	}
}

// listWarning records a warning the statement raised.
func (a *Assembler) listWarning(err error) {
	if a.listCur != nil {
		a.listCur.warnings = append(a.listCur.warnings, err)
	}
}

// listMessage records a .PRINT message.
func (a *Assembler) listMessage(text string) {
	if a.listCur != nil {
		a.listCur.messages = append(a.listCur.messages, text)
	}
}

// listFields returns l's fields as the listing shows them, once assembly
// is over. A field's bytes are the ones l's statement stored, finished by
// any fixup since, unless a later statement stored over them (see
// overwrite.go). A field the linker finishes is one field of the
// relocation's size, holding what the linker adds to (see markReloc),
// even when the statement stored it a byte at a time.
func (a *Assembler) listFields(l *listLine) []listBytes {
	relocs := a.listRelocations(l)

	var (
		out     []listBytes
		covered = map[*section]uint32{} // the end of what's been shown, per section
	)

	for _, f := range l.fields {
		end := f.offset + f.size

		switch {
		case f.stack:
			out = append(out, listBytes{offset: f.offset, data: littleEndian(f.value, f.size), group: f.group, mark: markReloc})
			covered[f.sect] = end

			continue

		case f.patch:
			out = append(out, listBytes{offset: f.offset, data: littleEndian(f.value, f.size), group: f.group})

			continue
		}

		// A relocation shown already may cover the start of this field,
		// or all of it.
		p := f.offset
		if c, ok := covered[f.sect]; ok && c > p {
			if c >= end {
				continue
			}

			p = c
		}

		for p < end {
			if r, ok := relocs[relocKey{f.sect, p, 0}]; ok {
				n := uint32(fixupSize(r.kind))

				// A G^ operand's field starts with its mode byte, which
				// the linker chooses (relative, EF, or absolute, 9F; the
				// register digit is PC's either way).
				if r.kind == fixPICR {
					out = append(out, listBytes{offset: p, data: []byte{0x0F}, group: f.group, mark: markGeneral})
					p++
					n--
				}

				out = append(out, listBytes{offset: p, data: littleEndian(r.expr.placeholder(), n), group: f.group, mark: markReloc})
				p += n

				continue
			}

			// The bytes up to the next relocation, or the field's end.
			q := p + 1
			for q < end {
				if _, ok := relocs[relocKey{f.sect, q, 0}]; ok {
					break
				}

				q++
			}

			data := make([]byte, 0, q-p)
			for i := p; i < q; i++ {
				data = append(data, f.sect.byteStoredBy(i, l.stmt))
			}

			out = append(out, listBytes{offset: p, data: data, group: f.group, mark: markNone})
			p = q
		}

		covered[f.sect] = p
	}

	return out
}

// listRelocations returns the relocations l's statement left for the
// linker, by section and offset (relocKey's event is 0).
func (a *Assembler) listRelocations(l *listLine) map[relocKey]relocation {
	out := map[relocKey]relocation{}

	if l.stmt == 0 {
		return out
	}

	for _, r := range a.relocs {
		if r.stmt == l.stmt {
			out[relocKey{r.sect, r.offset, 0}] = r
		}
	}

	return out
}

// byteStoredBy returns the byte at offset p in s as statement stmt left
// it: the image's, which a fixup may have finished since, unless a later
// statement stored over it, and then the value stmt stored (see
// overwrite.go).
func (s *section) byteStoredBy(p uint32, stmt int) byte {
	history := s.owners[p]

	for i := len(history) - 1; i >= 0; i-- {
		if history[i].stmt == stmt {
			if i == len(history)-1 {
				break
			}

			return history[i].value
		}
	}

	return s.img.loadByte(s.base + p)
}

// littleEndian returns the low n bytes of v, low byte first, as the VAX
// stores them (an octaword's upper bytes are 0).
func littleEndian(v, n uint32) []byte {
	out := make([]byte, n)

	for i := uint32(0); i < n && i < 4; i++ {
		out[i] = byte(v >> (8 * i))
	}

	return out
}

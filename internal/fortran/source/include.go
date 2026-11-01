package source

import (
	"strings"
)

// An INCLUDE statement (section 1.4.2) has one of two forms:
//
//	INCLUDE 'file-spec[/[NO]LIST]'
//	INCLUDE '[text-library](module-name)[/[NO]LIST]'
//
// The first reads a file, by default of type FOR; the second a module of
// a text library (Phase 83). The included lines take the INCLUDE
// statement's place, and may themselves include others.
//
// /NOLIST, the default, keeps the included lines out of the MACRO file (and
// Phase 85's listing). A file's own /LIST can't bring back lines in a file
// that wasn't listed: Line.Listed needs every INCLUDE above it to say /LIST.
//
// Since FORTRAN ignores blanks, "IN CLUDE 'X'" is an INCLUDE statement
// too; and since its keywords aren't reserved, an INCLUDE is recognized
// by its whole shape, the word and then a character constant and nothing
// else. (No other statement starts with a name followed by an
// apostrophe.)

// includeSpec is what an INCLUDE statement's string says.
type includeSpec struct {
	// name is the string without /LIST or /NOLIST.
	name string

	// isModule says the second form: module is the module's name, and
	// library the text library named before it, "" for none.
	isModule        bool
	library, module string

	// list is /LIST.
	list bool
}

// readIncludeSpec reads an INCLUDE statement's string. A parenthesis at the
// name's end is a module, since a file specification can't hold one.
func readIncludeSpec(text string) includeSpec {
	spec := includeSpec{name: strings.TrimSpace(text)}

	if slash := strings.LastIndexByte(spec.name, '/'); slash >= 0 {
		switch strings.ToUpper(strings.TrimSpace(spec.name[slash+1:])) {
		case "LIST":
			spec.list = true
			spec.name = strings.TrimSpace(spec.name[:slash])
		case "NOLIST":
			spec.name = strings.TrimSpace(spec.name[:slash])
		}
	}

	if open := strings.IndexByte(spec.name, '('); open >= 0 && strings.HasSuffix(spec.name, ")") {
		spec.isModule = true
		spec.library = strings.TrimSpace(spec.name[:open])
		spec.module = strings.TrimSpace(spec.name[open+1 : len(spec.name)-1])
	}

	return spec
}

// include reports whether s is an INCLUDE statement, and if so what its
// string says. A statement that starts like one but isn't one in full is
// reported as an error and dropped: its spec's name is empty.
func (r *reader) include(s *Statement) (includeSpec, bool) {
	keyword, rest := squeezeKeyword(s.Text, "INCLUDE")
	if keyword < 0 || !strings.HasPrefix(s.Text[rest:], "'") {
		return includeSpec{}, false
	}

	var spec includeSpec

	text, end, ok := characterConstant(s.Text, rest)
	if ok {
		spec = readIncludeSpec(text)
	}

	if !ok || spec.name == "" || (spec.isModule && spec.module == "") || strings.TrimSpace(s.Text[end:]) != "" {
		r.messages.Report("BADINCLUDE", s.Position(keyword))

		return includeSpec{}, true
	}

	return spec, true
}

// includeFile reads the file or module the INCLUDE statement s names into
// the program, after s's lines. line is s's initial line, whose Listed and
// Library the included lines inherit from.
func (r *reader) includeFile(s *Statement, spec includeSpec, depth int, line *Line) {
	if spec.name == "" {
		return // a malformed INCLUDE, already reported
	}

	pos := s.Position(strings.IndexByte(s.Text, '\''))

	switch {
	case depth+1 > MaxIncludeDepth:
		r.messages.Report("INCLDEPTH", pos, MaxIncludeDepth)
		r.stopped = true

		return

	case r.opener == nil:
		r.messages.Report("INCLOPEN", pos, spec.name, "no INCLUDE files can be read here")
		r.stopped = true

		return
	}

	from := r.program.Lines[s.First].File

	var (
		file *File
		err  error
	)

	if spec.isModule {
		file, err = r.opener.OpenModule(spec.library, spec.module, from)
	} else {
		file, err = r.opener.OpenInclude(spec.name, from)
	}

	if err != nil {
		r.messages.Report("INCLOPEN", pos, spec.name, err.Error())
		r.stopped = true

		return
	}

	r.readFile(file, depth+1, line.Listed && spec.list, line.Library || spec.isModule)
}

// squeezeKeyword finds keyword at the start of text, ignoring blanks and
// case: it returns the index of the keyword's first character and the
// index just past its last, or -1 if text doesn't start with it.
func squeezeKeyword(text, keyword string) (int, int) {
	start, k := -1, 0

	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == ' ' || c == '\t' {
			continue
		}

		if k == len(keyword) {
			return start, i
		}

		if c|0x20 != keyword[k]|0x20 {
			return -1, 0
		}

		if k == 0 {
			start = i
		}

		k++
	}

	if k == len(keyword) {
		return start, len(text)
	}

	return -1, 0
}

// characterConstant reads the character constant starting with the
// apostrophe at text[start]: its value (each doubled apostrophe made
// one) and the index just past its closing apostrophe. ok is false if it
// has no closing apostrophe.
func characterConstant(text string, start int) (value string, end int, ok bool) {
	var b strings.Builder

	for i := start + 1; i < len(text); i++ {
		if text[i] != '\'' {
			b.WriteByte(text[i])

			continue
		}

		if i+1 < len(text) && text[i+1] == '\'' {
			b.WriteByte('\'')
			i++

			continue
		}

		return b.String(), i + 1, true
	}

	return "", len(text), false
}

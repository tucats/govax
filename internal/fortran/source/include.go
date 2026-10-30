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
// Since FORTRAN ignores blanks, "IN CLUDE 'X'" is an INCLUDE statement
// too; and since its keywords aren't reserved, an INCLUDE is recognized
// by its whole shape, the word and then a character constant and nothing
// else. (No other statement starts with a name followed by an
// apostrophe.)

// include reports whether s is an INCLUDE statement, and if so the name
// it gives (its string, without /LIST or /NOLIST). A statement that
// starts like one but isn't one in full is reported as an error and
// dropped.
func (r *reader) include(s *Statement) (string, bool) {
	keyword, rest := squeezeKeyword(s.Text, "INCLUDE")
	if keyword < 0 || !strings.HasPrefix(s.Text[rest:], "'") {
		return "", false
	}

	name, end, ok := characterConstant(s.Text, rest)
	if ok {
		name = trimListQualifier(name)
	}

	if !ok || name == "" || strings.TrimSpace(s.Text[end:]) != "" {
		r.messages.Report("BADINCLUDE", s.Position(keyword))

		return "", true
	}

	return name, true
}

// includeFile reads the file the INCLUDE statement s names into the
// program, after s's lines.
func (r *reader) includeFile(s *Statement, name string, depth int) {
	if name == "" {
		return // a malformed INCLUDE, already reported
	}

	pos := s.Position(strings.IndexByte(s.Text, '\''))

	switch {
	case strings.HasSuffix(name, ")") && strings.Contains(name, "("):
		r.messages.Report("NOTIMPL", pos, "INCLUDE of a text library module")

		return

	case depth+1 > MaxIncludeDepth:
		r.messages.Report("INCLDEPTH", pos, MaxIncludeDepth)
		r.stopped = true

		return

	case r.opener == nil:
		r.messages.Report("INCLOPEN", pos, name, "no INCLUDE files can be read here")
		r.stopped = true

		return
	}

	from := r.program.Lines[s.First].File

	file, err := r.opener.OpenInclude(name, from)
	if err != nil {
		r.messages.Report("INCLOPEN", pos, name, err.Error())
		r.stopped = true

		return
	}

	r.readFile(file, depth+1)
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

// trimListQualifier removes a trailing /LIST or /NOLIST (any case, with
// blanks around it) from an INCLUDE statement's name. The listing it
// controls is Phase 85's.
func trimListQualifier(name string) string {
	name = strings.TrimSpace(name)

	if slash := strings.LastIndexByte(name, '/'); slash >= 0 {
		switch strings.ToUpper(strings.TrimSpace(name[slash+1:])) {
		case "LIST", "NOLIST":
			name = strings.TrimSpace(name[:slash])
		}
	}

	return name
}

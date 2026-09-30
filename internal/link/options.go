package link

import (
	"fmt"
	"strconv"
	"strings"
)

// This file reads a LINK options file (the Linker manual, chapter 10: an
// input file with /OPTIONS). Each line is either input files, separated by
// commas, each with its own qualifiers:
//
//	file[/LIBRARY][/INCLUDE=(module[,...])][/SELECTIVE_SEARCH][/SHAREABLE]
//
// or an option, keyword=value:
//
//	STACK=pages
//	IDENTIFICATION=ident
//	NAME=image-name
//	SYMBOL=name,value
//
// An exclamation point starts a comment, and a hyphen at the end of a line
// continues it on the next. Keywords and qualifiers may be abbreviated to
// any unique prefix, and numbers may be decimal or have a radix prefix
// (%X, %D, %O).

// InputFile is one input file of a link, as the command line or an options
// file names it, with its qualifiers.
type InputFile struct {
	Name string
	// Library is /LIBRARY: a library searched for symbols the modules
	// refer to but don't define, before the system libraries.
	Library bool
	// Include is /INCLUDE=(...): modules of a library added to the link
	// whether or not anything refers to them.
	Include []string
	// Selective is /SELECTIVE_SEARCH: an object file whose definitions are
	// taken only for symbols already referred to.
	Selective bool
	// Shareable is /SHAREABLE (options files only): a shareable image
	// whose symbols the link uses.
	Shareable bool
	// Options is /OPTIONS (the command line only): an options file.
	Options bool
}

// OptionsFile is what an options file says.
type OptionsFile struct {
	// Files are its input files, in order.
	Files []InputFile
	// Stack is STACK=, the user stack's pages; 0 if not given.
	Stack int
	// Ident is IDENTIFICATION=, and Name is NAME=; "" if not given.
	Ident, Name string
	// Symbols are SYMBOL= definitions, in order.
	Symbols []Symbol
}

// Symbol is a global symbol an options file defines (SYMBOL=), which takes
// precedence over any definition of it in the modules.
type Symbol struct {
	Name  string
	Value uint32
}

// linkOptions are the options an options file may give, each a function
// that records its value.
var linkOptions = map[string]func(o *OptionsFile, value string) error{
	"STACK": func(o *OptionsFile, value string) error {
		n, err := optionNumber(value)
		if err != nil || n == 0 || n > 1<<21 {
			return fmt.Errorf("STACK=%s: a number of pages", value)
		}

		o.Stack = int(n)

		return nil
	},
	"IDENTIFICATION": func(o *OptionsFile, value string) error {
		o.Ident = unquote(value)

		if len(o.Ident) > 15 {
			return fmt.Errorf("IDENTIFICATION=%s: at most 15 characters", value)
		}

		return nil
	},
	"NAME": func(o *OptionsFile, value string) error {
		o.Name = unquote(value)

		if o.Name == "" || len(o.Name) > 39 {
			return fmt.Errorf("NAME=%s: 1 to 39 characters", value)
		}

		return nil
	},
	"SYMBOL": func(o *OptionsFile, value string) error {
		name, number, ok := strings.Cut(value, ",")
		name = strings.ToUpper(strings.TrimSpace(name))

		n, err := optionNumber(number)
		if !ok || name == "" || len(name) > 31 || err != nil {
			return fmt.Errorf("SYMBOL=%s: a name and a value", value)
		}

		o.Symbols = append(o.Symbols, Symbol{Name: name, Value: n})

		return nil
	},
}

// inputQualifiers are the qualifiers an input file may have in an options
// file, each a function that records it.
var inputQualifiers = map[string]func(f *InputFile, value string, hasValue bool) error{
	"LIBRARY": func(f *InputFile, _ string, hasValue bool) error {
		f.Library = true

		return noValue("LIBRARY", hasValue)
	},
	"INCLUDE": func(f *InputFile, value string, hasValue bool) error {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
		for _, m := range splitOutside(value, ',') {
			if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
				f.Include = append(f.Include, m)
			}
		}

		if !hasValue || len(f.Include) == 0 {
			return fmt.Errorf("/INCLUDE needs module names")
		}

		return nil
	},
	"SELECTIVE_SEARCH": func(f *InputFile, _ string, hasValue bool) error {
		f.Selective = true

		return noValue("SELECTIVE_SEARCH", hasValue)
	},
	"SHAREABLE": func(f *InputFile, value string, hasValue bool) error {
		f.Shareable = true

		// /SHAREABLE=COPY copies the image into the link, which isn't
		// supported; NOCOPY is the default.
		if hasValue && !strings.HasPrefix("NOCOPY", strings.ToUpper(value)) {
			return fmt.Errorf("/SHAREABLE=%s isn't supported", value)
		}

		return nil
	},
}

func noValue(name string, hasValue bool) error {
	if hasValue {
		return fmt.Errorf("/%s takes no value", name)
	}

	return nil
}

// ParseOptions reads an options file's lines.
func ParseOptions(lines []string) (*OptionsFile, error) {
	o := &OptionsFile{}

	var statement strings.Builder

	for n, line := range lines {
		line = strings.TrimSpace(stripComment(line))

		if cont, ok := strings.CutSuffix(line, "-"); ok {
			statement.WriteString(cont)

			continue
		}

		statement.WriteString(line)
		text := strings.TrimSpace(statement.String())
		statement.Reset()

		if text == "" {
			continue
		}

		if err := o.statement(text); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}

	if rest := strings.TrimSpace(statement.String()); rest != "" {
		if err := o.statement(rest); err != nil {
			return nil, fmt.Errorf("the last line: %w", err)
		}
	}

	return o, nil
}

// statement reads one option or line of input files.
func (o *OptionsFile) statement(text string) error {
	keyword, value, isOption := strings.Cut(text, "=")
	if isOption && !strings.ContainsAny(keyword, "/:[]<>.;\"") {
		name, err := matchKeyword(strings.ToUpper(strings.TrimSpace(keyword)), linkOptions)
		if err != nil {
			return fmt.Errorf("option %s: %w", strings.TrimSpace(keyword), err)
		}

		return linkOptions[name](o, strings.TrimSpace(value))
	}

	for _, spec := range splitOutside(text, ',') {
		f, err := parseInputFile(spec)
		if err != nil {
			return err
		}

		o.Files = append(o.Files, f)
	}

	return nil
}

// parseInputFile reads one input file and its qualifiers.
func parseInputFile(spec string) (InputFile, error) {
	parts := splitOutside(spec, '/')

	f := InputFile{Name: unquote(strings.TrimSpace(parts[0]))}
	if f.Name == "" {
		return f, fmt.Errorf("%q names no file", spec)
	}

	for _, q := range parts[1:] {
		name, value, hasValue := strings.Cut(q, "=")

		key, err := matchKeyword(strings.ToUpper(strings.TrimSpace(name)), inputQualifiers)
		if err != nil {
			return f, fmt.Errorf("%s: /%s: %w", f.Name, strings.TrimSpace(name), err)
		}

		if err := inputQualifiers[key](&f, strings.TrimSpace(value), hasValue); err != nil {
			return f, fmt.Errorf("%s: %w", f.Name, err)
		}
	}

	return f, nil
}

// matchKeyword finds the keyword word abbreviates, uniquely.
func matchKeyword[V any](word string, keywords map[string]V) (string, error) {
	if _, ok := keywords[word]; ok {
		return word, nil
	}

	var found []string

	for k := range keywords {
		if word != "" && strings.HasPrefix(k, word) {
			found = append(found, k)
		}
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("isn't supported")
	}

	return "", fmt.Errorf("is ambiguous")
}

// optionNumber reads a number: decimal, or with a radix prefix.
func optionNumber(s string) (uint32, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	base := 10

	switch {
	case strings.HasPrefix(s, "%X"):
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "%O"):
		base, s = 8, s[2:]
	case strings.HasPrefix(s, "%D"):
		s = s[2:]
	}

	n, err := strconv.ParseUint(s, base, 32)

	return uint32(n), err
}

// stripComment removes a comment: from an exclamation point that isn't in
// quotes.
func stripComment(line string) string {
	quoted := false

	for i, c := range line {
		switch {
		case c == '"':
			quoted = !quoted
		case c == '!' && !quoted:
			return line[:i]
		}
	}

	return line
}

// splitOutside splits s at each sep that isn't in quotes or parentheses.
func splitOutside(s string, sep rune) []string {
	var (
		out    []string
		depth  int
		quoted bool
		start  int
	)

	for i, c := range s {
		switch {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}

	return append(out, s[start:])
}

// unquote removes the quotes around a quoted string.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	return s
}

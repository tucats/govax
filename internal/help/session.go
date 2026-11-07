package help

import (
	"errors"
	"io"
	"strings"
)

// Session is HELP's interactive side: it shows what a request names, then
// prompts for more and answers each response, as VMS 7.1's HELP does
// (testdata/help90's part D, round 1c, and round 2's terminal session). It
// knows nothing of terminals or files: its caller gives it a way to write
// a line and a way to read one, and does what the terminal does (echoing,
// clearing the screen, Ctrl/Z's "Exit").
//
// The prompts are "Topic?" at the library's level-1 topics and "KEY KEY
// Subtopic?" below them, naming the whole path ("DEEPER L2 L3
// Subtopic?"), each starting with "@NAME " in a library a response named
// ("@H90B BONLY Subtopic?"). A response is:
//
//   - keys, looked up below the topic the prompt names (at "Topic?", as a
//     new request, which may start with "@library");
//   - "?", which shows the prompt's topic again (at "Topic?", HELP's own
//     topic and the library's topics, as HELP with no keys shows them);
//   - an empty line, which goes up one level: from "Topic?" in a library
//     a response named back to the main library's "Topic?", and from the
//     main library's "Topic?" out of HELP;
//   - the end of input (Ctrl/Z, or the end of a file of answers), which
//     ends HELP at once.
//
// After a topic with subtopics HELP prompts for them. After one with
// none, a qualifier line, or several topics, the prompt stays where it
// was ("Topic?" after the first request; see next). A response that finds nothing shows "Sorry" and leaves
// the prompt where it was; a first request that finds nothing prompts at
// "Topic?" (the debugger's "HELP EST", which looks up "DEBUG EST",
// prompted "Topic?").
type Session struct {
	// Library is the main library, whose level-1 prompt is "Topic?".
	Library Library
	// Open opens the library "@spec" names, returning it and the name
	// its prompts show (H90B for "@H90DIR:H90B"). If it's nil, or it
	// fails, the request is one that found nothing.
	Open func(spec string) (Library, string, error)
	// Write writes one line of output.
	Write func(line string)
	// Read shows a prompt and reads a response; io.EOF ends HELP.
	Read func(prompt string) (string, error)
	// BeforeDisplay, if set, is called before each response's output (a
	// terminal's /PAGE clears the screen there).
	BeforeDisplay func()
	// Width is the output's width (80 if zero).
	Width int
	// Options are the requests' settings.
	Options Options
	// Prompt is HELP/PROMPT: without it HELP answers the first request
	// and ends.
	Prompt bool
	// Users are the user libraries (HLP$LIBRARY, HLP$LIBRARY_1, ... in
	// the process, group, and system tables), in the order they're
	// searched. A first word that names no level-1 topic in the library
	// HELP is in is looked for in the main library and then in each of
	// these, and "@name" may name one by the start of its name.
	Users []NamedLibrary
	// LibList is HELP/LIBLIST (the default): the user libraries' names
	// are listed after a list of a library's topics.
	LibList bool
}

// NamedLibrary is a help library and the name HELP shows for it: its
// file's name, without device, directory, or type ("H90B").
type NamedLibrary struct {
	Name    string
	Library Library
}

// place is where a Session's prompt is: a library, and a path of topics
// in it (none at "Topic?"). home is the library "Topic?" is in, the root:
// the main library, or one "@name" moved to. lib is the library the
// path's topics are in, which is another when the topic was found by
// searching the other libraries.
type place struct {
	home  Library
	lib   Library
	name  string // the root's name for "@NAME", "" for the main library
	steps []step
}

// at is the place of "Topic?" in the library lib, named name.
func at(lib Library, name string) place {
	return place{home: lib, lib: lib, name: name}
}

// prompt is the place's prompt.
func (p place) prompt() string {
	var b strings.Builder

	if p.name != "" {
		b.WriteString("@" + p.name + " ")
	}

	if len(p.steps) == 0 {
		b.WriteString("Topic? ")

		return b.String()
	}

	for _, st := range p.steps {
		b.WriteString(st.heading + " ")
	}

	b.WriteString("Subtopic? ")

	return b.String()
}

// Run answers a request's words, then, with Prompt, prompts until HELP
// ends. It returns an error only if Read fails other than with io.EOF.
func (s *Session) Run(words []string) error {
	p := s.request(at(s.Library, ""), words)

	if !s.Prompt {
		return nil
	}

	for {
		answer, err := s.Read(p.prompt())
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		words := Split(answer)

		switch {
		case len(words) == 0:
			switch {
			case len(p.steps) > 0:
				p.steps = p.steps[:len(p.steps)-1]
				if len(p.steps) == 0 {
					p.lib = p.home
				}
			case p.name != "":
				// Unconfirmed after a second "@": the probe went into
				// only one other library.
				p = at(s.Library, "")
			default:
				return nil
			}
		case len(words) == 1 && words[0] == "?":
			s.again(p)
		case len(p.steps) == 0:
			p = s.request(p, words)
		default:
			p = s.below(p, words)
		}
	}
}

// request answers words at "Topic?" (or as HELP's first request) in p's
// library, and returns where to prompt next. A first word "@spec" moves
// to that library; the rest of the words are looked up there. A library
// "@spec" can't name is a request that found nothing: VMS 7.1's "HELP
// @NOSUCHLIB ALPHA" was "Sorry, no documentation on @NOSUCHLIB ALPHA",
// with the main library's topics (testdata/help90's part C).
func (s *Session) request(p place, words []string) place {
	if len(words) > 0 && strings.HasPrefix(words[0], "@") {
		lib, name, ok := s.atLibrary(words[0][1:])
		if !ok {
			s.show(s.render([]Display{{
				Kind: ShowSorry, Sorry: strings.ToUpper(strings.Join(words, " ")),
				List: rootList(p.home.Names()), Listed: true, ListDepth: 1, rooted: true,
			}}))

			return at(p.home, p.name)
		}

		p = at(lib, name)
		words = words[1:]
	}

	if len(words) == 0 {
		s.show(s.render(instructions(p.home, p.home.Names(), s.Options)))

		return at(p.home, p.name)
	}

	displays, err := lookupFrom(p.home, nil, words)
	if err != nil {
		// Unconfirmed where HELP prompts after this message; the probe
		// asked only with /NOPROMPT.
		s.show([]string{"%LBR-E-INVKEY, invalid key"})

		return at(p.home, p.name)
	}

	found := p.home

	// Nothing at level 1: the main library and the user libraries are
	// searched in turn, and the first in which the first word names a
	// topic answers (testdata/help90's part C: "HU BONLY" found H90B's
	// BONLY, "HU CONLY" H90C's). If none does, the root's "Sorry" stands.
	if missed(displays) {
		for _, lib := range s.searchOrder(p.home) {
			if d, err := lookupFrom(lib, nil, words); err == nil && !missed(d) {
				displays, found = d, lib

				break
			}
		}
	}

	s.show(s.render(displays))

	p = at(p.home, p.name)
	p.steps = next(displays, nil)

	if len(p.steps) > 0 {
		p.lib = found
	}

	return p
}

// missed reports whether displays are a request's answer when its first
// word named nothing at level 1.
func missed(displays []Display) bool {
	return len(displays) == 1 && displays[0].levelOneMiss
}

// searchOrder is the libraries searched after home when a first word
// names nothing there: the main library, then the user libraries, each
// once (the manual: root, main, process, group, system).
func (s *Session) searchOrder(home Library) []Library {
	var order []Library

	seen := map[Library]bool{home: true}

	for _, lib := range append([]Library{s.Library}, s.libraries()...) {
		if lib != nil && !seen[lib] {
			seen[lib] = true
			order = append(order, lib)
		}
	}

	return order
}

// libraries are the user libraries' Libraries.
func (s *Session) libraries() []Library {
	libs := make([]Library, len(s.Users))
	for i, u := range s.Users {
		libs[i] = u.Library
	}

	return libs
}

// atLibrary is the library "@spec" names: the first user library whose
// name starts with spec (VMS 7.1's "@H90" chose H90B, the first of H90B
// and H90C), or the library Open finds.
func (s *Session) atLibrary(spec string) (Library, string, bool) {
	upper := strings.ToUpper(spec)

	if upper != "" && !strings.ContainsAny(upper, ":[<") {
		for _, u := range s.Users {
			if strings.HasPrefix(strings.ToUpper(u.Name), upper) {
				return u.Library, u.Name, true
			}
		}
	}

	if s.Open == nil {
		return nil, "", false
	}

	lib, name, err := s.Open(spec)
	if err != nil || lib == nil {
		return nil, "", false
	}

	return lib, name, true
}

// render lays displays out at the session's width, with the user
// libraries' names after each list of a library's topics when LibList is
// set:
//
//	  Additional help libraries available (type @name for topics):
//
//	  H90B  H90C
//
// (testdata/help90's part C, after the topics of "HU NOSUCH", "HU @H90B",
// and "HU/LIBLIST"). Whether the names wrap at the width is unconfirmed:
// the probe had no more than four.
func (s *Session) render(displays []Display) []string {
	if s.LibList && len(s.Users) > 0 {
		names := make([]string, len(s.Users))
		for i, u := range s.Users {
			names[i] = u.Name
		}

		for i := range displays {
			if displays[i].rooted {
				displays[i].Libraries = names
			}
		}
	}

	return Render(displays, s.width())
}

// below answers words at a "Subtopic?" prompt: they're looked up below
// the prompt's topic.
func (s *Session) below(p place, words []string) place {
	displays, _ := lookupFrom(p.lib, p.steps, words)
	s.show(s.render(displays))

	p.steps = next(displays, p.steps)

	return p
}

// again shows the prompt's topic again, for "?".
func (s *Session) again(p place) {
	if len(p.steps) == 0 {
		s.show(s.render(instructions(p.home, p.home.Names(), s.Options)))

		return
	}

	headings := make([]string, len(p.steps))
	nodes := make([]*Topic, len(p.steps))

	for i, st := range p.steps {
		headings[i], nodes[i] = st.heading, st.topic
	}

	s.show(s.render([]Display{topicDisplay(headings, nodes)}))
}

// next is where HELP prompts after showing displays, from at: the topic
// shown, if one topic was shown and it has numbered subtopics; otherwise
// where the prompt was. After a first request (at is nil) that means
// "Topic?": VMS 7.1's debugger, asked for a leaf below DEBUG
// (Probe_Concept), for a qualifier line (EXAMINE /ASCII), for a topic's
// Qualifiers, or for several topics (EX, *), prompted "Topic?" at once
// (testdata/help90's dbgh.dlg, round 1b). At a "Subtopic?" prompt a leaf
// leaves the prompt at its topic (round 2's S5 at "DEEPER L2 L3 L4
// Subtopic?"), and so did "*" below ALPHA. A response of several words
// reaching a leaf further down keeps the prompt where it was too
// (unconfirmed: the probe's responses were single words). If nothing was
// found, the prompt stays at at.
func next(displays []Display, at []step) []step {
	var shown []*Display

	for i := range displays {
		if d := &displays[i]; d.path != nil {
			shown = append(shown, d)
		}
	}

	if len(shown) != 1 {
		return at
	}

	path := shown[0].path

	if t := path[len(path)-1]; t.Slash || len(t.Subtopics) == 0 {
		return at
	}

	steps := make([]step, len(path))
	for i, t := range path {
		steps[i] = step{topic: t, heading: shown[0].Headings[i]}
	}

	return steps
}

// show writes one response's output.
func (s *Session) show(lines []string) {
	if s.BeforeDisplay != nil {
		s.BeforeDisplay()
	}

	for _, line := range lines {
		s.Write(line)
	}
}

func (s *Session) width() int {
	if s.Width <= 0 {
		return 80
	}

	return s.Width
}

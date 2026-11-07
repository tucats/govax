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
// none, or a qualifier line's, the prompt stays where it was: at the
// topic's parent. A response that finds nothing shows "Sorry" and leaves
// the prompt where it was; a first request that finds nothing prompts at
// "Topic?" (the debugger's "HELP EST", which looks up "DEBUG EST",
// prompted "Topic?").
type Session struct {
	// Library is the main library, whose level-1 prompt is "Topic?".
	Library Library
	// Open opens the library "@spec" names, returning it and the name
	// its prompts show (H90B for "@H90DIR:H90B"). If it's nil, or it
	// fails, the error's text is written and the prompt stays.
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
}

// place is where a Session's prompt is: a library, and a path of topics
// in it (none at "Topic?").
type place struct {
	lib   Library
	name  string // the library's name for "@NAME", "" for the main library
	steps []step
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
	p := s.request(place{lib: s.Library}, words)

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
			case p.name != "":
				// Unconfirmed after a second "@": the probe went into
				// only one other library.
				p = place{lib: s.Library}
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
// to that library; the rest of the words are looked up there.
func (s *Session) request(p place, words []string) place {
	if len(words) > 0 && strings.HasPrefix(words[0], "@") {
		lib, name, err := s.open(words[0][1:])
		if err != nil {
			s.show(strings.Split(err.Error(), "\n"))

			return p
		}

		p = place{lib: lib, name: name}
		words = words[1:]
	}

	if len(words) == 0 {
		s.show(Render(instructions(p.lib, p.lib.Names(), s.Options), s.width()))

		return place{lib: p.lib, name: p.name}
	}

	displays, err := lookupFrom(p.lib, nil, words)
	if err != nil {
		// Unconfirmed where HELP prompts after this message; the probe
		// asked only with /NOPROMPT.
		s.show([]string{"%LBR-E-INVKEY, invalid key"})

		return place{lib: p.lib, name: p.name}
	}

	s.show(Render(displays, s.width()))

	p.steps = next(displays, nil)

	return p
}

// below answers words at a "Subtopic?" prompt: they're looked up below
// the prompt's topic.
func (s *Session) below(p place, words []string) place {
	displays, _ := lookupFrom(p.lib, p.steps, words)
	s.show(Render(displays, s.width()))

	p.steps = next(displays, p.steps)

	return p
}

// again shows the prompt's topic again, for "?".
func (s *Session) again(p place) {
	if len(p.steps) == 0 {
		s.show(Render(instructions(p.lib, p.lib.Names(), s.Options), s.width()))

		return
	}

	headings := make([]string, len(p.steps))
	nodes := make([]*Topic, len(p.steps))

	for i, st := range p.steps {
		headings[i], nodes[i] = st.heading, st.topic
	}

	s.show(Render([]Display{topicDisplay(headings, nodes)}, s.width()))
}

// next is where HELP prompts after showing displays, from at: the topic
// shown, if it has subtopics; its parent if it has none; and if several
// topics were shown, the topics their paths share (unconfirmed: the probe
// showed only "*" below a topic, after which the prompt stayed at that
// topic). If nothing was found, the prompt stays at at.
func next(displays []Display, at []step) []step {
	var (
		shown    []*Display
		headings []string
	)

	for i := range displays {
		if d := &displays[i]; d.path != nil {
			shown = append(shown, d)
		}
	}

	if len(shown) == 0 {
		return at
	}

	path := shown[0].path
	headings = shown[0].Headings

	if len(shown) == 1 {
		t := path[len(path)-1]
		if t.Slash || len(t.Subtopics) == 0 {
			path = path[:len(path)-1]
		}
	} else {
		for _, d := range shown[1:] {
			n := 0
			for n < len(path) && n < len(d.path) && path[n] == d.path[n] {
				n++
			}

			path = path[:n]
		}
	}

	steps := make([]step, len(path))
	for i, t := range path {
		steps[i] = step{topic: t, heading: headings[i]}
	}

	return steps
}

// open opens the library "@spec" names.
func (s *Session) open(spec string) (Library, string, error) {
	if s.Open == nil {
		return nil, "", errors.New("Sorry, no documentation on @" + strings.ToUpper(spec))
	}

	return s.Open(spec)
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

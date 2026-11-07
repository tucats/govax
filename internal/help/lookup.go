package help

import (
	"errors"
	"strings"
)

// ErrInvalidKey is a request whose first word VMS's librarian can't look
// up: "HELP ALPHA,Parameters" is %LBR-E-INVKEY (testdata/help90, round
// 1b). A comma in a later word is part of a key ("ODD Key,_With_Comma"
// works). Which other characters LBR$LOOKUP_KEY refuses is unconfirmed.
var ErrInvalidKey = errors.New("invalid key")

// DisplayKind says what a Display shows.
type DisplayKind int

const (
	// ShowTopic is a numbered topic: its headings, its text, and, unless
	// HELP is showing a topic and everything below it, its subtopics.
	ShowTopic DisplayKind = iota
	// ShowSlash is a qualifier line's topic: its headings, then the line
	// and its text.
	ShowSlash
	// ShowSorry is "Sorry, no documentation on ...", after the headings
	// of the topics the request found, and the list of the last one's
	// subtopics.
	ShowSorry
	// ShowList is "Information available:" and the library's topics
	// (HELP/NOINSTRUCTIONS with no request).
	ShowList
)

// Display is one part of HELP's answer to a request: a topic shown, or
// the message that one isn't there.
type Display struct {
	Kind DisplayKind
	// Headings are the keys of the topics above and including the one
	// shown, one per line, each indented 2 more than the one before: the
	// path to the topic, as the request found it.
	Headings []string
	// Text is the topic's lines (ShowTopic, ShowSlash: a qualifier
	// line's topic starts with its own line).
	Text []string
	// Depth is how far right Text is shifted, in steps of 2 blanks: a
	// numbered topic's level in the request's path, or one less for a
	// qualifier line's.
	Depth int
	// Sorry is the request's words, upper-cased, for ShowSorry.
	Sorry string
	// List is the topics listed after the text (Listed tells an empty
	// list from none), and ListDepth their indentation, in steps of 2.
	List      []Entry
	Listed    bool
	ListDepth int

	// Libraries are the user libraries' names, listed after List when
	// HELP/LIBLIST is in effect and List is the library's topics.
	Libraries []string

	// rooted says List is the library's level-1 topics, after which
	// Session lists the user libraries; levelOneMiss that the request's
	// first word named no level-1 topic, so Session searches the other
	// libraries.
	rooted       bool
	levelOneMiss bool

	// path is the topics from level 1 down to the one shown (ShowTopic,
	// ShowSlash), which Session uses to decide where to prompt next. A
	// qualifier line's path ends with its own topic.
	path []*Topic
}

// Options are a request's settings.
type Options struct {
	// Instructions is HELP/INSTRUCTIONS (the default): a request with
	// no words shows the library's HELP topic before its list of
	// topics. Without it, the list is all that's shown.
	Instructions bool
}

// Lookup finds the topics a request's words name in lib, the root
// library, and returns what HELP shows for them.
//
// The first word names level-1 topics (the library's modules): a module
// whose name is the word, if there is one, or else every module whose name
// it abbreviates or, with wildcards, matches. Each later word names
// subtopics of the topics found so far, every one it abbreviates or
// matches: below level 1 an exact key doesn't win over a longer one
// ("SAME Ex" and "SAME Example" both show Example and Examples). "..." at
// the end of the first word shows each topic found and every topic below
// it; on any later word it's part of the word.
func Lookup(lib Library, words []string, opts Options) ([]Display, error) {
	if len(words) == 0 {
		return instructions(lib, lib.Names(), opts), nil
	}

	return lookupFrom(lib, nil, words)
}

// step is one topic of the path a Session is at: the topic, and its
// heading as HELP showed it.
type step struct {
	topic   *Topic
	heading string
}

// lookupFrom finds words below the end of at, a path from level 1, or,
// with at empty, from the library's modules. Below level 1 "..." is part
// of a word, and the "Sorry" line names at's headings before the words
// (testdata/help90's part D: "..." at "ALPHA Subtopic?" is "Sorry, no
// documentation on ALPHA ...").
func lookupFrom(lib Library, at []step, words []string) ([]Display, error) {
	names := lib.Names()

	words = append([]string(nil), words...)

	if len(at) > 0 {
		headings := make([]string, len(at))
		nodes := make([]*Topic, len(at))

		for i, st := range at {
			headings[i], nodes[i] = st.heading, st.topic
		}

		s := &search{
			lib:          lib,
			names:        names,
			words:        words,
			sorry:        strings.ToUpper(strings.Join(append(append([]string(nil), headings...), words...), " ")),
			current:      headings,
			currentNodes: nodes,
		}

		s.descend(nodes[len(nodes)-1], headings, nodes, words)

		if len(s.out) == 0 && s.failed != nil {
			return []Display{*s.failed}, nil
		}

		return s.out, nil
	}

	if strings.Contains(words[0], ",") {
		return nil, ErrInvalidKey
	}

	all := false
	if w := words[0]; len(w) > 3 && strings.HasSuffix(w, "...") {
		words[0], all = w[:len(w)-3], true
	}

	for i := maxWords; i < len(words); i++ {
		words[i] = words[i-maxWords]
	}

	s := &search{
		lib:   lib,
		names: names,
		words: words,
		all:   all,
		sorry: strings.ToUpper(strings.Join(words, " ")),
	}

	// The first word: an exact name wins, and is shown as the word was
	// given, upper-cased (MixedCase is shown as MIXEDCASE); otherwise a
	// module is shown by its name.
	type found struct{ name, heading string }

	var level1 []found

	if !strings.ContainsAny(words[0], "*%") {
		for _, name := range names {
			if strings.EqualFold(name, words[0]) {
				level1 = []found{{name, strings.ToUpper(words[0])}}

				break
			}
		}
	}

	if level1 == nil {
		for _, name := range names {
			if matches(words[0], name) {
				level1 = append(level1, found{name, name})
			}
		}
	}

	if len(level1) == 0 {
		// Nothing at level 1: the library's topics are listed, but not
		// after "KEY..." (testdata/help90's "SKIP...").
		d := Display{Kind: ShowSorry, Sorry: s.sorry, ListDepth: 1, levelOneMiss: true}
		if !all {
			d.List, d.Listed, d.rooted = s.rootList(), true, true
		}

		return []Display{d}, nil
	}

	for _, f := range level1 {
		t := lib.Module(f.name)
		if t == nil {
			continue
		}

		path := []string{f.heading}
		nodes := []*Topic{t}
		s.current, s.currentNodes = path, nodes
		s.descend(t, path, nodes, words[1:])
	}

	if len(s.out) == 0 && s.failed != nil {
		return []Display{*s.failed}, nil
	}

	return s.out, nil
}

// instructions is HELP's answer to a request with no words: the root
// library's HELP topic and the library's topics, or with /NOINSTRUCTIONS
// the topics alone. A library with no HELP topic says "Sorry, no
// documentation on HELP" and lists its topics.
func instructions(lib Library, names []string, opts Options) []Display {
	list := rootList(names)

	if !opts.Instructions {
		return []Display{{Kind: ShowList, List: list, Listed: true, ListDepth: 1, rooted: true}}
	}

	for _, name := range names {
		if strings.EqualFold(name, "HELP") {
			if t := lib.Module(name); t != nil {
				return []Display{{
					Kind: ShowTopic, Headings: []string{"HELP"}, Text: t.Text, Depth: 1,
					List: list, Listed: true, ListDepth: 1, rooted: true,
				}}
			}
		}
	}

	return []Display{{Kind: ShowSorry, Sorry: "HELP", List: list, Listed: true, ListDepth: 1, rooted: true}}
}

// rootList is the list of a library's topics: its modules, in index
// order.
func rootList(names []string) []Entry {
	list := make([]Entry, len(names))
	for i, n := range names {
		list[i] = Entry{Key: n}
	}

	return list
}

// search is one request's walk down the topics its words name.
type search struct {
	lib   Library
	names []string
	words []string
	all   bool   // the first word ended in "..."
	sorry string // the words, for a "Sorry" display

	// current is the path of the numbered topics last found, which a
	// qualifier line found is shown below. VMS 7.1 keeps it from one
	// match to the next, so in "HELP ALPHA /*" the qualifier line
	// /NOTQUAL (in ALPHA Description) is shown under /DIRECT, the
	// numbered topic matched before it (testdata/help90, round 1b).
	current      []string
	currentNodes []*Topic

	out    []Display
	failed *Display // where the request went deepest without a match
}

func (s *search) rootList() []Entry { return rootList(s.names) }

// descend finds rest's words below t, whose path is path (the headings)
// and nodes (the topics).
func (s *search) descend(t *Topic, path []string, nodes []*Topic, rest []string) {
	if len(rest) == 0 {
		s.show(t, path, nodes)

		return
	}

	word := rest[0]
	found := false

	for _, c := range t.candidates(word) {
		if !matches(word, c.Key) {
			continue
		}

		found = true

		if c.Slash {
			if len(rest) > 1 {
				// A qualifier line has no subtopics (unconfirmed how
				// VMS words this; the probe had no such request).
				s.fail(c, append(append([]string(nil), s.current...), c.Key), nil)

				continue
			}

			s.out = append(s.out, Display{
				Kind:     ShowSlash,
				Headings: append([]string(nil), s.current...),
				Text:     append([]string{c.Key}, c.Text...),
				Depth:    len(s.current),
				path:     append(append([]*Topic(nil), s.currentNodes...), c),
			})

			continue
		}

		p := append(append([]string(nil), path...), c.Key)
		n := append(append([]*Topic(nil), nodes...), c)
		s.current, s.currentNodes = p, n
		s.descend(c, p, n, rest[1:])
	}

	if !found {
		s.fail(t, path, t.list())
	}
}

// fail notes that the request found nothing below path. If nothing is
// found anywhere, Lookup shows the deepest such place (the first of
// several as deep; unconfirmed, since the probe's wildcards that failed
// all failed at one place).
func (s *search) fail(t *Topic, path []string, list []Entry) {
	if s.failed != nil && len(s.failed.Headings) >= len(path) {
		return
	}

	s.failed = &Display{
		Kind: ShowSorry, Headings: path, Sorry: s.sorry,
		List: list, Listed: len(list) > 0, ListDepth: len(path),
	}
}

// show adds t's display, or with "..." t's and every topic's below it.
func (s *search) show(t *Topic, path []string, nodes []*Topic) {
	if s.all {
		s.showAll(t, path, nodes)

		return
	}

	list := t.list()
	s.out = append(s.out, topicDisplay(path, nodes))

	// A topic with no text and no subtopics is shown, and then HELP says
	// it has no documentation and lists the library's topics, as though
	// the request had found nothing at all (testdata/help90's "EMPTY
	// Child").
	if len(t.Text) == 0 && len(list) == 0 {
		s.out = append(s.out, Display{
			Kind: ShowSorry, Sorry: s.sorry,
			List: s.rootList(), Listed: true, ListDepth: 1, rooted: true,
		})
	}
}

// showAll adds t's display and those of all its numbered subtopics, in
// the module's order, with no lists.
func (s *search) showAll(t *Topic, path []string, nodes []*Topic) {
	s.out = append(s.out, Display{Kind: ShowTopic, Headings: path, Text: t.Text, Depth: len(path), path: nodes})

	for _, c := range t.Subtopics {
		s.showAll(c, append(append([]string(nil), path...), c.Key), append(append([]*Topic(nil), nodes...), c))
	}
}

// topicDisplay is a numbered topic's display: the last of nodes, under
// the headings of path, with its text and its subtopics.
func topicDisplay(path []string, nodes []*Topic) Display {
	t := nodes[len(nodes)-1]
	list := t.list()

	return Display{
		Kind: ShowTopic, Headings: path, Text: t.Text, Depth: len(path),
		List: list, Listed: len(list) > 0, ListDepth: len(path), path: nodes,
	}
}

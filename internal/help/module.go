package help

import "strings"

// Topic is one key of a help module and the text that follows it: a
// level-1 topic (the module itself), a subtopic at a deeper level, or a
// qualifier line's topic (Slash).
type Topic struct {
	// Key is the key as the module writes it ("Parameters", "/BRIEF",
	// "Two Words"): a key line's text after its level, or a qualifier
	// line itself, without trailing blanks.
	Key string
	// Level is the key line's level, 1 for the module's own key. A
	// qualifier line's topic is one level below the topic it is in.
	Level int
	// Slash reports a qualifier line's topic: a line of text starting
	// with "/" in column 1 rather than a key line.
	Slash bool
	// Text is the topic's lines, up to the next key line. For a
	// numbered topic this includes its qualifier lines and their text,
	// which HELP shows as part of it; for a qualifier line's topic it
	// is the lines after the qualifier line, up to the next qualifier
	// line or key line.
	Text []string
	// Subtopics are the topic's numbered subtopics (key lines one level
	// down), in the module's order.
	Subtopics []*Topic
	// Slashes are the qualifier lines in the topic's own text, in order.
	Slashes []*Topic
}

// keyLine reports whether line is a key line as HELP reads one, and its
// level and key: a level number, a blank or a tab, and a key. HELP takes
// a tab after the number as LIBRARY doesn't (testdata/help90's NOBLANK
// module: LIBRARY stored "2", a tab, and "Tabbed" as text, and HELP listed
// Tabbed as a subtopic). A level of 0, a number with nothing after it, or
// one followed by a letter is text. The key is the rest of the line,
// blanks and all ("2 Two Words" is the key "Two Words").
func keyLine(line string) (level int, key string, ok bool) {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}

	if digits == 0 || digits >= len(line) || (line[digits] != ' ' && line[digits] != '\t') {
		return 0, "", false
	}

	for _, c := range line[:digits] {
		level = level*10 + int(c-'0')
		if level > 1000 {
			return 0, "", false
		}
	}

	key = strings.TrimSpace(line[digits:])
	if level == 0 || key == "" {
		return 0, "", false
	}

	return level, key, true
}

// ParseModule turns a help module's records into its tree of topics and
// returns the level-1 topic. It returns nil if the records hold no key
// line. Lines before the first key line belong to no topic and are
// dropped (LIBRARY drops them too, so a library's modules have none).
//
// A key line belongs under the nearest topic before it at a lower level:
// "1 A", "2 B", "3 C", "2 D" makes B and D A's subtopics and C B's. A key
// more than one level down (LIBRARY refuses one, with ILLKEYLVL) is put
// under that nearest topic too (unconfirmed: VMS's libraries can't hold
// one). A second level-1 key ends the module, since LIBRARY starts a new
// module there.
func ParseModule(records [][]byte) *Topic {
	var (
		root  *Topic
		open  []*Topic // the numbered topics the next key may go under, by depth
		slash *Topic   // the qualifier line's topic the text now goes to, if any
	)

	for _, rec := range records {
		line := string(rec)

		if level, key, ok := keyLine(line); ok {
			if root != nil && level <= 1 {
				break
			}

			t := &Topic{Key: key, Level: level}
			slash = nil

			if root == nil {
				t.Level = 1
				root = t
				open = []*Topic{t}

				continue
			}

			for len(open) > 1 && open[len(open)-1].Level >= level {
				open = open[:len(open)-1]
			}

			parent := open[len(open)-1]
			parent.Subtopics = append(parent.Subtopics, t)
			open = append(open, t)

			continue
		}

		if root == nil {
			continue
		}

		current := open[len(open)-1]
		current.Text = append(current.Text, line)

		if strings.HasPrefix(line, "/") {
			slash = &Topic{Key: strings.TrimRight(line, " \t"), Level: current.Level + 1, Slash: true}
			current.Slashes = append(current.Slashes, slash)

			continue
		}

		if slash != nil {
			slash.Text = append(slash.Text, line)
		}
	}

	return root
}

// Entry is one key of a list of topics ("Additional information
// available:"), and whether it is a qualifier line's.
type Entry struct {
	Key   string
	Slash bool
}

// list is the topics HELP lists under t: each numbered subtopic, followed
// by the qualifier lines in its text. t's own qualifier lines aren't
// listed, since its text shows them (testdata/help90: ALPHA lists
// Qualifiers and then Qualifiers' /BRIEF, /COUNT, and /NOCONFIRM, and
// Qualifiers' own list has only its numbered /SUB).
func (t *Topic) list() []Entry {
	var out []Entry

	for _, c := range t.Subtopics {
		out = append(out, Entry{Key: c.Key})
		for _, s := range c.Slashes {
			out = append(out, Entry{Key: s.Key, Slash: true})
		}
	}

	return out
}

// candidates is the topics a word can name under t, in the module's
// order. A word that doesn't start with "/" names only numbered
// subtopics: "*" isn't a qualifier line's, and "BRIEF" isn't "/BRIEF". A
// word that does also names t's own qualifier lines and its subtopics'
// (testdata/help90: "ALPHA Qualifiers /BRIEF" and "ALPHA /BRIEF" both find
// /BRIEF, a qualifier line in Qualifiers' text). Qualifier lines deeper
// than that aren't candidates (unconfirmed: the probe had none).
func (t *Topic) candidates(word string) []*Topic {
	if !strings.HasPrefix(word, "/") {
		return t.Subtopics
	}

	out := append([]*Topic(nil), t.Slashes...)

	for _, c := range t.Subtopics {
		out = append(out, c)
		out = append(out, c.Slashes...)
	}

	return out
}

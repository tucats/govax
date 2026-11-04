package fdl

// Definition is what an FDL file says: its sections in the order the file
// gave them.
type Definition struct {
	Sections []*Section
}

// Section is one primary attribute and the secondaries given in it.
type Section struct {
	// Name is the primary's name, in capitals: "FILE", "KEY".
	Name string

	// ID is a numbered primary's number: KEY 1's 1.
	ID int

	// Text is TITLE's or IDENT's text.
	Text string

	// values are the secondaries given, by name; order the names in the
	// order first given.
	values map[string]Value
	order  []string
}

// Value is one secondary attribute's value. Which field holds it is by
// its Kind.
type Value struct {
	Kind Kind

	Switch bool
	Number int64

	// Text is a String's or FreeText's text, a Keyword's keyword (in full, in
	// capitals), an Owner's text ("[200,201]"), or a Date's as written.
	Text string

	// Protection is a protection's mask, a bit set denying, as a file
	// header's: system in bits 0 to 3 (read, write, execute, delete),
	// then owner, group, and world.
	Protection uint16

	// Time is a Date's, as a VMS time.
	Time uint64

	// Char is a Character's.
	Char byte
}

// Section returns the definition's section called name (FILE, RECORD,
// ...), or nil if it has none.
func (d *Definition) Section(name string) *Section {
	for _, s := range d.Sections {
		if s.Name == name {
			return s
		}
	}

	return nil
}

// Numbered returns the definition's sections called name (KEY, AREA) in
// the order of their numbers, which Parse holds to be the file's.
func (d *Definition) Numbered(name string) []*Section {
	var list []*Section

	for _, s := range d.Sections {
		if s.Name == name {
			list = append(list, s)
		}
	}

	return list
}

// Get returns the value of s's secondary called name, and whether it was
// given. A nil Section has none.
func (s *Section) Get(name string) (Value, bool) {
	if s == nil {
		return Value{}, false
	}

	v, ok := s.values[name]

	return v, ok
}

// Has reports whether s's secondary called name was given.
func (s *Section) Has(name string) bool {
	_, ok := s.Get(name)

	return ok
}

// Switch returns a switch's value, or def if it wasn't given.
func (s *Section) Switch(name string, def bool) bool {
	if v, ok := s.Get(name); ok {
		return v.Switch
	}

	return def
}

// Number returns a number's value, or def if it wasn't given.
func (s *Section) Number(name string, def int64) int64 {
	if v, ok := s.Get(name); ok {
		return v.Number
	}

	return def
}

// Keyword returns a keyword's value, or def if it wasn't given.
func (s *Section) Keyword(name, def string) string {
	if v, ok := s.Get(name); ok {
		return v.Text
	}

	return def
}

// String returns a string's text, and whether it was given.
func (s *Section) String(name string) (string, bool) {
	v, ok := s.Get(name)

	return v.Text, ok
}

// Names returns the names of s's secondaries in the order first given.
func (s *Section) Names() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.order...)
}

// set records a secondary's value, reporting whether it was already given.
func (s *Section) set(name string, v Value) (again bool) {
	if s.values == nil {
		s.values = map[string]Value{}
	}

	_, again = s.values[name]
	if !again {
		s.order = append(s.order, name)
	}

	s.values[name] = v

	return again
}

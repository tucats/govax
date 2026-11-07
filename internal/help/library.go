package help

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/lbr"
)

// Library is a help library as Lookup reads it: its modules' names, in
// the order of the library's index (alphabetical, case aside), and each
// module's tree of topics.
type Library interface {
	// Names returns the modules' names as the library keeps them
	// ("ALPHA", "MixedCase"), in index order.
	Names() []string
	// Module returns the level-1 topic of the module a name from Names
	// names, or nil if it can't be read.
	Module(name string) *Topic
}

// LBRLibrary is a Library over a help library internal/lbr has read. Each
// module is parsed the first time it's asked for, and kept.
type LBRLibrary struct {
	lib     *lbr.Library
	names   []string
	modules map[string]*Topic
}

// NewLBRLibrary makes a Library of lib, a help library.
func NewLBRLibrary(lib *lbr.Library) *LBRLibrary {
	l := &LBRLibrary{lib: lib, modules: map[string]*Topic{}}

	if len(lib.Indexes) > 0 {
		for _, k := range lib.Indexes[0].Keys {
			l.names = append(l.names, k.Name)
		}
	}

	return l
}

// Names returns the library's module names, in index order.
func (l *LBRLibrary) Names() []string { return l.names }

// Module returns the named module's topics, or nil if the library has no
// such module or it can't be read.
func (l *LBRLibrary) Module(name string) *Topic {
	if t, ok := l.modules[name]; ok {
		return t
	}

	var t *Topic

	if rfa, ok := l.lib.Lookup(name); ok {
		if m, err := l.lib.Module(rfa); err == nil {
			t = ParseModule(m.Records)
		}
	}

	l.modules[name] = t

	return t
}

// MemoryLibrary is a Library of modules held in memory, by name: for
// tests, and for help text that isn't in a library file.
type MemoryLibrary struct {
	names   []string
	modules map[string]*Topic
}

// NewMemoryLibrary makes a Library of help text: a help file's lines,
// split into modules at each level-1 key line as LIBRARY/HELP splits
// them.
func NewMemoryLibrary(lines []string) *MemoryLibrary {
	l := &MemoryLibrary{modules: map[string]*Topic{}}

	var module [][]byte

	flush := func() {
		if t := ParseModule(module); t != nil {
			if _, dup := l.modules[t.Key]; !dup {
				l.names = append(l.names, t.Key)
			}

			l.modules[t.Key] = t
		}

		module = nil
	}

	for _, line := range lines {
		if level, _, ok := keyLine(line); ok && level == 1 {
			flush()
		}

		module = append(module, []byte(line))
	}

	flush()

	// The index's order: by name, upper-cased.
	sortNames(l.names)

	return l
}

// Names returns the modules' names in the index's order.
func (l *MemoryLibrary) Names() []string { return l.names }

// Module returns the named module's topics.
func (l *MemoryLibrary) Module(name string) *Topic { return l.modules[name] }

// sortNames sorts names as a help library's index orders its keys: by
// their upper-cased text.
func sortNames(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		return strings.ToUpper(names[i]) < strings.ToUpper(names[j])
	})
}

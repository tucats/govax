//go:build ignore

// decode reads the objects of Phase 88's definition probe (README.md): real VAX
// MACRO's vax/def_*.obj, each internal definition macro called with GLOBAL, so an
// object's global symbol directory holds every name the macro defines, with its
// value. It writes testdata/idx88/defined.txt, one "NAME = value" line per name:
// values only, never the macros' text (the author's answer 5, docs/PHASE-88 -
// keyed index files.md). Run it from the repository root:
//
//	go run testdata/idx88/decode.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
)

const dir = "testdata/idx88"

// load reads one object file, stored in the host's copy of ODS-2's
// variable-length record layout, and decodes its records.
func load(path string) (*obj.Module, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records, err := obj.ReadRecords(f)
	if err != nil {
		return nil, err
	}

	return obj.Decode(records)
}

func main() {
	objects, err := filepath.Glob(filepath.Join(dir, "vax", "def_*.obj"))
	if err != nil {
		log.Fatal(err)
	}

	if len(objects) == 0 {
		log.Fatalf("no objects in %s", filepath.Join(dir, "vax"))
	}

	sort.Strings(objects)

	var b strings.Builder

	b.WriteString("# The names of the indexed file definitions, as real VAX MACRO (VMS 7.1)\n")
	b.WriteString("# assembled Phase 88's definition probe: testdata/idx88/decode.go wrote this\n")
	b.WriteString("# from testdata/idx88/vax/def_*.obj.\n")

	for _, path := range objects {
		m, err := load(path)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}

		probe := strings.ToUpper(strings.TrimSuffix(filepath.Base(path), ".obj"))
		fmt.Fprintf(&b, "\n# $%sDEF\n", strings.TrimPrefix(probe, "DEF_"))

		var lines []string

		for _, s := range m.Symbols() {
			if s.Defined() {
				lines = append(lines, fmt.Sprintf("%s = %#x", s.Name, s.Value))
			}
		}

		sort.Strings(lines)

		for _, l := range lines {
			b.WriteString(l + "\n")
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "defined.txt"), []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

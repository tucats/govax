//go:build ignore

// decode reads the object of Phase 83's definition probe (README.md): real
// VAX MACRO's vax/def_opc.obj, $OPCDEF called with GLOBAL, so the object's
// global symbol directory holds every name the macro defines, with its
// value. It writes testdata/for83/defined.txt, one "NAME = value" line per
// name, which internal/vmsdef/gen's -values reads. Run it from the
// repository root:
//
//	go run testdata/for83/decode.go
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

const dir = "testdata/for83"

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

	b.WriteString("# The names $OPCDEF defines, as real VAX MACRO (VMS 7.1) assembled\n")
	b.WriteString("# Phase 83's definition probe: testdata/for83/decode.go wrote this\n")
	b.WriteString("# from testdata/for83/vax/def_opc.obj.\n")

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

//go:build ignore

// decode reads the Phase 32 oracle's definition probes (docs/PHASE-32.md,
// subtask 3): real MACRO's objects for def_*.mar, each a $xxxDEF macro
// followed by a .LONG of every candidate name. A name the macro defines
// is data in the object; one it doesn't is an external reference. It
// writes testdata/mar/rms/defined.txt: for each macro, every candidate
// name with its value or "undefined". internal/vmsdef/gen's -values reads
// the values from it. Run it from the repository root:
//
//	go run testdata/mar/rms/decode.go
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
)

const dir = "testdata/mar/rms"

// stored is one longword the data psect holds: a value, or an external
// reference to a symbol.
type stored struct {
	value    uint32
	external string
}

// longwords returns, in order, the longwords stored in the module's DATA
// psect. Immediate data is a byte stream (MACRO splits a long run of it
// across records at any byte), so it is gathered first and cut into
// longwords at the end.
func longwords(m *obj.Module) ([]stored, error) {
	data := -1

	for i, p := range m.Psects() {
		if p.Name == "DATA" {
			data = i
		}
	}

	type item struct {
		psect  int
		value  uint32
		global string
	}

	var (
		stack     []item
		buf       []byte
		externals = map[int]string{}
		current   = -1
	)

	pop := func() item {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		return x
	}

	for _, rec := range m.Records {
		t, ok := rec.(*obj.TIR)
		if !ok || t.Type != obj.RecTIR {
			continue
		}

		for _, c := range t.Commands {
			switch op := c.Op.String(); {
			case op == "STA_PB" || op == "STA_PW" || op == "STA_PL":
				stack = append(stack, item{psect: int(c.Psect), value: c.Value})
			case op == "STA_GBL":
				stack = append(stack, item{psect: -1, global: c.Name})
			case strings.HasPrefix(op, "STA_"):
				stack = append(stack, item{psect: -1, value: c.StackedValue()})
			case op == "CTL_SETRB":
				current = pop().psect
			case op == "CTL_AUGRB":
			case op == "STO_IMM":
				if current == data {
					buf = append(buf, c.Data...)
				}
			case op == "STO_L" || op == "STO_LD" || op == "STO_PIDR":
				x := pop()
				if current != data {
					continue
				}

				if x.global != "" {
					externals[len(buf)] = x.global
				}

				buf = binary.LittleEndian.AppendUint32(buf, x.value)
			default:
				if current == data {
					return nil, fmt.Errorf("unexpected %s in the data", op)
				}
			}
		}
	}

	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("%d bytes of data isn't whole longwords", len(buf))
	}

	out := make([]stored, 0, len(buf)/4)
	for i := 0; i < len(buf); i += 4 {
		out = append(out, stored{value: binary.LittleEndian.Uint32(buf[i:]), external: externals[i]})
	}

	return out, nil
}

// globalProbes are the objects of the error probes that call a $xxxDEF
// with GLOBAL: each defines every name the macro does as a global symbol.
var globalProbes = map[string]string{
	"FAB": "ERR_DEF_GLOBAL.OBJ;1",
	"SS":  "ERR_SS_GLOBAL.OBJ;1",
}

type global struct {
	name  string
	value uint32
}

// globals returns the global symbols an object defines.
func globals(path string) ([]global, error) {
	m, err := load(path)
	if err != nil {
		return nil, err
	}

	var out []global

	for _, s := range m.Symbols() {
		if s.Defined() {
			out = append(out, global{name: s.Name, value: s.Value})
		}
	}

	return out, nil
}

// names returns the names a probe's .LONG lines store, in order.
func names(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string

	s := bufio.NewScanner(f)
	for s.Scan() {
		if n, ok := strings.CutPrefix(s.Text(), "\t.LONG\t"); ok {
			out = append(out, n)
		}
	}

	return out, s.Err()
}

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
	objects, err := filepath.Glob(filepath.Join(dir, "vax", "DEF_*.OBJ;*"))
	if err != nil {
		log.Fatal(err)
	}

	sort.Strings(objects)

	var b strings.Builder

	b.WriteString("# The names each $xxxDEF macro defines, as real VAX MACRO (VMS 7.3)\n")
	b.WriteString("# assembled the definition probes: testdata/mar/rms/decode.go wrote this\n")
	b.WriteString("# from testdata/mar/rms/vax/DEF_*.OBJ. \"undefined\" means the macro\n")
	b.WriteString("# leaves the name undefined (the probe's .LONG became an external).\n")

	for _, path := range objects {
		probe := strings.ToLower(strings.SplitN(filepath.Base(path), ".", 2)[0])
		if probe == "def_twice" {
			continue
		}

		m, err := load(path)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}

		ns, err := names(filepath.Join(dir, probe+".mar"))
		if err != nil {
			log.Fatal(err)
		}

		ls, err := longwords(m)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}

		if len(ls) != len(ns) {
			log.Fatalf("%s: %d longwords for %d names", path, len(ls), len(ns))
		}

		family := strings.ToUpper(strings.TrimPrefix(probe, "def_"))

		fmt.Fprintf(&b, "\n# $%sDEF\n", family)

		probed := map[string]bool{}

		for i, n := range ns {
			probed[n] = true

			if ls[i].external != "" {
				fmt.Fprintf(&b, "%s undefined\n", n)
			} else {
				fmt.Fprintf(&b, "%s = %#x\n", n, ls[i].value)
			}
		}

		// A global-form probe's object defines every name its macro does,
		// as a global symbol: any the candidates missed are added.
		if g, ok := globalProbes[family]; ok {
			extra, err := globals(filepath.Join(dir, "vax", g))
			if err != nil {
				log.Fatal(err)
			}

			for _, s := range extra {
				if !probed[s.name] {
					fmt.Fprintf(&b, "%s = %#x\n", s.name, s.value)
				}
			}
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "defined.txt"), []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

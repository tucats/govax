package sysdef

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmsdef"
)

// moduleOpener gives a compilation this package's modules, as the FORTRAN
// command does.
type moduleOpener struct{}

func (moduleOpener) OpenInclude(name string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no file " + name)
}

func (moduleOpener) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	lines, ok := Module(module)
	if !ok {
		return nil, errors.New("no module " + module)
	}

	return &source.File{Spec: module, Lines: lines}, nil
}

// compileWith compiles a program's lines with the modules at hand.
func compileWith(lines ...string) fortran.Result {
	return fortran.Compile(fortran.Options{Includes: moduleOpener{}, Compiled: time.Unix(0, 0).UTC()},
		fortran.File{Spec: "T.FOR", Name: "T", Lines: lines})
}

// TestEveryModuleCompiles compiles a program that includes each module, under
// IMPLICIT NONE: every line is FORTRAN, and every name is declared.
func TestEveryModuleCompiles(t *testing.T) {
	for _, name := range Names() {
		r := compileWith("      PROGRAM T", "      IMPLICIT NONE", "      INCLUDE '("+name+")'", "      END")
		if r.Errors != 0 || r.Warnings != 0 {
			t.Errorf("%s: %v", name, r.Messages[:min(3, len(r.Messages))])
		}
	}
}

// TestAllModulesTogether compiles one unit including them all, so no name is in
// two of them.
func TestAllModulesTogether(t *testing.T) {
	lines := []string{"      PROGRAM T", "      IMPLICIT NONE"}
	for _, name := range Names() {
		lines = append(lines, "      INCLUDE '("+name+")'")
	}

	r := compileWith(append(lines, "      END")...)
	if r.Errors != 0 || r.Warnings != 0 {
		t.Errorf("%v", r.Messages[:min(5, len(r.Messages))])
	}
}

// TestPlannedModules checks the modules the phase's plan names, with the number
// of names each had in the survey's prototype, and that a name's value is
// vmsdef's as a hexadecimal constant.
func TestPlannedModules(t *testing.T) {
	counts := map[string]int{
		"$SSDEF": 731, "$IODEF": 547, "$JPIDEF": 222, "$SYIDEF": 445, "$DVIDEF": 187, "$RMSDEF": 268,
		"$LIBDEF": 117, "$FORDEF": 172, "$MTHDEF": 32, "$DSCDEF": 138, "$STSDEF": 32, "$LNMDEF": 40,
		"$SECDEF": 67, "$PSLDEF": 39, "$PRVDEF": 43, "$CHFDEF": 21, "$OPCDEF": 228,
	}

	for name, want := range counts {
		lines, ok := Module(name)
		if !ok {
			t.Errorf("no module %s", name)

			continue
		}

		got := 0

		for _, line := range lines {
			if strings.Contains(line, "PARAMETER") {
				got++
			}
		}

		if got != want {
			t.Errorf("%s has %d names, want %d", name, got, want)
		}
	}

	lines, _ := Module("$ssdef") // any case
	if !slices.Contains(lines, "      PARAMETER (SS$_ABORT = '0000002C'X)") {
		t.Error("$SSDEF hasn't SS$_ABORT's value")
	}

	if vmsdef.Symbols["SS$_ABORT"] != 0x2C {
		t.Errorf("SS$_ABORT is %#x in vmsdef", vmsdef.Symbols["SS$_ABORT"])
	}

	if _, ok := Module("$NOSUCHDEF"); ok {
		t.Error("found $NOSUCHDEF")
	}
}

// TestNames checks the rules for names: capitals, the most FORTRAN allows, one
// spelling of a name that differs by case in vmsdef, and a split family's names
// in the right modules.
func TestNames(t *testing.T) {
	seen := map[string]string{}

	for _, name := range Names() {
		lines, _ := Module(name)

		for _, line := range lines {
			rest, ok := strings.CutPrefix(line, "      INTEGER*4 ")
			if !ok {
				continue
			}

			if len(rest) > maxName || rest != strings.ToUpper(rest) {
				t.Errorf("%s: name %s", name, rest)
			}

			if other, dup := seen[rest]; dup {
				t.Errorf("%s is in %s and %s", rest, other, name)
			}

			seen[rest] = name
		}
	}

	for name, module := range map[string]string{
		"TT$_PRO_SERIES":    "$TTDEF",
		"LIB$_NORMAL":       "$LIBDEF",
		"LIB$K_DCFOPR_AB":   "$LIBDCFDEF",
		"LIB$K_DAY_OF_WEEK": "$LIBDTDEF",
		"LIB$K_VM_FIXED":    "$LIBVMDEF",
		"LIB$M_CLI_CTRLY":   "$LIBCLIDEF",
		"TT2$M_ANSICRT":     "$TT2DEF",
	} {
		if seen[name] != module {
			t.Errorf("%s is in %q, want %s", name, seen[name], module)
		}
	}
}

// TestServices checks $SYSSRVNAM: every service, as a function and EXTERNAL,
// without the vector's cells and spares.
func TestServices(t *testing.T) {
	lines, ok := Module("$SYSSRVNAM")
	if !ok {
		t.Fatal("no $SYSSRVNAM")
	}

	for _, want := range []string{"      INTEGER*4 SYS$ASCTIM", "      EXTERNAL SYS$ASCTIM", "      EXTERNAL SYS$QIOW", "      EXTERNAL SYS$OPEN"} {
		if !slices.Contains(lines, want) {
			t.Errorf("no %q", want)
		}
	}

	for _, line := range lines {
		for _, bad := range []string{"SYS$GL_", "SPARE", "_2", "SYS$SRCHANDLER"} {
			if strings.Contains(line, bad) {
				t.Errorf("%q has %s", line, bad)
			}
		}
	}
}

// TestIOSTATNames checks $FORIOSDEF: the numbers IOSTAT= returns, which are
// FOR$K_'s wherever vmsdef has that name too.
func TestIOSTATNames(t *testing.T) {
	values := allValues()

	list := iostatConstants(values)
	if len(list) == 0 {
		t.Fatal("no names")
	}

	checked := 0

	for _, c := range list {
		short := strings.TrimPrefix(c.name, iostatPrefix)
		if k, ok := values["FOR$K_"+short]; ok {
			checked++

			if k != c.value {
				t.Errorf("%s is %d, FOR$K_%s is %d", c.name, c.value, short, k)
			}
		}
	}

	if checked < 50 {
		t.Errorf("only %d names checked against FOR$K_", checked)
	}

	lines, _ := Module("$FORIOSDEF")
	if !slices.Contains(lines, "      PARAMETER (FOR$IOS_FILNOTFOU = '0000001D'X)") {
		t.Error("FOR$IOS_FILNOTFOU isn't 29")
	}
}

// TestHandlerLayouts holds $CHFDEF's structures to vmsdef's offsets and sizes.
func TestHandlerLayouts(t *testing.T) {
	values := allValues()

	for _, s := range handlerLayouts {
		size, ok := values[s.size]
		if !ok || int(size) != 4*len(s.fields) {
			t.Errorf("%s: %s is %d, with %d longwords", s.name, s.size, size, len(s.fields))
		}

		for i, field := range s.fields {
			if field == "" {
				continue
			}

			if offset, ok := values[field]; !ok || int(offset) != 4*i {
				t.Errorf("%s: %s is at %d, the structure has it at %d", s.name, field, offset, 4*i)
			}
		}
	}
}

// TestLibrary reads the library Library makes: every module, with its text.
func TestLibrary(t *testing.T) {
	data, err := Library(time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	lib, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	if lib.Type != lbr.TypeText {
		t.Fatalf("a %s library", lib.Type)
	}

	for _, name := range Names() {
		rfa, ok := lib.Lookup(name)
		if !ok {
			t.Errorf("no %s", name)

			continue
		}

		m, err := lib.Module(rfa)
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		lines, _ := Module(name)
		if len(m.Records) != len(lines) {
			t.Errorf("%s: %d records, want %d", name, len(m.Records), len(lines))

			continue
		}

		for i, record := range m.Records {
			if string(record) != lines[i] {
				t.Errorf("%s, line %d: %q, want %q", name, i+1, record, lines[i])

				break
			}
		}
	}
}

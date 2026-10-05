package console

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The fixture ANALYZE/OBJECT's tests use: real MACRO's HELLO.OBJ and real
// ANALYZE's analysis of it.
const (
	analyzeFixtureObj = "../../testdata/mar/vax/hello.obj"
	analyzeFixtureAnl = "../../testdata/mar/vax/hello.anl"
)

// analysisTimeRE matches a page header's date and time, and
// analysisFileRE the file line after it.
var (
	analysisTimeRE = regexp.MustCompile(`(?m)^(Analyze Object File +)[ 0-9]{2}-[A-Z]{3}-[0-9]{4} [0-9:.]{11}(   Page [0-9]+\n)[^\n]*\n`)
	analysisCmdRE  = regexp.MustCompile(`(?m)^ANALYZE/OBJECT[^\n]*\n\z`)
)

// normalizeAnalysis masks what differs between two runs of ANALYZE on the
// same object: the times, the file's name, and the command.
func normalizeAnalysis(text string) string {
	text = analysisTimeRE.ReplaceAllString(text, "${1}TIME${2}FILE\n")

	return analysisCmdRE.ReplaceAllString(text, "COMMAND\n")
}

func TestAnalyze_hostFile(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	out := filepath.Join(t.TempDir(), "hello.anl")

	if err := d.Dispatch(`ANALYZE/OBJECT/OUTPUT="` + out + `" "` + analyzeFixtureObj + `"`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("ANALYZE/OUTPUT wrote to the console: %q", buf.String())
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(analyzeFixtureAnl)
	if err != nil {
		t.Fatal(err)
	}

	if normalizeAnalysis(string(got)) != normalizeAnalysis(string(want)) {
		t.Errorf("the analysis differs from VMS's:\n%s", got)
	}

	abs, _ := filepath.Abs(analyzeFixtureObj)
	if !strings.Contains(string(got), "\n"+abs+"\nANALYZ V07-04\n") {
		t.Errorf("the page header doesn't name %s", abs)
	}
}

func TestAnalyze_console(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	// The record-type qualifiers may come before or after /OBJECT.
	for _, line := range []string{
		`ANALYZE/OBJECT/GSD "` + analyzeFixtureObj + `"`,
		`ANALYZE/GSD/OBJECT "` + analyzeFixtureObj + `"`,
	} {
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		text := buf.String()

		for _, want := range []string{
			"This is an OpenVMS VAX object file",
			"6.  GLOBAL SYMBOL DIRECTORY (OBJ$C_GSD), 20 bytes",
			"OBJ$C_TIR\t    5\t    93",
			"The analysis uncovered NO errors.",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the report lacks %q", line, want)
			}
		}

		if strings.Contains(text, "MODULE HEADER") || strings.Contains(text, "TEXT INFORMATION") {
			t.Errorf("%s: /GSD showed other records", line)
		}
	}
}

func TestAnalyze_volume(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	f, err := os.Open(analyzeFixtureObj)
	if err != nil {
		t.Fatal(err)
	}

	records, err := obj.ReadRecords(f)
	f.Close()

	if err != nil {
		t.Fatal(err)
	}

	s := c.ContainerSession
	if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]HELLO.OBJ"}, rms.VariableRecords, records); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("ANALYZE/OBJECT/OUTPUT DUA0:[000000]HELLO"); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	lines, _, err := s.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]HELLO.ANL"}, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading the analysis: %v", err)
	}

	if len(lines) < 4 || string(lines[0]) != "\f" || string(lines[2]) != "DUA0:[000000]HELLO.OBJ;1" {
		t.Errorf("the analysis begins %q", lines[:min(4, len(lines))])
	}

	if last := string(lines[len(lines)-1]); !strings.HasPrefix(last, "ANALYZE/OBJECT/OUTPUT DUA0:[000000]HELLO ") || len(last) != 80 {
		t.Errorf("the analysis ends %q", last)
	}
}

func TestAnalyze_errors(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	if err := d.Dispatch("ANALYZE"); !errors.Is(err, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER)) {
		t.Errorf("ANALYZE: %v, want CLI_MISSINGPARAMETER", err)
	}

	err := d.Dispatch(`ANALYZE/OBJECT "../../testdata/mar/list/vax/dbgsrc.obj"`)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_ANALYZEERRORS)) {
		t.Errorf("ANALYZE of a bad object: %v, want CLI_ANALYZEERRORS", err)
	}

	if !strings.Contains(buf.String(), "The analysis uncovered 2 errors.") {
		t.Error("the report of the bad object wasn't written")
	}

	err = d.Dispatch(`ANALYZE/OBJECT "` + filepath.Join(t.TempDir(), "missing.obj") + `"`)
	if !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("ANALYZE of a missing file: %v, want SS_NOSUCHFILE", err)
	}
}

func TestAnalyze_library(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	lib := filepath.Join(t.TempDir(), "mods.olb")

	if err := c.Library(LibraryOptions{Library: lib, LibraryHost: true, Create: true, Object: true,
		Inputs: []string{analyzeFixtureObj, "../../testdata/mar/vax/entry.obj"}, InputHost: true}); err != nil {
		t.Fatalf("LIBRARY/CREATE: %v", err)
	}

	cases := []struct {
		line       string
		has, hasnt []string
	}{
		{`ANALYZE/OBJECT "` + lib + `"`, []string{`module name: "HELLO"`, `module name: "ENTRY"`}, nil},
		{`ANALYZE/OBJECT/INCLUDE "` + lib + `"`, []string{`module name: "HELLO"`, `module name: "ENTRY"`}, nil},
		{`ANALYZE/OBJECT/INCLUDE=HEL* "` + lib + `"`, []string{`module name: "HELLO"`}, []string{`"ENTRY"`}},
	}

	for _, tt := range cases {
		buf.Reset()

		if err := d.Dispatch(tt.line); err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}

		text := buf.String()

		for _, want := range append(tt.has, "The analysis uncovered NO errors.") {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the report lacks %q", tt.line, want)
			}
		}

		for _, bad := range tt.hasnt {
			if strings.Contains(text, bad) {
				t.Errorf("%s: the report has %q", tt.line, bad)
			}
		}
	}

	if err := d.Dispatch(`ANALYZE/OBJECT/INCLUDE=NONE "` + lib + `"`); !errors.Is(err, vmserrors.New(vmserrors.CLI_ANALYZE)) {
		t.Errorf("a module that isn't there: %v, want CLI_ANALYZE", err)
	}

	if err := d.Dispatch(`ANALYZE/OBJECT/INCLUDE "` + analyzeFixtureObj + `"`); !errors.Is(err, vmserrors.New(vmserrors.CLI_ANALYZE)) {
		t.Errorf("/INCLUDE on an object file: %v, want CLI_ANALYZE", err)
	}
}

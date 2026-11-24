package link

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// tir111Object reads VMS 7.1's MACRO's object of testdata/tir111's module name,
// one assembled with errors.
func tir111Object(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "testdata", "tir111", "vax", name+".obj"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	records, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(records)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// TestLinkCompilationErrors links two objects VMS's MACRO wrote for modules with
// errors (their end records' severity 2), with one without, and gets VMS 7.1's
// LINK's messages (testdata/f90r's round 1): ERRORS for the first, NOIMGFIL,
// ERRORS for the second, and no image (Phase 113, subtask 5).
func TestLinkCompilationErrors(t *testing.T) {
	inputs := []Input{
		{File: "DUA1:[WORK]TIRA.OBJ;1", Module: tir111Object(t, "tira")},
		{File: "DUA1:[WORK]TIRX.OBJ;1", Module: tir111Object(t, "tirx")},
		{File: "DUA1:[WORK]TIRR.OBJ;1", Module: tir111Object(t, "tirr")},
	}

	_, err := Link(inputs, Options{})

	var fe *FatalError
	if !errors.As(err, &fe) {
		t.Fatalf("error %v, want a FatalError", err)
	}

	want := "%LINK-E-ERRORS, compilation errors\n\tin module .MAIN. file DUA1:[WORK]TIRX.OBJ;1\n" +
		"%LINK-E-NOIMGFIL, image file not created\n" +
		"%LINK-E-ERRORS, compilation errors\n\tin module .MAIN. file DUA1:[WORK]TIRR.OBJ;1"
	if fe.Error() != want || fe.Status() != 0x006482B2 {
		t.Errorf("error %q, status %X", fe.Error(), fe.Status())
	}
}

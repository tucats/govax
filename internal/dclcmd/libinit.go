package dclcmd

import (
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/link"
)

// LIB$INITIALIZE's module (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 6; internal/link/libinit.go). VMS's LINK takes it from STARLET.OLB;
// govax's, assembled from internal/bootdata's libinit.mar, is offered after
// the system libraries, so a STARLET.OLB that has the module still supplies
// its own.

// libinitSource is a symbol source with one module, govax's LIB$INITIALIZE.
type libinitSource struct{}

// libinitFile is the module's source, and libinitName the file a link names it
// by (the map's, and LINK-E-ERRORS's).
const (
	libinitFile = "libinit.mar"
	libinitName = "LIB$INITIALIZE (built into govax)"
)

// libinitModule is the module, assembled once.
var libinitModule = sync.OnceValues(func() (*link.Input, error) {
	text, err := fs.ReadFile(bootdata.FS, libinitFile)
	if err != nil {
		return nil, err
	}

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	if _, err := a.Assemble(string(text)); err != nil {
		return nil, fmt.Errorf("%s: %w", libinitFile, err)
	}

	m, err := a.Object(asm.ObjectOptions{Created: time.Date(2026, time.November, 23, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", libinitFile, err)
	}

	return &link.Input{File: libinitName, Module: m, System: true}, nil
})

// Lookup implements link.SymbolSource: LIB$INITIALIZE alone.
func (libinitSource) Lookup(name string) (link.Definition, bool, error) {
	if name != "LIB$INITIALIZE" {
		return link.Definition{}, false, nil
	}

	in, err := libinitModule()
	if err != nil {
		return link.Definition{}, false, err
	}

	return link.Definition{Module: in}, true, nil
}

// Image implements link.SymbolSource: it describes no shareable image.
func (libinitSource) Image(string) (link.SharedImage, bool) { return link.SharedImage{}, false }

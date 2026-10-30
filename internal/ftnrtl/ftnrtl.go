// Package ftnrtl is FTNRTL.EXE, the run-time library of govax's FORTRAN
// compiler (docs/PHASE-74 - FORTRAN plan.md, Decision 13): a shareable
// image of routines written in MACRO-32, which a compiled FORTRAN
// program calls for what its own instructions don't do (I/O, STOP, ...).
//
// The library is written in MACRO, not emulated in Go as LIBRTL is,
// because it is part of the FORTRAN system govax is building: its
// routines run as VAX code, are traced and debugged as any program's,
// and show, as the compiler's output does, how such a library is made
// on VMS. Its sources are here:
//
//	mar/ftn_vector.mar   the transfer vector: one entry per routine, at
//	                     offsets that never change
//	mar/ftn_*.mar        the routines, a module per subject
//	ftnrtl.opt           the LINK options that make the image
//	ftnrtl.exe           the image, generated from them and committed
//
// "go generate ./internal/ftnrtl" rebuilds ftnrtl.exe with govax's own
// assembler and linker (internal/ftnrtl/build, the code the MACRO and
// LINK commands use), and a test checks that the committed image is what
// the sources build.
//
// The image is embedded in govax, so it is always there: when the image
// activator (RUN) and LINK look for a shareable image named FTNRTL, they
// look in the usual places first (a logical name FTNRTL, SYS$SHARE, the
// host library directory) and take this one last (Image). So someone
// changing FTNRTL can build their own with MACRO and LINK/SHAREABLE and
// DEFINE FTNRTL to it.
package ftnrtl

import (
	"embed"
	"strings"
)

//go:generate go run ./mkftnrtl

// Name is the image's name, as a program's list of shareable images
// names it.
const Name = "FTNRTL"

// Files holds the library's sources (mar/*.mar and ftnrtl.opt) and the
// built image (ftnrtl.exe).
//
//go:embed ftnrtl.opt ftnrtl.exe mar/*.mar
var Files embed.FS

// The files' names in Files.
const (
	OptionsFile = "ftnrtl.opt"
	ImageFile   = "ftnrtl.exe"
	SourceDir   = "mar"
)

// image is the embedded image's contents.
var image = mustRead(ImageFile)

// Image returns the embedded FTNRTL.EXE's contents, and true, if name
// is FTNRTL (in any case); it is a shareable image locator's last place
// to look (rms.ImageLocator.Builtin). The bytes are shared: the caller
// must not change them.
func Image(name string) ([]byte, bool) {
	if !strings.EqualFold(strings.TrimSpace(name), Name) {
		return nil, false
	}

	return image, true
}

// mustRead reads one of the embedded files, which is there by
// construction.
func mustRead(name string) []byte {
	data, err := Files.ReadFile(name)
	if err != nil {
		panic("ftnrtl: " + err.Error())
	}

	return data
}

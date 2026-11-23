// Package fcalc is FCALC.EXE, a scientific calculator written in Fortran 90:
// the Fortran 90 program's milestone (docs/planning/PHASE-111 - Fortran 90
// milestone.md), which govax ships on its system volume as a tool and as an
// example of a whole program in modules (docs/MANUAL.md, chapter 14).
//
// Its sources are here, the file a module each but the last, the main
// program:
//
//	src/*.f90           the sources, commented for a reader learning
//	                    Fortran 90 and numerical programming
//	src/compiles.txt    the order they are compiled in, a module before
//	                    any module that USEs it
//	src/gfortran/       gfortran's twin of FCPLATFORM, the one module that
//	                    differs between systems (not shipped)
//	buildfcalc.com      the DCL procedure that builds FCALC.EXE on govax
//	fcalc.exe           the image, generated from the sources and committed
//
// "go generate ./internal/fcalc" rebuilds fcalc.exe with govax's own
// compiler, assembler, and linker (internal/fcalc/build, the code the
// FORTRAN, MACRO, and LINK commands use), and a test checks that the
// committed image is what the sources build. The image is committed rather
// than built as govax starts because compiling fcalc's 11,000 lines takes
// seconds; on the system volume it is SYS$SYSTEM:FCALC.EXE from the first
// start (internal/sysvol), and the sources, in
// SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC], build it again with BUILDFCALC.COM.
//
// testdata/fcalc holds fcalc's scripts and what each prints, which
// internal/console's TestFcalcMilestone runs fcalc on in every mode the
// compiler has.
package fcalc

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:generate go run ./mkfcalc

// Files holds fcalc's sources, the build procedure, and the built image.
//
//go:embed src/*.f90 src/compiles.txt buildfcalc.com fcalc.exe
var Files embed.FS

// The files' names in Files.
const (
	SourceDir     = "src"
	OrderFile     = "src/compiles.txt"
	ImageFile     = "fcalc.exe"
	ProcedureFile = "buildfcalc.com"
)

// Image returns FCALC.EXE's contents. The bytes are shared: the caller
// must not change them.
func Image() []byte { return mustRead(ImageFile) }

// Modules returns fcalc's modules by name, uppercase, in the order they
// are compiled, read from fsys (laid out as Files is): FCPLATFORM first,
// FCALC, the main program, last.
func Modules(fsys fs.FS) ([]string, error) {
	text, err := fs.ReadFile(fsys, OrderFile)
	if err != nil {
		return nil, err
	}

	return strings.Fields(strings.ToUpper(string(text))), nil
}

// SourceFile is the name in Files of a module's source: src/fckinds.f90
// for FCKINDS.
func SourceFile(module string) string {
	return path.Join(SourceDir, strings.ToLower(module)+".f90")
}

// mustRead reads one of the embedded files, which is there by
// construction.
func mustRead(name string) []byte {
	data, err := Files.ReadFile(name)
	if err != nil {
		panic("fcalc: " + err.Error())
	}

	return data
}

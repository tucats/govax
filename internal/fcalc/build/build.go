// Package build makes FCALC.EXE from its sources (internal/fcalc), with
// internal/fortran, internal/asm, and internal/link, as these commands
// would (BUILDFCALC.COM's):
//
//	$ FORTRAN/G_FLOATING FCPLATFORM
//	  ... each module in compiles.txt's order ...
//	$ MACRO FCPLATFORM
//	  ... each module ...
//	$ LINK FCALC,FCPLATFORM,FCKINDS,...,FCSTATE
//
// Each module is compiled /OPTIMIZE (the default), its module text, which
// the FORTRAN command would write as a NAME.F90$MOD file, kept in memory for
// the next modules' USE statements. Everything that would vary from build to
// build (the compile, assembly, and link times, and the tools' names, which
// carry govax's version) is fixed, so the same sources always make the same
// image, byte for byte, and the committed image can be checked against them.
// The link's symbols are govax's own (dclcmd.BuiltinSources): FTNRTL from
// the image built into govax, and LIBRTL, MTHRTL, and LBRSHR from govax's
// tables, so the image is the same wherever it's built.
//
// It is a package of its own, not part of internal/fcalc, because it needs
// the LINK command's symbol tables, and internal/sysvol, which dclcmd uses,
// needs internal/fcalc for the image.
package build

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/fcalc"
	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/obj"
)

// The fixed names and times a build records.
var (
	// buildTime is each module's compile and creation time and the link
	// time.
	buildTime = time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC)

	// compiler names the compiler in the MACRO files' headings, assembler
	// the language processor in each object's header, and linker the
	// linker in the image header (15 characters at most).
	compiler  = "GOVAX"
	assembler = "GOVAX MACRO"
	linker    = "GOVAX"
)

// ImageName is the image's name, as its header has it.
const ImageName = "FCALC"

// Build builds FCALC.EXE from the sources in fsys (laid out as
// internal/fcalc's Files are) and returns the image.
func Build(fsys fs.FS) ([]byte, error) {
	order, err := fcalc.Modules(fsys)
	if err != nil {
		return nil, err
	}

	if len(order) == 0 {
		return nil, errors.New(fcalc.OrderFile + ": no modules")
	}

	macros, err := asm.SystemMacros()
	if err != nil {
		return nil, err
	}

	found := moduleFiles{}
	objects := map[string]*obj.Module{}

	for _, name := range order {
		macro, err := compile(fsys, name, found)
		if err != nil {
			return nil, err
		}

		if objects[name], err = assemble(name, macro, macros); err != nil {
			return nil, err
		}
	}

	// The main program first, so the image is named for it and starts at
	// its transfer address.
	main := order[len(order)-1]
	inputs := []link.Input{{File: main + ".OBJ", Module: objects[main]}}

	for _, name := range order[:len(order)-1] {
		inputs = append(inputs, link.Input{File: name + ".OBJ", Module: objects[name]})
	}

	img, err := link.Link(inputs, link.Options{
		ImageName: ImageName,
		Time:      buildTime,
		LinkerID:  linker,
		Traceback: true,
		Sources:   dclcmd.BuiltinSources(nil),
	})
	if err != nil {
		return nil, err
	}

	// Any message from LINK (an undefined symbol, say) is a mistake in the
	// sources.
	for _, m := range img.Messages {
		return nil, fmt.Errorf("LINK: %s", m)
	}

	return img.Bytes, nil
}

// moduleFiles are the module texts compiled so far, by module name
// (sema.ModuleFinder): what the FORTRAN command writes as NAME.F90$MOD.
type moduleFiles map[string][]string

// Find implements sema.ModuleFinder.
func (m moduleFiles) Find(name string) ([]string, string, bool, []string) {
	spec := strings.ToUpper(name) + ".F90$MOD"
	if lines, ok := m[strings.ToUpper(name)]; ok {
		return lines, spec, true, nil
	}

	return nil, "", false, []string{spec}
}

// compile compiles the module name, /G_FLOATING (fcalc's kind needs G's
// range) and /OPTIMIZE, keeps the module texts it made in found, and
// returns its MACRO file. Any message is an error: fcalc's sources draw
// none.
func compile(fsys fs.FS, name string, found moduleFiles) ([]string, error) {
	file := fcalc.SourceFile(name)

	text, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}

	result := fortran.CompileAll(fortran.Options{
		GFloating:   true,
		Optimize:    true,
		Modules:     found,
		Version:     compiler,
		Compiled:    buildTime,
		CommandLine: "FORTRAN/G_FLOATING " + name,
	}, []fortran.File{{
		Spec:  name + ".F90",
		Name:  name,
		Lines: strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"),
		Free:  true,
	}})

	if len(result.Messages) > 0 {
		var list []string
		for _, m := range result.Messages {
			list = append(list, m.String())
		}

		return nil, fmt.Errorf("%s:\n%s", file, strings.Join(list, "\n"))
	}

	for _, m := range result.Modules {
		found[strings.ToUpper(m.Name)] = m.Lines
	}

	return result.MACRO, nil
}

// assemble assembles a module's MACRO file into an object module. A
// warning is an error here, as it is for the compiler.
func assemble(name string, macro []string, macros asm.MacroLibrary) (*obj.Module, error) {
	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)
	a.SetMacroLibraries(macros)

	if _, err := a.Assemble(strings.Join(macro, "\n") + "\n"); err != nil {
		var list *asm.Errors
		if errors.As(err, &list) {
			err = errors.Join(list.List...)
		}

		return nil, fmt.Errorf("%s.MAR: %w", name, err)
	}

	if w := a.Warnings(); len(w) > 0 {
		return nil, fmt.Errorf("%s.MAR: %w", name, errors.Join(w...))
	}

	m, err := a.Object(asm.ObjectOptions{Created: buildTime, Language: assembler})
	if err != nil {
		return nil, fmt.Errorf("%s.MAR: %w", name, err)
	}

	return m, nil
}

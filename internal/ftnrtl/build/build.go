// Package build makes FTNRTL.EXE from its sources (internal/ftnrtl): it
// assembles each MACRO module with internal/asm and links them into a
// shareable image with internal/link, as these commands would:
//
//	$ MACRO FTN_VECTOR
//	$ MACRO FTN_CONTROL
//	$ MACRO FTN_UNITS
//	$ MACRO FTN_LISTOUT
//	$ MACRO FTN_LISTIN
//	$ LINK/SHAREABLE=FTNRTL/NOSYSLIB FTNRTL/OPTIONS
//
// The options file names the modules; each comes from mar/<name>.mar.
// Everything that would vary from build to build (the modules' creation
// time, the link time, the assembler's and linker's names, which carry
// govax's version in the commands) is fixed, so the same sources always
// make the same image, byte for byte, and the committed image can be
// checked against them. /NOSYSLIB keeps VMS's libraries out of it: the
// image's references (LIB$PUT_OUTPUT, the system services) resolve
// through govax's own tables, wherever it's built.
//
// It is a package of its own, not part of internal/ftnrtl, because it
// needs the LINK command's symbol tables (dclcmd.GovaxSymbols), and
// dclcmd needs internal/ftnrtl to find the image.
package build

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/obj"
)

// The fixed names and times a build records.
var (
	// buildTime is each module's creation time and the link time.
	buildTime = time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)

	// assembler names the language processor in each module's header,
	// and linker the linker in the image header (15 characters at most).
	assembler = "govax MACRO"
	linker    = "govax"
)

// Build builds FTNRTL.EXE from the sources in fsys (laid out as
// internal/ftnrtl's Files are) and returns the image.
func Build(fsys fs.FS) ([]byte, error) {
	options, err := readOptions(fsys)
	if err != nil {
		return nil, err
	}

	macros, err := systemMacros()
	if err != nil {
		return nil, err
	}

	var modules []link.Input

	for _, f := range options.Files {
		if f.Library || f.Shareable || len(f.Include) > 0 {
			return nil, fmt.Errorf("%s: %s: only object modules may be named", ftnrtl.OptionsFile, f.Name)
		}

		m, err := assemble(fsys, moduleName(f.Name), macros)
		if err != nil {
			return nil, err
		}

		modules = append(modules, link.Input{File: f.Name, Module: m, Cluster: f.Cluster})
	}

	img, err := link.Link(modules, link.Options{
		ImageName:  ftnrtl.Name,
		Time:       buildTime,
		LinkerID:   linker,
		Traceback:  true,
		Sources:    []link.SymbolSource{dclcmd.GovaxSymbols(nil)},
		Ident:      options.Ident,
		Symbols:    options.Symbols,
		Clusters:   options.Clusters,
		Collects:   options.Collects,
		PsectAttrs: options.PsectAttrs,
		Shareable:  true,
		GSMatch:    options.GSMatch,
		Universals: options.Universals,
	})
	if err != nil {
		return nil, err
	}

	// Any message from LINK (an undefined symbol, say) is a mistake in
	// the sources.
	for _, m := range img.Messages {
		return nil, fmt.Errorf("LINK: %s", m)
	}

	return img.Bytes, nil
}

// readOptions reads the options file.
func readOptions(fsys fs.FS) (*link.OptionsFile, error) {
	text, err := fs.ReadFile(fsys, ftnrtl.OptionsFile)
	if err != nil {
		return nil, err
	}

	p := link.OptionsParser{Shareable: true}

	for _, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		if err := p.Line(line); err != nil {
			return nil, fmt.Errorf("%s: %w", ftnrtl.OptionsFile, err)
		}
	}

	o, err := p.End()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ftnrtl.OptionsFile, err)
	}

	if names := o.Unimplemented(); len(names) > 0 {
		return nil, fmt.Errorf("%s: %s isn't supported", ftnrtl.OptionsFile, strings.Join(names, ", "))
	}

	return o, nil
}

// moduleName is the source's name for an options file's object file
// name: ftn_vector for FTN_VECTOR.OBJ.
func moduleName(file string) string {
	if dot := strings.IndexByte(file, '.'); dot >= 0 {
		file = file[:dot]
	}

	return strings.ToLower(file)
}

// assemble assembles mar/<name>.mar into an object module. A warning is
// an error here: FTNRTL's own sources should draw none.
func assemble(fsys fs.FS, name string, macros asm.MacroLibrary) (*obj.Module, error) {
	file := path.Join(ftnrtl.SourceDir, name+".mar")

	text, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)
	a.SetMacroLibraries(macros)

	if _, err := a.Assemble(string(text)); err != nil {
		var list *asm.Errors
		if errors.As(err, &list) {
			err = errors.Join(list.List...)
		}

		return nil, fmt.Errorf("%s: %w", file, err)
	}

	if w := a.Warnings(); len(w) > 0 {
		return nil, fmt.Errorf("%s: %w", file, errors.Join(w...))
	}

	m, err := a.Object(asm.ObjectOptions{Created: buildTime, Language: assembler})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}

	return m, nil
}

// systemMacros is govax's own system macro library, from bootdata: the
// macros ($EXIT_S, $SSDEF, ...) the sources use.
func systemMacros() (asm.MacroLibrary, error) {
	data, err := fs.ReadFile(bootdata.FS, bootdata.StarletLibrary)
	if err != nil {
		return nil, err
	}

	l, err := lbr.Open(data)
	if err != nil {
		return nil, err
	}

	return asm.NewMacroLibrary(l)
}

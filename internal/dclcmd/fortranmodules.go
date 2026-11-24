package dclcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Module files (docs/planning/PHASE-106 - Fortran 90 modules.md, subtask 5;
// Part A's findings 9 and 12, and the author's answers 6 and 7).
//
// A compilation that compiles a MODULE M without an error writes its text, the
// lines internal/fortran/sema made (sema.ModuleText), as the module file
// M.F90$MOD: in the directory /MODULE=dir names, or else in the default
// directory, where the MACRO file goes (outputLocation's rules). A file whose
// text is the same as the new one but for its SOURCE and COMPILER lines
// (sema.ModuleDigest) is left as it is, so its time, and on a volume its
// version, says when the module last changed: a procedure that rebuilds only
// what changed can rely on it. No module file is written by a compilation with
// an error, as no MACRO file is. /NOMACRO writes no MACRO file but still writes
// module files, as gfortran's -fsyntax-only does: compiling a module for its
// users alone is a reason to give it.
//
// A USE of a module this compilation doesn't compile reads its module file,
// found by fortranModules (sema.ModuleFinder), which looks, in order:
//
//  1. where this compilation writes module files: the /MODULE directory, or the
//     default directory;
//  2. the first source's directory;
//  3. each directory of /MODULE_PATH=(dir,...), or, with no /MODULE_PATH, each
//     translation of the logical name FORT$MODULES;
//  4. govax's definition modules, STARLET_SSDEF and the rest (fortranstarlet.go).
//
// A directory is a host directory or a volume's (rms.Session.Side's rules; a
// logical name stands for its translation). On the host, a file is looked for
// with its name in the case of the first source's type (m.f90$mod for
// prog.f90), then in the other case. When no place has it, the compiler's
// MODNOTFOUND lists every name looked for.

// moduleFileType is a module file's type, and fortModulesName the logical name
// that lists the directories searched after the others when /MODULE_PATH isn't
// given.
const (
	moduleFileType  = "F90$MOD"
	fortModulesName = "FORT$MODULES"
)

// moduleDirectory is one directory of the search: a host directory's path, or
// a volume directory's specification ("" for the default directory).
type moduleDirectory struct {
	host bool
	path string
}

// fortranModules finds the module files of one compilation's USE statements
// (sema.ModuleFinder).
type fortranModules struct {
	c *Context

	// source is the compilation's first source file, opts the command.
	source rms.FileLocation
	opts   FortranOptions

	// dirs are the directories searched, in order, made at the first search,
	// and err what stopped them being made.
	dirs  []moduleDirectory
	made  bool
	dirsE error
}

// moduleFileName is module name's file name in the case a host file of the
// source's should have: lowercase for a source whose type is lowercase, as
// outputFileName gives a host output's type; uppercase on a volume.
func moduleFileName(source rms.FileLocation, name string, other bool) string {
	file := name + "." + moduleFileType

	lower := false
	if source.Host {
		ext := strings.TrimPrefix(filepath.Ext(source.Name), ".")
		lower = ext != "" && strings.ToLower(ext) == ext
	}

	if lower != other {
		return strings.ToLower(file)
	}

	return strings.ToUpper(file)
}

// moduleDirectoryOf is the directory spec names, for /MODULE, /MODULE_PATH, or
// a translation of FORT$MODULES (what, for a message): a host directory, which
// must exist if mustExist, or a volume directory, which must name no file.
func (c *Context) moduleDirectoryOf(spec, what string, mustExist bool) (moduleDirectory, error) {
	loc, err := c.ContainerSession.Side(spec, false)
	if err != nil {
		return moduleDirectory{}, FileFailure(err, spec)
	}

	if loc.Host {
		if mustExist {
			if info, err := os.Stat(loc.Name); err != nil || !info.IsDir() {
				return moduleDirectory{}, vmserrors.New(vmserrors.CLI_FORMODULEDIR, what, loc.Name)
			}
		}

		return moduleDirectory{host: true, path: loc.Name}, nil
	}

	if rest, version := rms.SplitVersion(loc.Name); rms.NameType(rest) != "" || version != "" {
		return moduleDirectory{}, vmserrors.New(vmserrors.CLI_FORMODULEDIR, what, strings.ToUpper(loc.Name))
	}

	return moduleDirectory{path: loc.Name}, nil
}

// moduleOutputDirectory is where a compilation of the source writes module
// files: the /MODULE directory, or the default directory, which for a host
// source, or with no default on a volume, is the host's current one.
func (c *Context) moduleOutputDirectory(opts FortranOptions, source rms.FileLocation) (moduleDirectory, error) {
	if opts.ModuleDir != "" {
		return c.moduleDirectoryOf(opts.ModuleDir, "/MODULE", true)
	}

	if source.Host || !c.ContainerSession.DefaultOnVolume() {
		return moduleDirectory{host: true, path: inCurrentDirectory(".")}, nil
	}

	return moduleDirectory{}, nil
}

// file is the location of the file named name in the directory.
func (d moduleDirectory) file(s *rms.Session, name string) (rms.FileLocation, error) {
	switch {
	case d.host:
		return rms.FileLocation{Host: true, Name: filepath.Join(d.path, name)}, nil
	case d.path == "":
		return s.Locate(name, false)
	}

	return s.LocateRelated(name, false, rms.FileLocation{Name: d.path})
}

// directories makes the search's directories, once: the output directory, the
// source's, then /MODULE_PATH's or FORT$MODULES's, each once.
func (fm *fortranModules) directories() ([]moduleDirectory, error) {
	if fm.made {
		return fm.dirs, fm.dirsE
	}

	fm.made = true

	add := func(d moduleDirectory) {
		for _, have := range fm.dirs {
			if have == d {
				return
			}
		}

		fm.dirs = append(fm.dirs, d)
	}

	out, err := fm.c.moduleOutputDirectory(fm.opts, fm.source)
	if err != nil {
		fm.dirsE = err

		return nil, err
	}

	add(out)

	if fm.source.Host {
		add(moduleDirectory{host: true, path: filepath.Dir(fm.source.Name)})
	} else if name, _ := rms.SplitVersion(fm.source.Name); name != "" {
		add(moduleDirectory{path: strings.TrimSuffix(name, rms.NameType(name))})
	}

	paths := fm.opts.ModulePath
	what := "/MODULE_PATH"

	if len(paths) == 0 {
		paths = fm.c.translations(fortModulesName)
		what = fortModulesName
	}

	for _, p := range paths {
		d, err := fm.c.moduleDirectoryOf(p, what, false)
		if err != nil {
			fm.dirsE = err

			return nil, err
		}

		add(d)
	}

	return fm.dirs, nil
}

// translations is the equivalence strings of the logical name, from the first
// table of the process's search that has it, or none.
func (c *Context) translations(name string) []string {
	s := c.ContainerSession

	db := c.Logicals
	if db == nil && s != nil {
		db = s.Logicals
	}

	if db == nil {
		return nil
	}

	for _, table := range []string{"LNM$PROCESS", "LNM$JOB", "LNM$GROUP", "LNM$SYSTEM"} {
		e, err := db.Translate(table, name, lnm.User, 0)
		if err != nil {
			continue
		}

		values := make([]string, len(e.Equivalences))
		for i, equivalence := range e.Equivalences {
			values[i] = equivalence.Value
		}

		return values
	}

	return nil
}

// Find reads module name's file from the first directory that has one
// (sema.ModuleFinder), returning its lines and its specification; or ok false
// and every name it looked for. A file that is there but can't be read is
// named among them with the reason.
func (fm *fortranModules) Find(name string) ([]string, string, bool, []string) {
	s := fm.c.ContainerSession

	dirs, err := fm.directories()
	if err != nil {
		return nil, "", false, []string{err.Error()}
	}

	var looked []string

	for _, d := range dirs {
		names := []string{moduleFileName(fm.source, name, false)}
		if d.host {
			names = append(names, moduleFileName(fm.source, name, true))
		}

		for _, n := range names {
			loc, err := d.file(s, n)
			if err != nil {
				looked = append(looked, n+" ("+err.Error()+")")

				continue
			}

			records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
			if err == nil {
				return recordLines(records), sourceSpec(found), true, nil
			}

			shown := loc.Name
			if !loc.Host {
				shown = fm.c.expandedName(loc.Name)
			}

			if !isMissingFile(err) {
				shown += " (" + FileFailure(err, shown).Error() + ")"
			}

			looked = append(looked, shown)
		}
	}

	// Last, a definition module to USE (fortranstarlet.go).
	if lines, spec, ok := starletModule(name, fm.opts.GFloating); ok {
		return lines, spec, true, nil
	}

	return nil, "", false, looked
}

// writeModuleFiles writes each module the compilation compiled as its module
// file, in the output directory, unless a file of the same text is there
// already.
func (c *Context) writeModuleFiles(modules []*sema.ModuleText, opts FortranOptions, source rms.FileLocation) error {
	if len(modules) == 0 {
		return nil
	}

	s := c.ContainerSession

	dir, err := c.moduleOutputDirectory(opts, source)
	if err != nil {
		return err
	}

	for _, mt := range modules {
		loc, err := dir.file(s, moduleFileName(source, mt.Name, false))
		if err != nil {
			return FileFailure(err, mt.Name+"."+moduleFileType)
		}

		if loc.Host && !s.MayUseHost() {
			return FileFailure(&rms.DeniedError{Name: loc.Name, Host: true, Output: true}, loc.Name)
		}

		// A file that isn't there, or can't be read, is written (as a MACRO
		// file would be); one of the same text is left alone.
		if old, _, err := s.ReadRecordFile(loc, rms.TextRecords); err == nil && sema.ModuleDigest(recordLines(old)) == mt.Digest {
			continue
		}

		records := make([][]byte, len(mt.Lines))
		for i, line := range mt.Lines {
			records[i] = []byte(line)
		}

		if _, err := s.CreateRecordFile(loc, rms.TextRecords, records); err != nil {
			return FileFailure(err, loc.Name)
		}
	}

	return nil
}

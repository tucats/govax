package sysvol

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/fortran/sysdef"
	"github.com/tucats/govax/internal/ftnrtl"
)

// The system volume's label, and its default device and disk type
// (Decision 0). The label is at most 12 characters, ODS-2's limit.
const (
	Label         = "GOVAXSYS"
	DefaultDevice = "DUA0"
	DefaultType   = "RD54"
)

// The directories on the volume. Root is [SYSTEM], which govax's
// SYS$SYSROOT names; the others are its subdirectories, each the place a
// system logical name leads (STARTUP.COM defines them, Decision 5).
const (
	Root    = "[SYSTEM]"
	SysExe  = "[SYSTEM.SYSEXE]"  // SYS$SYSTEM: the page file, the console ROM
	SysMgr  = "[SYSTEM.SYSMGR]"  // SYS$MANAGER: the startup procedures, the grammars
	SysHlp  = "[SYSTEM.SYSHLP]"  // SYS$HELP: the help libraries
	SysLib  = "[SYSTEM.SYSLIB]"  // SYS$LIBRARY and SYS$SHARE: libraries, FTNRTL.EXE
	Sources = "[SYSTEM.SOURCES]" // SYS$SOURCES: FTNRTL's, the kernel's, and STARLET's sources
)

// HelpSources holds HELPLIB.HLB's help files, and DebugHelpSources, below
// it, DBG$HELP.HLB's (BUILDHELP.COM builds the libraries from them).
const (
	HelpSources      = "[SYSTEM.SYSHLP.SOURCES]"
	DebugHelpSources = "[SYSTEM.SYSHLP.SOURCES.DEBUG]"
)

// Directories lists every directory on the volume, each after its parent,
// the order they are made in.
var Directories = []string{Root, SysExe, SysMgr, SysHlp, HelpSources, DebugHelpSources, SysLib, Sources}

// PageFile is the default page file's place on the volume, by directory
// and name (Decision 8). It isn't one of Files(): the page file's own
// code makes it (internal/rms's pagefile.go), at vax.sysgen.paging.size
// blocks, and nothing in it outlives the session, so it has no contents
// to supply, compare, or refresh.
const PageFile = SysExe + "PAGEFILE.SYS"

// Format is how a file's contents are laid out in records on the volume.
type Format int

const (
	// Text is a text file: variable-length records with carriage-return
	// carriage control, one record per line, as CREATE and EDIT make them.
	Text Format = iota

	// Binary is a file of 512-byte fixed-length records with no carriage
	// control, the form LINK gives an image and LIBRARY a library. Its
	// contents are padded with zeros to a whole number of records.
	Binary
)

// String names a Format as SHOW and the tests do.
func (f Format) String() string {
	switch f {
	case Text:
		return "text"
	case Binary:
		return "binary"
	}

	return fmt.Sprintf("Format(%d)", int(f))
}

// Kind is whose a file is: govax's, kept up to date by later versions of
// govax, or the user's, which govax only starts.
type Kind int

const (
	// Supplied is a file govax supplies: in the manifest, and replaced by
	// a newer govax's copy unless the user has changed it.
	Supplied Kind = iota

	// Seeded is a file govax writes once, if it's missing, and never
	// again: a starting point for the user's own (SYSTARTUP_VMS.COM).
	Seeded
)

// File is one file on the system volume.
type File struct {
	// Directory is one of the directories above, such as SysLib.
	Directory string

	// Name is the file's name and type, uppercase, such as "FTNRTL.EXE".
	Name string

	// Format is how its contents are recorded.
	Format Format

	// Kind is whose it is.
	Kind Kind

	// HelpLibrary marks a help library, which the refresh treats module by
	// module (helplib.go).
	HelpLibrary bool

	// read returns the file's contents as govax carries them, before
	// they're put in their canonical form (Contents).
	read func() ([]byte, error)
}

// Spec is the file's specification on the volume, without a device or
// version: "[SYSTEM.SYSLIB]FTNRTL.EXE".
func (f File) Spec() string {
	return f.Directory + f.Name
}

// Contents returns the file's contents in their canonical form for its
// format (Canonical): what govax writes, and what a manifest's sum is of.
func (f File) Contents() ([]byte, error) {
	data, err := f.read()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Spec(), err)
	}

	return Canonical(data, f.Format), nil
}

// definitionTime is FORSYSDEF.TLB's creation, revision, and insertion
// time, fixed so that the library's bytes don't change unless its modules
// do (as STARLET.MLB's are, internal/bootdata's starletTime). A library
// made with the time of each start would differ every time, and look to
// the manifest like a new version.
var definitionTime = time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)

// Files returns every file on the system volume, in the order Populate
// writes them: directory by directory, as Directories lists them.
func Files() []File {
	files := []File{
		{Directory: SysExe, Name: "DEFAULT.ROM", Format: Binary, read: boot("default.rom")},

		{Directory: SysMgr, Name: "STARTUP.COM", Format: Text, read: boot("startup.com")},
		{Directory: SysMgr, Name: "SYSTARTUP_VMS.COM", Format: Text, Kind: Seeded, read: boot("systartup_vms.com")},
		{Directory: SysMgr, Name: "CONSOLE.DCL", Format: Text, read: boot("console.dcl")},
		{Directory: SysMgr, Name: "DEBUG.DCL", Format: Text, read: boot("debug.dcl")},

		{Directory: SysHlp, Name: bootdata.HelpLibraryName(bootdata.HelpLibrary), Format: Binary, HelpLibrary: true, read: boot(bootdata.HelpLibrary)},
		{Directory: SysHlp, Name: bootdata.HelpLibraryName(bootdata.DebugHelpLibrary), Format: Binary, HelpLibrary: true, read: boot(bootdata.DebugHelpLibrary)},

		{Directory: SysLib, Name: strings.ToUpper(bootdata.StarletLibrary), Format: Binary, read: boot(bootdata.StarletLibrary)},
		{Directory: SysLib, Name: "FTNRTL.EXE", Format: Binary, read: ftnrtlFile(ftnrtl.ImageFile)},
		{Directory: SysLib, Name: "FORSYSDEF.TLB", Format: Binary, read: func() ([]byte, error) {
			return sysdef.Library(definitionTime)
		}},
	}

	// The help libraries' sources, which BUILDHELP.COM builds them from.
	files = append(files, helpSources(HelpSources, bootdata.HelpSourceDir)...)
	files = append(files, helpSources(DebugHelpSources, bootdata.DebugHelpSourceDir)...)

	// FTNRTL's sources: every module, then the options that link them, and
	// the procedure that builds the library from them.
	for _, name := range ftnrtlSources() {
		files = append(files, File{
			Directory: Sources,
			Name:      strings.ToUpper(path.Base(name)),
			Format:    Text,
			read:      ftnrtlFile(name),
		})
	}

	files = append(files,
		File{Directory: Sources, Name: strings.ToUpper(ftnrtl.OptionsFile), Format: Text, read: ftnrtlFile(ftnrtl.OptionsFile)},
		File{Directory: Sources, Name: "BUILDFTN.COM", Format: Text, read: boot("buildftn.com")},

		// The microkernel's sources, which STARTUP.COM assembles.
		File{Directory: Sources, Name: "KERNEL.ASM", Format: Text, read: boot("kernel.asm")},
		File{Directory: Sources, Name: "SSDEF.ASM", Format: Text, read: boot("ssdef.asm")},

		// STARLET's sources, and the procedure that builds the library.
		File{Directory: Sources, Name: strings.ToUpper(bootdata.StarletSource), Format: Text, read: boot(bootdata.StarletSource)},
		File{Directory: Sources, Name: strings.ToUpper(bootdata.StarletDefSource), Format: Text, read: boot(bootdata.StarletDefSource)},
		File{Directory: Sources, Name: "BUILDSTARLET.COM", Format: Text, read: boot("buildstarlet.com")},

		// The procedure that builds the help libraries from their sources.
		File{Directory: Sources, Name: "BUILDHELP.COM", Format: Text, read: boot("buildhelp.com")},
	)

	// Directory by directory, in Directories' order; within one, as above.
	order := map[string]int{}
	for i, d := range Directories {
		order[d] = i
	}

	sort.SliceStable(files, func(i, j int) bool { return order[files[i].Directory] < order[files[j].Directory] })

	return files
}

// helpSources are the system volume's copies, in the directory dir, of the
// help files in bootdata's directory embedded, sorted.
func helpSources(dir, embedded string) []File {
	entries, err := fs.ReadDir(bootdata.FS, embedded)
	if err != nil {
		panic("sysvol: " + err.Error()) // embedded, so there by construction
	}

	var files []File

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".hlp") {
			files = append(files, File{
				Directory: dir,
				Name:      strings.ToUpper(e.Name()),
				Format:    Text,
				read:      boot(path.Join(embedded, e.Name())),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return files
}

// Resolved maps the names govax reads some of its built-in files by, through
// its resolver (internal/respath), to the system volume's copies that stand
// in for them (Decision 4): the grammars and the help files, read as govax
// starts. A copy on the host (in the current directory or a --path
// directory) still comes first, and govax's own last. (The help libraries
// aren't here: HELP finds SYS$HELP's through RMS, internal/dclcmd's help.go.)
var Resolved = map[string]string{
	"console.dcl": SysMgr + "CONSOLE.DCL",
	"debug.dcl":   SysMgr + "DEBUG.DCL",
}

// Lookup returns the file whose specification is spec (in any case), and
// true, or false if no file on the volume has it.
func Lookup(spec string) (File, bool) {
	for _, f := range Files() {
		if strings.EqualFold(f.Spec(), spec) {
			return f, true
		}
	}

	return File{}, false
}

// boot returns a reader of one of internal/bootdata's files.
func boot(name string) func() ([]byte, error) {
	return func() ([]byte, error) { return fs.ReadFile(bootdata.FS, name) }
}

// ftnrtlFile returns a reader of one of internal/ftnrtl's files.
func ftnrtlFile(name string) func() ([]byte, error) {
	return func() ([]byte, error) { return ftnrtl.Files.ReadFile(name) }
}

// ftnrtlSources returns the names, in ftnrtl.Files, of FTNRTL's MACRO
// modules, sorted. They are read from the embedded directory, so a
// module added to the library is on the volume without a change here.
func ftnrtlSources() []string {
	entries, err := ftnrtl.Files.ReadDir(ftnrtl.SourceDir)
	if err != nil {
		panic("sysvol: " + err.Error()) // embedded, so there by construction
	}

	names := make([]string, 0, len(entries))

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".mar") {
			names = append(names, path.Join(ftnrtl.SourceDir, e.Name()))
		}
	}

	sort.Strings(names)

	return names
}

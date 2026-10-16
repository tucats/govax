package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
)

// The page file (docs/PHASE-64 - page file.md, subtask 2; the paging
// plan's Decision 9).
//
// With paging on, a page that leaves a working set modified goes to the
// modified page list, and its contents must be written somewhere before
// its frame can be used again. A section page goes back to its section's
// file (mpw.go). A page with no file of its own, a demand-zero page a
// program has written or a private copy of a copy-on-reference page, goes
// to the page file: on VMS, SYS$SYSTEM:PAGEFILE.SYS; in govax, the file
// vax.sysgen.paging.file names, on a mounted volume or the host, of
// vax.sysgen.paging.size blocks. Each block holds one page. Which blocks
// are in use is kept in Go (internal/paging's PageFileMap), and nothing
// in the file outlives the System that wrote it.
//
// The file isn't opened when the System is made: govax's default volume
// is mounted only after vax.init has run VMINIT. It's opened the first
// time the pager needs it (OpenPageFile). If it can't be, the console
// is told once, and paging goes on without a page file: pages bound for
// it stay on the modified page list (the plan's Decision 7).
//
// Opening it may create it, which changes the volume's storage bitmap
// and directory. That mustn't happen in the middle of an RMS operation
// that's changing them too, as a page fault inside an RMS service could
// be, so OpenPageFile must be called at an instruction boundary (where
// no service is running), as the reclaimer is. Reading and writing its
// blocks afterward is safe anywhere (internal/rms's pagefile.go).

// DefaultPageFileSize is vax.sysgen.paging.size's default: the page
// file's size, in blocks, when it's created (16MB, enough for two jobs
// at PQL_DPGFLQUOTA's default).
const DefaultPageFileSize = 32768

// DefaultMPWWriteCluster is MPW_WRTCLUSTER's default, VMS's for a VAX:
// the most pages the modified page writer writes to the page file at
// once, and so the largest run of blocks the page file's map hands out.
const DefaultMPWWriteCluster = 120

// pageFileState is how far the System has got with its page file.
type pageFileState uint8

const (
	pageFileUnopened pageFileState = iota // not asked for yet
	pageFileOpen                          // open: System.pageFile
	pageFileFailed                        // couldn't be opened, or none configured
	pageFileClosed                        // closed, with its System
)

// PageFile is the System's open page file: its blocks, and the map of
// which ones are in use.
type PageFile struct {
	store *rms.PageFileStore

	// Map is the page file's allocation map.
	Map *paging.PageFileMap
}

// Name is the page file's full name: a volume file's specification with
// its version, or a host file's path.
func (pf *PageFile) Name() string { return pf.store.Name }

// Size is the page file's size in blocks.
func (pf *PageFile) Size() uint32 { return pf.store.Blocks() }

// Created reports that the page file was made when it was opened.
func (pf *PageFile) Created() bool { return pf.store.Created }

// Read reads block vbn of the page file into buf, a page long.
func (pf *PageFile) Read(vbn uint32, buf []byte) error { return pf.store.ReadBlock(vbn, buf) }

// Write writes a page, data, to block vbn of the page file. It never
// writes past the file's end.
func (pf *PageFile) Write(vbn uint32, data []byte) error { return pf.store.WriteBlock(vbn, data) }

// PageFile returns the System's page file if it's open, else nil.
func (sys *System) PageFile() *PageFile {
	if sys.pageFileState != pageFileOpen {
		return nil
	}

	return sys.pageFile
}

// OpenPageFile returns the System's page file, opening it the first time
// it's asked for: creating it if it's missing, and holding it so that no
// one else may read or write it. It returns nil with paging off, with no
// page file configured, or when the file can't be opened, which the
// console is told about once. Call it at an instruction boundary (this
// file's opening comment).
func (sys *System) OpenPageFile() *PageFile {
	if sys.pageFileState != pageFileUnopened {
		return sys.PageFile()
	}

	sys.pageFileState = pageFileFailed

	if !sys.Sysgen.Paging || sys.Sysgen.PageFileName == "" {
		return nil
	}

	size := sys.Sysgen.PageFileSize
	if size == 0 {
		size = DefaultPageFileSize
	}

	store, err := sys.pageFileSession().OpenPageFile(sys.Sysgen.PageFileName, size)
	if err != nil {
		sys.consoleNotice(fmt.Sprintf("%%GOVAX-W-NOPAGEFILE, page file %s not opened; paging continues without one\n-GOVAX-W-REASON, %v\n",
			sys.Sysgen.PageFileName, err))

		return nil
	}

	sys.pageFile = &PageFile{
		store: store,
		Map:   paging.NewPageFileMap(store.Blocks(), sys.Sysgen.MPWWriteCluster),
	}
	sys.pageFileState = pageFileOpen

	return sys.pageFile
}

// pageFileSession is the RMS session a page file's name is resolved in:
// process 1's, with the console's logical names and default directory,
// or a bare one on the System's mounts.
func (sys *System) pageFileSession() *rms.Session {
	if procs := sys.Processes(); len(procs) > 0 && procs[0].Session != nil {
		return procs[0].Session
	}

	return rms.NewSession(sys.Mounts)
}

// ClosePageFile closes the System's page file, if it's open, so that its
// volume can be dismounted. The console calls it before it replaces the
// System (INIT, VMINIT, ZERO) and at the session's end. The System has no
// page file afterward.
func (sys *System) ClosePageFile() error {
	state := sys.pageFileState
	sys.pageFileState = pageFileClosed

	if state != pageFileOpen {
		return nil
	}

	pf := sys.pageFile
	sys.pageFile = nil

	return pf.store.Close()
}

// AllocatePageFile takes a run of page file blocks for up to want pages
// (paging.PageFileMap.Allocate), telling the console when the file first
// passes 65% and 90% full. It reports false when there's no page file or
// it's full.
func (sys *System) AllocatePageFile(want uint32) (paging.Run, bool) {
	pf := sys.PageFile()
	if pf == nil {
		return paging.Run{}, false
	}

	run, warning, ok := pf.Map.Allocate(want)

	switch warning {
	case paging.PageFrag:
		sys.consoleNotice("%SYSTEM-W-PAGEFRAG, page file filling up; please create more space\n")
	case paging.PageCrit:
		sys.consoleNotice("%SYSTEM-W-PAGECRIT, page file nearly full; system trying to continue\n")
	}

	return run, ok
}

// FreePageFile gives a run of page file blocks back.
func (sys *System) FreePageFile(run paging.Run) error {
	pf := sys.PageFile()
	if pf == nil {
		return fmt.Errorf("corevms: freeing page file blocks %d to %d with no page file open", run.First, run.Last())
	}

	return pf.Map.Free(run)
}

// consoleNotice writes a message for the operator on the console:
// process 1's terminal.
func (sys *System) consoleNotice(text string) {
	if procs := sys.Processes(); len(procs) > 0 {
		procs[0].writeConsole(text)
	}
}

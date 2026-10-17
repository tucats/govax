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
// On VMS the page file is made when the system is installed (or by
// AUTOGEN) and installed at every boot, so its blocks are the system's
// before anything else can take them, and a volume without room for it
// is known at once. govax does the same by installing it when the
// system starts (InstallPageFile, subtask 8): the console calls it once
// vax.init has run, before any of the user's commands, and for each new
// System after that (VMINIT, INIT, ZERO). Installing creates the file if
// it's missing and makes it vax.sysgen.paging.size blocks if it isn't
// (internal/rms's pagefile.go). The System can't install it when it's
// made, during vax.init: a volume vax.init mounts after its VMINIT isn't
// there yet.
//
// With vax.sysgen.paging.defer, or before the system has started, the
// file is opened instead the first time the pager needs it
// (OpenPageFile), as it was before subtask 8. Either way, if it can't
// be opened the console is told once, and paging goes on without a page
// file: pages bound for it stay on the modified page list (the plan's
// Decision 7).
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

	switch {
	case store.GrowFailed != nil:
		sys.consoleNotice(fmt.Sprintf("%%GOVAX-W-PAGEFILESIZE, page file %s kept at %d blocks; it couldn't grow to %d\n-GOVAX-W-REASON, %v\n",
			store.Name, store.Blocks(), size, store.GrowFailed))
	case store.Resized != 0:
		sys.consoleNotice(fmt.Sprintf("%%GOVAX-I-PAGEFILESIZE, page file %s changed from %d to %d blocks\n",
			store.Name, store.Resized, store.Blocks()))
	}

	sys.pageFile = &PageFile{
		store: store,
		Map:   paging.NewPageFileMap(store.Blocks(), sys.Sysgen.MPWWriteCluster),
	}
	sys.pageFileState = pageFileOpen

	return sys.pageFile
}

// InstallPageFile opens the System's page file as the system starts
// (this file's opening comment): with paging on, a page file named, and
// vax.sysgen.paging.defer not set; otherwise it does nothing, and the
// file waits for the pager's first need. It returns the page file, or
// nil.
func (sys *System) InstallPageFile() *PageFile {
	if !sys.Sysgen.Paging || sys.Sysgen.PageFileName == "" || sys.Sysgen.PageFileDefer {
		return nil
	}

	return sys.OpenPageFile()
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

	if err := pf.Map.Free(run); err != nil {
		return err
	}

	// Space again in a page file that was full: the writer may now write
	// the pages it passed over, so its counts of them are forgotten.
	if sys.pageFileFull {
		sys.pageFileFull = false
		sys.mpwStuck, sys.reclaimStuck = 0, 0
	}

	return nil
}

// Exhaustion (docs/PHASE-64 - page file.md, "Design notes"; the
// author's rule, 2026-10-15). When the page file is full, pages bound for
// it stay on the modified list, and when no frame is left, a fault fails,
// as it does when physical memory runs out without paging: no process
// waits for space (VMS's RWMPE and RWMPB states aren't modeled). Each is
// told to the console: a full page file once, as PAGEFRAG and PAGECRIT
// are (a file nearly full fills and empties by a few blocks at a time),
// and a failed fault once until a fault succeeds again.

// notePageFileFull records that the page file is full, telling the
// console the first time.
func (sys *System) notePageFileFull() {
	pf := sys.PageFile()
	if pf == nil {
		return
	}

	sys.pageFileFull = true

	if sys.pageFileFullTold {
		return
	}

	sys.pageFileFullTold = true
	sys.consoleNotice(fmt.Sprintf("%%GOVAX-W-PAGEFILEFULL, page file %s is full; modified pages stay in memory\n", pf.Name()))
}

// noteFault records how a page fault at addr ended: one that failed with
// paging on and no free frame tells the console, once until a fault
// succeeds again, that memory is exhausted, and why the modified pages
// couldn't make room.
func (sys *System) noteFault(addr uint32, ok bool) {
	if ok {
		sys.memoryOut = false

		return
	}

	db := sys.mem.Paging()
	if db == nil || db.FreeCount() > 0 || sys.memoryOut {
		return
	}

	sys.memoryOut = true

	text := fmt.Sprintf("%%GOVAX-F-MEMEXHAUST, physical memory exhausted; page fault at %08X not resolved\n", addr)

	if db.ModifiedCount() > 0 {
		switch {
		case sys.pageFileFull:
			text += "-GOVAX-I-PAGEFILEFULL, the page file is full\n"
		case sys.PageFile() == nil:
			text += "-GOVAX-I-NOPAGEFILE, there is no page file to write modified pages to\n"
		}
	}

	sys.consoleNotice(text)
}

// consoleNotice writes a message for the operator on the console:
// process 1's terminal.
func (sys *System) consoleNotice(text string) {
	if procs := sys.Processes(); len(procs) > 0 {
		procs[0].writeConsole(text)
	}
}

// Reporting (subtask 7): what SHOW MEMORY and $GETSYI say about the page
// file.

// PageFileStatus is where the System's page file stands, for a report.
type PageFileStatus uint8

const (
	// PageFileNone: paging is off, or no page file is configured.
	PageFileNone PageFileStatus = iota

	// PageFileNotOpened: one is configured, but nothing has needed it yet.
	PageFileNotOpened

	// PageFileInUse: it's open.
	PageFileInUse

	// PageFileUnavailable: it couldn't be opened (the console was told
	// why), or it was closed with its System.
	PageFileUnavailable
)

// PageFileReport describes the System's page file and its use.
type PageFileReport struct {
	Status PageFileStatus

	// Name is the open file's full name, or the configured one when it
	// isn't open.
	Name string

	// Size, Free, and Used count its blocks (each holds a page), and
	// AllocationSize is the run of blocks its map looks for now. All are
	// 0 unless it's open.
	Size, Free, Used, AllocationSize uint32

	// Warned is the highest use threshold reported (PAGEFRAG, PAGECRIT),
	// and Full whether the writer last found it full.
	Warned paging.Warning
	Full   bool

	// Charged is the pages charged against the PGFLQUOTA of every job,
	// the page file's committed use, which may be more or less than Used:
	// a page is charged when it's made, and takes a block only when the
	// writer writes it.
	Charged uint64
}

// PageFileReport reports on the System's page file. It never opens it.
func (sys *System) PageFileReport() PageFileReport {
	r := PageFileReport{Name: sys.Sysgen.PageFileName, Charged: sys.pageFileCommitted()}

	switch {
	case sys.pageFileState == pageFileOpen:
		m := sys.pageFile.Map
		r.Status = PageFileInUse
		r.Name = sys.pageFile.Name()
		r.Size, r.Free, r.Used = m.Size(), m.FreeBlocks(), m.UsedBlocks()
		r.AllocationSize, r.Warned, r.Full = m.AllocationSize(), m.Warned(), sys.pageFileFull

	case !sys.Sysgen.Paging || sys.Sysgen.PageFileName == "":
		r.Status = PageFileNone

	case sys.pageFileState == pageFileUnopened:
		r.Status = PageFileNotOpened

	default:
		r.Status = PageFileUnavailable
	}

	return r
}

// pageFileCommitted is the total of every job's pages charged against
// its PGFLQUOTA.
func (sys *System) pageFileCommitted() uint64 {
	seen := map[*Job]bool{}

	var total uint64

	for _, env := range sys.Processes() {
		if env.Process == nil {
			continue
		}

		if job := env.Process.Job; job != nil && !seen[job] {
			seen[job] = true
			total += uint64(job.PageFileCharged)
		}
	}

	return total
}

// pageFileSizes are $GETSYI's SYI$_PAGEFILE_PAGE and PAGEFILE_FREE: the
// page file's size and its free blocks, in pages. On VMS the page file is
// installed when the system starts, so a program always sees it; govax
// opens it at first need, and a program asking about it is one (a service
// runs between instructions, where opening is safe). With none, both
// are 0.
func (sys *System) pageFileSizes() (size, free uint32) {
	pf := sys.OpenPageFile()
	if pf == nil {
		return 0, 0
	}

	return pf.Map.Size(), pf.Map.FreeBlocks()
}

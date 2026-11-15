package corevms

import (
	"sort"
	"strings"
	"sync"
)

// The known file list (docs/planning/PHASE-94 - privileges.md, Decision 9):
// the images INSTALL has made "known" to the system, each with the
// attributes it was installed with.
//
// # Installed images and privileges
//
// An ordinary image runs with the privileges its process has. VMS lets a
// system manager *install* an image with privileges of its own (INSTALL
// ADD file/PRIVILEGED=(SYSNAM,OPER)): while any process runs it, those
// privileges are the image's privilege mask, IMAGPRIV, and are enabled in
// the process's current mask too, CURPRIV = PROCPRIV | IMAGPRIV. When the
// image exits, rundown copies PROCPRIV back into CURPRIV, and they're gone.
// So an installed image is the one way an unprivileged user's process
// gains a privilege it isn't authorized for: the image is trusted to use
// it only as it should (VMS's AUTHORIZE, SET PASSWORD, and SHOW are such
// images).
//
// INSTALL itself needs CMKRNL (ruleInstall), since it changes what every
// process's image activator does; INSTALL LIST needs nothing. Each entry
// is found by its file's physical name without the version, and the
// activator gives an image an entry's privileges only when the file it
// found is the version that was installed (unconfirmed: a newer version
// of an installed image, made after INSTALL, runs without them here).
//
// The list is in Go, not in VAX memory: VMS keeps it as known file entries
// (KFEs) in paged pool, which no user-mode program reads.

// KnownFile is one entry of the known file list.
type KnownFile struct {
	// Name is the file's physical specification without its version,
	// as the image activator records an image's name
	// ("DUA1:[000000]PRIV94I.EXE"): the entry's key.
	Name string

	// Version is the version that was installed.
	Version uint16

	// Volume is the volume's logical name, DISK$label, or its device
	// when it has no label; Directory is the directory without its
	// brackets ("000000"); File and Type the name's parts. INSTALL
	// LIST shows them.
	Volume, Directory, File, Type string

	// Privileges is the image's privilege mask, IMAGPRIV while it runs.
	Privileges uint64

	// Attributes are the qualifiers it was installed with, by name
	// (OPEN, HEADER_RESIDENT, SHARED, PROTECTED, WRITEABLE, ...):
	// recorded and shown, but only /PRIVILEGED changes what govax does.
	Attributes map[string]bool

	// Accesses is how many times the image activator has run it.
	Accesses uint32
}

// PrivilegeNames is the names of the entry's privileges, in $PRVDEF's
// order, as INSTALL LIST/FULL shows them.
func (kf KnownFile) PrivilegeNames() []string {
	var names []string

	for bit := range 64 {
		if kf.Privileges&(1<<bit) == 0 {
			continue
		}

		if name, ok := privilegeName(bit); ok {
			names = append(names, name)
		}
	}

	return names
}

// KnownFileList is the System's known file list, shared by every process
// (a subprocess's INSTALL and process 1's RUN see the same entries).
type KnownFileList struct {
	mu      sync.Mutex
	entries map[string]*KnownFile
}

// NewKnownFileList returns an empty list.
func NewKnownFileList() *KnownFileList {
	return &KnownFileList{entries: map[string]*KnownFile{}}
}

// KnownFileKey is the key of the file whose physical specification is
// spec, with or without a version: the specification in upper case,
// without a leading "_" or its version.
func KnownFileKey(spec string) string {
	spec = strings.ToUpper(strings.TrimPrefix(spec, "_"))
	if i := strings.LastIndexByte(spec, ';'); i >= 0 && i > strings.LastIndexAny(spec, "]>") {
		spec = spec[:i]
	}

	return spec
}

// Add puts kf on the list, reporting false (and changing nothing) if its
// file is already there.
func (l *KnownFileList) Add(kf KnownFile) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := KnownFileKey(kf.Name)
	if _, exists := l.entries[key]; exists {
		return false
	}

	kf.Name = key
	l.entries[key] = &kf

	return true
}

// Remove takes the entry for the file named spec off the list, reporting
// whether there was one.
func (l *KnownFileList) Remove(spec string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := KnownFileKey(spec)
	if _, exists := l.entries[key]; !exists {
		return false
	}

	delete(l.entries, key)

	return true
}

// Find returns a copy of the entry for the file named spec.
func (l *KnownFileList) Find(spec string) (KnownFile, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	kf, ok := l.entries[KnownFileKey(spec)]
	if !ok {
		return KnownFile{}, false
	}

	return *kf, true
}

// Entries returns copies of every entry, in INSTALL LIST's order: by
// volume, directory, and type (LIST's headings), then by name.
func (l *KnownFileList) Entries() []KnownFile {
	l.mu.Lock()
	defer l.mu.Unlock()

	list := make([]KnownFile, 0, len(l.entries))
	for _, kf := range l.entries {
		list = append(list, *kf)
	}

	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Volume != b.Volume {
			return a.Volume < b.Volume
		}

		if a.Directory != b.Directory {
			return a.Directory < b.Directory
		}

		if a.Type != b.Type {
			return a.Type < b.Type
		}

		return a.File < b.File
	})

	return list
}

// activated is the image activator's look at the list for the image
// whose file's physical specification is spec: the entry's privileges if
// it was installed and spec is its installed version (counting the
// access), and 0 otherwise.
func (l *KnownFileList) activated(spec string) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	kf, ok := l.entries[KnownFileKey(spec)]
	if !ok || specVersion(spec) != kf.Version {
		return 0
	}

	kf.Accesses++

	return kf.Privileges
}

// specVersion is the version number at the end of a file specification,
// or 0 if it has none.
func specVersion(spec string) uint16 {
	i := strings.LastIndexByte(spec, ';')
	if i < 0 || i < strings.LastIndexAny(spec, "]>") {
		return 0
	}

	var v uint16

	for _, ch := range spec[i+1:] {
		if ch < '0' || ch > '9' {
			return 0
		}

		v = v*10 + uint16(ch-'0')
	}

	return v
}

// StartImage is the image activator's last step for the process's main
// image, found in the file whose physical specification is name: the
// process's JPI$_IMAGNAME (SetImageName), and its privileges. An image on
// the known file list runs with IMAGPRIV its installed privileges and
// CURPRIV = PROCPRIV | IMAGPRIV, as the System Services manual's $SETPRV
// entry says and VMS 7.1 showed (testdata/priv94's INSTALL sections);
// image rundown takes them away (resetImagePrivileges). Any other image
// has IMAGPRIV 0 and the privileges the process has.
func (env *Environment) StartImage(name string) {
	env.SetImageName(name)

	p := env.Process
	p.ImagePrivileges = 0

	if env.System == nil || env.KnownFiles == nil || name == "" {
		return
	}

	if priv := env.KnownFiles.activated(name); priv != 0 {
		p.ImagePrivileges = priv
		p.CurrentPrivileges = p.ProcessPrivileges | priv
	}
}

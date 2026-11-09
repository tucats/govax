package rms

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/ods2/filespec"
)

// This file is RMS's name processing (docs/PHASE-33.md, subtask 2): how
// $PARSE, $OPEN, $CREATE, and the rest turn a FAB's file name into the
// expanded file specification. Written from the OpenVMS RMS Reference
// Manual (FAB$L_DNA, NAM$L_RLF, FAB$V_OFP, and Table 5-2's NAM$L_FNB).
// It is the only name processor: the DCL utilities reach it through
// logicals.go's expandSpec (docs/PHASE-91, Decision 4).
//
// A specification is filled in from several sources, each supplying only
// the fields the ones before it left out:
//
//  1. the primary name (FAB$L_FNA), after its logical name is translated
//     (a field both the program and the translation give is an error);
//  2. the default name (FAB$L_DNA), translated the same way;
//  3. the related file's resultant string (NAM$L_RLF), for the device,
//     directory, name, and type (with FAB$V_OFP, the directory, name,
//     and type only);
//  4. the process defaults: SYS$DISK's device and the default directory.
//
// A relative directory ("[.SUB]", "[-]") is applied to the directory the
// later sources give. A field no source gives is empty: no name, a type
// of ".", and a version of ";".
//
// A device can bring a root with it: a directory ending in a period,
// "_DUA1:[USERS.SMITH.]", which then looks like the volume's master file
// directory (docs/PHASE-91, Decision 2; the Guide to OpenVMS File
// Applications, 6.3). A name is a device, a root, and a directory
// *relative to the root*, so nothing here can climb out of a root, and
// only parsedName.volumeDir puts the two together for the volume.

// fileName is a file specification's fields as name processing keeps
// them: the text of each, with its delimiters and as written ("DUA1:",
// "[USERS.]", "[TEST.SUB]", "<TEST>", "A", ".DAT", ";3"). An empty field
// is one not given; a given but empty type or version is "." or ";".
type fileName struct {
	Dev, Dir, Name, Type, Ver string

	// Root is the root, if there is one. While a name is being put together
	// Dir holds only the directory; in a finished name (finishName) Dir has
	// the root before it when the root is shown.
	Root string
}

// maxNameLen is the longest file name or type ODS-2 allows.
const maxNameLen = 39

// maxDirDepth is how many directory levels RMS allows below the MFD.
const maxDirDepth = 8

// maxRootDepth is how many levels a root may have (VMS 7.1: testdata/lnm91,
// DEEP7 and DEEP8). The directory below it may have maxDirDepth more.
const maxRootDepth = 7

// ellipsis is the directory element "...": this directory and every one
// below it.
const ellipsis = "..."

// isNameChar reports whether c can appear in a name, type, or directory
// element: a letter, a digit, "$", "_", "-", or a wildcard.
func isNameChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '$' || c == '_' || c == '-' || c == '*' || c == '%'
}

// validName reports whether s is all name characters and no longer than
// a name may be.
func validName(s string) bool {
	if len(s) > maxNameLen {
		return false
	}

	for i := range len(s) {
		if !isNameChar(s[i]) {
			return false
		}
	}

	return true
}

// scanName splits text into its fields. A field that isn't well formed
// gives the RMS status for it instead: RMS$_DEV, RMS$_DIR, RMS$_FNM,
// RMS$_TYP, RMS$_VER, or RMS$_SYN. A version after a second "." must be
// a number ("A.B.C" is RMS$_SYN, as VMS has it); one after ";" that isn't
// is RMS$_VER. Node names (DECnet) aren't supported, and give RMS$_SYN.
func scanName(text string) (fileName, uint32) {
	return scanFields(text, rmsSyntaxError)
}

// opensDir reports whether c begins a directory.
func opensDir(c byte) bool {
	return c == '[' || c == '<'
}

// nextDir splits s, which begins with a directory, into that directory's
// text without its brackets and what follows the directory. ok is false
// when it has no closing bracket.
func nextDir(s string) (body, rest string, ok bool) {
	end := byte(']')
	if s[0] == '<' {
		end = '>'
	}

	k := strings.IndexByte(s, end)
	if k < 0 {
		return "", "", false
	}

	return s[1:k], s[k+1:], true
}

// isRoot reports whether a directory's text (without its brackets) is a
// root: it ends in a period that isn't part of an ellipsis.
func isRoot(body string) bool {
	return strings.HasSuffix(body, ".") && !strings.HasSuffix(body, ellipsis)
}

// scanFields is scanName, with secondRoot the status for a directory after
// a root that is itself a root: RMS$_SYN when it was typed, and RMS$_DEV in
// a logical name's translation (VMS 7.1: testdata/lnm91, "[A.][B.]" and
// A2 = BASE:[SUB.]).
func scanFields(text string, secondRoot uint32) (fileName, uint32) {
	s := strings.ToUpper(text)

	var f fileName

	if j := strings.IndexByte(s, ':'); j >= 0 && !strings.ContainsAny(s[:j], "[<") {
		if strings.HasPrefix(s[j:], "::") {
			return f, rmsSyntaxError
		}

		dev := strings.TrimPrefix(s[:j], "_")
		if dev == "" || !validName(dev) || strings.ContainsAny(dev, "*%") {
			return f, rmsDeviceError
		}

		f.Dev, s = s[:j+1], s[j+1:]
	}

	if s != "" && opensDir(s[0]) {
		// A directory with no closing bracket ("[[[") is a syntax error
		// (VMS 7.1: testdata/lib58's FS bad default name).
		body, rest, ok := nextDir(s)
		if !ok {
			return f, rmsSyntaxError
		}

		if isRoot(body) {
			if _, sts := scanRoot(body); sts != 0 {
				return f, sts
			}

			f.Root, s = s[:len(s)-len(rest)], rest

			if s != "" && opensDir(s[0]) {
				if body, rest, ok = nextDir(s); !ok {
					return f, rmsSyntaxError
				}

				if isRoot(body) {
					return f, secondRoot
				}
			}
		}

		if s != "" && opensDir(s[0]) {
			if _, ok := scanDir(body); !ok {
				return f, rmsDirError
			}

			f.Dir, s = s[:len(s)-len(rest)], rest
		}
	}

	k := strings.IndexAny(s, ".;")
	if k < 0 {
		k = len(s)
	}

	if !validName(s[:k]) {
		// A directory after the name is a syntax error (VMS 7.1:
		// testdata/for81, FOR81R's "A[B").
		if strings.ContainsAny(s[:k], "[<") {
			return f, rmsSyntaxError
		}

		return f, rmsFileNameError
	}

	f.Name, s = s[:k], s[k:]

	if strings.HasPrefix(s, ".") {
		k = strings.IndexAny(s[1:], ".;")
		if k < 0 {
			k = len(s) - 1
		}

		if !validName(s[1 : k+1]) {
			return f, rmsTypeError
		}

		f.Type, s = s[:k+1], s[k+1:]
	}

	if s != "" {
		v := s[1:]
		if !validVersion(v) {
			// A version that isn't a number at all (";Q") is a syntax
			// error, one out of range RMS$_VER (VMS 7.1: testdata/lib58's
			// FF bad syntax).
			if s[0] == '.' || !numericVersion(v) {
				return f, rmsSyntaxError
			}

			return f, rmsInvalidVersion
		}

		f.Ver, s = ";"+v, ""
	}

	return f, 0
}

// numericVersion reports whether v is a number, with or without a
// minus sign.
func numericVersion(v string) bool {
	v = strings.TrimPrefix(v, "-")
	if v == "" {
		return false
	}

	for i := range len(v) {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
	}

	return true
}

// validVersion reports whether v (the text after ";") is a version name
// processing accepts: none, "*", a number, or a negative one (that many
// versions below the highest). VMS's $PARSE takes ";32768" (the oracle's
// PARSE case 24), so the bound here is a word's.
func validVersion(v string) bool {
	if v == "" || v == "*" {
		return true
	}

	n, err := strconv.Atoi(v)

	return err == nil && n >= -32767 && n <= 65535 && v[0] != '+'
}

// dirSpec is a directory's elements: names, "..." for an ellipsis, and,
// in a relative directory, leading "-" for each level up.
type dirSpec struct {
	// Relative is true for a directory applied to another ("[.SUB]",
	// "[-]", "[]", "[...]").
	Relative bool

	Elems []string
}

// scanDir splits a directory's text (without its brackets) into elements,
// reporting whether it's well formed.
func scanDir(body string) (dirSpec, bool) {
	var d dirSpec

	if body == "" {
		return dirSpec{Relative: true}, true
	}

	if strings.HasPrefix(body, ".") && !strings.HasPrefix(body, ellipsis) {
		d.Relative = true
		body = body[1:]
	}

	for i := 0; i < len(body); {
		switch {
		case strings.HasPrefix(body[i:], ellipsis):
			if len(d.Elems) == 0 {
				d.Relative = true
			}

			d.Elems = append(d.Elems, ellipsis)
			i += len(ellipsis)

		case body[i] == '.':
			if i == 0 || i == len(body)-1 || body[i-1] == '.' && !strings.HasSuffix(body[:i], ellipsis) {
				return d, false
			}

			i++

		default:
			j := i
			for j < len(body) && body[j] != '.' {
				j++
			}

			e := body[i:j]
			if !validName(e) || e == "" {
				return d, false
			}

			if strings.Trim(e, "-") == "" {
				// A "-" after a name takes that name back out (VMS 7.1:
				// testdata/lnm91, [SUB.-] and [.K.-]), and may not climb
				// above where an absolute directory started.
				n := len(e)

				for n > 0 && len(d.Elems) > 0 && d.Elems[len(d.Elems)-1] != ellipsis && d.Elems[len(d.Elems)-1][0] != '-' {
					d.Elems = d.Elems[:len(d.Elems)-1]
					n--
				}

				if n == 0 {
					i = j

					continue
				}

				if n < len(e) && !d.Relative || len(d.Elems) > 0 && d.Elems[len(d.Elems)-1][0] != '-' {
					return d, false
				}

				e = strings.Repeat("-", n)
				d.Relative = true
			}

			d.Elems = append(d.Elems, e)
			i = j
		}
	}

	if len(d.names()) > maxDirDepth {
		return d, false
	}

	return d, true
}

// scanRoot splits a root's text (without its brackets, ending in its
// period) into elements, or gives the RMS status for one that isn't well
// formed, as VMS 7.1's (testdata/lnm91): an empty element is RMS$_SYN; a
// wildcard, a "-", or a bad name RMS$_DIR; and more than maxRootDepth
// levels RMS$_DEV.
//
// A root starting with a period ("[.A.]", "[.]") is accepted and names no
// directory: VMS keeps its text and ends with RMS$_DNF. It comes back
// Relative.
func scanRoot(body string) (dirSpec, uint32) {
	var d dirSpec

	s := body
	if strings.HasPrefix(s, ".") {
		d.Relative = true
		s = s[1:]
	}

	s = strings.TrimSuffix(s, ".")

	if s == "" {
		return d, 0
	}

	for _, e := range strings.Split(s, ".") {
		switch {
		case e == "":
			return d, rmsSyntaxError
		case !validName(e), strings.ContainsAny(e, "*%"), strings.Trim(e, "-") == "":
			return d, rmsDirError
		}

		d.Elems = append(d.Elems, e)
	}

	if len(d.names()) > maxRootDepth {
		return d, rmsDeviceError
	}

	return d, 0
}

// dirBody is a directory's text without its brackets.
func dirBody(dir string) string {
	if len(dir) < 2 {
		return ""
	}

	return dir[1 : len(dir)-1]
}

// likeBrackets is the directory dir in the kind of brackets like has.
func likeBrackets(dir, like string) string {
	if like != "" && like[0] == '<' {
		return "<" + dirBody(dir) + ">"
	}

	return "[" + dirBody(dir) + "]"
}

// applyDir applies the relative directory rel to base, an absolute one.
// It fails when "-" climbs above the MFD. As VMS writes it, a directory
// below the MFD reached this way keeps the MFD's name: [.SUB] on [000000]
// is [000000.SUB] (the oracle's PARSE case 21).
func applyDir(rel, base dirSpec) (dirSpec, bool) {
	out := append([]string{}, base.Elems...)
	if len(out) == 0 {
		out = []string{"000000"}
	}

	for _, e := range rel.Elems {
		if strings.Trim(e, "-") == "" {
			if len(out) < len(e) || len(out) == len(e) && out[0] == "000000" {
				return dirSpec{}, false
			}

			out = out[:len(out)-len(e)]

			continue
		}

		out = append(out, e)
	}

	if len(out) == 1 && out[0] == "000000" {
		out = nil
	}

	return dirSpec{Elems: out}, true
}

// String renders d as a directory specification: "[TEST.SUB]",
// "[TEST...]", or "[000000]" for the MFD.
func (d dirSpec) String() string {
	if len(d.Elems) == 0 && !d.Relative {
		return "[000000]"
	}

	var b strings.Builder

	b.WriteByte('[')

	if d.Relative && (len(d.Elems) == 0 || d.Elems[0] != ellipsis && d.Elems[0][0] != '-') {
		b.WriteByte('.')
	}

	for i, e := range d.Elems {
		if i > 0 && e != ellipsis && d.Elems[i-1] != ellipsis {
			b.WriteByte('.')
		}

		b.WriteString(e)
	}

	b.WriteByte(']')

	return b.String()
}

// names returns d's elements without ellipses, for looking it up: the
// MFD's own name, [000000.X]'s first element, isn't one.
func (d dirSpec) names() []string {
	var out []string

	for i, e := range d.Elems {
		if e != ellipsis && (i > 0 || e != "000000") {
			out = append(out, e)
		}
	}

	return out
}

// parsedName is a file name after name processing: the expanded
// specification's fields, and what $PARSE reports about it.
type parsedName struct {
	fileName

	// Lookup is the device name to find the volume by: Dev without its
	// "_" and ":".
	Lookup string

	// PhysicalDev is the device as the translation wrote it ("_DUA1:"),
	// which Dev is unless a concealed name's taken its place.
	PhysicalDev string

	// DirSpec is the directory's elements, relative to the root if there
	// is one. RootSpec is the root's; Relative in it means a root that
	// names no directory (scanRoot).
	DirSpec, RootSpec dirSpec

	// ShownRoot is the root's text when the name shows it (a root that isn't
	// concealed, "[USERS.]"), which Dir then begins with, and "" otherwise.
	ShownRoot string

	// FailsNext, on the last of a search list's usable names, is the status
	// of the element after it, which $SEARCH reports on reaching it.
	FailsNext uint32

	// FNB is the NAM$L_FNB bits name processing sets.
	FNB uint32

	// Permanent, when the name led to a process-permanent file (ppf.go),
	// is the file; Dev is then the logical name whose string named it,
	// with a colon, and Lookup that name.
	Permanent *lnm.PermanentFile
}

// String is the expanded specification. A root that's shown is in Dir,
// with the directory.
func (p parsedName) String() string {
	return p.Dev + p.Dir + p.Name + p.Type + p.Ver
}

// shownDir is the directory text a file found at path shows: path is
// relative to p's root, as p.DirSpec is, and the text has p's brackets and,
// if p shows its root, the root before it.
func (p parsedName) shownDir(path []string) string {
	return p.ShownRoot + likeBrackets(dirSpec{Elems: path}.String(), p.Dir[len(p.ShownRoot):])
}

// relative is dirs, a directory path from the volume's MFD, as the name
// shows it: the root's levels taken off. A name whose directory was written
// from "000000" keeps that as its first level, as VMS shows it under a root
// ([000000.SUB], VMS 7.1: testdata/lnm91), and as $SEARCH does.
func (p parsedName) relative(dirs []string) []string {
	below := p.belowRoot(dirs)

	if p.Root != "" && len(below) > 0 && len(p.DirSpec.Elems) > 0 && p.DirSpec.Elems[0] == "000000" {
		return append([]string{"000000"}, below...)
	}

	return below
}

// belowRoot is a volume path with p's root's levels taken off, the path a
// save set records (VMS 7.1: [SUB]A.DAT for BASE:[SUB]A.DAT).
func (p parsedName) belowRoot(dirs []string) []string {
	if p.Root == "" {
		return dirs
	}

	root := p.RootSpec.names()
	if p.RootSpec.Relative {
		root = []string{"."}
	}

	if len(dirs) < len(root) {
		return dirs
	}

	return dirs[len(root):]
}

// physical is p as NAM$V_NOCONCEAL shows it: the device as translated and
// any root, "_DUA1:[USERS.][SUB]A.DAT;". VMS's image activator shows and
// records a file this way.
func (p parsedName) physical() string {
	return p.PhysicalDev + p.Root + p.Dir[len(p.ShownRoot):] + p.Name + p.Type + p.Ver
}

// physicalFile is physical for a file found at the volume path dirs,
// file its name, type, and version.
func (p parsedName) physicalFile(dirs []string, file string) string {
	return p.PhysicalDev + p.Root + likeBrackets(dirSpec{Elems: p.belowRoot(dirs)}.String(), p.Dir[len(p.ShownRoot):]) + file
}

// resultFor is the resultant name of a file with spec's fields, its
// directory a path from the volume's MFD, found through p: the "." and ";"
// are always there, even with no type or version.
func (p parsedName) resultFor(spec filespec.Spec) string {
	dir := p.Dir
	if !spec.Recursive {
		dir = p.shownDir(p.relative(spec.Dirs))
	}

	return p.Dev + dir + spec.Name + "." + spec.Type + ";" + spec.Version
}

// volumeDir is p's directory as the volume has it: the root's levels, then
// the directory's. A root that names no directory gives an element no
// directory can have, so that looking for it finds nothing.
func (p parsedName) volumeDir() dirSpec {
	elems := p.DirSpec.Elems
	if p.Root == "" {
		return dirSpec{Elems: elems}
	}

	if len(elems) > 0 && elems[0] == "000000" {
		elems = elems[1:]
	}

	root := p.RootSpec.names()
	if p.RootSpec.Relative {
		root = []string{"."}
	}

	return dirSpec{Elems: append(append([]string{}, root...), elems...)}
}

// spec is p as a filespec.Spec, for the volume layer.
func (p parsedName) spec() filespec.Spec {
	ver := strings.TrimPrefix(p.Ver, ";")

	return filespec.Spec{
		Device:    p.Lookup,
		Dirs:      p.volumeDir().names(),
		Recursive: len(p.DirSpec.Elems) > 0 && p.DirSpec.Elems[len(p.DirSpec.Elems)-1] == ellipsis,
		Name:      p.Name,
		Type:      strings.TrimPrefix(p.Type, "."),
		Version:   ver,
	}
}

// defaultName is a name that stands in for the process defaults
// (nameInputs.Against): a device with the root and concealed name that go
// with it, a directory relative to that root, and a name and type.
type defaultName struct {
	Disk      translatedName
	Dirs      []string
	Recursive bool
	Name      string
	Type      string
}

// defaults is p as the defaults of the name after it. A concealed name
// comes back as a concealed name, so that the name after this one shows it
// too.
func (p parsedName) defaults() *defaultName {
	d := &defaultName{
		Disk:      translatedName{fileName: fileName{Dev: p.Dev, Root: p.Root}},
		Dirs:      p.DirSpec.names(),
		Recursive: len(p.DirSpec.Elems) > 0 && p.DirSpec.Elems[len(p.DirSpec.Elems)-1] == ellipsis,
		Name:      p.Name,
		Type:      p.Type,
	}

	if p.FNB&fnbCnclDev != 0 {
		d.Disk.Dev, d.Disk.Concealed = p.Lookup+":", p.Dev
	}

	return d
}

// nameInputs are a name processing's sources.
type nameInputs struct {
	// Primary and Default are the FAB's file name and default name.
	Primary, Default string

	// Related is the related file's resultant string, or "", and
	// BadRelated true when NAM$L_RLF names a block that isn't a NAM.
	Related    string
	BadRelated bool

	// OFP is FAB$V_OFP: the related file gives only the name and type.
	OFP bool

	// NoConceal is NAM$V_NOCONCEAL: show a concealed device's
	// translation rather than its logical name.
	NoConceal bool

	// ThroughPermanent takes a process-permanent file's name as the device
	// its string names, for callers that want a device or a volume file
	// (the DCL utilities, $ASSIGN): the name isn't reported as Permanent.
	ThroughPermanent bool

	// Against, when set, stands in for the process defaults: its device
	// for SYS$DISK's, its directory for the default directory (and what a
	// relative directory applies to), and its name and type for ones no
	// other source gives. It's how a name takes its defaults from the name
	// before it in a list, or from the file being renamed.
	Against *defaultName

	// BadString is RMS$_FNA or RMS$_DNA when the file name's or default
	// name's string can't be read, and 0 otherwise. A status, not a fault:
	// RMS checks a program's strings. FORRTL's INQUIRE gives a $PARSE an
	// address of 0 with a length, to end the search it was in, and doesn't
	// look at the status, so which one VMS gives is unconfirmed.
	BadString uint32
}

// FNB bits.
var (
	fnbExpVer     = vmsConst("NAM$M_EXP_VER")
	fnbExpType    = vmsConst("NAM$M_EXP_TYPE")
	fnbExpName    = vmsConst("NAM$M_EXP_NAME")
	fnbWildVer    = vmsConst("NAM$M_WILD_VER")
	fnbWildType   = vmsConst("NAM$M_WILD_TYPE")
	fnbWildName   = vmsConst("NAM$M_WILD_NAME")
	fnbExpDir     = vmsConst("NAM$M_EXP_DIR")
	fnbExpDev     = vmsConst("NAM$M_EXP_DEV")
	fnbWildcard   = vmsConst("NAM$M_WILDCARD")
	fnbSearchList = vmsConst("NAM$M_SEARCH_LIST")
	fnbCnclDev    = vmsConst("NAM$M_CNCL_DEV")
	fnbHighVer    = vmsConst("NAM$M_HIGHVER")
	fnbLowVer     = vmsConst("NAM$M_LOWVER")
	fnbWildDir    = vmsConst("NAM$M_WILD_DIR")
	fnbWildUFD    = vmsConst("NAM$M_WILD_UFD")
	fnbWildSFD1   = vmsConst("NAM$M_WILD_SFD1")
	fnbDirLvls    = vmsConst("NAM$V_DIR_LVLS")
	fnbRootDir    = vmsConst("NAM$M_ROOT_DIR")
)

// translatedName is one translation of a name: its fields (the program's
// and the logical name's together), and the device to show when it went
// through a concealed logical name.
type translatedName struct {
	fileName
	Concealed string

	// Fails, in a search list's last element, is the status of the element
	// that follows the ones given: the list is usable up to there (VMS 7.1:
	// testdata/lnm91's LIST3, whose $SEARCH fails only on reaching it).
	Fails uint32

	// Permanent is the process-permanent file the name led to, if any:
	// the fields are then only the program's own, with the logical name
	// as the device.
	Permanent *lnm.PermanentFile
}

// translateName translates text's logical name and scans each result. A
// search list gives one per element, and the first element that can't be
// used ends it (Fails), unless it's the first of all, whose status is
// returned.
//
// A process-permanent file ends the name there, unless throughPermanent
// asks for the device its string names instead.
func translateName(db *lnm.Database, text string, throughPermanent bool) ([]translatedName, uint32) {
	if text == "" {
		return []translatedName{{}}, 0
	}

	fs, err := translateSpec(db, strings.ToUpper(text))
	if err != nil {
		return nil, rmsLogicalNameError
	}

	out := make([]translatedName, 0, len(fs))

	for i, f := range fs {
		t, sts := translateElement(db, f, throughPermanent)
		if sts != 0 {
			if i == 0 {
				return nil, sts
			}

			return append(out, translatedName{Fails: sts}), 0
		}

		out = append(out, t)
	}

	return out, 0
}

// translateElement scans one translation: the fields the program wrote,
// and the equivalence string's.
func translateElement(db *lnm.Database, f lnm.FileSpec, throughPermanent bool) (translatedName, uint32) {
	own, sts := scanName(f.Remainder)
	if sts != 0 {
		return translatedName{}, sts
	}

	// A process-permanent file ends name processing: the device is
	// the logical name, and nothing else is merged (the Guide to
	// OpenVMS File Applications, 6.2).
	if f.Permanent != nil && !throughPermanent {
		own.Dev = f.PermanentName + ":"

		return translatedName{fileName: own, Permanent: f.Permanent}, 0
	}

	var t translatedName

	typedRoot := own.Root != ""

	prefix := f.Spec[:len(f.Spec)-len(f.Remainder)]
	if prefix != "" {
		eqv, sts := scanFields(prefix, rmsDeviceError)
		if sts != 0 {
			return t, sts
		}

		if f.Concealed != "" && concealsMore(db, f.Concealed) {
			return t, rmsDeviceError
		}

		if own, sts = applyTranslation(own, eqv); sts != 0 {
			return t, sts
		}
	}

	t.fileName = own

	// A root the program typed can't be hidden by the device's name
	// (unconfirmed: the probe has no typed root on a concealed name).
	if f.Concealed != "" && !typedRoot {
		t.Concealed = f.Concealed + ":"
	}

	return t, 0
}

// concealsMore reports whether a concealed logical name translates to more
// than a device and a root, which RMS refuses wherever the name is used
// (VMS 7.1: testdata/lnm91, A4 and round 2's C).
func concealsMore(db *lnm.Database, name string) bool {
	fs, err := translateSpec(db, name+":")
	if err != nil {
		return true
	}

	for _, f := range fs {
		eqv, sts := scanFields(f.Spec, rmsDeviceError)
		if sts != 0 || eqv.Dir != "" || eqv.Name != "" || eqv.Type != "" || eqv.Ver != "" {
			return true
		}
	}

	return false
}

// applyTranslation is own, the fields the program wrote, with those of
// eqv, the logical name's translation. A field both give is an error, for
// every logical name (VMS 7.1: testdata/lnm91's round 2), with one
// exception kept for the User's Manual's PAY_FILE:*.DAT (11.3.3), where the
// program's "*" name and its type are used instead; the probe didn't try
// that form.
func applyTranslation(own, eqv fileName) (fileName, uint32) {
	if own.Root != "" && eqv.Root != "" {
		return own, rmsDeviceError
	}

	if eqv.Dir != "" && (own.Dir != "" || own.Root != "") {
		return own, rmsDirError
	}

	if own.Name != "*" {
		if own.Name != "" && eqv.Name != "" {
			return own, rmsFileNameError
		}

		if own.Type != "" && eqv.Type != "" {
			return own, rmsTypeError
		}
	}

	return mergeName(own, eqv)
}

// mergeName fills the fields f leaves out from def. A root goes with its
// device: f takes def's only with def's device. A root f already has on a
// device def gives, itself with a root, is RMS$_DEV, as a second root
// through a concealed name is (unconfirmed for defaults).
func mergeName(f, def fileName) (fileName, uint32) {
	if f.Dev == "" {
		if f.Root != "" && def.Root != "" {
			return f, rmsDeviceError
		}

		f.Dev = def.Dev

		if f.Root == "" {
			f.Root = def.Root
		}
	}

	if f.Dir == "" {
		f.Dir = def.Dir
	}

	if f.Name == "" {
		f.Name = def.Name
	}

	if f.Type == "" {
		f.Type = def.Type
	}

	if f.Ver == "" {
		f.Ver = def.Ver
	}

	return f, 0
}

// expandName runs name processing on in, returning the expanded names in
// search order: one, unless the primary name (or SYS$DISK, when the name
// has no device) is a search list. On failure it returns the RMS status
// instead.
func (ctx *Context) expandName(in nameInputs) ([]parsedName, uint32) {
	var base filespec.Spec
	if ctx.Session != nil {
		base = ctx.Session.Default
	}

	return expandNames(ctx.Logicals, base, in)
}

// expandNames is name processing itself, with the logical names db and
// the process default directory base (expandName's are a Context's).
func expandNames(db *lnm.Database, base filespec.Spec, in nameInputs) ([]parsedName, uint32) {
	if in.BadString != 0 {
		return nil, in.BadString
	}

	primaries, sts := translateName(db, in.Primary, in.ThroughPermanent)
	if sts != 0 {
		return nil, sts
	}

	defs, sts := translateName(db, in.Default, in.ThroughPermanent)
	if sts != 0 {
		return nil, sts
	}

	var related translatedName

	if in.Related != "" {
		// A resultant string can show a concealed name, so it's translated
		// as any other name. It's otherwise not checked, except that a
		// concealed name's rules hold (VMS 7.1: testdata/lnm91's round 2).
		rs, sts := translateName(db, in.Related, in.ThroughPermanent)
		if sts == rmsDeviceError {
			return nil, sts
		}

		if sts == 0 {
			related = rs[0]
		}

		related.Ver = ""
		if in.OFP {
			related.Dev, related.Root, related.Dir, related.Concealed = "", "", "", ""
		}
	}

	if in.Against != nil {
		base = filespec.Spec{Dirs: in.Against.Dirs}
	}

	defaultDir := dirSpec{Elems: base.Dirs}.String()
	if in.Against != nil && in.Against.Recursive {
		defaultDir = dirSpec{Elems: append(append([]string{}, base.Dirs...), ellipsis)}.String()
	}

	var out []parsedName

	searchList := len(primaries) > 1

	for _, p := range primaries {
		if p.Fails != 0 {
			if len(out) > 0 {
				out[len(out)-1].FailsNext = p.Fails
			}

			break
		}

		if perm := permanentName(p, defs[0]); perm != nil {
			out = append(out, *perm)

			continue
		}

		var fnb uint32

		if p.Dev != "" {
			fnb |= fnbExpDev
		}

		if p.Dir != "" {
			fnb |= fnbExpDir
		}

		if p.Name != "" {
			fnb |= fnbExpName
		}

		if p.Type != "" {
			fnb |= fnbExpType
		}

		if p.Ver != "" {
			fnb |= fnbExpVer
		}

		f := p.fileName
		concealed := p.Concealed

		// The default name, then the related file, fill what the primary
		// name left out. A directory from any of them that's relative is
		// applied to the process default directory, not to a directory
		// a later source gives (the oracle's PARSE cases 20 and 21). A
		// directory they give decides the primary's brackets (VMS 7.1:
		// testdata/lnm91's round 2, for the default name; the related
		// file by analogy, unconfirmed).
		dir := f.Dir
		f.Dir = ""

		for _, src := range []translatedName{defs[0], related} {
			switch {
			case dir == "":
				dir = src.Dir
			case src.Dir != "":
				dir = likeBrackets(dir, src.Dir)
			}

			if f.Dev == "" && p.Root == "" && concealed == "" {
				concealed = src.Concealed
			}

			src.Dir = ""

			if f, sts = mergeName(f, src.fileName); sts != 0 {
				return nil, sts
			}
		}

		if in.Against != nil {
			f, _ = mergeName(f, fileName{Name: in.Against.Name, Type: in.Against.Type})
		}

		if dir != "" {
			d, _ := scanDir(dirBody(dir))
			if d.Relative {
				out, ok := applyDir(d, dirSpec{Elems: base.Dirs})
				if !ok {
					return nil, rmsDirError
				}

				dir = likeBrackets(out.String(), dir)
			}
		}

		// The process defaults: SYS$DISK (a search list gives one name
		// per element) and the default directory.
		disks := []translatedName{{}}

		switch {
		case f.Dev != "":
		case in.Against != nil:
			disks[0] = in.Against.Disk
		case db != nil:
			if _, err := db.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0); err == nil {
				if disks, sts = translateName(db, sysDiskName+":", in.ThroughPermanent); sts != 0 {
					return nil, sts
				}

				if len(disks) > 1 {
					searchList = true
				}
			}
		}

		for _, disk := range disks {
			if disk.Fails != 0 {
				if len(out) > 0 {
					out[len(out)-1].FailsNext = disk.Fails
				}

				break
			}

			g := f
			g.Dir = dir

			if g, sts = mergeName(g, fileName{Dev: disk.Dev, Root: disk.Root}); sts != 0 {
				return nil, sts
			}

			if g.Dir == "" {
				g.Dir = disk.Dir
			}

			if g.Dir == "" {
				g.Dir = defaultDir
			}

			c := concealed
			if c == "" && f.Dev == "" && p.Root == "" {
				c = disk.Concealed
			}

			out = append(out, finishName(g, c, fnb, in.NoConceal))
		}
	}

	if searchList {
		for i := range out {
			out[i].FNB |= fnbSearchList
		}
	}

	return out, 0
}

// scanSpec is text's own fields as a filespec.Spec: nothing translated and
// no defaults applied, and a device's "_" kept. For a name that's already
// complete, such as a resultant string. On failure it returns the RMS
// status, as scanName does.
func scanSpec(text string) (filespec.Spec, uint32) {
	f, sts := scanName(text)
	if sts != 0 {
		return filespec.Spec{}, sts
	}

	spec := finishName(f, "", 0, false).spec()
	spec.Device = strings.TrimSuffix(f.Dev, ":")

	return spec, 0
}

// permanentName is the name a primary name p, with the default name def,
// expands to when either leads to a process-permanent file: p's device
// or, when it has none, def's (NAM$M_EXP_DEV only for p's own). Only the
// device comes from def (VMS 7.1:
// F$PARSE("X.DAT","SYS$OUTPUT") is SYS$OUTPUT:X.DAT;); the name has no
// directory, and no other default applies. It is nil for any other name.
func permanentName(p, def translatedName) *parsedName {
	fnb := fnbPPF | fnbExpDev

	if p.Permanent == nil {
		if p.Dev != "" || def.Permanent == nil {
			return nil
		}

		p.Permanent, p.Dev = def.Permanent, def.Dev
		fnb = fnbPPF
	}

	for _, f := range []struct {
		field string
		bit   uint32
	}{{p.Dir, fnbExpDir}, {p.Name, fnbExpName}, {p.Type, fnbExpType}, {p.Ver, fnbExpVer}} {
		if f.field != "" {
			fnb |= f.bit
		}
	}

	dir := p.Dir
	n := finishName(p.fileName, "", fnb, false)
	n.Dir = dir
	n.Permanent = p.Permanent

	return &n
}

// finishName fills in the fields no source gave and works out the FNB
// bits the finished name implies.
func finishName(f fileName, concealed string, fnb uint32, noConceal bool) parsedName {
	if f.Type == "" {
		f.Type = "."
	}

	if f.Ver == "" {
		f.Ver = ";"
	}

	p := parsedName{fileName: f, FNB: fnb, PhysicalDev: f.Dev}
	p.Lookup = strings.TrimSuffix(strings.TrimPrefix(f.Dev, "_"), ":")
	p.DirSpec, _ = scanDir(dirBody(f.Dir))
	p.Dir = likeBrackets(p.DirSpec.String(), f.Dir)

	showRoot := true

	if concealed != "" {
		p.FNB |= fnbCnclDev

		if !noConceal {
			p.Dev = concealed
			showRoot = false
		}
	}

	if f.Root != "" {
		p.FNB |= fnbRootDir
		p.RootSpec, _ = scanRoot(dirBody(f.Root))

		if showRoot {
			p.ShownRoot = f.Root
			p.Dir = f.Root + p.Dir
		}
	}

	if strings.ContainsAny(p.Name, "*%") {
		p.FNB |= fnbWildName
	}

	if strings.ContainsAny(p.Type, "*%") {
		p.FNB |= fnbWildType
	}

	if p.Ver == ";*" {
		p.FNB |= fnbWildVer
	}

	level := 0

	for _, e := range p.DirSpec.Elems {
		if e == ellipsis {
			p.FNB |= fnbWildDir

			continue
		}

		if strings.ContainsAny(e, "*%") {
			p.FNB |= fnbWildDir

			switch {
			case level == 0:
				p.FNB |= fnbWildUFD
			case level <= 7:
				p.FNB |= fnbWildSFD1 << (level - 1)
			}
		}

		level++
	}

	if level > 1 {
		p.FNB |= uint32(min(level-1, 7)) << fnbDirLvls
	}

	if p.FNB&(fnbWildName|fnbWildType|fnbWildVer|fnbWildDir) != 0 {
		p.FNB |= fnbWildcard
	}

	return p
}

// IsODSFileName reports whether s is a valid ODS-2 file name: a name and type
// of 1..39 characters, each a letter, digit, "$", "_", or "-", and a version
// of none or a number. If the name contains obvious indicators that it is a
// host name instead, we bail out early (e.g., "DUA0:[000000]FILE.TXT;1" is a
// valid ODS-2 file name, but "./foo.com" is not).
func IsODSFileName(s string) bool {
	// Check for obvious Unix-isms in the file name.
	if strings.ContainsAny(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return false
	}

	// Check for obvious Windows-isms in the file name.
	for _, part := range []string{
		"\\",
		".\\",
		"..\\",
	} {
		if strings.Contains(s, part) {
			return false
		}
	}

	// Validate that this is a valid ODS-2 file name. If it is not, we will return false.
	// Note this is disabled; a mal-formed ODS2 name should not be interpreted as a host
	// file name. We don't want "FOO]BAT.TXT"
	// _, sts := scanName(s)
	sts := 0

	return sts == 0
}

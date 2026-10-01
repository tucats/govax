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
//
// A specification is filled in from several sources, each supplying only
// the fields the ones before it left out:
//
//  1. the primary name (FAB$L_FNA), after its logical name is translated
//     (the fields the program wrote win over the translation's);
//  2. the default name (FAB$L_DNA), translated the same way;
//  3. the related file's resultant string (NAM$L_RLF), for the device,
//     directory, name, and type (with FAB$V_OFP, the directory, name,
//     and type only);
//  4. the process defaults: SYS$DISK's device and the default directory.
//
// A relative directory ("[.SUB]", "[-]") is applied to the directory the
// later sources give. A field no source gives is empty: no name, a type
// of ".", and a version of ";".

// fileName is a file specification's fields as name processing keeps
// them: the text of each, with its delimiters ("DUA1:", "[TEST.SUB]",
// "A", ".DAT", ";3"). An empty field is one not given; a given but empty
// type or version is "." or ";".
type fileName struct {
	Dev, Dir, Name, Type, Ver string
}

// maxNameLen is the longest file name or type ODS-2 allows.
const maxNameLen = 39

// maxDirDepth is how many directory levels RMS allows below the MFD.
const maxDirDepth = 8

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
// RMS$_TYP, RMS$_VER, or RMS$_SYN for text left over. Node names
// (DECnet) aren't supported, and give RMS$_SYN.
func scanName(text string) (fileName, uint32) {
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

	if s != "" && (s[0] == '[' || s[0] == '<') {
		end := byte(']')
		if s[0] == '<' {
			end = '>'
		}

		k := strings.IndexByte(s, end)
		if k < 0 {
			return f, rmsDirError
		}

		if _, ok := scanDir(s[1:k]); !ok {
			return f, rmsDirError
		}

		f.Dir, s = "["+s[1:k]+"]", s[k+1:]
	}

	k := strings.IndexAny(s, ".;")
	if k < 0 {
		k = len(s)
	}

	if !validName(s[:k]) {
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
			return f, rmsInvalidVersion
		}

		f.Ver, s = ";"+v, ""
	}

	return f, 0
}

// validVersion reports whether v (the text after ";") is a version RMS
// accepts: none, "*", a number up to 32767, or a negative one (that many
// versions below the highest).
func validVersion(v string) bool {
	if v == "" || v == "*" {
		return true
	}

	n, err := strconv.Atoi(v)

	return err == nil && n >= -32767 && n <= 32767 && v[0] != '+'
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
				if len(d.Elems) > 0 && d.Elems[len(d.Elems)-1][0] != '-' {
					return d, false
				}

				d.Relative = true
			}

			d.Elems = append(d.Elems, e)
			i = j
		}
	}

	if len(d.names()) > maxDirDepth {
		return d, false
	}

	// [000000] is the MFD itself, and [000000.X] is [X].
	if len(d.Elems) > 0 && d.Elems[0] == "000000" && !d.Relative {
		d.Elems = d.Elems[1:]
	}

	return d, true
}

// applyDir applies the relative directory rel to base, an absolute one.
// It fails when "-" climbs above the MFD.
func applyDir(rel, base dirSpec) (dirSpec, bool) {
	out := append([]string{}, base.Elems...)

	for _, e := range rel.Elems {
		if strings.Trim(e, "-") == "" {
			if len(out) < len(e) {
				return dirSpec{}, false
			}

			out = out[:len(out)-len(e)]

			continue
		}

		out = append(out, e)
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

// names returns d's elements without ellipses, for looking it up.
func (d dirSpec) names() []string {
	var out []string

	for _, e := range d.Elems {
		if e != ellipsis {
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

	// DirSpec is the directory's elements.
	DirSpec dirSpec

	// FNB is the NAM$L_FNB bits name processing sets.
	FNB uint32
}

// String is the expanded specification.
func (p parsedName) String() string {
	return p.Dev + p.Dir + p.Name + p.Type + p.Ver
}

// spec is p as a filespec.Spec, for the volume layer.
func (p parsedName) spec() filespec.Spec {
	ver := strings.TrimPrefix(p.Ver, ";")

	return filespec.Spec{
		Device:    p.Lookup,
		Dirs:      p.DirSpec.names(),
		Recursive: len(p.DirSpec.Elems) > 0 && p.DirSpec.Elems[len(p.DirSpec.Elems)-1] == ellipsis,
		Name:      p.Name,
		Type:      strings.TrimPrefix(p.Type, "."),
		Version:   ver,
	}
}

// nameInputs are a name processing's sources.
type nameInputs struct {
	// Primary and Default are the FAB's file name and default name.
	Primary, Default string

	// Related is the related file's resultant string, or "".
	Related string

	// OFP is FAB$V_OFP: the related file gives no device.
	OFP bool

	// NoConceal is NAM$V_NOCONCEAL: show a concealed device's
	// translation rather than its logical name.
	NoConceal bool
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
)

// translatedName is one translation of a name: its fields (the program's
// over the logical name's), and the device to show when it went through
// a concealed logical name.
type translatedName struct {
	fileName
	Concealed string
}

// translateName translates text's logical name and scans each result,
// the program's fields winning over the translation's. A search list
// gives one per element.
func translateName(db *lnm.Database, text string) ([]translatedName, uint32) {
	if text == "" {
		return []translatedName{{}}, 0
	}

	fs, err := translateSpec(db, strings.ToUpper(text))
	if err != nil {
		return nil, rmsLogicalNameError
	}

	out := make([]translatedName, 0, len(fs))

	for _, f := range fs {
		own, sts := scanName(f.Remainder)
		if sts != 0 {
			return nil, sts
		}

		prefix := f.Spec[:len(f.Spec)-len(f.Remainder)]
		if prefix != "" {
			eqv, sts := scanName(prefix)
			if sts != 0 {
				return nil, sts
			}

			if own, sts = mergeName(own, eqv); sts != 0 {
				return nil, sts
			}
		}

		t := translatedName{fileName: own}
		if f.Concealed != "" {
			t.Concealed = f.Concealed + ":"
		}

		out = append(out, t)
	}

	return out, 0
}

// mergeName fills the fields f leaves out from def. A relative directory
// in f is applied to def's when def's is absolute; otherwise the two
// relative ones combine.
func mergeName(f, def fileName) (fileName, uint32) {
	if f.Dev == "" {
		f.Dev = def.Dev
	}

	if f.Dir == "" {
		f.Dir = def.Dir
	} else if def.Dir != "" {
		d, _ := scanDir(strings.Trim(f.Dir, "[]"))
		if d.Relative {
			base, _ := scanDir(strings.Trim(def.Dir, "[]"))

			if base.Relative {
				d = dirSpec{Relative: true, Elems: append(append([]string{}, base.Elems...), d.Elems...)}
				f.Dir = d.String()
			} else {
				out, ok := applyDir(d, base)
				if !ok {
					return f, rmsDirError
				}

				f.Dir = out.String()
			}
		}
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
	db := ctx.Logicals

	primaries, sts := translateName(db, in.Primary)
	if sts != 0 {
		return nil, sts
	}

	defs, sts := translateName(db, in.Default)
	if sts != 0 {
		return nil, sts
	}

	var related fileName

	if in.Related != "" {
		if related, sts = scanName(in.Related); sts != 0 {
			related = fileName{}
		}

		related.Ver = ""
		if in.OFP {
			related.Dev = ""
		}
	}

	var base filespec.Spec
	if ctx.Session != nil {
		base = ctx.Session.Default
	}

	defaultDir := dirSpec{Elems: base.Dirs}.String()

	var out []parsedName

	searchList := len(primaries) > 1

	for _, p := range primaries {
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

		if f, sts = mergeName(f, defs[0].fileName); sts != 0 {
			return nil, sts
		}

		if concealed == "" && p.Dev == "" {
			concealed = defs[0].Concealed
		}

		if f, sts = mergeName(f, related); sts != 0 {
			return nil, sts
		}

		// The process defaults: SYS$DISK (a search list gives one name
		// per element) and the default directory.
		disks := []translatedName{{}}

		if f.Dev == "" {
			if _, err := db.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0); err == nil {
				if disks, sts = translateName(db, sysDiskName+":"); sts != 0 {
					return nil, sts
				}

				if len(disks) > 1 {
					searchList = true
				}
			}
		}

		for _, disk := range disks {
			g := f

			if g, sts = mergeName(g, disk.fileName); sts != 0 {
				return nil, sts
			}

			if g, sts = mergeName(g, fileName{Dir: defaultDir}); sts != 0 {
				return nil, sts
			}

			c := concealed
			if c == "" && f.Dev == "" {
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

// finishName fills in the fields no source gave and works out the FNB
// bits the finished name implies.
func finishName(f fileName, concealed string, fnb uint32, noConceal bool) parsedName {
	if f.Type == "" {
		f.Type = "."
	}

	if f.Ver == "" {
		f.Ver = ";"
	}

	p := parsedName{fileName: f, FNB: fnb}
	p.Lookup = strings.TrimSuffix(strings.TrimPrefix(f.Dev, "_"), ":")
	p.DirSpec, _ = scanDir(strings.Trim(f.Dir, "[]"))
	p.Dir = p.DirSpec.String()

	if concealed != "" {
		p.FNB |= fnbCnclDev

		if !noConceal {
			p.Dev = concealed
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

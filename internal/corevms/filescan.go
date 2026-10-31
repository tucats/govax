package corevms

import (
	"strings"
)

// $FILESCAN, the service that finds a file specification at the start of a
// string and says where its parts are (Phase 80, subtask 10), written from
// VSI's system services page: LIB$TPARSE's TPA$_FILESPEC and TPA$_NODE are
// defined to use it.
//
//	SYS$FILESCAN srcstr ,valuelst ,[fldflags] ,[auxout] ,[retlen]
//
// It checks a part's syntax and its characters, and nothing else: not a part's
// length, not whether the device exists. The specification ends at the first
// character that can't continue it, which isn't an error: the service reports
// the parts it found, and SS$_NORMAL.

// FileSpecParts is where each part of a file specification is in the string
// scanned: a part is text[start:end], and missing when start equals end.
type FileSpecParts struct {
	Node, Primary, Access, Secondary    FileSpecPart
	Device, Root, Directory, Name, Type FileSpecPart
	Version                             FileSpecPart

	// End is the length of the whole specification.
	End int
}

// FileSpecPart is one part's place in the string scanned. Found is its flag in
// $FILESCAN's mask, which a part may have with no characters: a type alone sets
// the name's, and "::" the primary node's (testdata/tpa80).
type FileSpecPart struct {
	Start, End int
	Found      bool
}

// Len is the part's length, 0 when it's missing.
func (p FileSpecPart) Len() int { return p.End - p.Start }

// isFileNameChar says whether c can be in a node, device, name, or type.
func isFileNameChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '$' || c == '_' || c == '-' ||
		c >= 0xC0 && c <= 0xFD && c != 0xD0 && c != 0xDE && c != 0xF0
}

// isWildChar says whether c can be in a name or type but not a node or device.
func isWildChar(c byte) bool {
	return c == '*' || c == '%'
}

// ScanFileSpec finds the file specification at the start of text.
func ScanFileSpec(text string) FileSpecParts {
	var parts FileSpecParts

	i := scanNodes(text, &parts)
	i = scanDevice(text, i, &parts)
	i = scanDirectory(text, i, &parts)
	i = scanFileName(text, i, &parts)
	parts.End = i

	return parts
}

// scanNodes reads NODE["access"]:: as often as it's there. The first is the
// primary node; the others are the secondary node information.
func scanNodes(text string, parts *FileSpecParts) int {
	i := 0

	for {
		name := i
		for name < len(text) && isFileNameChar(text[name]) {
			name++
		}

		access := name
		if access < len(text) && text[access] == '"' {
			end := closingQuote(text, access)
			if end < 0 {
				return i
			}

			access = end + 1
		}

		if !strings.HasPrefix(text[access:], "::") {
			return i
		}

		if parts.Node.End == 0 {
			parts.Primary = FileSpecPart{i, name, true}
			parts.Access = FileSpecPart{name, access, access > name}
		} else {
			parts.Secondary = FileSpecPart{parts.Access.End + 2, access + 2, true}
		}

		parts.Node = FileSpecPart{0, access + 2, true}
		i = access + 2
	}
}

// closingQuote returns the index of the quotation mark that ends the quoted
// text starting at text[i], where a doubled one stands for itself, or -1.
func closingQuote(text string, i int) int {
	for j := i + 1; j < len(text); j++ {
		if text[j] != '"' {
			continue
		}

		if j+1 < len(text) && text[j+1] == '"' {
			j++

			continue
		}

		return j
	}

	return -1
}

// scanDevice reads NAME: at text[i:].
func scanDevice(text string, i int, parts *FileSpecParts) int {
	j := i
	for j < len(text) && isFileNameChar(text[j]) {
		j++
	}

	if j == i || j >= len(text) || text[j] != ':' {
		return i
	}

	parts.Device = FileSpecPart{i, j + 1, true}

	return j + 1
}

// scanDirectory reads [dir] or <dir>, or a root and then perhaps a directory,
// [root.][dir], at text[i:]. A directory ending in "." is a root by that alone,
// unless the "." is the end of "...".
func scanDirectory(text string, i int, parts *FileSpecParts) int {
	end := directoryEnd(text, i)
	if end < 0 {
		return i
	}

	if text[end-2] != '.' || strings.HasSuffix(text[:end-1], "...") {
		parts.Directory = FileSpecPart{i, end, true}

		return end
	}

	parts.Root = FileSpecPart{i, end, true}

	if next := directoryEnd(text, end); next >= 0 {
		parts.Directory = FileSpecPart{end, next, true}

		return next
	}

	return end
}

// directoryEnd returns the index just past the bracketed directory at
// text[i:], or -1 if there's none.
func directoryEnd(text string, i int) int {
	if i >= len(text) {
		return -1
	}

	var closing byte

	switch text[i] {
	case '[':
		closing = ']'
	case '<':
		closing = '>'
	default:
		return -1
	}

	comma := false

	for j := i + 1; j < len(text); j++ {
		switch c := text[j]; {
		case c == closing:
			if comma && !isUICText(text[i+1:j]) {
				return -1
			}

			return j + 1
		case c == ',':
			comma = true
		case isFileNameChar(c), isWildChar(c), c == '.':
		default:
			return -1
		}
	}

	return -1
}

// isUICText says whether a directory with a comma in it is a numeric UIC, the
// only directory that may have one: "[1,2]", and not "[A,B]" (testdata/tpa80;
// wildcards here are unconfirmed).
func isUICText(text string) bool {
	group, member, found := strings.Cut(text, ",")
	if !found || group == "" || member == "" {
		return false
	}

	for _, c := range []byte(group + member) {
		if (c < '0' || c > '9') && !isWildChar(c) {
			return false
		}
	}

	return true
}

// scanFileName reads the name, type, and version at text[i:]. After a node, the
// name may be a quoted file specification, quotation marks included.
func scanFileName(text string, i int, parts *FileSpecParts) int {
	if parts.Node.End > 0 && i < len(text) && text[i] == '"' {
		if end := closingQuote(text, i); end >= 0 {
			parts.Name = FileSpecPart{i, end + 1, true}

			return end + 1
		}
	}

	nameChars := func(j int) int {
		for j < len(text) && (isFileNameChar(text[j]) || isWildChar(text[j])) {
			j++
		}

		return j
	}

	// A hyphen can't start a name.
	j := i
	if j < len(text) && text[j] != '-' {
		j = nameChars(i)
	}

	parts.Name = FileSpecPart{i, j, j > i}

	var end int

	if j < len(text) && text[j] == '.' {
		end = nameChars(j + 1)
		parts.Type = FileSpecPart{j, end, true}

		if end < len(text) && text[end] == '.' {
			end = scanVersion(text, end, '.', parts)
		} else {
			end = scanVersion(text, end, ';', parts)
		}
	} else {
		end = scanVersion(text, j, ';', parts)
	}

	if parts.Type.Found || parts.Version.Found {
		parts.Name.Found = true
	}

	return end
}

// scanVersion reads a version at text[i:], after its delimiter: digits with an
// optional sign, or a wildcard.
func scanVersion(text string, i int, delimiter byte, parts *FileSpecParts) int {
	if i >= len(text) || text[i] != delimiter {
		return i
	}

	j := i + 1
	if j < len(text) && text[j] == '-' {
		j++
	}

	for j < len(text) && (text[j] >= '0' && text[j] <= '9' || isWildChar(text[j])) {
		j++
	}

	parts.Version = FileSpecPart{i, j, true}

	return j
}

// fileScanItem is one $FILESCAN item code or flag: its symbols' suffix's part.
var fileScanItems = []struct {
	name string
	part func(*FileSpecParts) FileSpecPart
}{
	{"NODE", func(p *FileSpecParts) FileSpecPart { return p.Node }},
	{"DEVICE", func(p *FileSpecParts) FileSpecPart { return p.Device }},
	{"ROOT", func(p *FileSpecParts) FileSpecPart { return p.Root }},
	{"DIRECTORY", func(p *FileSpecParts) FileSpecPart { return p.Directory }},
	{"NAME", func(p *FileSpecParts) FileSpecPart { return p.Name }},
	{"TYPE", func(p *FileSpecParts) FileSpecPart { return p.Type }},
	{"VERSION", func(p *FileSpecParts) FileSpecPart { return p.Version }},
	{"NODE_PRIMARY", func(p *FileSpecParts) FileSpecPart { return p.Primary }},
	{"NODE_ACS", func(p *FileSpecParts) FileSpecPart { return p.Access }},
	{"NODE_SECONDARY", func(p *FileSpecParts) FileSpecPart { return p.Secondary }},
}

// fscnFilespec is FSCN$_FILESPEC, the item for the whole specification. The
// other items' codes follow it, in fileScanItems' order; each one's flag is the
// bit of its place in that list.
const fscnFilespec = 1

// serviceSysFilescan is SYS$FILESCAN.
func serviceSysFilescan(env *Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.mem, env.cpu

	src := optArg(argv, 0)
	if src == 0 {
		return ssAccVio, nil
	}

	text, ok, err := strGet(env, src, 65535)
	if err != nil || !ok {
		return ssAccVio, nil
	}

	base, err := mem.LoadLongword(cpu, src+4)
	if err != nil {
		return ssAccVio, nil
	}

	parts := ScanFileSpec(text)

	// With an auxiliary buffer and a node to simplify, the string is copied
	// there and the addresses are the buffer's. VMS 7.1 leaves the buffer and
	// retlen alone for a specification with no node (testdata/tpa80). With one
	// it's unconfirmed, and a quoted primary node isn't reduced.
	if aux := optArg(argv, 3); aux != 0 && parts.Node.Found {
		size, err := mem.LoadWord(cpu, aux)
		if err != nil {
			return ssAccVio, nil
		}

		buffer, err := mem.LoadLongword(cpu, aux+4)
		if err != nil {
			return ssAccVio, nil
		}

		n := min(len(text), int(size))
		for i := range n {
			if mem.StoreByte(cpu, buffer+uint32(i), text[i]) != nil {
				return ssAccVio, nil
			}
		}

		if retlen := optArg(argv, 4); retlen != 0 && mem.StoreWord(cpu, retlen, uint16(n)) != nil {
			return ssAccVio, nil
		}

		base = buffer
	}

	if list := optArg(argv, 1); list != 0 {
		if status := storeFileScanItems(env, list, base, &parts); status != ssNormal {
			return status, nil
		}
	}

	if flags := optArg(argv, 2); flags != 0 {
		var mask uint32

		for bit, item := range fileScanItems {
			if item.part(&parts).Found {
				mask |= 1 << bit
			}
		}

		if mem.StoreLongword(cpu, flags, mask) != nil {
			return ssAccVio, nil
		}
	}

	return ssNormal, nil
}

// storeFileScanItems fills in the item list at list, whose entries are a word
// for the length, the word item code, and a longword for the address.
func storeFileScanItems(env *Environment, list, base uint32, parts *FileSpecParts) uint32 {
	mem, cpu := env.mem, env.cpu

	for ; ; list += 8 {
		head, err := mem.LoadLongword(cpu, list)
		if err != nil {
			return ssAccVio
		}

		if head == 0 {
			return ssNormal
		}

		var part FileSpecPart

		switch code := int(head >> 16); {
		case code == fscnFilespec:
			part = FileSpecPart{Start: 0, End: parts.End}
		case code > fscnFilespec && code <= fscnFilespec+len(fileScanItems):
			part = fileScanItems[code-fscnFilespec-1].part(parts)
		default:
			return ssBadParam
		}

		var addr uint32
		if part.Len() > 0 {
			addr = base + uint32(part.Start)
		}

		if mem.StoreWord(cpu, list, uint16(part.Len())) != nil || mem.StoreLongword(cpu, list+4, addr) != nil {
			return ssAccVio
		}
	}
}

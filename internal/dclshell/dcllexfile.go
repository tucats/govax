package dclshell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The lexical functions on files and devices (docs/PHASE-50 - DCL command
// procedures.md, subtask 14; the User's Manual, 15.4): F$PARSE, F$SEARCH,
// F$GETDVI, F$DEVICE, and F$FILE_ATTRIBUTES (Phase 51). F$PARSE and F$SEARCH apply RMS's rules (rms.Session's
// Parse and Search) to a file on a mounted volume, and govax's
// host-or-volume rule (rms.Session.Locate) decides which side a name
// means.

// parseFields are F$PARSE's field keywords: which part of the parsed
// specification each returns.
var parseFields = map[string]func(p rms.ParsedName) string{
	"NODE":      func(p rms.ParsedName) string { return p.Node },
	"DEVICE":    func(p rms.ParsedName) string { return p.Device },
	"DIRECTORY": func(p rms.ParsedName) string { return p.Directory },
	"NAME":      func(p rms.ParsedName) string { return p.Name },
	"TYPE":      func(p rms.ParsedName) string { return p.Type },
	"VERSION":   func(p rms.ParsedName) string { return p.Version },
}

// parseTypes are F$PARSE's parse-type keywords: SYNTAX_ONLY (don't check
// that the device and directory are there) and NO_CONCEAL (show the
// device a concealed logical name stands for).
type parseType struct{ syntaxOnly, noConceal bool }

var parseTypes = map[string]parseType{
	"SYNTAX_ONLY": {syntaxOnly: true},
	"NO_CONCEAL":  {noConceal: true},
}

// lexParse is F$PARSE(filespec [,default-spec] [,related-spec] [,field]
// [,parse-type]): filespec with the fields it leaves out filled in, from
// default-spec, then related-spec's name and type, then the default
// device and directory (15.4.2), the whole of it or the one field asked
// for. It is "" when the device isn't mounted or the directory isn't
// there (unless SYNTAX_ONLY), and for a specification RMS can't parse
// (unconfirmed against VMS: that a syntax error is "", not a message).
func lexParse(e *dclExpression, args []lexicalArg) (dclValue, error) {
	field := func(p rms.ParsedName) string { return p.String() }

	if args[3].present {
		f, _, err := lexicalKeyword(parseFields, args[3])
		if err != nil {
			return dclValue{}, err
		}

		field = f
	}

	var how parseType

	if args[4].present {
		t, _, err := lexicalKeyword(parseTypes, args[4])
		if err != nil {
			return dclValue{}, err
		}

		how = t
	}

	p, found, err := e.console.ContainerSession.Parse(args[0].str(), args[1].str(), args[2].str(), how.syntaxOnly, how.noConceal)
	if err != nil || !found {
		return dclString(""), nil
	}

	return dclString(field(p)), nil
}

// searchStream is one F$SEARCH stream's place: the specification being
// searched for, every file it names, and the next one to return.
type searchStream struct {
	spec  string
	files []string
	next  int
}

// lexSearch is F$SEARCH(filespec [,stream-id]): the next file filespec
// names, "" when there are no more (15.4.2). Each stream (0 when no
// stream-id is given) keeps its place between calls while filespec stays
// the same; a different one starts a new search. After the "" that ends
// a search the stream starts again. A name with no wildcards is found on
// every call, as VMS 7.3 found it (testdata/dcl50's round 5). On a
// mounted volume the
// files are RMS's, written "DUA0:[WORK]LOGIN.COM;3"; on the host they
// are host paths, matched without regard to case.
func lexSearch(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console
	spec, id := args[0].str(), args[1].int()

	if c.searchStreams == nil {
		c.searchStreams = map[int32]*searchStream{}
	}

	if wildcard := strings.ContainsAny(spec, "*%") || strings.Contains(spec, "..."); !wildcard {
		files, err := c.searchFiles(spec)
		if err != nil || len(files) == 0 {
			return dclString(""), err
		}

		return dclString(files[0]), nil
	}

	stream := c.searchStreams[id]
	if stream == nil || stream.spec != spec {
		files, err := c.searchFiles(spec)
		if err != nil {
			return dclValue{}, err
		}

		stream = &searchStream{spec: spec, files: files}
		c.searchStreams[id] = stream
	}

	if stream.next >= len(stream.files) {
		delete(c.searchStreams, id)

		return dclString(""), nil
	}

	stream.next++

	return dclString(stream.files[stream.next-1]), nil
}

// searchFiles returns every file spec names, on the host or on a mounted
// volume, as F$SEARCH returns them one by one. A name whose side can't be
// decided (an unmounted device) names no files.
func (c *Interpreter) searchFiles(spec string) ([]string, error) {
	loc, err := c.ContainerSession.Locate(spec, false)
	if err != nil {
		return nil, nil //nolint:nilerr // F$SEARCH finds nothing there
	}

	if loc.Host {
		return hostSearch(loc.Name), nil
	}

	files, err := c.ContainerSession.Search(spec)
	if err != nil {
		return nil, dclcmd.FileFailure(err, spec)
	}

	return files, nil
}

// hostSearch returns the host files name names: in its directory (the
// current one if it has none), the files whose names match its last
// element, where "*" stands for any characters and "%" for one, without
// regard to case, sorted.
func hostSearch(name string) []string {
	dir, pattern := filepath.Split(name)

	entries, err := os.ReadDir(filepath.Clean(dir + "."))
	if err != nil {
		return nil
	}

	var files []string

	for _, entry := range entries {
		if !entry.IsDir() && vmsWildMatch(strings.ToUpper(pattern), strings.ToUpper(entry.Name())) {
			files = append(files, dir+entry.Name())
		}
	}

	sort.Strings(files)

	return files
}

// vmsWildMatch reports whether s matches pattern, in which "*" matches
// any characters (none among them) and "%" exactly one.
func vmsWildMatch(pattern, s string) bool {
	for pattern != "" {
		switch pattern[0] {
		case '*':
			for i := len(s); i >= 0; i-- {
				if vmsWildMatch(pattern[1:], s[i:]) {
					return true
				}
			}

			return false
		case '%':
			if s == "" {
				return false
			}
		default:
			if s == "" || s[0] != pattern[0] {
				return false
			}
		}

		pattern, s = pattern[1:], s[1:]
	}

	return s == ""
}

// deviceStream is one F$DEVICE stream's place: the selection it was
// started with, the devices found, and the next one to return.
type deviceStream struct {
	selection string
	devices   []string
	next      int
}

// lexDevice is F$DEVICE([search_devnam] [,devclass] [,devtype]
// [,stream-id]): the next device whose name matches search_devnam (with
// wildcards; every device when it is left out), of the class and type
// named, or "" after the last (the DCL Dictionary). devclass is a class
// name, $DCDEF's without DC$_ ("DISK", "TERM", "MAILBOX"), and devtype a
// type name, $DTDEF's without DT$_ ("RD54"); a name with no such code is
// CLI_IVKEYW, and a type without a class SS$_BADPARAM (the Dictionary
// says it is an error, not which). Each stream (0 without a stream-id)
// keeps its place between calls while the selection stays the same; a
// different selection starts again, and after the "" the next call
// starts again too, as F$SEARCH's streams do. The names are
// corevms.Environment.DeviceScan's, "_GOVAX$DUA0:" (unconfirmed against
// VMS: the node's name in them, as VMS 7.3 equated OPEN's logical name to
// "_SIMVAX$DUA1"; and the order, by name, where VMS's is "random").
func lexDevice(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console

	env, err := c.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	pattern := args[0].str()

	class, byClass, err := deviceCode("DC$_", args[1])
	if err != nil {
		return dclValue{}, err
	}

	devType, byType, err := deviceCode("DT$_", args[2])
	if err != nil {
		return dclValue{}, err
	}

	if byType && !byClass {
		return dclValue{}, c.statusFailure(vmsdef.Symbols["SS$_BADPARAM"])
	}

	selection := fmt.Sprintf("%s\x00%d\x00%v\x00%d\x00%v", strings.ToUpper(pattern), class, byClass, devType, byType)
	id := args[3].int()

	if c.deviceStreams == nil {
		c.deviceStreams = map[int32]*deviceStream{}
	}

	stream := c.deviceStreams[id]
	if stream == nil || stream.selection != selection {
		stream = &deviceStream{selection: selection, devices: env.DeviceScan(pattern, class, devType, byClass, byType)}
		c.deviceStreams[id] = stream
	}

	if stream.next >= len(stream.devices) {
		delete(c.deviceStreams, id)

		return dclString(""), nil
	}

	stream.next++

	return dclString(stream.devices[stream.next-1]), nil
}

// deviceCode is F$DEVICE's device class or type argument as its code:
// the value of prefix and the name ("DC$_DISK"), and whether one was
// given (an argument left out, or "", selects any). A name with no code
// is CLI_IVKEYW.
func deviceCode(prefix string, arg lexicalArg) (code uint32, given bool, err error) {
	name := strings.ToUpper(strings.TrimSpace(arg.str()))
	if name == "" {
		return 0, false, nil
	}

	code, ok := vmsdef.Symbols[prefix+name]
	if !ok {
		return 0, false, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, name)
	}

	return code, true, nil
}

// fileItem returns one of F$FILE_ATTRIBUTES's items for a file.
type fileItem func(c *Interpreter, a rms.FileAttributes) dclValue

// fileFlag, fileInt, and fileTime make the common kinds of item: a truth
// ("TRUE" or "FALSE"), an integer, and a date and time, written as
// $ASCTIM writes it ("12-OCT-2026 14:03:10.25"; a time never set is the
// base time, "17-NOV-1858 00:00:00.00").
func fileFlag(f func(a rms.FileAttributes) bool) fileItem {
	return func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclTrueFalse(f(a)) }
}

func fileInt[T uint8 | uint16 | uint32](f func(a rms.FileAttributes) T) fileItem {
	return func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclInteger(int32(f(a))) }
}

func fileTime(f func(a rms.FileAttributes) uint64) fileItem {
	return func(_ *Interpreter, a rms.FileAttributes) dclValue {
		text, _ := corevms.FormatTime(f(a), false)

		return dclString(text)
	}
}

// fileFormats and fileOrganizations are RFM's and ORG's values, by the
// record format's and the organization's codes.
var (
	fileFormats       = []string{"UDF", "FIX", "VAR", "VFC", "STM", "STMLF", "STMCR"}
	fileOrganizations = []string{"SEQ", "REL", "IDX"}
)

// fileItems are F$FILE_ATTRIBUTES's keywords (the DCL Dictionary's Table
// DCLI-5, without PRESHELVED, which is Alpha's), and what each returns:
// the file header's fields, as rms.FileAttributes has them. What an
// ODS-2 volume in govax has no counterpart for is answered as a plain
// sequential file would answer it (unconfirmed against VMS: KNOWN,
// MOVE, SHELVABLE, SHELVED, STORED_SEMANTICS, FILE_LENGTH_HINT; and
// BLS, 512, and DVI's form, the device's name as OPEN's logical name is
// equated to it).
var fileItems = map[string]fileItem{
	"AI":  fileFlag(rms.FileAttributes.AfterImage),
	"ALQ": fileInt(func(a rms.FileAttributes) uint32 { return a.Allocated }),
	"BDT": fileTime(func(a rms.FileAttributes) uint64 { return a.Backup }),
	"BI":  fileFlag(rms.FileAttributes.BeforeImage),
	"BKS": fileInt(func(a rms.FileAttributes) uint8 { return a.BucketSize }),
	"BLS": func(*Interpreter, rms.FileAttributes) dclValue { return dclInteger(512) },
	"CBT": fileFlag(rms.FileAttributes.ContiguousBestTry),
	"CDT": fileTime(func(a rms.FileAttributes) uint64 { return a.Created }),
	"CTG": fileFlag(rms.FileAttributes.Contiguous),
	"DEQ": fileInt(func(a rms.FileAttributes) uint16 { return a.Extend }),
	"DID": func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclString(a.DID) },

	"DIRECTORY": fileFlag(rms.FileAttributes.Directory),
	"DVI": func(c *Interpreter, a rms.FileAttributes) dclValue {
		if a.Device == "" {
			return dclString("")
		}

		return dclString(c.physicalDevice(a.Device))
	},
	"EDT":   fileTime(func(a rms.FileAttributes) uint64 { return a.Expires }),
	"EOF":   fileInt(rms.FileAttributes.Used),
	"ERASE": fileFlag(rms.FileAttributes.Erase),
	"FFB":   fileInt(func(a rms.FileAttributes) uint16 { return a.FirstFreeByte }),
	"FID":   func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclString(a.FID) },

	"FILE_LENGTH_HINT": func(*Interpreter, rms.FileAttributes) dclValue { return dclString("(-1,-1)") },

	"FSZ":          fileInt(func(a rms.FileAttributes) uint8 { return a.FixedControl }),
	"GBC":          fileInt(func(a rms.FileAttributes) uint16 { return a.GlobalBuffers }),
	"GRP":          fileInt(func(a rms.FileAttributes) uint32 { return a.Owner >> 16 }),
	"JOURNAL_FILE": fileFlag(func(rms.FileAttributes) bool { return false }),
	"KNOWN":        fileFlag(func(rms.FileAttributes) bool { return false }),
	"LOCKED":       fileFlag(rms.FileAttributes.Locked),
	"LRL":          fileInt(func(a rms.FileAttributes) uint16 { return a.LongestRecord }),
	"MBM":          fileInt(func(a rms.FileAttributes) uint32 { return a.Owner & 0xFFFF }),
	"MOVE":         fileFlag(func(rms.FileAttributes) bool { return true }),
	"MRN":          fileInt(func(rms.FileAttributes) uint32 { return 0 }),
	"MRS":          fileInt(func(a rms.FileAttributes) uint16 { return a.RecordSize }),
	"NOA":          fileInt(func(rms.FileAttributes) uint32 { return 0 }),
	"NOBACKUP":     fileFlag(rms.FileAttributes.NoBackup),
	"NOK":          fileInt(func(rms.FileAttributes) uint32 { return 0 }),
	"ORG": func(_ *Interpreter, a rms.FileAttributes) dclValue {
		return dclString(codeName(fileOrganizations, a.Organization))
	},
	"PRO": func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclString(a.ProtectionText()) },
	"PVN": fileInt(func(rms.FileAttributes) uint32 { return 0 }),
	"RAT": func(_ *Interpreter, a rms.FileAttributes) dclValue { return dclString(recordAttributeName(a.RecordAttributes)) },
	"RCK": fileFlag(rms.FileAttributes.ReadCheck),
	"RDT": fileTime(func(a rms.FileAttributes) uint64 { return a.Revised }),
	"RFM": func(_ *Interpreter, a rms.FileAttributes) dclValue {
		return dclString(codeName(fileFormats, a.Format))
	},
	"RU":  fileFlag(rms.FileAttributes.RecoveryUnit),
	"RVN": fileInt(func(a rms.FileAttributes) uint16 { return a.Revision }),

	"SHELVABLE":        fileFlag(func(rms.FileAttributes) bool { return true }),
	"SHELVED":          fileFlag(func(rms.FileAttributes) bool { return false }),
	"STORED_SEMANTICS": func(*Interpreter, rms.FileAttributes) dclValue { return dclString("") },

	"UIC": func(c *Interpreter, a rms.FileAttributes) dclValue {
		return dclString(c.RTL.IdentifierText(a.Owner))
	},
	"VERLIMIT": func(_ *Interpreter, a rms.FileAttributes) dclValue {
		if a.VersionLimit == 0 {
			return dclInteger(32767)
		}

		return dclInteger(int32(a.VersionLimit))
	},
	"WCK": fileFlag(rms.FileAttributes.WriteCheck),
}

// codeName is names[code], or "" for a code past its end.
func codeName(names []string, code uint8) string {
	if int(code) < len(names) {
		return names[code]
	}

	return ""
}

// recordAttributeName is RAT's value for a file's record attributes
// (FAB$B_RAT): its carriage control, FTN, CR, or PRN, or "" for none.
func recordAttributeName(rat uint8) string {
	switch {
	case rat&vmsdefByte("FAB$M_FTN") != 0:
		return "FTN"
	case rat&vmsdefByte("FAB$M_CR") != 0:
		return "CR"
	case rat&vmsdefByte("FAB$M_PRN") != 0:
		return "PRN"
	}

	return ""
}

// vmsdefByte is the symbol name's value, as a byte mask.
func vmsdefByte(name string) uint8 { return uint8(vmsdef.Symbols[name]) }

// lexFileAttributes is F$FILE_ATTRIBUTES(filespec, item): an attribute
// of the one file filespec names (fileItems), read from its header
// (rms.Session.FileAttributes). A name with wildcards is RMS$_WLD, and
// one that finds no file RMS$_FNF (RMS$_DNF for a directory that isn't
// there), each as its message (unconfirmed against VMS: which message
// DCL shows for these).
func lexFileAttributes(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console

	if _, err := c.lexicalEnvironment(); err != nil {
		return dclValue{}, err
	}

	item, _, err := lexicalKeyword(fileItems, args[1])
	if err != nil {
		return dclValue{}, err
	}

	spec := strings.TrimSpace(args[0].str())

	attrs, err := c.ContainerSession.FileAttributes(spec)
	if err != nil {
		var fe *rms.DCLFileError
		if errors.As(err, &fe) {
			return dclValue{}, c.statusFailure(fe.Status)
		}

		return dclValue{}, dclcmd.FileFailure(err, spec)
	}

	return item(c, attrs), nil
}

// dviItems are F$GETDVI's keywords: each $GETDVI item (by its name
// without DVI$_), and how its value is written. init fills it from
// corevms.DVIItemNames: the items in dviStringItems, dviPIDItems, and
// dviUICItems are strings, hexadecimal PIDs, and UICs; the device
// characteristics and terminal characteristics are "TRUE" or "FALSE";
// the rest are integers. EXISTS says whether there is such a device.
var dviItems = map[string]func(data string, env *corevms.Environment) dclValue{}

var (
	dviStringItems = map[string]bool{
		"DEVNAM": true, "FULLDEVNAM": true, "ALLDEVNAM": true, "ROOTDEVNAM": true,
		"MEDIA_NAME": true, "MEDIA_TYPE": true, "TT_PHYDEVNAM": true, "VOLNAM": true,
	}
	dviPIDItems   = map[string]bool{"PID": true, "ACPPID": true}
	dviUICItems   = map[string]bool{"OWNUIC": true}
	dviCountItems = map[string]bool{"TT_PAGE": true}
	dviFlagItems  = map[string]bool{"REMOTE_DEVICE": true, "SERVED_DEVICE": true, "VOLSETMEM": true}
)

// dviIsFlag reports whether the item called name is a truth: one of
// dviFlagItems, a terminal characteristic (TT_), or a device
// characteristic bit ($DEVDEF's DEV$M_ names: MNT, DIR, ...).
func dviIsFlag(name string) bool {
	if dviFlagItems[name] {
		return true
	}

	if strings.HasPrefix(name, "TT_") && !dviCountItems[name] && !dviStringItems[name] {
		return true
	}

	_, ok := vmsdef.Symbols["DEV$M_"+name]

	return ok
}

func init() {
	for _, full := range corevms.DVIItemNames() {
		name := strings.TrimPrefix(full, "DVI$_")

		switch {
		case dviStringItems[name]:
			dviItems[name] = func(data string, _ *corevms.Environment) dclValue { return dclString(data) }
		case dviPIDItems[name]:
			dviItems[name] = func(data string, _ *corevms.Environment) dclValue {
				return dclString(fmt.Sprintf("%08X", itemLongword(data)))
			}
		case dviUICItems[name]:
			dviItems[name] = func(data string, env *corevms.Environment) dclValue {
				return dclString(env.IdentifierText(itemLongword(data)))
			}
		case dviIsFlag(name):
			dviItems[name] = func(data string, _ *corevms.Environment) dclValue { return dclTrueFalse(itemLongword(data)&1 != 0) }
		default:
			dviItems[name] = func(data string, _ *corevms.Environment) dclValue { return dclInteger(int32(itemLongword(data))) }
		}
	}

	// EXISTS is answered before the device is looked up (lexGetDVI).
	dviItems["EXISTS"] = nil
}

// lexGetDVI is F$GETDVI(device-name, item): the item (dviItems) for the
// device, named as $GETDVI takes it (a device or a logical name for one).
// A name that is no device is SS$_NOSUCHDEV, except for EXISTS, which is
// then "FALSE".
func lexGetDVI(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	value, key, err := lexicalKeyword(dviItems, args[1])
	if err != nil {
		return dclValue{}, err
	}

	device := strings.TrimSpace(args[0].str())

	if key == "EXISTS" {
		_, status := env.DVIItem(device, "DVI$_DEVCLASS")

		return dclTrueFalse(status == 0), nil
	}

	data, status := env.DVIItem(device, "DVI$_"+key)
	if status != 0 {
		return dclValue{}, e.console.statusFailure(status)
	}

	return value(data, env), nil
}

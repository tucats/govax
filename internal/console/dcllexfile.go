package console

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The lexical functions on files and devices (docs/PHASE-50 - DCL command
// procedures.md, subtask 14; the User's Manual, 15.4): F$PARSE, F$SEARCH,
// and F$GETDVI. F$PARSE and F$SEARCH apply RMS's rules (rms.Session's
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
// a search the stream starts again, so a name with no wildcards is found
// on every other call (unconfirmed against VMS). On a mounted volume the
// files are RMS's, written "DUA0:[WORK]LOGIN.COM;3"; on the host they
// are host paths, matched without regard to case.
func lexSearch(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console
	spec, id := args[0].str(), args[1].int()

	if c.searchStreams == nil {
		c.searchStreams = map[int32]*searchStream{}
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
func (c *Console) searchFiles(spec string) ([]string, error) {
	loc, err := c.ContainerSession.Locate(spec, false)
	if err != nil {
		return nil, nil //nolint:nilerr // F$SEARCH finds nothing there
	}

	if loc.Host {
		return hostSearch(loc.Name), nil
	}

	files, err := c.ContainerSession.Search(spec)
	if err != nil {
		return nil, fileFailure(err, spec)
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

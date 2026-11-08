package lbrrtl

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/lnm"
)

// The help routines run on internal/help, the engine DCL's HELP and the
// debugger's use: LBR$GET_HELP shows what a request's keys find in an open
// library, a line at a time, and LBR$OUTPUT_HELP is DCL's HELP whole, with
// the program's own routines to write and read.

// HLP$M_ flags (LBR$OUTPUT_HELP's flags argument; testdata/help90/defined.txt).
const (
	hlpPrompt  = 0x01
	hlpProcess = 0x02
	hlpGroup   = 0x04
	hlpSystem  = 0x08
	hlpLibList = 0x10
	hlpHelp    = 0x20
)

// defaultWidth is the line width when a routine is given none, or 0.
const defaultWidth = 80

// maxKeys is how many keys LBR$GET_HELP takes (key_1 to key_10).
const maxKeys = 10

// width reads a width argument passed by reference: defaultWidth if it's
// omitted or 0.
func width(env *corevms.Environment, addr uint32) (int, error) {
	w, err := loadOptional(env, addr, 0)
	if err != nil || w == 0 {
		return defaultWidth, err
	}

	return int(int32(w)), nil
}

// helpLibrary is c's library as the help engine reads it, made on first
// need.
func (c *control) helpLibrary() help.Library {
	if c.help == nil {
		c.help = help.NewLBRLibrary(c.lib)
	}

	return c.help
}

// getHelp is LBR$GET_HELP library_index [,line_width] [,routine] [,data]
// [,key_1] ... [,key_10]: the help text the keys name, as DCL's HELP finds
// and lays it out (help.Lookup and help.Lines), each line passed to routine
// with four arguments: a descriptor of the line, without the blanks HELP
// would put before it; the address of its flags (HLP$M_NOHLPTXT,
// HLP$M_KEYNAMLIN, HLP$M_OTHERINFO); data, or the address of a 0 if data is
// omitted; and the address of its level. Without a routine each line goes to
// SYS$OUTPUT as HELP shows it, two more blanks in (VMS 7.1's showed
// "  ALPHA" and "    Examples" for "ALPHA Examples"). A routine's failure
// status stops it and is returned. As VMS 7.1's did (testdata/help90's
// LBR90, round 1b):
//
//   - no key_1, or HELP alone, is the HELP topic followed by the library's
//     topics, as DCL's HELP shows for no keys (DCL's "HELP HELP" lists the
//     HELP topic's own subtopics instead);
//   - more than ten keys is LBR$_INVNAM;
//   - a library that isn't a help library is LBR$_NOTHLPLIB.
//
// A key list ends at the first key whose descriptor's address, length, or
// text's address is 0 (the manual).
func getHelp(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > 4+maxKeys {
		return lbrInvNam, nil
	}

	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if c.lib.Type != lbr.TypeHelp {
		return lbrNotHlpLib, nil
	}

	w, err := width(env, arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	words, err := helpKeys(env, argv[min(len(argv), 4):])
	if err != nil {
		return librtl.StringFailure(env, err)
	}

	var displays []help.Display

	if len(words) == 0 || len(words) == 1 && strings.EqualFold(words[0], "HELP") {
		displays, err = help.Lookup(c.helpLibrary(), nil, help.Options{Instructions: true})
	} else {
		displays, err = help.Lookup(c.helpLibrary(), words, help.Options{})
	}

	if errors.Is(err, help.ErrInvalidKey) {
		return lbrInvKey, nil
	}

	routine, data := arg(argv, 2), arg(argv, 3)

	for _, line := range help.Lines(displays, w) {
		if routine == 0 {
			text := line.String()
			if text != "" {
				text = "  " + text
			}

			if _, err := env.PutOutput(text); err != nil {
				return 0, err
			}

			continue
		}

		r0, err := callWith(env, routine, func() ([]uint32, error) {
			return helpLineArguments(env, line, data)
		})
		if err != nil {
			return librtl.MemoryFault(env, err)
		}

		if r0&1 == 0 {
			return r0, nil
		}
	}

	return ssNormal, nil
}

// helpKeys reads LBR$GET_HELP's key descriptors, up to the first that ends
// the list.
func helpKeys(env *corevms.Environment, keys []uint32) ([]string, error) {
	mem, cpu := env.Memory(), env.CPU()

	var words []string

	for _, desc := range keys {
		if desc == 0 {
			break
		}

		n, err := mem.LoadWord(cpu, desc)
		if err != nil {
			return nil, err
		}

		text, err := mem.LoadLongword(cpu, desc+4)
		if err != nil {
			return nil, err
		}

		if n == 0 || text == 0 {
			break
		}

		key, err := librtl.ReadString(env, desc)
		if err != nil {
			return nil, err
		}

		words = append(words, key)
	}

	return words, nil
}

// helpLineArguments puts a line's arguments for LBR$GET_HELP's routine
// below the stack pointer.
func helpLineArguments(env *corevms.Environment, line help.Line, data uint32) ([]uint32, error) {
	desc, err := env.StackString(line.Text)
	if err != nil {
		return nil, err
	}

	flags, err := below(env, longword(uint32(line.Flags)))
	if err != nil {
		return nil, err
	}

	level, err := below(env, longword(uint32(line.Level)))
	if err != nil {
		return nil, err
	}

	if data == 0 {
		if data, err = below(env, longword(0)); err != nil {
			return nil, err
		}
	}

	return []uint32{desc, flags, data, level}, nil
}

// errStopped ends an LBR$OUTPUT_HELP session whose routine failed.
var errStopped = errors.New("lbrrtl: a routine failed")

// inputSize is the buffer LBR$OUTPUT_HELP gives its input routine for a
// response.
const inputSize = 256

// outputHelp is LBR$OUTPUT_HELP output_routine [,output_width] [,line_desc]
// [,library_name] [,flags] [,input_routine]: DCL's HELP (help.Session), its
// lines written by output_routine (called as LIB$PUT_OUTPUT is) and its
// prompts' responses read by input_routine (called as LIB$GET_INPUT is,
// each prompt starting with a carriage return and line feed, as VMS 7.1's
// did). The request is line_desc's keys; the main library is library_name,
// with the defaults SYS$HELP:HELPLIB.HLB; the user libraries are the
// HLP$LIBRARY names of the tables the flags allow. HLP$M_PROMPT prompts,
// HLP$M_LIBLIST lists the user libraries, HLP$M_HELP shows the HELP topic
// before the topics when there are no keys. Without a flags argument every
// library and prompting are on (the manual), so an input routine is needed.
// As VMS 7.1's did (testdata/help90's LBR90, round 1b):
//
//   - no output routine is LBR$_ILLOUTROU, prompting with no input
//     routine LBR$_ILLINROU, and more than six arguments LBR$_TOOMNYARG;
//   - the end of the responses (RMS$_EOF from the input routine) ends HELP
//     with SS$_NORMAL, and another failure is LBR$_USRINPERR;
//   - the output routine's failure ends HELP, returning its status.
func outputHelp(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > 6 {
		return lbrTooMnyArg, nil
	}

	output, input := arg(argv, 0), arg(argv, 5)

	if output == 0 {
		return lbrIllOutRou, nil
	}

	flags := uint32(hlpPrompt | hlpProcess | hlpGroup | hlpSystem)

	if arg(argv, 4) != 0 {
		var err error
		if flags, err = env.Memory().LoadLongword(env.CPU(), arg(argv, 4)); err != nil {
			return librtl.MemoryFault(env, err)
		}
	}

	if flags&hlpPrompt != 0 && input == 0 {
		return lbrIllInRou, nil
	}

	w, err := width(env, arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	var request, main string

	if arg(argv, 2) != 0 {
		if request, err = librtl.ReadString(env, arg(argv, 2)); err != nil {
			return librtl.StringFailure(env, err)
		}
	}

	if arg(argv, 3) != 0 {
		if main, err = librtl.ReadString(env, arg(argv, 3)); err != nil {
			return librtl.StringFailure(env, err)
		}
	}

	if main == "" {
		main = "SYS$HELP:HELPLIB"
	}

	lib, _, err := openHelpLibrary(env, main)
	if err != nil {
		stateOf(env).stv = ssNoFile

		return rmsFNF, nil
	}

	// status is what the routine returns; a routine's failure sets it, and
	// fault a memory error, either of which stops the session.
	var (
		status uint32 = ssNormal
		fault  error
	)

	stopped := func() bool { return status&1 == 0 || fault != nil }

	s := &help.Session{
		Library: lib,
		Open:    func(spec string) (help.Library, string, error) { return openHelpLibrary(env, spec) },
		Width:   w,
		Options: help.Options{Instructions: flags&hlpHelp != 0},
		Prompt:  flags&hlpPrompt != 0,
		Users:   userLibraries(env, flags),
		LibList: flags&hlpLibList != 0,
	}

	s.Write = func(line string) {
		if stopped() {
			return
		}

		r0, err := callWith(env, output, func() ([]uint32, error) {
			desc, err := env.StackString(line)

			return []uint32{desc}, err
		})

		switch {
		case err != nil:
			fault = err
		case r0&1 == 0:
			status = r0
		}
	}

	s.Read = func(prompt string) (string, error) {
		if stopped() {
			return "", errStopped
		}

		answer, r0, err := readResponse(env, input, "\r\n"+prompt)

		switch {
		case err != nil:
			fault = err

			return "", errStopped
		case r0 == rmsEOF:
			return "", io.EOF
		case r0&1 == 0:
			status = lbrUsrInpErr

			return "", errStopped
		}

		return answer, nil
	}

	_ = s.Run(help.Split(request))

	if fault != nil {
		return librtl.MemoryFault(env, fault)
	}

	return status, nil
}

// readResponse calls input, an input routine, as LIB$GET_INPUT is called:
// a fixed string of inputSize bytes for the response, the prompt, and the
// address of a word for the response's length. It returns the response and
// the routine's R0.
func readResponse(env *corevms.Environment, input uint32, prompt string) (string, uint32, error) {
	var buffer, length uint32

	r0, err := callWith(env, input, func() ([]uint32, error) {
		var err error

		if buffer, err = below(env, make([]byte, inputSize)); err != nil {
			return nil, err
		}

		desc, err := below(env, append(longword(inputSize|dscClassS<<24|dscTypeT<<16), longword(buffer)...))
		if err != nil {
			return nil, err
		}

		p, err := env.StackString(prompt)
		if err != nil {
			return nil, err
		}

		if length, err = below(env, longword(0)); err != nil {
			return nil, err
		}

		return []uint32{desc, p, length}, nil
	})
	if err != nil || r0&1 == 0 {
		return "", r0, err
	}

	mem, cpu := env.Memory(), env.CPU()

	n, err := mem.LoadWord(cpu, length)
	if err != nil {
		return "", r0, err
	}

	text := make([]byte, min(int(n), inputSize))
	for i := range text {
		if text[i], err = mem.LoadByte(cpu, buffer+uint32(i)); err != nil {
			return "", r0, err
		}
	}

	return string(text), r0, nil
}

// A descriptor's class and type (DSC$K_CLASS_S, DSC$K_DTYPE_T).
const (
	dscClassS = 1
	dscTypeT  = 14
)

// openHelpLibrary opens the help library spec names, with the defaults
// SYS$HELP:.HLB, and returns it and the name HELP's prompts show for it:
// its file's name, without device, directory, type, or version (H90B for
// H90DIR:H90B).
func openHelpLibrary(env *corevms.Environment, spec string) (help.Library, string, error) {
	data, found, err := readLibraryFile(env.Session, spec, "SYS$HELP:.HLB")
	if err != nil {
		return nil, "", err
	}

	lib, err := lbr.Open(data)
	if err != nil {
		return nil, "", err
	}

	if lib.Type != lbr.TypeHelp {
		return nil, "", errors.New("lbrrtl: not a help library")
	}

	return help.NewLBRLibrary(lib), fileName(found.Name), nil
}

// fileName is a file specification's name alone.
func fileName(spec string) string {
	if at := strings.LastIndexAny(spec, "]>:/"); at >= 0 {
		spec = spec[at+1:]
	}

	if at := strings.IndexByte(spec, ';'); at >= 0 {
		spec = spec[:at]
	}

	return strings.ToUpper(strings.TrimSuffix(spec, path.Ext(spec)))
}

// userLibraries are the user libraries the flags allow: in the process,
// group, and system tables in that order, HLP$LIBRARY, HLP$LIBRARY_1, ...,
// up to the first name that isn't defined, each that can be opened (as
// DCL's HELP finds them, internal/dclcmd's userHelpLibraries).
func userLibraries(env *corevms.Environment, flags uint32) []help.NamedLibrary {
	if env.Logicals == nil {
		return nil
	}

	var libs []help.NamedLibrary

	for _, table := range []struct {
		flag uint32
		name string
	}{
		{hlpProcess, lnm.ProcessTableName}, {hlpGroup, "LNM$GROUP"}, {hlpSystem, lnm.SystemTableName},
	} {
		if flags&table.flag == 0 {
			continue
		}

		for n := 0; ; n++ {
			name := "HLP$LIBRARY"
			if n > 0 {
				name = fmt.Sprintf("HLP$LIBRARY_%d", n)
			}

			e, err := env.Logicals.Translate(table.name, name, lnm.User, lnm.AttrCaseBlind)
			if err != nil || len(e.Equivalences) == 0 {
				break
			}

			if lib, libName, err := openHelpLibrary(env, e.Equivalences[0].Value); err == nil {
				libs = append(libs, help.NamedLibrary{Name: libName, Library: lib})
			}
		}
	}

	return libs
}

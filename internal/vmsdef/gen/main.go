// Command gen adds VMS definitions to govax's own tables in
// internal/vmsdef: the symbolic constants in Symbols
// (symbols_generated.go), the message texts in Messages and
// MessageFacilities (messages_generated.go), and shareable images'
// global symbol tables in SharedImages and ImageSymbols
// (images_generated.go), and the absolute symbols of object libraries'
// definition modules in LibrarySymbols (library_generated.go). It reads
// definition files of
// the kinds VMS ships, merges what they define into the tables govax
// already has, and rewrites the generated files, sorted, so that a merge
// reads as a diff.
//
// govax's tables are committed, and building govax doesn't run gen: the
// VMS files it reads are licensed, and they aren't part of this
// repository (docs/PHASE-31.md). Run it when govax needs definitions it
// doesn't have yet, from the repository root:
//
//	go run ./internal/vmsdef/gen -bliss ~/vms/ssdef.txt -sdl ~/vms/iodef.sdl
//
// Each input is a flag naming a file, and they're read in the order
// given:
//
//	-h FILE      a C header's "#define NAME value" lines (fabdef.h; see header.go)
//	-sdl FILE    an SDL source's modules (lnmdef.sdl; see sdl.go)
//	-bliss FILE  a BLISS LITERAL listing (ssdef.txt; see bliss.go)
//	-msg FILE    a message file's listing (sysmsg.txt; see msg.go)
//	-image FILE  a shareable image's global symbol table (librtl.exe; see images.go)
//	-olb FILE    an object library's definition modules (starlet.olb; see library.go)
//
// Three further flags change how the symbol and library inputs after them
// are read: -prefix P keeps only the names that begin with P (and
// -prefix "" keeps all of them again); -sdl-stop MODULE reads an SDL
// source only as far as "module MODULE;" (objfmt.sdl's Alpha definitions
// begin at $EOBJRECDEF); and -into symbols merges an object library's
// symbols into Symbols rather than LibrarySymbols (-into library, the
// default). That's how STARLET's values correct or extend govax's own:
//
//	go run ./internal/vmsdef/gen -replace -into symbols -prefix 'SS$_' -olb starlet.olb
//
// -drop NAME removes a name from Symbols: one VMS no longer defines, such
// as an obsolete code whose value a newer one took. It may be repeated.
//
// A name the table has already, with the same value, is left alone. One
// with a different value is an error, which names every such conflict,
// unless -replace is given. Messages merge the same way, by condition
// value, and facilities by number, and so do images, by name, and their
// symbols, and libraries' symbols by name. -n reports what would be added
// or changed and writes nothing. Each input's file name is added to
// SymbolSources, MessageSources, ImageSources, or LibrarySources, a record
// of what each table was built from.
//
// With no inputs, gen rewrites the tables as they are; tests check that
// the results are the committed files, byte for byte.
package main

import (
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// input is one definition file to merge, as the command line gives it.
type input struct {
	kind   string // "h", "sdl", "bliss", "msg", "image", or "olb"
	path   string
	prefix string // keep only names with this prefix; "" keeps all
	stop   string // for "sdl", the module to stop reading at, if any
	into   string // for "olb", the table to merge into: "symbols", or "library" ("")
}

// inputList collects the inputs in command-line order, with the -prefix,
// -sdl-stop, and -into settings in effect at each.
type inputList struct {
	inputs []input
	prefix string
	stop   string
	into   string
}

// inputFlag is one of the flags that names an input of kind.
type inputFlag struct {
	list *inputList
	kind string
}

func (f inputFlag) String() string { return "" }

func (f inputFlag) Set(path string) error {
	f.list.inputs = append(f.list.inputs, input{kind: f.kind, path: path, prefix: f.list.prefix, stop: f.list.stop, into: f.list.into})

	return nil
}

// dropList is the names -drop gives.
type dropList []string

func (d *dropList) String() string { return strings.Join(*d, ",") }

func (d *dropList) Set(name string) error {
	*d = append(*d, name)

	return nil
}

// settingFlag is -prefix, -sdl-stop, or -into: it changes a setting for
// the inputs after it.
type settingFlag struct{ value *string }

func (f settingFlag) String() string { return "" }

func (f settingFlag) Set(v string) error {
	*f.value = v

	return nil
}

func main() {
	var list inputList

	flag.Var(inputFlag{&list, "h"}, "h", "merge a C header's #define constants")
	flag.Var(inputFlag{&list, "sdl"}, "sdl", "merge an SDL source's definitions")
	flag.Var(inputFlag{&list, "bliss"}, "bliss", "merge a BLISS LITERAL listing's literals")
	flag.Var(inputFlag{&list, "msg"}, "msg", "merge a message listing's facilities and messages")
	flag.Var(inputFlag{&list, "image"}, "image", "merge a shareable image's global symbol table")
	flag.Var(inputFlag{&list, "olb"}, "olb", "merge an object library's definition modules' symbols")
	flag.Var(settingFlag{&list.prefix}, "prefix", "keep only names with this prefix, in the inputs after it")
	flag.Var(settingFlag{&list.stop}, "sdl-stop", "read the SDL sources after it only as far as this module")
	flag.Var(settingFlag{&list.into}, "into", "merge the object libraries after it into \"symbols\" (Symbols) or \"library\" (LibrarySymbols)")

	var drops dropList

	flag.Var(&drops, "drop", "remove this name from Symbols (repeatable)")

	replace := flag.Bool("replace", false, "let an input change a value the table already has")
	dryRun := flag.Bool("n", false, "report what would change, and write nothing")
	dir := flag.String("dir", filepath.Join("internal", "vmsdef"), "the directory of internal/vmsdef's generated files")
	flag.Parse()

	if flag.NArg() > 0 {
		log.Fatalf("gen: unexpected argument %q: each input is given by -h, -sdl, -bliss, -msg, -image, or -olb", flag.Arg(0))
	}

	symbolsPath := filepath.Join(*dir, symbolsFile)
	messagesPath := filepath.Join(*dir, messagesFile)
	imagesPath := filepath.Join(*dir, imagesFile)
	libraryPath := filepath.Join(*dir, libraryFile)

	for _, path := range []string{symbolsPath, messagesPath, imagesPath, libraryPath} {
		if _, err := os.Stat(path); err != nil {
			log.Fatalf("gen: %v (run gen from the repository root, or give -dir)", err)
		}
	}

	symbols := maps.Clone(vmsdef.Symbols)
	symbolSources := append([]string(nil), vmsdef.SymbolSources...)
	messages := maps.Clone(vmsdef.Messages)
	facilities := maps.Clone(vmsdef.MessageFacilities)
	messageSources := append([]string(nil), vmsdef.MessageSources...)
	images := maps.Clone(vmsdef.SharedImages)
	imageSymbols := maps.Clone(vmsdef.ImageSymbols)
	imageSources := append([]string(nil), vmsdef.ImageSources...)
	librarySymbols := maps.Clone(vmsdef.LibrarySymbols)
	librarySources := append([]string(nil), vmsdef.LibrarySources...)

	var conflicts []string

	for _, in := range list.inputs {
		var r mergeResult

		file := filepath.Base(in.path)

		switch in.kind {
		case "msg":
			msgs, facs, err := parseMessages(readSource(in.path))
			if err != nil {
				log.Fatalf("gen: %s: %v", in.path, err)
			}

			r = mergeMessages(messages, facilities, msgs, facs, *replace)
			messageSources = addSource(messageSources, file)

		case "image":
			name, image, defs, err := readImage([]byte(readSource(in.path)))
			if err != nil {
				log.Fatalf("gen: %s: %v", in.path, err)
			}

			r = mergeImage(images, imageSymbols, name, image, defs, *replace)
			imageSources = addSource(imageSources, file)

		case "olb":
			defs, modules, err := readLibrary([]byte(readSource(in.path)), in.prefix)
			if err != nil {
				log.Fatalf("gen: %s: %v", in.path, err)
			}

			fmt.Fprintf(os.Stderr, "gen: %s: %d definition modules\n", in.path, modules)

			switch in.into {
			case "", "library":
				r = mergeSymbols(librarySymbols, defs, *replace)
				librarySources = addSource(librarySources, file)

			case "symbols":
				r = mergeSymbols(symbols, defs, *replace)
				symbolSources = addSource(symbolSources, file)

			default:
				log.Fatalf("gen: -into %q: the table is \"symbols\" or \"library\"", in.into)
			}

		default:
			defs, err := in.read()
			if err != nil {
				log.Fatalf("gen: %s: %v", in.path, err)
			}

			r = mergeSymbols(symbols, defs, *replace)
			symbolSources = addSource(symbolSources, file)
		}

		conflicts = append(conflicts, r.conflicts...)

		fmt.Fprintf(os.Stderr, "gen: %s: %d added, %d changed, %d already present\n", in.path, len(r.added), len(r.changed), r.same)

		if *dryRun {
			for _, a := range r.added {
				fmt.Printf("add %s\n", a)
			}

			for _, c := range r.changed {
				fmt.Printf("change %s\n", c)
			}
		}
	}

	for _, name := range drops {
		v, ok := symbols[name]
		if !ok {
			log.Fatalf("gen: -drop %s: Symbols has no such name", name)
		}

		delete(symbols, name)
		fmt.Fprintf(os.Stderr, "gen: dropped %s (%#x)\n", name, v)
	}

	if len(conflicts) > 0 {
		log.Fatalf("gen: %d definitions already have other values (use -replace to change them):\n\t%s", len(conflicts), strings.Join(conflicts, "\n\t"))
	}

	if *dryRun {
		return
	}

	if err := os.WriteFile(symbolsPath, generateSymbols(symbols, symbolSources), 0o644); err != nil {
		log.Fatalf("gen: %v", err)
	}

	if err := os.WriteFile(messagesPath, generateMessages(messages, facilities, messageSources), 0o644); err != nil {
		log.Fatalf("gen: %v", err)
	}

	if err := os.WriteFile(imagesPath, generateImages(images, imageSymbols, imageSources), 0o644); err != nil {
		log.Fatalf("gen: %v", err)
	}

	if err := os.WriteFile(libraryPath, generateLibrary(librarySymbols, librarySources), 0o644); err != nil {
		log.Fatalf("gen: %v", err)
	}

	fmt.Fprintf(os.Stderr, "gen: wrote %d symbols, %d messages in %d facilities, %d images' %d symbols, and %d library symbols to %s\n",
		len(symbols), len(messages), len(facilities), len(images), len(imageSymbols), len(librarySymbols), *dir)
}

func readSource(path string) string {
	src, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("gen: %v", err)
	}

	return string(src)
}

// read parses the input's file.
func (in input) read() (map[string]uint32, error) {
	data, err := os.ReadFile(in.path)
	if err != nil {
		return nil, err
	}

	src := string(data)

	switch in.kind {
	case "h":
		return parseDefines(src, in.prefix), nil

	case "bliss":
		return parseBlissLiterals(src, in.prefix)

	default:
		if in.stop != "" {
			src, _, _ = strings.Cut(src, "module "+in.stop+";")
		}

		defs, err := parseSDL(src)
		if err != nil {
			return nil, err
		}

		return withPrefix(defs, in.prefix), nil
	}
}

// withPrefix returns the definitions in defs whose names begin with
// prefix.
func withPrefix(defs map[string]uint32, prefix string) map[string]uint32 {
	if prefix == "" {
		return defs
	}

	out := map[string]uint32{}

	for name, v := range defs {
		if strings.HasPrefix(name, prefix) {
			out[name] = v
		}
	}

	return out
}

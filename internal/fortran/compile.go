package fortran

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/listing"
	"github.com/tucats/govax/internal/fortran/opt"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// Options are the choices a compilation is made with: the FORTRAN
// command's qualifiers, and what the MACRO file's header says.
type Options struct {
	// ExtendSource is /EXTEND_SOURCE: a fixed-format line's statement
	// field runs to column 132 instead of column 72.
	ExtendSource bool

	// DLines is /D_LINES: lines with a D in column 1 are compiled as
	// statements instead of being treated as comments (VAX FORTRAN's
	// debugging lines).
	DLines bool

	// Continuations is /CONTINUATIONS=n, the most continuation lines a
	// statement may have: 0 means the default (19), and
	// source.NoContinuations means none (/CONTINUATIONS=0).
	Continuations int

	// NoI4 is /NOI4: INTEGER and LOGICAL with no length are 2 bytes.
	NoI4 bool

	// GFloating is /G_FLOATING: REAL*8 is G_floating, not D_floating.
	GFloating bool

	// Align is /[NO]ALIGN (sema/align.go).
	Align sema.Alignment

	// Checks is /CHECK: what the compiled code looks out for (sema.Checks).
	Checks sema.Checks

	// Warnings is /WARNINGS: what the compiler says, beyond errors (diag.Warnings).
	Warnings diag.Warnings

	// Standard is /STANDARD: what to flag as an extension to FORTRAN-77 (sema.Conformance).
	Standard sema.Conformance

	// Debug is /DEBUG: what MACRO is to write for the debugger (debug.go).
	Debug Debug

	// Optimize is /OPTIMIZE: run the optimizer's passes on the IR (package opt). The FORTRAN
	// command sets it unless it was given /NOOPTIMIZE.
	Optimize bool

	// List is /LIST: make a listing (Result.Listing), and Show is what /SHOW says it
	// holds (listing.DefaultShow with no /SHOW).
	List bool
	Show listing.Show

	// CrossReference is /CROSS_REFERENCE: a listing ends each unit with where its names
	// and labels are defined, assigned, and used.
	CrossReference bool

	// Includes reads the files and modules INCLUDE statements name; nil
	// means a program can include nothing.
	Includes source.Opener

	// Version is govax's version ("1.0-115"), named in the header as the
	// compiler's version.
	Version string

	// Compiled is when the compilation happened, for the header. Tests
	// fix it so the output doesn't change from run to run.
	Compiled time.Time

	// CommandLine is the command as typed, for the header.
	CommandLine string
}

// File is a source file to compile.
type File struct {
	// Spec is the file's full specification, as messages and the header
	// show it: a volume file's with its version (DUA0:[WORK]SQUARES.FOR;1)
	// or a host file's path.
	Spec string

	// Name is the file's name without its directory, type, or version
	// (SQUARES), from which a main program without a PROGRAM statement
	// is named (SQUARES$MAIN), and the MACRO module of a file with no
	// program units.
	Name string

	// Lines are the file's records, without line terminators.
	Lines []string

	// Handle is the caller's note of where the file is, handed to
	// Options.Includes to find the files it includes relative to it.
	Handle any
}

// Result is what a compilation produced.
type Result struct {
	// MACRO is the MACRO-32 source file's lines, without line
	// terminators; nil when the compilation failed (an error or fatal
	// message), since then there is no file to write.
	MACRO []string

	// Listing is the listing's lines, each page's first starting with a form feed:
	// made with Options.List, whether or not the compilation failed, when it's
	// wanted most.
	Listing []string

	// Messages are the messages reported, in order.
	Messages []diag.Message

	// Errors and Warnings count the messages by severity: Errors those
	// that stop the MACRO file being written (errors and fatal ones),
	// Warnings the warnings.
	Errors   int
	Warnings int
}

// Compile compiles one source file.
func Compile(opts Options, file File) Result {
	return CompileAll(opts, []File{file})
}

// CompileAll compiles source files joined into one compilation (FORTRAN A+B):
// their statements are one sequence, so a program unit may even run on from
// one file into the next. The first file names what a file's name names.
func CompileAll(opts Options, files []File) (result Result) {
	var (
		messages diag.List
		routines []*ir.Routine
		ident    string
		tree     *ast.File
		prog     *sema.Program
		maps     []codegen.StorageMap
	)

	file := files[0]

	sources := make([]*source.File, len(files))
	for i, f := range files {
		sources[i] = &source.File{Spec: f.Spec, Lines: f.Lines, Handle: f.Handle}
	}

	// The first pass: the source's lines into statements.
	program := source.ReadAll(source.Form{
		ExtendSource:  opts.ExtendSource,
		DLines:        opts.DLines,
		Continuations: opts.Continuations,
	}, sources, opts.Includes, &messages)

	sema.ReportSourceExtensions(program, opts.Standard, &messages)

	if opts.Debug.Symbols && opts.Optimize {
		messages.Report("DEBUGOPTIMIZE", diag.Position{})
	}

	// The second, third, and fourth passes: each statement's kind and
	// tokens, then the syntax tree, then its meaning. Semantic analysis
	// runs even after the parser's errors, since the tree keeps a
	// statement with an error (and its label) in its place, so that one
	// compilation reports as much as it can. Then, for a program with no
	// errors, the fifth: lowering each unit to the IR.
	if !messages.Failed() {
		tree = parse.Parse(lex.Lex(program.Statements, &messages), &messages)

		align := opts.Align
		align.Warn = align.Warn || opts.Warnings.Alignment

		prog = sema.Analyze(tree, MainName(file.Name), sema.Options{
			NoI4:      opts.NoI4,
			GFloating: opts.GFloating,
			Align:     align,
			Checks:    opts.Checks,
			Warnings:  opts.Warnings,
			Standard:  opts.Standard,
		}, &messages)

		if !messages.Failed() {
			routines = ir.Lower(prog)
			ident = prog.Ident

			if opts.Optimize {
				opt.Optimize(routines)
			}
		}
	}

	// One place holds back what /WARNINGS doesn't want, so that every pass reports
	// alike, and the count, the status, and a listing all go by what's shown.
	messages.Drop(opts.Warnings.Held)

	result = Result{
		Messages: messages.Messages(),
		Errors:   messages.Count(diag.Error) + messages.Count(diag.Fatal),
		Warnings: messages.Count(diag.Warning),
	}

	// A listing is the last thing made, with what the passes that ran have left.
	defer func() {
		if opts.List {
			result.Listing = listing.Make(listingInput(opts, file, program, result.Messages, tree, prog, maps))
		}
	}()

	if messages.Failed() {
		return result
	}

	// The last pass: the MACRO file. Its module is named for the first
	// program unit (Decision 3), or for the source file if it has none.
	module := ModuleName(file.Name)
	if len(routines) > 0 {
		module = routines[0].Name
	}

	var w codegen.Writer

	writeHeader(&w, opts, files, module, ident)

	var generating codegen.Options
	if opts.Optimize {
		generating.Improve = opt.Improve
	}

	generating.DebugTable = opts.Debug.Symbols
	maps = codegen.Generate(&w, routines, program.Lines, generating)
	result.MACRO = w.Lines()

	return result
}

// writeHeader writes the MACRO file's opening: the module's .TITLE and
// .IDENT, then comments saying what made the file, from what, and when.
// ident is the file's CDEC$ IDENT string, "" with none.
func writeHeader(w *codegen.Writer, opts Options, files []File, module, ident string) {
	w.Statement("", ".TITLE", module, "")
	w.Statement("", ".IDENT", identOperand(ident), "")
	opts.Debug.writeDirectives(w)
	w.Comment("")
	w.Comment("Compiled by govax FORTRAN V" + opts.Version + " on " + vmsTime(opts.Compiled))

	for _, file := range files {
		w.Comment("from " + file.Spec)
	}

	if opts.CommandLine != "" {
		w.Comment("by the command " + opts.CommandLine)
	}

	if opts.Optimize {
		w.Comment("Optimized by /OPTIMIZE's passes: " + strings.Join(opt.Names(), ", "))
	}

	w.Comment("")
	w.Comment("Each line of the FORTRAN source appears as a comment, with its line number, before")
	w.Comment("the instructions it became. An INCLUDE file's lines appear only with INCLUDE's")
	w.Comment("/LIST, their include depth before the number; a comment names the file either way.")
	w.Comment("Each routine's storage map says where its variables are.")
	w.Comment("")
}

// identOperand is .IDENT's operand for ident: the string between delimiters it
// doesn't hold, which MACRO lets be any printing character but <, ;, and =. It
// holds at most 31 characters, so one is always free. MACRO's ident can't be
// empty, so a file with none, or with CDEC$ IDENT ”, has 01 (ours, unconfirmed).
func identOperand(ident string) string {
	if ident == "" {
		ident = "01"
	}

	for c := byte('/'); ; c++ {
		if c > '~' {
			c = '!'
		}

		if c != '<' && c != ';' && c != '=' && strings.IndexByte(ident, c) < 0 {
			return string(c) + ident + string(c)
		}
	}
}

// ModuleName is the MACRO module name for a source file named name: the
// name in capitals, any character a MACRO symbol can't hold made an
// underscore, at most 31 characters (MACRO's limit for a symbol), and
// not starting with a digit. A source file with no usable name gives
// FORTRAN.
func ModuleName(name string) string {
	var b strings.Builder

	for _, c := range strings.ToUpper(name) {
		switch {
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '$', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}

	module := b.String()

	switch {
	case module == "":
		module = "FORTRAN"
	case module[0] >= '0' && module[0] <= '9':
		module = "F_" + module
	}

	if len(module) > 31 {
		module = module[:31]
	}

	return module
}

// MainName is the name of a main program without a PROGRAM statement in
// the source file named name (section 4.12): the file's name, cut to 26
// characters, with $MAIN after it, so SQUARES.FOR's is SQUARES$MAIN.
func MainName(name string) string {
	module := ModuleName(name)
	if len(module) > 26 {
		module = module[:26]
	}

	return module + "$MAIN"
}

// vmsTime is t as VMS shows a date and time: 29-OCT-2026 14:05:11.20.
func vmsTime(t time.Time) string {
	return strings.ToUpper(t.Format("2-Jan-2006 15:04:05.00"))
}

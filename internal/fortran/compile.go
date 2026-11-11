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

	// NoF77 is /NOF77: FORTRAN-66's reading of what FORTRAN-77 changed (sema.Unit.NoF77).
	NoF77 bool

	// Recursive is /RECURSIVE: every subprogram may be active more than once at a
	// time, as if each FUNCTION and SUBROUTINE statement said RECURSIVE
	// (sema.Unit.Recursive; docs/PHASE-89 - reentrant FORTRAN.md).
	Recursive bool

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
//
// This is the compiler's driver, and the best place to see the passes in
// order. Each pass's output is kept in a local variable below, because the
// listing (made at the very end) shows something from nearly every one.
//
// The result is a named result parameter, (result Result), rather than an
// unnamed one. That matters for the deferred function further down: a
// deferred function runs after the return statement has set the result,
// and because it can name result, it can still add the listing to it.
func CompileAll(opts Options, files []File) (result Result) {
	// What each pass leaves. Any of these may stay nil (or empty) when an
	// earlier pass found errors and the later ones never ran.
	var (
		messages diag.List            // every pass reports here
		routines []*ir.Routine        // the IR: one routine per program unit
		ident    string               // the CDEC$ IDENT text, for .IDENT
		tree     *ast.File            // the parser's syntax tree
		prog     *sema.Program        // semantic analysis's units and symbols
		maps     []codegen.StorageMap // where each variable went, for /LIST
	)

	// The first file names the compilation: its module, and its main
	// program when there's no PROGRAM statement.
	file := files[0]

	// The source pass works on its own type, source.File, which holds just
	// what it needs, so it doesn't depend on this package.
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

	// /STANDARD's complaints about the layout of lines (tab format, long
	// names, and the like) can only be judged from the lines themselves,
	// which the later passes no longer see.
	sema.ReportSourceExtensions(program, opts.Standard, &messages)

	// Optimized code may keep a variable in a register or leave out a store
	// to it, so the debugger may show a stale value. Say so, but go on.
	if opts.Debug.Symbols && opts.Optimize {
		messages.Report("DEBUGOPTIMIZE", diag.Position{})
	}

	// The second, third, and fourth passes: each statement's kind and
	// tokens, then the syntax tree, then its meaning. Semantic analysis
	// runs even after the parser's errors, since the tree keeps a
	// statement with an error (and its label) in its place, so that one
	// compilation reports as much as it can. Then, for a program with no
	// errors, the fifth: lowering each unit to the IR.
	//
	// messages.Failed says whether anything worse than a warning has been
	// reported so far.
	if !messages.Failed() {
		// lex.Lex's result goes straight into parse.Parse: the lexer's
		// tokens aren't kept, because nothing after the parser needs them.
		tree = parse.Parse(lex.Lex(program.Statements, &messages), &messages)

		// /WARNINGS=ALIGNMENT asks for the same warnings as /ALIGN's own
		// warning setting, so fold the two together. align is a copy (Go
		// copies a struct on assignment), so opts itself isn't changed.
		align := opts.Align
		align.Warn = align.Warn || opts.Warnings.Alignment

		prog = sema.Analyze(tree, MainName(file.Name), sema.Options{
			NoI4:      opts.NoI4,
			NoF77:     opts.NoF77,
			Recursive: opts.Recursive,
			GFloating: opts.GFloating,
			Align:     align,
			Checks:    opts.Checks,
			Warnings:  opts.Warnings,
			Standard:  opts.Standard,
		}, &messages)

		// Lowering and optimizing assume a correct program, so they run
		// only when analysis found no errors. The optimizer changes the
		// routines in place; it returns nothing.
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
	// It's wanted whether the compilation fails (to show the messages beside
	// their lines) or succeeds (to show the storage maps the code generator
	// makes below), so it is made in a deferred function, which Go runs on the
	// way out of CompileAll whichever return statement is taken. By then maps
	// holds whatever the code generator left in it, because the closure reads
	// the variable itself, not a copy made when defer was written.
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

	// A Writer collects the MACRO file's lines; its zero value is ready to
	// use, as many Go types are.
	var w codegen.Writer

	writeHeader(&w, opts, files, module, ident)

	// The optimizer has two halves: passes over the IR (run above) and
	// passes over the instructions the code generator chooses. codegen
	// can't import opt (opt imports codegen, and Go forbids import cycles),
	// so the instruction passes are handed to it as a function value,
	// Improve, which it calls on each routine's instructions.
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

	// MACRO's .IDENT /text/ takes as its delimiter whatever character
	// comes first, so /V1.0/ and !V1.0! mean the same. Try printing
	// characters in turn, starting at the conventional /, wrapping from ~
	// back to !, until one is found that the text doesn't contain. The
	// loop has no condition: it ends only by returning, which the comment
	// above shows it always does.
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
	// A strings.Builder builds a string a piece at a time without copying
	// it at each step. Ranging over a string gives its characters as runes
	// (Unicode code points), so a character outside ASCII becomes a single
	// underscore, not one per byte of its UTF-8 encoding.
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
//
// Go's time formats are written as an example of one fixed moment, Monday,
// January 2, 2006 at 15:04:05: each part of that moment stands for the
// same part of t. So "2-Jan-2006" is day-month-year, and ".00" asks for
// hundredths of a second; ToUpper makes "Oct" VMS's "OCT".
func vmsTime(t time.Time) string {
	return strings.ToUpper(t.Format("2-Jan-2006 15:04:05.00"))
}

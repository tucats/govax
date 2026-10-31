package fortran

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/lex"
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

	// Includes reads the files INCLUDE statements name; nil means a
	// program can include nothing.
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
	var (
		messages diag.List
		routines []*ir.Routine
	)

	// The first pass: the source's lines into statements.
	program := source.Read(source.Form{
		ExtendSource:  opts.ExtendSource,
		DLines:        opts.DLines,
		Continuations: opts.Continuations,
	}, &source.File{Spec: file.Spec, Lines: file.Lines, Handle: file.Handle}, opts.Includes, &messages)

	// The second, third, and fourth passes: each statement's kind and
	// tokens, then the syntax tree, then its meaning. Semantic analysis
	// runs even after the parser's errors, since the tree keeps a
	// statement with an error (and its label) in its place, so that one
	// compilation reports as much as it can. Then, for a program with no
	// errors, the fifth: lowering each unit to the IR.
	if !messages.Failed() {
		tree := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
		prog := sema.Analyze(tree, MainName(file.Name), sema.Options{NoI4: opts.NoI4}, &messages)

		if !messages.Failed() {
			routines = ir.Lower(prog)
		}
	}

	result := Result{
		Messages: messages.Messages(),
		Errors:   messages.Count(diag.Error) + messages.Count(diag.Fatal),
		Warnings: messages.Count(diag.Warning),
	}

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

	writeHeader(&w, opts, file, module)
	codegen.Generate(&w, routines, program.Lines)
	result.MACRO = w.Lines()

	return result
}

// writeHeader writes the MACRO file's opening: the module's .TITLE and
// .IDENT, then comments saying what made the file, from what, and when.
func writeHeader(w *codegen.Writer, opts Options, file File, module string) {
	w.Statement("", ".TITLE", module, "")
	w.Statement("", ".IDENT", "/01/", "")
	w.Comment("")
	w.Comment("Compiled by govax FORTRAN V" + opts.Version + " on " + vmsTime(opts.Compiled))
	w.Comment("from " + file.Spec)

	if opts.CommandLine != "" {
		w.Comment("by the command " + opts.CommandLine)
	}

	w.Comment("")
	w.Comment("Each line of the FORTRAN source appears as a comment, with its line number, before")
	w.Comment("the instructions it became; a line from an INCLUDE file has its include depth")
	w.Comment("before the number. Each routine's storage map says where its variables are.")
	w.Comment("")
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

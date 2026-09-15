package console

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	iodev "github.com/tucats/govax/internal/io"
)

// Dispatcher routes one command line to either a fixed-spelling handler
// (matching console_dispatch_table's exact, <=4-character verb spellings —
// e.g. "EXAM"/"EX"/"DUMP" all reach EXAMINE, since read_verb only ever
// looks at a token's first 4 characters) or, if no fixed spelling matches,
// the DCL grammar — matching console_dispatch's own two-tier fallback.
//
// The fixed-spelling set and DCL/verb split here follows console_dispatch_
// table's own -1-vs-real-function split (SHOW, EXIT/QUIT, CLEAR, TEST,
// VMINIT are DCL-driven; EXAMINE, SET, STEP, ... are fixed) with one
// deliberate deviation, documented where it's implemented: DEPOSIT
// (exam.go) is a Go-native addition with no C-source command of its own.
// RUN/R now means what it does in the C source (see run.go's Console.Run,
// Phase 13) — VMS executable-image activation, not plain CPU execution
// (that's EXEC/GO/G, unaffected).
type Dispatcher struct {
	Console *Console
	Grammar *dcl.Grammar
	Help    *Help
}

// NewDispatcher returns a Dispatcher wired to c and g, with every DCL
// verb/syntax this port implements bound to its handler.
func NewDispatcher(c *Console, g *dcl.Grammar, h *Help) *Dispatcher {
	d := &Dispatcher{Console: c, Grammar: g, Help: h}
	d.bindGrammar()
	return d
}

// Dispatch parses and executes one command line, matching console_
// dispatch's own read-verb/fixed-table/DCL-fallback structure.
func (d *Dispatcher) Dispatch(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	verb, _ := readCommandVerb(line)
	verb4 := strings.ToUpper(verb)
	if len(verb4) > 4 {
		verb4 = verb4[:4]
	}

	if h, ok := fixedCommands[verb4]; ok {
		_, rest := readCommandVerb(line)
		return h(d, rest)
	}

	r, err := d.Grammar.Parse(line)
	if err != nil {
		return err
	}
	if r.EntryPoint != "" {
		return fmt.Errorf("console: %s requires the RTL microkernel (not yet implemented, see docs/PHASE-08.md)", r.Active)
	}
	return d.Grammar.Dispatch(r)
}

// readCommandVerb reads the leading command word (up to whitespace, '/',
// ',', or '='), matching read_verb — including its "a leading '@' is a
// token by itself" special case (the INCLUDE-file shorthand).
func readCommandVerb(s string) (verb, rest string) {
	if strings.HasPrefix(s, "@") {
		return "@", s[1:]
	}
	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '/', ',', '=':
			return s[:i], s[i:]
		}
		i++
	}
	return s, ""
}

// bindGrammar binds every DCL verb/syntax this port implements a handler
// for. Everything else (device/RTL/assembler-dependent SHOW/CLEAR/DEFINE
// sub-forms, TEST, CALL's DCL entry — unreachable anyway since CALL is
// intercepted by the fixed table first) is deliberately left unbound:
// Grammar.Dispatch's own "no handler bound" error already reports that
// clearly, so no separate stub code is needed for each one.
func (d *Dispatcher) bindGrammar() {
	g := d.Grammar

	g.Bind("EXIT", func(id int64, r *dcl.Result) error { return d.Console.Quit() })

	g.Bind("VMINIT", func(id int64, r *dcl.Result) error {
		return d.Console.VMInit(
			uint32(r.Int("P0")), uint32(r.Int("P1")), uint32(r.Int("S0")),
			uint32(r.Int("KSP")), uint32(r.Int("ESP")), uint32(r.Int("SSP")), uint32(r.Int("ISP")),
		)
	})

	g.Bind("CLEAR_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol("", true) })
	g.Bind("CLEAR_SYMBOLS", func(id int64, r *dcl.Result) error { return d.Console.ClearSymbol(r.String("P1"), false) })
	g.Bind("CLEAR_BREAK_ALL", func(id int64, r *dcl.Result) error { return d.Console.ClearBreakpoint(0, true) })
	g.Bind("CLEAR_BREAKPOINT", func(id int64, r *dcl.Result) error {
		addr, _, err := d.Console.Evaluator().Eval(r.String("BREAK_ADDR"))
		if err != nil {
			return err
		}
		return d.Console.ClearBreakpoint(addr, false)
	})

	g.Bind("SHOW_REG", func(id int64, r *dcl.Result) error { return d.Console.ShowRegisters() })
	g.Bind("SHOW_PSL", func(id int64, r *dcl.Result) error { return d.Console.ShowPSL() })
	g.Bind("SHOW_MEMORY", func(id int64, r *dcl.Result) error { return d.Console.ShowMemory() })
	g.Bind("SHOW_SYM", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbols() })
	g.Bind("SHOW_SYM_ALL", func(id int64, r *dcl.Result) error { return d.Console.ShowSymbols() })
	g.Bind("SHOW_BREAK", func(id int64, r *dcl.Result) error { return d.Console.ShowBreakpoints() })
	g.Bind("SHOW_RADIX", func(id int64, r *dcl.Result) error { return d.Console.ShowRadix() })
	g.Bind("SHOW_BASE", func(id int64, r *dcl.Result) error { return d.Console.ShowBase() })
	g.Bind("SHOW_CPU", func(id int64, r *dcl.Result) error { return d.Console.ShowCPU() })
	g.Bind("SHOW_VERSION", func(id int64, r *dcl.Result) error { return d.Console.ShowVersion() })
	g.Bind("SHOW_KSP", func(id int64, r *dcl.Result) error { return d.Console.ShowStack(StackKSP) })
	g.Bind("SHOW_ESP", func(id int64, r *dcl.Result) error { return d.Console.ShowStack(StackESP) })
	g.Bind("SHOW_SSP", func(id int64, r *dcl.Result) error { return d.Console.ShowStack(StackSSP) })
	g.Bind("SHOW_ISP", func(id int64, r *dcl.Result) error { return d.Console.ShowStack(StackISP) })
	g.Bind("SHOW_USP", func(id int64, r *dcl.Result) error { return d.Console.ShowStack(StackUSP) })

	// The bare SHOW verb is reached for every show_types keyword with no
	// /syntax= redirect of its own — the plain register/privileged-
	// register name shortcuts (SHOW R0, SHOW PC, SHOW P0BR, ...).
	g.Bind("SHOW", func(id int64, r *dcl.Result) error {
		return d.Console.ShowRegisterOrPrivReg(r.Keyword("SHOW_TYPE"))
	})

	// Phase 09 (internal/io): device abstraction and logical name tables.
	g.Bind("SHOW_DEVICE", func(id int64, r *dcl.Result) error {
		return d.Console.ShowDevices(r.String("NAME"), r.Present("FULL"))
	})
	g.Bind("DEFINE_DEVICE", func(id int64, r *dcl.Result) error {
		d.Console.DefineDevice(r.String("NAME"), iodev.DeviceOptions{
			Cluster:     uint32(r.Int("CLUSTER")),
			Cylinders:   uint32(r.Int("CYLINDERS")),
			DevBufSize:  uint32(r.Int("DEVBUFSIZE")),
			DevChar:     uint32(r.Int("DEVCHAR")),
			DevChar2:    uint32(r.Int("DEVCHAR2")),
			DevClass:    iodev.DeviceClass(r.Int("DEVCLASS")),
			DevDepend:   uint32(r.Int("DEVDEPEND")),
			DevDepend2:  uint32(r.Int("DEVDEPEND2")),
			DevType:     uint32(r.Int("DEVTYPE")),
			FreeBlocks:  uint32(r.Int("FREEBLOCKS")),
			LockID:      uint32(r.Int("LOCKID")),
			MaxBlock:    uint32(r.Int("MAXBLOCK")),
			MaxFiles:    uint32(r.Int("MAXFILES")),
			OwnUIC:      uint32(r.Int("OWNUIC")),
			RecSize:     uint32(r.Int("RECSIZE")),
			Sectors:     uint32(r.Int("SECTORS")),
			Serial:      uint32(r.Int("SERIAL")),
			VolName:     r.String("VOLNAME"),
			MediaName:   r.String("MEDIANAME"),
			MediaType:   r.String("MEDIATYPE"),
			RootDevName: r.String("ROOTDEVNAME"),
		})
		return nil
	})

	g.Bind("SHOW_LOGICAL", func(id int64, r *dcl.Result) error {
		return d.Console.ShowLogicals(r.String("TABLE"), r.String("NAME"))
	})
	g.Bind("DEFINE_LOGICAL", func(id int64, r *dcl.Result) error {
		table := r.String("TABLE")
		if table == "" {
			table = "LNM_PROCESS" // define_logical.c's own default
		}
		return d.Console.DefineLogical(table, r.String("NAME"), r.String("VALUE"))
	})
}

type fixedHandler func(d *Dispatcher, rest string) error

// fixedCommands is the Go equivalent of console_dispatch_table's real
// (non -1) entries — see this file's own top comment for the two
// deliberate deviations (DEPOSIT, RUN/R).
//
// Populated in init() rather than as a plain var initializer: cmdTime
// passes the Dispatcher.Dispatch method (to run a sub-command and time
// it), and Dispatch itself reads fixedCommands — a plain var initializer
// referencing cmdTime would make the compiler see that as an
// initialization cycle (fixedCommands -> cmdTime -> Dispatch ->
// fixedCommands), even though nothing is actually invoked until well after
// package initialization.
var fixedCommands map[string]fixedHandler

func init() {
	fixedCommands = map[string]fixedHandler{
		"INIT": cmdInit,
		"ZERO": cmdZero,

		"EXAM": cmdExamine, "EX": cmdExamine, "DUMP": cmdExamine,
		"DEP": cmdDeposit, "D": cmdDeposit,

		"STEP": cmdStep, "ST": cmdStep, "S": cmdStep,

		"EXEC": cmdExecute, "GO": cmdExecute, "G": cmdExecute,
		"RUN": cmdRun, "R": cmdRun,

		"SAVE": cmdSave,
		"LOAD": cmdLoad,

		"TIME": cmdTime,
		"PRIN": cmdPrint, "ECHO": cmdPrint,
		"HELP": cmdHelp, "?": cmdHelp,
		"INCL": cmdInclude, "INC": cmdInclude, "@": cmdInclude,

		"SET": cmdSet,

		"ASM": cmdAssemble, "ASSE": cmdAssemble,
		"DISA": cmdDisassemble, "DIS": cmdDisassemble,
		"CALL": cmdCall,
		"BOOT": cmdNotImplemented("BOOT", "device/RTL support"),
		"ROM":  cmdNotImplemented("ROM", "device support"),
	}
}

func cmdNotImplemented(name, dependency string) fixedHandler {
	return func(d *Dispatcher, rest string) error {
		return fmt.Errorf("console: %s requires %s (not yet implemented, see docs/PHASE-08.md)", name, dependency)
	}
}

// cmdAssemble implements the batch "ASM <filename>" form (Console.Assemble,
// Phase 12) -- see that method's doc comment for why the bare, interactive
// "ASM" (no filename) REPL mode isn't implemented.
func cmdAssemble(d *Dispatcher, rest string) error {
	path := strings.Trim(strings.TrimSpace(rest), `"`)
	if path == "" {
		return fmt.Errorf("console: interactive ASM mode (no filename) is not implemented; use ASM <filename>")
	}
	entryAddr, hasEntry, err := d.Console.Assemble(path)
	if err != nil {
		return err
	}
	if hasEntry {
		// console.c's own post-command hook: a .END-named entry address
		// auto-invokes "CALL __ENTRY" (no arguments) once the file
		// finishes assembling.
		return d.Console.Call(entryAddr, false)
	}
	return nil
}

func cmdInit(d *Dispatcher, rest string) error {
	v, _, err := (&Evaluator{Symbols: d.Console.Symbols, Radix: d.Console.Radix}).Eval(strings.TrimSpace(rest))
	if err != nil {
		return fmt.Errorf("console: INIT requires a page count: %w", err)
	}
	return d.Console.Init(v * 512)
}

func cmdZero(d *Dispatcher, rest string) error { return d.Console.Zero() }

func cmdStep(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Step(nil)
	}
	v, _, err := d.Console.Evaluator().Eval(rest)
	if err != nil {
		return err
	}
	return d.Console.Step(&v)
}

func cmdExecute(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Execute(nil)
	}
	v, _, err := d.Console.Evaluator().Eval(rest)
	if err != nil {
		return err
	}
	return d.Console.Execute(&v)
}

// parseRunQualifier reads RUN's optional single leading qualifier, matching
// console_run's own read_verb-and-CHAR4-compare parsing: at most one of
// /NOINIT, /INIT, /BREAK|/DEBUG|/STEP, or /NOEXECUTE, matched by its first
// four characters (an unrecognized "/word" is left in place for the
// filename parse to reject, exactly as console_run puts back its saved
// pointer on a mismatch).
func parseRunQualifier(rest string) (RunOptions, string) {
	rest = strings.TrimLeft(rest, " \t")
	var opts RunOptions
	if !strings.HasPrefix(rest, "/") {
		return opts, rest
	}
	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}
	word := strings.ToUpper(rest[1:i])
	word4 := word
	if len(word4) > 4 {
		word4 = word4[:4]
	}
	tail := rest[i:]

	switch word4 {
	case "NOIN":
		opts.RunInits = false
		return opts, tail
	case "INIT":
		opts.RunInits = true
		return opts, tail
	case "BREA", "DEBU", "STEP":
		opts.Step = true
		return opts, tail
	case "NOEX":
		opts.NoExecute = true
		return opts, tail
	default:
		return opts, rest // unrecognized qualifier; leave it for the filename parse
	}
}

func cmdRun(d *Dispatcher, rest string) error {
	opts, rest := parseRunQualifier(rest)
	fn := strings.Trim(strings.TrimSpace(rest), `"`)
	if fn == "" {
		return fmt.Errorf("console: missing file name to run")
	}
	return d.Console.Run(fn, opts)
}

// parseCallQualifier reads CALL's optional leading /STEP|/BREAK|/DEBUG
// qualifier, matching console_call's own read_verb-and-CHAR4-compare check
// (see docs/PHASE-13.md's design notes on RUN's identical convention).
func parseCallQualifier(rest string) (step bool, tail string) {
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "/") {
		return false, rest
	}
	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}
	word := strings.ToUpper(rest[1:i])
	if len(word) > 4 {
		word = word[:4]
	}
	switch word {
	case "STEP", "BREA", "DEBU", "DBG":
		return true, rest[i:]
	default:
		return false, rest
	}
}

// cmdCall implements the CALL command (Console.Call, Phase 13's own
// primitive extended in Phase 12 with console_call's argument-list syntax):
// CALL [/STEP] <entry-expr>[(arg1[,arg2...])].
func cmdCall(d *Dispatcher, rest string) error {
	step, rest := parseCallQualifier(rest)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return fmt.Errorf("console: CALL requires an entry-point address or symbol")
	}

	ev := d.Console.Evaluator()
	addr, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}

	var args []uint32
	remainder = strings.TrimSpace(remainder)
	if strings.HasPrefix(remainder, "(") {
		remainder = remainder[1:]
		for {
			remainder = strings.TrimSpace(remainder)
			if strings.HasPrefix(remainder, ")") {
				remainder = remainder[1:]
				break
			}
			if remainder == "" {
				return fmt.Errorf("console: CALL: incomplete argument list")
			}
			if len(args) > 0 {
				if !strings.HasPrefix(remainder, ",") {
					return fmt.Errorf("console: CALL: expected ',' in argument list")
				}
				remainder = remainder[1:]
			}
			var v uint32
			v, remainder, err = ev.Eval(remainder)
			if err != nil {
				return err
			}
			args = append(args, v)
		}
	}

	return d.Console.Call(addr, step, args...)
}

func cmdTime(d *Dispatcher, rest string) error {
	return d.Console.Time(strings.TrimSpace(rest), d.Dispatch)
}

func cmdPrint(d *Dispatcher, rest string) error { return d.Console.Print(rest) }

func cmdHelp(d *Dispatcher, rest string) error {
	return d.Console.Help(d.Help, strings.Fields(rest))
}

func cmdInclude(d *Dispatcher, rest string) error {
	path := strings.Trim(strings.TrimSpace(rest), `"`)
	return d.Console.Include(path, d.Dispatch)
}

// parseExamSize reads an optional leading "/BYTE"/"/WORD"/"/LONGWORD"/
// "/ASCII"/"/PTE" format switch (unambiguous-prefix-matched, matching
// EXAMINE/DEPOSIT's shared size vocabulary — see exam.go), defaulting to
// SizeLongword.
func parseExamSize(rest string) (ExamSize, string) {
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "/") {
		return SizeLongword, rest
	}
	i := 1
	for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		i++
	}
	sw, tail := strings.ToUpper(rest[1:i]), rest[i:]

	switch {
	case sw == "B" || strings.HasPrefix("BYTE", sw):
		return SizeByte, tail
	case sw == "W" || strings.HasPrefix("WORD", sw):
		return SizeWord, tail
	case sw == "L" || strings.HasPrefix("LONGWORD", sw):
		return SizeLongword, tail
	case sw == "A" || strings.HasPrefix("ASCII", sw):
		return SizeASCII, tail
	case sw == "PTE":
		return SizePTE, tail
	default:
		return SizeLongword, rest // not a recognized size switch; leave it for the caller
	}
}

func cmdExamine(d *Dispatcher, rest string) error {
	sz, rest := parseExamSize(rest)
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return d.Console.Examine("", d.Console.DepositAddr, 1, sz)
	}
	if _, ok := registerNames[strings.ToUpper(rest)]; ok {
		return d.Console.Examine(rest, 0, 1, sz)
	}

	ev := d.Console.Evaluator()
	addr, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}
	count := uint32(1)
	if remainder = strings.TrimSpace(remainder); remainder != "" {
		end, _, err := ev.Eval(remainder)
		if err != nil {
			return err
		}
		if end < addr {
			return fmt.Errorf("console: end address before start address")
		}
		count = (end-addr)/sizeBytes(sz) + 1
	}
	return d.Console.Examine("", addr, count, sz)
}

func cmdDeposit(d *Dispatcher, rest string) error {
	sz, rest := parseExamSize(rest)
	rest = strings.TrimSpace(rest)

	var targetStr, valueStr string
	if eq := strings.IndexByte(rest, '='); eq >= 0 {
		targetStr, valueStr = strings.TrimSpace(rest[:eq]), strings.TrimSpace(rest[eq+1:])
	} else if fields := strings.Fields(rest); len(fields) >= 2 {
		targetStr, valueStr = fields[0], fields[1]
	} else {
		return fmt.Errorf("console: DEPOSIT requires an address/register and a value")
	}

	ev := d.Console.Evaluator()
	val, _, err := ev.Eval(valueStr)
	if err != nil {
		return err
	}
	if _, ok := registerNames[strings.ToUpper(targetStr)]; ok {
		return d.Console.Deposit(targetStr, 0, sz, val)
	}
	addr, _, err := ev.Eval(targetStr)
	if err != nil {
		return err
	}
	return d.Console.Deposit("", addr, sz, val)
}

// cmdDisassemble implements DISASSEMBLE/DISA: an optional [start[ end]]
// address range (each an expression, matching EXAMINE's own convention),
// defaulting start to the current deposit address and end to start (a
// single instruction) — matching console_disasm.c's own argument parsing.
func cmdDisassemble(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return d.Console.Disassemble(d.Console.DepositAddr, d.Console.DepositAddr)
	}

	ev := d.Console.Evaluator()
	start, remainder, err := ev.Eval(rest)
	if err != nil {
		return err
	}
	end := start
	if remainder = strings.TrimSpace(remainder); remainder != "" {
		end, _, err = ev.Eval(remainder)
		if err != nil {
			return err
		}
	}
	return d.Console.Disassemble(start, end)
}

// cmdSet implements SET's own small syntax: SET RADIX n, SET BREAKPOINT
// addr, or the general SET <name>=<value> form (see set.go).
func cmdSet(d *Dispatcher, rest string) error {
	rest = strings.TrimSpace(rest)
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return fmt.Errorf("console: SET requires an argument")
	}

	switch strings.ToUpper(fields[0]) {
	case "RADIX":
		if len(fields) < 2 {
			return fmt.Errorf("console: SET RADIX requires a value")
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			return fmt.Errorf("console: invalid radix %q", fields[1])
		}
		return d.Console.SetRadix(n)

	case "BREAKPOINT", "BREAK":
		if len(fields) < 2 {
			return fmt.Errorf("console: SET BREAKPOINT requires an address")
		}
		addr, _, err := d.Console.Evaluator().Eval(fields[1])
		if err != nil {
			return err
		}
		d.Console.AddBreakpoint(addr)
		return nil
	}

	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		return fmt.Errorf("console: unrecognized SET syntax %q", rest)
	}
	name := strings.TrimSpace(rest[:eq])
	val, _, err := d.Console.Evaluator().Eval(strings.TrimSpace(rest[eq+1:]))
	if err != nil {
		return err
	}
	return d.Console.SetSymbol(name, val)
}

// cmdSave/cmdLoad implement SAVE/LOAD's "/ROM <file>" and "/NVRAM <file>"
// forms (rom.go); the plain (no qualifier) SAVE/LOAD .VAX-file form isn't
// implemented — see rom.go's doc comment.
func cmdSave(d *Dispatcher, rest string) error {
	kind, file, err := parseRomOrNvramArg(rest)
	if err != nil {
		return err
	}
	if kind == "ROM" {
		return d.Console.SaveROM(file)
	}
	return d.Console.SaveNVRAM(file)
}

func cmdLoad(d *Dispatcher, rest string) error {
	kind, file, err := parseRomOrNvramArg(rest)
	if err != nil {
		return err
	}
	if kind == "ROM" {
		return d.Console.LoadROM(file)
	}
	return d.Console.LoadNVRAM(file)
}

func parseRomOrNvramArg(rest string) (kind, file string, err error) {
	rest = strings.TrimSpace(rest)
	switch {
	case strings.HasPrefix(strings.ToUpper(rest), "/ROM"):
		kind, rest = "ROM", rest[4:]
	case strings.HasPrefix(strings.ToUpper(rest), "/NVRAM"):
		kind, rest = "NVRAM", rest[6:]
	default:
		return "", "", fmt.Errorf("console: SAVE/LOAD requires /ROM or /NVRAM (the plain .VAX form isn't implemented — see rom.go)")
	}
	file = strings.Trim(strings.TrimSpace(rest), `"`)
	if file == "" {
		return "", "", fmt.Errorf("console: %s requires a file name", kind)
	}
	return kind, file, nil
}

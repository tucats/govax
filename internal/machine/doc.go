// Package machine is the emulated VAX as the console's operator sees it
// (docs/PHASE-52 - console package split.md): the CPU, its memory and
// engine (internal/cpu), and around them what govax's console keeps of
// the machine: INIT and ZERO, VMINIT's address space and its S0 layout,
// ROM and NVRAM, the shim stubs (corevms.ShimLink), the image loader and
// each process's image state, RUN and its IMAGE$INIT driver, the run
// loop and the instruction trace, the console's symbol table and
// expression evaluator, the ASM command's assembler session, EXAMINE and
// DEPOSIT's data sizes, disassembly, and the debug-symbol helpers SHOW
// CALLS, STEP, and the debugger use. export.go is the surface the
// debugger (internal/debugger) reaches the machine through.
//
// A Machine is the engine's cpu.SystemServices: SYS$ services, shims,
// ASTs, and exceptions go to the running process (corevms.EngineHooks),
// and what belongs to the console that runs the machine (its output, its
// default directory, $STATUS, the terminal's input, XFC$CONSOLE_CMD,
// Ctrl/Y, the subprocess CLI) is reached through Hooks, which
// internal/console sets. NewMachine gives a machine hooks of its own, so
// it works, and is tested, without a console.
package machine

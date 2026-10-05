// Package dbgsym reads an image's debug symbol table (DST) into the
// symbol table a debugger works from: the image's modules, the routines,
// labels, and data each defines, its psects, and (Phase 41's subtask 6)
// the line-number table that maps listing lines to code addresses.
//
// A program linked with traceback or /DEBUG carries a DST: a stream of
// variable-length records, each a length byte, a type byte, and fields,
// written by the compiler or assembler into the object module and
// relocated by the linker into the image (docs/DEBUG-RECORDS.md). A
// traceback link holds only the scope records (module, routine, psect);
// a /DEBUG link adds labels, data symbols, constants, and the line and
// source correlation records, and two tables beside the DST: the debug
// module table, each module's psect ranges (dmt.go), and the global
// symbol table, the link's globals, which the debugger falls back on
// where no module names an address (gst.go).
//
// The VMS debugger names things by path: DBGDIS\START is routine START
// in module DBGDIS, DBGDIS\START\LOOP is a label inside that routine, and
// DBGDIS\COUNT is data in the module. Program.Lookup accepts those paths,
// and each Symbol's Scope is its path less its own name.
//
// What's read here, and how the debugger scopes it, comes from DIGITAL's
// documentation of the format (docs/DEBUG-RECORDS.md) checked against
// real images and the VMS 7.3 debugger's own output for them
// (testdata/dbg; docs/PHASE-41.md). It's the groundwork for a debugger:
// the console's disassembler is its first user.
package dbgsym

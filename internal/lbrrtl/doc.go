// Package lbrrtl is LBRSHR.EXE, VMS's librarian routines (docs/PHASE-90 -
// VMS help.md, Decision 8): the LBR$ routines a program calls to read a
// library of any type, its keys and its modules' records, and to show help
// text, and (subtask 11) to create and change one: LBR$PUT_RECORD,
// LBR$PUT_END, the key routines, LBR$DELETE_DATA, and LBR$SET_MODULE.
// They're written from the OpenVMS Utility Routines manual's chapter 12 and
// held to VMS 7.1's answers to testdata/help90's LBR90.MAR (round 1b, part
// E; internal/console's TestLBR90Oracle) and LBR90W.MAR (round 3;
// TestLBR90WOracle, which also holds the libraries made to VMS's, block
// for block).
//
// Like internal/mthrtl, the routines are Go functions behind stubs: each
// entry of LBRSHR's transfer vector (vmsdef's copy of its global symbol
// table) has an XFC$SHIM code of its own, codeBase plus its offset, and the
// entries this package has no routine for signal NOTIMPL with their names.
//
// A library a program opens is read whole through internal/lbr, found by
// RMS's rules (internal/rms). One opened to create or update is an
// lbr.Updater, which changes the library's blocks as VMS's LBRSHR does,
// written back by LBR$CLOSE: a new file for LBR$C_CREATE, the same file for
// LBR$C_UPDATE. A control index names Go state kept per image
// (corevms.Environment.ImageData), as LIB$'s zones do. LBR$GET_HELP and
// LBR$OUTPUT_HELP run on internal/help, the engine DCL's HELP uses, and call
// the program's own routines back (corevms.Environment.Callback).
package lbrrtl

// Package lbrrtl is LBRSHR.EXE, VMS's librarian routines (docs/PHASE-90 -
// VMS help.md, Decision 8): the LBR$ routines a program calls to read a
// library of any type, its keys and its modules' records, and to show help
// text. They're written from the OpenVMS Utility Routines manual's chapter
// 12 and held to VMS 7.1's answers to testdata/help90's LBR90.MAR (round
// 1b, part E; internal/console's TestLBR90Oracle).
//
// Like internal/mthrtl, the routines are Go functions behind stubs: each
// entry of LBRSHR's transfer vector (vmsdef's copy of its global symbol
// table) has an XFC$SHIM code of its own, codeBase plus its offset, and the
// entries this package has no routine for, the writing ones among them,
// signal NOTIMPL with their names.
//
// A library a program opens is read whole through internal/lbr, found by
// RMS's rules (internal/rms). A control index names Go state kept per image
// (corevms.Environment.ImageData), as LIB$'s zones do. LBR$GET_HELP and
// LBR$OUTPUT_HELP run on internal/help, the engine DCL's HELP uses, and call
// the program's own routines back (corevms.Environment.Callback).
package lbrrtl

// Package mthrtl is MTHRTL.EXE, VMS's mathematics run-time library, written
// in Go (docs/PHASE-77 - MTHRTL.md): the MTH$ routines (square root,
// logarithms, the trigonometric functions, ...) and the OTS$ powers and
// complex arithmetic VMS keeps with them. It serves any program, in MACRO,
// govax's FORTRAN, or an image VMS's compilers made, and isn't part of
// FTNRTL.
//
// # How a program reaches a routine
//
// A shareable image's routines are reached through its transfer vector, a
// table at the image's start with one entry per routine, so that a program
// linked against one version of the image still finds its routines in the
// next. A program's image names a routine by its entry's offset. MTHRTL.EXE
// has 402 entries of 8 bytes each, under 404 names (vmsdef.ImageSymbols,
// captured from VMS 7.1's image), and govax stands for the image: each
// entry has a stub in system space, whose XFC instruction runs a routine
// here, as LIBRTL's do (internal/librtl).
//
// An entry is reached in one of two ways, and a stub must match:
//
//   - By CALLS or CALLG: a procedure call, with an argument list and a call
//     frame. Its Go routine is a corevms.ShimFunc. The MTH$ routines take
//     their arguments by reference and the OTS$ routines by value.
//   - By JSB, with the arguments in registers and no call frame, which is
//     faster on a real VAX. Its Go routine is a corevms.JSBShimFunc. These
//     are the MTH$ names ending _Rn, n being the highest register the
//     routine may change. (OTS$POWHH_R3 and its kind are CALL entries,
//     named for returning a result in R0:R3.)
//
// Every entry has its routine but MTH$$REDUCE_, internal to VMS's library, whose
// stubs signal GOVAX-E-NOTIMPL with the routine's name. testdata/mth77 holds the
// rest to VMS 7.1's answers.
//
// # Files
//
// routines.go is the tables and the stubs' registration; float.go moves
// floating values between the VAX's registers and memory and vaxfloat's
// Values; condition.go signals the library's conditions. engine.go is the
// functions on arbitrary-precision numbers, written once for every format;
// function.go makes a table's routines from a function and a format; explog.go
// is the square root, exponential, and logarithm functions; trig.go, on
// enginetrig.go's part of the engine, is the trigonometric functions and their
// inverses, in radians and degrees; hyperbolic.go, on enginehyper.go's, is the
// hyperbolic functions; floating.go is the routines of the manual's Table 1-1
// that a floating instruction or two could do (absolute value, truncation,
// remainder, conversions, ...); integer.go the ones with word and longword
// arguments or results, the bit functions among them; extreme.go the maximum and
// minimum of any number of arguments; complex.go the functions of complex
// numbers; random.go the random number generator; and ots.go the OTS$ powers and complex multiplication and division,
// whose integer powers and complex arithmetic are vaxfloat's, shared with the
// FORTRAN compiler's constants.
package mthrtl

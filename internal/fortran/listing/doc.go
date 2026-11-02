// Package listing lays out the FORTRAN compiler's listing, the file /LIST writes
// (docs/PHASE-85 - FORTRAN compiler usability.md, Decision 11).
//
// A compiler's messages tell a programmer what's wrong; a listing shows it in
// place, on paper. Each page has a two-line heading. Then come the source's lines,
// each with its number, a line from an INCLUDE file with its include depth before
// that, and the messages about a line under it. After the whole source come the
// storage maps:
//
//	SQUARES                                  2-NOV-2026 14:05:11  govax FORTRAN V1.0-115        Page 1
//	                                         DUA0:[WORK]SQUARES.FOR;1
//
//	    1        PROGRAM SQUARES
//	    2        INTEGER I, N
//	    3        INCLUDE 'LIMITS.FOR/LIST'
//	  1 1        PARAMETER (MAX = 10)
//	    4        DO 20 I = 1, MAX
//	    5        GO TO 30
//	                   ^
//	%FORT-E-UNDEFLBL, statement label 30 is never defined
//	    6  20    CONTINUE
//	    7        END
//
//
//	Storage map for SQUARES, the main program
//	  I  INTEGER  $LOCAL+0  SQUARES.I
//	  N  INTEGER  $LOCAL+4  SQUARES.N
//
// With /CROSS_REFERENCE each unit's map is followed by where its names and labels
// are mentioned, a line number each, with D if the line defines the name and A if it
// assigns it, and a line of an INCLUDE file with its include depth before that:
//
//	Cross reference for SQUARES                 (D defined, A assigned, a blank is used)
//	  I     INTEGER   2D    4A    5
//	  N     INTEGER   2D    5A
//	  MAX   constant  1:1D  4
//	  20    label     4     6D
//
// The layout is govax's own, in MACRO's manner. VAX FORTRAN's is in a manual not
// to hand, and no program depends on a listing's columns.
//
// This package knows nothing of how those things were found: Make is given the
// source pass's lines, every pass's messages, and the code generator's maps and
// semantic analysis's cross references as text, and is the same whether or not the compilation succeeded. A compilation
// with errors has no maps, and a cross reference only as far as analysis reached, but its listing is the one most wanted.
package listing

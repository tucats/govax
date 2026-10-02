//go:build ignore

// gen writes LCTL.MAR, the Phase 29 probe's listing-control source
// (docs/PHASE-29.md, subtask 1). Run it from the repository root:
//
//	go run testdata/mar/list/gen.go
//
// LCTL.MAR lists the same body (macro definitions, macro calls, repeat
// blocks, and conditionals) once under each .SHOW and .NOSHOW option, so
// its listing shows what each option changes. VAX MACRO has no .INCLUDE,
// so the body is written out each time. Every option is set and then
// undone with its opposite: by the MACRO manual, each one keeps a count,
// so the pair leaves it at its default for the next section.
package main

import (
	"fmt"
	"os"
	"strings"
)

// section is one copy of the body: its subtitle, and the directives
// before and after it.
type section struct {
	subtitle, before, after string
}

var sections = []section{
	{"Defaults", "", ""},
	{".SHOW EXPANSIONS", ".SHOW\tEXPANSIONS", ".NOSHOW\tEXPANSIONS"},
	{".NOSHOW EXPANSIONS", ".NOSHOW\tEXPANSIONS", ".SHOW\tEXPANSIONS"},
	{".SHOW BINARY", ".SHOW\tBINARY", ".NOSHOW\tBINARY"},
	{".NOSHOW BINARY", ".NOSHOW\tBINARY", ".SHOW\tBINARY"},
	{".SHOW CALLS", ".SHOW\tCALLS", ".NOSHOW\tCALLS"},
	{".NOSHOW CALLS", ".NOSHOW\tCALLS", ".SHOW\tCALLS"},
	{".SHOW DEFINITIONS", ".SHOW\tDEFINITIONS", ".NOSHOW\tDEFINITIONS"},
	{".NOSHOW DEFINITIONS", ".NOSHOW\tDEFINITIONS", ".SHOW\tDEFINITIONS"},
	{".SHOW CONDITIONALS", ".SHOW\tCONDITIONALS", ".NOSHOW\tCONDITIONALS"},
	{".NOSHOW CONDITIONALS", ".NOSHOW\tCONDITIONALS", ".SHOW\tCONDITIONALS"},
	{".SHOW ME,MEB,MC,MD,CND (the abbreviations)", ".SHOW\tME, MEB, MC, MD, CND", ".NOSHOW\tME, MEB, MC, MD, CND"},
	{".SHOW ME and BINARY together", ".SHOW\tEXPANSIONS, BINARY", ".NOSHOW\tEXPANSIONS, BINARY"},
	{".LIST ME (.LIST with an argument)", ".LIST\tME", ".NLIST\tME"},
	{".NLIST MEB (.NLIST with an argument)", ".NLIST\tMEB", ".LIST\tMEB"},
	{"Defaults again", "", ""},
}

// body is what each section lists. It defines no labels, so it can be
// written out more than once; its macros are redefined each time.
const body = `	.MACRO	PAIR	A, B
	.BYTE	A
	.BYTE	B
	.ENDM	PAIR

	.MACRO	OUTER	X
	.WORD	X
	PAIR	X, X+1
	.ENDM	OUTER

	.MACRO	CHOOSE	N
	.IF	EQ N
	.BYTE	0
	.IFF
	.BYTE	1
	.ENDC
	.ENDM	CHOOSE

	.MACRO	NOBYTES	SYM
SYM = 1
	.ENDM	NOBYTES

	PAIR	1, 2			; a call
	OUTER	3			; a call with a nested call
	CHOOSE	0			; conditionals inside a macro
	CHOOSE	1
	NOBYTES	...X			; a macro that stores nothing

	.REPEAT	2			; a repeat block
	.WORD	^X1234
	.ENDR

	.IRP	V, <5, 6, 7>
	.BYTE	V
	.ENDR

	.IF	DF LCTL_D1		; a true conditional
	.BYTE	8
	.ENDC

	.IF	NDF LCTL_D1		; a false one
	.BYTE	9
	.ENDC

	.IIF	DF LCTL_D1, .BYTE 10	; immediate conditionals, true and false
	.IIF	NDF LCTL_D1, .BYTE 11

	.IF	EQ 0			; subconditionals
	.BYTE	12
	.IFF
	.BYTE	13
	.IFT
	.BYTE	14
	.IFTF
	.BYTE	15
	.ENDC
`

// head starts the source: a title longer than the heading's 40 columns.
const head = `	.TITLE	LCTL	Phase 29 probe: listing controls, each .SHOW option, .NLIST levels, and pages
	.IDENT	/LCTL-1.0/

; LCTL.MAR - written by testdata/mar/list/gen.go; don't edit it by hand.
; The same body is listed under each .SHOW and .NOSHOW option, each
; section with its own .SBTTL, so the listing's table of contents (if
; MACRO makes one) names every section.

	.PSECT	DATA, WRT, NOEXE, LONG
LCTL_D1:
`

// tail lists the other controls: .NLIST and .LIST levels, counted .SHOW
// options, .PAGE, and lines longer than the listing.
const tail = `	.SBTTL	.NLIST and .LIST levels
	.BYTE	20			; listed
	.NLIST
	.BYTE	21			; level -1: not listed
	.NLIST
	.BYTE	22			; level -2
	.LIST
	.BYTE	23			; level -1: still not listed
	.LIST
	.BYTE	24			; level 0: listed again
	.LIST
	.BYTE	25			; level 1: listed
	.NLIST
	.BYTE	26			; level 0

	.SBTTL	Counted options
	.SHOW	EXPANSIONS
	.SHOW	EXPANSIONS
	.NOSHOW	EXPANSIONS
	PAIR	27, 28			; shown if the option counts
	.NOSHOW	EXPANSIONS
	PAIR	29, 30			; back to the default

	.SBTTL	A subtitle longer than the forty columns of the heading's own subtitle field, to see where it is cut
	.BYTE	31

	.SBTTL	.PAGE
	.BYTE	32			; before the first .PAGE
	.PAGE
	.BYTE	33			; the first line of a new page
	.PAGE
	.PAGE
	.BYTE	34			; after two .PAGEs in a row

	.SBTTL	Long lines
	.BYTE	35			; a comment long enough to run well past the 132 columns of a listing line, to see whether the listing cuts it, wraps it, or keeps it
	.ASCII	/This string, with its statement, runs past the end of the listing line: does MACRO list all of its bytes?/

	.PSECT	CODE, NOWRT, EXE, LONG
	.ENTRY	LCTL, ^M<>
	MOVL	#1, R0
	RET

	.END	LCTL
`

func main() {
	var b strings.Builder

	b.WriteString(head)

	for _, s := range sections {
		fmt.Fprintf(&b, "\n\t.SBTTL\t%s\n\n", s.subtitle)

		if s.before != "" {
			fmt.Fprintf(&b, "\t%s\n\n", s.before)
		}

		b.WriteString(body)

		if s.after != "" {
			fmt.Fprintf(&b, "\n\t%s\n", s.after)
		}
	}

	b.WriteString("\n" + tail)

	if err := os.WriteFile("testdata/mar/list/lctl.mar", []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

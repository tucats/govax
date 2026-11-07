//go:build ignore

// gen.go writes round 1 of the Phase 90 probe (docs/PHASE-90 - VMS
// help.md, subtask 2): VMS's LIBRARY/HELP, HELP, the debugger's HELP, and
// the LBR$ routines, run on help modules written for the probe. Run this
// from the repository root:
//
//	go run testdata/help90/gen.go
//
// It writes the modules (*.hlp, our own text; t1.txt, t2.txt, and
// h90m.mar for a text and a macro library), help90.com (parts A to E),
// help90d.com (part D, HELP prompting in a procedure), help90v.com (parts F
// and G, whose output may hold VMS's own help text and so stays local),
// the debugger's command files, dbgt.mar, lbr90.mar, exchange.cmd, and
// copyout.cmd.
//
// Every topic's text names the topic, so a log shows which text HELP chose,
// and none of it is VMS's.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/help90"

func main() {
	for name, text := range modules() {
		write(name, text)
	}

	write("help90.com", help90())
	write("help90d.com", help90d())
	write("help90v.com", help90v())
	write("help90f.com", help90f())
	write("help90d.dbg", debuggerCommands("DBGH"))
	write("help90v.dbg", debuggerCommands("DBGV"))
	write("dbgt.mar", dbgt)
	write("lbr90.mar", lbr90())
	write("exchange.cmd", exchange())
	write("copyout.cmd", copyout())
}

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

// files is every file the VAX needs, in the order exchange.cmd copies them.
var files = []string{
	"h90a1.hlp", "h90a2.hlp", "alpha2.hlp", "h90b.hlp", "h90c.hlp",
	"h90g.hlp", "h90s.hlp", "h90d.hlp", "badtext.hlp", "skip.hlp",
	"noblank.hlp", "toolong.hlp", "lower.hlp", "digits.hlp", "zero.hlp", "t1.txt", "t2.txt",
	"h90m.mar", "resp.txt", "help90.com", "help90d.com", "help90v.com",
	"help90f.com", "help90d.dbg", "help90v.dbg", "dbgt.mar", "lbr90.mar",
}

// ---------------------------------------------------------------------
// The modules.

func modules() map[string]string {
	m := map[string]string{
		"h90a1.hlp":   h90a1,
		"h90a2.hlp":   h90a2(),
		"alpha2.hlp":  alpha2,
		"h90b.hlp":    h90b,
		"h90c.hlp":    "1 CONLY\n   Text of CONLY, in library H90C.\n2 Sub\n   Text of CONLY Sub.\n",
		"h90g.hlp":    "1 GONLY\n   Text of GONLY, in library H90G (the group table's).\n",
		"h90s.hlp":    "1 SONLY\n   Text of SONLY, in library H90S (the system table's).\n",
		"h90d.hlp":    h90d,
		"badtext.hlp": "Text before any key line.\n1 BADTEXT\n   Text of BADTEXT.\n",
		"skip.hlp":    "1 SKIP\n   Text of SKIP.\n3 THREE\n   Text of SKIP THREE, a level 3 key after level 1.\n2 TWO\n   Text of SKIP TWO.\n",
		"noblank.hlp": "1 NOBLANK\n   Text of NOBLANK.\n2Nospace\n   Text after a key line with no blank.\n2\n   Text after a bare level digit.\n2\tTabbed\n   Text after a key line with a tab.\n2 Last\n   Text of NOBLANK Last.\n",
		"toolong.hlp": "1 A_LEVEL_ONE_KEY_OF_FORTY_CHARACTERS_XYZ\n   Text of the long key.\n",
		"digits.hlp":  "1 DIGITS\n   Text of DIGITS.\n10 apples is a line that starts with two digits.\n",
		"zero.hlp":    "1 ZERO\n   Text of ZERO.\n0 zero is a line that starts with a zero.\n",
		"lower.hlp":   "1 lower\n   Text of lower, a level 1 key in lowercase.\n2 sub\n   Text of lower sub.\n",
		"t1.txt":      "First line of text module T1.\n   Second line, indented.\n\nFourth line, after an empty one.\n",
		"t2.txt":      "Only line of text module T2.\n",
		"h90m.mar":    "\t.MACRO\tPROBE1\tA\n\t.LONG\tA\n\t.ENDM\tPROBE1\n\t.MACRO\tPROBE2\n\t.BYTE\t2\n\t.ENDM\tPROBE2\n",
		"resp.txt":    "Parameters\n\n?\n",
	}

	return m
}

const h90a1 = `1 ALPHA
   Text of ALPHA, level 1. ALPHA is the first of the probe's
   topics.

   Format

     ALPHA filespec [count]


2 Parameters


filespec

   Text of the parameter filespec.

count

   Text of the parameter count.


2 Qualifiers


/BRIEF

      /BRIEF

   Text of the qualifier /BRIEF.

/COUNT

      /COUNT=n

   Text of the qualifier /COUNT.

/NOCONFIRM

   Text of the qualifier /NOCONFIRM.


2 /DIRECT
   Text of ALPHA /DIRECT, a qualifier with a key line of its own.
3 Example
   Text of ALPHA /DIRECT Example.
2 Description
   Text of ALPHA Description.
/NOTQUAL
   The line above starts with a slash, outside Qualifiers.
3 Details
   Text of ALPHA Description Details.
2 Examples
   1.  $ ALPHA X.DAT

       Text of ALPHA's first example.
1 ALPHABET
   Text of ALPHABET, level 1.
2 Letters
   Text of ALPHABET Letters.
1 ALPS
   Text of ALPS, level 1, with no subtopics.
1 HELP
   Text of the probe library's own HELP topic.
2 Parameters
   Text of HELP Parameters, in the probe library.
2 /QUALIFIER
   Text of HELP /QUALIFIER, in the probe library.
`

// h90a2 is the modules that test matching, levels, layout, and text.
func h90a2() string {
	var b strings.Builder

	// DEEP: a chain to level 9, with a sibling at each level.
	b.WriteString("1 DEEP\n   Text of DEEP, level 1.\n")
	for lv := 2; lv <= 9; lv++ {
		path := "DEEP"
		for k := 2; k <= lv; k++ {
			path += fmt.Sprintf(" L%d", k)
		}
		fmt.Fprintf(&b, "%d L%d\n   Text of %s, level %d.\n", lv, lv, path, lv)
		fmt.Fprintf(&b, "%d S%d\n   Text of S%d, L%d's sibling at level %d.\n", lv, lv, lv, lv, lv)
	}

	// WIDE: subtopics of every length from 1 to 32, for the columns.
	b.WriteString("1 WIDE\n   Text of WIDE: its subtopics' keys are 1 to 32 characters long.\n")
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 20, 24, 28, 32} {
		key := string(rune('A'+n%26)) + strings.Repeat("x", n-1)
		fmt.Fprintf(&b, "2 %s\n   Text of WIDE %s, a key of %d characters.\n", key, key, n)
	}

	// MANY: forty short keys.
	b.WriteString("1 MANY\n   Text of MANY: forty subtopics of three characters.\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "2 M%02d\n   Text of MANY M%02d.\n", i, i)
	}

	// ORDER: keys out of order, in both cases, to see whether HELP sorts.
	b.WriteString("1 ORDER\n   Text of ORDER: its keys are out of order.\n")
	for _, k := range []string{"Zebra", "apple", "Mango", "banana", "_under", "9nine", "Apple2", "zulu"} {
		fmt.Fprintf(&b, "2 %s\n   Text of ORDER %s.\n", k, k)
	}

	// ODD: keys with punctuation and blanks.
	b.WriteString("1 ODD\n   Text of ODD: keys with punctuation.\n")
	for _, k := range []string{"@_(Paren_Name)", "Key,_With_Comma", ":=", "$DOLLAR", "Two Words", "Slash/Inside", "dot.name", "Plus+"} {
		fmt.Fprintf(&b, "2 %s\n   Text of ODD %s.\n", k, k)
	}

	b.WriteString(`1 SAME
   Text of SAME: the same key under two parents, and twice in one.
2 P1
   Text of SAME P1.
3 Child
   Text of SAME P1 Child.
2 P2
   Text of SAME P2.
3 Child
   Text of SAME P2 Child.
2 Twice
   Text of the first SAME Twice.
2 Twice
   Text of the second SAME Twice.
2 Example
   Text of SAME Example.
2 Examples
   Text of SAME Examples.
1 TEXTY

   Text of TEXTY follows an empty line.
No indent on this line.
` + "\tA tab begins this line.\n" + `   ` + strings.Repeat("Long line of TEXTY. ", 5) + `end.

   The line above holds one blank.
   Quotes " and ' and an exclamation mark ! in the text.


1 NOSUB
   Text of NOSUB, a topic with no subtopics.
1 EMPTY
2 Child
2 Other
   Text of EMPTY Other; EMPTY and EMPTY Child have no text.
1 MixedCase
   Text of MixedCase, a level 1 key in mixed case.
2 lowerSub
   Text of MixedCase lowerSub.
1 BETA_GAMMA
   Text of BETA_GAMMA.
1 QUALS
   Text of QUALS.
2 Command_Qualifiers
/CQ1
   Text of QUALS Command_Qualifiers /CQ1.
/CQ2
   Text of QUALS Command_Qualifiers /CQ2.
2 Qual
/QQ1
   Text of QUALS Qual /QQ1, under a topic not named Qualifiers.
2 Qualifiers
   Text of QUALS Qualifiers, before its first slash line.
 /INDENTED is a line with a slash after a blank.
/[NO]FLAG
   Text of QUALS /[NO]FLAG.
/LAST
   Text of QUALS /LAST.
3 /SUB
   Text of QUALS /SUB, a level 3 key after the slash lines.
4 Deeper
   Text of QUALS /SUB Deeper.
`)

	return b.String()
}

const alpha2 = `1 ALPHA
   Text of the replacement ALPHA, from ALPHA2.HLP.
2 Parameters
   Text of the replacement ALPHA Parameters.
`

const h90b = `1 BONLY
   Text of BONLY, in library H90B.
2 Sub
   Text of BONLY Sub.
1 ALPHA
   Text of library H90B's ALPHA.
`

const h90d = `1 DEBUG
   Text of the probe's DEBUG module, level 1.
2 Probe_Concept
   Text of Probe_Concept (probe).
2 EXAMINE
   Text of EXAMINE (probe).
3 Parameters
   Text of EXAMINE Parameters (probe).
3 Qualifiers
/ASCII
   Text of EXAMINE /ASCII (probe).
/HEX
   Text of EXAMINE /HEX (probe).
3 Examples
   Text of EXAMINE Examples (probe).
2 EXIT
   Text of EXIT (probe).
2 SET
   Text of SET (probe).
3 BREAK
   Text of SET BREAK (probe).
4 Qualifiers
/AFTER
   Text of SET BREAK /AFTER (probe).
3 MODE
   Text of SET MODE (probe).
1 EXTRA
   Text of EXTRA, a second module in the probe's debugger library.
`

// ---------------------------------------------------------------------
// The procedures.

// requests is part B: HELP's matching and layout on library H90A.
var requests = []string{
	"", "HELP", "HELP Parameters", "HELP /QUALIFIER",
	"ALPHA", "alpha", "ALPHAB", "ALP", "AL", "A", "ALPS",
	"ALPHA Parameters", "ALPHA PAR", "ALPHA P", "ALPHA Parameters filespec",
	"ALPHA filespec", "ALPHA Parameters *",
	"ALPHA Qualifiers", "ALPHA Q", "ALPHA Qualifiers /BRIEF", "ALPHA Qualifiers BRIEF",
	"ALPHA /BRIEF", "ALPHA/BRIEF", "ALPHA /BR", "ALPHA /B", "ALPHA BRIEF",
	"ALPHA /COUNT", "ALPHA /CONFIRM", "ALPHA /NOCONFIRM", "ALPHA /NOSUCH",
	"ALPHA /DIRECT", "ALPHA /DIRECT Example", "ALPHA /DI", "ALPHA /", "ALPHA /*",
	"ALPHA Description", "ALPHA Description /NOTQUAL", "ALPHA /NOTQUAL",
	"ALPHA Description Details", "ALPHA Examples", "ALPHA E", "ALPHA Ex",
	"ALPHA *", "ALPHA * *", "ALPHA...", "ALPHA ...", "ALPHA Description...",
	"ALPHA,Parameters", `"ALPHA Parameters"`, "ALPHA Qualifiers/BRIEF",
	"*", "A*", "AL%HA", "%%%%", "*A", "ALPHA Q*", "ALPHA *S", "ALPHA %escription",
	"NOSUCH", "NOSUCH SUB", "ALPHA NOSUCH", "ALPHA Parameters NOSUCH",
	"DEEP", "DEEP L2", "DEEP L2 L3 L4 L5 L6 L7 L8 L9",
	"DEEP L2 L3 L4 L5 L6 L7 L8 L9 L10", "DEEP S2", "DEEP L2 S3",
	"DEEP ...", "DEEP * *", "DEEP L2...",
	"WIDE", "MANY", "ORDER", "ORDER *",
	"ODD", "ODD @", "ODD @_(Paren_Name)", "ODD (", "ODD Key", "ODD Key,_With_Comma",
	"ODD :=", "ODD $DOLLAR", "ODD Two", "ODD Two Words", "ODD Two_Words",
	"ODD Slash", "ODD Slash/Inside", "ODD dot", "ODD Plus+",
	"SAME", "SAME P1 Child", "SAME * Child", "SAME Twice", "SAME Example",
	"SAME Examples", "SAME Ex",
	"TEXTY", "NOSUB", "NOSUB ANY", "EMPTY", "EMPTY Child",
	"MixedCase", "MIXEDCASE", "MIXEDCASE lowersub",
	"BETA_GAMMA", "BETA", "BETA GAMMA", "BETA-GAMMA", "BETA_",
	"QUALS", "QUALS Command_Qualifiers", "QUALS /CQ1", "QUALS Qual",
	"QUALS Qual /QQ1", "QUALS /QQ1", "QUALS Qualifiers /INDENTED",
	"QUALS /INDENTED", "QUALS /FLAG", "QUALS /NOFLAG", "QUALS /[NO]FLAG",
	"QUALS /LAST", "QUALS /SUB", "QUALS /SUB Deeper", "QUALS Qualifiers /SUB",
}

func help90() string {
	var b strings.Builder

	b.WriteString(`$ ! HELP90.COM - round 1 (since round 1b) of the Phase 90 probe:
$ ! LIBRARY/HELP, HELP, and the LBR$ routines, on the probe's own help
$ ! modules.
$ ! Written by testdata/help90/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @HELP90/OUTPUT=HELP90.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ SHOW LOGICAL HLP$LIBRARY*
$ SHOW LOGICAL DBG$HELP
$ !
$ ! Part A: LIBRARY/HELP.
$ !
$ ! A library is an object library unless /HELP, /TEXT, or /MACRO says
$ ! otherwise (round 1), so every command here says which.
$ LIBRARY/CREATE/HELP H90A H90A1,H90A2
$ LIBRARY/LIST/FULL/HELP H90A
$ LIBRARY/LIST/HELP H90A
$ LIBRARY/LIST/FULL H90A.HLB
$ LIBRARY/CREATE/HELP H90B H90B
$ LIBRARY/CREATE/HELP H90C H90C
$ LIBRARY/CREATE/HELP H90G H90G
$ LIBRARY/CREATE/HELP H90S H90S
$ LIBRARY/CREATE/HELP H90D H90D
$ LIBRARY/LIST/FULL/HELP H90D
$ ! Data reduction is a command of its own (round 1's MAXPARM).
$ LIBRARY/CREATE/HELP H90R H90A1,H90A2
$ LIBRARY/DATA=REDUCE/HELP H90R
$ LIBRARY/LIST/FULL/HELP H90R
$ LIBRARY/CREATE=(KEYSIZE:80)/HELP H90L H90A1
$ LIBRARY/LIST/FULL/HELP H90L
$ LIBRARY/CREATE/HELP H90E H90A1
$ LIBRARY/INSERT/HELP H90E H90A1
$ LIBRARY/REPLACE/HELP H90E ALPHA2
$ LIBRARY/REPLACE/HELP H90E H90C
$ LIBRARY/DELETE=ALPS/HELP H90E
$ LIBRARY/DELETE=NOSUCH/HELP H90E
$ LIBRARY/LIST/FULL/HELP H90E
$ LIBRARY/EXTRACT=ALPHA/OUTPUT=ALPHAX.HLP/HELP H90E
$ TYPE ALPHAX.HLP
$ LIBRARY/EXTRACT=(ALPHA,ALPHABET)/OUTPUT=ALPHAY.HLP/HELP H90A
$ TYPE ALPHAY.HLP
$ LIBRARY/EXTRACT=*/OUTPUT=ALLX.HLP/HELP H90A
$ TYPE ALLX.HLP
$ LIBRARY/EXTRACT=alpha/OUTPUT=ALPHAZ.HLP/HELP H90A
$ TYPE ALPHAZ.HLP
$ LIBRARY/EXTRACT=MIXEDCASE/OUTPUT=MIXEDX.HLP/HELP H90A
$ TYPE MIXEDX.HLP
$ LIBRARY/EXTRACT="MixedCase"/OUTPUT=MIXEDY.HLP/HELP H90A
$ TYPE MIXEDY.HLP
$ LIBRARY/EXTRACT=TEXTY/OUTPUT=SYS$OUTPUT/HELP H90A
$ ! The malformed modules.
$ LIBRARY/CREATE/HELP H90X1 BADTEXT
$ LIBRARY/LIST/FULL/HELP H90X1
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X1
$ LIBRARY/CREATE/HELP H90X2 SKIP
$ LIBRARY/LIST/FULL/HELP H90X2
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X2
$ LIBRARY/CREATE/HELP H90X3 NOBLANK
$ LIBRARY/LIST/FULL/HELP H90X3
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X3
$ LIBRARY/CREATE/HELP H90X4 TOOLONG
$ LIBRARY/LIST/FULL/HELP H90X4
$ LIBRARY/CREATE=(KEYSIZE:40)/HELP H90K TOOLONG
$ LIBRARY/LIST/FULL/HELP H90K
$ LIBRARY/CREATE/HELP H90X5 LOWER
$ LIBRARY/LIST/FULL/HELP H90X5
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X5
$ LIBRARY/CREATE/HELP H90X6 DIGITS
$ LIBRARY/LIST/FULL/HELP H90X6
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X6
$ LIBRARY/CREATE/HELP H90X7 ZERO
$ LIBRARY/LIST/FULL/HELP H90X7
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90X7
$ ! Several bad modules in one command: does LIBRARY go on?
$ LIBRARY/CREATE/HELP H90X8 H90C,SKIP,H90G
$ LIBRARY/LIST/FULL/HELP H90X8
$ LIBRARY/CREATE H90Q.HLB H90C.HLP
$ LIBRARY/LIST/FULL H90Q.HLB
$ LIBRARY/CREATE/TEXT H90T T1,T2
$ LIBRARY/LIST/FULL/TEXT H90T
$ LIBRARY/CREATE/MACRO H90M H90M
$ LIBRARY/LIST/FULL/MACRO H90M
$ DIRECTORY/SIZE=ALL H90*.*LB
$ !
$ ! Part B: HELP's matching and layout, on H90A, without prompting.
$ !
$ H := HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A
`)

	for _, r := range requests {
		fmt.Fprintf(&b, "$ H %s\n", r)
	}

	b.WriteString(`$ H/NOINSTRUCTIONS
$ H/INSTRUCTIONS
$ H/NOINSTRUCTIONS HELP
$ ! A library with no HELP module, and the reduced and wide-key copies.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90B
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90B HELP
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90R ALPHA...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90L ALPHA /BRIEF
$ ! The malformed modules, as HELP reads them.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X1 BADTEXT...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X2 SKIP...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X2 SKIP
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X3 NOBLANK...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X3 NOBLANK
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90X5 LOWER...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90K A_LEVEL
$ ! The library's name and type.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=[]H90A ALPS
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A.HLB ALPS
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90T ALPS
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:NOSUCH ALPS
$ ! Part B2: output.
$ H/OUTPUT=H90O.LIS ALPHA
$ TYPE H90O.LIS
$ DIRECTORY/FULL H90O.LIS
$ H/OUTPUT ALPS
$ DIRECTORY HELP.LIS
$ TYPE HELP.LIS
$ H/OUTPUT=[] NOSUB
$ TYPE HELP.LIS
$ H/NOOUTPUT ALPHA
$ H/OUTPUT=H90N.LIS NOSUCH
$ TYPE H90N.LIS
$ !
$ ! Part C: the user libraries.
$ !
$ HU := HELP/NOPROMPT/LIBRARY=H90DIR:H90A
$ DEFINE HLP$LIBRARY 'L'H90B
$ DEFINE HLP$LIBRARY_1 'L'H90C
$ HU NOSUB
$ HU/NOLIBLIST NOSUB
$ HU BONLY
$ HU BONLY Sub
$ HU CONLY
$ HU ALPHA
$ HU NOSUCH
$ HU @H90B ALPHA
$ HU @H90B
$ HU @H90 ALPHA
$ HU @'L'H90C CONLY
$ HU @H90DIR:H90C CONLY
$ HU @NOSUCHLIB ALPHA
$ HU/USERLIBRARY=NONE BONLY
$ HU/NOUSERLIBRARY BONLY
$ HU/USERLIBRARY=PROCESS BONLY
$ HU/USERLIBRARY=SYSTEM BONLY
$ HU/LIBLIST
$ ! The group and system tables, only if this VAX has no HLP$LIBRARY there.
$ G = F$TRNLNM("HLP$LIBRARY","LNM$GROUP")
$ S = F$TRNLNM("HLP$LIBRARY","LNM$SYSTEM")
$ IF G .NES. "" .OR. S .NES. "" THEN GOTO NOTABLES
$ DEFINE/GROUP HLP$LIBRARY 'L'H90G
$ DEFINE/SYSTEM HLP$LIBRARY 'L'H90S
$ HU GONLY
$ HU SONLY
$ HU/LIBLIST
$ HU/USERLIBRARY=(PROCESS,GROUP) SONLY
$ HU/USERLIBRARY=SYSTEM GONLY
$ HU/USERLIBRARY=GROUP GONLY
$ DEASSIGN/GROUP HLP$LIBRARY
$ DEASSIGN/SYSTEM HLP$LIBRARY
$ NOTABLES:
$ ! A gap in the numbers, then a library that isn't there.
$ DEASSIGN HLP$LIBRARY_1
$ DEFINE HLP$LIBRARY_2 'L'H90C
$ HU CONLY
$ HU/LIBLIST
$ DEASSIGN HLP$LIBRARY_2
$ DEFINE HLP$LIBRARY_1 'L'NOSUCH
$ HU BONLY
$ HU/LIBLIST
$ DEFINE HLP$LIBRARY_1 'L'H90C
$ ! /NOLIBRARY with user libraries: which is the root?
$ HELP/NOPROMPT/NOLIBRARY BONLY
$ HELP/NOPROMPT/NOLIBRARY CONLY
$ HELP/NOPROMPT/NOLIBRARY ALPHA
$ !
$ ! Part E: the LBR$ routines (LBR90.MAR), with the user libraries still
$ ! defined for OUTPUT_HELP's flags.
$ !
$ MACRO LBR90
$ LINK LBR90
$ RUN LBR90
$ SHOW SYMBOL $STATUS
$ DEASSIGN HLP$LIBRARY
$ DEASSIGN HLP$LIBRARY_1
$ !
$ ! Part D: prompting in a procedure.
$ !
$ @HELP90D
$ !
$ ! The libraries, for comparing with govax's.
$ !
$ BACKUP H90*.*LB;*,*.HLP;*,*.LIS;* H90.BCK/SAVE_SET/LOG
$ EXIT
`)

	return b.String()
}

func help90d() string {
	return `$ ! HELP90D.COM - part D of round 1 of the Phase 90 probe: HELP's
$ ! prompts answered from a procedure's data lines, and from a file.
$ ! Written by testdata/help90/gen.go; HELP90.COM runs it. If HELP prompts
$ ! at the terminal instead, answer each prompt with Ctrl/Z, and say so.
$ SET NOON
$ SET VERIFY
$ HP := HELP/PROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A
$ HP ALPHA
Parameters

?
Qualifiers
/BRIEF



$ SHOW SYMBOL $STATUS
$ HP
ALPHA
Description
Details





$ HP
DEEP
L2
L3
NOSUCH
?
$ ! HELP's input ended at the $ line.
$ SHOW SYMBOL $STATUS
$ HP NOSUCH
ALPHA


$ HP
@H90DIR:H90B
BONLY



$ HP ALPHA
*
...
ALPHA



$ HP NOSUB


$ HP MANY
M05


$ DEFINE/USER_MODE SYS$INPUT H90DIR:RESP.TXT
$ HP ALPHA
$ HP/OUTPUT=H90P.LIS ALPHA
Parameters



$ TYPE H90P.LIS
$ EXIT
`
}

func help90v() string {
	return `$ ! HELP90V.COM - part G of round 1 of the Phase 90 probe: HELP on VMS's
$ ! own EXCHANGE, SUBMIT, and ANALYZE modules. Written by
$ ! testdata/help90/gen.go. Its log holds VMS's own help text, so it stays
$ ! out of the repository (the phase doc's Decision 9). Run it after
$ ! HELP90.COM, from the same directory:
$ !
$ !     @HELP90V/OUTPUT=HELP90V.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ !
$ ! Part G: VMS's own modules, from the main library.
$ !
$ HV := HELP/NOPROMPT/NOUSERLIBRARY
$ HV
$ HV HELP
$ HV EXCHANGE
$ HV EXCHANGE COPY
$ HV EXCHANGE COPY Qualifiers
$ HV EXCHANGE COPY /ALLOCATION
$ HV EXCHANGE /NETWORK
$ HV EXCHANGE...
$ HV SUBMIT
$ HV SUBMIT Parameter
$ HV SUBMIT Parameter filespec
$ HV SUBMIT /AFTER
$ HV SUBMIT/AFTER
$ HV SUBMIT Qualifiers /AFTER
$ HV SUBMIT /AF
$ HV SUBMIT *
$ HV SUBMIT...
$ HV ANALYZE
$ HV ANALYZE /IMAGE
$ HV ANALYZE/IMAGE
$ HV ANALYZE /IMAGE Qualifiers
$ HV ANALYZE /IMAGE /GST
$ HV ANALYZE /AUDIT /SELECT
$ HV ANALYZE /AUDIT /SELECT ACCESS
$ HV ANALYZE /MEDIA Command_Qualifiers
$ HV ANALYZE /MEDIA /LOG
$ HV ANALYZE /OBJ
$ HV NOSUCH_H90
$ ! /LIBRARY without a device and directory, and /NOLIBRARY alone.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90A ALPS
$ HELP/NOPROMPT/NOLIBRARY/NOUSERLIBRARY ALPS
$ EXIT
`
}

func help90f() string {
	return `$ ! HELP90F.COM - part F of the Phase 90 probe (round 1b): the debugger's
$ ! HELP, with DBG$HELP naming the probe's library, then with nothing
$ ! defined. Written by testdata/help90/gen.go. The second session shows
$ ! VMS's own help, so this log stays out of the repository; DBGH.DLG, the
$ ! first session's, holds only the probe's. Run it after HELP90.COM:
$ !
$ !     @HELP90F/OUTPUT=HELP90F.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ MACRO/DEBUG DBGT
$ LINK/DEBUG DBGT
$ DEFINE/USER_MODE DBG$INPUT HELP90D.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ DEFINE/USER_MODE DBG$HELP H90DIR:H90D
$ RUN DBGT
$ TYPE DBGH.DLG
$ DEFINE/USER_MODE DBG$INPUT HELP90V.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ RUN DBGT
$ EXIT
`
}

// debuggerCommands is a debugger session's commands, logged to name.DLG.
// The debugger's HELP prompts for more, reading the debugger's input
// (round 1), so four empty lines follow each request: enough to climb from
// the deepest prompt and leave HELP, the rest empty debugger commands.
func debuggerCommands(name string) string {
	var b strings.Builder
	b.WriteString("SET LOG " + name + ".DLG\nSET OUTPUT LOG,VERIFY\n")
	for _, r := range []string{
		"HELP", "HELP EXAMINE", "HELP EX", "HELP EXAMINE Qualifiers",
		"HELP EXAMINE/ASCII", "HELP EXAMINE /ASCII", "HELP EXAMINE /HEX",
		"HELP SET BREAK", "HELP SET BREAK /AFTER", "HELP SET", "HELP NOSUCH",
		"HELP Probe_Concept", "HELP DEBUG", "HELP EXTRA", "HELP *",
		"HELP EXAMINE...",
	} {
		b.WriteString(r + "\n\n\n\n\n")
	}
	// One request answered at its prompt, as a user would.
	b.WriteString("HELP SET\nBREAK\n/AFTER\n\n\n\n\n")
	b.WriteString("EXIT\n")

	return b.String()
}

const dbgt = `	.TITLE	DBGT - an image for the debugger's HELP
	.PSECT	C,EXE,NOWRT
	.ENTRY	START,^M<>
	MOVL	#1,R0
	RET
	.END	START
`

// ---------------------------------------------------------------------
// LBR90.MAR: the LBR$ routines.

// A prog collects the program's strings and cases.
type prog struct {
	data  strings.Builder
	code  strings.Builder
	cases []string
	strs  map[string]string
	n     int
}

// str returns the label of an .ASCID holding s.
func (p *prog) str(s string) string {
	if l, ok := p.strs[s]; ok {
		return l
	}

	l := fmt.Sprintf("S%d", len(p.strs)+1)
	p.strs[s] = l
	fmt.Fprintf(&p.data, "%s:\t.ASCID\t~%s~\n", l, s)

	return l
}

func (p *prog) emit(format string, a ...any) {
	fmt.Fprintf(&p.code, "\t"+format+"\n", a...)
}

// kase starts a case: a routine with the handler, its number and title.
func (p *prog) kase(title string, f func()) {
	p.n++
	name := fmt.Sprintf("C%d", p.n)
	p.cases = append(p.cases, name)
	fmt.Fprintf(&p.code, "\t.ENTRY\t%s,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n", name)
	p.emit("MOVAB\tHND,(FP)")
	p.emit("PUSHAB\t%s", p.str(title))
	p.emit("PUSHL\t#%d", p.n)
	p.emit("PUSHAB\tMCASE")
	p.emit("CALLS\t#3,OUT")
	f()
	p.emit("RET")
}

// call pushes args in reverse and calls routine. An arg is "&X" (PUSHAL X,
// an address), "$text" (PUSHAQ of an .ASCID), "#n" (PUSHL #n), or "0".
func (p *prog) call(routine string, args ...string) {
	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "&"):
			p.emit("PUSHAL\t%s", a[1:])
		case strings.HasPrefix(a, "$"):
			p.emit("PUSHAQ\t%s", p.str(a[1:]))
		case a == "0":
			p.emit("CLRL\t-(SP)")
		default:
			p.emit("PUSHL\t%s", a)
		}
	}

	p.emit("CALLS\t#%d,%s", len(args), routine)
}

// stat shows R0 as what's status, with its message.
func (p *prog) stat(what string) {
	p.emit("PUSHL\tR0")
	p.emit("PUSHAQ\t%s", p.str(what))
	p.emit("CALLS\t#2,STAT")
}

// lbr calls an LBR$ routine and shows its status.
func (p *prog) lbr(routine string, args ...string) {
	p.call("G^LBR$"+routine, args...)
	p.stat(routine)
}

// script sets the input routine's responses (a list of counted strings
// ended by 255) and the output routines' limit.
func (p *prog) script(responses []string) {
	l := fmt.Sprintf("RSP%d", p.n)
	fmt.Fprintf(&p.data, "%s:", l)
	for _, r := range responses {
		fmt.Fprintf(&p.data, "\t.ASCIC\t~%s~\n", r)
	}
	fmt.Fprintf(&p.data, "\t.BYTE\t255\n")
	p.emit("MOVAB\t%s,SCRIPT", l)
}

func lbr90() string {
	p := &prog{strs: map[string]string{}}

	// E0: the definitions' values.
	p.kase("definitions", func() {
		for _, name := range []string{
			"LBR$C_CREATE", "LBR$C_READ", "LBR$C_UPDATE",
			"LBR$C_TYP_UNK", "LBR$C_TYP_OBJ", "LBR$C_TYP_MLB", "LBR$C_TYP_HLP",
			"LBR$C_TYP_TXT", "LBR$C_TYP_SHSTB",
			"HLP$M_PROMPT", "HLP$M_PROCESS", "HLP$M_GROUP", "HLP$M_SYSTEM",
			"HLP$M_LIBLIST", "HLP$M_HELP", "HLP$M_NOHLPTXT", "HLP$M_KEYNAMLIN",
			"HLP$M_OTHERINFO",
		} {
			p.emit("PUSHL\t#%s", name)
			p.emit("PUSHAQ\t%s", p.str(name))
			p.emit("CALLS\t#2,VALUE")
		}
		for _, name := range []string{
			"LBR$_NORMAL", "LBR$_KEYNOTFND", "LBR$_ILLCTL", "LBR$_LIBNOTOPN",
			"LBR$_NOTHLPLIB", "LBR$_TYPMISMCH", "LBR$_ILLINROU", "LBR$_ILLOUTROU",
			"LBR$_NOHLPLIBS", "LBR$_TOOMNYARG", "LBR$_USRINPERR",
		} {
			p.emit("MOVL\t#%s,R0", name)
			p.stat(name)
		}
	})

	// The help library: open, header, keys, records.
	p.kase("INI_CONTROL a help library", func() {
		p.lbr("INI_CONTROL", "&IDX", "&FREAD", "&THLP")
		p.emit("PUSHL\tIDX")
		p.emit("PUSHAQ\t%s", p.str("index"))
		p.emit("CALLS\t#2,VALUE")
	})
	p.kase("OPEN H90A", func() { p.lbr("OPEN", "&IDX", "$H90DIR:H90A", "0", "$.HLB") })
	p.kase("GET_HEADER", func() {
		p.lbr("GET_HEADER", "&IDX", "&HDR")
		p.emit("CALLS\t#0,HEADER")
	})
	p.kase("LOOKUP_KEY ALPHA and its records", func() {
		p.lbr("LOOKUP_KEY", "&IDX", "$ALPHA", "&RFA")
		p.emit("PUSHAL\tRFA")
		p.emit("CALLS\t#1,SHOWRFA")
		p.emit("CALLS\t#0,RECS")
	})
	for _, k := range []string{"alpha", "ALP", "NOSUCH", "MixedCase", "MIXEDCASE", "HELP", "ALPHABET"} {
		p.kase("LOOKUP_KEY "+k, func() {
			p.lbr("LOOKUP_KEY", "&IDX", "$"+k, "&RFA2")
			p.emit("CALLS\t#0,RECS")
		})
	}
	p.kase("SET_LOCATE, ALPHABET's records, SET_MOVE", func() {
		p.lbr("SET_LOCATE", "&IDX")
		p.lbr("LOOKUP_KEY", "&IDX", "$ALPHABET", "&RFA2")
		p.emit("CALLS\t#0,RECS")
		p.lbr("SET_MOVE", "&IDX")
	})
	p.kase("FIND ALPHA's RFA", func() {
		p.lbr("FIND", "&IDX", "&RFA")
		p.emit("CALLS\t#0,RECS")
	})
	p.kase("GET_RECORD with a short buffer", func() {
		p.lbr("LOOKUP_KEY", "&IDX", "$TEXTY", "&RFA2")
		p.emit("MOVW\t#10,INBD")
		p.emit("CALLS\t#0,RECS")
		p.emit("MOVW\t#512,INBD")
	})
	p.kase("GET_RECORD with no buffers", func() {
		p.lbr("LOOKUP_KEY", "&IDX", "$NOSUB", "&RFA2")
		p.lbr("GET_RECORD", "&IDX")
		p.lbr("GET_RECORD", "&IDX", "&INBD")
	})
	for _, m := range []string{"", "A*", "%%%%", "alpha*", "*S"} {
		title := "GET_INDEX"
		args := []string{"&IDX", "&ONE", "&KEYR"}
		if m != "" {
			title += " matching " + m
			args = append(args, "$"+m)
		}
		p.kase(title, func() {
			p.emit("CLRL\tCOUNT")
			p.lbr("GET_INDEX", args...)
		})
	}
	p.kase("GET_INDEX stopped by its routine", func() {
		p.emit("CLRL\tCOUNT")
		p.emit("MOVL\t#2,LIMIT")
		p.lbr("GET_INDEX", "&IDX", "&ONE", "&KEYR")
		p.emit("CLRL\tLIMIT")
	})
	p.kase("SET_INDEX 1 and 2", func() {
		p.lbr("SET_INDEX", "&IDX", "&ONE")
		p.lbr("SET_INDEX", "&IDX", "&TWO")
	})

	// GET_HELP.
	helps := []struct {
		width string
		keys  []string
	}{
		{"0", nil},
		{"0", []string{"ALPHA"}},
		{"0", []string{"ALPHA", "Parameters"}},
		{"0", []string{"ALPHA", "/BRIEF"}},
		{"0", []string{"ALPHA", "Qualifiers", "/BRIEF"}},
		{"0", []string{"ALPHA", "*"}},
		{"0", []string{"ALPHA..."}},
		{"0", []string{"ALPHA", "..."}},
		{"0", []string{"*"}},
		{"0", []string{"AL"}},
		{"0", []string{"NOSUCH"}},
		{"0", []string{"ALPHA", "NOSUCH"}},
		{"0", []string{"HELP"}},
		{"0", []string{"DEEP", "L2", "L3", "L4", "L5", "L6", "L7", "L8", "L9"}},
		{"0", []string{"DEEP", "L2", "L3", "L4", "L5", "L6", "L7", "L8", "L9", "L10"}},
		{"0", []string{"DEEP", "L2", "L3", "L4", "L5", "L6", "L7", "L8", "L9", "L10", "L11"}},
		{"&W40", []string{"WIDE"}},
		{"&W132", []string{"MANY"}},
		{"&W80", []string{"TEXTY"}},
		{"0", []string{"QUALS", "/QQ1"}},
		{"0", []string{"QUALS", "/CQ1"}},
		{"0", []string{"ODD"}},
		{"0", []string{"EMPTY"}},
		{"0", []string{"SAME", "Twice"}},
	}
	for _, h := range helps {
		h := h
		p.kase("GET_HELP "+strings.Join(h.keys, " ")+" width "+h.width, func() {
			args := []string{"&IDX", h.width, "&HELPR", "0"}
			for _, k := range h.keys {
				args = append(args, "$"+k)
			}
			p.lbr("GET_HELP", args...)
		})
	}
	p.kase("GET_HELP with data", func() {
		p.lbr("GET_HELP", "&IDX", "0", "&HELPR", "&DATAV", "$ALPHA", "$Examples")
	})
	p.kase("GET_HELP with no routine", func() {
		p.lbr("GET_HELP", "&IDX", "0", "0", "0", "$ALPHA", "$Examples")
	})
	p.kase("GET_HELP stopped by its routine", func() {
		p.emit("CLRL\tCOUNT")
		p.emit("MOVL\t#3,LIMIT")
		p.lbr("GET_HELP", "&IDX", "0", "&HELPR", "0", "$ALPHA")
		p.emit("CLRL\tLIMIT")
	})

	// Closing, and what comes after.
	p.kase("CLOSE, then use the index", func() {
		p.lbr("CLOSE", "&IDX")
		p.lbr("LOOKUP_KEY", "&IDX", "$ALPHA", "&RFA2")
		p.lbr("GET_RECORD", "&IDX", "&INBD", "&OUTBD")
		p.lbr("CLOSE", "&IDX")
	})
	p.kase("an index never set", func() {
		p.emit("MOVL\t#12345,IDX2")
		p.lbr("LOOKUP_KEY", "&IDX2", "$ALPHA", "&RFA2")
	})

	// A text library and a macro library.
	p.kase("a text library", func() {
		p.lbr("INI_CONTROL", "&IDX2", "&FREAD", "&TTXT")
		p.lbr("OPEN", "&IDX2", "$H90DIR:H90T", "0", "$.TLB")
		p.emit("CALLS\t#0,HEADER2")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX2", "&ONE", "&KEYR")
		p.lbr("LOOKUP_KEY", "&IDX2", "$T1", "&RFA2")
		p.emit("PUSHAL\tIDX2")
		p.emit("CALLS\t#1,RECSI")
		p.lbr("GET_HELP", "&IDX2", "0", "&HELPR", "0", "$T1")
		p.lbr("CLOSE", "&IDX2")
	})
	p.kase("a macro library", func() {
		p.lbr("INI_CONTROL", "&IDX2", "&FREAD", "&TMLB")
		p.lbr("OPEN", "&IDX2", "$H90DIR:H90M", "0", "$.MLB")
		p.emit("CALLS\t#0,HEADER2")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX2", "&ONE", "&KEYR")
		p.lbr("LOOKUP_KEY", "&IDX2", "$PROBE1", "&RFA2")
		p.emit("PUSHAL\tIDX2")
		p.emit("CALLS\t#1,RECSI")
		p.lbr("CLOSE", "&IDX2")
	})
	p.kase("a help library opened as a text library", func() {
		p.lbr("INI_CONTROL", "&IDX2", "&FREAD", "&TTXT")
		p.lbr("OPEN", "&IDX2", "$H90DIR:H90A", "0", "$.HLB")
		p.emit("CALLS\t#0,G^LBR$RET_RMSSTV")
		p.stat("RET_RMSSTV")
		p.lbr("CLOSE", "&IDX2")
	})
	p.kase("a library that isn't there", func() {
		p.lbr("INI_CONTROL", "&IDX2", "&FREAD", "&THLP")
		p.lbr("OPEN", "&IDX2", "$H90DIR:NOSUCH", "0", "$.HLB")
		p.emit("CALLS\t#0,G^LBR$RET_RMSSTV")
		p.stat("RET_RMSSTV")
		p.lbr("CLOSE", "&IDX2")
	})
	p.kase("OPEN with no default name", func() {
		p.lbr("INI_CONTROL", "&IDX2", "&FREAD", "&THLP")
		p.lbr("OPEN", "&IDX2", "$H90DIR:H90A")
		p.lbr("CLOSE", "&IDX2")
	})

	// OUTPUT_HELP.
	type out struct {
		title string
		args  []string
		resp  []string
		limit int
	}
	lib := "$H90DIR:H90A"
	outs := []out{
		{"ALPHA, flags 0", []string{"&ORTN", "0", "$ALPHA", lib, "&ZERO"}, nil, 0},
		{"no line, flags 0", []string{"&ORTN", "0", "0", lib, "&ZERO"}, nil, 0},
		{"no line, HELP", []string{"&ORTN", "0", "0", lib, "&FHELP"}, nil, 0},
		{"no line, LIBLIST and PROCESS", []string{"&ORTN", "0", "0", lib, "&FLIBP"}, nil, 0},
		{"HELP, flags 0", []string{"&ORTN", "0", "$HELP", lib, "&ZERO"}, nil, 0},
		{"ALPHA Parameters, width 40", []string{"&ORTN", "&W40", "$ALPHA Parameters", lib, "&ZERO"}, nil, 0},
		{"ALPHA/BRIEF", []string{"&ORTN", "0", "$ALPHA/BRIEF", lib, "&ZERO"}, nil, 0},
		{"NOSUCH", []string{"&ORTN", "0", "$NOSUCH", lib, "&ZERO"}, nil, 0},
		{"@H90DIR:H90B BONLY", []string{"&ORTN", "0", "$@H90DIR:H90B BONLY", lib, "&ZERO"}, nil, 0},
		{"BONLY, PROCESS", []string{"&ORTN", "0", "$BONLY", lib, "&FPROC"}, nil, 0},
		{"BONLY, flags omitted", []string{"&ORTN", "0", "$BONLY", lib}, nil, 0},
		{"prompting from ALPHA", []string{"&ORTN", "0", "$ALPHA", lib, "&FPROMPT", "&IRTN"},
			[]string{"Parameters", "", "?", "Qualifiers", "/BRIEF", "", "", ""}, 0},
		{"prompting from no line", []string{"&ORTN", "0", "0", lib, "&FPROMPT", "&IRTN"},
			[]string{"ALPHA", "Description", "Details", "", "", "", ""}, 0},
		{"prompting into another library", []string{"&ORTN", "0", "0", lib, "&FPROMPT", "&IRTN"},
			[]string{"@H90DIR:H90B", "BONLY", "", "", ""}, 0},
		{"prompting down DEEP", []string{"&ORTN", "0", "$DEEP", lib, "&FPROMPT", "&IRTN"},
			[]string{"L2", "L3", "NOSUCH", "?", "", "", "", ""}, 0},
		{"prompting, end of file at once", []string{"&ORTN", "0", "0", lib, "&FPROMPT", "&IRTN"},
			[]string{}, 0},
		{"prompting, * and ...", []string{"&ORTN", "0", "$ALPHA", lib, "&FPROMPT", "&IRTN"},
			[]string{"*", "...", "ALPHA", "", ""}, 0},
		{"prompting from NOSUCH", []string{"&ORTN", "0", "$NOSUCH", lib, "&FPROMPT", "&IRTN"},
			[]string{"ALPHA", "", ""}, 0},
		{"prompting with no input routine", []string{"&ORTN", "0", "$ALPHA", lib, "&FPROMPT"}, nil, 0},
		{"no output routine", []string{"0", "0", "$ALPHA", lib, "&ZERO"}, nil, 0},
		{"output routine failing", []string{"&ORTN", "0", "$ALPHA", lib, "&ZERO"}, nil, 2},
		{"too many arguments", []string{"&ORTN", "0", "$ALPHA", lib, "&ZERO", "0", "0"}, nil, 0},
	}
	for _, o := range outs {
		o := o
		p.kase("OUTPUT_HELP "+o.title, func() {
			p.emit("CLRL\tSCRIPT")
			if o.resp != nil {
				p.script(o.resp)
			}
			p.emit("CLRL\tCOUNT")
			p.emit("MOVL\t#%d,LIMIT", o.limit)
			p.lbr("OUTPUT_HELP", o.args...)
			p.emit("CLRL\tLIMIT")
		})
	}

	var b strings.Builder
	b.WriteString(`	.TITLE	LBR90 - Phase 90's probe of the LBR$ routines
; Written by testdata/help90/gen.go.
	$SSDEF
	$RMSDEF
	$LBRDEF
	$HLPDEF
	.PSECT	D,NOEXE,WRT,LONG
IDX:	.LONG	0
IDX2:	.LONG	0
FREAD:	.LONG	LBR$C_READ
THLP:	.LONG	LBR$C_TYP_HLP
TTXT:	.LONG	LBR$C_TYP_TXT
TMLB:	.LONG	LBR$C_TYP_MLB
ZERO:	.LONG	0
ONE:	.LONG	1
TWO:	.LONG	2
W40:	.LONG	40
W80:	.LONG	80
W132:	.LONG	132
FHELP:	.LONG	HLP$M_HELP
FLIBP:	.LONG	HLP$M_LIBLIST!HLP$M_PROCESS
FPROC:	.LONG	HLP$M_PROCESS
FPROMPT: .LONG	HLP$M_PROMPT
RFA:	.LONG	0,0
RFA2:	.LONG	0,0
HDR:	.BLKL	128
COUNT:	.LONG	0
LIMIT:	.LONG	0
DATAV:	.LONG	^X12345678
SCRIPT:	.LONG	0
RLEN:	.WORD	0
INB:	.BLKB	512
INBD:	.WORD	512
	.BYTE	14,1
	.ADDRESS INB
OUTBD:	.WORD	0
	.BYTE	14,1
	.LONG	0
NONE:	.ASCID	~(none)~
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
BD:	.WORD	400
	.BYTE	14,1
	.ADDRESS BF
BF:	.BLKB	400
MT:	.BLKB	256
MTD:	.WORD	256
	.BYTE	14,1
	.ADDRESS MT
MTL:	.WORD	0
MCASE:	.ASCID	~#!UL !AS~
MVAL:	.ASCID	~  !AS = !XL~
MSTAT:	.ASCID	~  !AS: r=!XL !AD~
MRFA:	.ASCID	~  rfa=!UL,!UW~
MREC:	.ASCID	~  rec !UL n=!UW |!AS|~
MRECS:	.ASCID	~  records end: r=!XL~
MKEY:	.ASCID	~  key |!AS| rfa=!UL,!UW~
MHLP:	.ASCID	~  h f=!XL lv=!XL la=!UL d=!UL |!AS|~
MOUT:	.ASCID	~  o |!AS|~
MIN:	.ASCID	~  i |!AS| -> |!AD|~
MINE:	.ASCID	~  i |!AS| -> end of file~
MHDR:	.ASCID	~  !3UL:!8(9XL)~
MCOND:	.ASCID	~  =c !XL n=!UL a=!XL !XL~
MUNW:	.ASCID	~  =u~
`)
	b.WriteString(p.data.String())
	b.WriteString(`	.PSECT	C,EXE,NOWRT,LONG
; OUT(control, values...) writes a line.
	.ENTRY	OUT,^M<R2,R3,R4,R5>
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=LD,OUTBUF=BD,PRMLST=8(AP)
	PUSHAB	LD
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
; VALUE(name, value)
	.ENTRY	VALUE,^M<>
	PUSHL	8(AP)
	PUSHL	4(AP)
	PUSHAB	MVAL
	CALLS	#3,OUT
	RET
; STAT(name, status) shows a status and its message.
	.ENTRY	STAT,^M<R2>
	MOVL	8(AP),R2
	CLRW	MTL
	PUSHL	#0
	PUSHL	#15
	PUSHAQ	MTD
	PUSHAW	MTL
	PUSHL	R2
	CALLS	#5,G^SYS$GETMSG
	PUSHAB	MT
	MOVZWL	MTL,-(SP)
	PUSHL	R2
	PUSHL	4(AP)
	PUSHAB	MSTAT
	CALLS	#5,OUT
	MOVL	R2,R0
	RET
; HND shows a condition, continues below severe, and unwinds from a
; severe one.
HND:	.WORD	^M<R2>
	MOVL	4(AP),R2
	CMPL	4(R2),#SS$_UNWIND
	BEQL	50$
	PUSHL	12(R2)
	PUSHL	8(R2)
	PUSHL	(R2)
	PUSHL	4(R2)
	PUSHAB	MCOND
	CALLS	#5,OUT
	BICL3	#^XFFFFFFF8,4(R2),R0
	CMPL	R0,#4
	BEQL	40$
	MOVL	#SS$_CONTINUE,R0
	RET
40$:	PUSHAB	MUNW
	CALLS	#1,OUT
	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
; SHOWRFA(rfa) shows an RFA.
	.ENTRY	SHOWRFA,^M<R2>
	MOVL	4(AP),R2
	MOVZWL	4(R2),-(SP)
	PUSHL	(R2)
	PUSHAB	MRFA
	CALLS	#3,OUT
	RET
; RECS shows IDX's records from the current one; RECSI(index) another's.
	.ENTRY	RECS,^M<>
	PUSHAL	IDX
	CALLS	#1,RECSI
	RET
	.ENTRY	RECSI,^M<R2,R3>
	CLRL	R3
10$:	CLRW	OUTBD
	CLRL	OUTBD+4
	PUSHAQ	OUTBD
	PUSHAQ	INBD
	PUSHL	4(AP)
	CALLS	#3,G^LBR$GET_RECORD
	MOVL	R0,R2
	BLBC	R2,20$
	INCL	R3
	PUSHAQ	OUTBD
	MOVZWL	OUTBD,-(SP)
	PUSHL	R3
	PUSHAB	MREC
	CALLS	#4,OUT
	CMPL	R3,#200
	BLSS	10$
20$:	PUSHL	R2
	PUSHAB	MRECS
	CALLS	#2,OUT
	RET
; HEADER shows LBR$GET_HEADER's 128 longwords; HEADER2 gets IDX2's first.
	.ENTRY	HEADER2,^M<>
	PUSHAL	HDR
	PUSHAL	IDX2
	CALLS	#2,G^LBR$GET_HEADER
	PUSHL	R0
	PUSHAQ	HNAME
	CALLS	#2,STAT
	CALLS	#0,HEADER
	RET
HNAME:	.ASCID	~GET_HEADER~
	.ENTRY	HEADER,^M<R2,R3,R4>
	MOVAL	HDR,R2
	CLRL	R3
10$:	PUSHL	28(R2)
	PUSHL	24(R2)
	PUSHL	20(R2)
	PUSHL	16(R2)
	PUSHL	12(R2)
	PUSHL	8(R2)
	PUSHL	4(R2)
	PUSHL	(R2)
	PUSHL	R3
	PUSHAB	MHDR
	CALLS	#10,OUT
	ADDL2	#32,R2
	ADDL2	#8,R3
	CMPL	R3,#128
	BLSS	10$
	RET
; KEYR(key, rfa), GET_INDEX's routine: shows the key, and fails once
; COUNT reaches a nonzero LIMIT.
	.ENTRY	KEYR,^M<R2>
	INCL	COUNT
	MOVL	8(AP),R2
	MOVZWL	4(R2),-(SP)
	PUSHL	(R2)
	PUSHL	4(AP)
	PUSHAB	MKEY
	CALLS	#4,OUT
	BRB	LIMITED
; LIMITED returns 0 once COUNT reaches a nonzero LIMIT, else 1.
LIMITED: MOVL	#1,R0
	TSTL	LIMIT
	BEQL	10$
	CMPL	COUNT,LIMIT
	BLSS	10$
	CLRL	R0
10$:	RET
; HELPR(line, flags, data, level), GET_HELP's routine. The level is
; shown at the address the fourth argument holds when it looks like an
; address (la=1), else as it is (la=0); d=1 if the data argument is
; DATAV's address, 2 if it's the address of a 0, else 0.
	.ENTRY	HELPR,^M<R2,R3,R4>
	INCL	COUNT
	MOVL	16(AP),R2
	CLRL	R3
	CMPL	R2,#^X1000
	BLSSU	10$
	MOVL	(R2),R2
	MOVL	#1,R3
10$:	CLRL	R4
	CMPL	12(AP),#DATAV
	BNEQ	20$
	MOVL	#1,R4
	BRB	30$
20$:	TSTL	12(AP)
	BEQL	30$
	TSTL	@12(AP)
	BNEQ	30$
	MOVL	#2,R4
30$:	PUSHL	4(AP)
	PUSHL	R4
	PUSHL	R3
	PUSHL	R2
	PUSHL	@8(AP)
	PUSHAB	MHLP
	CALLS	#6,OUT
	BRW	LIMITED
; ORTN(line), OUTPUT_HELP's output routine.
	.ENTRY	ORTN,^M<>
	INCL	COUNT
	PUSHL	4(AP)
	PUSHAB	MOUT
	CALLS	#2,OUT
	BRW	LIMITED
; IRTN(string, prompt, length), OUTPUT_HELP's input routine: the next of
; SCRIPT's counted strings, or RMS$_EOF at the 255 that ends them.
	.ENTRY	IRTN,^M<R2,R3>
	MOVAQ	NONE,R3
	CMPB	(AP),#2
	BLSS	5$
	TSTL	8(AP)
	BEQL	5$
	MOVL	8(AP),R3
5$:	MOVL	SCRIPT,R2
	BEQL	20$
	CMPB	(R2),#255
	BEQL	20$
	MOVZBW	(R2),RLEN
	PUSHL	4(AP)
	PUSHAB	1(R2)
	PUSHAW	RLEN
	CALLS	#3,G^LIB$SCOPY_R_DX
	CMPB	(AP),#3
	BLSS	10$
	TSTL	12(AP)
	BEQL	10$
	MOVW	RLEN,@12(AP)
10$:	MOVL	SCRIPT,R2
	PUSHAB	1(R2)
	MOVZBL	(R2),-(SP)
	PUSHL	R3
	PUSHAB	MIN
	CALLS	#4,OUT
	MOVZBL	@SCRIPT,R0
	ADDL2	#1,R0
	ADDL2	R0,SCRIPT
	MOVL	#1,R0
	RET
20$:	PUSHL	R3
	PUSHAB	MINE
	CALLS	#2,OUT
	MOVL	#RMS$_EOF,R0
	RET
`)
	b.WriteString(p.code.String())
	b.WriteString("\t.ENTRY\tSTART,^M<>\n")
	for _, c := range p.cases {
		fmt.Fprintf(&b, "\tCALLS\t#0,%s\n", c)
	}
	b.WriteString("\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// ---------------------------------------------------------------------
// The exchange volume.

func exchange() string {
	var b strings.Builder
	b.WriteString(`! EXCHANGE.CMD - builds round 1b's exchange volume with govax.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/help90b-exchange.dsk" /DEVICE=RD54 HELP90
MOUNT/WRITE DUA5 "testdata/disks/help90b-exchange.dsk"
`)
	for _, f := range files {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}
	b.WriteString("DIRECTORY DUA5:[000000]\nDISMOUNT DUA5\n")

	return b.String()
}

// copyout brings everything back to a local directory first: nothing goes
// into vax/ until it's been checked for VMS's own help text (README.md).
func copyout() string {
	return `! COPYOUT.CMD - copies the VAX run's output off the exchange volume,
! into testdata/disks/help90-vms/round1b/, which git ignores. README.md says how
! the logs are checked before any of them goes into vax/.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90b-exchange.dsk"
COPY DUA5:[000000]HELP90.LOG "testdata/disks/help90-vms/round1b/help90.log"/HOST
COPY DUA5:[000000]H90.BCK "testdata/disks/help90-vms/round1b/h90.bck"/HOST/BINARY
COPY DUA5:[000000]HELP90F.LOG "testdata/disks/help90-vms/round1b/help90f.log"/HOST
COPY DUA5:[000000]DBGH.DLG "testdata/disks/help90-vms/round1b/dbgh.dlg"/HOST
COPY DUA5:[000000]DBGV.DLG "testdata/disks/help90-vms/round1b/dbgv.dlg"/HOST
DISMOUNT DUA5
`
}

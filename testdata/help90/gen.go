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
// the debugger's command files, dbgt.mar, and lbr90.mar; and round 3's
// (subtask 11): lbr90w.mar (the LBR$ writing routines on every kind of
// library), help90w.com, their input files, and exchange.cmd and
// copyout.cmd, which are round 3's since it was written (rounds 1 to 1c's
// are in the repository's history).
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
	write("help90c.com", help90c())
	write("help90d.dbg", debuggerCommands("DBGH"))
	write("help90v.dbg", debuggerCommands("DBGV"))
	write("dbgt.mar", dbgt)
	write("lbr90.mar", lbr90())

	for name, text := range round3Modules() {
		write(name, text)
	}

	write("lbr90w.mar", lbr90w())
	write("help90w.com", help90w())
	write("exchange.cmd", exchange())
	write("copyout.cmd", copyout())
}

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

// files is every file rounds 1 to 1c needed, in the order their
// exchange.cmd copied them.
var files = []string{
	"h90a1.hlp", "h90a2.hlp", "alpha2.hlp", "h90b.hlp", "h90c.hlp",
	"h90g.hlp", "h90s.hlp", "h90d.hlp", "badtext.hlp", "skip.hlp",
	"noblank.hlp", "toolong.hlp", "lower.hlp", "digits.hlp", "zero.hlp", "t1.txt", "t2.txt",
	"h90m.mar", "resp.txt", "help90.com", "help90d.com", "help90v.com",
	"help90f.com", "help90d.dbg", "help90v.dbg", "dbgt.mar", "lbr90.mar",
	"deeper.hlp", "help90c.com",
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
		"deeper.hlp":  deeper(),
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

// deeper is round 1c's DEEPER: what DEEP was meant to be (round 1b's DEEP
// puts each level under the sibling before it, so L3 is DEEP S2 L3). Here
// each L is the next one's parent, and each S follows its L's whole
// subtree, so DEEPER L2 L3 ... L9 is a path and S2 to S9 are leaves.
func deeper() string {
	var b strings.Builder

	b.WriteString("1 DEEPER\n   Text of DEEPER, level 1.\n")
	for lv := 2; lv <= 9; lv++ {
		path := "DEEPER"
		for k := 2; k <= lv; k++ {
			path += fmt.Sprintf(" L%d", k)
		}
		fmt.Fprintf(&b, "%d L%d\n   Text of %s, level %d.\n", lv, lv, path, lv)
	}
	for lv := 9; lv >= 2; lv-- {
		fmt.Fprintf(&b, "%d S%d\n   Text of S%d, L%d's sibling at level %d, a leaf.\n", lv, lv, lv, lv, lv)
	}

	return b.String()
}

// help90c is round 1c: DEEPER's cases, and the libraries round 2's
// terminal session uses, made again on this round's volume.
func help90c() string {
	var b strings.Builder
	b.WriteString(`$ ! HELP90C.COM - round 1c of the Phase 90 probe: HELP on a chain of
$ ! levels to 9 (round 1b's DEEP was nested wrongly), and the libraries
$ ! round 2's terminal session uses. Written by testdata/help90/gen.go.
$ ! Run it with the exchange volume's [000000] as the default directory:
$ !
$ !     @HELP90C/OUTPUT=HELP90C.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ LIBRARY/CREATE/HELP H90A H90A1,H90A2
$ LIBRARY/CREATE/HELP H90B H90B
$ LIBRARY/CREATE/HELP H90D H90D
$ LIBRARY/CREATE/HELP H90Y DEEPER
$ MACRO/DEBUG DBGT
$ LINK/DEBUG DBGT
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP H90Y
$ H := HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90Y
`)
	for _, c := range deeperCases {
		fmt.Fprintf(&b, "$ H %s\n", c)
	}
	b.WriteString(`$ ! KEY... on a topic with no subtopics, in H90A.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A ALPHA Parameters...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A NOSUB...
$ ! Prompting down the chain, from a procedure's data lines.
$ HELP/PROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90Y DEEPER
L2
L3
L4
?
S5
NOSUCH








$ SHOW SYMBOL $STATUS
$ EXIT
`)

	return b.String()
}

// deeperCases are round 1c's requests on DEEPER.
var deeperCases = []string{
	"DEEPER", "DEEPER L2", "DEEPER L2 L3", "DEEPER L2 L3 L4 L5 L6 L7 L8",
	"DEEPER L2 L3 L4 L5 L6 L7 L8 L9", "DEEPER L2 L3 L4 L5 L6 L7 L8 L9 L10",
	"DEEPER L2 L3 L4 L5 L6 L7 L8 S9", "DEEPER S2", "DEEPER L2 S3",
	"DEEPER L", "DEEPER L2 L", "DEEPER S", "DEEPER *", "DEEPER * *", "DEEPER * * *",
	"DEEPER...", "DEEPER L2...", "DEEPER L2 L3 L4 L5 L6 L7 L8...",
	"DEEPER L2 L3 L4 L5 L6 L7 L8 L9...", "DEEPER S2...", "DEEPER L2 L3 L4 L5 L6 L7 L8 L9 *",
	"DEEPER L2 *...", "DEEPER * L3", "DEEPER % L3", "DEEPER L% L%",
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
// Round 3 (subtask 11): the LBR$ writing routines, on every kind of
// library, and what subtask 3 left of LIBRARY/HELP.

// round3Files is every file round 3's VAX run needs, in exchange.cmd's
// order.
var round3Files = []string{
	"t1.txt", "t2.txt", "one.txt", "two.txt", "w10.hlp", "h90o.mar",
	"h90om.mar", "h90b.hlp", "toolong.hlp", "digits.hlp", "multi.hlp",
	"notfirst.hlp", "goodbad.hlp", "lbr90w.mar", "help90w.com",
}

// round3Modules is round 3's input files: one.txt and two.txt are W2's
// modules as LIBRARY makes them (W2L), w10.hlp W10's (W10L), h90o.mar the
// object module W12 holds, and the help files the LIBRARY cases of part D.
func round3Modules() map[string]string {
	return map[string]string{
		"one.txt":      "One's first record.\nOne's second record.\nOne's third record.\n",
		"two.txt":      "Two's only record.\n",
		"w10.hlp":      w10Lines,
		"multi.hlp":    "1 TWO WORDS\n   Text of TWO WORDS.\n1 THREE WORD KEY\n   Text of THREE WORD KEY.\n",
		"notfirst.hlp": "2 SUBFIRST\n   Text of SUBFIRST, a level-2 key first.\n1 AFTER\n   Text of AFTER.\n",
		"goodbad.hlp":  "1 GOODONE\n   Text of GOODONE.\n1 BADONE\n   Text of BADONE.\n3 SKIPPED\n   Text of SKIPPED.\n1 LATER\n   Text of LATER.\n",
		"h90o.mar": `	.TITLE	H90O - an object module for LBR90W's object library
	.PSECT	C,EXE,NOWRT
	.ENTRY	H90O_SYM,^M<>
	MOVL	#1,R0
	RET
	.ENTRY	H90O_TWO,^M<>
	MOVL	#3,R0
	RET
	.END
`,
		"h90om.mar": `	.TITLE	H90OM - calls H90O_SYM, found in W12.OLB
	.PSECT	C,EXE,NOWRT
	.ENTRY	START,^M<>
	CALLS	#0,G^H90O_SYM
	RET
	.END	START
`,
	}
}

// w10Lines is W10's two modules, which LBR90W writes record by record and
// LIBRARY/HELP makes W10L from.
const w10Lines = "1 WALPHA\n   Text of WALPHA.\n2 Sub\n   Text of WALPHA Sub.\n1 walpha2\n   Text of walpha2.\n"

// opts writes a create options array: the 20 longwords LBR$OPEN takes, the
// first nine in the manual's order (type, key length, allocation, indexes,
// header bytes, entries, history records, format, casing).
func (p *prog) opts(label string, v ...string) {
	fmt.Fprintf(&p.data, "%s:", label)
	for i := 0; i < 20; i++ {
		x := "0"
		if i < len(v) {
			x = v[i]
		}
		fmt.Fprintf(&p.data, "\t.LONG\t%s\n", x)
	}
}

func (p *prog) ini(idx, fn, typ string) { p.lbr("INI_CONTROL", "&"+idx, "&"+fn, "&"+typ) }

func (p *prog) open(idx, name, opts, dns string) {
	o := "0"
	if opts != "" {
		o = "&" + opts
	}
	p.lbr("OPEN", "&"+idx, "$"+name, o, "$"+dns)
}

func (p *prog) closeLib(idx string) { p.lbr("CLOSE", "&"+idx) }

// module writes a module's records, shows the RFA the first returned, and
// ends the module.
func (p *prog) module(idx, rfa string, recs ...string) {
	for i, r := range recs {
		p.lbr("PUT_RECORD", "&"+idx, "$"+r, "&"+rfa)
		if i == 0 {
			p.showRFA(rfa)
		}
	}
	p.lbr("PUT_END", "&"+idx)
}

func (p *prog) showRFA(rfa string) {
	p.emit("PUSHAL\t%s", rfa)
	p.emit("CALLS\t#1,SHOWRFA")
}

func (p *prog) insert(idx, key, rfa string) { p.lbr("INSERT_KEY", "&"+idx, "$"+key, "&"+rfa) }

func (p *prog) header(idx string) {
	p.emit("PUSHAL\t%s", idx)
	p.emit("CALLS\t#1,HEADERI")
}

func (p *prog) index(idx string) {
	p.emit("CLRL\tCOUNT")
	p.lbr("GET_INDEX", "&"+idx, "&ONE", "&KEYR")
}

// records looks a key up and shows its module's records (or, with count,
// only how many there are and their length).
func (p *prog) records(idx, key string, count bool) {
	p.lbr("LOOKUP_KEY", "&"+idx, "$"+key, "&RFAX")
	p.emit("PUSHAL\t%s", idx)
	if count {
		p.emit("CALLS\t#1,RECN")
	} else {
		p.emit("CALLS\t#1,RECSI")
	}
}

// modhdr shows a module's header through LBR$SET_MODULE.
func (p *prog) modhdr(idx, rfa string) {
	p.emit("PUSHAL\t%s", rfa)
	p.emit("PUSHAL\t%s", idx)
	p.emit("CALLS\t#2,MODHDR")
}

// readback opens a library to read it and shows its header, its keys, and
// each key's records.
func (p *prog) readback(name, typ, dns string, keys []string, counted ...string) {
	p.ini("IDX3", "FREAD", typ)
	p.open("IDX3", name, "", dns)
	p.header("IDX3")
	p.index("IDX3")
	for _, k := range keys {
		p.records("IDX3", k, false)
	}
	for _, k := range counted {
		p.records("IDX3", k, true)
	}
	p.closeLib("IDX3")
}

func lbr90w() string {
	p := &prog{strs: map[string]string{}}

	txt := []string{"LBR$C_TYP_TXT", "31", "8", "1", "0", "96", "20", "CRE$C_VMSV3", "CRE$C_MACTXTCAS"}
	with := func(base []string, at int, v string) []string {
		o := append([]string(nil), base...)
		o[at] = v
		return o
	}
	p.opts("OPTT", txt...)
	p.opts("OPTU", with(txt, 4, "8")...)
	p.opts("OPTH3", with(txt, 6, "3")...)
	p.opts("OPTH0", with(txt, 6, "0")...)
	p.opts("OPTS", with(with(txt, 2, "1"), 5, "8")...)
	p.opts("OPTV2", with(txt, 7, "CRE$C_VMSV2")...)
	p.opts("OPTX9", with(txt, 3, "9")...)
	p.opts("OPTK", with(txt, 1, "200")...)
	p.opts("OPTZ", "LBR$C_TYP_TXT")
	p.opts("OPTH", "LBR$C_TYP_HLP", "31", "8", "1", "0", "96", "20", "CRE$C_VMSV3", "CRE$C_HLPCASING")
	p.opts("OPTM", "LBR$C_TYP_MLB", "31", "8", "1", "0", "96", "20", "CRE$C_VMSV3", "CRE$C_MACTXTCAS")
	p.opts("OPTO", "LBR$C_TYP_OBJ", "31", "8", "2", "0", "96", "20", "CRE$C_VMSV3", "CRE$C_OBJCASING")
	p.opts("OPTB", "LBR$C_TYP_USRLW", "0", "8", "1", "0", "96", "20", "CRE$C_VMSV3", "CRE$C_MACTXTCAS")

	// The definitions round 1 didn't print, and every LBR$_ status's text.
	p.kase("definitions", func() {
		for _, name := range []string{
			"CRE$C_VMSV2", "CRE$C_VMSV3", "CRE$C_HLPCASING", "CRE$C_OBJCASING",
			"CRE$C_MACTXTCAS", "CRE$L_TYPE", "CRE$L_KEYLEN", "CRE$L_ALLOC",
			"CRE$L_IDXMAX", "CRE$L_UHDMAX", "CRE$L_ENTALL", "CRE$L_LUHMAX",
			"CRE$L_VERTYP", "CRE$L_IDXOPT", "LBR$C_MAXRECSIZ", "LBR$C_FLUSHDATA",
			"LBR$C_FLUSHALL", "LBR$C_TYP_USRLW", "LBR$C_TYP_USRHI", "MHD$B_USRDAT",
			"LHI$L_TYPE", "LHI$L_NINDEX", "LHI$L_MODCNT", "LHI$L_NUMLUHREC",
		} {
			p.emit("PUSHL\t#%s", name)
			p.emit("PUSHAQ\t%s", p.str(name))
			p.emit("CALLS\t#2,VALUE")
		}
		for _, name := range []string{
			"ALLWRNGBLK", "BADPARAM", "DUPKEY", "EMPTYHIST", "ENDTOPIC", "ERRCLOSE",
			"HDRTRUNC", "ILLCREOPT", "ILLCTL", "ILLFMT", "ILLFUNC", "ILLIDXNUM",
			"ILLINROU", "ILLOP", "ILLOUTROU", "ILLOUTWID", "ILLTYP", "INTRNLERR",
			"INVKEY", "INVNAM", "INVRFA", "KEYINDEX", "KEYINS", "KEYNOTFND",
			"LIBNOTOPN", "LIBOPN", "LKPNOTDON", "NOFILNAM", "NOHISTORY", "NOHLPTXT",
			"NOMTCHFOU", "NORMAL", "NOTHLPLIB", "NOUPDHIST", "NULIDX", "OLDLIBRARY",
			"OLDMISMCH", "RECLNG", "RECTRUNC", "REFCNTZERO", "RFAPASTEOF",
			"STILLKEYS", "TOOMNYARG", "TOOMNYLIB", "TYPMISMCH", "UPDURTRAV",
			"USRINPERR", "WRITEERR",
		} {
			p.emit("MOVL\t#LBR$_%s,R0", name)
			p.stat("LBR$_" + name)
		}
	})

	// W1: an empty text library, with the manual's options.
	p.kase("W1, an empty text library", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W1", "OPTT", ".TLB")
		p.header("IDX")
		p.closeLib("IDX")
	})
	p.kase("W1 read back", func() { p.readback("H90DIR:W1", "TTXT", ".TLB", nil) })

	// W2: two modules, the same as LIBRARY's W2L from ONE.TXT and TWO.TXT.
	p.kase("W2, two modules", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W2", "OPTT", ".TLB")
		p.module("IDX", "RFA1", "One's first record.", "One's second record.", "One's third record.")
		p.insert("IDX", "ONE", "RFA1")
		p.module("IDX", "RFA2", "Two's only record.")
		p.insert("IDX", "TWO", "RFA2")
		p.records("IDX", "ONE", false)
		p.index("IDX")
		p.header("IDX")
		p.modhdr("IDX", "RFA1")
		p.closeLib("IDX")
	})
	p.kase("W2 read back", func() { p.readback("H90DIR:W2", "TTXT", ".TLB", []string{"ONE", "TWO"}) })

	// W2 changed: a module added, one replaced.
	p.kase("W2 updated: THREE added, TWO replaced", func() {
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:W2", "", ".TLB")
		p.header("IDX")
		p.module("IDX", "RFA3", "Three's first record.", "", "Three's third record, after an empty one.")
		p.insert("IDX", "THREE", "RFA3")
		p.lbr("LOOKUP_KEY", "&IDX", "$TWO", "&RFA4")
		p.showRFA("RFA4")
		p.module("IDX", "RFA5", "Two's new record.", "Two's second new record.")
		p.lbr("REPLACE_KEY", "&IDX", "$TWO", "&RFA4", "&RFA5")
		p.lbr("DELETE_DATA", "&IDX", "&RFA4")
		p.header("IDX")
		p.closeLib("IDX")
	})
	p.kase("W2 read back again", func() {
		p.readback("H90DIR:W2", "TTXT", ".TLB", []string{"ONE", "TWO", "THREE"})
	})

	// W3: deletion, and what a new module takes of the space it frees.
	p.kase("W3, B deleted and D added", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W3", "OPTT", ".TLB")
		p.module("IDX", "RFA1", "A's record.")
		p.insert("IDX", "A", "RFA1")
		p.module("IDX", "RFA2", "B's first record.", "B's second record.")
		p.insert("IDX", "B", "RFA2")
		p.module("IDX", "RFA3", "C's record.")
		p.insert("IDX", "C", "RFA3")
		p.closeLib("IDX")
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:W3", "", ".TLB")
		p.lbr("LOOKUP_KEY", "&IDX", "$B", "&RFA4")
		p.lbr("DELETE_KEY", "&IDX", "$B")
		p.lbr("DELETE_DATA", "&IDX", "&RFA4")
		p.lbr("DELETE_DATA", "&IDX", "&RFA4")
		p.module("IDX", "RFA5", "D's record.")
		p.insert("IDX", "D", "RFA5")
		p.header("IDX")
		p.closeLib("IDX")
	})
	p.kase("W3 read back", func() { p.readback("H90DIR:W3", "TTXT", ".TLB", []string{"A", "B", "C", "D"}) })

	// W4: the errors and edges of keys and records.
	p.kase("W4, keys' errors", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W4", "OPTT", ".TLB")
		p.module("IDX", "RFA1", "X's record.")
		p.insert("IDX", "X", "RFA1")
		p.insert("IDX", "X", "RFA1")
		p.insert("IDX", "x", "RFA1")
		p.insert("IDX", "Y", "BADRFA")
		p.insert("IDX", "Z", "ODDRFA")
		p.insert("IDX", "ZERO", "ZRFA")
		p.lbr("DELETE_KEY", "&IDX", "$NOSUCH")
		p.lbr("DELETE_DATA", "&IDX", "&RFA1")
		p.insert("IDX", "ALIAS", "RFA1")
		p.lbr("DELETE_KEY", "&IDX", "$X")
		p.lbr("DELETE_DATA", "&IDX", "&RFA1")
		p.lbr("REPLACE_KEY", "&IDX", "$NEWKEY", "&RFA1", "&RFA1")
		p.lbr("REPLACE_KEY", "&IDX", "$ALIAS", "&BADRFA", "&RFA1")
		p.lbr("REPLACE_KEY", "&IDX", "$ALIAS", "&RFA1", "&BADRFA")
		p.insert("IDX", "K234567890123456789012345678901", "RFA1")
		p.insert("IDX", "K2345678901234567890123456789012", "RFA1")
		p.insert("IDX", "", "RFA1")
		p.insert("IDX", "a b", "RFA1")
		p.lbr("SET_INDEX", "&IDX", "&TWO")
		p.lbr("SET_INDEX", "&IDX", "&ONE")
		p.lbr("PUT_END", "&IDX")
		p.module("IDX", "RFA2")
		p.insert("IDX", "NORECS", "RFA2")
		p.lbr("PUT_RECORD", "&IDX", "$", "&RFA3")
		p.showRFA("RFA3")
		p.lbr("PUT_RECORD", "&IDX", "&BIG2048", "&RFA3")
		p.lbr("PUT_RECORD", "&IDX", "&BIG2049", "&RFA3")
		p.lbr("PUT_RECORD", "&IDX", "&BIG2050", "&RFA3")
		p.lbr("PUT_END", "&IDX")
		p.insert("IDX", "LONGRECS", "RFA3")
		p.emit("CLRL\tCOUNT")
		p.lbr("SEARCH", "&IDX", "&ONE", "&RFA1", "&KEYR")
		p.emit("CLRL\tCOUNT")
		p.lbr("SEARCH", "&IDX", "&ONE", "&BADRFA", "&KEYR")
		p.closeLib("IDX")
	})
	p.kase("W4 read back", func() {
		p.readback("H90DIR:W4", "TTXT", ".TLB", []string{"x", "ALIAS", "NEWKEY", "NORECS", "Y", "ZERO"}, "LONGRECS")
	})
	p.kase("W4: inserting and deleting from GET_INDEX's routine", func() {
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:W4", "", ".TLB")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX", "&ONE", "&KEYINS")
		p.emit("CLRL\tCOUNT")
		p.lbr("LOOKUP_KEY", "&IDX", "$ALIAS", "&RFA1")
		p.lbr("SEARCH", "&IDX", "&ONE", "&RFA1", "&KEYINS")
		p.closeLib("IDX")
	})

	// Writing where it can't be done.
	p.kase("writing a library opened to read", func() {
		p.ini("IDX", "FREAD", "TTXT")
		p.open("IDX", "H90DIR:W2", "", ".TLB")
		p.lbr("PUT_RECORD", "&IDX", "$A record.", "&RFA1")
		p.lbr("PUT_END", "&IDX")
		p.insert("IDX", "NEW", "RFA1")
		p.lbr("LOOKUP_KEY", "&IDX", "$ONE", "&RFA2")
		p.lbr("REPLACE_KEY", "&IDX", "$ONE", "&RFA2", "&RFA2")
		p.lbr("DELETE_KEY", "&IDX", "$ONE")
		p.lbr("DELETE_DATA", "&IDX", "&RFA2")
		p.lbr("PUT_HISTORY", "&IDX", "$A history record.")
		p.lbr("FLUSH", "&IDX", "#LBR$C_FLUSHALL")
		p.closeLib("IDX")
	})
	p.kase("writing through an index never set, and a closed one", func() {
		p.lbr("PUT_RECORD", "&NEVER", "$A record.", "&RFA1")
		p.lbr("INSERT_KEY", "&NEVER", "$NEW", "&RFA1")
		p.lbr("PUT_RECORD", "&IDX", "$A record.", "&RFA1")
		p.lbr("PUT_END", "&IDX")
		p.lbr("DELETE_KEY", "&IDX", "$ONE")
		p.lbr("PUT_HISTORY", "&IDX", "$A history record.")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
	})

	// History records.
	p.kase("W5, history records past LUHMAX 3", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W5", "OPTH3", ".TLB")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		for _, h := range []string{"First history record.", "Second history record.", "Third.", "Fourth.", "Fifth, which leaves three."} {
			p.lbr("PUT_HISTORY", "&IDX", "$"+h)
		}
		p.lbr("PUT_HISTORY", "&IDX", "$")
		p.lbr("PUT_HISTORY", "&IDX", "&BIG2049")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.module("IDX", "RFA1", "A module beside the history.")
		p.insert("IDX", "M", "RFA1")
		p.header("IDX")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "TTXT")
		p.open("IDX", "H90DIR:W5", "", ".TLB")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.emit("CLRL\tCOUNT")
		p.emit("MOVL\t#1,LIMIT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.emit("CLRL\tLIMIT")
		p.closeLib("IDX")
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:W5", "", ".TLB")
		p.lbr("PUT_HISTORY", "&IDX", "$Sixth, after reopening.")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.header("IDX")
		p.closeLib("IDX")
	})
	p.kase("W6, LUHMAX 0", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W6", "OPTH0", ".TLB")
		p.lbr("PUT_HISTORY", "&IDX", "$A history record.")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.closeLib("IDX")
	})
	p.kase("H90T's history, made by LIBRARY", func() {
		p.ini("IDX", "FREAD", "TTXT")
		p.open("IDX", "H90DIR:H90T", "", ".TLB")
		p.header("IDX")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_HISTORY", "&IDX", "&HISTR")
		p.lbr("LOOKUP_KEY", "&IDX", "$T1", "&RFA1")
		p.modhdr("IDX", "RFA1")
		p.closeLib("IDX")
	})

	// Module headers.
	p.kase("W7, module headers with UHDMAX 8", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W7", "OPTU", ".TLB")
		p.module("IDX", "RFA1", "M1's record.")
		p.insert("IDX", "M1", "RFA1")
		p.modhdr("IDX", "RFA1")
		p.lbr("SET_MODULE", "&IDX", "&RFA1", "0", "0", "$USERDATA+")
		p.modhdr("IDX", "RFA1")
		p.lbr("SET_MODULE", "&IDX", "&RFA1", "0", "0", "$UD")
		p.modhdr("IDX", "RFA1")
		p.emit("CLRL\tMHLEN")
		p.lbr("SET_MODULE", "&IDX", "&RFA1", "&MHBS", "&MHLEN")
		p.emit("PUSHL\tMHLEN")
		p.emit("PUSHAQ\t%s", p.str("buflen"))
		p.emit("CALLS\t#2,VALUE")
		p.lbr("SET_MODULE", "&IDX", "&BADRFA", "&MHBD", "&MHLEN")
		p.lbr("SET_MODULE", "&IDX", "&RFA1")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "TTXT")
		p.open("IDX", "H90DIR:W7", "", ".TLB")
		p.lbr("LOOKUP_KEY", "&IDX", "$M1", "&RFA2")
		p.modhdr("IDX", "RFA2")
		p.lbr("SET_MODULE", "&IDX", "&RFA2", "0", "0", "$READONLY")
		p.closeLib("IDX")
	})

	// FLUSH.
	p.kase("W8, FLUSH", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W8", "OPTT", ".TLB")
		p.module("IDX", "RFA1", "F's record.")
		p.lbr("FLUSH", "&IDX", "#LBR$C_FLUSHDATA")
		p.insert("IDX", "F", "RFA1")
		p.lbr("FLUSH", "&IDX", "#LBR$C_FLUSHALL")
		p.lbr("FLUSH", "&IDX", "#99")
		p.records("IDX", "F", false)
		p.closeLib("IDX")
	})

	// W9: many keys in a scrambled order, and a long module.
	p.kase("W9, 126 modules and a long one", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W9", "OPTS", ".TLB")
		p.emit("PUSHL\t#126")
		p.emit("PUSHAL\tIDX")
		p.emit("CALLS\t#2,MANY")
		p.emit("PUSHAL\tRFA1")
		p.emit("PUSHL\t#300")
		p.emit("PUSHAL\tIDX")
		p.emit("CALLS\t#3,PUTN")
		p.lbr("PUT_END", "&IDX")
		p.insert("IDX", "LONG", "RFA1")
		p.header("IDX")
		p.closeLib("IDX")
	})
	p.kase("W9 read back", func() {
		p.readback("H90DIR:W9", "TTXT", ".TLB", []string{"K001", "K064", "K126"}, "LONG")
	})

	// W10: a help library, the same as LIBRARY/HELP's W10L from W10.HLP.
	p.kase("W10, a help library", func() {
		p.ini("IDX", "FCRE", "THLP")
		p.open("IDX", "H90DIR:W10", "OPTH", ".HLB")
		p.module("IDX", "RFA1", "1 WALPHA", "   Text of WALPHA.", "2 Sub", "   Text of WALPHA Sub.")
		p.insert("IDX", "WALPHA", "RFA1")
		p.module("IDX", "RFA2", "1 walpha2", "   Text of walpha2.")
		p.insert("IDX", "walpha2", "RFA2")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "THLP")
		p.open("IDX", "H90DIR:W10", "", ".HLB")
		p.index("IDX")
		p.lbr("GET_HELP", "&IDX", "0", "&HELPR", "0", "$WALPHA", "$Sub")
		p.lbr("GET_HELP", "&IDX", "0", "&HELPR", "0", "$WALPHA2")
		p.closeLib("IDX")
	})

	// W11: a macro library.
	p.kase("W11, a macro library", func() {
		p.ini("IDX", "FCRE", "TMLB")
		p.open("IDX", "H90DIR:W11", "OPTM", ".MLB")
		p.module("IDX", "RFA1", "\t.MACRO\tWMAC\tA", "\t.LONG\tA", "\t.ENDM\tWMAC")
		p.insert("IDX", "WMAC", "RFA1")
		p.closeLib("IDX")
		p.readback("H90DIR:W11", "TMLB", ".MLB", []string{"WMAC"})
	})

	// W12: an object library, two indexes, the module from H90O.OBJ.
	p.kase("W12, an object library", func() {
		p.ini("IDX", "FCRE", "TOBJ")
		p.open("IDX", "H90DIR:W12", "OPTO", ".OLB")
		p.emit("PUSHAL\tRFA1")
		p.emit("PUSHAL\tIDX")
		p.emit("CALLS\t#2,PUTOBJ")
		p.lbr("PUT_END", "&IDX")
		p.showRFA("RFA1")
		p.insert("IDX", "H90O", "RFA1")
		p.lbr("SET_INDEX", "&IDX", "&TWO")
		p.insert("IDX", "H90O_SYM", "RFA1")
		p.insert("IDX", "H90O_TWO", "RFA1")
		p.emit("CLRL\tCOUNT")
		p.lbr("SEARCH", "&IDX", "&TWO", "&RFA1", "&KEYR")
		p.lbr("SET_INDEX", "&IDX", "&THREE")
		p.modhdr("IDX", "RFA1")
		p.header("IDX")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "TOBJ")
		p.open("IDX", "H90DIR:W12", "", ".OLB")
		p.index("IDX")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX", "&TWO", "&KEYR")
		p.records("IDX", "H90O", true)
		p.closeLib("IDX")
	})

	// W13: binary keys, in a library of a user's type.
	p.kase("W13, binary keys", func() {
		p.ini("IDX", "FCRE", "TUSR")
		p.open("IDX", "H90DIR:W13", "OPTB", ".ULB")
		for _, k := range []string{"5", "1", "3"} {
			p.module("IDX", "RFA"+k, "Record of binary key "+k+".")
			p.lbr("INSERT_KEY", "&IDX", "&BK"+k, "&RFA"+k)
		}
		p.lbr("INSERT_KEY", "&IDX", "&BK3", "&RFA3")
		p.lbr("LOOKUP_KEY", "&IDX", "&BK3", "&RFAX")
		p.emit("PUSHAL\tIDX")
		p.emit("CALLS\t#1,RECSI")
		p.lbr("DELETE_KEY", "&IDX", "&BK1")
		p.header("IDX")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "TUSR")
		p.open("IDX", "H90DIR:W13", "", ".ULB")
		p.emit("CLRL\tCOUNT")
		p.lbr("SEARCH", "&IDX", "&ONE", "&RFA5", "&KEYB")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX", "&ONE", "&KEYB")
		p.closeLib("IDX")
	})

	// Create options.
	p.kase("W14, VMS V2 format", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W14", "OPTV2", ".TLB")
		p.module("IDX", "RFA1", "V2's record.")
		p.insert("IDX", "V2", "RFA1")
		p.closeLib("IDX")
		p.readback("H90DIR:W14", "TTXT", ".TLB", []string{"V2"})
	})
	p.kase("W15, options all 0 but the type", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W15", "OPTZ", ".TLB")
		p.module("IDX", "RFA1", "Z's record.")
		p.insert("IDX", "Z", "RFA1")
		p.header("IDX")
		p.closeLib("IDX")
		p.readback("H90DIR:W15", "TTXT", ".TLB", []string{"Z"})
	})
	p.kase("create errors", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:WE1", "", ".TLB")
		p.closeLib("IDX")
		p.ini("IDX", "FCRE", "THLP")
		p.open("IDX", "H90DIR:WE2", "OPTT", ".TLB")
		p.closeLib("IDX")
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:WE3", "OPTX9", ".TLB")
		p.closeLib("IDX")
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:WE4", "OPTK", ".TLB")
		p.closeLib("IDX")
		p.lbr("INI_CONTROL", "&IDX", "&FBAD", "&TTXT")
		p.lbr("INI_CONTROL", "&IDX", "&FCRE", "&TBAD")
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:NOSUCH", "", ".TLB")
		p.emit("CALLS\t#0,G^LBR$RET_RMSSTV")
		p.stat("RET_RMSSTV")
		p.closeLib("IDX")
	})
	p.kase("W1 created again, and W2 opened twice to update", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W1", "OPTT", ".TLB")
		p.closeLib("IDX")
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:W2", "", ".TLB")
		p.ini("IDX2", "FUPD", "TTXT")
		p.open("IDX2", "H90DIR:W2", "", ".TLB")
		p.emit("CALLS\t#0,G^LBR$RET_RMSSTV")
		p.stat("RET_RMSSTV")
		p.closeLib("IDX2")
		p.closeLib("IDX")
	})
	p.kase("W16, records with no PUT_END or key", func() {
		p.ini("IDX", "FCRE", "TTXT")
		p.open("IDX", "H90DIR:W16", "OPTT", ".TLB")
		p.module("IDX", "RFA1", "Keyless's record.")
		p.lbr("PUT_RECORD", "&IDX", "$Unended's first record.", "&RFA2")
		p.lbr("PUT_RECORD", "&IDX", "$Unended's second record.", "&RFA2")
		p.header("IDX")
		p.closeLib("IDX")
		p.readback("H90DIR:W16", "TTXT", ".TLB", nil)
	})

	// Libraries LIBRARY made, changed by the routines.
	p.kase("H90T updated: T3 added, T1 replaced", func() {
		p.ini("IDX", "FUPD", "TTXT")
		p.open("IDX", "H90DIR:H90T", "", ".TLB")
		p.module("IDX", "RFA1", "Only line of T3, by the routines.")
		p.insert("IDX", "T3", "RFA1")
		p.lbr("LOOKUP_KEY", "&IDX", "$T1", "&RFA2")
		p.module("IDX", "RFA3", "T1's new line.")
		p.lbr("REPLACE_KEY", "&IDX", "$T1", "&RFA2", "&RFA3")
		p.lbr("DELETE_DATA", "&IDX", "&RFA2")
		p.lbr("PUT_HISTORY", "&IDX", "$A history record by LBR90W.")
		p.closeLib("IDX")
		p.readback("H90DIR:H90T", "TTXT", ".TLB", []string{"T1", "T2", "T3"})
	})
	p.kase("H90S, a shareable image library LIBRARY made", func() {
		p.ini("IDX", "FREAD", "TSHR")
		p.open("IDX", "H90DIR:H90S", "", ".OLB")
		p.header("IDX")
		p.index("IDX")
		p.emit("CLRL\tCOUNT")
		p.lbr("GET_INDEX", "&IDX", "&TWO", "&KEYR")
		p.lbr("LOOKUP_KEY", "&IDX", "$H90OS", "&RFA1")
		p.modhdr("IDX", "RFA1")
		p.emit("PUSHAL\tIDX")
		p.emit("CALLS\t#1,RECN")
		p.closeLib("IDX")
	})

	p.kase("the NAM block, a related name, and the resultant name", func() {
		p.emit("CLRL\tRNSL")
		p.lbr("INI_CONTROL", "&IDX", "&FREAD", "&TTXT", "&ONAM")
		p.lbr("OPEN", "&IDX", "$H90DIR:W2", "0", "$.TLB", "0", "&RNSD", "&RNSL")
		p.emit("CALLS\t#0,SHOWNAM")
		p.index("IDX")
		p.closeLib("IDX")
		p.emit("CLRL\tRNSL")
		p.lbr("INI_CONTROL", "&IDX", "&FREAD", "&TTXT", "&ONAM")
		p.lbr("OPEN", "&IDX")
		p.emit("CALLS\t#0,SHOWNAM")
		p.index("IDX")
		p.closeLib("IDX")
		p.emit("CLRL\tRNSL")
		p.ini("IDX", "FREAD", "TTXT")
		p.lbr("OPEN", "&IDX", "$H90DIR:", "0", "0", "&ONAM", "&RNSD", "&RNSL")
		p.emit("CALLS\t#0,SHOWNAM")
		p.index("IDX")
		p.closeLib("IDX")
		p.emit("CLRL\tRNSL")
		p.lbr("INI_CONTROL", "&IDX", "&FREAD", "&TTXT", "&ONAM")
		p.lbr("OPEN", "&IDX", "$H90DIR:NOSUCH", "0", "$.TLB", "0", "&RNSD", "&RNSL")
		p.emit("CALLS\t#0,SHOWNAM")
		p.closeLib("IDX")
		p.ini("IDX", "FREAD", "TTXT")
		p.lbr("OPEN", "&IDX")
		p.closeLib("IDX")
	})

	// Last, since it leaves control indexes in use: too many of them.
	p.kase("17 control indexes", func() {
		p.emit("CALLS\t#0,SEVENTEEN")
	})

	var b strings.Builder
	b.WriteString(`	.TITLE	LBR90W - Phase 90's round 3: the LBR$ writing routines
; Written by testdata/help90/gen.go.
	$SSDEF
	$RMSDEF
	$RABDEF
	$NAMDEF
	$LBRDEF
	$HLPDEF
	$CREDEF
	$MHDDEF
	$LHIDEF
	.PSECT	D,NOEXE,WRT,LONG
IDX:	.LONG	0
IDX2:	.LONG	0
IDX3:	.LONG	0
NEVER:	.LONG	12345
MANYX:	.BLKL	17
FCRE:	.LONG	LBR$C_CREATE
FREAD:	.LONG	LBR$C_READ
FUPD:	.LONG	LBR$C_UPDATE
FBAD:	.LONG	99
TTXT:	.LONG	LBR$C_TYP_TXT
THLP:	.LONG	LBR$C_TYP_HLP
TMLB:	.LONG	LBR$C_TYP_MLB
TOBJ:	.LONG	LBR$C_TYP_OBJ
TSHR:	.LONG	LBR$C_TYP_SHSTB
TUSR:	.LONG	LBR$C_TYP_USRLW
TBAD:	.LONG	99
ZERO:	.LONG	0
ONE:	.LONG	1
TWO:	.LONG	2
THREE:	.LONG	3
BK1:	.LONG	1
BK3:	.LONG	3
BK5:	.LONG	5
RFA1:	.LONG	0,0
RFA2:	.LONG	0,0
RFA3:	.LONG	0,0
RFA4:	.LONG	0,0
RFA5:	.LONG	0,0
RFAX:	.LONG	0,0
ZRFA:	.LONG	0,0
BADRFA:	.LONG	9999,0
ODDRFA:	.LONG	1,3
HDR:	.BLKL	128
COUNT:	.LONG	0
LIMIT:	.LONG	0
LASTP:	.LONG	0
LASTE:	.LONG	0
LASTI:	.LONG	0
MHLEN:	.LONG	0
MHB:	.BLKB	512
MHBD:	.WORD	512
	.BYTE	14,1
	.ADDRESS MHB
MHBS:	.WORD	4
	.BYTE	14,1
	.ADDRESS MHB
BIG:	.BLKB	3000
BIG2048: .WORD	2048
	.BYTE	14,1
	.ADDRESS BIG
BIG2049: .WORD	2049
	.BYTE	14,1
	.ADDRESS BIG
BIG2050: .WORD	2050
	.BYTE	14,1
	.ADDRESS BIG
BIGIN:	.BLKB	3000
BIGIND:	.WORD	3000
	.BYTE	14,1
	.ADDRESS BIGIN
KEYBF:	.BLKB	16
KEYD:	.WORD	0
	.BYTE	14,1
	.ADDRESS KEYBF
KEYBD:	.WORD	16
	.BYTE	14,1
	.ADDRESS KEYBF
RECBF:	.BLKB	128
RECD:	.WORD	0
	.BYTE	14,1
	.ADDRESS RECBF
RECBD:	.WORD	128
	.BYTE	14,1
	.ADDRESS RECBF
INB:	.BLKB	512
INBD:	.WORD	512
	.BYTE	14,1
	.ADDRESS INB
OUTBD:	.WORD	0
	.BYTE	14,1
	.LONG	0
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
	.ALIGN	LONG
OFAB:	$FAB	FNM=<H90DIR:H90O.OBJ>
	.ALIGN	LONG
ORAB:	$RAB	FAB=OFAB,UBF=OBUF,USZ=2048
OBUF:	.BLKB	2048
	.ALIGN	LONG
ONAM:	$NAM	RSA=NRS,RSS=128,ESA=NES,ESS=128
NRS:	.BLKB	128
NES:	.BLKB	128
RNSB:	.BLKB	128
RNSD:	.WORD	128
	.BYTE	14,1
	.ADDRESS RNSB
RNSL:	.LONG	0
ORD:	.WORD	0
	.BYTE	14,1
	.ADDRESS OBUF
MCASE:	.ASCID	~#!UL !AS~
MVAL:	.ASCID	~  !AS = !XL~
MSTAT:	.ASCID	~  !AS: r=!XL !AD~
MRFA:	.ASCID	~  rfa=!UL,!UW~
MREC:	.ASCID	~  rec !UL n=!UW |!AS|~
MRECS:	.ASCID	~  records end: r=!XL~
MRECN:	.ASCID	~  records n=!UL bytes=!UL end: r=!XL~
MKEY:	.ASCID	~  key |!AS| rfa=!UL,!UW~
MBKEY:	.ASCID	~  bkey !XL a=!UL rfa=!UL,!UW~
MHLP:	.ASCID	~  h f=!XL lv=!XL la=!UL d=!UL |!AS|~
MHIST:	.ASCID	~  hist n=!UL |!AF|~
MHDR:	.ASCID	~  !3UL:!8(9XL)~
MDUMP:	.ASCID	~  !XW:!#(3XB)~
MMOD:	.ASCID	~  module header len=!UL~
MMANY:	.ASCID	~  many n=!UL put bad=!UL r=!XL end bad=!UL r=!XL insert bad=!UL r=!XL~
MPUTN:	.ASCID	~  putn n=!UL bad=!UL r=!XL~
MOBJ:	.ASCID	~  object records=!UL~
MKN:	.ASCID	~K!3ZL~
MRN:	.ASCID	~Record of K!3ZL.~
MLN:	.ASCID	~L!4ZL !74*.~
MNAM:	.ASCID	~  rns n=!UL |!AD| nam rsl=!UL |!AD| fid=!UW,!UW,!UW~
MSEV:	.ASCID	~  index !UL: r=!XL~
MCOND:	.ASCID	~  =c !XL n=!UL a=!XL !XL~
MUNW:	.ASCID	~  =u~
NINS:	.ASCID	~INSERT_KEY in a routine~
NDEL:	.ASCID	~DELETE_KEY in a routine~
NSET:	.ASCID	~SET_MODULE~
NOBJ:	.ASCID	~object file~
NPUT:	.ASCID	~PUT_RECORD~
NHDR:	.ASCID	~GET_HEADER~
INTRAV:	.ASCID	~INTRAV~
`)
	b.WriteString(p.data.String())
	b.WriteString(lbr90wCode)
	b.WriteString(p.code.String())
	b.WriteString("\t.ENTRY\tSTART,^M<>\n")
	b.WriteString("\tMOVC5\t#0,BIG,#^A/B/,#3000,BIG\n")
	for _, c := range p.cases {
		fmt.Fprintf(&b, "\tCALLS\t#0,%s\n", c)
	}
	b.WriteString("\tMOVL\t#1,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// lbr90wCode is LBR90W's own routines; OUT to LIMITED are LBR90's.
const lbr90wCode = `	.PSECT	C,EXE,NOWRT,LONG
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
; RECSI(index) shows the index's records from the current one.
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
; RECN(index) counts the index's records from the current one, and their
; bytes.
	.ENTRY	RECN,^M<R2,R3,R4>
	CLRL	R3
	CLRL	R4
10$:	CLRW	OUTBD
	CLRL	OUTBD+4
	PUSHAQ	OUTBD
	PUSHAQ	BIGIND
	PUSHL	4(AP)
	CALLS	#3,G^LBR$GET_RECORD
	MOVL	R0,R2
	BLBC	R2,20$
	INCL	R3
	MOVZWL	OUTBD,R0
	ADDL2	R0,R4
	CMPL	R3,#5000
	BLSS	10$
20$:	PUSHL	R2
	PUSHL	R4
	PUSHL	R3
	PUSHAB	MRECN
	CALLS	#4,OUT
	RET
; HEADERI(index) shows LBR$GET_HEADER's 128 longwords.
	.ENTRY	HEADERI,^M<R2,R3>
	PUSHAL	HDR
	PUSHL	4(AP)
	CALLS	#2,G^LBR$GET_HEADER
	PUSHL	R0
	PUSHAQ	NHDR
	CALLS	#2,STAT
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
; DUMPB(address, length) shows bytes, 16 to a line, with their offsets.
	.ENTRY	DUMPB,^M<R2,R3,R4,R5>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
	CLRL	R5
10$:	TSTL	R3
	BLEQ	40$
	MOVL	#16,R4
	CMPL	R3,R4
	BGEQ	20$
	MOVL	R3,R4
20$:	MOVL	R4,R1
30$:	DECL	R1
	MOVZBL	(R2)[R1],-(SP)
	TSTL	R1
	BGTR	30$
	PUSHL	R4
	PUSHL	R5
	PUSHAB	MDUMP
	ADDL3	#3,R4,R0
	CALLS	R0,OUT
	ADDL2	R4,R2
	ADDL2	R4,R5
	SUBL2	R4,R3
	BRB	10$
40$:	RET
; MODHDR(index, rfa) shows a module's header, read by LBR$SET_MODULE
; into a buffer filled with EE first.
	.ENTRY	MODHDR,^M<R2,R3,R4,R5>
	MOVC5	#0,MHB,#^XEE,#512,MHB
	CLRL	MHLEN
	PUSHAL	MHLEN
	PUSHAQ	MHBD
	PUSHL	8(AP)
	PUSHL	4(AP)
	CALLS	#4,G^LBR$SET_MODULE
	PUSHL	R0
	PUSHAQ	NSET
	CALLS	#2,STAT
	PUSHL	MHLEN
	PUSHAB	MMOD
	CALLS	#2,OUT
	MOVL	MHLEN,R0
	CMPL	R0,#512
	BLEQU	10$
	MOVL	#64,R0
10$:	PUSHL	R0
	PUSHAB	MHB
	CALLS	#2,DUMPB
	RET
; KEYR(key, rfa), GET_INDEX's and SEARCH's routine: shows the key, and
; fails once COUNT reaches a nonzero LIMIT.
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
; KEYB(key, rfa), the routine for binary keys: the key argument shown as
; the longword it addresses when it looks like an address (a=1), else as
; it is (a=0).
	.ENTRY	KEYB,^M<R2,R3>
	INCL	COUNT
	MOVL	4(AP),R2
	CLRL	R3
	CMPL	R2,#^X1000
	BLSSU	10$
	MOVL	(R2),R2
	MOVL	#1,R3
10$:	MOVL	8(AP),R0
	MOVZWL	4(R0),-(SP)
	PUSHL	(R0)
	PUSHL	R3
	PUSHL	R2
	PUSHAB	MBKEY
	CALLS	#5,OUT
	BRW	LIMITED
; KEYINS(key, rfa): inserts and deletes a key from inside GET_INDEX or
; SEARCH, then stops it.
	.ENTRY	KEYINS,^M<>
	INCL	COUNT
	PUSHL	8(AP)
	PUSHAQ	INTRAV
	PUSHAL	IDX
	CALLS	#3,G^LBR$INSERT_KEY
	PUSHL	R0
	PUSHAQ	NINS
	CALLS	#2,STAT
	PUSHL	4(AP)
	PUSHAL	IDX
	CALLS	#2,G^LBR$DELETE_KEY
	PUSHL	R0
	PUSHAQ	NDEL
	CALLS	#2,STAT
	CLRL	R0
	RET
; HELPR(line, flags, data, level), GET_HELP's routine, as LBR90's.
	.ENTRY	HELPR,^M<R2,R3>
	INCL	COUNT
	MOVL	16(AP),R2
	CLRL	R3
	CMPL	R2,#^X1000
	BLSSU	10$
	MOVL	(R2),R2
	MOVL	#1,R3
10$:	PUSHL	4(AP)
	PUSHL	#0
	PUSHL	R3
	PUSHL	R2
	PUSHL	@8(AP)
	PUSHAB	MHLP
	CALLS	#6,OUT
	BRW	LIMITED
; HISTR(record), GET_HISTORY's routine: the record as text and in hex.
	.ENTRY	HISTR,^M<R2>
	INCL	COUNT
	MOVL	4(AP),R2
	PUSHL	4(R2)
	MOVZWL	(R2),-(SP)
	MOVZWL	(R2),-(SP)
	PUSHAB	MHIST
	CALLS	#4,OUT
	MOVZWL	(R2),-(SP)
	PUSHL	4(R2)
	CALLS	#2,DUMPB
	BRW	LIMITED
; MANY(index, n) writes n modules of a record each, keyed Knnn in the
; order 37*i mod 127, and shows how many of each call failed.
	.ENTRY	MANY,^M<R2,R3,R4,R5,R6,R7,R8,R9>
	CLRL	R6
	CLRL	R7
	CLRL	R8
	CLRL	R9
10$:	INCL	R6
	CMPL	R6,8(AP)
	BLEQ	15$
	BRW	50$
15$:	MULL3	#37,R6,R2
	CLRL	R3
	EDIV	#127,R2,R4,R5
	$FAO_S	CTRSTR=MKN,OUTLEN=KEYD,OUTBUF=KEYBD,P1=R5
	$FAO_S	CTRSTR=MRN,OUTLEN=RECD,OUTBUF=RECBD,P1=R5
	PUSHAL	RFAX
	PUSHAQ	RECD
	PUSHL	4(AP)
	CALLS	#3,G^LBR$PUT_RECORD
	MOVL	R0,LASTP
	BLBS	R0,20$
	INCL	R7
20$:	PUSHL	4(AP)
	CALLS	#1,G^LBR$PUT_END
	MOVL	R0,LASTE
	BLBS	R0,30$
	INCL	R8
30$:	PUSHAL	RFAX
	PUSHAQ	KEYD
	PUSHL	4(AP)
	CALLS	#3,G^LBR$INSERT_KEY
	MOVL	R0,LASTI
	BLBS	R0,40$
	INCL	R9
40$:	BRW	10$
50$:	PUSHL	LASTI
	PUSHL	R9
	PUSHL	LASTE
	PUSHL	R8
	PUSHL	LASTP
	PUSHL	R7
	PUSHL	8(AP)
	PUSHAB	MMANY
	CALLS	#8,OUT
	RET
; PUTN(index, n, rfa) writes n records of 80 bytes, Lnnnn and dots.
	.ENTRY	PUTN,^M<R2,R3,R4,R5,R6>
	CLRL	R6
	CLRL	R3
10$:	INCL	R6
	CMPL	R6,8(AP)
	BGTR	30$
	$FAO_S	CTRSTR=MLN,OUTLEN=RECD,OUTBUF=RECBD,P1=R6
	PUSHL	12(AP)
	PUSHAQ	RECD
	PUSHL	4(AP)
	CALLS	#3,G^LBR$PUT_RECORD
	MOVL	R0,LASTP
	BLBS	R0,10$
	INCL	R3
	BRB	10$
30$:	PUSHL	LASTP
	PUSHL	R3
	PUSHL	8(AP)
	PUSHAB	MPUTN
	CALLS	#4,OUT
	RET
; PUTOBJ(index, rfa) writes H90O.OBJ's records as a module's.
	.ENTRY	PUTOBJ,^M<R2,R3>
	CLRL	R3
	$OPEN	FAB=OFAB
	BLBC	R0,80$
	$CONNECT RAB=ORAB
	BLBC	R0,80$
10$:	$GET	RAB=ORAB
	BLBC	R0,30$
	INCL	R3
	MOVW	ORAB+RAB$W_RSZ,ORD
	PUSHL	8(AP)
	PUSHAQ	ORD
	PUSHL	4(AP)
	CALLS	#3,G^LBR$PUT_RECORD
	BLBS	R0,10$
	PUSHL	R0
	PUSHAQ	NPUT
	CALLS	#2,STAT
	BRB	10$
30$:	$CLOSE	FAB=OFAB
	PUSHL	R3
	PUSHAB	MOBJ
	CALLS	#2,OUT
	RET
80$:	PUSHL	R0
	PUSHAQ	NOBJ
	CALLS	#2,STAT
	RET
; SHOWNAM shows OPEN's resultant name and the NAM block's.
	.ENTRY	SHOWNAM,^M<R2>
	MOVL	RNSL,R2
	CMPL	R2,#128
	BLEQU	10$
	MOVL	#128,R2
10$:	MOVZWL	ONAM+NAM$W_FID+4,-(SP)
	MOVZWL	ONAM+NAM$W_FID+2,-(SP)
	MOVZWL	ONAM+NAM$W_FID,-(SP)
	PUSHAB	NRS
	MOVZBL	ONAM+NAM$B_RSL,-(SP)
	MOVZBL	ONAM+NAM$B_RSL,-(SP)
	PUSHAB	RNSB
	PUSHL	R2
	PUSHL	RNSL
	PUSHAB	MNAM
	CALLS	#10,OUT
	RET
; SEVENTEEN initializes 17 control indexes, showing each status.
	.ENTRY	SEVENTEEN,^M<R2,R3>
	CLRL	R2
	MOVAL	MANYX,R3
10$:	INCL	R2
	PUSHAL	TTXT
	PUSHAL	FREAD
	PUSHL	R3
	CALLS	#3,G^LBR$INI_CONTROL
	PUSHL	R0
	PUSHL	R2
	PUSHAB	MSEV
	CALLS	#3,OUT
	ADDL2	#4,R3
	CMPL	R2,#17
	BLSS	10$
	RET
`

func help90w() string {
	var b strings.Builder
	b.WriteString(`$ ! HELP90W.COM - round 3 of the Phase 90 probe: the LBR$ writing routines
$ ! on every kind of library (LBR90W.MAR), the libraries they make as
$ ! LIBRARY sees them, and what round 1 left of LIBRARY/HELP.
$ ! Written by testdata/help90/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @HELP90W/OUTPUT=HELP90W.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ !
$ ! Part A: what LBR90W reads, made by LIBRARY, MACRO, and LINK.
$ !
$ LIBRARY/CREATE/TEXT H90T T1,T2
$ LIBRARY/REPLACE/TEXT H90T T2
$ LIBRARY/LIST/FULL/HISTORY/TEXT H90T
$ MACRO H90O
$ MACRO H90OM
$ LINK/SHAREABLE=H90OS H90O,SYS$INPUT/OPTIONS
UNIVERSAL=H90O_SYM,H90O_TWO
$ LIBRARY/CREATE/SHARE H90S H90OS
$ LIBRARY/LIST/FULL/NAMES/SHARE H90S
$ MACRO LBR90W
$ LINK LBR90W
$ !
$ ! Part B: LBR90W.
$ !
$ RUN LBR90W
$ SHOW SYMBOL $STATUS
$ !
$ ! Part C: the libraries as LIBRARY sees them, and LIBRARY's own to
$ ! compare with them.
$ !
$ DIRECTORY/SIZE=ALL W*.*LB;*,H90T.TLB;*
`)
	for _, l := range []string{"W1", "W2", "W3", "W4", "W5", "W6", "W7", "W8", "W9", "W14", "W15", "W16", "H90T"} {
		fmt.Fprintf(&b, "$ LIBRARY/LIST/FULL/HISTORY/TEXT %s\n", l)
	}
	b.WriteString(`$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/TEXT W2
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/TEXT H90T
$ LIBRARY/CREATE/TEXT W2L ONE,TWO
$ LIBRARY/LIST/FULL/HISTORY/TEXT W2L
$ LIBRARY/LIST/FULL/HELP W10
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:W10 WALPHA...
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:W10 WALPHA2
$ LIBRARY/CREATE/HELP W10L W10
$ LIBRARY/LIST/FULL/HELP W10L
$ LIBRARY/LIST/FULL/MACRO W11
$ LIBRARY/EXTRACT=WMAC/OUTPUT=SYS$OUTPUT/MACRO W11
$ LIBRARY/LIST/FULL/NAMES/OBJECT W12
$ LIBRARY/CREATE/OBJECT W12L H90O
$ LIBRARY/LIST/FULL/NAMES/OBJECT W12L
$ LINK H90OM,W12/LIBRARY
$ RUN H90OM
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL W13.ULB
$ !
$ ! Part D: what round 1 left of LIBRARY/HELP, each with its status.
$ !
$ LIBRARY/CREATE/HELP WD1 H90B
$ SHOW SYMBOL $STATUS
$ LIBRARY/INSERT/HELP WD1 H90B
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE/HELP WD2 DIGITS
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE/HELP WD3 TOOLONG
$ SHOW SYMBOL $STATUS
$ LIBRARY/EXTRACT=Q*/OUTPUT=WD.HLP/HELP WD1
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE WD4.HLB WD1.HLB
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE/HELP WD5 MULTI
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL/HELP WD5
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP WD5
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:WD5 TWO
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:WD5 "TWO WORDS"
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:WD5 THREE WORD
$ LIBRARY/CREATE/HELP WD6 NOTFIRST
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL/HELP WD6
$ LIBRARY/EXTRACT=*/OUTPUT=SYS$OUTPUT/HELP WD6
$ LIBRARY/CREATE/HELP WD7 GOODBAD
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL/HELP WD7
$ LIBRARY/EXTRACT=NOSUCH/OUTPUT=WDT.TXT/TEXT H90T
$ SHOW SYMBOL $STATUS
$ LIBRARY/EXTRACT=Q*/OUTPUT=WDT.TXT/TEXT H90T
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE/HELP WD8 H90B
$ LIBRARY/DATA=REDUCE/HELP WD8
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL/HELP WD8
$ LIBRARY/DATA=EXPAND/HELP WD8
$ SHOW SYMBOL $STATUS
$ LIBRARY/LIST/FULL/HELP WD8
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:WD8 BONLY...
$ DIRECTORY/SIZE=ALL WD*.*;*
$ !
$ ! The libraries and listings, for the host.
$ !
$ BACKUP W*.*LB;*,H90T.TLB;*,H90S.OLB;*,WD*.*;* H90W.BCK/SAVE_SET/LOG
`)

	return b.String()
}

func exchange() string {
	var b strings.Builder
	b.WriteString(`! EXCHANGE.CMD - builds round 3's exchange volume with govax.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/help90w-exchange.dsk" /DEVICE=RD54 HELP90
MOUNT/WRITE DUA5 "testdata/disks/help90w-exchange.dsk"
`)
	for _, f := range round3Files {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}
	b.WriteString("DIRECTORY DUA5:[000000]\nDISMOUNT DUA5\n")

	return b.String()
}

// copyout brings everything back to a local directory first: the log and
// the save set go into vax/round3/ after the README's check.
func copyout() string {
	return `! COPYOUT.CMD - copies round 3's output off the exchange volume, into
! testdata/disks/help90-vms/round3/, which git ignores. README.md says how
! the log is checked before it goes into vax/.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90w-exchange.dsk"
COPY DUA5:[000000]HELP90W.LOG "testdata/disks/help90-vms/round3/help90w.log"/HOST
COPY DUA5:[000000]H90W.BCK "testdata/disks/help90-vms/round3/h90w.bck"/HOST/BINARY
DISMOUNT DUA5
`
}

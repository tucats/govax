# The Phase 90 probe: VMS help

These fixtures are `docs/PHASE-90 - VMS help.md`'s probe (subtask 2). They
ask VMS 7.1 how LIBRARY/HELP builds help libraries, how HELP finds and shows
topics and prompts for more, how the debugger's HELP finds its library, and
what the LBR$ routines do. Every question is asked of **help modules written
for the probe**, whose text names its own topic ("Text of ALPHA
Parameters."), so a log shows which text HELP chose. None of it is VMS's.

`gen.go` writes everything here but this README:

    go run testdata/help90/gen.go

| File | What it holds |
| ---- | ------------- |
| `h90a1.hlp`, `h90a2.hlp` | Library H90A's modules: ALPHA (parameters, a Qualifiers topic of `/` lines, a numbered `/DIRECT` key, a `/` line outside Qualifiers), ALPHABET and ALPS (abbreviations), HELP; DEEP (to level 9), WIDE (keys of 1 to 32 characters), MANY (40 keys), ORDER (out of order, mixed case), ODD (punctuation and blanks), SAME (repeated keys), TEXTY (empty, tabbed, long, and digit-led lines), NOSUB, EMPTY, MixedCase, BETA_GAMMA, QUALS (`/` lines under other topics, an indented `/`, `/[NO]FLAG`, a numbered key after `/` lines) |
| `alpha2.hlp` | A replacement ALPHA, for LIBRARY/REPLACE |
| `h90b.hlp`, `h90c.hlp`, `h90g.hlp`, `h90s.hlp` | The user libraries' modules (process, group, and system tables) |
| `h90d.hlp` | A debugger library of the probe's own: one DEBUG module, as VMS's DBG$HELP.HLB is, and an EXTRA module |
| `badtext.hlp`, `skip.hlp`, `noblank.hlp`, `toolong.hlp`, `lower.hlp` | Malformed modules: text before the first key, a skipped level, key lines with no blank, a bare digit or a tab, a level-1 key of 40 characters, a lowercase level-1 key |
| `t1.txt`, `t2.txt`, `h90m.mar` | A text library's and a macro library's modules, for the LBR$ routines |
| `resp.txt` | HELP's answers from a file (part D) |
| `help90.com` | Round 1, parts A (LIBRARY), B (HELP's matching and layout, 126 requests), B2 (output), C (user libraries), E (`LBR90`), then `HELP90D.COM` (part D), and a save set of the libraries and listings |
| `help90d.com` | Part D: HELP's prompts answered by a procedure's data lines, and by a file |
| `help90v.com` | Parts F (the debugger's HELP, with DBG$HELP naming the probe's library, then without it) and G (HELP on VMS's own EXCHANGE, SUBMIT, and ANALYZE modules). Its output holds VMS's text |
| `help90d.dbg`, `help90v.dbg`, `dbgt.mar` | The debugger sessions' commands, and the image they run |
| `lbr90.mar` | Part E: 79 cases calling the LBR$ routines on the probe's libraries: the definitions' values, INI_CONTROL, OPEN, GET_HEADER, LOOKUP_KEY, GET_RECORD (move and locate), FIND, GET_INDEX with matches, SET_INDEX, GET_HELP with its action routine's every call (flags, level, data), OUTPUT_HELP with output and input routines of its own (scripted answers to its prompts), the errors |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |

`lbr90.mar` was rehearsed on govax against stand-in LBR$ routines (not
kept): it assembles, links, runs all 79 cases, and its handler reports a
fault in a routine and goes on to the next case.

## Keeping VMS's text out of the repository

The phase doc's Decision 9 is a deliberately conservative rule: nothing
that may hold VMS's own help text goes into the repository. Logs of HELP
showing our modules hold only our text and HELP's framing (its headings and
messages), and those are committed. The rest stays local:

- `copyout.cmd` copies **everything** to `testdata/disks/help90-vms/`,
  which git ignores. Nothing goes straight into `vax/`.
- `help90.log` and `h90.bck` move to `vax/` only after a check that every
  line of HELP's output in them is one of our modules' lines, a command, or
  HELP's framing. If HELP took any text from VMS's own libraries (its
  instructions for `HELP` with no topic could come from SYS$HELP's
  HELPLIB.HLB rather than the probe's library), that case is cut from the
  committed log and noted here.
- `help90v.log` (part G is VMS's own modules) and `dbgv.dlg` (the
  debugger's own library) never leave `testdata/disks/help90-vms/`. Their
  tests skip without them, as DUNGEON's do. `dbgh.dlg` moves to `vax/` only
  if the check shows the debugger read the probe's library through
  DBG$HELP.

## The VAX run, round 1

Allow a few minutes. If a step waits at the terminal, it's part D's HELP
reading the terminal instead of the procedure: answer each prompt with
Ctrl/Z, and say so.

1. Build the exchange volume, from the repository root:

       govax console < testdata/help90/exchange.cmd

   This makes `testdata/disks/help90-exchange.dsk` (RD54 size, label
   HELP90).

2. On the VAX, mount it as DUA5, and run both procedures from its top
   directory. Part C defines HLP$LIBRARY in the group and system tables
   (and takes the names away again) only if neither table has an
   HLP$LIBRARY already; that needs GRPNAM and SYSNAM, as the SYSTEM account
   has.

       $ MOUNT DUA5: HELP90
       $ SET DEFAULT DUA5:[000000]
       $ @HELP90/OUTPUT=HELP90.LOG
       $ @HELP90V/OUTPUT=HELP90V.LOG
       $ DISMOUNT DUA5:

3. Back on the host:

       mkdir -p testdata/disks/help90-vms
       govax console < testdata/help90/copyout.cmd

   which brings `help90.log`, `h90.bck`, `help90v.log`, `dbgh.dlg`, and
   `dbgv.dlg` into `testdata/disks/help90-vms/`.

## Round 2, at the terminal

HELP's prompts typed by hand, which a procedure can't show (the prompts and
the terminal's own echo). Log the session on the host (simh's
`SET CONSOLE LOG=help90-term.log`, or the terminal program's log), with the
exchange volume mounted as in round 1 and the libraries round 1 made on it.
The log goes to `testdata/disks/help90-vms/help90-term.log`, and is checked
like `help90.log` before it goes to `vax/`.

    $ SET DEFAULT DUA5:[000000]
    $ DEFINE H90DIR DUA5:[000000]
    $ HP := HELP/NOUSERLIBRARY/LIBRARY=H90DIR:H90A
    $ SET TERMINAL/WIDTH=80

Then type, in order (Return means a Return on an empty line):

| # | Type | What it shows |
| - | ---- | ------------- |
| 1 | `HP` | The topic list and `Topic?` |
| 2 | `ALPHA` | ALPHA and `ALPHA Subtopic?` |
| 3 | `Parameters` | ALPHA Parameters, then the prompt it leaves you at |
| 4 | `?` | What `?` shows at that prompt |
| 5 | Return | Up one level |
| 6 | `Qualifiers` | Qualifiers and its prompt |
| 7 | `/BRIEF` | The qualifier, and the prompt after a topic with no subtopics |
| 8 | `/NOSUCH` | An unknown subtopic |
| 9 | Return, Return | Back to `Topic?` |
| 10 | `DEEP` then `L2`, `L3`, `L4` | Each level's prompt, with its keys |
| 11 | Return four times | Each level back up, and `Topic?` |
| 12 | `@H90DIR:H90B` | The other library's topics and its `Topic?` prompt |
| 13 | `BONLY` | A topic there |
| 14 | Return, Return | Back to the main library's `Topic?` |
| 15 | `NOSUCH` | "Not found", and the prompt after it |
| 16 | `ALPHA` then Ctrl/Z | Ctrl/Z at a subtopic prompt |
| 17 | `HP ALPHA` then Return | Starting from a topic, and Return at its prompt |
| 18 | `SET TERMINAL/WIDTH=132`, then `HP/NOPROMPT WIDE` and `HP/NOPROMPT MANY` | The subtopic columns at 132 |
| 19 | `SET TERMINAL/WIDTH=80`, then `HP/PAGE TEXTY...` | /PAGE's stop, if the screen fills; Return to go on |
| 20 | `SET TERMINAL/WIDTH=80`, `DEFINE DBG$HELP H90DIR:H90D`, `RUN/DEBUG DBGT`, then `HELP`, `HELP EXAMINE`, `HELP SET BREAK`, `EXIT` | The debugger's HELP at the terminal, on the probe's library: does it prompt? |
| 21 | `DEASSIGN DBG$HELP` | |

## What the run settles

The phase doc's A.4 lists the questions. Its "What the probe settled"
section, written when the logs come back, records the answers.

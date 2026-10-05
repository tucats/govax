!
!   DCLRTL grammar definition for the govax debugger (internal/debugger,
!   docs/PHASE-42.md). The debugger has its own command set, modeled on
!   the VMS debugger's, and its own grammar: the VMS command line verbs
!   (DIRECTORY, MACRO, SET DEFAULT, ...) are in console.dcl and aren't
!   available at the "DBG> " prompt.
!
!   Each later subtask of Phase 42 adds its commands here. Verb and
!   syntax ids start at 1, since this grammar is parsed apart from
!   console.dcl's.
!

grammar debugger

    ! EXIT ends the debugger session and returns to the console. QUIT
    ! ends it too; on VMS the difference is that EXIT runs the program's
    ! exit handlers and QUIT doesn't (govax keeps no exit handlers yet,
    ! so for now they are the same).
    verb exit/id=1

    verb quit/id=2

    ! HELP [topic ...] reads debug.help. A "/" starts a new word, as in
    ! DCL.
    verb help/id=3
        parameter   topic/id=4                  -
                    /type=$rest_of_line
    verb ?/alias=help

    ! @file runs the debugger commands in a command file.
    verb include/id=5
        parameter   file/id=6                   -
                    /type=$string               -
                    /prompt="File"
    verb @/alias=include

    ! GO [address] continues the program (from the address, if one is
    ! given). It runs until a breakpoint, the program's end, or Ctrl-C.
    verb go/id=7
        parameter   address/id=8                -
                    /type=$expression
    verb execute/alias=go
    verb g/alias=go

    ! CALL[/STEP] routine[(argument,...)] calls a routine under the
    ! debugger. The argument list may follow the routine after a blank,
    ! so it is a parameter of its own too.
    verb call/id=9
        qualifier   step/id=10/nonegatable
        qualifier   break/alias=step
        qualifier   debug/alias=step
        qualifier   dbg/alias=step
        parameter   routine/id=11               -
                    /type=$expression           -
                    /prompt="Routine"
        parameter   arguments/id=12             -
                    /type=$expression

    ! STEP[/mode] [address]: /INTO (also /IN, /INSTRUCTION), /OVER, or
    ! /RETURN, defaulting to SET STEP's mode.
    verb step/id=13
        qualifier   into/id=14/nonegatable
        qualifier   in/alias=into
        qualifier   instruction/alias=into
        qualifier   over/id=15/nonegatable
        qualifier   return/id=16/nonegatable
        parameter   address/id=17               -
                    /type=$expression
        disallow    any2(into, over, return)
    verb st/alias=step
    verb s/alias=step

    ! SET BREAK [qualifiers] [address[,address...]] [WHEN (cond)] [DO (cmds)]
    ! sets breakpoints. The parameter is the rest of the line, which
    ! internal/debugger takes apart: it holds a list of addresses and the
    ! WHEN and DO clauses. A qualifier picks a kind of breakpoint instead
    ! of an address; at most one may be given. /INSTRUCTION may name the
    ! opcodes (/INSTRUCTION=(MOVB,MOVC3)); with none, every instruction
    ! breaks. /FAULT=code is govax's own (a break when that exception is
    ! about to be delivered).
    type set_types
        keyword     break               /syntax=set_break/nonegatable
        keyword     breakpoint          /syntax=set_break/nonegatable

    verb set/id=20
        parameter   what/id=21                  -
                    /type=set_types             -
                    /prompt="What"

        syntax set_break/id=22
            qualifier   after/id=23             -
                        /type=$integer/nonegatable
            qualifier   temporary/id=24/nonegatable
            qualifier   call/id=25/nonegatable
            qualifier   branch/id=26/nonegatable
            qualifier   line/id=27/nonegatable
            qualifier   exception/id=28/nonegatable
            qualifier   return/id=29/nonegatable
            qualifier   instruction/id=30       -
                        /type=$string/list/nonegatable -
                        /default=""
            qualifier   fault/id=31             -
                        /type=$string/nonegatable
            parameter   target/id=32            -
                        /type=$rest_of_line

    ! SHOW BREAK lists the breakpoints.
    type show_types
        keyword     break               /syntax=show_break
        keyword     breakpoint          /syntax=show_break
        keyword     breakpoints         /syntax=show_break

    verb show/id=40
        parameter   what/id=41                  -
                    /type=show_types            -
                    /prompt="What"

        syntax show_break/id=42

    ! CANCEL BREAK [qualifier] [address[,address...]] removes breakpoints:
    ! at the addresses, of a kind (/CALL, /BRANCH, ...), or all of them
    ! (/ALL).
    type cancel_types
        keyword     break               /syntax=cancel_break
        keyword     breakpoint          /syntax=cancel_break

    verb cancel/id=50
        parameter   what/id=51                  -
                    /type=cancel_types          -
                    /prompt="What"

        syntax cancel_break/id=52
            qualifier   all/id=53/nonegatable
            qualifier   call/id=54/nonegatable
            qualifier   branch/id=55/nonegatable
            qualifier   line/id=56/nonegatable
            qualifier   exception/id=57/nonegatable
            qualifier   return/id=58/nonegatable
            qualifier   instruction/id=59       -
                        /type=$string/list/nonegatable -
                        /default=""
            qualifier   fault/id=60             -
                        /type=$string/nonegatable -
                        /default=""
            parameter   target/id=61            -
                        /type=$rest_of_line

end

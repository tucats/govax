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

end

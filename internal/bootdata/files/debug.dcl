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

    ! STEP[/qualifiers] [count] takes count steps (one if none is given).
    ! /LINE (the default) and /INSTRUCTION say how far a step goes;
    ! /OVER (the default), /INTO (/IN), and /RETURN say what to do about
    ! calls. /BRANCH and /CALL step to the next instruction of that class.
    ! /SILENT reports nothing, and /SOURCE shows the source line. SET STEP
    ! changes the defaults. Of /INTO, /OVER, and /RETURN the last one
    ! typed wins (internal/debugger reads their order from the line).
    verb step/id=13
        qualifier   instruction/id=14/nonegatable
        qualifier   line/id=15/nonegatable
        qualifier   into/id=16/nonegatable
        qualifier   in/alias=into
        qualifier   over/id=17/nonegatable
        qualifier   return/id=18/nonegatable
        qualifier   branch/id=19/nonegatable
        qualifier   call/id=62/nonegatable
        qualifier   silent/id=63
        qualifier   source/id=64
        parameter   count/id=65                 -
                    /type=$expression
        disallow    any2(instruction, line)
        disallow    any2(branch, call)
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
        keyword     step                /syntax=set_step/nonegatable
        keyword     source              /syntax=set_source/nonegatable
        keyword     mode                /syntax=set_mode/nonegatable
        keyword     radix               /syntax=set_radix/nonegatable
        keyword     psl                 /syntax=set_psl/nonegatable
        keyword     pte                 /syntax=set_pte/nonegatable
        keyword     page                /syntax=set_pte/nonegatable
        keyword     fault               /syntax=set_fault_history/nonegatable
        keyword     history             /syntax=set_fault_history/nonegatable
        keyword     vm                  /syntax=set_vm
        keyword     mapen               /syntax=set_vm
        keyword     base                /syntax=set_base/nonegatable

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

        ! SET STEP keyword[,keyword...]: LINE or INSTRUCTION; OVER, INTO
        ! (IN), or RETURN; SILENT or NOSILENT; SOURCE or NOSOURCE.
        syntax set_step/id=33
            parameter   words/id=34             -
                        /type=$rest_of_line     -
                        /prompt="Step type"

        ! SET SOURCE dir[,dir...]: the directories searched for source files.
        syntax set_source/id=70
            parameter   directories/id=71       -
                        /type=$rest_of_line     -
                        /prompt="Directories"

        ! SET MODE keyword[,keyword...]: [NO]SYMBOLIC, and [NO]OPERANDS
        ! with an optional =FULL or =BRIEF.
        syntax set_mode/id=80
            parameter   words/id=81             -
                        /type=$rest_of_line     -
                        /prompt="Mode"

        ! SET RADIX [/INPUT|/OUTPUT] DECIMAL|HEXADECIMAL|OCTAL|BINARY. With
        ! neither qualifier it sets both radixes.
        syntax set_radix/id=82
            qualifier   input/id=83/nonegatable
            qualifier   output/id=84/nonegatable
            parameter   radix/id=85             -
                        /type=$rest_of_line     -
                        /prompt="Radix"

        ! SET PSL field=value[,field=value...] changes fields of the
        ! processor status longword (govax's own).
        syntax set_psl/id=200
            parameter   fields/id=201           -
                        /type=$rest_of_line     -
                        /prompt="Fields"

        ! SET PTE address [TO address] field=value[,field=value...] changes
        ! page table entries (govax's own); the handler reads the changes.
        syntax set_pte/id=202
            parameter   changes/id=203          -
                        /type=$rest_of_line     -
                        /prompt="Address"

        syntax set_fault_history/id=204
            parameter   count/id=205            -
                        /type=$integer          -
                        /prompt="Count"

        ! SET VM and SET NOVM (or SET MAPEN) turn address translation on
        ! and off (govax's own).
        syntax set_vm/id=206

        syntax set_base/id=207
            parameter   address/id=208          -
                        /type=$rest_of_line     -
                        /prompt="Address"

    ! SHOW BREAK lists the breakpoints, and SHOW STEP the defaults STEP
    ! uses.
    type show_types
        keyword     break               /syntax=show_break
        keyword     breakpoint          /syntax=show_break
        keyword     breakpoints         /syntax=show_break
        keyword     step                /syntax=show_step
        keyword     source              /syntax=show_source
        keyword     registers           /syntax=show_reg
        keyword     reg                 /syntax=show_reg
        keyword     psl                 /syntax=show_psl
        keyword     cpu_status          /syntax=show_cpu
        keyword     clock               /syntax=show_clock
        keyword     base                /syntax=show_base
        keyword     memory              /syntax=show_memory
        keyword     vm                  /syntax=show_memory
        keyword     maps                /syntax=show_map
        keyword     tb                  /syntax=show_tb
        keyword     translation_buffer  /syntax=show_tb
        keyword     regions             /syntax=show_regions
        keyword     page                /syntax=show_page
        keyword     pte                 /syntax=show_page
        keyword     scb                 /syntax=show_scb
        keyword     shim                /syntax=show_shim
        keyword     exceptions          /syntax=show_fault
        keyword     faults              /syntax=show_fault
        keyword     calls               /syntax=show_calls
        keyword     call_frames         /syntax=show_calls
        keyword     stack               /syntax=show_stack
        keyword     sp                  /syntax=show_sp
        keyword     isp                 /syntax=show_isp
        keyword     ksp                 /syntax=show_ksp
        keyword     esp                 /syntax=show_esp
        keyword     ssp                 /syntax=show_ssp
        keyword     usp                 /syntax=show_usp
        keyword     mode                /syntax=show_mode
        keyword     radix               /syntax=show_radix
        keyword     r0
        keyword     r1
        keyword     r2
        keyword     r3
        keyword     r4
        keyword     r5
        keyword     r6
        keyword     r7
        keyword     r8
        keyword     r9
        keyword     r10
        keyword     r11
        keyword     r12
        keyword     r13
        keyword     r14
        keyword     r15
        keyword     ap
        keyword     fp
        keyword     pc
        keyword     p0br
        keyword     p0lr
        keyword     p1br
        keyword     p1lr
        keyword     sbr
        keyword     slr
        keyword     pcbb
        keyword     scbb
        keyword     ipl
        keyword     astlvl
        keyword     sirr
        keyword     sisr
        keyword     iccs
        keyword     nicr
        keyword     icr
        keyword     todr
        keyword     rxcs
        keyword     rxdb
        keyword     txcs
        keyword     txdb
        keyword     tbdr
        keyword     savisp
        keyword     savpc
        keyword     savpsl
        keyword     wcsa
        keyword     wcsb
        keyword     tbia
        keyword     tbis
        keyword     pmr
        keyword     sid
        keyword     tbchk

    verb show/id=40
        parameter   what/id=41                  -
                    /type=show_types            -
                    /prompt="What"

        syntax show_break/id=42

        syntax show_step/id=43

        syntax show_source/id=72

        ! The commands below show the machine's state; they are govax's own
        ! except SHOW CALLS, SHOW STACK, SHOW MODE, and SHOW RADIX, which
        ! are the VMS debugger's.
        syntax show_reg/id=210
        syntax show_psl/id=211
        syntax show_cpu/id=212
        syntax show_clock/id=213
        syntax show_base/id=214
        syntax show_memory/id=215
            qualifier   statistics/id=216
            qualifier   runtime/id=217
            qualifier   full/id=218
        syntax show_map/id=219
        syntax show_tb/id=220
        syntax show_regions/id=221
        syntax show_page/id=222
            qualifier   write/id=223/nonegatable
            qualifier   read/id=224/nonegatable
            parameter   address/id=225          -
                        /type=$rest_of_line     -
                        /prompt="Address"
            disallow    read and write
        syntax show_scb/id=226
            qualifier   all/id=227
        syntax show_shim/id=228
        syntax show_fault/id=229
        syntax show_mode/id=230
        syntax show_radix/id=231

        ! SHOW CALLS [count] lists the call frames, a row each; SHOW STACK
        ! [count] describes each in full.
        syntax show_calls/id=232
            parameter   count/id=233            -
                        /type=$rest_of_line
        syntax show_stack/id=234
            parameter   count/id=235            -
                        /type=$rest_of_line

        ! SHOW SP, KSP, ESP, SSP, USP, and ISP dump the longwords on a
        ! stack (govax's own): the current one, or a mode's.
        syntax show_sp/id=236
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238
        syntax show_ksp/id=239
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238
        syntax show_esp/id=240
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238
        syntax show_ssp/id=241
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238
        syntax show_usp/id=242
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238
        syntax show_isp/id=243
            parameter   count/id=237            -
                        /type=$rest_of_line
            qualifier   all/id=238

    ! CANCEL SOURCE forgets SET SOURCE's directories.
    ! CANCEL BREAK [qualifier] [address[,address...]] removes breakpoints:
    ! at the addresses, of a kind (/CALL, /BRANCH, ...), or all of them
    ! (/ALL).
    type cancel_types
        keyword     break               /syntax=cancel_break
        keyword     breakpoint          /syntax=cancel_break
        keyword     source              /syntax=cancel_source
        keyword     radix               /syntax=cancel_radix
        keyword     mode                /syntax=cancel_mode
        keyword     interrupt           /syntax=cancel_interrupt
        keyword     tb                  /syntax=cancel_tb
        keyword     translation_buffer  /syntax=cancel_tb
        keyword     memory              /syntax=cancel_memory

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

        syntax cancel_source/id=73

        syntax cancel_radix/id=86

        ! CANCEL MODE restores SET MODE's defaults.
        syntax cancel_mode/id=260

        ! CANCEL INTERRUPT [/ALL] [number] removes a pending interrupt,
        ! CANCEL TB empties the translation buffer, and CANCEL
        ! MEMORY/STATISTICS zeroes the memory counters (all govax's own).
        syntax cancel_interrupt/id=261
            qualifier   all/id=262
            parameter   code/id=263             -
                        /type=$rest_of_line
        syntax cancel_tb/id=264
        syntax cancel_memory/id=265
            qualifier   statistics/id=266

    ! CLEAR is a govax synonym for CANCEL (the console's old verb).
    verb clear/alias=cancel

    ! EXAMINE[/qualifiers] [location[,location...]] shows what is at each
    ! location. With /INSTRUCTION it is the machine instruction there (a
    ! location may be a range, start:end); /OPERANDS[=FULL] explains
    ! each operand; /CONSTANTS and /SHAREABLE are govax's, naming a
    ! constant and a G^ reference's routine. Without them it is the data:
    ! typed by the program's debug symbols, or by /BYTE, /WORD,
    ! /LONGWORD, /QUADWORD, or /ASCII[:count] (govax's own /PTE shows a
    ! page table entry, and /PSL a processor status longword), and shown
    ! in the output radix, or /HEXADECIMAL, /DECIMAL, /OCTAL, or /BINARY.
    ! /SYMBOLIC is accepted (SET MODE NOSYMBOLIC doesn't change data
    ! names). The location is the rest of the line, which
    ! internal/debugger takes apart.
    verb examine/id=87
        qualifier   instruction/id=88/nonegatable
        qualifier   operands/id=89              -
                    /type=$string               -
                    /default=""
        qualifier   constants/id=90/nonegatable
        qualifier   shareable/id=91/nonegatable
        qualifier   byte/id=93/nonegatable
        qualifier   word/id=94/nonegatable
        qualifier   longword/id=95/nonegatable
        qualifier   quadword/id=96/nonegatable
        qualifier   ascii/id=97                 -
                    /type=$string               -
                    /default=""/nonegatable
        qualifier   psl/id=98/nonegatable
        qualifier   pte/id=99/nonegatable
        qualifier   hexadecimal/id=100/nonegatable
        qualifier   decimal/id=101/nonegatable
        qualifier   octal/id=102/nonegatable
        qualifier   binary/id=103/nonegatable
        qualifier   symbolic/id=104
        parameter   location/id=92              -
                    /type=$rest_of_line
        disallow    any2(byte, word, longword, quadword, psl, pte, instruction)
        disallow    any2(hexadecimal, decimal, octal, binary)
    verb ex/alias=examine

    ! DEPOSIT[/type] location = value stores a value (in the input radix)
    ! or, with /ASCII[:count], a quoted string at a location: a register
    ! or an address. The type is /BYTE, /WORD, /LONGWORD, or /QUADWORD,
    ! else the type of the data the location labels, else a longword.
    verb deposit/id=105
        qualifier   byte/id=106/nonegatable
        qualifier   word/id=107/nonegatable
        qualifier   longword/id=108/nonegatable
        qualifier   quadword/id=109/nonegatable
        qualifier   ascii/id=110                -
                    /type=$string               -
                    /default=""/nonegatable
        parameter   assignment/id=111           -
                    /type=$rest_of_line         -
                    /prompt="Location"
        disallow    any2(byte, word, longword, quadword)
    verb d/alias=deposit

    ! EVALUATE[/radix] expression shows an expression's value: the
    ! contents of a data label (EVALUATE/ADDRESS shows its address). The
    ! number is in the output radix, or /HEXADECIMAL, /DECIMAL, /OCTAL, or
    ! /BINARY.
    verb evaluate/id=112
        qualifier   address/id=113/nonegatable
        qualifier   hexadecimal/id=114/nonegatable
        qualifier   decimal/id=115/nonegatable
        qualifier   octal/id=116/nonegatable
        qualifier   binary/id=117/nonegatable
        parameter   expression/id=118           -
                    /type=$rest_of_line         -
                    /prompt="Expression"
        disallow    any2(hexadecimal, decimal, octal, binary)
    verb eval/alias=evaluate

    ! SYMBOLIZE address shows each name the debugger has for an address:
    ! the program's symbols and lines, and then the global symbol table's.
    verb symbolize/id=119
        parameter   address/id=120              -
                    /type=$rest_of_line         -
                    /prompt="Address"

end

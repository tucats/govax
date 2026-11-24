!  Copyright (c) 1997-2026 Forest Edge Software
!                See LICENSE for applicable MIT license information.
!
!   DCLRTL grammar definition for govax console commands
!

grammar console

    ! Logical names (docs/PHASE-25.md). DEFINE and ASSIGN share one
    ! handler; /PROCESS, /JOB, /GROUP, /SYSTEM and /TABLE= all select a
    ! table.

    type lnm_attributes
        keyword concealed/id=1
        keyword terminal/id=2

    ! SHOW MEMORY/FILES is VMS's page file report (Phase 64), with each
    ! file's details under /FULL.
    syntax show_memory/id=215
        qualifier   statistics/id=216
        qualifier   runtime/id=217
        qualifier   full/id=218
        qualifier   files/id=233

    syntax show_logical
        parameter name/id=301/type=$any/list
        qualifier table/id=300/type=$any/list
        qualifier process/id=302
        qualifier job/id=307
        qualifier group/id=303
        qualifier system/id=304
        qualifier full/id=305
        qualifier structure/id=306
        disallow any2(process, job, group, system, table)

    syntax show_translation
        parameter name/id=311/type=$any/prompt="Log_Name"
        qualifier table/id=310/type=$any

    syntax create_name_table
        parameter table/id=241/type=$any/prompt="Table"
        qualifier parent_table/id=242/type=$any
        qualifier protection/id=247/type=$protection
        qualifier quota/id=248/type=$integer
        qualifier user_mode/id=243
        qualifier supervisor_mode/id=244
        qualifier executive_mode/id=245
        qualifier log/id=246
        disallow any2(user_mode, supervisor_mode, executive_mode)

    !
    ! docs/PHASE-34.md: CREATE/DIRECTORY makes a directory on a mounted
    ! volume, and every missing directory above it (internal/rms.Session.
    ! CreateDirectory, internal/dclcmd/create.go). OWNER_UIC is a UIC or
    ! PARENT; PROTECTION is a protection code (Phase 93's $UIC and
    ! $PROTECTION: the grammar checks their shape, the command the rest).
    !
    syntax create_directory
        parameter directories/id=1501/type=$string/list/prompt="Directory"
        qualifier owner_uic/id=1502/type=$uic
        qualifier version_limit/id=1503/type=$integer
        qualifier protection/id=1504/type=$protection
        qualifier allocation/id=1505/type=$integer
        qualifier log/id=1506

    !
    ! CREATE/FDL=fdl-file [file] makes an empty file from an FDL file's
    ! description (docs/PHASE-88 - keyed index files.md, subtask 10b;
    ! internal/dclcmd/createfdl.go). The file is optional: without one, the
    ! FDL file's FILE NAME names it. /FDL takes its value, then switches to
    ! this syntax.
    !
    syntax create_fdl
        parameter file/id=1515/type=$string
        qualifier fdl/id=1514/type=$string
        qualifier log/id=1516

    !
    ! CREATE with neither /NAME_TABLE nor /DIRECTORY makes a text file
    ! from SYS$INPUT (internal/dclcmd/createfile.go): the terminal up to
    ! Ctrl/Z, or a command procedure's data lines. /OWNER_UIC names its
    ! owner, /PROTECTION is merged into the protection it would otherwise
    ! get (docs/planning/PHASE-93 - file protection.md, subtask 11b).
    !
    verb create
        parameter file/id=1511/type=$string/prompt="File"
        qualifier host/id=1512/parameter=file
        qualifier log/id=1513
        qualifier owner_uic/id=1517/type=$uic
        qualifier protection/id=1518/type=$protection
        qualifier name_table/syntax=create_name_table
        qualifier directory/syntax=create_directory
        qualifier fdl/id=1514/type=$string/syntax=create_fdl

    !
    ! CONVERT copies records from up to 10 files to one, changing their
    ! organization and format (docs/PHASE-88 - keyed index files.md, subtask
    ! 10c; internal/dclcmd/convert.go).
    !
    verb convert
        parameter input/id=1520/type=$string/list/prompt="File"
        parameter output/id=1521/type=$string/prompt="Output"
        qualifier append/id=1522
        qualifier create/id=1523
        qualifier exceptions_file/id=1524/type=$string/default=""
        qualifier exit/id=1525
        qualifier fast_load/id=1526
        qualifier fdl/id=1527/type=$string
        qualifier fill_buckets/id=1528
        qualifier fixed_control/id=1529
        qualifier key/id=1530/type=$integer
        qualifier merge/id=1531
        qualifier pad/id=1532/type=$any/default=""
        qualifier prolog/id=1533/type=$integer
        qualifier read_check/id=1534
        qualifier share/id=1535
        qualifier sort/id=1536
        qualifier statistics/id=1537
        qualifier truncate/id=1538
        qualifier work_files/id=1539/type=$integer
        qualifier write_check/id=1540

    verb assign
        parameter value/id=221/type=$any/list/prompt="Equ_Name"
        parameter name/id=222/type=$any/prompt="Log_Name"
        qualifier process/id=223
        qualifier job/id=232
        qualifier group/id=224
        qualifier system/id=225
        qualifier table/id=226/type=$any
        qualifier user_mode/id=227
        qualifier supervisor_mode/id=228
        qualifier executive_mode/id=229
        qualifier translation_attributes/id=230/type=lnm_attributes/list
        qualifier log/id=231
        disallow any2(process, job, group, system, table)
        disallow any2(user_mode, supervisor_mode, executive_mode)

    verb deassign
        parameter name/id=251/type=$any
        qualifier process/id=252
        qualifier job/id=261
        qualifier group/id=253
        qualifier system/id=254
        qualifier table/id=255/type=$any
        qualifier user_mode/id=256
        qualifier supervisor_mode/id=257
        qualifier executive_mode/id=258
        qualifier all/id=259
        qualifier log/id=260
        disallow any2(process, job, group, system, table)
        disallow any2(user_mode, supervisor_mode, executive_mode)
    
    syntax show_device
        parameter name/id=100/type=$name
        qualifier full/id=101
        
    type dev_class
        keyword disk	/id=1
        keyword tape	/id=2
        keyword scom	/id=32
        keyword card	/id=65
        keyword term	/id=66
        keyword	lp	/id=67
        keyword workstation/id=70
        keyword realtime/id=96
        keyword decvoice/id=97
        keyword audio	/id=98
        keyword	video	/id=99
        keyword	bus	/id=128
        keyword mailbox	/id=160
        keyword recmsl_storage/id=179
        keyword misc	/id=200
        
    type dev_type
        keyword	rk06		/id=1
        keyword	rk07		/id=2
        keyword	rp04		/id=3
        ! The null device NLA0: has type 3, too.
        keyword	null		/id=3
        keyword rp05		/id=4
        keyword rp06		/id=5
        keyword rm03		/id=6
        keyword rp07		/id=7
        keyword	rp07ht		/id=8
        keyword rl01		/id=9
        keyword rl02		/id=10
        keyword	rx02		/id=11
        keyword rx04		/id=12
        keyword rm80		/id=13
        keyword tu58		/id=14
        keyword rm05		/id=15
        keyword rx01		/id=16
        keyword	ml11		/id=17
        keyword rb02		/id=18
        keyword rb80		/id=19
        keyword	ra80		/id=20
        keyword ra81		/id=21
        keyword	ra60		/id=22
        keyword	rz01		/id=23
        keyword rd51		/id=25
        keyword rx50		/id=26
        keyword RX33        /id=27
        keyword rd31        /id=28
        keyword RD52        /id=29
        keyword RD32        /id=30
        keyword RD33        /id=31
        keyword RD53        /id=32
        keyword RD54        /id=33
        keyword RA70        /id=34
        keyword RA82        /id=35
        keyword RA71        /id=36
        keyword RA72        /id=37
        keyword RA90        /id=38
        keyword RA92        /id=39
        keyword RA73        /id=40
        ! DT$_RAM_DISK, an MD device (Phase 96).
        keyword RAM_DISK    /id=58
        keyword vt100		/id=96
        
    syntax define_device
        parameter name/id=400/type=$name/prompt="Name"
        qualifier cluster/id=401/type=$integer
        qualifier cylinders/id=402/type=$integer
        qualifier devbufsize/id=403/type=$integer
        qualifier devchar/id=404/type=$integer
        qualifier devchar2/id=405/type=$integer
        qualifier devclass/id=406/type=dev_class
        qualifier devdepend/id=407/type=$integer
        qualifier devdepend2/id=408/type=$integer
        qualifier devtype/id=409/type=dev_type
        qualifier freeblocks/id=410/type=$integer
        qualifier lockid/id=411/type=$integer
        qualifier maxblock/id=412/type=$integer
        qualifier maxfiles/id=413/type=$integer
        qualifier ownuic/id=414/type=$integer
        qualifier recsize/id=415/type=$integer
        qualifier sectors/id=416/type=$integer
        qualifier serial/id=417/type=$integer
        qualifier volname/id=420/type=$string
        qualifier medianame/id=421/type=$string
        qualifier mediatype/id=422/type=$string
        qualifier rootdevname/id=423/type=$string
        ! /[NO]LOG: whether an MD device's definition says which unit it
        ! made (docs/planning/PHASE-96 - RAM disk.md, Decision 5).
        qualifier log/id=424
        
        
    verb define
        parameter name/id=211/type=$any/prompt="Log_Name"
        parameter value/id=212/type=$any/list/prompt="Equ_Name"
        qualifier	device/syntax=define_device
        qualifier process/id=213
        qualifier job/id=209
        qualifier group/id=214
        qualifier system/id=215
        qualifier table/id=216/type=$any
        qualifier user_mode/id=217
        qualifier supervisor_mode/id=218
        qualifier executive_mode/id=219
        qualifier translation_attributes/id=210/type=lnm_attributes/list
        qualifier log/id=220
        disallow any2(process, job, group, system, table)
        disallow any2(user_mode, supervisor_mode, executive_mode)
        
    ! EXIT [status] ends the command procedure it's in, or govax at the
    ! terminal; the status, a DCL expression, is the procedure's.
    verb exit
        parameter   status/id=1645              -
                    /type=$rest_of_line

    ! LOGOUT ends a subprocess's DCL, and so the process (docs/PHASE-50,
    ! subtask 19); in process 1 it ends govax. /BRIEF is the one line,
    ! /FULL the accounting report too; an interactive process's default
    ! is /BRIEF, a batch job's /FULL. The two are one setting, as in VMS's
    ! CLD: the last one given wins, so LO*GOUT := LOGOUT/FULL then LO/BRIEF
    ! is brief, and /NOFULL is /BRIEF, /NOBRIEF /FULL.
    verb logout/id=3186
        qualifier   brief/id=3187
        qualifier   full/id=3188
        synonym     brief nofull

    ! LO is LOGOUT, as on VMS, though LOAD begins with it too.
    verb lo/alias=logout

    ! AUTHORIZE [command]: govax's account utility (internal/authorize,
    ! docs/planning/PHASE-95 - authentication.md, Decision 10), a session
    ! at its UAF> prompt, or the one command given. MCR AUTHORIZE and RUN
    ! SYS$SYSTEM:AUTHORIZE reach it too (dclshell/authorize.go).
    verb authorize/id=2900
        parameter   command/id=2901             -
                    /type=$rest_of_line

    ! MCR image [text]: runs the image, with the text for
    ! LIB$GET_FOREIGN, as a foreign command does.
    verb mcr/id=1670
        parameter   image/id=1671               -
                    /type=$string               -
                    /prompt="Image"
        parameter   text/id=1672                -
                    /type=$rest_of_line

    ! ON condition THEN [$] command: what a command procedure does when a
    ! command fails with the condition's severity or worse, or on Ctrl/Y
    ! (docs/PHASE-50, subtask 10). CONTINUE does nothing, and goes on.
    ! The handler reads the condition (WARNING, ERROR, SEVERE_ERROR, or
    ! CONTROL_Y), so that one it doesn't have is DCL's ONERR.
    verb on/id=1646
        parameter   condition/id=1647           -
                    /type=$any                  -
                    /prompt="Condition"
        parameter   command/id=1648             -
                    /type=$rest_of_line         -
                    /prompt="Command"

    verb continue/id=1649

    ! WAIT delta-time: waits, until Ctrl/Y or Ctrl/C (docs/PHASE-50).
    verb wait/id=1673
        parameter   delta/id=1674               -
                    /type=$rest_of_line         -
                    /prompt="Time"

    ! DEBUG starts a debugger session on the machine as it stands, with
    ! nothing running (docs/PHASE-42.md). It is how to look at memory and
    ! registers now that EXAMINE and SHOW REGISTERS are debugger commands.
    verb debug/id=6000

    ! GO and CALL are the debugger's (debug.dcl); ASM is there too. CLEAR
    ! is gone from the console: the debugger's CANCEL (and its alias
    ! CLEAR) took breakpoints, interrupts, the translation buffer, the
    ! machine's symbol table, and the string pool. A DCL symbol is deleted
    ! with DELETE/SYMBOL, as in DCL.


    type show_types
        ! DEFAULT (docs/PHASE-23.md, subtask 3) has no reference/eVAX or
        ! testdata/dcl/console.dcl counterpart -- see this file's own
        ! "govax-native extension" comment at MOUNT's definition below.
        keyword     memory              /syntax=show_memory
        keyword         default         /syntax=show_default
        keyword		logical		/syntax=show_logical
        keyword		translation	/syntax=show_translation
        keyword		devices		/syntax=show_device
        keyword         version         /syntax=show_version
        keyword         nvram           /syntax=show_nvram      
        keyword         debug           /syntax=show_debug
        keyword         radix           /syntax=show_radix
        keyword         symbols         /syntax=show_sym
        keyword         rom             /syntax=show_rom
        keyword         share_prefix    /syntax=show_share
        ! SYSTEM and PROCESS (docs/PHASE-44.md, subtask 10): the process
        ! table and one process, as VMS's DCL shows them.
        keyword         system          /syntax=show_system
        keyword         process         /syntax=show_process
        ! WORKING_SET (docs/PHASE-63, subtask 9): the process's working
        ! set values.
        keyword         working_set     /syntax=show_working_set
        ! EDITOR: the editor EDIT runs, and where it works.
        keyword         editor          /syntax=show_editor
        ! PROTECTION (docs/planning/PHASE-93 - file protection.md,
        ! subtask 5): the process's default file protection.
        keyword         protection      /syntax=show_protection
        ! SECURITY and ACL (docs/planning/PHASE-93 - file protection.md,
        ! subtask 12): a file's owner, protection, and ACL.
        keyword         security        /syntax=show_security
        keyword         acl             /syntax=show_acl
        ! QUEUE (docs/planning/PHASE-98 - batch queues.md): batch queues,
        ! and with /MANAGER the queue manager.
        keyword         queue           /syntax=show_queue
        ! ENTRY (Phase 98, subtask 7): jobs, by entry number or user.
        keyword         entry           /syntax=show_entry
                

    ! SHOW (docs/PHASE-42.md): the console's keywords. The machine's
    ! (registers, memory, breakpoints, the stack, ...) are the debugger's.
    ! SHOW ASSEMBLER_FLAGS, COMMAND_ARGS, ERROR, EXPAND, and SHOW
    ! SYMBOL/TEMPORARY and /UNRESOLVED had no handler, and are gone (EXPAND
    ! in Phase 113, subtask 11, with an unreachable INSTRUCTIONS syntax: SHOW
    ! INSTRUCTIONS is the debugger's); SHOW SYMBOL/SYSTEM and the
    ! machine's SHOW SYMBOL/ALL are the debugger's now.
    verb show/id=120

        parameter       SHOW_TYPE/id=160            -
                        /type=show_types            -
                        /prompt="What"

        syntax          show_rom/id=154
        ! SHOW SYMBOL [/LOCAL | /GLOBAL] [/ALL] [name]: DCL's symbols, as
        ! DCL shows them (dclsym.go). The machine's symbol table is the
        ! debugger's SHOW SYMBOL.
        syntax          show_sym/id=149
            qualifier   all             /id=150
            qualifier   local           /id=151
            qualifier   global          /id=157
            parameter   symbol          /id=1021    -
                        /type=$name
            disallow    local and global
        syntax          show_radix/id=146
        syntax          show_nvram/id=122
        syntax          show_debug/id=129
        syntax          show_version/id=162
        syntax          show_share/id=161

        ! DEFAULT (docs/PHASE-23.md, subtask 3) has no reference/eVAX or
        ! testdata/dcl/console.dcl counterpart -- see this file's own
        ! "govax-native extension" comment at MOUNT's definition below.
        syntax          show_default/id=163

        ! SHOW SYSTEM and SHOW PROCESS [name] [/IDENTIFICATION=pid]
        ! [/PRIVILEGES] [/RIGHTS] [/QUOTAS] (docs/PHASE-44.md, subtask 10;
        ! /RIGHTS and /QUOTAS docs/planning/PHASE-95 - authentication.md);
        ! internal/dclcmd/showsys.go.
        syntax          show_system/id=164
        syntax          show_process/id=165
            qualifier   identification/id=1940      -
                        /type=$string
            parameter   process_name/id=1941        -
                        /type=$string
            qualifier   privileges/id=1942
            qualifier   rights/id=1943
            qualifier   quotas/id=1944
        syntax          show_protection/id=2720
        ! SHOW SECURITY file [/CLASS=FILE|VOLUME] and SHOW ACL file: one
        ! specification (wildcards allowed; a list is DCL's NOLIST, as on
        ! VMS 7.1), internal/dclcmd/showsecurity.go.
        syntax          show_security/id=2770
            parameter   file/id=2771                -
                        /type=$string/single        -
                        /prompt="Object"
            qualifier   class/id=2772/nonegatable   -
                        /type=$any
        syntax          show_acl/id=2775
            parameter   file/id=2776                -
                        /type=$string/single        -
                        /prompt="Object"
        syntax          show_working_set/id=166
        syntax          show_editor/id=167


    !
    ! govax-native extension (Phase 22, internal/rms): MOUNT/DISMOUNT have no
    ! reference/eVAX or testdata/dcl/console.dcl counterpart at all -- the C
    ! source never implemented real ODS-2 volume mounting (see
    ! docs/PHASE-22.md, "Why this phase looks different"). This file and
    ! testdata/dcl/console.dcl are expected to diverge starting here: if
    ! upstream eVAX ever changes and testdata/dcl/console.dcl is re-imported
    ! via `git archive`, do NOT copy that import over this file wholesale --
    ! doing so would silently delete this block (and any other govax-native
    ! grammar added below it).
    !
    verb mount/id=700

        parameter   device/id=701               -
                    /type=$name                 -
                    /prompt="Device"
        ! FILE is a container file, or for an MD device (a RAM disk) the
        ! volume label. It has no /prompt=, since a RAM disk needs none:
        ! MOUNT asks for "Container file" itself for other devices
        ! (internal/dclcmd/mount.go, Phase 96).
        parameter   file/id=702                 -
                    /type=$string
        qualifier   write/id=703
        qualifier   log/id=704

    verb dismount/id=710

        parameter   device/id=711               -
                    /type=$name                 -
                    /prompt="Device"

    !
    ! govax-native extension (Phase 23, internal/console + internal/rms):
    ! INITIALIZE unifies the pre-existing INIT (VAX-memory-allocation,
    ! Console.Init -- previously one of dispatch.go's fixedCommands entries,
    ! predating this grammar engine's own existence) and the new
    ! INITIALIZE/CONTAINER (ODS-2 container formatting) under one verb,
    ! selected by /VAX or /CONTAINER -- see docs/PHASE-23.md's "INITIALIZE:
    ! unifying INIT and INITIALIZE under one verb" design section. Neither
    ! qualifier is a default: this top-level entry has no handler of its
    ! own, so a bare INITIALIZE (or INIT) with no qualifier falls through to
    ! Grammar.Dispatch's own "no handler bound" error. INIT keeps working as
    ! DCL's ordinary unambiguous-prefix abbreviation of INITIALIZE
    ! (Grammar.matchVerb) -- it is not a second, separate verb.
    !
    verb initialize/id=800

        qualifier   vax/id=801/syntax=initialize_vax
        qualifier   container/id=802/syntax=initialize_container
        qualifier   memory/id=811/syntax=initialize_memory
        ! INITIALIZE/QUEUE (Phase 98): a batch queue made or changed.
        qualifier   queue/id=813/syntax=initialize_queue

    ! No /prompt= on PAGES: cmdInit's original CLI_NEEDPAGES error (a
    ! specific wording, not a formal interactive re-prompt) is preserved by
    ! the INITIALIZE_VAX handler checking Result.Present("PAGES") itself,
    ! rather than letting a formally required parameter produce a different
    ! message -- see docs/PHASE-23.md.
    syntax initialize_vax/id=803

        parameter   pages/id=804                -
                    /type=$rest_of_line

    ! INITIALIZE/CONTAINER's own internal/rms handler lands in Phase 23
    ! subtask 4 -- this syntax is stubbed (parameters/qualifiers only) here
    ! so /CONTAINER parses instead of erroring, even though nothing is bound
    ! to it yet (Grammar.Dispatch's "no handler bound" error covers it until
    ! then).
    syntax initialize_container/id=805

        parameter   path/id=806                 -
                    /type=$string               -
                    /prompt="Container file"
        parameter   label/id=808                -
                    /type=$string
        qualifier   size/id=807                 -
                    /type=$integer
        qualifier   cluster/id=809              -
                    /type=$integer
        qualifier   device/id=810               -
                    /type=$string
        ! VMS INITIALIZE's /HEADERS and /DIRECTORIES (Phase 111, subtask
        ! 11): the index file's headers and the MFD's entries made up front.
        qualifier   headers/id=814              -
                    /type=$integer
        qualifier   directories/id=815          -
                    /type=$integer

    ! INITIALIZE/MEMORY (was VMINIT) lays out virtual memory: the P0, P1,
    ! and S0 regions, the privileged modes' stacks, and the string pool,
    ! sizes in pages.
    syntax initialize_memory/id=812

        qualifier   p0/id=1801                  -
                    /type=$integer
        qualifier   p1/id=1802                  -
                    /type=$integer
        qualifier   s0/id=1803                  -
                    /type=$integer
        qualifier   ksp/id=1804                 -
                    /type=$integer/default=4
        qualifier   esp/id=1805                 -
                    /type=$integer/default=8
        qualifier   ssp/id=1806                 -
                    /type=$integer/default=8
        qualifier   isp/id=1807                 -
                    /type=$integer/default=4
        qualifier   stringpool/id=1808          -
                    /type=$integer/default=8
        ! /[NO]PAGING (docs/PHASE-62 - VMS paging.md, Decision 1): VMS
        ! paging on or off; without it, vax.sysgen.paging.enable.
        qualifier   paging/id=1818
        qualifier   debug/id=1809
        qualifier   verify/alias=debug
        qualifier   log/alias=debug

    !
    ! govax-native extension (Phase 23, internal/rms + internal/console):
    ! DIRECTORY has no reference/eVAX or testdata/dcl/console.dcl counterpart
    ! at all -- see this file's own "govax-native extension" comment at
    ! MOUNT's own definition above for why this file and testdata/dcl/
    ! console.dcl are expected to diverge starting at that point, which this
    ! block continues. No /prompt= on SPEC: an omitted file spec is a
    ! perfectly normal way to run DIRECTORY (it lists the whole current
    ! default directory, "*.*;*"), not a missing required argument -- see
    ! docs/PHASE-23.md's subtask 5 and internal/rms.Session.Directory's own
    ! doc comment.
    !
    verb directory/id=900

        parameter   spec/id=901                 -
                    /type=$string/list
        qualifier   full/id=902
        qualifier   file/id=903
        qualifier   size/id=904                 -
                    /type=$string/default=USED
        qualifier   date/id=905
        qualifier   owner/id=906
        qualifier   protection/id=907
        qualifier   versions/id=908             -
                    /type=$integer
        qualifier   heading/id=909
        qualifier   trailing/id=910
        qualifier   total/id=912
        ! /SECURITY and /ACL (docs/planning/PHASE-93 - file protection.md,
        ! subtask 12): the owner and protection with each file's ACEs, and
        ! the ACEs alone.
        qualifier   security/id=913
        qualifier   acl/id=914
        qualifier   output/id=911               -
                    /type=$string               -
                    /default=""

    !
    ! govax-native extension (Phase 23, internal/rms + internal/console):
    ! DELETE has no reference/eVAX or testdata/dcl/console.dcl counterpart at
    ! all -- see this file's own "govax-native extension" comment at MOUNT's
    ! own definition above for why this file and testdata/dcl/console.dcl are
    ! expected to diverge starting at that point, which this block
    ! continues. Unlike DIRECTORY's SPEC, DELETE's SPEC carries /prompt=:
    ! ods2's own cmdDelete declares MinArgs=1 (there is no sensible "delete
    ! everything in the current directory" default the way a bare DIRECTORY
    ! has one), so a bare DELETE with nothing typed after it is a missing
    ! required argument. The further rule that the spec name a version
    ! (";3", ";*", or the latest as ";0" or ";", never left out) is
    ! enforced by internal/rms.Session.Delete itself, not by this grammar --
    ! see docs/PHASE-23.md's subtask 6. SPEC is a list, as VMS's DELETE
    ! filespec[,...]; internal/dclcmd's Delete splits plus signs too.
    !
    verb delete/id=1100

        parameter   spec/id=1101               -
                    /type=$string/list          -
                    /prompt="File specification"
        qualifier   log/id=1102
        ! DELETE/QUEUE (Phase 98): a stopped queue and its jobs.
        qualifier   queue/id=1103/syntax=delete_queue
        ! DELETE/ENTRY=(n,...) [queue] (Phase 98, subtask 7): jobs.
        qualifier   entry/id=1104/syntax=delete_entry -
                    /type=$string/list

    !
    ! govax-native extension (Phase 23, internal/rms + internal/console):
    ! PURGE has no reference/eVAX or testdata/dcl/console.dcl counterpart at
    ! all -- see this file's own "govax-native extension" comment at MOUNT's
    ! own definition above. Like DIRECTORY's SPEC (and unlike DELETE's),
    ! PURGE's SPEC carries no /prompt=: a bare PURGE has a sensible default
    ! ("*.*", the whole current default directory), matching ods2's own
    ! cmdPurge. LIMIT is likewise optional -- absent, internal/rms.Session.
    ! Purge's caller (dispatch.go's own PURGE binding) defaults it to 1,
    ! the same "keep only the single most recent version" default ods2's
    ! own cmdPurge uses -- see docs/PHASE-23.md's subtask 7.
    !
    ! DUMP shows a volume file in hexadecimal, by block (/BLOCKS, the
    ! default) or by record (/RECORD), each with an optional range,
    ! (START=n,END=n,COUNT=n) (docs/PHASE-51).
    verb dump/id=1170

        parameter   spec/id=1171                -
                    /type=$string               -
                    /prompt="File"
        qualifier   blocks/id=1172/nonegatable  -
                    /type=$string/default=ALL
        qualifier   record/id=1173/nonegatable  -
                    /type=$string/default=ALL

    !
    ! docs/PHASE-69 - backup save sets.md: BACKUP lists, restores, compares,
    ! and saves VMS save sets (internal/dclcmd/backup.go). As in VMS's
    ! BACKUP, where a qualifier stands says what it is about (the System
    ! Management Utilities Reference Manual, 7.2): after the verb, a command
    ! qualifier; after INPUT, an input save-set or input file-selection
    ! qualifier; after OUTPUT, an output save-set or output file qualifier.
    ! /SAVE_SET, /CRC, /BY_OWNER, and govax's /HOST may follow either. OUTPUT
    ! has no /prompt=, since /LIST alone needs none; the handler asks for it
    ! otherwise. Which combinations of the input qualifiers (one date kind)
    ! and of the output's (one of /NEW_VERSION, /OVERLAY, /REPLACE) may be
    ! given together is checked there too: a DISALLOW is the entry's alone.
    !
    type backup_ignore
        keyword nobackup/id=1
        keyword interlock/id=2

    verb backup/id=2200

        parameter   input/id=2201               -
                    /type=$string/list          -
                    /prompt="From"
        qualifier   host/id=2202/parameter=input
        qualifier   save_set/id=2203/parameter=input
        qualifier   crc/id=2204/parameter=input
        qualifier   select/id=2205/parameter=input  -
                    /type=$string/list
        qualifier   exclude/id=2206/parameter=input -
                    /type=$string/list
        qualifier   since/id=2207/parameter=input   -
                    /type=$string
        qualifier   before/id=2208/parameter=input  -
                    /type=$string
        qualifier   by_owner/id=2209/parameter=input -
                    /type=$uic/default=""
        qualifier   backup/id=2210/parameter=input
        qualifier   created/id=2211/parameter=input
        qualifier   expired/id=2212/parameter=input
        qualifier   modified/id=2213/parameter=input

        parameter   output/id=2220              -
                    /type=$string
        qualifier   host/id=2221/parameter=output
        qualifier   save_set/id=2222/parameter=output
        qualifier   crc/id=2223/parameter=output
        qualifier   block_size/id=2224/parameter=output -
                    /type=$integer
        qualifier   group_size/id=2225/parameter=output -
                    /type=$integer
        qualifier   comment/id=2226/parameter=output -
                    /type=$string
        qualifier   by_owner/id=2227/parameter=output -
                    /type=$uic/default=""
        qualifier   new_version/id=2228/parameter=output
        qualifier   overlay/id=2229/parameter=output
        qualifier   replace/id=2230/parameter=output
        qualifier   protection/id=2231/parameter=output -
                    /type=$protection

        qualifier   list/id=2240                -
                    /type=$string               -
                    /default=""
        qualifier   full/id=2241
        qualifier   brief/id=2242
        qualifier   log/id=2243
        qualifier   compare/id=2244
        qualifier   ignore/id=2245              -
                    /type=backup_ignore/list
        disallow    full and brief

    verb purge/id=1150

        ! $file: DCL checks a directory's brackets (Phase 113, subtask 13c).
        parameter   spec/id=1151                -
                    /type=$file
        ! /KEEP=n, VMS's name (govax's was ods2's /LIMIT until Phase 113,
        ! subtask 11).
        qualifier   keep/id=1152                -
                    /type=$integer
        qualifier   log/id=1153

    !
    ! govax-native extension (Phase 23, internal/rms + internal/console):
    ! TYPE has no reference/eVAX or testdata/dcl/console.dcl counterpart at
    ! all -- see this file's own "govax-native extension" comment at MOUNT's
    ! own definition above. Like DELETE's SPEC (and unlike DIRECTORY's/
    ! PURGE's), TYPE's SPEC carries /prompt=: ods2's own cmdType declares
    ! MinArgs=1 (there is no sensible "type everything" default the way a
    ! bare DIRECTORY/PURGE has one). The further rule that the resolved
    ! spec match exactly one file (never a wildcard-matched several) is
    ! enforced by internal/rms.Session.Type itself, not by this grammar --
    ! see docs/PHASE-23.md's subtask 8.
    !
    ! /PAGE shows the file a screenful at a time, waiting for RETURN
    ! between screens (internal/dclcmd/page.go).
    !
    ! SPEC may be a host file or a volume file, by internal/rms/location.go's
    ! rules; /HOST forces the host. TYPE has one parameter, so /HOST is the
    ! verb's own qualifier: TYPE/HOST X and TYPE X/HOST mean the same.
    !
    ! govax's own: EDIT runs an editor on the host on a copy of the file
    ! (internal/dclcmd/edit.go). VMS's EDT and TPU have no place here.
    verb edit/id=2253

        parameter   spec/id=2254                -
                    /type=$string               -
                    /prompt="File"
        qualifier   host/id=2255

    verb type/id=1200

        parameter   spec/id=1201                -
                    /type=$string               -
                    /prompt="File specification"
        qualifier   page/id=1202
        qualifier   host/id=1203

    !
    ! govax-native extension (Phase 23, internal/dcl + internal/rms
    ! + internal/console): COPY has no reference/eVAX or testdata/dcl/
    ! console.dcl counterpart at all -- see this file's own "govax-native
    ! extension" comment at MOUNT's own definition above. Unlike every
    ! other verb in this block, SOURCE and DESTINATION each carry their
    ! own private HOST qualifier (via /parameter=, internal/dcl's
    ! new parameter-scoped-qualifier feature added for exactly this --
    ! docs/PHASE-23.md's subtask 2) rather than one qualifier shared
    ! across the whole command line -- see docs/PHASE-23.md's "COPY
    ! direction and the /HOST qualifier" design section for why: either
    ! argument can independently name a host path instead of a location
    ! on a mounted volume ("COPY foo.txt/HOST BAR.TXT" is host-to-
    ! container; "COPY FOO.TXT bar.txt/HOST" is container-to-host).
    ! Subtask 9 implemented the direction logic and /HOST itself,
    ! single-match-only; subtask 10 adds the remaining qualifiers below,
    ! plus lifting that restriction for a wildcarded SOURCE copying onto
    ! a directory destination (either a container directory or an
    ! existing host directory) -- see docs/PHASE-23.md's "COPY
    ! qualifier parity" design section and its own subtask 10 progress-
    ! log entry for exactly which direction(s) each one applies to
    ! (most only affect a host destination; only /BINARY, /QUIET,
    ! /VERBOSE, and /TEST carry over to a volume destination). /CRLF and
    ! /LF are mutually exclusive, enforced here via DISALLOW rather than
    ! in Go code, the same mechanism SHOW BREAK's own READ/WRITE
    ! restriction above already uses.
    !
    verb copy/id=1250

        parameter   source/id=1251              -
                    /type=$file                 -
                    /prompt="Source"
        qualifier   host/id=1252                -
                    /parameter=source

        parameter   destination/id=1253         -
                    /type=$file                 -
                    /prompt="Destination"
        qualifier   host/id=1254                -
                    /parameter=destination

        qualifier   binary/id=1255
        qualifier   quiet/id=1256
        qualifier   verbose/id=1257
        qualifier   test/id=1258
        qualifier   time/id=1259
        qualifier   ignore/id=1260
        qualifier   dirs/id=1261
        qualifier   stream/id=1262
        qualifier   vfc/id=1263
        qualifier   log/id=1270
        ! /PROTECTION is merged into what a volume output would otherwise
        ! get (docs/planning/PHASE-93 - file protection.md, subtask 11b).
        qualifier   protection/id=1271          -
                    /type=$protection
        qualifier   crlf/id=1264
        qualifier   lf/id=1265
        ! govax's (docs/PHASE-86; rms/copyformat.go): what a host file is to be
        ! on the volume, named as in FDL's RECORD section, since a host file
        ! can't say what records it holds.
        qualifier   record_format/id=1266       -
                    /type=$string
        qualifier   record_size/id=1267         -
                    /type=$integer
        qualifier   carriage_control/id=1268    -
                    /type=$string
        qualifier   control_field_size/id=1269  -
                    /type=$integer
        disallow    crlf and lf

    !
    ! govax-native extension (docs/PHASE-27.md subtask 10, internal/console
    ! + internal/asm + internal/rms): MACRO assembles a MACRO-32 source
    ! file into a VAX object module. SOURCE carries its own HOST qualifier,
    ! the same parameter-scoped one COPY uses; without it, the file-name
    ! rules in internal/rms/location.go decide whether SOURCE is a host
    ! file or a file on a mounted volume. OBJECT is VMS MACRO's own
    ! /[NO]OBJECT[=file]: with no value (the empty default), or no
    ! qualifier at all, the object is the source's name with type OBJ.
    ! LIBRARY (docs/PHASE-28.md subtask 9) names macro libraries to search
    ! ahead of STARLET.MLB. It is positional (Phase 113, subtask 5): after
    ! an element with no value it makes that element a library, VMS's
    ! "PROG+LIB/LIBRARY"; with a value, after the verb or an element, it
    ! names the libraries itself, govax's older form.
    ! LIST (docs/PHASE-29.md subtask 5) is VMS MACRO's /[NO]LIST[=file]:
    ! off unless given, and with no value (the empty default) the listing
    ! is the source's name with type LIS. SHOW (subtask 7) is VMS MACRO's
    ! /SHOW=(option,...) and /NOSHOW=(option,...), the listing options set
    ! for the whole listing. CROSS_REFERENCE (subtask 9) is VMS MACRO's
    ! /[NO]CROSS_REFERENCE[=(option,...)]: the listing's cross reference,
    ! of SYMBOLS and MACROS unless options say which. ENABLE and DISABLE
    ! (subtask 11) are VMS MACRO's /ENABLE=(function,...) and
    ! /DISABLE=(function,...), .ENABLE's functions at the start, and DEBUG
    ! is /[NO]DEBUG[=(ALL|SYMBOLS|TRACEBACK|NONE)].
    !
    verb macro/id=1300

        parameter   source/id=1301              -
                    /type=$string/list          -
                    /joinable                   -
                    /prompt="Source"
        qualifier   host/id=1302                -
                    /parameter=source

        qualifier   object/id=1303              -
                    /type=$string               -
                    /default=""
        qualifier   library/id=1304             -
                    /parameter=source           -
                    /placement=positional       -
                    /type=$string/list          -
                    /default=""
        qualifier   list/id=1305                -
                    /type=$string               -
                    /default=""
        qualifier   show/id=1306                -
                    /type=$string/list
        qualifier   cross_reference/id=1307     -
                    /type=$string/list          -
                    /default=""
        qualifier   enable/id=1308              -
                    /type=$string/list
        qualifier   disable/id=1309             -
                    /type=$string/list
        qualifier   debug/id=1310               -
                    /type=$string/list          -
                    /default=""

    !
    ! govax-native extension (docs/PHASE-74 - FORTRAN plan.md, Decision
    ! 18; internal/console + internal/fortran + internal/rms): FORTRAN
    ! compiles FORTRAN source files into MACRO-32 source files, which
    ! MACRO assembles. SOURCES is a comma-separated list, each compiled to
    ! its own MACRO file, and its /HOST applies to each, as LINK's does to
    ! its objects. MACRO is /[NO]MACRO[=file], the output as MACRO's
    ! /OBJECT is its object: with no value (the empty default), or no
    ! qualifier at all, the MACRO file is the source's name with type
    ! MAR, and /NOMACRO compiles without writing one. EXTEND_SOURCE and
    ! D_LINES are VAX FORTRAN's /[NO]EXTEND_SOURCE (statements to column
    ! 132) and /[NO]D_LINES (compile the debugging lines), and
    ! CONTINUATIONS is /CONTINUATIONS=n, the most continuation lines a
    ! statement may have (0 to 99; 19 when not given). I4 is /[NO]I4:
    ! INTEGER and LOGICAL with no length are 4 bytes, or with /NOI4 2.
    ! G_FLOATING is /[NO]G_FLOATING: REAL*8 is D_floating, or with
    ! /G_FLOATING G_floating. ALIGN is /ALIGN[=value] or /NOALIGN, how
    ! structures and common blocks are laid out (docs/PHASE-82): no value
    ! is ALL, a value is ALL, NONE, a rule, or class=rule, and a list of
    ! them is in parentheses. OPTIMIZE is /[NO]OPTIMIZE: run the optimizer's
    ! passes (docs/PHASE-84), which the command does unless told not to. LIST is
    ! /[NO]LIST[=file], a listing (docs/PHASE-85; empty default: the source's name
    ! with type LIS), and SHOW is /[NO]SHOW[=(keyword,...)], what it holds, and CROSS_REFERENCE
    ! is /[NO]CROSS_REFERENCE, where each name is mentioned. CHECK is /[NO]CHECK[=(keyword,...)], what the
    ! compiled code looks out for (sema.Checks), and WARNINGS is /[NO]WARNINGS[=(keyword,...)], what
    ! the compiler says beyond errors (diag.Warnings), and STANDARD is /[NO]STANDARD[=(keyword,...)], what
    ! to flag as an extension to FORTRAN-77 (sema.Conformance), DEBUG is /[NO]DEBUG[=(keyword,...)], what
    ! MACRO is to write for the debugger (fortran.Debug), and OBJECT is /[NO]OBJECT[=file], MACRO run on
    ! the result (empty default: the source's name with type OBJ), and F77 is /[NO]F77, FORTRAN-66's reading
    ! of what FORTRAN-77 changed, and RECURSIVE is /[NO]RECURSIVE, every
    ! subprogram recursive, as if each said RECURSIVE (docs/PHASE-89). F90 is
    ! /[NO]F90, Fortran 90's reading of the texts it and VAX FORTRAN read
    ! differently (docs/PHASE-99, Decision 2; the default is the source's form:
    ! free form /F90, fixed /NOF90), and SOURCE_FORM is /SOURCE_FORM=FREE or
    ! FIXED, the form of the files named whatever their types say (Decision 3).
    ! MODULE is /MODULE=directory, where module files (NAME.F90$MOD) are
    ! written and looked for first, and MODULE_PATH is /MODULE_PATH=(dir,...),
    ! the directories looked in last (docs/PHASE-106, subtask 5).
    ! Elements joined by + are one compilation
    ! (docs/PHASE-83, subtask 3), and LIBRARY is positional, as LINK's: the
    ! element is a text library that compilation's INCLUDE '(module)'
    ! statements search ("FORTRAN PROG+MYLIB/LIBRARY").
    !
    type fortran_source_form
        keyword free/id=1
        keyword fixed/id=2

    verb fortran/id=1370

        parameter   sources/id=1371             -
                    /type=$string/list          -
                    /joinable                   -
                    /prompt="File"
        qualifier   host/id=1372                -
                    /parameter=sources
        qualifier   library/id=1380             -
                    /parameter=sources          -
                    /placement=positional

        qualifier   macro/id=1373               -
                    /type=$string               -
                    /default=""
        qualifier   extend_source/id=1374
        qualifier   d_lines/id=1375
        qualifier   i4/id=1377
        qualifier   f77/id=1390
        qualifier   recursive/id=1391
        qualifier   f90/id=1392
        qualifier   source_form/id=1393         -
                    /type=fortran_source_form
        qualifier   g_floating/id=1378
        qualifier   align/id=1379               -
                    /type=$any/list             -
                    /default=""
        qualifier   optimize/id=1381
        qualifier   list/id=1382                -
                    /type=$string               -
                    /default=""
        qualifier   show/id=1383                -
                    /type=$any/list             -
                    /default=""
        qualifier   cross_reference/id=1384
        qualifier   check/id=1385               -
                    /type=$any/list             -
                    /default=""
        qualifier   warnings/id=1386            -
                    /type=$any/list             -
                    /default=""
        qualifier   standard/id=1387            -
                    /type=$any/list             -
                    /default=""
        qualifier   debug/id=1388               -
                    /type=$any/list             -
                    /default=""
        qualifier   object/id=1389              -
                    /type=$string               -
                    /default=""
        qualifier   continuations/id=1376       -
                    /type=$integer
        qualifier   module/id=1394              -
                    /type=$string
        qualifier   module_path/id=1395         -
                    /type=$string/list

    !
    ! govax-native extension (docs/PHASE-30.md, internal/console +
    ! internal/link): LINK links object modules into a VMS executable
    ! image. OBJECTS is a comma-separated list, and its /HOST applies to
    ! each, as MACRO's does to its source. EXECUTABLE is VMS LINK's own
    ! /[NO]EXECUTABLE[=file] (empty default: the first object's name with
    ! type EXE), and TRACEBACK is on unless /NOTRACEBACK. DEBUG is
    ! /[NO]DEBUG (docs/PHASE-29.md, subtask 16): the objects' debugger
    ! records go into the image too; its value, VMS's user-written
    ! debugger module, is refused. /NOSYSLIB
    ! skips IMAGELIB.OLB and STARLET.OLB, as VMS LINK's does. MAP is
    ! /[NO]MAP[=file] (empty default: the first object's name with type
    ! MAP), and /BRIEF makes the map brief, /FULL full (docs/PHASE-68,
    ! subtask 8; the two conflict). LIBRARY, INCLUDE,
    ! SELECTIVE_SEARCH, and OPTIONS are positional: each belongs to the
    ! file it follows ("LINK MAIN,MYLIB/LIBRARY,PROG/OPTIONS").
    ! SHAREABLE is /SHAREABLE[=file] (docs/PHASE-68, subtask 5): a
    ! shareable image, named by its value or as EXECUTABLE says.
    ! SYMBOL_TABLE is /SYMBOL_TABLE[=file] (subtask 6): the image's
    ! global symbol table as an object module file, NAME.STB by default.
    ! USERLIBRARY is /[NO]USERLIBRARY[=(table,...)] (subtask 12): the
    ! tables (PROCESS, GROUP, SYSTEM, ALL, NONE) whose LNK$LIBRARY names
    ! name default user libraries; ALL when it's left out.
    !
    verb link/id=1350

        parameter   objects/id=1351             -
                    /type=$string/list          -
                    /prompt="Object"
        qualifier   host/id=1352                -
                    /parameter=objects
        qualifier   library/id=1358             -
                    /parameter=objects          -
                    /placement=positional
        qualifier   include/id=1359             -
                    /parameter=objects          -
                    /placement=positional       -
                    /type=$string/list
        qualifier   selective_search/id=1360    -
                    /parameter=objects          -
                    /placement=positional
        qualifier   options/id=1361             -
                    /parameter=objects          -
                    /placement=positional

        qualifier   executable/id=1353          -
                    /type=$string               -
                    /default=""
        qualifier   shareable/id=1363           -
                    /type=$string               -
                    /default=""
        qualifier   symbol_table/id=1364        -
                    /type=$string               -
                    /default=""
        qualifier   traceback/id=1354
        qualifier   debug/id=1362               -
                    /type=$string               -
                    /default=""
        qualifier   syslib/id=1355
        qualifier   map/id=1356                 -
                    /type=$string               -
                    /default=""
        qualifier   brief/id=1357
        qualifier   full/id=1365
        qualifier   userlibrary/id=1366         -
                    /type=$string/list          -
                    /default=""

    !
    ! docs/PHASE-38.md (internal/anl, internal/dclcmd/analyze.go):
    ! ANALYZE/OBJECT describes object files as VMS's ANALYZE does. Each
    ! kind of analysis is a qualifier that switches to its own syntax:
    ! /OBJECT here, /IMAGE below. FILES may
    ! be host files or volume files (HOST forces the host). OUTPUT is
    ! /OUTPUT[=file] (empty default: NAME.ANL beside the input); without
    ! it the report goes to the console. DBG, EOM, GSD, LNK, MHD, TBT,
    ! and TIR limit the records shown; INCLUDE names modules of an object
    ! library (empty default: every module).
    !
    syntax analyze_object
        parameter   files/id=1901               -
                    /type=$string/list          -
                    /prompt="File"
        qualifier   host/id=1902                -
                    /parameter=files
        qualifier   output/id=1903              -
                    /type=$string               -
                    /default=""
        qualifier   dbg/id=1904
        qualifier   eom/id=1905
        qualifier   gsd/id=1906
        qualifier   lnk/id=1907
        qualifier   mhd/id=1908
        qualifier   tbt/id=1909
        qualifier   tir/id=1910
        qualifier   include/id=1911             -
                    /type=$string/list          -
                    /default=""

    !
    ! docs/PHASE-40.md: ANALYZE/IMAGE describes image files. FILES and
    ! HOST are as ANALYZE/OBJECT's; OUTPUT's empty default is NAME.ANI
    ! beside the input. HEADER limits the report to the image header;
    ! FIXUP_SECTION asks for the fixup section with it (both, without
    ! either).
    !
    syntax analyze_image
        parameter   files/id=1921               -
                    /type=$string/list          -
                    /prompt="File"
        qualifier   host/id=1922                -
                    /parameter=files
        qualifier   output/id=1923              -
                    /type=$string               -
                    /default=""
        qualifier   header/id=1924
        qualifier   fixup_section/id=1925

    !
    ! Phase 88: ANALYZE/RMS_FILE describes RMS files. OUTPUT's empty default
    ! is SYS$OUTPUT, or NAME.FDL beside the input with /FDL. CHECK is the
    ! report without another's name; FDL, STATISTICS, and SUMMARY each
    ! choose theirs.
    !
    syntax analyze_rms_file
        parameter   files/id=1931               -
                    /type=$string/list          -
                    /prompt="File"
        qualifier   host/id=1932                -
                    /parameter=files
        qualifier   output/id=1933              -
                    /type=$string               -
                    /default=""
        qualifier   check/id=1934
        qualifier   fdl/id=1935
        qualifier   statistics/id=1936
        qualifier   summary/id=1937

    verb analyze/id=1900
        qualifier object/syntax=analyze_object
        qualifier image/syntax=analyze_image
        qualifier rms_file/syntax=analyze_rms_file

    !
    ! govax-native extension (docs/PHASE-28.md subtask 7, internal/console
    ! + internal/lbr): LIBRARY creates, changes, extracts from, and lists
    ! macro (.MLB) and object (.OLB) libraries, in the style of VMS's
    ! LIBRARIAN. LIBRARY and INPUTS each carry their own HOST qualifier,
    ! as COPY's parameters do. With input files and no operation named,
    ! they replace modules of the same names (LIBRARIAN's default
    ! /REPLACE). DELETE and EXTRACT take module names, which may hold * and
    ! %. LIST is /LIST[=file] (empty default: the console). MACRO, OBJECT,
    ! SHARE (a shareable image symbol table library, whose inputs are
    ! shareable images: docs/PHASE-68, subtask 12), or TEXT (a text library,
    ! a module per input file: docs/PHASE-83, subtask 3), or HELP (a help
    ! library, a module per level-1 key: docs/PHASE-90, subtask 3) picks a new library's
    ! type; SQUEEZE is on unless /NOSQUEEZE. MODULE is positional, the
    ! module's name for the text file it follows, by default the file's name.
    ! CREATE may take (BLOCKS:n,GLOBALS:n,HISTORY:n,KEYSIZE:n,MODULES:n).
    ! HISTORY lists the library's update history with LIST, and DATA is
    ! /DATA=REDUCE or /DATA=EXPAND (docs/PHASE-90, subtask 11e).
    !
    verb library/id=1400

        parameter   library/id=1401             -
                    /type=$string               -
                    /prompt="Library"
        qualifier   host/id=1402                -
                    /parameter=library

        parameter   inputs/id=1403              -
                    /type=$string/list
        qualifier   host/id=1404                -
                    /parameter=inputs
        qualifier   module/id=1422              -
                    /parameter=inputs           -
                    /placement=positional       -
                    /type=$string

        qualifier   create/id=1405              -
                    /type=$string/list          -
                    /default=""
        qualifier   insert/id=1406
        qualifier   replace/id=1407
        qualifier   delete/id=1408              -
                    /type=$string/list
        qualifier   extract/id=1409             -
                    /type=$string/list
        qualifier   output/id=1410              -
                    /type=$string
        qualifier   list/id=1411                -
                    /type=$string               -
                    /default=""
        qualifier   full/id=1412
        qualifier   names/id=1413
        qualifier   width/id=1414               -
                    /type=$integer
        qualifier   macro/id=1415
        qualifier   object/id=1416
        qualifier   squeeze/id=1417
        qualifier   selective_search/id=1418
        qualifier   log/id=1419
        qualifier   share/id=1420
        qualifier   text/id=1421
        qualifier   help/id=1423
        qualifier   history/id=1424
        qualifier   data/id=1425                -
                    /type=$string
        disallow    macro and object
        disallow    macro and share
        disallow    object and share
        disallow    text and macro
        disallow    text and object
        disallow    text and share
        disallow    help and macro
        disallow    help and object
        disallow    help and share
        disallow    help and text
        disallow    insert and replace

    !
    ! govax-native extension (docs/PHASE-22.md subtask 18, internal/rms +
    ! internal/console): RENAME is VMS DCL's RENAME -- renames or moves
    ! files on a mounted volume, never across volumes. INPUTS is a comma-
    ! separated list whose device and directory carry over from one item
    ! to the next; OUTPUT's missing fields come from each input file's own
    ! name, and "*" in its name, type, or version means the input's. /LOG
    ! reports each file renamed. NEW_VERSION is on unless /NONEW_VERSION:
    ! without it, a file keeps its version even when the input named none.
    !
    verb rename/id=1450

        parameter   inputs/id=1451              -
                    /type=$string/list          -
                    /prompt="From"
        parameter   output/id=1452              -
                    /type=$string               -
                    /prompt="To"

        qualifier   log/id=1453
        qualifier   new_version/id=1454

    !
    ! docs/PHASE-37.md: the console's former fixed commands, which until
    ! then dispatch.go's fixedCommands table matched by their first four
    ! characters and parsed by hand. Their old spellings are verbs or
    ! aliases here, so each still works, and an exact spelling wins over
    ! an abbreviation (S is STEP, D is DEPOSIT). An $expression is a
    ! console expression, evaluated by the handler (expr.go).
    !
    ! STOP deletes a process (and its subprocesses): by name, or by its
    ! hexadecimal PID with /IDENTIFICATION, as SHOW PROCESS names one.
    ! With neither, it ends every command procedure (docs/PHASE-50,
    ! subtask 18). /IMAGE ends the process's image instead, and
    ! /EXIT[=mode] (the default) or /NOEXIT says which exit handlers run
    ! before the deletion (docs/PHASE-51).
    verb stop/id=1605
        qualifier   identification/id=1606      -
                    /type=$string
        qualifier   image/id=1608/nonegatable
        qualifier   exit/id=1609                -
                    /type=$string/default=EXECUTIVE_MODE
        parameter   process_name/id=1607        -
                    /type=$string
        ! STOP/QUEUE (Phase 98): a queue, or with /MANAGER its manager.
        qualifier   queue/id=3101/syntax=stop_queue

    ! The queue commands (docs/planning/PHASE-98 - batch queues.md,
    ! Decision 10; internal/dclcmd/queues.go and showqueue.go): batch
    ! queues and their manager. INITIALIZE/QUEUE, START/QUEUE, STOP/QUEUE,
    ! SET QUEUE, DELETE/QUEUE, and SHOW QUEUE are each a syntax of its
    ! own, reached by /QUEUE (or SET's and SHOW's QUEUE keyword), and
    ! /MANAGER switches START's, STOP's, and SHOW's to the manager's. The
    ! queue's attributes are the same qualifiers in INITIALIZE/QUEUE,
    ! START/QUEUE, and SET QUEUE. A qualifier VMS has that govax doesn't
    ! keep is DCL's IVQUAL (Decision 1).
    syntax initialize_queue/id=3000
        parameter   queue_name/id=3001          -
                    /type=$string               -
                    /prompt="Queue name"
        qualifier   batch/id=3002
        qualifier   start/id=3003
        qualifier   job_limit/id=3004/type=$integer
        qualifier   base_priority/id=3005/type=$integer
        qualifier   owner_uic/id=3006/type=$uic
        qualifier   protection/id=3007/type=$protection
        qualifier   wsdefault/id=3008/type=$integer
        qualifier   wsquota/id=3009/type=$integer
        qualifier   wsextent/id=3010/type=$integer
        qualifier   cpudefault/id=3011/type=$string
        qualifier   cpumaximum/id=3012/type=$string
        qualifier   retain/id=3013/type=$string
        qualifier   description/id=3014/type=$string

    verb start/id=3020
        qualifier   queue/id=3021/syntax=start_queue

    syntax start_queue/id=3022
        parameter   queue_name/id=3023          -
                    /type=$string               -
                    /prompt="Queue name"
        qualifier   manager/id=3024/syntax=start_queue_manager
        qualifier   job_limit/id=3025/type=$integer
        qualifier   base_priority/id=3026/type=$integer
        qualifier   owner_uic/id=3027/type=$uic
        qualifier   protection/id=3028/type=$protection
        qualifier   wsdefault/id=3029/type=$integer
        qualifier   wsquota/id=3030/type=$integer
        qualifier   wsextent/id=3031/type=$integer
        qualifier   cpudefault/id=3032/type=$string
        qualifier   cpumaximum/id=3033/type=$string
        qualifier   retain/id=3034/type=$string
        qualifier   description/id=3035/type=$string

    ! START/QUEUE/MANAGER [directory]: the directory is recorded and
    ! shown; the database is the host file (Decision 3).
    syntax start_queue_manager/id=3040
        parameter   directory/id=3041           -
                    /type=$string

    syntax stop_queue/id=3050
        parameter   queue_name/id=3051          -
                    /type=$string               -
                    /prompt="Queue name"
        qualifier   next/id=3052
        qualifier   reset/id=3053
        qualifier   manager/id=3054/syntax=stop_queue_manager
        qualifier   entry/id=3057/type=$integer
        ! /REQUEUE[=queue]/ENTRY=n [/HOLD] (subtask 10): the executing
        ! job stopped and requeued, to its own queue or the one named; a
        ! batch queue's needs /ENTRY, and /REQUEUE goes with neither /NEXT
        ! nor /RESET, which the handler checks (the empty default, which
        ! lets /REQUEUE go without a value, makes it present to a
        ! disallow).
        qualifier   requeue/id=3058/type=$string/default=""
        qualifier   hold/id=3059
        disallow    any2(next, reset, entry)
        disallow    any2(next, reset, hold)

    syntax stop_queue_manager/id=3055
        qualifier   cluster/id=3056

    syntax delete_queue/id=3060
        parameter   queue_name/id=3061          -
                    /type=$string               -
                    /prompt="Queue name"

    syntax set_queue/id=3070
        parameter   queue_name/id=3071          -
                    /type=$string               -
                    /prompt="Queue name"
        qualifier   job_limit/id=3072/type=$integer
        qualifier   base_priority/id=3073/type=$integer
        qualifier   owner_uic/id=3074/type=$uic
        qualifier   protection/id=3075/type=$protection
        qualifier   wsdefault/id=3076/type=$integer
        qualifier   wsquota/id=3077/type=$integer
        qualifier   wsextent/id=3078/type=$integer
        qualifier   cpudefault/id=3079/type=$string
        qualifier   cpumaximum/id=3080/type=$string
        qualifier   retain/id=3081/type=$string
        qualifier   description/id=3082/type=$string

    ! SHOW QUEUE [name]: VMS 7.1's has no /USER_NAME (the probe's A.4).
    syntax show_queue/id=3090
        parameter   queue_name/id=3091          -
                    /type=$string
        qualifier   batch/id=3092
        qualifier   all_jobs/id=3093
        qualifier   full/id=3094
        qualifier   brief/id=3095
        qualifier   summary/id=3096
        qualifier   by_job_status/id=3097       -
                    /type=$string/list
        qualifier   manager/id=3098/syntax=show_queue_manager

    syntax show_queue_manager/id=3099
        qualifier   full/id=3100

    ! The job commands (docs/planning/PHASE-98 - batch queues.md, subtask
    ! 7; internal/dclcmd/jobs.go): SUBMIT enters a job, SET ENTRY changes
    ! one, DELETE/ENTRY deletes one, SHOW ENTRY shows them. SUBMIT's files
    ! are a list, "," between jobs' files and "+" within one, which VMS
    ! takes alike: every file is one job's.
    ! SYNCHRONIZE [job-name] [/ENTRY=n] [/QUEUE=name] (Phase 98, subtask 8;
    ! internal/dclshell/dclsync.go): wait for a job to end, its status the
    ! command's.
    verb synchronize/id=3180
        parameter   job_name/id=3181            -
                    /type=$string
        qualifier   entry/id=3182/type=$integer
        qualifier   queue/id=3183/type=$string

    verb submit/id=3110
        parameter   files/id=3111               -
                    /type=$string/list          -
                    /prompt="File"
        qualifier   queue/id=3112/type=$string
        qualifier   after/id=3113/type=$string
        qualifier   user/id=3114/type=$string
        qualifier   parameters/id=3115/type=$string/list
        qualifier   hold/id=3116
        qualifier   log_file/id=3117/type=$string/default=""
        qualifier   name/id=3118/type=$string
        qualifier   keep/id=3119
        qualifier   printer/id=3120/type=$string/default=""
        qualifier   notify/id=3121
        qualifier   priority/id=3122/type=$integer
        qualifier   cputime/id=3123/type=$string
        qualifier   wsdefault/id=3124/type=$integer
        qualifier   wsquota/id=3125/type=$integer
        qualifier   wsextent/id=3126/type=$integer
        qualifier   restart/id=3127
        qualifier   retain/id=3128/type=$string
        qualifier   note/id=3129/type=$string
        qualifier   identify/id=3130
        qualifier   delete/id=3131

    ! SET ENTRY n: a job's options. /REQUEUE moves a job not executing to
    ! another queue.
    syntax set_entry/id=3140
        parameter   entry/id=3141               -
                    /type=$string/list          -
                    /prompt="Entry"
        qualifier   hold/id=3142
        qualifier   release/id=3143
        qualifier   after/id=3144/type=$string
        qualifier   parameters/id=3145/type=$string/list
        qualifier   name/id=3146/type=$string
        qualifier   priority/id=3147/type=$integer
        qualifier   note/id=3148/type=$string
        qualifier   log_file/id=3149/type=$string
        qualifier   cputime/id=3150/type=$string
        qualifier   requeue/id=3151/type=$string
        disallow    hold and release

    ! DELETE/ENTRY=(n,...) [queue]: the entries, in queue if it is given.
    syntax delete_entry/id=3160
        parameter   queue_name/id=3161          -
                    /type=$string
        qualifier   entry/id=3162/type=$string/list

    ! SHOW ENTRY [n,...]: jobs by number, or the user's.
    syntax show_entry/id=3170
        parameter   entry/id=3171               -
                    /type=$string/list
        qualifier   full/id=3172
        qualifier   brief/id=3173
        qualifier   user_name/id=3174/type=$string

    ! INSTALL subcommand [file] (docs/planning/PHASE-94 - privileges.md,
    ! Decision 9; internal/dclcmd/install.go): the known file list. ADD
    ! (CREATE) makes an image known, with privileges of its own by
    ! /PRIVILEGED; REMOVE (DELETE) forgets it; LIST shows the list or one
    ! entry. ADD's other qualifiers are recorded and shown. ADD and REMOVE
    ! need CMKRNL.
    type install_types
        keyword     add                 /syntax=install_add/nonegatable
        keyword     create              /syntax=install_add/nonegatable
        keyword     remove              /syntax=install_remove/nonegatable
        keyword     delete              /syntax=install_remove/nonegatable
        keyword     list                /syntax=install_list/nonegatable

    verb install/id=2850
        parameter   subcommand/id=2851          -
                    /type=install_types         -
                    /prompt="Subcommand"

    syntax install_add/id=2852
        parameter   file/id=2853                -
                    /type=$string               -
                    /prompt="File"
        qualifier   privileged/id=2854          -
                    /type=$any/list
        qualifier   open/id=2855
        qualifier   header_resident/id=2856
        qualifier   shared/id=2857
        qualifier   protected/id=2858
        qualifier   writeable/id=2859
        qualifier   accounting/id=2860
        qualifier   execute_only/id=2861
        qualifier   resident/id=2862
        qualifier   purge/id=2863

    syntax install_remove/id=2864
        parameter   file/id=2865                -
                    /type=$string               -
                    /prompt="File"

    syntax install_list/id=2866
        parameter   file/id=2867                -
                    /type=$string
        qualifier   full/id=2868/nonegatable

    verb rom/id=1602

    ! TIME runs a command and reports how long it took; with none, it
    ! prints the time of day.
    verb time/id=1610
        parameter   command/id=1611             -
                    /type=$rest_of_line

    ! PRINT writes a list of quoted strings and expressions.
    verb print/id=1620
        parameter   items/id=1621               -
                    /type=$expression/list
    verb echo/alias=print

    ! HELP's topic words are the help library's to match, so they're the
    ! rest of the line ("HELP SHOW SYMBOL/ALL"), and its qualifiers come
    ! right after the verb ("HELP/NOPROMPT COPY /LOG" asks about /LOG).
    ! The qualifiers are VMS's (docs/PHASE-90, subtask 6); SEARCH, EXACT,
    ! HIGHLIGHT, WRAP, MESSAGE, and PAGE=SAVE are taken and refused, for a
    ! later phase.
    verb help/id=1630
        qualifier   prompt/id=1632
        qualifier   instructions/id=1633
        qualifier   liblist/id=1634
        qualifier   library/id=1635             -
                    /type=$string
        qualifier   userlibrary/id=1636         -
                    /type=$string/list          -
                    /default=""
        qualifier   output/id=1637              -
                    /type=$string               -
                    /default=""
        qualifier   page/id=1638                -
                    /type=$string/list          -
                    /default=""
        qualifier   search/id=1639              -
                    /type=$string               -
                    /default=""
        qualifier   exact/id=1612
        qualifier   highlight/id=1613           -
                    /type=$string               -
                    /default=""
        qualifier   wrap/id=1614
        qualifier   message/id=1615
        parameter   topic/id=1631               -
                    /type=$rest_of_line

    ! IF expression [THEN [$] command], DCL's IF (docs/PHASE-50, subtask
    ! 12): the handler reads the expression and finds THEN. Without THEN,
    ! the IF starts a block of THEN, ELSE, and ENDIF lines.
    verb if/id=1640
        parameter   text/id=1641                -
                    /type=$rest_of_line
    verb then/id=1650
        parameter   command/id=1651             -
                    /type=$rest_of_line
    verb else/id=1652
        parameter   command/id=1653             -
                    /type=$rest_of_line
    verb endif/id=1654

    ! GOTO and GOSUB go to a label in a command procedure, and RETURN
    ! comes back from a GOSUB (subtasks 11 and 13).
    verb goto/id=1655
        parameter   label/id=1656               -
                    /type=$rest_of_line
    verb gosub/id=1657
        parameter   label/id=1658               -
                    /type=$rest_of_line
    verb return/id=1659
        parameter   status/id=1660              -
                    /type=$rest_of_line

    ! CALL[/OUTPUT=file] label [p1 ... p8] runs a subroutine of the same
    ! procedure, from its "label: SUBROUTINE" to its ENDSUBROUTINE, at a
    ! new command level. The handler reads the label and the parameters
    ! as @ reads its file and parameters.
    verb call/id=1661
        qualifier   output/id=1662              -
                    /type=$string
        parameter   text/id=1663                -
                    /type=$rest_of_line
    verb subroutine/id=1664
    verb endsubroutine/id=1665

    ! DECK makes the records after it data, "$" or not, up to its end of
    ! file indicator: an EOD command, or /DOLLARS's string. EOD ends a
    ! run of data lines (subtask 16; internal/dclshell/dclinput.go).
    verb deck/id=1666
        qualifier   dollars/id=1667             -
                    /type=$string/nonegatable   -
                    /default=""
    verb eod/id=1668

    ! DCL's file I/O (subtask 17; internal/dclshell/dclfile.go). OPEN
    ! opens a file and gives it a logical name, READ and WRITE read and
    ! write its records, and CLOSE closes it; SYS$INPUT, SYS$OUTPUT,
    ! SYS$ERROR, and SYS$COMMAND need no OPEN. /SHARE's keyword (READ or
    ! WRITE, WRITE when there is none) and each /ERROR and /END_OF_FILE
    ! label are read by the handlers. WRITE's data, after its qualifiers,
    ! is the rest of the line: expressions separated by commas. READ's
    ! /KEY, /INDEX, /MATCH, and /DELETE are for indexed files (Phase 88),
    ! /MATCH's keyword read by the handler. INQUIRE
    ! asks the terminal for a symbol's value; its prompt is the rest of
    ! the line too.
    verb open/id=2100
        qualifier   read/id=2101
        qualifier   write/id=2102
        qualifier   append/id=2103
        qualifier   share/id=2104               -
                    /type=$string/nonegatable   -
                    /default=""
        qualifier   error/id=2105               -
                    /type=$string/nonegatable
        parameter   logical_name/id=2106        -
                    /type=$string
        parameter   filespec/id=2107            -
                    /type=$string
        qualifier   host/id=2108                -
                    /parameter=filespec

    verb close/id=2110
        qualifier   error/id=2111               -
                    /type=$string/nonegatable
        qualifier   log/id=2112
        parameter   logical_name/id=2113        -
                    /type=$string

    verb read/id=2120
        qualifier   end_of_file/id=2121         -
                    /type=$string/nonegatable
        qualifier   error/id=2122               -
                    /type=$string/nonegatable
        qualifier   prompt/id=2123              -
                    /type=$string/nonegatable
        qualifier   lock/id=2124
        qualifier   time_out/id=2125            -
                    /type=$integer
        qualifier   key/id=2128                 -
                    /type=$string/nonegatable
        qualifier   index/id=2129               -
                    /type=$string/nonegatable
        qualifier   match/id=2118               -
                    /type=$string/nonegatable
        qualifier   delete/id=2119/nonegatable
        parameter   logical_name/id=2126        -
                    /type=$string
        parameter   symbol/id=2127              -
                    /type=$string

    verb write/id=2130
        qualifier   error/id=2131               -
                    /type=$string/nonegatable
        qualifier   symbol/id=2132/nonegatable
        qualifier   update/id=2133/nonegatable
        parameter   logical_name/id=2134        -
                    /type=$string
        parameter   data/id=2135                -
                    /type=$rest_of_line

    verb inquire/id=2140
        qualifier   global/id=2141/nonegatable
        qualifier   local/id=2142/nonegatable
        qualifier   punctuation/id=2143
        parameter   symbol/id=2144              -
                    /type=$string
        parameter   prompt/id=2145              -
                    /type=$rest_of_line

    ! RUN activates a VMS image. /NOINIT and /INIT override whether each
    ! shareable image's LIB$INITIALIZE runs; /DEBUG (/STEP, /BREAK) runs
    ! it under the debugger, stopped at its first instruction, and /NODEBUG
    ! doesn't (with neither, an image linked /DEBUG starts the debugger);
    ! /NOEXECUTE loads without running.
    verb run/id=1680
        parameter   file/id=1681                -
                    /type=$string               -
                    /prompt="File"
        qualifier   host/id=1682                -
                    /parameter=file
        qualifier   init/id=1683
        qualifier   debug/id=1684
        qualifier   step/alias=debug
        qualifier   break/alias=debug
        qualifier   execute/id=1685
    verb r/alias=run

    ! SPAWN [command] makes a subprocess running the subprocess CLI, as
    ! LIB$SPAWN does, and waits for it unless /NOWAIT. SPAWN/HOST runs the
    ! command as a program on the host instead (spawnhost.go).
    verb spawn/id=1690
        qualifier   wait/id=1691
        qualifier   input/id=1692               -
                    /type=$string
        qualifier   output/id=1693              -
                    /type=$string
        qualifier   process/id=1694             -
                    /type=$string
        qualifier   prompt/id=1695              -
                    /type=$string
        qualifier   symbols/id=1696
        qualifier   logical_names/id=1697
        qualifier   host/id=1699/nonegatable
        parameter   command/id=1698             -
                    /type=$rest_of_line

    ! ATTACH [process-name] or ATTACH/IDENTIFICATION=pid gives the
    ! terminal to another process of the job that waits for it (a SPAWN's
    ! parent, or a process that gave it away by ATTACH), and waits until
    ! it comes back, as DCL's ATTACH does.
    verb attach/id=1700
        qualifier   identification/id=1701      -
                    /type=$string
        parameter   process/id=1702             -
                    /type=$string

    ! ASM [file] assembles a file with the console's assembler, or with
    ! no file enters interactive assembler mode. A host file name with
    ! lowercase letters or a "/" must be quoted, as everywhere in DCL.
    ! The console accepts ASM only until the microkernel is in place (it
    ! is how STARTUP.COM loads kernel.asm); after that it is the debugger's
    ! command (debug.dcl).
    verb asm/id=1720
        parameter   file/id=1721                -
                    /type=$string
    verb assemble/alias=asm

    ! "@file [p1 ... p8]" runs a command procedure. DCL reads it before
    ! any verb, as the console's dispatcher does (internal/console/
    ! procedure.go): it has no verb here. INCLUDE, eVAX's version of it, is
    ! gone (docs/PHASE-50 - DCL command procedures.md).

    ! SAVE/ROM file and SAVE/NVRAM file write the ROM or NVRAM; LOAD
    ! reads one back. With LOAD's /NOERROR, a file that can't be opened
    ! loads nothing, and the file defaults to DEFAULT.ROM/DEFAULT.NVRAM.
    verb save/id=1740
        qualifier   rom/id=1741/nonegatable
        qualifier   nvram/id=1742/nonegatable
        parameter   file/id=1743                -
                    /type=$string
        disallow    rom and nvram

    verb load/id=1750
        qualifier   rom/id=1751/nonegatable
        qualifier   nvram/id=1752/nonegatable
        qualifier   error/id=1753
        parameter   file/id=1754                -
                    /type=$string
        disallow    rom and nvram

    ! SET keyword ...: the keyword picks the syntax. Docs/PHASE-42.md
    ! moved the machine's keywords (BREAK, STEP, TRACE, PSL, MODE, PTE, VM,
    ! BASE, ...) to the debugger's SET; VERBOSE takes NO, the rest don't.
    ! (SET name=value, the console's old symbol and register assignment,
    ! gave way to DCL's assignments, docs/PHASE-50.)
    type set_types
        keyword     radix               /syntax=set_radix/nonegatable
        keyword     debug               /syntax=set_debug/nonegatable
        keyword     dbg                 /syntax=set_debug/nonegatable
        keyword     verbose             /syntax=set_verbose
        keyword     verify              /syntax=set_verify/value
        keyword     prefix              /syntax=set_prefix
        keyword     default             /syntax=set_default/nonegatable
        keyword     prompt              /syntax=set_prompt/nonegatable/value
        keyword     on                  /syntax=set_on
        keyword     control             /syntax=set_control/value
        keyword     quantum             /syntax=set_quantum/nonegatable
        keyword     working_set         /syntax=set_working_set/nonegatable
        keyword     editor              /syntax=set_editor/nonegatable
        ! PROTECTION and PROCESS (docs/planning/PHASE-93 - file
        ! protection.md, subtask 5): SET PROTECTION=(code)/DEFAULT, the
        ! process's default file protection, and SET PROCESS/PRIVILEGES.
        keyword     protection          /syntax=set_protection/nonegatable/value
        keyword     process             /syntax=set_process/nonegatable
        ! TERMINAL (docs/planning/PHASE-94 - privileges.md, subtask 6): a
        ! terminal's characteristics, /PERMANENT with LOG_IO or PHY_IO.
        keyword     terminal            /syntax=set_terminal/nonegatable
        ! SECURITY, FILE, and ACL (docs/planning/PHASE-93 - file
        ! protection.md, subtask 11): a file's owner, protection, and ACL,
        ! and VMS's obsolete forms of the same changes.
        keyword     security            /syntax=set_security/nonegatable
        keyword     file                /syntax=set_file/nonegatable
        keyword     acl                 /syntax=set_acl/nonegatable
        ! QUEUE (docs/planning/PHASE-98 - batch queues.md): a batch
        ! queue's attributes.
        keyword     queue               /syntax=set_queue/nonegatable
        ! ENTRY (docs/planning/PHASE-98 - batch queues.md, subtask 7): a
        ! job's options, held or released.
        keyword     entry               /syntax=set_entry/nonegatable
        ! RESTART_VALUE (subtask 10): SET RESTART_VALUE=string, the label
        ! a batch job's restart goes on from.
        keyword     restart_value       /syntax=set_restart_value/nonegatable/value


    verb set/id=1760
        parameter   what/id=1764                -
                    /type=set_types             -
                    /prompt="What"

    ! SET QUANTUM value
    syntax set_quantum/id=1801
           parameter   count/id=1802               -
                       /type=$integer              -
                       /prompt="Quantum"
  
    ! SET WORKING_SET [/LIMIT=n] [/QUOTA=n] [/EXTENT=n] [/[NO]ADJUST]
    ! [/[NO]LOG]: the process's own working set values, within what it
    ! was authorized (docs/PHASE-63, subtask 9).
    syntax set_working_set/id=1820
        qualifier   limit/id=1821/nonegatable   -
                    /type=$integer
        qualifier   quota/id=1822/nonegatable   -
                    /type=$integer
        qualifier   extent/id=1823/nonegatable  -
                    /type=$integer
        qualifier   adjust/id=1824
        qualifier   log/id=1825

    ! SET PROTECTION[=(code)]/DEFAULT: the code merged into the process's
    ! default file protection, which a new file gets when nothing else
    ! gives it one (internal/dclcmd/protection.go). Without /DEFAULT, SET
    ! PROTECTION[=(code)] file changes a file's protection, VMS's obsolete
    ! form (internal/dclcmd/security.go); with no code, the file gets the
    ! process's default protection.
    syntax set_protection/id=2700
        parameter   code/id=2701                -
                    /type=$protection
        parameter   file/id=2702                -
                    /type=$string
        qualifier   default/id=2703/nonegatable

    ! SET SECURITY file: a file's profile changed, one specification
    ! (wildcards allowed; a list is DCL's NOLIST, as on VMS 7.1), with
    ! control access (internal/dclcmd/security.go). /PROTECTION merges a
    ! code into the file's; /OWNER names a UIC or an identifier; /DEFAULT
    ! gives the process's default protection and UIC and keeps only the
    ! protected ACEs; /LIKE=NAME=file copies another file's protection and
    ! ACL (and with /COPY_ATTRIBUTE=OWNER its owner); /ACL adds entries,
    ! and with /AFTER, /REPLACE, or /DELETE[=ALL] places, replaces, or
    ! deletes them. /EXCLUDE and /BY_OWNER choose among the files.
    syntax set_security/id=2730
        parameter   file/id=2731                -
                    /type=$string/single        -
                    /prompt="Object"
        qualifier   protection/id=2732/nonegatable  -
                    /type=$protection
        qualifier   owner/id=2733/nonegatable   -
                    /type=$uic
        qualifier   default/id=2734/nonegatable
        qualifier   like/id=2735/nonegatable    -
                    /type=$any/list
        qualifier   copy_attribute/id=2736/nonegatable -
                    /type=$any/list
        qualifier   acl/id=2737/nonegatable     -
                    /type=$acl/default=""
        qualifier   after/id=2738/nonegatable   -
                    /type=$acl
        qualifier   replace/id=2739/nonegatable -
                    /type=$acl
        qualifier   delete/id=2740/nonegatable  -
                    /type=$any/default=""
        qualifier   log/id=2741
        qualifier   exclude/id=2742/nonegatable -
                    /type=$string/list
        qualifier   by_owner/id=2743/nonegatable -
                    /type=$uic
        qualifier   class/id=2744/nonegatable   -
                    /type=$any

    ! SET FILE file[,...]: VMS's obsolete forms, /PROTECTION[=(code)] (no
    ! code is the process's default protection) and /OWNER_UIC[=uic] (no
    ! UIC is the process's), on SET SECURITY's code.
    syntax set_file/id=2750
        parameter   files/id=2751               -
                    /type=$string/list          -
                    /prompt="File"
        qualifier   protection/id=2752/nonegatable  -
                    /type=$protection/default=""
        qualifier   owner_uic/id=2753/nonegatable   -
                    /type=$uic/default=""
        qualifier   log/id=2754

    ! SET ACL/ACL=(ace[,...]) file: VMS's obsolete form of SET
    ! SECURITY/ACL, with its /AFTER, /REPLACE, /DELETE, /LIKE, and /DEFAULT.
    syntax set_acl/id=2760
        parameter   file/id=2761                -
                    /type=$string/single        -
                    /prompt="Object"
        qualifier   acl/id=2762/nonegatable     -
                    /type=$acl/default=""
        qualifier   after/id=2763/nonegatable   -
                    /type=$acl
        qualifier   replace/id=2764/nonegatable -
                    /type=$acl
        qualifier   delete/id=2765/nonegatable  -
                    /type=$any/default=""
        qualifier   like/id=2766/nonegatable    -
                    /type=$any/list
        qualifier   default/id=2767/nonegatable
        qualifier   log/id=2768

    ! SET PROCESS/PRIVILEGES=(name[,...]): privileges enabled, or with NO
    ! before a name disabled, for the process, as $SETPRV makes them
    ! permanent (internal/dclcmd/privileges.go). SET PROCESS [name]
    ! [/IDENTIFICATION=pid] /PRIORITY=n, /SUSPEND, /RESUME: another
    ! process changed as $SETPRI, $SUSPND, and $RESUME change it, by their
    ! rule, GROUP or WORLD (internal/dclcmd/setprocess.go,
    ! docs/planning/PHASE-94 - privileges.md, subtask 8).
    syntax set_process/id=2710
        parameter   process_name/id=2716            -
                    /type=$string
        qualifier   privileges/id=2711/nonegatable  -
                    /type=$any/list
        qualifier   identification/id=2712/nonegatable -
                    /type=$string
        qualifier   priority/id=2713/nonegatable    -
                    /type=$integer
        qualifier   suspend/id=2714/nonegatable
        qualifier   resume/id=2715/nonegatable
        disallow    suspend and resume

    ! SET TERMINAL [device]: the terminal's characteristics the qualifiers
    ! name, the rest unchanged (internal/dclcmd/terminal.go); the device is
    ! SYS$COMMAND's when none is given. /PERMANENT needs LOG_IO or PHY_IO.
    ! Each characteristic's qualifier is a bit of the terminal's
    ! characteristics (dclcmd.TerminalSwitches); UPPERCASE and HARDCOPY are
    ! LOWERCASE's and SCOPE's opposites.
    syntax set_terminal/id=2800
        parameter   device/id=2801              -
                    /type=$string
        qualifier   width/id=2802/nonegatable   -
                    /type=$integer
        qualifier   page/id=2803/nonegatable    -
                    /type=$integer
        qualifier   device_type/id=2804/nonegatable -
                    /type=$any
        qualifier   permanent/id=2805
        qualifier   advanced_video/id=2806
        qualifier   altypeahd/id=2807
        qualifier   ansi_crt/id=2808
        qualifier   application_keypad/id=2809
        qualifier   block_mode/id=2810
        qualifier   brdcstmbx/id=2811
        qualifier   broadcast/id=2812
        qualifier   dec_crt/id=2813
        qualifier   dec_crt2/id=2814
        qualifier   disconnect/id=2815
        qualifier   dma/id=2816
        qualifier   echo/id=2817
        qualifier   edit_mode/id=2818
        qualifier   eight_bit/id=2819
        qualifier   escape/id=2820
        qualifier   form/id=2821
        qualifier   hangup/id=2822
        qualifier   hardcopy/id=2823/nonegatable
        qualifier   hostsync/id=2824
        qualifier   insert/id=2825
        qualifier   line_editing/id=2826
        qualifier   local_echo/id=2827
        qualifier   lowercase/id=2828
        qualifier   modem/id=2829
        qualifier   pasthru/id=2830
        qualifier   printer_port/id=2831
        qualifier   readsync/id=2832
        qualifier   regis/id=2833
        qualifier   scope/id=2834
        qualifier   sixel_graphics/id=2835
        qualifier   tab/id=2836
        qualifier   ttsync/id=2837
        qualifier   type_ahead/id=2838
        qualifier   uppercase/id=2839/nonegatable
        qualifier   wrap/id=2840

    ! SET EDITOR [/COMMAND="host command"] [/SCRATCH="host directory"]: the
    ! editor EDIT runs, and where it works (internal/dclcmd/edit.go).
    syntax set_editor/id=2250
        qualifier   command/id=2251/nonegatable -
                    /type=$string
        qualifier   scratch/id=2252/nonegatable -
                    /type=$string

    ! SET RADIX 8|10|16|HEX|DEC.
    syntax set_radix/id=1773
        parameter   radix/id=1774               -
                    /type=$any                  -
                    /prompt="Radix"

    ! SET DEBUG [flag[,flag...]]: a flag with NO clears it; no flags sets
    ! NATIVE.
    syntax set_debug/id=1785
        parameter   flags/id=1786               -
                    /type=$any/list

    syntax set_verbose/id=1799

    ! SET ON and SET NOON: whether a command procedure acts on its
    ! commands' statuses (docs/PHASE-50, subtask 10).
    syntax set_on/id=1812
    ! SET [NO]CONTROL[=(T,Y)]: whether Ctrl/Y (and Ctrl/T) do anything
    ! (docs/PHASE-50, subtask 18).
    syntax set_control/id=1816
        parameter   keywords/id=1817            -
                    /type=$any/list
    ! SET [NO]VERIFY [=([NO]PROCEDURE, [NO]IMAGE)] and SET [NO]PREFIX
    ! "string": a command procedure's lines shown as they're read, and
    ! the prefix before them (docs/PHASE-50, subtask 15).
    syntax set_verify/id=1800
        parameter   keywords/id=1813            -
                    /type=$any/list
    syntax set_prefix/id=1814
        parameter   text/id=1815                -
                    /type=$string


    ! SET DEFAULT device:[directory], the default for file names
    ! (docs/PHASE-23.md).
    syntax set_default/id=1804
        parameter   spec/id=1805                -
                    /type=$string               -
                    /prompt="Directory"

    ! SET PROMPT="text", the console's prompt (and the vax.console.prompt
    ! setting). PROMPT takes its value after "=", so SET PROMPT="X" isn't
    ! an assignment to a symbol named PROMPT.
    ! SET RESTART_VALUE=string (docs/planning/PHASE-98 - batch
    ! queues.md, subtask 10): BATCH$RESTART for a batch job's restart.
    syntax set_restart_value/id=3184
        parameter   text/id=3185                -
                    /type=$string               -
                    /prompt="Value"

    syntax set_prompt/id=1810
        parameter   text/id=1811                -
                    /type=$string               -
                    /prompt="Prompt"

end

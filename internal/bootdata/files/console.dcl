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

    syntax show_memory/id=215
        qualifier   statistics/id=216
        qualifier   runtime/id=217
        qualifier   full/id=218

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
        qualifier user_mode/id=243
        qualifier supervisor_mode/id=244
        qualifier executive_mode/id=245
        qualifier log/id=246
        disallow any2(user_mode, supervisor_mode, executive_mode)

    !
    ! docs/PHASE-34.md: CREATE/DIRECTORY makes a directory on a mounted
    ! volume, and every missing directory above it (internal/rms.Session.
    ! CreateDirectory, internal/console/create.go). OWNER_UIC is a UIC or
    ! PARENT; PROTECTION is a list of category:access items.
    !
    syntax create_directory
        parameter directories/id=1501/type=$string/list/prompt="Directory"
        qualifier owner_uic/id=1502/type=$any
        qualifier version_limit/id=1503/type=$integer
        qualifier protection/id=1504/type=$any/list
        qualifier allocation/id=1505/type=$integer
        qualifier log/id=1506

    verb create
        qualifier name_table/syntax=create_name_table
        qualifier directory/syntax=create_directory

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
        
    type debug_types
        keyword     debug/id=5003

    verb about /entry=exe$about

    ! EXIT [status] ends the command procedure it's in, or govax at the
    ! terminal; the status, a DCL expression, is the procedure's. QUIT
    ! ends govax wherever it is.
    verb exit
        parameter   status/id=1645              -
                    /type=$rest_of_line

    verb quit

    ! ON condition THEN [$] command: what a command procedure does when a
    ! command fails with the condition's severity or worse, or on Ctrl/Y
    ! (docs/PHASE-50, subtask 10). CONTINUE does nothing, and goes on.
    type on_conditions
        keyword     warning/id=1
        keyword     error/id=2
        keyword     severe_error/id=3
        keyword     control_y/id=4

    verb on/id=1646
        parameter   condition/id=1647           -
                    /type=on_conditions         -
                    /prompt="Condition"
        parameter   command/id=1648             -
                    /type=$rest_of_line         -
                    /prompt="Command"

    verb continue/id=1649

    ! DEBUG starts a debugger session on the machine as it stands, with
    ! nothing running (docs/PHASE-42.md). It is how to look at memory and
    ! registers now that EXAMINE and SHOW REGISTERS are debugger commands.
    verb debug/id=6000
    
    verb test
    
        parameter   what/id=161                 -
                    /type=$rest_of_line         -
                    /prompt="Code"


    ! GO and CALL are the debugger's (debug.dcl); ASM is there too.

    
    ! CLEAR (docs/PHASE-42.md): what is left of it at the console. The
    ! debugger's CANCEL took breakpoints, interrupts, the translation
    ! buffer, memory statistics, and the machine's symbol table (CLEAR
    ! SYMBOL); CLEAR ERROR and CLEAR PROFILES had no handler and are gone.
    ! A DCL symbol is deleted with DELETE/SYMBOL, as in DCL.
    type clear_types
        keyword     memory              /syntax=clear_memory
        keyword     strings             /syntax=clear_strings

    verb clear

        parameter   CLEAR_TYPE/type=clear_types/prompt="What"

        syntax clear_strings/id=105
        syntax clear_memory/id=103


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
        keyword         xtest           /syntax=xtest
        keyword         string_pool     /syntax=show_string
        keyword         nvram           /syntax=show_nvram      
        keyword         quantum         /syntax=show_quantum
        keyword         debug           /syntax=show_debug
        keyword         instructions    /syntax=show_instructions
        keyword         radix           /syntax=show_radix
        keyword         symbols         /syntax=show_sym
        keyword         rom             /syntax=show_rom
        keyword         share_prefix    /syntax=show_share
        keyword         expand          /syntax=show_expand
        ! SYSTEM and PROCESS (docs/PHASE-44.md, subtask 10): the process
        ! table and one process, as VMS's DCL shows them.
        keyword         system          /syntax=show_system
        keyword         process         /syntax=show_process
                

	verb vminit

            qualifier       p0 /id=1801 -
		                /type=$integer

	    qualifier       p1 /id=1802 -
		                /type=$integer

	    qualifier       s0 /id=1803 -
		                /type=$integer

            qualifier       ksp /id=1804 -
                                /type=$integer -
                                /default=4

            qualifier       esp /id=1805 -
                                /type=$integer -
                                /default=8

            qualifier       ssp /id=1806 -
                                /type=$integer -
                                /default=8

            qualifier       isp /id=1807 -
                                /type=$integer -
                                /default=4

            qualifier       stringpool /id=1808 -
			        /type=$integer -
                                /default=8

            qualifier       debug /id=1809
            qualifier       verify /alias=debug
            qualifier       log /alias=debug

    ! SHOW (docs/PHASE-42.md): the console's keywords. The machine's
    ! (registers, memory, breakpoints, the stack, ...) are the debugger's.
    ! SHOW ASSEMBLER_FLAGS, COMMAND_ARGS, ERROR, and SHOW SYMBOL/TEMPORARY and
    ! /UNRESOLVED had no handler, and are gone; SHOW SYMBOL/SYSTEM and the
    ! machine's SHOW SYMBOL/ALL are the debugger's now.
    verb show/id=120

        parameter       SHOW_TYPE/id=160            -
                        /type=show_types            -
                        /prompt="What"

        syntax xtest/entry=exe$xtest /id=5001
            parameter   debug /type=debug_types /id = 5002
            qualifier   code/type=$integer /id=5005/nonegatable/default=101
            parameter   name/type=$any/id=5004

        syntax          show_expand/id=188
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
        syntax          show_string/id=121
        syntax          show_nvram/id=122
        syntax          show_quantum/id=128
        syntax          show_debug/id=129
        syntax          show_instructions/id=131
            qualifier   modes/id=1009
            qualifier   profile/id=1010
            qualifier   unimplemented/id=1011
            qualifier   all/id=1012
            parameter   opcode/id=1013              -
                        /type=$rest_of_line
            disallow    modes and profile
            disallow    modes and unimplemented
            disallow    modes and all
            disallow    unimplemented and all
            disallow    unimplemented and profile
            disallow    profile and all
        syntax          show_version/entry=exe$about
        syntax          show_share/id=161

        ! DEFAULT (docs/PHASE-23.md, subtask 3) has no reference/eVAX or
        ! testdata/dcl/console.dcl counterpart -- see this file's own
        ! "govax-native extension" comment at MOUNT's definition below.
        syntax          show_default/id=163

        ! SHOW SYSTEM and SHOW PROCESS [name] [/IDENTIFICATION=pid]
        ! (docs/PHASE-44.md, subtask 10); internal/console/showsys.go.
        syntax          show_system/id=164
        syntax          show_process/id=165
            qualifier   identification/id=1940      -
                        /type=$string
            parameter   process_name/id=1941        -
                        /type=$string


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
        parameter   file/id=702                 -
                    /type=$string               -
                    /prompt="Container file"
        qualifier   write/id=703

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
                    /type=$string
        qualifier   full/id=902
        qualifier   file/id=903
        qualifier   size/id=904
        qualifier   date/id=905
        qualifier   owner/id=906
        qualifier   protection/id=907
        qualifier   versions/id=908             -
                    /type=$integer

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
    ! required argument. The further rule that the spec name a *specific*
    ! version (";3" or ";*", never defaulted) is enforced by
    ! internal/rms.Session.Delete itself, not by this grammar -- see
    ! docs/PHASE-23.md's subtask 6.
    !
    verb delete/id=1100

        parameter   spec/id=1101               -
                    /type=$string               -
                    /prompt="File specification"

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
    verb purge/id=1150

        parameter   spec/id=1151                -
                    /type=$string
        qualifier   limit/id=1152               -
                    /type=$integer

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
    ! between screens (internal/console/page.go).
    !
    verb type/id=1200

        parameter   spec/id=1201                -
                    /type=$string               -
                    /prompt="File specification"
        qualifier   page/id=1202

    !
    ! govax-native extension (Phase 23, internal/console/dcl + internal/rms
    ! + internal/console): COPY has no reference/eVAX or testdata/dcl/
    ! console.dcl counterpart at all -- see this file's own "govax-native
    ! extension" comment at MOUNT's own definition above. Unlike every
    ! other verb in this block, SOURCE and DESTINATION each carry their
    ! own private HOST qualifier (via /parameter=, internal/console/dcl's
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
                    /type=$string               -
                    /prompt="Source"
        qualifier   host/id=1252                -
                    /parameter=source

        parameter   destination/id=1253         -
                    /type=$string               -
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
        qualifier   crlf/id=1264
        qualifier   lf/id=1265
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
    ! ahead of STARLET.MLB: govax's form of VMS's "PROG+LIB/LIBRARY".
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
                    /type=$string               -
                    /prompt="Source"
        qualifier   host/id=1302                -
                    /parameter=source

        qualifier   object/id=1303              -
                    /type=$string               -
                    /default=""
        qualifier   library/id=1304             -
                    /type=$string/list
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
    ! MAP), and /BRIEF makes the map brief. LIBRARY, INCLUDE,
    ! SELECTIVE_SEARCH, and OPTIONS are positional: each belongs to the
    ! file it follows ("LINK MAIN,MYLIB/LIBRARY,PROG/OPTIONS").
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
        qualifier   traceback/id=1354
        qualifier   debug/id=1362               -
                    /type=$string               -
                    /default=""
        qualifier   syslib/id=1355
        qualifier   map/id=1356                 -
                    /type=$string               -
                    /default=""
        qualifier   brief/id=1357

    !
    ! docs/PHASE-38.md (internal/anl, internal/console/analyze.go):
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

    verb analyze/id=1900
        qualifier object/syntax=analyze_object
        qualifier image/syntax=analyze_image

    !
    ! govax-native extension (docs/PHASE-28.md subtask 7, internal/console
    ! + internal/lbr): LIBRARY creates, changes, extracts from, and lists
    ! macro (.MLB) and object (.OLB) libraries, in the style of VMS's
    ! LIBRARIAN. LIBRARY and INPUTS each carry their own HOST qualifier,
    ! as COPY's parameters do. With input files and no operation named,
    ! they replace modules of the same names (LIBRARIAN's default
    ! /REPLACE). DELETE and EXTRACT take module names, which may hold * and
    ! %. LIST is /LIST[=file] (empty default: the console). MACRO or OBJECT
    ! picks a new library's type; SQUEEZE is on unless /NOSQUEEZE.
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

        qualifier   create/id=1405
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
        disallow    macro and object
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
    verb zero/id=1600

    ! STOP deletes a process (and its subprocesses): by name, or by its
    ! hexadecimal PID with /IDENTIFICATION, as SHOW PROCESS names one.
    verb stop/id=1605
        qualifier   identification/id=1606      -
                    /type=$string
        parameter   process_name/id=1607        -
                    /type=$string

    verb boot/id=1601
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

    ! HELP's topic words are the help file's to match, so they're the
    ! rest of the line ("HELP SHOW SYMBOL/ALL").
    verb help/id=1630
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
    ! LIB$SPAWN does, and waits for it unless /NOWAIT.
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
        parameter   command/id=1698             -
                    /type=$rest_of_line

    ! ASM [file] assembles a file with the console's assembler, or with
    ! no file enters interactive assembler mode. A host file name with
    ! lowercase letters or a "/" must be quoted, as everywhere in DCL.
    ! The console accepts ASM only until the microkernel is in place (it
    ! is how vax.init loads kernel.asm); after that it is the debugger's
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

    ! SET (console_set.c). SET name=value assigns a symbol, register, or
    ! privileged register; console_set.c looks for that form first, so a
    ! name that abbreviates a keyword (SET R=5) is still an assignment.
    ! (The debugger's DEPOSIT does the same for registers, with VMS's
    ! syntax.) Otherwise the keyword picks the syntax. Docs/PHASE-42.md
    ! moved the machine's keywords (BREAK, STEP, TRACE, PSL, MODE, PTE, VM,
    ! BASE, ...) to the debugger's SET; VERBOSE takes NO, the rest don't.
    type set_types
        keyword     radix               /syntax=set_radix/nonegatable
        keyword     debug               /syntax=set_debug/nonegatable
        keyword     dbg                 /syntax=set_debug/nonegatable
        keyword     verbose             /syntax=set_verbose
        keyword     verify              /syntax=set_verify/value
        keyword     prefix              /syntax=set_prefix
        keyword     quantum             /syntax=set_quantum/nonegatable
        keyword     uiquantum           /syntax=set_uiquantum/nonegatable
        keyword     default             /syntax=set_default/nonegatable
        keyword     prompt              /syntax=set_prompt/nonegatable/value
        keyword     on                  /syntax=set_on


    verb set/id=1760/assignment=set_symbol
        qualifier   permanent/id=1761/nonegatable
        qualifier   prm/alias=permanent
        qualifier   entry/id=1762/nonegatable
        qualifier   label/id=1763/nonegatable
        qualifier   lbl/alias=label
        parameter   what/id=1764                -
                    /type=set_types             -
                    /prompt="What"

    ! SET [/PERMANENT] [/ENTRY] [/LABEL] name=value.
    syntax set_symbol/id=1770
        qualifier   permanent/id=1761/nonegatable
        qualifier   prm/alias=permanent
        qualifier   entry/id=1762/nonegatable
        qualifier   label/id=1763/nonegatable
        qualifier   lbl/alias=label
        parameter   name/id=1771                -
                    /type=$name                 -
                    /separator="="              -
                    /prompt="Name"
        parameter   value/id=1772               -
                    /type=$expression           -
                    /prompt="Value"

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
    ! SET [NO]VERIFY [=([NO]PROCEDURE, [NO]IMAGE)] and SET [NO]PREFIX
    ! "string": a command procedure's lines shown as they're read, and
    ! the prefix before them (docs/PHASE-50, subtask 15).
    syntax set_verify/id=1800
        parameter   keywords/id=1813            -
                    /type=$any/list
    syntax set_prefix/id=1814
        parameter   text/id=1815                -
                    /type=$string

    syntax set_quantum/id=1801
        parameter   count/id=1802               -
                    /type=$integer              -
                    /prompt="Quantum"
    syntax set_uiquantum/id=1803
        parameter   count/id=1802               -
                    /type=$integer              -
                    /prompt="Quantum"

    ! SET DEFAULT device:[directory], the default for file names
    ! (docs/PHASE-23.md).
    syntax set_default/id=1804
        parameter   spec/id=1805                -
                    /type=$string               -
                    /prompt="Directory"

    ! SET PROMPT="text", the console's prompt (and the vax.console.prompt
    ! setting). PROMPT takes its value after "=", so SET PROMPT="X" isn't
    ! an assignment to a symbol named PROMPT.
    syntax set_prompt/id=1810
        parameter   text/id=1811                -
                    /type=$string               -
                    /prompt="Prompt"

end

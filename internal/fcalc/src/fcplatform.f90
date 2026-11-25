! fcalc: what differs from one system to another, VMS's version. Every other
! file is the same source on VMS and under gfortran, whose twin of this file
! is gfortran/fcplatform.f90. Keeping what differs in one small module is
! the usual way to write a portable program: the build picks the module's
! version for the system, and nothing else changes.
module fcplatform
  implicit none
  public

  ! VMS's FORTRAN treats the first character of each record written to the
  ! terminal as carriage control, a blank meaning "on a new line", as the
  ! standard's printing rule (9.4.5) allows. Each line fcalc writes starts
  ! with CC, so a blank here and nothing in the twin print the same lines.
  ! LEN=* takes the constant's length from its value: here, 1.
  character(len=*), parameter :: cc = ' '

  ! A record written to the terminal holds 133 bytes, the carriage control
  ! among them: RECORD_ROOM is what is left for text.
  integer, parameter :: record_room = 132

contains

  ! PROMPT_ENDS says whether a READ ends the record a prompt left open
  ! (module FCMESSAGES's SAY_MORE). VMS's FORTRAN writes such a record as the
  ! READ's prompt when it reads a terminal, and the next line is a record of
  ! its own; reading anything else (a file, in a command procedure), it
  ! leaves the record open, and the next WRITE goes on with it, as Fortran
  ! 90's nonadvancing output says it should. So the answer is whether
  ! SYS$INPUT is a terminal, which LIB$GETDVI tells: its arguments are the
  ! item code (DVI$_TRM, 74 in VMS's $DVIDEF), no channel (a null argument
  ! again), the device's name, and a longword for the answer, whose low bit
  ! is set for a terminal. A SAVE'd variable keeps the answer, so VMS is
  ! asked once.
  logical function prompt_ends()
    integer, parameter :: dvi_trm = 74
    integer, save :: known = 0 ! 0 not yet asked, 1 a terminal, 2 not
    integer :: status, terminal
    integer, external :: lib$getdvi

    if (known == 0) then
      terminal = 0
      status = lib$getdvi(dvi_trm, , 'SYS$INPUT', terminal)
      known = 2
      if (btest(status, 0) .and. btest(terminal, 0)) known = 1
    end if
    prompt_ends = known == 1
  end function prompt_ends

  ! FOREIGN_TEXT gives the text typed after fcalc's name when it is run as
  ! a foreign command,
  !
  !     $ FCALC == "$SYS$SYSTEM:FCALC"
  !     $ FCALC 2^10 + 1
  !
  ! TEXT(:LENGTH), with LENGTH 0 when there is none (RUN FCALC, or FCALC
  ! alone). DCL keeps the text, and LIB$GET_FOREIGN, a routine of VMS's
  ! run-time library, gives it to the program: its arguments are the text
  ! (a CHARACTER variable, which VMS passes by descriptor), a prompt for
  ! when there is none (left out here: the two commas with nothing between
  ! them are VMS's null argument, so LIB$GET_FOREIGN asks for nothing),
  ! and the text's length, a word, INTEGER(2). Its value is a VMS status,
  ! odd for success, which BTEST(STATUS, 0) tests. Calling a run-time
  ! library routine is no different from calling a Fortran function:
  ! EXTERNAL says the name is a procedure outside the program, and its type
  ! is the type of its value.
  subroutine foreign_text(text, length)
    character(len=*), intent(out) :: text
    integer, intent(out) :: length
    integer :: status
    integer(2) :: short
    integer, external :: lib$get_foreign

    text = ' '
    short = 0
    status = lib$get_foreign(text, , short)
    length = short
    if (.not. btest(status, 0)) length = 0
  end subroutine foreign_text

  ! SHOW_HELP shows the help text REQUEST names, "FCALC" and the topics
  ! after it, and is false if there is none to show. fcalc's help is the
  ! FCALC topic of the system's help library, SYS$HELP:HELPLIB.HLB, the text
  ! DCL's HELP FCALC shows, so it is written once, in VMS's help format, and
  ! the library is its one copy. LBR$OUTPUT_HELP, a routine of VMS's
  ! librarian, finds the topics and lays the text out, as HELP does: its
  ! arguments are a routine to write each line, the line's width (left out:
  ! 80), the request, the library (left out: SYS$HELP:HELPLIB), and flags
  ! (0: no prompting for more topics, and no user libraries). It doesn't
  ! write the lines itself: it calls WRITER with each, as a CHARACTER
  ! argument, so they go where all of fcalc's output goes (module
  ! FCMESSAGES's HELP_LINE). A procedure passed as an argument this way is
  ! a dummy procedure: INTEGER, EXTERNAL says WRITER is a function, and
  ! here it is passed on to LBR$OUTPUT_HELP as it came.
  logical function show_help(request, writer)
    character(len=*), intent(in) :: request
    integer, external :: writer
    integer, parameter :: no_flags = 0
    integer :: status
    integer, external :: lbr$output_help

    status = lbr$output_help(writer, , request, , no_flags)
    show_help = btest(status, 0)
  end function show_help
  ! OPEN_TEXT opens NAME on UNIT to write text: a file of fcalc's own, a
  ! SAVE or a LOG, for a person to read and edit as well as for fcalc to
  ! read back. STATUS is OPEN's STATUS= and POSITION its POSITION=, and IOS
  ! is IOSTAT='s answer. VMS's FORTRAN gives a formatted sequential file
  ! FORTRAN carriage control by default: each record's first character is
  ! taken as a printer's control, not text, so TYPE would show every line
  ! without it. CARRIAGECONTROL='LIST', a VAX FORTRAN keyword, makes the
  ! file an ordinary one of lines, every character text.
  subroutine open_text(unit, name, status, position, ios)
    integer, intent(in) :: unit
    character(len=*), intent(in) :: name, status, position
    integer, intent(out) :: ios

    open (unit=unit, file=name, status=status, action='WRITE', &
      position=position, carriagecontrol='LIST', iostat=ios)
  end subroutine open_text
end module fcplatform

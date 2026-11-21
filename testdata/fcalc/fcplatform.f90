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

contains

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
end module fcplatform

! fcalc: what differs from one system to another, gfortran's version (see
! ../fcplatform.f90, VMS's).
module fcplatform
  implicit none
  public

  ! gfortran prints the first character of a record as text: no carriage
  ! control.
  character(len=*), parameter :: cc = ''

  ! gfortran's records have no limit that fcalc's lines reach.
  integer, parameter :: record_room = huge(1)

contains

  ! PROMPT_ENDS: gfortran leaves a prompt's record open whatever it reads,
  ! a terminal or not, and the next WRITE goes on with it.
  logical function prompt_ends()
    prompt_ends = .false.
  end function prompt_ends

  ! FOREIGN_TEXT: Fortran 95 has no way to read a program's command line
  ! (Fortran 2003's GET_COMMAND is one), so under gfortran there is never
  ! any text, and fcalc always reads lines at its prompt.
  subroutine foreign_text(text, length)
    character(len=*), intent(out) :: text
    integer, intent(out) :: length

    text = ' '
    length = 0
  end subroutine foreign_text

  ! SHOW_HELP: fcalc's help is a topic of VMS's help library, which gfortran's
  ! systems don't have, so there is none to show.
  logical function show_help(request, writer)
    character(len=*), intent(in) :: request
    integer, external :: writer

    show_help = .false.
  end function show_help
  ! OPEN_TEXT opens NAME on UNIT to write text (a SAVE or a LOG): gfortran's
  ! files have no carriage control to choose.
  subroutine open_text(unit, name, status, position, ios)
    integer, intent(in) :: unit
    character(len=*), intent(in) :: name, status, position
    integer, intent(out) :: ios

    open (unit=unit, file=name, status=status, action='WRITE', &
      position=position, iostat=ios)
  end subroutine open_text
end module fcplatform

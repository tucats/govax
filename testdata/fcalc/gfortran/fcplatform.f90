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
end module fcplatform

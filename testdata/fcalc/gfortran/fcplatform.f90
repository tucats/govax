! fcalc: what differs from one system to another, gfortran's version (see
! ../fcplatform.f90, VMS's).
module fcplatform
  implicit none
  public

  ! gfortran prints the first character of a record as text: no carriage
  ! control.
  character(len=*), parameter :: cc = ''

contains

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

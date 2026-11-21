! fcalc: what differs from one system to another, gfortran's version (see
! ../fcplatform.f90, VMS's).
module fcplatform
  implicit none
  public

  ! gfortran prints the first character of a record as text: no carriage
  ! control.
  character(len=*), parameter :: cc = ''
end module fcplatform

! fcalc: what differs from one system to another, VMS's version. Every other
! file is the same source on VMS and under gfortran, whose twin of this file
! is gfortran/fcplatform.f90.
module fcplatform
  implicit none
  public

  ! VMS's FORTRAN treats the first character of each record written to the
  ! terminal as carriage control, a blank meaning "on a new line", as the
  ! standard's printing rule (9.4.5) allows. Each line fcalc writes starts
  ! with CC, so a blank here and nothing in the twin print the same lines.
  character(len=*), parameter :: cc = ' '
end module fcplatform

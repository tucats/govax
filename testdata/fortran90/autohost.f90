! autohost.f90: an internal procedure of a host with AUTOMATIC variables
! (docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md, subtask 4):
! STEP reaches its host's K and W in the host's frame through the FP the host
! passes it, and its FORMAT's variable format expression reads W there too, run
! by FTNRTL in the middle of the WRITE. DEC's AUTOMATIC and <W> keep it from
! gfortran.
program autohost
  call autos(4)
end program autohost

subroutine autos(n)
  integer n, k, w
  automatic k, w
  k = 0
  w = 3
  do i = 1, n
    call step(i)
  end do
  write (*, '(1x, a, i4)') 'k:', k
contains
  subroutine step(j)
    integer j
    k = k + j * j
    write (*, 100) k
100 format (1x, 'k=', i<w>)
  end subroutine step
end subroutine autos

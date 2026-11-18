! hostopt.f90: the optimizer beside internal procedures
! (docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md, subtask
! 9). Each loop below holds a value the optimizer would like to keep in a
! register, and each calls an internal procedure that changes it: a host
! variable in static storage (K), a host's dummy argument reached through its
! cell (SUMUP's N), a RECURSIVE host's variable in its frame (DEEP's LEVEL), and
! a host's OPTIONAL dummy, asked about with PRESENT in the loop. Every mode must
! print what the program means.
program hostopt
  interface
    subroutine sumup(n, total)
      integer, intent(inout) :: n
      integer, intent(out) :: total
    end subroutine sumup
    recursive subroutine deep(level, out)
      integer, intent(in) :: level
      integer, intent(inout) :: out
    end subroutine deep
    subroutine maybe(count, step)
      integer, intent(in) :: count
      integer, intent(in), optional :: step
    end subroutine maybe
  end interface

  integer i, k, total, m, out

  k = 0
  total = 0
  do i = 1, 10
    total = total + k
    call bump
  end do
  write (*, '(1x, a, 2i6)') 'static:', total, k

  m = 5
  call sumup(m, total)
  write (*, '(1x, a, 2i6)') 'dummy:', total, m

  out = 0
  call deep(3, out)
  write (*, '(1x, a, i6)') 'frame:', out

  call maybe(3)
  call maybe(3, 10)

contains
  subroutine bump
    k = k + i
  end subroutine bump
end program hostopt

subroutine sumup(n, total)
  integer, intent(inout) :: n
  integer, intent(out) :: total
  integer j

  total = 0
  do j = 1, 4
    total = total + n
    call double
  end do

contains
  subroutine double
    n = 2*n
  end subroutine double
end subroutine sumup

recursive subroutine deep(level, out)
  integer, intent(in) :: level
  integer, intent(inout) :: out
  integer here, j

  here = level
  do j = 1, 3
    out = out + here
    call lower
  end do
  if (level > 1) call deep(level - 1, out)

contains
  subroutine lower
    here = here - 1
  end subroutine lower
end subroutine deep

subroutine maybe(count, step)
  integer, intent(in) :: count
  integer, intent(in), optional :: step
  integer j, total

  total = 0
  do j = 1, count
    call add
  end do
  write (*, '(1x, a, i6)') 'optional:', total

contains
  subroutine add
    if (present(step)) then
      total = total + step
    else
      total = total + 1
    end if
  end subroutine add
end subroutine maybe

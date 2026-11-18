! hosts.f90: internal procedures of subprograms (docs/planning/PHASE-103 -
! Fortran 90 procedures and interfaces.md, subtask 3). An internal procedure
! reaches its host's dummy arguments (an adjustable array, a scalar, a
! CHARACTER*(*) dummy) through cells the host fills as it starts, its host's
! SAVE'd variable, and its host function's value, in a register (MEAN) or
! through a hidden argument (PAD).
program hosts
  real a(4)
  character(len=8) :: word, pad
  real mean
  data a /1.0, 2.0, 3.0, 4.0/
  word = 'abcdefgh'
  call scale(a, 4, 2.0, word)
  write (*, '(1x, 4f6.1)') a
  write (*, '(1x, 3a)') '[', word, ']'
  write (*, '(1x, a, f8.3)') 'mean:', mean(a, 4)
  write (*, '(1x, 3a)') '[', pad('abc', 6), ']'
end program hosts

subroutine scale(v, n, f, s)
  integer n
  real v(n), f
  character*(*) s
  integer :: k = 0
  call each
  call each
  write (*, '(1x, a, i3)') 'k:', k
  write (*, '(1x, 3a)') '[', mid(3), ']'
contains
  subroutine each
    do j = 1, n
      v(j) = v(j) * f
    end do
    s(k+1:k+1) = 'X'
    k = k + 1
  end subroutine each

  character*3 function mid(j)
    mid = s(j:j+2)
  end function mid
end subroutine scale

real function mean(v, n)
  integer n
  real v(n)
  mean = 0.0
  call add
  mean = mean / n
contains
  subroutine add
    do i = 1, n
      mean = mean + v(i)
    end do
  end subroutine add
end function mean

character*8 function pad(s, n)
  character*(*) s
  integer n
  pad = ' '
  call fill
contains
  subroutine fill
    pad(n - len(s) + 1:n) = s
  end subroutine fill
end function pad

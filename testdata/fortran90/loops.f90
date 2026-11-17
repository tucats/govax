! loops.f90: Fortran 90's DO constructs with EXIT and CYCLE (docs/planning/
! PHASE-101 - Fortran 90 control constructs.md, subtask 5), and the shapes the
! optimizer must keep right (the survey's finding 10): a named EXIT out of two
! counted loops whose variables may live in registers, CYCLE in a loop counted
! to its limit, a count kept through CYCLE counting down, a named CYCLE of the
! outer loop from the inner, DO with no loop control left by EXIT at its top
! and at its bottom, DO WHILE with CYCLE, a labeled and named DO ending at its
! END DO with CYCLE and EXIT from a CASE block, and three loops left from the
! innermost by EXIT naming the middle one. Each result is worked out by hand,
! with the loop variables' values after the loops end.
program loops
  implicit none
  integer :: i, j, k, n, m, total, count, np, cand
  integer :: grid(6, 6), primes(20), rows(5, 4)
  data rows / 1, 2, 3, 4, 5,   6, -1, 7, 8, 9,   -2, 1, 1, 1, 1,   4, 4, -4, 4, 4 /

  ! A search of a table, left by a named EXIT from the inner loop: both
  ! variables keep the values they had.
  do j = 1, 6
     do i = 1, 6
        grid(i, j) = i * 10 + j
     end do
  end do
  search: do j = 1, 6
     do i = 1, 6
        if (grid(i, j) == 43) exit search
     end do
  end do search
  write (*, '(1x, a, 2i3)') 'found: ', i, j

  ! CYCLE in a loop counted to its limit: the odd numbers' sum.
  total = 0
  n = 9
  do i = 1, n
     if (mod(i, 2) == 0) cycle
     total = total + i
  end do
  write (*, '(1x, a, i4, a, i3)') 'odd sum:', total, ', i =', i

  ! Counting down by 3 with CYCLE: the count is kept in its cell, so a CYCLE
  ! can neither lose a pass nor repeat one.
  count = 0
  total = 0
  do i = 20, 1, -3
     count = count + 1
     if (i > 10) cycle
     total = total + i
  end do
  write (*, '(1x, a, 3i4)') 'down:', count, total, i

  ! A named CYCLE of the outer loop from the inner: each row's entries up to
  ! its first negative one.
  total = 0
  rowsum: do j = 1, 4
     do i = 1, 5
        if (rows(i, j) < 0) cycle rowsum
        total = total + rows(i, j)
     end do
  end do rowsum
  write (*, '(1x, a, i4, a, 2i3)') 'rows:', total, ', i, j =', i, j

  ! A DO with no loop control: the primes below 50 by trial division, the
  ! loop left by EXIT at its top, a candidate refused by a named CYCLE.
  np = 0
  cand = 1
  candidates: do
     cand = cand + 1
     if (cand >= 50) exit candidates
     do k = 1, np
        if (primes(k) * primes(k) > cand) exit
        if (mod(cand, primes(k)) == 0) cycle candidates
     end do
     np = np + 1
     primes(np) = cand
  end do candidates
  write (*, '(1x, a, i3)') 'primes:', np
  write (*, '(1x, 15i3)') (primes(k), k = 1, np)

  ! A DO with no loop control left at its bottom, a repeat-until.
  n = 1
  m = 0
  do
     n = n * 2
     m = m + 1
     if (n > 1000) exit
  end do
  write (*, '(1x, a, i5, i3)') 'power:', n, m

  ! DO WHILE with CYCLE, which goes on to the test: Collatz's steps, halving
  ! and tripling, from 6 and from 7.
  do k = 6, 7
     n = k
     m = 0
     count = 0
     do while (n > 1)
        if (mod(n, 2) == 0) then
           n = n / 2
           m = m + 1
           cycle
        end if
        n = 3 * n + 1
        count = count + 1
     end do
     write (*, '(1x, a, i2, a, 2i3)') 'collatz', k, ':', m, count
  end do

  ! A labeled and named DO ending at its END DO, with CYCLE and EXIT naming it
  ! from inside a CASE construct: the numbers up to 23 that aren't multiples
  ! of 5, the loop left at 24.
  total = 0
  sieve: do 70 i = 1, 100
     select case (mod(i, 5))
     case (0)
        cycle sieve
     case (4)
        if (i > 20) exit sieve
     end select
     total = total + i
70 end do sieve
  write (*, '(1x, a, i5, a, i4)') 'sieve:', total, ', i =', i

  ! EXIT from the inner loop only: the outer goes on.
  count = 0
  do i = 1, 5
     do j = 1, 5
        if (j > i) exit
        count = count + 1
     end do
  end do
  write (*, '(1x, a, i3)') 'triangle:', count

  ! Three loops: the innermost cycles itself and leaves the middle one.
  count = 0
  outer: do i = 1, 3
     middle: do j = 1, 3
        inner: do k = 1, 3
           if (k + j > 4) exit middle
           if (k == j) cycle inner
           count = count + 1
        end do inner
     end do middle
  end do outer
  write (*, '(1x, a, 4i3)') 'nest: ', count, i, j, k
end program loops

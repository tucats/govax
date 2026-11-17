! average.f90: reading the terminal in free form (docs/planning/PHASE-99 -
! Fortran 90.md, subtask 6): a name by an A edit, then numbers list-directed
! until a negative one, with DO WHILE; average.in is what's typed.
program average
  implicit none
  real x, sum
  integer n, last
  character*10 who

  write (*, '(1x, a)') "Your name?"
  read (*, '(a)') who
  last = index(who, ' ') - 1     ! the name's length, if it has a blank after it
  if (last < 0) last = len(who)

  write (*, '(1x, a)') "Numbers, and a negative one to end:"
  n = 0; sum = 0
  read (*, *) x
  do while (x >= 0)
     n = n + 1; sum = sum + x
     read (*, *) x
  end do

  write (*, '(1x, a)') "Thank you, " // who(1:last) // "."
  if (n == 0) then
     write (*, '(1x, a)') "No numbers."
  else
     write (*, 20) n, sum / n
20   format (1x, i3, " numbers, average ", f8.3)
  end if
end program average

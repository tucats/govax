! mixed.f90: a program of both source forms (docs/planning/PHASE-99 - Fortran
! 90.md, subtask 6; Decision 3). This file and report.f90 are free form, by
! their type; fill.for is fixed form; and limits.inc, which both forms
! INCLUDE, is written so that either form reads it the same way: statements
! from column 7, no continuations, comments starting with !.
program mixed
  implicit none
  include 'limits.inc'
  integer squares(nmax), k

  call fill(squares, nmax)
  write (*, '(1x, 5i5)') (squares(k), k = 1, nmax)
  call report(squares, nmax)
end program mixed

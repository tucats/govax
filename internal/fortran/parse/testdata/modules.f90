! Phase 106's forms (docs/planning/PHASE-106 - Fortran 90 modules.md, subtask
! 2): MODULE, USE with renames and ONLY lists, PUBLIC and PRIVATE as
! statements, with and without ::, generic specifications in their lists,
! PRIVATE in a type's definition, and module procedures after CONTAINS.
module consts
  use base
  use other, twopi => pi2, e => euler
  private
  public :: pi, point, operator(+), assignment(=), swap
  private gauge
  real, parameter :: pi = 3.14159
  type point
    private
    real x, y
  end type point
  interface swap
    module procedure swapr
  end interface
contains
  subroutine swapr(a, b)
    real a, b
  end subroutine swapr
end module consts
program user
  use consts, only: pi, operator(+), local => swap
  use base, only:
  use other, only: assignment(=), operator(.cross.), operator(/)
  print *, pi
end program user

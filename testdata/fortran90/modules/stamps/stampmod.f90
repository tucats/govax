! stampmod.f90: a module whose text holds a VAX structure with initial values
! and a derived type with a CHARACTER component of length zero (Phase 113,
! subtask 2), which the module file must give back to stamps.f90's compilation.
module stampmod
  implicit none
  structure /stamp/
    integer*2 code /7/
    real*8 when /2.5d0/
    character*4 tag /'ABCD'/
    logical*1 flags(3) /.true., .false., .true./
    union
      map
        integer*4 whole /-1/
      end map
      map
        integer*2 halves(2)
      end map
    end union
    complex both /(1.0, -2.0)/
  end structure
  type hollow
    character(len=0) :: nothing
    integer :: n = 3
    character(len=2) :: pair = 'xy'
  end type
  record /stamp/ last
end module stampmod

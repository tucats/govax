! A parts inventory in an array of a derived type (docs/planning/PHASE-105 -
! Fortran 90 derived types.md, subtask 8). Each part is read whole from the
! terminal by a format (stock.in), a component at a time, until the part named
! END; the list is printed whole as a section, totaled by its components as
! arrays, written to an unformatted scratch file and read back into another
! array (the count first, so the section's extent is the count just read),
! sorted by value with whole parts swapped, and printed with each part's value
! after its components.
program stock
  implicit none
  type part
    character(len=8) :: name
    integer :: count
    real :: price
  end type part
  type(part) :: parts(10), copy(10), tmp
  integer :: n, i, j, k

  n = 0
  do
    read (*, '(a8, i5, f8.2)') tmp
    if (tmp%name == 'END') exit
    n = n + 1
    parts(n) = tmp
  end do
  write (*, '(1x, a8, i5, f8.2)') parts(1:n)
  write (*, '(1x, a, i6, f10.2)') 'total   ', sum(parts(1:n)%count), sum(parts(1:n)%count * parts(1:n)%price)

  open (12, status='SCRATCH', form='UNFORMATTED')
  write (12) n, parts(1:n)
  rewind (12)
  read (12) k, copy(1:k)
  close (12)

  do i = 1, k - 1
    do j = i + 1, k
      if (value(copy(j)) > value(copy(i))) then
        tmp = copy(i)
        copy(i) = copy(j)
        copy(j) = tmp
      end if
    end do
  end do
  write (*, '(1x, a8, i5, f8.2, f10.2)') (copy(i), value(copy(i)), i = 1, k)
  do i = 1, k
    if (copy(i)%count < 5) write (*, '(1x, a8, a)') copy(i)%name, ' reorder'
  end do

contains

  real function value(p)
    type(part), intent(in) :: p

    value = p%count * p%price
  end function value
end program stock

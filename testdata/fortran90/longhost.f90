! Internal procedures of module procedures whose full MACRO names, MODULE.HOST.INNER,
! pass MACRO's 31 characters, so the compiler gives them numbered names: the call and
! the routine must be given the same one (Phase 111, subtask 3a, found by fcalc's
! FCVALUES.FORMAT_VALUE.NEGLIGIBLE). Short names beside them keep their own.
module longhost_numbers
  implicit none
contains
  ! LONGHOST_NUMBERS.SCALED_TOTAL.ACCUMULATE_ONE is 44 characters.
  integer function scaled_total(values, factor)
    integer, intent(in) :: values(:), factor
    integer :: k

    scaled_total = 0
    do k = 1, size(values)
      call accumulate_one(values(k))
    end do

  contains

    subroutine accumulate_one(v)
      integer, intent(in) :: v

      scaled_total = scaled_total + v * factor
    end subroutine accumulate_one
  end function scaled_total

  ! Two long internal functions of one host, and a short one.
  integer function classify_number(n)
    integer, intent(in) :: n

    if (is_a_negative_number(n)) then
      classify_number = -1
    else if (is_exactly_zero_value(n)) then
      classify_number = 0
    else
      classify_number = one(n)
    end if

  contains

    logical function is_a_negative_number(m)
      integer, intent(in) :: m

      is_a_negative_number = m < 0
    end function is_a_negative_number

    logical function is_exactly_zero_value(m)
      integer, intent(in) :: m

      is_exactly_zero_value = m == 0
    end function is_exactly_zero_value

    integer function one(m)
      integer, intent(in) :: m

      one = m / m
    end function one
  end function classify_number
end module longhost_numbers

program longhost
  use longhost_numbers
  implicit none

  print '(1X, A, I6)', 'scaled total:', scaled_total((/ 1, 2, 3, 4 /), 10)
  print '(1X, A, 3I3)', 'classes:', classify_number(-5), classify_number(0), classify_number(7)
end program longhost

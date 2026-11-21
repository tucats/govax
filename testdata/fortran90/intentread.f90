! An INTENT(IN) dummy argument read from (Phase 111, subtask 2b): a READ's
! internal file and DECODE's buffer are read, not given a value, so either
! may be an INTENT(IN) dummy's text, whole or a substring.
program intentread
  implicit none
  real :: total

  call parse('12.5 7.25', total)
  print '(1x, f8.3)', total
  print '(1x, i6)', year('A 2024 B')

contains

  subroutine parse(text, sum)
    character(len=*), intent(in) :: text
    real, intent(out) :: sum
    real :: a, b

    read (text, *) a, b
    sum = a
    read (text(6:), *) b
    sum = sum + b
  end subroutine parse

  integer function year(text)
    character(len=*), intent(in) :: text

    decode (4, '(I4)', text(3:6)) year
  end function year
end program intentread

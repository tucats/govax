! Phase 113, subtask 2: CHARACTER of length zero where it was refused before:
! a component (in a structure constructor, an assignment, I/O, a substring,
! and an array component), a FUNCTION statement's type, and IMPLICIT's; and
! TRANSFER to a CHARACTER MOLD of zero length or of a length known only when
! the program runs (a passed-length dummy, an automatic object, a constructor
! of one), scalar and array.
program zerolen
  implicit none
  type t
    character(len=0) :: c
    integer :: n
    character(0) :: d(2)
    character(len=3) :: e
  end type
  type(t) :: x, y(3)
  character(len=0), external :: nothing
  character(len=5) :: s
  character(len=4) :: w
  character(len=0) :: z
  integer :: i

  x%n = 1
  x%e = 'xyz'
  x%c = 'abc'
  write (*, '(1x, a, i3, a, a, a)') '[', len(x%c), ']', x%c, '|'
  write (*, '(1x, a, a, i3, a, a, a, a)') '[', x, ']'
  x = t('a', 2, (/ 'b', 'c' /), 'pqr')
  y = x
  y%c = x%c
  y(2)%n = 7
  write (*, '(1x, a, 3i3, 1x, 3a)') 'y:', y%n, y%e
  write (*, '(1x, 3a)') '[', x%c(1:0), ']'
  s = 'hello'
  s = nothing(s) // s
  write (*, '(1x, 3a, i3)') '[', s, ']', len(nothing(s))
  i = size(y(1)%d)
  write (*, '(1x, a, i3, i4)') 'd:', i, len(y(1)%d(2))
  call implicit0

  i = 1145258561
  call show(i, 'abcd')
  call show(i, 'ab')
  call show(i, 'abcdef')
  w = transfer(i, z) // 'wxyz'
  write (*, '(1x, 3a, i2)') '[', w, ']', len(transfer(i, z))

contains

  subroutine implicit0
    implicit character*0 (c)
    character*2 cb

    ca = 'xyz'
    cb = 'xyz'
    write (*, '(1x, 5a, 2i2)') '[', ca, '|', cb, ']', len(ca), len(cb)
  end subroutine implicit0

  subroutine show(k, m)
    integer, intent(in) :: k
    character(len=*), intent(in) :: m
    character(len=len(m)) :: auto
    character(len=8) :: line
    integer :: n

    line = transfer(k, m)
    n = len(transfer(k, m))
    write (*, '(1x, a, i2, 1x, 3a)') 'scalar:', n, '[', line, ']'
    auto = transfer(k, auto)
    write (*, '(1x, 3a)') '[', auto, ']'
    write (*, '(1x, a, i2, 1x, 4a)') 'array:', size(transfer(k, m, 2)), '[', transfer(k, m, 2), ']'
    write (*, '(1x, a, i2)') 'no size:', size(transfer(k, (/ m /)))
  end subroutine show
end program zerolen

character(len=0) function nothing(x)
  character(*) :: x

  nothing = x
end function nothing

! Functions whose CHARACTER value's length is known only when the program
! runs, and ENTRY in array-valued functions (Phase 104, subtask 7c). A scalar
! made by the function as a dynamic string: through an interface body, in
! concatenations, LEN, comparisons, SELECT CASE, an output list's implied DO,
! an argument to a passed-length dummy, assigned to a substring; a recursive
! one with RESULT whose value is its own shorter value with a character more,
! down to no characters; an internal one whose length is SIZE of a dummy.
! Arrays of such a length, internal and external. ENTRY in functions whose
! value is an array of a known shape, of a shape known only at run time (the
! arguments in another order), and a string of a length known only then. And
! a string made a thousand times in a loop.
program madetext
  implicit none
  interface
    function repeat2(c, n)
      character(len=*) :: c
      integer :: n
      character(len=n*len(c)) :: repeat2
    end function
    recursive function stars(n) result(s)
      integer :: n
      character(len=n) :: s
    end function
    function tags(n, w)
      integer :: n, w
      character(len=w) :: tags(n)
    end function
    function sq(n)
      integer :: n, sq(3)
    end function
    function cu(n)
      integer :: n, cu(3)
    end function
    function up(n, k)
      integer :: n, k
      real :: up(n)
    end function
    function down(k, n)
      integer :: n, k
      real :: down(n)
    end function
    function word(n)
      integer :: n
      character(len=n) :: word
    end function
    function dash(n)
      integer :: n
      character(len=n) :: dash
    end function
  end interface
  character(len=12) :: s
  integer :: i, total

  s = repeat2('ab', 3)
  write (*, '(1x, 3a)') '[', s, ']'
  write (*, '(1x, a, i3, 1x, a)') 'len:  ', len(repeat2('xyz', 2)), repeat2('-', 5) // '|'
  write (*, '(1x, a, 4(a, 1x))') 'loop: ', (repeat2('+', i), i = 1, 4)
  if (repeat2('ab', 2) == 'abab') write (*, '(1x, a)') 'same:  yes'
  select case (repeat2('z', 2))
  case ('zz')
    write (*, '(1x, a)') 'case:  zz'
  case default
    write (*, '(1x, a)') 'case:  other'
  end select
  call show(repeat2('q', 3) // '!')
  s = '............'
  s(3:8) = repeat2('xy', 3)
  write (*, '(1x, 2a)') 'sub:   ', s

  write (*, '(1x, 3a, i2)') 'stars: [', stars(4), ']', len(stars(0))
  write (*, '(1x, 3a)') 'size:  [', unit((/ 1, 2, 3, 4, 5 /)), ']'

  write (*, '(1x, a, 3(a, 1x))') 'tags: ', tags(3, 4)
  write (*, '(1x, a, 3a)') 'pad:  ', pad('ab')
  write (*, '(1x, a, i3)') 'plen: ', len(pad('abcd'))

  write (*, '(1x, a, 6i5)') 'known:', sq(2), cu(2)
  write (*, '(1x, a, 7f5.1)') 'made: ', up(3, 1), down(-1, 4)
  write (*, '(1x, 5a)') 'entry: [', word(3), '][', dash(5), ']'

  total = 0
  do i = 1, 1000
    total = total + len(repeat2('ab', mod(i, 5)))
  end do
  write (*, '(1x, a, i6)') 'total:', total

contains

  function unit(v)
    integer :: v(:)
    character(len=size(v)) :: unit
    unit = '12345678'
  end function

  function pad(c)
    character(len=*) :: c
    character(len=len(c) + 2) :: pad(3)
    pad = '<' // c // '>'
  end function

  subroutine show(t)
    character(len=*) :: t
    write (*, '(1x, a, i3, 1x, a)') 'show: ', len(t), t
  end subroutine

end program madetext

function repeat2(c, n)
  character(len=*) :: c
  integer :: n, i
  character(len=n*len(c)) :: repeat2
  do i = 1, n
    repeat2((i - 1) * len(c) + 1:i * len(c)) = c
  end do
end function

recursive function stars(n) result(s)
  integer :: n
  character(len=n) :: s
  if (n > 0) s = '*' // stars(n - 1)
end function

function tags(n, w)
  integer :: n, w, i
  character(len=w) :: tags(n)
  tags = ' '
  do i = 1, n
    tags(i)(1:i) = '####'
  end do
end function

function sq(n)
  integer :: n, sq(3), cu(3), i
  do i = 1, 3
    sq(i) = (n + i) ** 2
  end do
  return
  entry cu(n)
  do i = 1, 3
    cu(i) = (n + i) ** 3
  end do
end function

function up(n, k)
  integer :: n, k, i
  real :: up(n), down(n)
  entry down(k, n)
  do i = 1, n
    up(i) = k * i
  end do
end function

function word(n)
  integer :: n
  character(len=n) :: word, dash
  word = 'wordwordword'
  return
  entry dash(n)
  dash = '-----------'
end function

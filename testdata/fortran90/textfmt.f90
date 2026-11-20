! A text formatter (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md,
! subtask 8): paragraphs typed at the terminal (textfmt.in), a blank line after
! each, are gathered into one string, their blanks closed up, cut into words by
! VERIFY and SCAN, and filled into lines no wider than WIDTH; each line is printed
! flush left, flush right by ADJUSTR, and centered by REPEAT. Each paragraph's
! first line is its title, printed with every word capitalized by IACHAR and
! ACHAR and underlined by REPEAT to its LEN_TRIM; its last word is found by INDEX
! with BACK; a table of what each paragraph had ends the run.
program textfmt
  implicit none
  integer, parameter :: width = 24, most = 4
  character(len=80) :: line, title
  character(len=400) :: para
  integer :: n, ios, k, i
  integer :: words(most), chars(most), lines(most)

  n = 0
  do
    read (*, '(a)', iostat=ios) title
    if (ios /= 0) exit
    if (len_trim(title) == 0) cycle
    para = ''
    do
      read (*, '(a)', iostat=ios) line
      if (ios /= 0 .or. len_trim(line) == 0) exit
      para = trim(para) // ' ' // line
    end do
    n = n + 1
    call closeup(para)
    title = capitals(adjustl(title))
    print '(1x, a)', trim(title)
    print '(1x, a)', repeat('=', len_trim(title))
    call fill(para, words(n), lines(n))
    chars(n) = len_trim(para)
    k = index(trim(para), ' ', back=.true.)
    print '(1x, 3a)', 'last word: "', para(k + 1:len_trim(para)), '"'
    print '(1x)'
    if (ios /= 0) exit
  end do

  print '(1x, a)', repeat('-', width)
  print '(1x, a)', ' #  words  chars  lines'
  do i = 1, n
    print '(1x, i2, 3i7)', i, words(i), chars(i), lines(i)
  end do
  print '(1x, a)', repeat('-', width)
  print '(1x, a2, 3i7)', '  ', sum(words(1:n)), sum(chars(1:n)), sum(lines(1:n))

contains

  ! The text with its leading blanks gone and every run of blanks one blank.
  subroutine closeup(s)
    character(len=*), intent(inout) :: s
    integer :: k

    s = adjustl(s)
    do
      k = index(trim(s), '  ')
      if (k == 0) exit
      s = s(1:k) // s(k + 2:)
    end do
  end subroutine closeup

  ! S with the first letter of each word a capital.
  function capitals(s) result(c)
    character(len=*), intent(in) :: s
    character(len=len(s)) :: c
    integer :: i
    logical :: first

    c = s
    first = .true.
    do i = 1, len_trim(c)
      if (c(i:i) == ' ') then
        first = .true.
      else
        if (first .and. iachar(c(i:i)) >= iachar('a') .and. iachar(c(i:i)) <= iachar('z')) then
          c(i:i) = achar(iachar(c(i:i)) - iachar('a') + iachar('A'))
        end if
        first = .false.
      end if
    end do
  end function capitals

  ! S's words filled into lines of WIDTH, each printed three ways; the count
  ! of words and of lines.
  subroutine fill(s, nwords, nlines)
    character(len=*), intent(in) :: s
    integer, intent(out) :: nwords, nlines
    character(len=width) :: cur
    integer :: pos, start, length

    nwords = 0
    nlines = 0
    cur = ''
    pos = 1
    do
      start = verify(s(pos:), ' ')
      if (start == 0) exit
      start = start + pos - 1
      length = scan(s(start:), ' ') - 1
      nwords = nwords + 1
      if (len_trim(cur) > 0 .and. len_trim(cur) + 1 + length > width) then
        call show(cur)
        nlines = nlines + 1
        cur = ''
      end if
      if (len_trim(cur) == 0) then
        cur = s(start:start + length - 1)
      else
        cur = trim(cur) // ' ' // s(start:start + length - 1)
      end if
      pos = start + length
    end do
    if (len_trim(cur) > 0) then
      call show(cur)
      nlines = nlines + 1
    end if
  end subroutine fill

  ! One line flush left, flush right, and centered, between bars.
  subroutine show(text)
    character(len=width), intent(in) :: text
    character(len=width) :: centered

    centered = repeat(' ', (width - len_trim(text)) / 2) // text
    print '(1x, 7a)', '|', text, '|', adjustr(text), '|', centered, '|'
  end subroutine show

end program textfmt

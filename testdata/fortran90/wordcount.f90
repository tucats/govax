program wordcount
  ! Text from the terminal read a character at a time (docs/planning/PHASE-109 -
  ! Fortran 90 IO.md, subtask 8; wordcount.in): a nonadvancing READ of one
  ! character, each line's end found by EOR= and the input's by END=, an empty
  ! line and leading and repeated blanks among them; the words kept in an array
  ! of a derived type with their counts, sorted by swapping whole objects; a
  ! histogram of their lengths, each bar drawn a character at a time by
  ! nonadvancing WRITEs and ended by an advancing WRITE with no items; the
  ! sorted words packed into lines of at most 40 columns; and the numbers among
  ! them read from internal files.
  implicit none
  type tally
    character(len=20) :: text
    integer :: count
  end type tally
  integer, parameter :: maxwords = 100, width = 40
  character :: c
  character(len=20) :: word
  character(len=80) :: line, longest
  type(tally) :: words(maxwords), swap
  integer :: lengths(maxwords), histogram(20), at(1)
  integer :: nwords, nunique, nlines, nchars, longest_length, k, i, j, col, n, total, numbers

  nwords = 0
  nunique = 0
  nlines = 0
  nchars = 0
  longest_length = 0
  word = ' '
  k = 0
  lines: do
    line = ' '
    i = 0
    do
      read (*, '(A1)', advance='NO', eor=20, end=90) c
      i = i + 1
      line(i:i) = c
      if (c /= ' ') then
        k = k + 1
        word(k:k) = c
      else
        call finish_word
      end if
    end do
20  call finish_word
    nlines = nlines + 1
    nchars = nchars + i
    if (i > longest_length) then
      longest_length = i
      longest = line
    end if
  end do lines
90 continue

  print '(1X, A, I3, A, I4, A, I4)', 'Lines:', nlines, '  words:', nwords, '  characters:', nchars
  print '(1X, A, I3, 3A)', 'Longest line,', longest_length, ' characters: "', &
      longest(1:longest_length), '"'
  ! Fortran 90's MAXLOC has no DIM=: its value is a vector of one subscript.
  at = maxloc(lengths(1:nwords))
  print '(1X, 2A)', 'Longest word: ', trim(words(at(1))%text)

  ! The lengths' histogram, a bar a character at a time.
  histogram = 0
  do i = 1, nwords
    histogram(lengths(i)) = histogram(lengths(i)) + 1
  end do
  do j = 1, maxval(lengths(1:nwords))
    write (*, '(1X, I2, I3, " |")', advance='NO') j, histogram(j)
    do i = 1, histogram(j)
      write (*, '(A)', advance='NO') '*'
    end do
    write (*, '()')
  end do

  ! The words once each, sorted, packed into lines of at most width columns.
  do i = 1, nwords
    do j = 1, nunique
      if (words(j)%text == words(i)%text) exit
    end do
    if (j > nunique) then
      nunique = nunique + 1
      words(nunique) = tally(words(i)%text, 0)
    end if
    words(j)%count = words(j)%count + 1
  end do
  do i = 2, nunique
    do j = i, 2, -1
      if (words(j)%text >= words(j-1)%text) exit
      swap = words(j)
      words(j) = words(j-1)
      words(j-1) = swap
    end do
  end do
  col = 0
  do i = 1, nunique
    n = len_trim(words(i)%text)
    if (col + n + 1 > width) then
      write (*, '()')
      col = 0
    end if
    write (*, '(1X, A)', advance='NO') words(i)%text(1:n)
    col = col + n + 1
  end do
  if (col > 0) write (*, '()')

  write (*, '(1X, A)', advance='NO') 'Repeated:'
  do i = 1, nunique
    if (words(i)%count > 1) then
      write (*, '(1X, A, "*", I1)', advance='NO') trim(words(i)%text), words(i)%count
    end if
  end do
  write (*, '()')

  ! The numbers among the words, read from internal files.
  numbers = 0
  total = 0
  do i = 1, nunique
    if (verify(trim(words(i)%text), '0123456789') == 0) then
      read (words(i)%text, '(I20)') n
      numbers = numbers + 1
      total = total + n
    end if
  end do
  print '(1X, A, I3, A, I6)', 'Numbers:', numbers, '  sum:', total

contains

  ! The word being gathered, if there is one, joins the list.
  subroutine finish_word
    if (k > 0) then
      nwords = nwords + 1
      words(nwords) = tally(word, 1)
      lengths(nwords) = k
      word = ' '
      k = 0
    end if
  end subroutine finish_word

end program wordcount

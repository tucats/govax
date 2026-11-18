! Sorting (Phase 104, subtask 9): a quicksort written as a recursive function
! whose value's shape is SIZE of its assumed-shape dummy, made of a constructor
! of its own values for the elements PACK chooses below and above the first;
! a merge sort, a recursive subroutine with an automatic work array of its
! dummy's size, given contiguous sections of its own array; each held to the
! other on 300 values from a linear congruential generator, and empty and
! one-element arrays sorted too.
program sorter
  implicit none
  interface
    recursive function qsort(v) result(s)
      integer :: v(:)
      integer :: s(size(v))
    end function
  end interface
  integer :: data(12), w(12), big(300), check(300), x, i

  data = (/ 33, 7, 21, 7, 98, -4, 0, 56, 12, 33, 1, 15 /)
  write (*, '(1x, a, 12i4)') 'quick:', qsort(data)
  w = data
  call msort(w, size(w))
  write (*, '(1x, a, 12i4)') 'merge:', w
  write (*, '(1x, a, 2i3)') 'small:', size(qsort(data(1:0))), qsort(data(5:5))
  write (*, '(1x, a, 3i4)') 'desc: ', qsort((/ 3, 2, 1 /))

  x = 1
  do i = 1, size(big)
    x = mod(x * 1103 + 12345, 32768)
    big(i) = x - 16384
  end do
  check = big
  call msort(check, size(check))
  write (*, '(1x, a, l2)') 'ordered:', all(check(1:size(check) - 1) <= check(2:))
  write (*, '(1x, a, l2)') 'same:   ', all(qsort(big) == check)
  write (*, '(1x, a, 3i7)') 'ends:   ', check(1), check(150), check(300)
  write (*, '(1x, a, i9)') 'sum:    ', sum(qsort(big))
end program sorter

recursive function qsort(v) result(s)
  integer :: v(:)
  integer :: s(size(v))
  if (size(v) <= 1) then
    s = v
    return
  end if
  s = (/ qsort(pack(v(2:), v(2:) < v(1))), v(1), qsort(pack(v(2:), v(2:) >= v(1))) /)
end function qsort

recursive subroutine msort(a, n)
  integer :: n, a(n)
  integer :: t(n), i, j, k, h
  if (n < 2) return
  h = n / 2
  call msort(a, h)
  call msort(a(h + 1:), n - h)
  i = 1
  j = h + 1
  do k = 1, n
    if (j > n) then
      t(k) = a(i)
      i = i + 1
    else if (i > h) then
      t(k) = a(j)
      j = j + 1
    else if (a(i) <= a(j)) then
      t(k) = a(i)
      i = i + 1
    else
      t(k) = a(j)
      j = j + 1
    end if
  end do
  a = t
end subroutine msort

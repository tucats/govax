! The sieve of Eratosthenes (Phase 104, subtask 9): an allocatable LOGICAL
! array of bounds 2:N for each N, its multiples struck out by sections with a
! stride, the primes gathered by PACK of a constructor into an allocatable
! array of COUNT's size, both deallocated for the next N; ALLOCATED before and
! after; ALLOCATE with STAT= of an array already allocated; and a hundred
! sieves of growing size in a subroutine whose array is its own allocatable
! local, deallocated at each return.
program sieve
  implicit none
  logical, allocatable :: prime(:)
  integer, allocatable :: list(:)
  integer :: n, i, stat, total

  do n = 10, 100, 45
    allocate (prime(2:n))
    prime = .true.
    do i = 2, int(sqrt(real(n)))
      if (prime(i)) prime(i * i:n:i) = .false.
    end do
    allocate (list(count(prime)))
    list = pack((/ (i, i = 2, n) /), prime)
    write (*, '(1x, i4, a, i3, a)') n, ':', size(list), ' primes'
    write (*, '(1x, 15i4)') list
    write (*, '(1x, a, 2i5)') 'bounds:', lbound(prime), ubound(prime)
    deallocate (prime, list)
  end do

  write (*, '(1x, a, l2)') 'allocated:', allocated(prime)
  allocate (prime(5))
  write (*, '(1x, a, l2)') 'allocated:', allocated(prime)
  allocate (prime(6), stat=stat)
  write (*, '(1x, a, l2, i3)') 'again failed:', stat /= 0, size(prime)
  deallocate (prime)

  total = 0
  do n = 2, 200, 2
    total = total + primes(n)
  end do
  write (*, '(1x, a, i6)') 'total:', total

contains

  integer function primes(n)
    integer :: n, i
    logical, allocatable :: p(:)
    allocate (p(n))
    p = .true.
    p(1) = .false.
    do i = 2, n
      if (p(i)) p(2 * i:n:i) = .false.
    end do
    primes = count(p)
  end function

end program sieve

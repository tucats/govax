! The transformational functions (Phase 104, subtask 8), each against the
! standard's examples (13.13), from variables so that nothing is folded:
! SPREAD, CSHIFT and EOSHIFT (by a scalar and by an array of shifts, with and
! without BOUNDARY, of CHARACTER too), PACK (with and without VECTOR, a scalar
! MASK), UNPACK, RESHAPE of values (PAD, ORDER, a shape known only at run
! time), and reductions by a DIM= known only at run time; a statement whose
! target is the function's argument; values of a size known only at run time
! in a subroutine; one as an argument to an assumed-shape dummy; MERGE of
! CHARACTER values of a length known only at run time; and a ring rotated a
! thousand times.
program transform
  implicit none
  interface
    subroutine shapes(a)
      integer :: a(:, :)
    end subroutine
  end interface
  integer :: v(6), m(3, 3), z(3, 3), i, k, n, ring(5)
  character :: c(3, 3)
  logical :: mk(3, 3)

  v = (/ (i, i = 1, 6) /)
  m = reshape((/ 1, 4, 7, 2, 5, 8, 3, 6, 9 /), (/ 3, 3 /))
  n = 3

  write (*, '(1x, a, 9i3)') 'spread: ', spread(v(2:4), 1, n)
  write (*, '(1x, a, 9i3)') 'spread2:', spread(v(2:4), 2, n)
  write (*, '(1x, a, 6i3)') 'cshift: ', cshift(v, 2)
  write (*, '(1x, a, 6i3)') 'cshift-:', cshift(v, -2)
  write (*, '(1x, a, 9i3)') 'cshift2:', cshift(m, shift=(/ -1, 1, 0 /), dim=2)
  write (*, '(1x, a, 6i3)') 'eoshift:', eoshift(v, 3)
  write (*, '(1x, a, 6i3)') 'eosh-9: ', eoshift(v, -2, 9)

  c = reshape((/ 'A', 'D', 'G', 'B', 'E', 'H', 'C', 'F', 'I' /), (/ 3, 3 /))
  write (*, '(1x, a, 9a2)') 'eosh2:  ', eoshift(c, shift=(/ 1, -1, 0 /), boundary=(/ '*', '/', '?' /), dim=2)
  write (*, '(1x, a, 9a2)') 'eosh-c: ', eoshift(c, 1)

  z = 0
  z(2, 1) = 9
  z(3, 3) = 7
  write (*, '(1x, a, 2i3)') 'pack:   ', pack(z, z /= 0)
  write (*, '(1x, a, 6i3)') 'packv:  ', pack(z, mask=z /= 0, vector=v)
  write (*, '(1x, a, 9i3)') 'packall:', pack(m, .true.)
  write (*, '(1x, a, i3)') 'packnot:', size(pack(m, .false.))

  mk = .false.
  mk(2, 1) = .true.
  mk(1, 2) = .true.
  mk(3, 3) = .true.
  write (*, '(1x, a, 9i3)') 'unpack: ', unpack(v(1:3), mask=mk, field=reshape((/ 1, 0, 0, 0, 1, 0, 0, 0, 1 /), (/ 3, 3 /)))
  write (*, '(1x, a, 9i3)') 'unpack0:', unpack(v(4:6), mk, 0)

  write (*, '(1x, a, 8i3)') 'reshape:', reshape(v, (/ 2, n + 1 /), (/ 0, 0 /), (/ 2, 1 /))
  write (*, '(1x, a, 6i3)') 'resh2:  ', reshape(v, (/ n - 1, n /))
  write (*, '(1x, a, 2i3)') 'rshape: ', shape(reshape(v, (/ n - 1, n /)))

  do k = 1, 2
    write (*, '(1x, a, i2, 3i4)') 'sum dim', k, sum(m, dim=k)
  end do
  write (*, '(1x, a, 3l2)') 'any:    ', any(m > 7, dim=n - 1)
  write (*, '(1x, a, 3i4)') 'maxval: ', maxval(m, dim=n - 2, mask=m /= 8)

  v = cshift(v, 1)
  write (*, '(1x, a, 6i3)') 'self:   ', v
  m = transpose(eoshift(m, 1, -1))
  write (*, '(1x, a, 9i3)') 'teosh:  ', m

  call shapes(spread(v(1:2), 2, n))
  call runtime(4)
  call strings(3)

  ring = (/ 1, 2, 3, 4, 5 /)
  do i = 1, 1000
    ring = cshift(ring, mod(i, 3) - 1)
  end do
  write (*, '(1x, a, 5i3)') 'ring:   ', ring
end program transform

subroutine shapes(a)
  integer :: a(:, :)
  write (*, '(1x, a, 2i3, 6i3)') 'shapes: ', shape(a), a
end subroutine

subroutine runtime(n)
  integer :: n, i
  real :: x(n), y(n, n)
  x = (/ (real(i), i = 1, n) /)
  y = spread(x, 1, n)
  write (*, '(1x, a, 4f5.1)') 'rt sum: ', sum(y, dim=1)
  write (*, '(1x, a, 4f5.1)') 'rt csh: ', cshift(x, n - 1)
  write (*, '(1x, a, 4f5.1)') 'rt eos: ', eoshift(x, 1, -1.0)
  write (*, '(1x, a, 2f5.1)') 'rt pack:', pack(x, x > 2.0)
end subroutine

subroutine strings(n)
  integer :: n
  character(len=n) :: a(2), b(2)
  a = (/ 'abc', 'def' /)
  b = (/ 'xyz', 'uvw' /)
  write (*, '(1x, a, 2(a, 1x))') 'merge:  ', merge(a, b, (/ .true., .false. /))
  write (*, '(1x, a, 2(a, 1x))') 'cshc:   ', cshift(a, 1)
end subroutine

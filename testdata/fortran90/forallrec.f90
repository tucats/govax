! FORALL's harder cases (Phase 113, subtask 3), each once a NOTIMPL: every
! value and every target's subscripts computed before anything is stored, for
! a target whose subscripts read its own array, an array target, a CHARACTER
! target of run-time length, a target reached through a pointer, a triangle of
! tuples, pointer assignments of every kind, a defined assignment, and WHEREs;
! a nested FORALL's bounds and a mask kept as they were when it was reached,
! though its statements change what they read; and an elemental defined
! assignment of CHARACTER values of run-time length that read its target.
module tools
  implicit none
  type cents
    integer :: c
  end type cents
  type label
    character(len=6) :: name
  end type label
  type ref
    integer, pointer :: p
  end type ref
  type tag
    character(len=4), pointer :: s
  end type tag
  type vec
    integer, pointer :: v(:)
  end type vec
  type holder
    integer :: a(4)
  end type holder
  interface assignment(=)
    module procedure to_cents, to_label
  end interface
contains
  pure subroutine to_cents(m, d)
    type(cents), intent(out) :: m
    integer, intent(in) :: d
    m%c = 100 * d
  end subroutine to_cents

  elemental subroutine to_label(x, s)
    type(label), intent(out) :: x
    character(len=*), intent(in) :: s
    x%name = s // '*'
  end subroutine to_label

  ! A CHARACTER target whose length is the caller's.
  subroutine reverse(c, n)
    integer, intent(in) :: n
    character(len=*), intent(inout) :: c(n)
    integer :: i
    forall (i = 1:n) c(i) = c(n+1-i)
  end subroutine reverse
end module tools

program forallrec
  use tools
  implicit none
  integer :: iv(5), w(3,3), t(4,4), s(4,4), b(3,4), c(3,4), u(4,4), z(4,4)
  integer :: x(3,3), y(3,3), i, j, k
  logical :: keep(3,3)
  integer, target :: obj(4), whole(6)
  character(len=4), target :: words(4)
  character(len=5) :: names(4)
  type(ref) :: refs(4)
  type(tag) :: tags(4)
  type(vec) :: vecs(3)
  type(cents) :: purse(4)
  type(label) :: labs(3)
  type(holder), target :: box
  type(holder), pointer :: hp

  ! Every target's subscripts first: IV(IV(I)) with IV as it was.
  iv = (/ 3, 1, 2, 5, 4 /)
  forall (i = 1:5) iv(iv(i)) = 10 * i
  print '(5I4)', iv

  ! An array target: each row from the old columns.
  forall (i = 1:3, j = 1:3) w(i,j) = 10*i + j
  forall (i = 1:3) w(i,:) = w(:,i)
  print '(3I4)', (w(i,:), i = 1, 3)

  ! A CHARACTER target of run-time length.
  names = (/ 'alpha', 'beta ', 'gamma', 'delta' /)
  call reverse(names, 4)
  print '(4(1X,A))', names

  ! A target through a pointer: its storage isn't known, so recorded.
  box%a = (/ 1, 2, 3, 4 /)
  hp => box
  forall (i = 1:4) hp%a(i) = hp%a(5-i) * 10
  print '(4I4)', box%a

  ! A triangle of tuples whose values read the rows above.
  forall (i = 1:4, j = 1:4) t(i,j) = 10*i + j
  forall (i = 2:4)
    forall (j = 1:i-1) t(i,j) = t(i-1,j) + 100
  end forall
  print '(4I5)', (t(i,:), i = 1, 4)

  ! A triangle done in turn, whose nested bound is an expression of the outer
  ! index: computed for each outer tuple, not once before them.
  s = 0
  forall (i = 2:4)
    forall (j = 1:i-1) s(i,j) = 10*i + j
  end forall
  print '(4I3)', (s(i,:), i = 1, 4)

  ! A nested FORALL's bound its own statements change: the bound is the one
  ! it had when the nested FORALL was reached.
  b = 0
  b(:,1) = (/ 2, 3, 1 /)
  c = 0
  forall (i = 1:3)
    forall (j = 1:b(i,1))
      b(i,j) = 5 - j
      c(i,j) = b(i,j)
    end forall
  end forall
  print '(4I3)', (c(i,:), i = 1, 3)

  ! A mask its statements change, in a triangle: Z is set where U was.
  forall (i = 1:4, j = 1:4) u(i,j) = mod(i + j, 2)
  z = 0
  forall (i = 1:4)
    forall (j = 1:i, u(i,j) > 0)
      u(i,j) = 0
      z(i,j) = 1
    end forall
  end forall
  print '(4I2)', (z(i,:), i = 1, 4)
  print '(I3)', sum(u)

  ! Pointer assignments that read the pointers they change.
  obj = (/ 10, 20, 30, 40 /)
  forall (i = 1:4) refs(i)%p => obj(i)
  forall (i = 1:3) refs(i+1)%p => refs(i)%p
  print '(4I4)', (refs(i)%p, i = 1, 4)

  words = (/ 'five', 'four', 'nine', 'zero' /)
  forall (i = 1:4) tags(i)%s => words(i)
  forall (i = 1:3) tags(i)%s => tags(i+1)%s
  print '(4(1X,A))', (tags(i)%s, i = 1, 4)

  whole = (/ 1, 2, 3, 4, 5, 6 /)
  forall (i = 1:3) vecs(i)%v => whole(2*i-1:2*i)
  forall (i = 1:2) vecs(i+1)%v => vecs(i)%v
  print '(6I3)', (vecs(i)%v, i = 1, 3)

  ! A defined assignment whose values read what it assigns.
  forall (i = 1:4) purse(i) = i
  forall (i = 1:4) purse(i) = purse(5-i)%c
  print '(4I6)', purse%c

  ! A WHERE whose values overlap what it assigns.
  forall (i = 1:3, j = 1:3) x(i,j) = 10*i + j
  forall (i = 1:3)
    where (x(i,:) > 20) x(i,:) = x(:,i)
  end forall
  print '(3I4)', (x(i,:), i = 1, 3)

  ! A WHERE of two assignments, each done for every tuple before the next.
  forall (i = 1:3, j = 1:3) x(i,j) = 10*i + j
  keep = .true.
  keep(1,2) = .false.
  y = 0
  forall (i = 1:3)
    where (keep(i,:))
      x(i,:) = x(:,i)
      y(i,:) = x(i,:) + x(:,i)
    end where
  end forall
  print '(3I4)', (x(i,:), i = 1, 3)
  print '(3I4)', (y(i,:), i = 1, 3)

  ! An elemental defined assignment of CHARACTER values whose length is
  ! known only when the program runs, read from the target itself.
  labs = (/ 'red   ', 'green ', 'blue  ' /)
  k = 3
  labs = labs(3:1:-1)%name(1:k)
  print '(3(1X,A))', (trim(labs(i)%name), i = 1, 3)
end program forallrec

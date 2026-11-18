! opts.f90: the Fortran 90 side of shortlist (shortlist.for says why).
subroutine opts(x, n, label)
  real, intent(in) :: x
  integer, intent(in), optional :: n
  character*(*), intent(in), optional :: label

  write (*, '(1x, a, f4.1, 2l2)') 'opts', x, present(n), present(label)
  call show(n, label)

contains
  subroutine show(m, text)
    integer, intent(in), optional :: m
    character*(*), intent(in), optional :: text

    if (present(m)) write (*, '(1x, a, i3)') '  count', m
    if (present(text)) write (*, '(1x, 2a)') '  label ', text
  end subroutine show
end subroutine opts

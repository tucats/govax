! NULL(), => NULL() initialization, and default initialization of
! derived types' components: static, local, allocated, INTENT(OUT).
module nullm
  implicit none
  type report
    character(len=8) :: name = 'none'
    integer :: day
    integer :: year = 1995
    integer, pointer :: link => null()
    real :: v(3) = 1.5
  end type report
end module nullm

program nullinit
  use nullm
  implicit none
  integer, pointer :: p => null(), q
  type(report) :: r
  type(report), allocatable :: rs(:)
  type(report) :: nov
  integer :: k
  print *, associated(p)
  q => null()
  print *, associated(q)
  print '(A8,I5,L2,3F5.1)', r%name, r%year, associated(r%link), r%v
  nov = report('sales', 15, 1996, null(), 0.0)
  print '(A8,2I5)', nov%name, nov%day, nov%year
  allocate (rs(2))
  print '(A8,I5,L2)', rs(2)%name, rs(2)%year, associated(rs(2)%link)
  r%year = 2000
  call reset(r)
  print '(A8,I5)', r%name, r%year
  do k = 1, 2
    call local(k)
  end do
contains
  subroutine reset(x)
    type(report), intent(out) :: x
  end subroutine reset
  subroutine local(k)
    integer, intent(in) :: k
    type(report) :: t
    print '(I2,I6)', k, t%year
    t%year = 2001
  end subroutine local
end program nullinit

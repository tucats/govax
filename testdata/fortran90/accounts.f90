! A SEQUENCE type shared by a common block and interface bodies (docs/planning/
! PHASE-105 - Fortran 90 derived types.md, subtask 8). Every unit defines
! ACCOUNT again, with SEQUENCE and the same components, which makes them one
! type: the main program's interface bodies, which have no host, and the
! external procedures, which reach the book through COMMON /LEDGER/. POST
! changes the account it is given and counts the postings in the block; AUDIT
! prints the book; RICHEST returns an ACCOUNT, 18 bytes, through a hidden first
! argument, and is an output item itself.
program accounts
  implicit none
  type account
    sequence
    character(len=6) :: owner
    integer :: number
    double precision :: balance
  end type account
  interface
    subroutine post(a, amount)
      type account
        sequence
        character(len=6) :: owner
        integer :: number
        double precision :: balance
      end type account
      type(account), intent(inout) :: a
      double precision, intent(in) :: amount
    end subroutine post
    function richest()
      type account
        sequence
        character(len=6) :: owner
        integer :: number
        double precision :: balance
      end type account
      type(account) :: richest
    end function richest
  end interface
  type(account) :: book(3)
  integer :: nposts
  common /ledger/ book, nposts

  book = (/ account('ADAMS', 101, 50d0), account('BAKER', 102, 0d0), account('CLARK', 103, 125.5d0) /)
  nposts = 0
  call post(book(2), 300d0)
  call post(book(1), -20.25d0)
  call post(book(3), 10d0)
  call audit
  write (*, '(1x, a, i4, f10.2)') richest()
  write (*, '(1x, a, i3)') 'posts', nposts
end program accounts

subroutine post(a, amount)
  type account
    sequence
    character(len=6) :: owner
    integer :: number
    double precision :: balance
  end type account
  type(account), intent(inout) :: a
  double precision, intent(in) :: amount
  type(account) :: others(3)
  integer :: n
  common /ledger/ others, n

  a%balance = a%balance + amount
  n = n + 1
end subroutine post

subroutine audit
  type account
    sequence
    character(len=6) :: owner
    integer :: number
    double precision :: balance
  end type account
  type(account) :: book(3)
  integer :: n
  common /ledger/ book, n

  write (*, '(1x, a, i4, f10.2)') book
end subroutine audit

function richest()
  type account
    sequence
    character(len=6) :: owner
    integer :: number
    double precision :: balance
  end type account
  type(account) :: richest
  type(account) :: book(3)
  integer :: n, i
  common /ledger/ book, n

  richest = book(1)
  do i = 2, 3
    if (book(i)%balance > richest%balance) richest = book(i)
  end do
end function richest

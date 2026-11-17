! tokens.f90: a tokenizer by SELECT CASE on CHARACTER (docs/planning/PHASE-101
! - Fortran 90 control constructs.md, subtask 5). Each line typed is cut into
! names, numbers, strings, operators, and punctuation: the class of a token's
! first character chooses its case (twenty tests, so a binary search), a DO
! with no loop control reads lines until the input's end, named loops are left
! by EXIT and CYCLE from inside CASE blocks, and each token's kind is counted
! by a CASE construct on a passed-length dummy.
program tokens
  implicit none
  character(len=72) :: line
  character(len=8) :: kind
  character(len=8) :: names(8)
  integer :: ios, pos, first, last, nline, depth, k
  integer :: counts(8)
  integer, external :: lastnb, classof
  logical, external :: namechar
  data names / 'name', 'integer', 'real', 'string', 'operator', 'punct', &
       'error', 'unknown' /
  data counts / 8*0 /

  nline = 0
  lines: do
     read (*, '(a)', iostat=ios) line
     if (ios /= 0) exit lines
     nline = nline + 1
     last = lastnb(line)
     write (*, '(1x, a, i1, 2a)') 'line ', nline, ': ', line(1:last)
     depth = 0
     pos = 1
     scan: do while (pos <= last)
        first = pos
        select case (line(pos:pos))
        case (' ')
           pos = pos + 1
           cycle scan
        case ('!')
           exit scan
        case ('A':'Z', 'a':'z')
           kind = 'name'
           do
              pos = pos + 1
              if (pos > last) exit
              if (.not. namechar(line(pos:pos))) exit
           end do
        case ('0':'9')
           kind = 'integer'
           digits: do
              pos = pos + 1
              if (pos > last) exit digits
              select case (line(pos:pos))
              case ('0':'9')
              case ('.')
                 if (kind == 'real') exit digits
                 kind = 'real'
              case default
                 exit digits
              end select
           end do digits
        case ('''', '"')
           kind = 'string'
           quoted: do
              pos = pos + 1
              if (pos > last) then
                 kind = 'error'
                 exit quoted
              end if
              if (line(pos:pos) == line(first:first)) then
                 if (pos < last) then
                    if (line(pos+1:pos+1) == line(first:first)) then
                       pos = pos + 1
                       cycle quoted
                    end if
                 end if
                 pos = pos + 1
                 exit quoted
              end if
           end do quoted
        case ('*', '/', '=', '<', '>')
           kind = 'operator'
           select case (line(pos:min(pos + 1, last)))
           case ('**', '//', '==', '/=', '<=', '>=')
              pos = pos + 2
           case default
              pos = pos + 1
           end select
        case ('+', '-')
           kind = 'operator'
           pos = pos + 1
        case ('(')
           kind = 'punct'
           depth = depth + 1
           pos = pos + 1
        case (')')
           kind = 'punct'
           depth = depth - 1
           pos = pos + 1
        case (',')
           kind = 'punct'
           pos = pos + 1
        case default
           kind = 'unknown'
           pos = pos + 1
        end select
        k = classof(kind)
        counts(k) = counts(k) + 1
        write (*, '(3x, a, 1x, a)') kind, line(first:pos-1)
     end do scan
     if (depth /= 0) write (*, '(3x, a, i2)') 'unbalanced', depth
  end do lines

  write (*, '(1x, a, i2)') 'lines:', nline
  do k = 1, 8
     if (counts(k) == 0) cycle
     write (*, '(1x, a, i3)') names(k), counts(k)
  end do
end program tokens

! The position of a line's last character that isn't a blank, or 0: the loop
! variable keeps its value when EXIT leaves the loop.
integer function lastnb(s)
  implicit none
  character(len=*), intent(in) :: s
  integer :: i
  do i = len(s), 1, -1
     if (s(i:i) /= ' ') exit
  end do
  lastnb = i
end function lastnb

! Whether a character may continue a name: seven tests, a chain.
logical function namechar(c)
  implicit none
  character(len=1), intent(in) :: c
  select case (c)
  case ('A':'Z', 'a':'z', '0':'9', '_')
     namechar = .true.
  case default
     namechar = .false.
  end select
end function namechar

! A kind's place in the table of counts: eight values, a binary search.
integer function classof(kind)
  implicit none
  character(len=*), intent(in) :: kind
  select case (kind)
  case ('name')
     classof = 1
  case ('integer')
     classof = 2
  case ('real')
     classof = 3
  case ('string')
     classof = 4
  case ('operator')
     classof = 5
  case ('punct')
     classof = 6
  case ('error')
     classof = 7
  case default
     classof = 8
  end select
end function classof

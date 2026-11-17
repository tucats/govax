! Statements continued every way free form allows: a name split with
! the second &, a character constant across three lines, comment lines
! between, a ; on a continued line, and a Hollerith constant holding
! the characters free form gives meanings to.
subroutine continued(arg&
     &ument)
  character*40 text
  text = 'a character constant &
         &over three lines, &
   ! a comment line between them
         &with a ! in it'
  x = 1 + &
      2; y = 3 ;; z = 4 ;
30 format (6h!;&ab!, i3) ! the Hollerith text is not a comment
  call s ( &
      &1, 2 )
end

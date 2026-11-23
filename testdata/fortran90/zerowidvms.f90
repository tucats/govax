! Fortran 95's zero widths where govax answers as VMS would (Phase 112,
! subtask 12). A zero width is never asterisks: a value the record's room
! can't hold makes the record too long, FORRTL's OUTSTAOVE (IOSTAT= 66), and
! nothing of the record is written; a record's length is the processor's,
! and govax's is VMS's 133 columns. And a negative value that shows only
! zeros has no minus sign, the rule VMS's FORRTL gives every F field, where
! gfortran writes "-.00". VMS's own, so not in compare.py.
program zerowidvms
  implicit none
  integer :: ios

  write (*, '(1X,120X,F0.2)', iostat=ios) 1.0e30
  print '(1X,"A record too long: ",I0)', ios
  write (*, '(1X,100X,I0)', iostat=ios) 123456789
  print '(1X,"One that fits: ",I0)', ios
  print '(1X,A,F0.2,A)', '[', -0.001, ']'
  print '(1X,A,F0.0,A)', '[', -0.3, ']'
end program zerowidvms

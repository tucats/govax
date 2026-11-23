! Zero field widths on output: I0, B0, O0, Z0, F0.d.
program zerowid
  implicit none
  print '(A,I0,A)', '[', 42, ']'
  print '(A,I0,A)', '[', -7, ']'
  print '(A,B0,A)', '[', 10, ']'
  print '(A,O0,A)', '[', 64, ']'
  print '(A,Z0,A)', '[', 255, ']'
  print '(A,F0.2,A)', '[', 3.14159, ']'
  print '(A,F0.3,A)', '[', -0.5, ']'
  print '(A,I0.4,A)', '[', 5, ']'
end program zerowid

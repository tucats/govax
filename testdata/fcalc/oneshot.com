$! oneshot.com: fcalc as a foreign command (VMS and govax only: Fortran 90
$! has no way to read a command line, so gfortran's fcalc never sees one).
$! The text after FCALC is one line, run, and then fcalc ends; with none, it
$! reads lines, here from the procedure's own data lines. DCL compresses
$! blanks and makes letters capitals, which fcalc doesn't mind; a ! would
$! start DCL's comment.
$ fcalc = "$" + f$parse("FCALC.EXE")
$ fcalc 2^10 + 1
$ fcalc sqrt(-4) * (1 + i)
$ fcalc 1/0
$ fcalc @setup
$ fcalc save [nosuch]state
$ fcalc
set echo
1 + 2
@setup
$ exit

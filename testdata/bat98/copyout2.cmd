! COPYOUT2.CMD - copies Phase 98's probe results off bat98-exchange.dsk
! into testdata/bat98/vax (README.md). Written by testdata/bat98/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/copyout2.cmd
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/bat98-exchange.dsk"
DIRECTORY DUA5:[BAT98LOG],[BAT98SCR],[000000]*.OBJ
COPY DUA5:[BAT98LOG]BAT98B.LOG "testdata/bat98/vax/bat98b.log"/HOST/QUIET
DISMOUNT DUA5

! COPYOUT.CMD - copies Phase 94's probe results off priv94-exchange.dsk
! into testdata/priv94/vax (README.md). Written by testdata/priv94/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout.cmd
!
! The images come back byte for byte (/BINARY): a later oracle may run
! VMS's own.
MOUNT DUA5 "testdata/disks/priv94-exchange.dsk"
COPY DUA5:[PRIV94LOG]PRIV94.LOG "testdata/priv94/vax/priv94.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94U_PRIV94.LOG "testdata/priv94/vax/priv94u_priv94.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94U_PRIV94N.LOG "testdata/priv94/vax/priv94u_priv94n.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94Z.LOG "testdata/priv94/vax/priv94z.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94.EXE "testdata/priv94/vax/priv94.exe"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94H.EXE "testdata/priv94/vax/priv94h.exe"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94I.EXE "testdata/priv94/vax/priv94i.exe"/HOST/QUIET
DISMOUNT DUA5

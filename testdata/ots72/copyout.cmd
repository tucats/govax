! COPYOUT.CMD - copies the Phase 72 OTS$ probe's results off
! ots72-exchange.dsk into testdata/ots72/vax (README.md). Written by
! testdata/ots72/gen.go. Run from the repository root after
! @OTS72/OUTPUT=OTS72.LOG on the VAX:
!
!     govax console < testdata/ots72/copyout.cmd
!
MOUNT DUA2 "testdata/disks/ots72-exchange.dsk"
COPY DUA2:[000000]OTS72.OUT "testdata/ots72/vax/ots72-vax.txt"/HOST/QUIET
COPY DUA2:[000000]OTS72T.OUT "testdata/ots72/vax/ots72t-vax.txt"/HOST/QUIET
COPY DUA2:[000000]OTS72.LOG "testdata/ots72/vax/ots72.log"/HOST/QUIET
DISMOUNT DUA2

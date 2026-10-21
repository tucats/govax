! COPYOUT.CMD - copies Phase 71's probe results off ppf71-exchange.dsk
! into testdata/ppf71/vax (README.md). Run from the repository root
! after @PPF71/OUTPUT=PPF71.LOG on the VAX:
!
!     govax console < testdata/ppf71/copyout.cmd
!
! The image comes back byte for byte (/BINARY), so govax's oracle can
! run VMS's own image.
MOUNT DUA1 "testdata/disks/ppf71-exchange.dsk"
COPY DUA1:[000000]PPF71.LOG "testdata/ppf71/vax/ppf71.log"/HOST/QUIET
COPY DUA1:[000000]PPF71S.LOG "testdata/ppf71/vax/ppf71s.log"/HOST/QUIET
COPY DUA1:[000000]PPF71P.LOG "testdata/ppf71/vax/ppf71p.log"/HOST/QUIET
COPY DUA1:[000000]PPF71B.DAT "testdata/ppf71/vax/ppf71b.dat"/HOST/QUIET
COPY DUA1:[000000]PPF71.MAP "testdata/ppf71/vax/ppf71.map"/HOST/QUIET
COPY/BINARY DUA1:[000000]PPF71.EXE "testdata/ppf71/vax/ppf71.exe"/HOST/QUIET
DISMOUNT DUA1

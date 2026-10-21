! COPYOUT.CMD - copies Phase 70's probe results off tb70-exchange.dsk
! into testdata/tb70/vax (README.md). Run from the repository root
! after @TB70/OUTPUT=TB70.LOG on the VAX:
!
!     govax console < testdata/tb70/copyout.cmd
!
! The images come back byte for byte (/BINARY), so govax's
! oracle can run VMS's own images.
MOUNT DUA1 "testdata/disks/tb70-exchange.dsk"
COPY DUA1:[000000]TB70.LOG "testdata/tb70/vax/tb70.log"/HOST/QUIET
COPY DUA1:[000000]TB70.MAP "testdata/tb70/vax/tb70.map"/HOST/QUIET
COPY DUA1:[000000]TB70NT.MAP "testdata/tb70/vax/tb70nt.map"/HOST/QUIET
COPY DUA1:[000000]TB70D.MAP "testdata/tb70/vax/tb70d.map"/HOST/QUIET
COPY DUA1:[000000]TB70SHR.MAP "testdata/tb70/vax/tb70shr.map"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70.EXE "testdata/tb70/vax/tb70.exe"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70NT.EXE "testdata/tb70/vax/tb70nt.exe"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70D.EXE "testdata/tb70/vax/tb70d.exe"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70SHR.EXE "testdata/tb70/vax/tb70shr.exe"/HOST/QUIET
COPY DUA1:[000000]TB70.OBJ "testdata/tb70/vax/tb70.obj"/HOST/QUIET
COPY DUA1:[000000]TB70LONG.OBJ "testdata/tb70/vax/tb70long.obj"/HOST/QUIET
COPY DUA1:[000000]TB70SHR.OBJ "testdata/tb70/vax/tb70shr.obj"/HOST/QUIET
COPY DUA1:[000000]TB70D.OBJ "testdata/tb70/vax/tb70d.obj"/HOST/QUIET
COPY DUA1:[000000]TB70LONGD.OBJ "testdata/tb70/vax/tb70longd.obj"/HOST/QUIET
DISMOUNT DUA1

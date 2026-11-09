! COPYOUT.CMD - copies Phase 91's probe results off lnm91-exchange.dsk
! into testdata/lnm91/vax (README.md). Written by testdata/lnm91/gen.go.
! Run from the repository root after @LNM91/OUTPUT=LNM91.LOG on the VAX:
!
!     govax console < testdata/lnm91/copyout.cmd
!
! The image comes back byte for byte (/BINARY), so govax's oracle can run
! VMS's own image.
MOUNT DUA5 "testdata/disks/lnm91-exchange.dsk"
COPY DUA5:[000000]LNM91.LOG "testdata/lnm91/vax/lnm91.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]LNM91.EXE "testdata/lnm91/vax/lnm91.exe"/HOST/QUIET
DISMOUNT DUA5

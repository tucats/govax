! COPYOUTB.CMD - copies round 2's log off lnm91b-exchange.dsk into
! testdata/lnm91/vax (README.md). Written by testdata/lnm91/gen.go. Run from
! the repository root after @LNM91B/OUTPUT=LNM91B.LOG on the VAX:
!
!     govax console < testdata/lnm91/copyoutb.cmd
!
MOUNT DUA5 "testdata/disks/lnm91b-exchange.dsk"
COPY DUA5:[000000]LNM91B.LOG "testdata/lnm91/vax/lnm91b.log"/HOST/QUIET
DISMOUNT DUA5

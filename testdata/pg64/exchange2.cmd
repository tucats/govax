! EXCHANGE2.CMD - adds round 2 of Phase 64's probe to the existing
! exchange volume (README.md): the program with part X, PG64X.COM, and
! PG64M.COM. Run from the repository root:
!
!     govax console < testdata/pg64/exchange2.cmd
!
MOUNT/WRITE DUA1 "testdata/disks/pg64-exchange.dsk"
COPY "testdata/pg64/pg64.mar"/HOST DUA1:[000000]PG64.MAR
COPY "testdata/pg64/pg64x.com"/HOST DUA1:[000000]PG64X.COM
COPY "testdata/pg64/pg64m.com"/HOST DUA1:[000000]PG64M.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1

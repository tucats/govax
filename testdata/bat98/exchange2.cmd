! EXCHANGE2.CMD - adds round 2's files to Phase 98's probe exchange
! volume (README.md, "Round 2"). Written by testdata/bat98/gen.go. Run from
! the repository root, with the volume detached from the VAX:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/bat98-exchange.dsk"
COPY "testdata/bat98/bat98b.com"/HOST DUA5:[000000]BAT98B.COM
COPY "testdata/bat98/bat98q.mar"/HOST DUA5:[000000]BAT98Q.MAR
DIRECTORY/OWNER/PROTECTION DUA5:[000000]BAT98B.COM,BAT98Q.MAR
DISMOUNT DUA5

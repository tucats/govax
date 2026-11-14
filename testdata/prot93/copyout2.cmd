! COPYOUT2.CMD - copies round 2's logs off prot93-exchange.dsk into
! testdata/prot93/vax/round2 (README.md's "Round 2"). Written by
! testdata/prot93/gen.go. Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout2.cmd
!
MOUNT DUA5 "testdata/disks/prot93-exchange.dsk"
COPY DUA5:[PROT93LOG]PROT93R.LOG "testdata/prot93/vax/round2/prot93r.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93S.LOG "testdata/prot93/vax/round2/prot93s.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93V_PROT93A.LOG "testdata/prot93/vax/round2/prot93v_prot93a.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93V_PROT93B.LOG "testdata/prot93/vax/round2/prot93v_prot93b.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93V_PROT93C.LOG "testdata/prot93/vax/round2/prot93v_prot93c.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93Y.LOG "testdata/prot93/vax/round2/prot93y.log"/HOST/QUIET
DISMOUNT DUA5

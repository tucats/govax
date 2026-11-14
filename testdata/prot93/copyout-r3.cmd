! COPYOUT-R3.CMD - copies round 3's results off sec93-exchange.dsk:
! the logs, the map, and the program's image into testdata/prot93/vax/round3,
! and round 9 of the macro probes into testdata/mp/macros/vax. Written by
! testdata/prot93/gen3.go. Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout-r3.cmd
!
MOUNT DUA5 "testdata/disks/sec93-exchange.dsk"
COPY DUA5:[PROT93LOG]SEC93.LOG "testdata/prot93/vax/round3/sec93.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93U.LOG "testdata/prot93/vax/round3/sec93u.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X.LOG "testdata/prot93/vax/round3/sec93x.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93Y.LOG "testdata/prot93/vax/round3/sec93y.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_567.LOG "testdata/prot93/vax/round3/sec93x_567.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_568.LOG "testdata/prot93/vax/round3/sec93x_568.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_569.LOG "testdata/prot93/vax/round3/sec93x_569.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_570.LOG "testdata/prot93/vax/round3/sec93x_570.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_571.LOG "testdata/prot93/vax/round3/sec93x_571.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_572.LOG "testdata/prot93/vax/round3/sec93x_572.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_573.LOG "testdata/prot93/vax/round3/sec93x_573.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_574.LOG "testdata/prot93/vax/round3/sec93x_574.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_575.LOG "testdata/prot93/vax/round3/sec93x_575.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_576.LOG "testdata/prot93/vax/round3/sec93x_576.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_577.LOG "testdata/prot93/vax/round3/sec93x_577.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_578.LOG "testdata/prot93/vax/round3/sec93x_578.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_579.LOG "testdata/prot93/vax/round3/sec93x_579.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_580.LOG "testdata/prot93/vax/round3/sec93x_580.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_581.LOG "testdata/prot93/vax/round3/sec93x_581.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_582.LOG "testdata/prot93/vax/round3/sec93x_582.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_583.LOG "testdata/prot93/vax/round3/sec93x_583.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_584.LOG "testdata/prot93/vax/round3/sec93x_584.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_585.LOG "testdata/prot93/vax/round3/sec93x_585.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_586.LOG "testdata/prot93/vax/round3/sec93x_586.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_587.LOG "testdata/prot93/vax/round3/sec93x_587.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93X_588.LOG "testdata/prot93/vax/round3/sec93x_588.log"/HOST/QUIET
COPY DUA5:[SEC93]SEC93.MAP "testdata/prot93/vax/round3/sec93.map"/HOST/QUIET
COPY/BINARY DUA5:[SEC93]SEC93.EXE "testdata/prot93/vax/round3/sec93.exe"/HOST/QUIET
COPY/BINARY DUA5:[SEC93]SEC93.OBJ "testdata/prot93/vax/round3/sec93.obj"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_CHANGE_ACL.OBJ "testdata/mp/macros/vax/r9_change_acl.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_CHANGE_ACL.ANL "testdata/mp/macros/vax/r9_change_acl.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_PARSE_ACL.OBJ "testdata/mp/macros/vax/r9_parse_acl.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_PARSE_ACL.ANL "testdata/mp/macros/vax/r9_parse_acl.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_FORMAT_ACL.OBJ "testdata/mp/macros/vax/r9_format_acl.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_FORMAT_ACL.ANL "testdata/mp/macros/vax/r9_format_acl.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_CHKPRO.OBJ "testdata/mp/macros/vax/r9_chkpro.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_CHKPRO.ANL "testdata/mp/macros/vax/r9_chkpro.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_CHECK_ACCESS.OBJ "testdata/mp/macros/vax/r9_check_access.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_CHECK_ACCESS.ANL "testdata/mp/macros/vax/r9_check_access.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_GET_SECURITY.OBJ "testdata/mp/macros/vax/r9_get_security.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_GET_SECURITY.ANL "testdata/mp/macros/vax/r9_get_security.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_SET_SECURITY.OBJ "testdata/mp/macros/vax/r9_set_security.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_SET_SECURITY.ANL "testdata/mp/macros/vax/r9_set_security.anl"/HOST/QUIET
COPY/BINARY DUA5:[SEC93MAC]R9_SETDFPROT.OBJ "testdata/mp/macros/vax/r9_setdfprot.obj"/HOST/QUIET
COPY DUA5:[SEC93MAC]R9_SETDFPROT.ANL "testdata/mp/macros/vax/r9_setdfprot.anl"/HOST/QUIET
COPY DUA5:[SEC93MAC]MACROS9.LOG "testdata/mp/macros/vax/macros9.log"/HOST/QUIET
DISMOUNT DUA5

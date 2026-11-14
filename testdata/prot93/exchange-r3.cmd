! EXCHANGE-R3.CMD - builds round 3 of Phase 93's probe on a new volume.
! Written by testdata/prot93/gen3.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange-r3.cmd
!
INITIALIZE/CONTAINER "testdata/disks/sec93-exchange.dsk" /DEVICE=RD53 SEC93
MOUNT/WRITE DUA5 "testdata/disks/sec93-exchange.dsk"
CREATE/DIRECTORY DUA5:[PROT93LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SEC93]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SEC93MAC]
COPY "testdata/prot93/sec93.mar"/HOST DUA5:[000000]SEC93.MAR
COPY "testdata/prot93/sec93.com"/HOST DUA5:[000000]SEC93.COM
COPY "testdata/prot93/sec93u.com"/HOST DUA5:[000000]SEC93U.COM
COPY "testdata/prot93/sec93y.com"/HOST DUA5:[000000]SEC93Y.COM
COPY "testdata/prot93/sec93t.dat"/HOST DUA5:[PROT93LOG]T.DAT
COPY "testdata/mp/macros/r9_change_acl.mar"/HOST DUA5:[SEC93MAC]R9_CHANGE_ACL.MAR
COPY "testdata/mp/macros/r9_parse_acl.mar"/HOST DUA5:[SEC93MAC]R9_PARSE_ACL.MAR
COPY "testdata/mp/macros/r9_format_acl.mar"/HOST DUA5:[SEC93MAC]R9_FORMAT_ACL.MAR
COPY "testdata/mp/macros/r9_chkpro.mar"/HOST DUA5:[SEC93MAC]R9_CHKPRO.MAR
COPY "testdata/mp/macros/r9_check_access.mar"/HOST DUA5:[SEC93MAC]R9_CHECK_ACCESS.MAR
COPY "testdata/mp/macros/r9_get_security.mar"/HOST DUA5:[SEC93MAC]R9_GET_SECURITY.MAR
COPY "testdata/mp/macros/r9_set_security.mar"/HOST DUA5:[SEC93MAC]R9_SET_SECURITY.MAR
COPY "testdata/mp/macros/r9_setdfprot.mar"/HOST DUA5:[SEC93MAC]R9_SETDFPROT.MAR
COPY "testdata/mp/macros/macros9.com"/HOST DUA5:[SEC93MAC]MACROS9.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000],[PROT93LOG],[SEC93MAC]
DISMOUNT DUA5

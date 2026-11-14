! EXCHANGE.CMD - builds Phase 93's probe exchange volume with govax.
! Written by testdata/prot93/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/prot93-exchange.dsk" /DEVICE=RD53 PROT93
MOUNT/WRITE DUA5 "testdata/disks/prot93-exchange.dsk"
CREATE/DIRECTORY DUA5:[PROT93LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/prot93/prot93.mar"/HOST DUA5:[000000]PROT93.MAR
COPY "testdata/prot93/hello.mar"/HOST DUA5:[000000]HELLO.MAR
COPY "testdata/prot93/prot93.com"/HOST DUA5:[000000]PROT93.COM
COPY "testdata/prot93/prot93u.com"/HOST DUA5:[000000]PROT93U.COM
COPY "testdata/prot93/prot93b.com"/HOST DUA5:[000000]PROT93B.COM
COPY "testdata/prot93/prot93z.com"/HOST DUA5:[000000]PROT93Z.COM
COPY "testdata/prot93/prot93c.dat"/HOST DUA5:[000000]PROT93C.DAT
COPY "testdata/prot93/prot93d.dat"/HOST DUA5:[000000]PROT93D.DAT
COPY "testdata/prot93/prot93e.dat"/HOST DUA5:[000000]PROT93E.DAT
COPY "testdata/prot93/def_chp.mar"/HOST DUA5:[000000]DEF_CHP.MAR
COPY "testdata/prot93/def_oss.mar"/HOST DUA5:[000000]DEF_OSS.MAR
COPY "testdata/prot93/def_ace.mar"/HOST DUA5:[000000]DEF_ACE.MAR
COPY "testdata/prot93/def_acl.mar"/HOST DUA5:[000000]DEF_ACL.MAR
COPY "testdata/prot93/def_arm.mar"/HOST DUA5:[000000]DEF_ARM.MAR
COPY "testdata/prot93/def_kgb.mar"/HOST DUA5:[000000]DEF_KGB.MAR
COPY "testdata/prot93/def_prv.mar"/HOST DUA5:[000000]DEF_PRV.MAR
DIRECTORY/OWNER/PROTECTION DUA5:[000000]
DISMOUNT DUA5

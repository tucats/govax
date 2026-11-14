! COPYOUT.CMD - copies Phase 93's probe results off prot93-exchange.dsk
! into testdata/prot93/vax (README.md). Written by testdata/prot93/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout.cmd
!
! The program and the definition objects come back byte for byte
! (/BINARY): a later oracle runs VMS's own image, and decode.go reads the
! objects' values.
MOUNT DUA5 "testdata/disks/prot93-exchange.dsk"
COPY DUA5:[PROT93LOG]PROT93.LOG "testdata/prot93/vax/prot93.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93U_PROT93A.LOG "testdata/prot93/vax/prot93u_prot93a.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93U_PROT93B.LOG "testdata/prot93/vax/prot93u_prot93b.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93U_PROT93C.LOG "testdata/prot93/vax/prot93u_prot93c.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]PROT93Z.LOG "testdata/prot93/vax/prot93z.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]PROT93.EXE "testdata/prot93/vax/prot93.exe"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_CHP.OBJ "testdata/prot93/vax/def_chp.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_OSS.OBJ "testdata/prot93/vax/def_oss.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_ACE.OBJ "testdata/prot93/vax/def_ace.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_ACL.OBJ "testdata/prot93/vax/def_acl.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_ARM.OBJ "testdata/prot93/vax/def_arm.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_KGB.OBJ "testdata/prot93/vax/def_kgb.obj"/HOST/QUIET
COPY/BINARY DUA5:[000000]DEF_PRV.OBJ "testdata/prot93/vax/def_prv.obj"/HOST/QUIET
DISMOUNT DUA5

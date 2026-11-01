! EXCHANGE.CMD - builds the Phase 82 LINK probe's exchange volume with govax.
! Written by testdata/link82/gen.go. Run from the repository root:
!
!     govax console < testdata/link82/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/link82-exchange.dsk" /DEVICE=RD54 LINK82
MOUNT/WRITE DUA2 "testdata/disks/link82-exchange.dsk"
COPY "testdata/link82/attrg.opt"/HOST DUA2:[000000]ATTRG.OPT
COPY "testdata/link82/attrq.opt"/HOST DUA2:[000000]ATTRQ.OPT
COPY "testdata/link82/attrro.opt"/HOST DUA2:[000000]ATTRRO.OPT
COPY "testdata/link82/attrwrt.opt"/HOST DUA2:[000000]ATTRWRT.OPT
COPY "testdata/link82/base.mar"/HOST DUA2:[000000]BASE.MAR
COPY "testdata/link82/base2.mar"/HOST DUA2:[000000]BASE2.MAR
COPY "testdata/link82/cl1.opt"/HOST DUA2:[000000]CL1.OPT
COPY "testdata/link82/cl2.opt"/HOST DUA2:[000000]CL2.OPT
COPY "testdata/link82/cl3.opt"/HOST DUA2:[000000]CL3.OPT
COPY "testdata/link82/cl4.opt"/HOST DUA2:[000000]CL4.OPT
COPY "testdata/link82/cl5.opt"/HOST DUA2:[000000]CL5.OPT
COPY "testdata/link82/coll.opt"/HOST DUA2:[000000]COLL.OPT
COPY "testdata/link82/lbase.mar"/HOST DUA2:[000000]LBASE.MAR
COPY "testdata/link82/lgbl.mar"/HOST DUA2:[000000]LGBL.MAR
COPY "testdata/link82/link82.com"/HOST DUA2:[000000]LINK82.COM
COPY "testdata/link82/link82a.com"/HOST DUA2:[000000]LINK82A.COM
COPY "testdata/link82/link82c.com"/HOST DUA2:[000000]LINK82C.COM
COPY "testdata/link82/lnowrt.mar"/HOST DUA2:[000000]LNOWRT.MAR
COPY "testdata/link82/lovr.mar"/HOST DUA2:[000000]LOVR.MAR
COPY "testdata/link82/lquad.mar"/HOST DUA2:[000000]LQUAD.MAR
COPY "testdata/link82/lshr.mar"/HOST DUA2:[000000]LSHR.MAR
COPY "testdata/link82/mainl.mar"/HOST DUA2:[000000]MAINL.MAR
COPY "testdata/link82/mainn.mar"/HOST DUA2:[000000]MAINN.MAR
COPY "testdata/link82/mainp.mar"/HOST DUA2:[000000]MAINP.MAR
COPY "testdata/link82/va4.mar"/HOST DUA2:[000000]VA4.MAR
COPY "testdata/link82/vabs.mar"/HOST DUA2:[000000]VABS.MAR
COPY "testdata/link82/vbyte.mar"/HOST DUA2:[000000]VBYTE.MAR
COPY "testdata/link82/vcon.mar"/HOST DUA2:[000000]VCON.MAR
COPY "testdata/link82/vexe.mar"/HOST DUA2:[000000]VEXE.MAR
COPY "testdata/link82/vlcl.mar"/HOST DUA2:[000000]VLCL.MAR
COPY "testdata/link82/vlcl2.mar"/HOST DUA2:[000000]VLCL2.MAR
COPY "testdata/link82/vlib.mar"/HOST DUA2:[000000]VLIB.MAR
COPY "testdata/link82/vmulti.mar"/HOST DUA2:[000000]VMULTI.MAR
COPY "testdata/link82/vnopic.mar"/HOST DUA2:[000000]VNOPIC.MAR
COPY "testdata/link82/vnord.mar"/HOST DUA2:[000000]VNORD.MAR
COPY "testdata/link82/vnoshr.mar"/HOST DUA2:[000000]VNOSHR.MAR
COPY "testdata/link82/vnowrt.mar"/HOST DUA2:[000000]VNOWRT.MAR
COPY "testdata/link82/vpage.mar"/HOST DUA2:[000000]VPAGE.MAR
COPY "testdata/link82/vquad.mar"/HOST DUA2:[000000]VQUAD.MAR
COPY "testdata/link82/vvec.mar"/HOST DUA2:[000000]VVEC.MAR
COPY "testdata/link82/vword.mar"/HOST DUA2:[000000]VWORD.MAR
DIRECTORY DUA2:[000000]
DISMOUNT DUA2

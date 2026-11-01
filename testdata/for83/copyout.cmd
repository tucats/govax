! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for83/gen.go. Run from the repository root:
!
!     govax console < testdata/for83/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for83-exchange.dsk"
COPY DUA2:[000000]STR83-VAX.TXT "testdata/str83/vax/str83-vax.txt"/HOST
COPY DUA2:[000000]FOR83-VAX.TXT "testdata/for83/vax/for83-vax.txt"/HOST
COPY DUA2:[000000]FOR83B-VAX.TXT "testdata/for83/vax/for83b-vax.txt"/HOST
COPY DUA2:[000000]FOR83C-VAX.TXT "testdata/for83/vax/for83c-vax.txt"/HOST
COPY DUA2:[000000]FOR83.LOG "testdata/for83/vax/for83.log"/HOST
COPY DUA2:[000000]DEF_OPC.ANL "testdata/for83/vax/def_opc.anl"/HOST
COPY DUA2:[000000]E1.TXT "testdata/for83/vax/e1.txt"/HOST
COPY DUA2:[000000]E2.TXT "testdata/for83/vax/e2.txt"/HOST
COPY DUA2:[000000]E4.TXT "testdata/for83/vax/e4.txt"/HOST
COPY DUA2:[000000]E6.TXT "testdata/for83/vax/e6.txt"/HOST
COPY DUA2:[000000]S1.TLB "testdata/for83/vax/s1.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S2.TLB "testdata/for83/vax/s2.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S3.TLB "testdata/for83/vax/s3.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S4.TLB "testdata/for83/vax/s4.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S5.TLB "testdata/for83/vax/s5.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S6.TLB "testdata/for83/vax/s6.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S7.TLB "testdata/for83/vax/s7.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S8.TLB "testdata/for83/vax/s8.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]S9.TLB "testdata/for83/vax/s9.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]T1.TLB "testdata/for83/vax/t1.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]T2.TLB "testdata/for83/vax/t2.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]T3.TLB "testdata/for83/vax/t3.tlb"/HOST/BINARY/QUIET
COPY DUA2:[000000]DEF_OPC.OBJ "testdata/for83/vax/def_opc.obj"/HOST/BINARY/QUIET
DISMOUNT DUA2

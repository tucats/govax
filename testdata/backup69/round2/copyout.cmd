! COPYOUT.CMD - copies Phase 69's second round's results off
! backup69-round2.dsk into testdata/backup69/vax/round2/ (README.md). Run
! from the repository root after @RUN2 on the VAX:
!
!     govax console < testdata/backup69/round2/copyout.cmd
!
! govax's save sets come back byte for byte (/BINARY) into govax/, as
! VMS read them, for TestBackup69Round2 to replay RUN2.COM on.
MOUNT DUA1 "testdata/disks/backup69-round2.dsk"
COPY DUA1:[000000]RUN2.LOG "testdata/backup69/vax/round2/run2.log"/HOST/QUIET
COPY DUA1:[RESULTS]TREE2.LST "testdata/backup69/vax/round2/tree2.lst"/HOST/QUIET
COPY DUA1:[RESULTS]SETS2.LST "testdata/backup69/vax/round2/sets2.lst"/HOST/QUIET
COPY DUA1:[RESULTS]VB69.LST "testdata/backup69/vax/round2/vb69.lst"/HOST/QUIET
COPY DUA1:[RESULTS]B69FULL2.LIS "testdata/backup69/vax/round2/b69full2.lis"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69.BCK "testdata/backup69/vax/round2/govax/b69.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69K8.BCK "testdata/backup69/vax/round2/govax/b69k8.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69G0.BCK "testdata/backup69/vax/round2/govax/b69g0.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69G3.BCK "testdata/backup69/vax/round2/govax/b69g3.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69CRC.BCK "testdata/backup69/vax/round2/govax/b69crc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69NOCRC.BCK "testdata/backup69/vax/round2/govax/b69nocrc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69COM.BCK "testdata/backup69/vax/round2/govax/b69com.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69EXC.BCK "testdata/backup69/vax/round2/govax/b69exc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69BYO.BCK "testdata/backup69/vax/round2/govax/b69byo.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69TXT.BCK "testdata/backup69/vax/round2/govax/b69txt.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69NONE.BCK "testdata/backup69/vax/round2/govax/b69none.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]TEXT.BCK "testdata/backup69/vax/round2/govax/text.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69OWN.BCK "testdata/backup69/vax/round2/govax/b69own.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]LIST.BCK "testdata/backup69/vax/round2/govax/list.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69IGN.BCK "testdata/backup69/vax/round2/govax/b69ign.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]HOST.BCK "testdata/backup69/vax/round2/govax/host.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69D.BCK "testdata/backup69/vax/round2/b69d.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69CRCD.BCK "testdata/backup69/vax/round2/b69crcd.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69HDRD.BCK "testdata/backup69/vax/round2/b69hdrd.bck"/HOST/QUIET
DISMOUNT DUA1

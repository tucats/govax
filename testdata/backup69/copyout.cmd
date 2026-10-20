! COPYOUT.CMD - copies Phase 69's probe results off backup69-exchange.dsk
! into the repository (README.md). Run from the repository root after
! @BACKUP69 on the VAX:
!
!     govax console < testdata/backup69/copyout.cmd
!
! The save sets come back byte for byte (/BINARY). B69NONE.BCK, a save
! that selects no files, may not exist; then its line reports so.
MOUNT DUA1 "testdata/disks/backup69-exchange.dsk"
COPY DUA1:[000000]BACKUP69.LOG "testdata/backup69/vax/backup69.log"/HOST/QUIET
COPY DUA1:[SETS]TREE.LST "testdata/backup69/vax/tree.lst"/HOST/QUIET
COPY DUA1:[SETS]SETS.LST "testdata/backup69/vax/sets.lst"/HOST/QUIET
COPY DUA1:[SETS]R1.LST "testdata/backup69/vax/r1.lst"/HOST/QUIET
COPY DUA1:[SETS]R7.LST "testdata/backup69/vax/r7.lst"/HOST/QUIET
COPY DUA1:[SETS]TEXT.LIS "testdata/backup69/vax/text.lis"/HOST/QUIET
COPY DUA1:[SETS]B69FULL.LIS "testdata/backup69/vax/b69full.lis"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69.BCK "testdata/backup69/vax/b69.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69K8.BCK "testdata/backup69/vax/b69k8.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69G0.BCK "testdata/backup69/vax/b69g0.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69G3.BCK "testdata/backup69/vax/b69g3.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69CRC.BCK "testdata/backup69/vax/b69crc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69NOCRC.BCK "testdata/backup69/vax/b69nocrc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69COM.BCK "testdata/backup69/vax/b69com.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69EXC.BCK "testdata/backup69/vax/b69exc.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69BYO.BCK "testdata/backup69/vax/b69byo.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69TXT.BCK "testdata/backup69/vax/b69txt.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69NONE.BCK "testdata/backup69/vax/b69none.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]TEXT.BCK "testdata/backup69/vax/text.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69OWN.BCK "testdata/backup69/vax/b69own.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]LIST.BCK "testdata/backup69/vax/list.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69IGN.BCK "testdata/backup69/vax/b69ign.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69CUT.BCK "testdata/backup69/vax/b69cut.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69CRCD.BCK "testdata/backup69/vax/b69crcd.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69G0D.BCK "testdata/backup69/vax/b69g0d.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69NOCRCD.BCK "testdata/backup69/vax/b69nocrcd.bck"/HOST/QUIET
COPY/BINARY DUA1:[SETS]B69HDRD.BCK "testdata/backup69/vax/b69hdrd.bck"/HOST/QUIET
DISMOUNT DUA1

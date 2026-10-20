! EXCHANGE.CMD - builds the exchange volume for Phase 69's second round
! (testdata/backup69/round2/README.md): govax restores round 1's tree
! from VMS's save sets, makes Part 2's save sets of it, and writes one
! save set to the host, which is copied onto the volume as a binary
! transfer would leave it. Then RUN2.COM and SSCOPY.MAR. Run from the
! repository root:
!
!     govax console < testdata/backup69/round2/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/backup69-round2.dsk" /DEVICE=RD54 /CLUSTER=3 BACKUP69
MOUNT/WRITE DUA1 "testdata/disks/backup69-round2.dsk"
SET DEFAULT DUA1:[000000]
!
! Round 1's tree, as VMS saved it, with every saved attribute.
!
BACKUP "testdata/backup69/vax/b69.bck"/SAVE_SET [*...]/BY_OWNER=ORIGINAL
BACKUP "testdata/backup69/vax/b69ign.bck"/SAVE_SET [*...]/BY_OWNER=ORIGINAL/OVERLAY
CREATE/DIRECTORY [SETS]
CREATE/DIRECTORY [RESULTS]
!
! Part 2's saves, made by govax. /SINCE and /BEFORE name round 1's day,
! when the tree's files were made, where Part 2 said TODAY.
!
BACKUP/LOG [B69...]*.* [SETS]B69.BCK/SAVE_SET
BACKUP [B69...]*.* [SETS]B69K8.BCK/SAVE_SET/BLOCK_SIZE=8192
BACKUP [B69...]*.* [SETS]B69G0.BCK/SAVE_SET/GROUP_SIZE=0
BACKUP [B69...]*.* [SETS]B69G3.BCK/SAVE_SET/GROUP_SIZE=3
BACKUP [B69...]*.* [SETS]B69CRC.BCK/SAVE_SET/CRC
BACKUP [B69...]*.* [SETS]B69NOCRC.BCK/SAVE_SET/NOCRC
BACKUP [B69...]*.* [SETS]B69COM.BCK/SAVE_SET/COMMENT="Phase 69's probe, with a comment"
BACKUP [B69...]*.*/EXCLUDE=(*.DAT,[B69.SUB1.SUB2]*.*) [SETS]B69EXC.BCK/SAVE_SET
BACKUP [B69...]*.*/BY_OWNER=[200,201] [SETS]B69BYO.BCK/SAVE_SET
BACKUP [B69]*.TXT;*/SINCE=20-OCT-2026 [SETS]B69TXT.BCK/SAVE_SET
BACKUP/LOG [B69...]*.*/BEFORE=20-OCT-2026 [SETS]B69NONE.BCK/SAVE_SET
BACKUP [B69]TEXT.TXT [SETS]TEXT.BCK/SAVE_SET
BACKUP [B69]TEXT.TXT [SETS]B69OWN.BCK/SAVE_SET/BY_OWNER=[200,201]
BACKUP [B69]TEXT.TXT,[B69.SUB1]A.TXT [SETS]LIST.BCK/SAVE_SET
BACKUP [B69]NOBACK.TXT/IGNORE=NOBACKUP [SETS]B69IGN.BCK/SAVE_SET
!
! A save set govax writes to the host, then copied onto the volume byte
! for byte: fixed 512-byte records, not the save set's 32256.
!
BACKUP [B69...]*.* "testdata/disks/backup69-host.bck"/SAVE_SET/CRC
COPY/BINARY "testdata/disks/backup69-host.bck" [SETS]HOST.BCK
!
COPY "testdata/backup69/round2/run2.com"/HOST DUA1:[000000]RUN2.COM
COPY "testdata/backup69/sscopy.mar"/HOST DUA1:[000000]SSCOPY.MAR
DIRECTORY/SIZE=ALL [SETS]
SET DEFAULT DUA0:[000000]
DISMOUNT DUA1

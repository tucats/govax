# Phase 69's save sets

`docs/PHASE-69 - backup save sets.md` is the phase; its Part A describes
the save set format as these files show it.

## The probe (subtask 2)

One command procedure, `BACKUP69.COM`, run once on the simh VAX on a
fresh exchange volume, makes a tree of files, saves it with each output
save-set qualifier the phase uses, lists the save sets, restores them
every way the phase supports, compares, and tries the errors, among
them damaged and truncated save sets that `SSCOPY` makes from VMS's own.

| File | What it holds |
| ---- | ------------- |
| `backup69.com` | The procedure (below) |
| `sscopy.mar` | SSCOPY: copies a save set record by record (one save set block each), optionally stopping after n records or complementing one byte of one record. It reads its five arguments from SYS$INPUT: input, output, records to copy (0 for all), record to change (0 for none), byte offset. Tested on govax first |
| `exchange.cmd` | The govax console script that builds the exchange volume (with `testdata/mar/pi.mar`, which the procedure assembles and links) |
| `copyout.cmd` | The govax console script that copies the results back into `vax/`, the save sets byte for byte (COPY/BINARY) |

### What the procedure does

| Part | What it asks |
| ---- | ------------ |
| 1, the tree | `[B69]`, `[B69.SUB1]` (version limit 5), `[B69.SUB1.SUB2]`. TEXT.TXT (variable, CR) and CONVERT's copies of it as stream LF (STMLF.TXT), fixed 80 (FIX80.DAT), VFC with print control (VFC.TXT); an indexed file (INDEXED.IDX); an empty file; PI.OBJ and PI.EXE (an image, fixed 512, contiguous best try); MEDIUM.DAT (about 74 blocks) and BIG.DAT (about 169), larger than a save set block; ACL.TXT (two ACEs), EXPIRE.TXT, VERLIM.TXT (two versions, limit 3), MULTI.TXT (three versions of different formats), OWNED.TXT ([200,201]), NOBACK.TXT (NOBACKUP), PROT.TXT, CONTIG.DAT (contiguous). `TREE.LST` is their DIRECTORY/FULL |
| 2, saves | B69 (defaults, `/LOG`), B69K8 (`/BLOCK_SIZE=8192`), B69G0 and B69G3 (`/GROUP_SIZE=0`, `3`), B69CRC and B69NOCRC (`/CRC`, `/NOCRC`), B69COM (`/COMMENT`), B69EXC (input `/EXCLUDE`), B69BYO (input `/BY_OWNER`), B69TXT (input `/SINCE`, all versions of the .TXT files), B69NONE (input `/BEFORE=TODAY`: no files), TEXT (one file), B69OWN (output `/BY_OWNER`: the save set's owner), LIST (an input list), B69IGN (`/IGNORE=NOBACKUP`). `SETS.LST` is the save set files' DIRECTORY/FULL |
| 3, listings | BACKUP/LIST of each; `/FULL` of B69 and B69COM; `/BRIEF`; `/LIST=file` (TEXT.LIS, and B69FULL.LIS with `/FULL`); `/SELECT` and `/EXCLUDE` on a listing |
| 4, restores | B69 to `[R1...]` (`R1.LST`, the restored headers); `/SELECT=[B69.SUB1...]` to `[R3...]`; everything flat into `[R4]`; `/SELECT=*.TXT`; `/EXCLUDE=*.DAT`; one file to `[R7]`; to the default directory as `*.*` and `[]`; a new name (`[R9]NEW.TXT`); one version of three; `/BY_OWNER=ORIGINAL`, `DEFAULT`, `PARENT`, a UIC, and none (`[R10A]` to `[R10E]`); an existing file without a version qualifier, then `/NEW_VERSION`, `/OVERLAY`, `/REPLACE` (`R7.LST`); the 8K, G0, G3, NOCRC, and IGN save sets restored |
| 5, compares | B69TXT against the tree; TEXT against a restored copy, the copy with a line added, and the copy deleted; then a restore to `[*...]`, where the file exists, without and with `/NEW_VERSION` |
| 6, errors | `/LIST` without `/SAVE_SET`; a missing save set; a text file as a save set; a `/SELECT` that matches nothing. SSCOPY's copies: B69CUT (B69's first 2 blocks), B69CRCD, B69G0D, B69NOCRCD (byte 5000 of block 2 complemented), B69HDRD (byte 100 of block 2, in its header). Each is listed and restored to `[Dname...]` |

Every BACKUP that may fail is followed by `SHOW SYMBOL $STATUS`.

### Running it

1. From the repository root, build the exchange volume:

       govax console < testdata/backup69/exchange.cmd

   This makes `testdata/disks/backup69-exchange.dsk` (gitignored), an
   RD54 labeled BACKUP69. The probe needs room for about 20 save sets
   and a dozen restored trees, a few thousand blocks in all.
2. On the VAX, mount it and run the procedure once, with its `[000000]`
   as the default directory:

       $ MOUNT DUA1: BACKUP69
       $ SET DEFAULT DUA1:[000000]
       $ @BACKUP69/OUTPUT=BACKUP69.LOG

   The procedure needs SYSTEM's privileges (SET FILE/OWNER_UIC, the
   restores' owners). It defines no logical names. A second run on the
   same volume would find its own files, so rebuild the volume first.
3. Back on the host, copy the results into `vax/`:

       govax console < testdata/backup69/copyout.cmd

## `vax/` — the first save set (2026-10-20)

Made by the author on the simh VAX (OpenVMS VAX V7.1, BACKUP V6.2) and
brought over on the local-only container `testdata/disks/xfer.dsk`:

```
$ SET DEFAULT DUA1:[XFER]
$ BACKUP [.PI]*.* PI.BCK/SAVE
```

| File | What it is |
| --- | --- |
| `pi.bck` | `DUA1:[XFER]PI.BCK;1`, the save set: 126 blocks, fixed-length 32256-byte records, copied off the volume block for block (two save set blocks) |
| `pi.exe`, `pi.mar`, `pi.obj` | `DUA1:[XFER.PI]`'s three files, the ones saved, block for block up to each file's end of file. They are what a restore must give back. `pi.mar` is `testdata/mar/pi.mar` as VMS keeps it: variable-length records with carriage-return carriage control |

The files' attributes on the VAX volume (DIRECTORY/FULL):

| File | File ID | Used/allocated | Record format | Created | Revised |
| --- | --- | --- | --- | --- | --- |
| PI.EXE;1 | (19,1,0) | 6/6 | fixed 512, none; contiguous best try | 20-OCT-2026 10:44:31.84 | same (1) |
| PI.MAR;1 | (18,1,0) | 24/24 | variable, longest 79, CR | 20-OCT-2026 10:41:49.47 | 10:44:09.82 (2) |
| PI.OBJ;1 | (16,2,0) | 5/6, extend 20 | variable, longest 214, none | 20-OCT-2026 10:44:29.98 | 10:44:30.11 (1) |

All are owned by [SYSTEM] ([1,4]) with protection
(S:RWED,O:RWED,G:RE,W), in directory [XFER.PI], file ID (17,1,0).

VMS's listing of the save set:

```
$ backup/list pi.bck/save
Listing of save set(s)

Save set:          PI.BCK
Written by:        SYSTEM      
UIC:               [000001,000004]
Date:              20-OCT-2026 10:45:22.88
Command:           BACKUP [.PI]*.* PI.BCK/SAVE
Operating system:  OpenVMS VAX version V7.1
BACKUP version:    V6.2
CPU ID register:   040704D2
Written on:        _SIMVAX$DUA1:
Block size:        32256
Group size:        10
Buffer count:      8

[XFER.PI]PI.EXE;1                                           6  20-OCT-2026 10:44
[XFER.PI]PI.MAR;1                                          24  20-OCT-2026 10:41
[XFER.PI]PI.OBJ;1                                           5  20-OCT-2026 10:44

Total of 3 files, 35 blocks
End of save set
```

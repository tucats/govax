# Phase 69's save sets

`docs/PHASE-69 - backup save sets.md` is the phase; its Part A describes
the save set format as these files show it.

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

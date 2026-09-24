# Disk Device Attributes

This chart shows various disk drive phsyical device types, and their
geometry. This chart can be used to map device classes onto container
volumes, etc.

Legend for important fields:

- **type** is the device type name
- **surf** is the number of magnetic surfaces
- **sec** is the sector count for the device
- **cyl** is the cylinder count for the device
- **LBNs** is the numbrer of logical blocks of data available for the user

Blocks are always 512-bytes in size.

| type | sec | surf |    cyl |     LBNs   |
| ---- | --- | ---- | ------ | ---------- |
| RX50 | 10  |   1  |    80  |        800 |
| RX33 | 15  |   2  |    80  |       2400 |
| RD51 | 18  |   4  |   306  |      21600 |
| RD31 | 17  |   4  |   615  |      41560 |
| RD52 | 17  |   8  |   512  |      60480 |
| RD32 | 17  |   6  |   820  |      83204 |
| RD33 | 17  |   7  |  1170  |     138565 |
| RD53 | 17  |   8  |  1024  |     138672 |
| RD54 | 17  |   15 |  1225  |     311200 |
| RA60 | 42  |   6  |  1600  |     400176 |
| RA70 | 33  |   11 |  1507  |     547041 |
| RA80 | 31  |   14 |  546   |     237212 |
| RA81 | 51  |   14 |  1258  |     891072 |
| RA82 | 57  |   15 |  1435  |    1216665 |
| RA71 | 51  |   14 |  1921  |    1367310 |
| RA72 | 51  |   20 |  1921  |    1953300 |
| RA90 | 69  |   13 |  2656  |    2376153 |
| RA92 | 73  |   13 |  3101  |    2940951 |
| RA73 | 70  |   21 |  2667  |    3920490 |

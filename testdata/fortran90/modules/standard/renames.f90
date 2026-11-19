! 11.3.3.8, public entities renamed, and 12.1.2.2.2, host association and
! use association: the standard's modules as it writes them.

MODULE J; REAL JX, JY, JZ; END MODULE
MODULE K
USE J, ONLY : KX => JX, KY => JY
! KX and KY are local names to module K
REAL KZ
! KZ is local name to module K
REAL JZ
! JZ is local name to module K
END MODULE

MODULE B; REAL BX, Q; INTEGER IX, JX; END MODULE
MODULE C; REAL CX; END MODULE
MODULE D; REAL DX, DY, DZ; END MODULE
MODULE E; REAL EX, EY, EZ; END MODULE
MODULE F; REAL FX; END MODULE
MODULE G; USE F; REAL GX; END MODULE

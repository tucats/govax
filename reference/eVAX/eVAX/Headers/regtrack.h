
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     regtrack.h
//
//  Purpose:    This contains definitions and prototypes for supporting register
//		tracking, used during TRACE and STEP functions.
//
//  History:    10/22/01    Separated out register tracking functionality.
//

struct REGSET {
    ULONGWORD	reg[ 16 ];
    ULONGWORD	psl;
};

int save_regset( struct REGSET * regset );
int check_regset( struct REGSET * regset );


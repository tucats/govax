//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     include.h
//
//  Purpose:    This module contains definitions for data structures
//              used to store the nested stack of include files.
//
//  History:    02/15/98    Initial creation
//
//



struct INCLUDE {
    struct INCLUDE  *next;
    FILE            *fp;
    LONGWORD            verify;
    LONGWORD            assembler_mode;
    char            fname[ 1 ];
};


//  Prototypes routines (located in driver.c) for handling the
//  include file stack.

LONGWORD push_include( char * name, char * extension );

LONGWORD pop_include(void);

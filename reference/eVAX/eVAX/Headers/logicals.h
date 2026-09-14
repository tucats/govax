
//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, 
//                see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     logicals.h
//
//  Purpose:    Header file support for VMS logical names.
//
//  History:    12/05/01    Created.


#define LNM_M_NO_ALIAS     0x00000001
#define LNM_M_CONFINE      0x00000002
#define LNM_M_CRELOG       0x00000004
#define LNM_M_TABLE        0x00000008
#define LNM_M_CONCEALED    0x00000100
#define LNM_M_TERMINAL     0x00000200
#define LNM_M_EXISTS       0x00000400
#define LNM_M_SHAREABLE    0x00010000
#define LNM_M_CLUSTERWIDE  0x00020000
#define LNM_M_CREATE_IF    0x01000000
#define LNM_M_CASE_BLIND   0x02000000
#define LNM_M_INTERLOCKED  0x04000000
#define LNM_M_LOCAL_ACTION 0x08000000

#define LNM_C_TABNAMLEN       31
#define LNM_C_NAMLENGTH      255
#define LNM_C_MAXDEPTH        10
#define LNM__INDEX             1
#define LNM__STRING            2
#define LNM__ATTRIBUTES        3
#define LNM__TABLE             4
#define LNM__LENGTH            5
#define LNM__ACMODE            6
#define LNM__MAX_INDEX         7
#define LNM__PARENT            8
#define LNM__LNMB_ADDR         9
#define LNM__AGENT_ACMODE     10
#define LNM__CHAIN            -1

struct LNM {
    struct LNM *	next;
    char		name[ 64 ];
    struct LNM * 	tables;
    LONGWORD		attr;
    char		accmode;
    char *		value;
};


struct LNM * get_logical( char * tabnam, char * lognam, long attr );


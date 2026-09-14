//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_time.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//  History:    01/11/00    First written.  
//


#include "vax.h"
#include "shim.h"
#include "ss_def.h"
#include <time.h>



/*
 *      time_t time( time_t * the_time );
 */

LONGWORD decc_time( LONGWORD argc, LONGWORD argv[])
{

	time_t the_time;
    ULONGWORD time32;
    LONGWORD rc, time_addr;

    the_time = time( 0L );
    time32 = ( LONGWORD ) the_time;

    if( argc >= 1 ) {
        time_addr = argv[ 0 ];
        if( time_addr ) {
            rc = store_memory( time_addr, ( void * ) &time32, 4 );
        }
    }

    return time32;

}



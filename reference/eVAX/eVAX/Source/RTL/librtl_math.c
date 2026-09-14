//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_math.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//  History:    11/05/99    New header format standardization, built from librtl.c
//


#include "vax.h"
#include "shim.h"

#include "fpu.h"


/*----------------------------------------------------------------------*
 *                                                                      *
 *    fid = lib$adawi( long sum, long base, long sign );                *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD lib_adawi( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD sum_addr, base_addr, sign_addr;
    LONGWORD sum, base, sign, rc;
    
    sum_addr = argv[ 0 ];
    base_addr = argv[ 1 ];
    sign_addr = argv[ 2 ];
    
    rc = load_memory( sum_addr, ( void * ) &sum, 4 );
    if( rc )
        return rc;

    rc = load_memory( base_addr, ( void * ) &base, 4 );
    if( rc )
        return rc;

    base = base + sum;
    if( base < 0 )
        sign = -1;
    else
    if( base == 0 )
        sign = 0;
    else
        sign = 1;
    
    rc = store_memory( sign_addr, ( void * ) &sign, 4 );
    if( rc )
        return rc;

    vax.R0 = VAX_OK;
    return 1;

}


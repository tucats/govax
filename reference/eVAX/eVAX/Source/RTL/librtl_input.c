//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_input.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//              This supports input primitives. There is a LIB$GET_INPUT that is
//              located in the microkernel that uses this primitive to read a buffer.
//
//  History:    11/05/99    New header format standardization, built from librtl.c
//
//              11/24/99    Remove gets() and use fgets() instead.
//


#include "vax.h"

#if 0
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "memmap.h"         // to instantiate memory map list headers (uses GLOBAL)

#endif

#include "shim.h"

#include <stdio.h>
#include <errno.h>





/*----------------------------------------------------------------------*
 *                                                                      *
 *    str = decc$gets( char * buff );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_gets( LONGWORD argc, LONGWORD *argv )
{

	char buff[ 512 ];
	char * p;
	
	if( argc != 1 )
		return 0L;

	p = fgets( buff, 511, stdin );
	
	store_string( buff, argv[ 0 ], (int) strlen( p ) + 1);
	return argv[ 0 ];
}



/*----------------------------------------------------------------------*
 *                                                                      *
 *    n = decc$atoi( char * buff );                                     *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_atoi( LONGWORD argc, LONGWORD * argv )
{
	char * p;
	int rslt;
	
	if( argc != 1 )
		return 0L;

	p = load_dstring( argv[ 0 ] );
	rslt = atoi( p );
	freemem( p );
	
	return rslt;
	

}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    count = exe$input( char * buff, long len );                         *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD exe_input( LONGWORD argc, LONGWORD * argv )
{
    LONGWORD buff_addr;
    LONGWORD buff_len;
    LONGWORD i, n, rc;
    char * bp;
    
/*  First parameter is address of buffer; second is size of buffer */

    if( argc != 2 )
		return 0L;

    buff_addr = argv[ 0 ];
    buff_len =  argv[ 1 ];

/*  Allocate a buffer big enough to match it */

    bp = malloc( buff_len );
    if( bp == 0 )
        return 0;

/*  Read into the buffer */

    fgets( bp, (int)buff_len, stdin );

/*  Move the data to the caller's buffer */

    n = strlen( bp );
    if( bp[ n-1 ] == '\r' || bp[ n-1 ] == '\n')
        n--;
        
    for( i = 0; i < n; i++ ) {
        rc = store_memory( buff_addr + i, ( void * ) &( bp[ i ]), 1 );
        if( rc ) {
            free( bp );
            return 0;
        }
	}

    free( bp );
    
    return n;
}



//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_print.c
//
//  Purpose:    This module implements the PRINT command which just displays
//              the argument string to the console.
//
//  History:    02/17/98        Initial implementation
//
//              01/06/99        Allow expressions, etc. in print
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

/*
 *   Accept a list of quoted strings or numeric values and print them.
 */

LONGWORD console_print( char ** P )
{
    short n, j;
    char * p;
    LONGWORD rc, value, parsing;
    static char * pbuff = 0L;

    if(!( vax.console.flags & CONSOLE_VERBOSE )) {
        p = *P;
        while( !isend( *p ))
            p++;
        *P = p;
        return VAX_OK;
    }
    
    if( pbuff == 0L )
        pbuff = getmem( 256 );

    /* vax = *Vax;  -- now use global vax */
    parsing = 1;
    while( parsing ) {
        p = *P;
  
        flush_blanks( &p );

        if( isend( *p )) {
            parsing = 0;
            continue;
        }

        if( *p == ',' ) {
            *P = p + 1;
            continue;
        }

        if( *p == '"' ) {
             p++;
            for( n = 0; p[ n ] != 0 && p[ n ] != '"'; n++ ) {
                if( p[ n ] == '"' && n > 0 && p[ n-1 ] != '\\' )
                    break;
            }

            j = ( short ) ( ( n > 254 ) ? 254 : n );
            strncpy( pbuff, p, j );
            pbuff[ j ] = 0;
    
            printf( "%s", pbuff );
            *P = ( p + n + 1 );
        }
        else {
            rc = asm_expr( &p, &value );
            if( rc )
                return rc;

            if( vax.console.radix == 10 )
                printf( "%d", value );
            else
                printf( "%08X", value );
            *P = p;
        }
    }    
    
    printf( "\n" );

    return VAX_OK;
}

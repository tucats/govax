//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_dispatch.c
//
//  Purpose:    This routine accepts a single console command as an ASCIZ string,
//              decodes it, and runs the correct dispatcher.  It assumes that the
//              command has been correctly uppercased and contains only a single
//              command (no ';' allowed).
//
//  History:    07/28/97    New header format standardization
//
//              11/16/99    Added support for DCL-based parsing and dispatch.
//
//              05/20/00    If the DCL command has an associated /ENTRY name, then
//                          arrange to call the VAX code at that address.  Only do
//                          this if the microkernel is intact, though.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "dclrtl.h"

#include <ctype.h>


extern struct CONSOLE_DISPATCH_TABLE console_dispatch_table[];

char * expand_command( char * cmd );


//  Execute a console command

LONGWORD console_dispatch( char * cmd )
{
    LONGWORD rc;
    char *p, *pp;
    LONGWORD verb;
    LONGWORD n;
    console_handler routine;
    int expand;
    
    
    static char dispatcher_initialized = 0;

    if( !dispatcher_initialized ) {
        console_dispatch_init();
        dispatcher_initialized = 1;
    }
    
    if( vax.console.flags & CONSOLE_EXPAND ) {
        p = pp = expand_command( cmd );
        if( p == 0L )
            return VAX_MEM;
        expand = 1;
    }
    else {
        p = pp = cmd;
        expand = 0;
    }
    
    if( p == 0L )
        return VAX_OK;
        
    rc = VAX_OK;

    if( vax.console.assembler_mode ) {
        rc = assemble( &p );
        if( expand )
            freemem(pp);
        return rc;
    }
    
    //  Read verb (must be unambigious in first 4 bytes
    
    read_verb( &p, &verb );
    
    //  Find what command this is to dispatch
    
    routine = 0L;
    
    //printf("Testing verb %08lX\n", verb );

    for( n = 0; console_dispatch_table[ n ].handler; n++ ) {
   
        // printf("   compare to %08lX\n", console_dispatch_table[n].verb[0]);
 
        if( console_dispatch_table[ n ].verb[ 0 ] == verb ||
            console_dispatch_table[ n ].verb[ 1 ] == verb ||
            console_dispatch_table[ n ].verb[ 2 ] == verb ) {
                routine = console_dispatch_table[ n ].handler;
                if( ( long ) routine == -1L ) {
                    routine = 0L;
                    break;
                }
                    
                rc = (*routine)( &p );
                break;
            }
    }
    
    if( routine == 0L ) 
        rc = VAX_UNKCMD;

    if( rc == VAX_OK ) {
        flush_blanks( &p );
        if( !isend( *p )) 
            rc = VAX_EXTRACMD;
    }
    
    //  If we didn't already figure out (and handle) the command, then
    //  let's try the DCL parse gadget.  If it parses okay, then dispatch
    //  the command!
    
    if( routine == 0L ) {
    
        if( vax.debug & DBG_DCL )
            DCLsetdebug( 2, 0 );
            
        rc = DCLparse( "evax", pp, DCL_INPUT );
        
        if( !DCLERROR( rc )) {

            char * entrypt;

            entrypt = DCLgetentry();
            if( entrypt && entrypt[ 0 ] ) {
                if( !MKVALID )
                    rc = VAX_NOMK;
                else
                    rc = console_call( &entrypt );
            }
            else
                rc = DCLdispatch();
        }
        else
            rc = VAX_SYNTAX;
            
        DCLreset();

        DCLsetdebug( 0, 0 );

    
    }
    
    if( expand )
        freemem(pp); /* Release the parsed command buffer back to the caller */
    
    return rc;
}


//	Handle substitutions, etc. in command text.

char * expand_command( char * cmd )
{
    char * p;
    int	n, nn, i, v;
    char vname[ 100 ];
    LONGWORD rc, value;
    int did_sub;
    
    n = (int) strlen( cmd );
    nn = 0;

    /* Note maximum command line after expansion is 500 bytes long */
    
    p = getmem(500);
    if( p == 0L )
        return p;
        
    p[ 0 ] = 0;

    i = 0;
    did_sub = 0;
    while( i < n ) {
    
        if( ( i < n-1) && cmd[ i ] == '&' && cmd[ i+1 ] == '&' ) {
            did_sub = 1;
            p[nn] = 0;
            i = i + 2;
            v = 0;
            while(( isalnum(cmd [i] ) || cmd[ i ] == '$'  || cmd[ i ] == '_' ) ){
                vname[ v++ ] = toupper(cmd[ i++ ]);
            }
            vname[ v ] = 0;
            i--;
            rc = get_symbol_direct( vname, &value );
            if( rc ) {
                strcat( p, "?" );
                strcat( p, vname );
                strcat( p, "?" );
            }
            else {
                if( vax.console.last_symbol-> flags & SYM_STRING )
                        strcat( p, vax.console.last_symbol-> svalue );
                else {
                        sprintf( vname, ( vax.console.radix == 10 ) ? "%d" : "%09X", value );
                        strcat( p, vname );
                    }
                }
                nn = (int) strlen( p );
        }
        else {
            p[ nn ] = cmd[ i ];
            nn++;
        }
        i++;
    }
    p[ nn ] = 0;
    
    if(( vax.debug & DBG_EXPAND ) && did_sub ) {
        printf ("Command line expansion:\n   Old: %s\n   New: %s\n", cmd, p );
    }
    
    return p;
}

            

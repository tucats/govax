//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_include.c
//
//  Purpose:    This module implements the INCLUDE or @ command, which accesses
//              a file to be included, and pushes it on the top of the include
//              stack.
//
//              This module also implements the ASM command shell, which puts the
//              console into assembler mode, and optionally starts processing an
//              external file containing the assembler statements.
//
//  History:    02/15/98        Initial implementation
//
//

#include "vax.h"
#include "console_proto.h"
#include "include.h"
#include "asmproto.h"

LONGWORD console_include( char ** P )
{
    LONGWORD rc, scanning, verify;
    char * p, *cmdbuff;
    LONGWORD verb, asm_mode;
    extern struct INCLUDE * include;
    int cmdline = 0;
    
    /* vax = *Vax;  -- now use global vax */
    p = *P;

//  If there's no then there's a terrible error.

    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

    scanning = 1;
    verify = vax.console.verify;
    asm_mode = 0;

    while( scanning ) {

//  See if there is an explicit /[NO]VERIFY switch present.  If so, use
//  the appropriate value.  Otherwise, use the default console case.
//  Note that /LIST is a synonym for /VERIFY to support the ASM case
//  that uses the include processor.

        read_verb( &p, &verb );
        if( verb == CHAR4('/','V','E','R') || 
            verb == CHAR4('/','V',' ',' ') || 
            verb == CHAR4('/','L','I','S') ) {
            *P = p;
            verify = 1;
        }
        else
        if( verb == CHAR4('/','N','O','V') || verb == CHAR4('/','N','O','L') ) {
            *P = p;
            verify = 0;
        }
        else
        if( verb == CHAR4('/','C','O','M')) {
            *P = p;
            cmdline = 1;
        }
        if( verb == CHAR4('/','A','S','M')) {
            *P = p;
            asm_mode = 1;
        }
        else {
            scanning = 0;
            p = *P;
        }
    }

//  Was it the INCLUDE/COMMAND_LINE gimmick?  If so then fetch the command line and execute it.

    if( cmdline ) {

//  CONSOLE$ARG_CMD is a SYM_STRING symbol -- its native host string pointer
//  lives in svalue, not in the numeric (VAX-facing) value field.  Reading it
//  out through get_symbol_direct()'s LONGWORD* would truncate the pointer to
//  32 bits (see AUDIT.md N1).

         rc = get_symbol_direct( "CONSOLE$ARG_CMD", &verb );
         cmdbuff = ( rc == VAX_OK ) ? vax.console.last_symbol-> svalue : "";

         flush_blanks( &cmdbuff );
         flush_blanks( &p );
         if( !isend( *cmdbuff )) {
            if( !isend( *p ))
                console_dispatch( p );
            else
                console_dispatch( cmdbuff );
                
            console_dispatch( "EXIT" );
        }
        return VAX_OK;
    }
    
    
//  Push the file.

    rc = push_include( p, asm_mode ? ".asm" : ".com" );

//  Set the verify flag if the push worked okay.  Then we're done.

    if( rc == VAX_OK ) {
        p = p + strlen( p );
        include-> verify = verify;
    }
    
    *P = p;
    
    return rc;
}


//
//  The ASM command is sort of a special case of an INCLUDE operation.  If you
//  put ASM with no text after it, it puts the console into assembler mode.  If
//  you specify a file name, it will process that file as an assembler file.

LONGWORD console_asm( char ** P )
{

    char * p;
    LONGWORD rc;
    char buff[ 256 ], *bp;

    /* vax = *Vax;  -- now use global vax */
    if( !vax_init )
        return VAX_NOVAX;

//  See if there's a name on the line.  If not, then just turn on ASM mode and
//  we're done for now.

    p = *P;
    flush_blanks( &p );
    if( isend( *p )) {
        vax.console.assembler_mode = 1;
        return VAX_OK;
    }

//  Pass the command as if it was an INCLUDE statement, such that /VERIFY, etc.
//  are supported.

    strcpy( buff, " /ASM " );
    strcat( buff, *P );
    bp = &( buff[ 0 ]);

    rc = console_include( &bp );
    if( rc != VAX_OK )
        vax.console.assembler_mode = 0;
    else {
        while( !isend( **P ))
            (*P)++;
        vax.console.assembler_mode = 1;   
    }

    return rc;
}


//  IF <expression> THEN statement

LONGWORD console_if( char ** P )
{

    char * p = *P;
    LONGWORD long1, rc, verb;
    char * bp;
    char cmd[ 256 ];

   if( !vax_init )
        return VAX_NOVAX;

    rc = asm_expr( &p, &long1 );
    if( rc )
        return rc;

    flush_blanks( &p );
    bp = p;

    read_verb( &p, &verb );
    if( verb != CHAR4('T','H','E','N') )
        p = bp;

    if( long1  ) {

        strcpy( cmd, p );
        rc = console_dispatch( cmd );
    }
    else
        rc = VAX_OK;

    while( !isend( *p ))
        p++;

    *P = p;

    return rc;

}

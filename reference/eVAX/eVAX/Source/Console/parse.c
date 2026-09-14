//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     parse.c
//
//  Purpose:    This module implements miscellaneous parsing tools used by the
//              console processor to handle command input.
//
//  History:    08/06/97    New header format standardization
//
//


#include "vax.h"
#include "vaxrc.h"


//  Read the next token as a verb, and return four characters of verb.
//  This routine _will_ skip leading blanks to find the start of the
//  verb, so the caller doesn't have to.

LONGWORD read_verb( char ** P, LONGWORD * code )
{
    union VERB {
        char    ch[ 4 ];
        LONGWORD    code;
    } verb;
    LONGWORD n;
    
    char * p, ch;
    
    p = *P;
    flush_blanks( &p );
    
    if( isend( *p ))
        return VAX_INCOMPLETE;
        
    verb.code = CHAR4( ' ', ' ', ' ', ' ' );
    n = 0;
    
    //  If it's an '@' then that's a token by itself.
    
    if( *p == '@' ) {
        verb.ch[ 0 ] = '@';
        p++;
    }
    else 
    
    //  Otherwise pick up the next four characters and skip to a blank or = character
    
    for( n = 0; *p != 0; n++ ) {
        if( isend( *p ) || is_blank( *p ))
            break;
        if( *p == '=')
            break;
        if( *p == '/' && n > 0 )
            break;
        if( *p == ',' )
            break;
            
        if( n < 4 ) {
            ch = *p;

    //  The Intel standard appears to be that multibyte character
    //  constants are stored in reversed order from the naturally
    //  occuring longword. So we conditionally have to reverse
    //  the storage order if we're compiling for Intel.

        verb.ch[ n ] = ch;

        }
        p++;
    }
    *P = p;
    *code = verb.code;
    
    return VAX_OK;
}



int is_blank( char ch )
{

    if( ch == ' ' || ch == '\t' )
        return 1;
    else
        return 0;
}



short isend( char ch )
{

    char comment;
    
    if( !vax_init )
        comment = CONSOLE_COMMENT;
    else
        comment = vax.console.comment;
        
    if( ch == '\n' || /* ch == comment || */ ch == 0 )
        return 1;
    else
        return 0;
}



//  Uppercase a buffer in place, except for double-quoted strings

LONGWORD uppercase( char * p )
{

    LONGWORD n, q, sq;
    char ch;
    
    q = sq = 0;

    for( n = 0; p[ n ]; n++ ) {

        ch = p[ n ];
        
        if( n > 0 && q && p[n-1] == '\\' && ch == '"' )
            continue;

        if( ch == '\'' )
            sq = 1 - sq;
            
        if( ch == '"' && !sq )
            q = 1 - q;
        
        if( !sq && !q && ch == ';' ) {
            p[ n ] = 0;
            break;
        }
        
        if( !sq && !q && ch >= 'a' && ch <= 'z' )
            p[ n ] = ch - 32;
    }
    
    return VAX_OK;
}


//  Flush leading blanks

LONGWORD flush_blanks( char ** P )
{

    char * p;
    
    for( p = *P; is_blank(*p); p++ )
        continue;
        
    *P = p;
    return VAX_OK;
}

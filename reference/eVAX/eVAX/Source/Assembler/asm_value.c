//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_value.c
//
//  Purpose:    Parses numeric or symbolic values on behalf of the pseudo-
//              assembler AND the console in the virtual VAX.
//
//
//  History:    02/01/98    Created by removing from asm.c
//
//                          Added support for symbolic forward references where
//                          appropriate.  Also cleaned up return codes from pseudo
//                          operations to distinguish between
//                          "not a pseudo op" and "a bad pseudo op".
//
//              09/15/98    Added asm_float() which can parse a floating constant
//                          and store it in vax.last_float, for use by routines
//                          that parse constants, such as asm_hex().  You can use
//                          ^F as a data type to mean a floating value in a
//                          constant.  However, the caller of asm_value or asm_hex
//                          must be expecting the VAX_ASMFLOAT return code to indicate
//                          a floating value was found.
//
//              12/08/98    Conditional compilation for the case
//                          where we are on VMS... the floating
//                          conversions must be done differently.
//
//              10/07/99    Added handling for '<c>' literals.
//
//              10/14/99    Removed special VMS case, let the C runtime do it for us
//                          just like UNIX does.


#include "vax.h"
#include "vaxinstr.h"

//  Local prototypes

#include "asmproto.h"

LONGWORD char_literal( char ** P, LONGWORD *Value );

#ifdef VMS
#include <descrip.h>
#include <ots$routines.h>
#else
#ifdef macintosh
#include <fp.h>         /* MacOS include, required to access the numeric text */
                        /* scanner function.                                  */
#endif
#endif

//  Parse out a floating point number.  Result is in vax.last_float.

LONGWORD asm_float( char ** P )
{

    char *  p, c;
    short len, found, n;


#ifdef macintosh
    double_t x;
    decimal dt;
    short valid;
    short p1, p2;
#endif

    char buff[ 32 ];
    
    static char float_characters[] = "0123456789.-+Ee";
    
/*  Find the end of the floating point number */

    p = *P;

    found = 1; 
    len = 0;
    
    while( found ) {
    
        c = *p;
        if( c == 0 ) {  /* EOS is special, stop but point past for later adjustment */
            p ++;
            break;
        }
        
        found = 0;
        for( n = 0; n < sizeof( float_characters ); n++ ) {
            if( c == float_characters[ n ] ) {
                found = 1;
                len++;
                p++;
                break;
            }
        }

    }
    
    strncpy( buff, *P, len );
    buff[ len ] = 0;
    
    vax.last_float = 0.0;

#ifdef macintosh
/* MacOS version */

    /*  Try to scan in the number */
    
    valid = 0;
    p1 = 0;
    p2 = 0;
    str2dec( buff, &p2, &dt, &valid );

/*
 *  If the value was valid, move it back to the result.  We detect this
 *  by seeing if the scanner moved the pointer or not.
 */
 
    if( p2 > p1 ) {
        x = dec2num( &dt );
        vax.last_float = x;
    }
    else {
        vax.last_float = 0.0;
        return VAX_ASMINVFLOAT;
    }

#else

/*  UNIX, VMS, Windows version */

    vax.last_float = atof( buff );
    
#endif

    /*  Store back the pointer and we're done */
    
    *P = p;
    return VAX_OK;
}


//  Parse out a value.  This can be an unresolved forward reference if we want

LONGWORD asm_value( char ** P, ULONGWORD * V, LONGWORD mode )
{

    LONGWORD rc;
    
    
    vax.assembler.location = vax.console.deposit;
    vax.assembler.displacement = mode;
    vax.assembler.was_forward = 0;    
    rc = asm_expr( P, ( LONGWORD *  ) V );
    
    vax.assembler.location = 0L;
    vax.assembler.displacement = 0L;

        if( rc == VAX_OK ) {
            if( vax.assembler.flags & ASM_EXPRESSION &&
                vax.assembler.was_forward )
                rc = VAX_ASMINVFEX;
        }

    return rc;
}


//
//  Parse a hexadecimal value
//  

LONGWORD asm_hex( char ** P, ULONGWORD *Value )
{

    char * p = *P;
    ULONGWORD value;
    unsigned char ch;
    LONGWORD rc, saved_radix;
    int running;
    int bytes;

/*      vax.assembler.was_forward = 0; */

    if( vax.console.radix == 10 )
        return asm_dec( P, ( LONGWORD * ) Value );


    value = 0L;
    flush_blanks( &p );
    *Value = 0L;
    running = 1;
    bytes = 1;
    
    /* If there isn't a value here, say so! */
    
    if( isend( *p ))
        return VAX_INCOMPLETE;
        
    if( p[ 0 ] == '^' && p[ 1 ] == 'D' ) {
        p = p + 2;
        saved_radix = vax.console.radix;
        vax.console.radix = 10;
        rc = asm_dec( &p, ( LONGWORD * ) Value );
        vax.console.radix = saved_radix;
        *P = p;
        return rc;
    }

    if( p[ 0 ] == '^' && p[ 1 ] == 'M' ) {
        p = p + 2;
        rc = asm_mask( &p, ( LONGWORD * ) Value );
        *P = p;
        return rc;
    }
    
    /* If it's the radix prefix, then skip it, else check for floating or symbol */
    
    if(( p[ 0 ] == '^' && p[ 1 ] == 'X' ) ||
       ( p[ 0 ] == '0' && p[ 1 ] == 'X' )) {
        p = p + 2;
    }
    else {
        if( p[ 0 ] == '^' && p[ 1 ] == 'F' ) {
            p = p + 2;
            rc = asm_float( &p );
            if( rc == VAX_OK )
                rc = VAX_ASMFLOAT;  /* Signal unexpected float value found */
            *P = p;
            return rc;
        }
        
        
        if( ( *p >= 'A' && *p <= 'Z' ) || *p == '_' || *p == '$' || *p == '\'' ) {
            rc = asm_symbol( &p, ( LONGWORD * ) Value );
            *P = p;
            return rc;
        }
    }
        
    bytes = 0;
    while( running ) {
    
        ch = *p;
        
        if(( ch >= '0' && ch <= '9' ) || ( ch >= 'A' && ch <= 'F' )) {
        
            if( ch >= 'A' )
                ch = ( char ) (( ch - 'A' ) + '9' + 1 );
            
            value = ( value << 4 ) + ( ch - '0' );
            p++;
            bytes++;
            continue;
        }
        break;
    }
    
    *Value = value;
    *P = p;
    if( bytes )
        return VAX_OK;
    else
        return VAX_ASMINVCONST;
        
}


//  Assemble a decimal value from the input buffer

LONGWORD asm_dec( char ** P, LONGWORD *Value )
{

    char * p = *P;
    ULONGWORD value;
    unsigned char ch;
    LONGWORD rc;
    LONGWORD saved_radix;
    int running, bytes, sign;

    running = 1;
    bytes = 1;
    sign = 0;
    value = 0L;
    flush_blanks( &p );

/*  vax.assembler.was_forward = 0; */

    if(( p[ 0 ] == '^' && p[ 1 ] == 'X' ) ||
       ( p[ 0 ] == '0' && p[ 1 ] == 'X' )) {
        saved_radix = vax.console.radix; 
        vax.console.radix = 16;
        rc = asm_hex( &p, ( ULONGWORD * ) Value );
        vax.console.radix = saved_radix;
        *P = p;
        return rc;
    }

    if( p[ 0 ] == '^' && p[ 1 ] == 'D' ) {
        p = p + 2;
    }

    if( p[ 0 ] == '^' && p[ 1 ] == 'F' ) {
        p = p + 2;
        rc = asm_float( &p );
        if( rc == VAX_OK )
            rc = VAX_ASMFLOAT;  /* Signal unexpected float value found */
        *P = p;
        return rc;
    }

    if( ( *p >= 'A' && *p <= 'Z' ) || *p == '_' || *p == '$' || *p == '\'' ) {
        rc = asm_symbol( &p, ( LONGWORD * ) Value );
        *P = p;
        return rc;
    }

    bytes = 0;
    sign = 0;
    while( running ) {
    
        ch = *p;
        if( ch == '+' ) {
            if( sign )
                break;
            sign = 1;
            p++;
            continue;
        }
        
        if( ch == '-' ) {
            if( sign )
                break;
                
            sign = -1;
            p++;
            continue;
        }
        
        if( ch >= '0' && ch <= '9' )  {
            if( !sign )
                sign = 1;
            value = value *10 + ( ch - '0');
            p++;
            bytes++;
            continue;
        }
        break;
    }
    
    value = value * sign;
    *Value = value;
    *P = p;

    if( bytes )
        return VAX_OK;
    else
        return VAX_ASMINVCONST;
        
}


//  Parse a symbol from the input buffer and use it's value

LONGWORD asm_symbol( char ** P, LONGWORD *Value )
{

    char * p = *P;
    LONGWORD rc;
    
    //  You wouldn't think this would be here, but it is.  In an early version of eVAX,
    //  symbols were delimited by single quotes to represent a substitution.  Long since,
    //  symbol handling has been more thoroughly integrated into parsing, expressions,
    //  etc.  But many routines come here when they see a leading single quote.  So here
    //  is where I do the silly business of handling quoted character literals.
    
    
    if( *p == '\'' ) {
        return char_literal( P, Value );
    }

    //  Nope, not a character literal, so parse as a symbol.
    
    rc = get_symbol( &p, Value );
            
    *P = p;
    
    return rc;
}


//  Parse out a <r> register mask

LONGWORD asm_mask( char ** P, LONGWORD * Value )
{

    char r[ 12 ];
    static char * rname[] = { 
    "R0",   "R1",   "R2",   "R3", 
    "R4",   "R5",   "R6",   "R7", 
    "R8",   "R9",   "R10",  "R11",
    "IV",   "DV",   "" };
    
    static char rbit[] = {
    0,  1,  2,  3,
    4,  5,  6,  7,
    8,  9,  10, 11,
    15, 14
    };
    
    LONGWORD mask, n;
    char * p, *rn;
    int running;
	rn = 0L;
	running = 1;
    p = *P;
    mask = 0L;
    *Value = 0L;
    
    flush_blanks( &p );
    if( p[ 0 ] == '^' && p[ 1 ] == 'M' ) {
        p = p + 2;
        flush_blanks( &p );
    }
    
    if( *p != '<' )
        return VAX_ASMINVMASK;

    p++;
    
//  Loop a LONGWORD time over the string...

    while( running ) {
    
        flush_blanks( &p );
        if( *p == ',' ) {
            p++;
            continue;
        }
        
        if( *p == '>' || isend( *p ))
            break;
            
    //  Find the next item in the list.
    
        n = 0;
        while( *p != ',' && !isend( *p ) && *p != '>' ) {
            if( *p != ' ' ) {
                r[ n ] = *p;
                n++;
            }
            p++;
            if( n == 12 )
                return VAX_ASMINVMASK;
        }
        r[ n ] = 0;
    
        for( n = 0; n < 30; n++ ) {
            rn = rname[ n ];
            if( *rn == 0 )
                break;
            if( strcmp( rn, r ) == 0 )
                break;
        }
        
        if( *rn == 0 ) 
            return VAX_ASMINVMASK;
        
        mask |= ( 1 << rbit[ n ] );
    }
    
    if( *p != '>' )
        return VAX_ASMINVMASK;
    p++;
    *P = p;
    *Value = mask;
    return VAX_OK;
}


LONGWORD char_literal( char ** P, LONGWORD *Value )
{
    char * p = *P;
    LONGWORD value;
    char ch;
    int size;
    
    if( *p != '\'' )
        return VAX_ASMINVCHARLIT;
    
    value = 0L;
    
    p++;
    size = 0;
    while( *p != '\'' ) {
        if( size > 3 )
            return VAX_ASMINVCHARLIT;
        
        ch = *p;
        if( ch == '\\' ) {
            p++;
            ch = *p;

            switch( ch ) {
            
            case 'n':   ch = '\n';  break;
            case 'r':   ch = '\r';  break;
            case 't':   ch = '\t';  break;
            }
        }
                    
        value = ( ch << ( 8*size)) + value;
        p++;
        size++;
    }
    
    if( *p == '\'' )
        p++;
    
    *P = p;
    *Value = value;
    
    return VAX_OK;
}

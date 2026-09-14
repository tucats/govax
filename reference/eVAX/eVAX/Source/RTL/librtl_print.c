//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_print.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//              This module implements the printf() family of runtime routines.
//
//  History:    11/05/99    New header format standardization, captured from librtl.c
//
//              09/11/01    Fix bug in parsing w.d specs in C format strings.


#include "vax.h"
#include "shim.h"

#include "fpu.h"

static int decc_apply_format( char * buff, char * fmt, LONGWORD * argv, LONGWORD * pos );

//
//  Generic routine that applies a format.  Given a format string (with only the
//  format data in it) and a pointer to the argument list, this advances the arg
//  pointer appropriately and formats the data into the caller's buffer.  The
//  routine generally uses the host's sprintf() routine to do the formatting of
//  data read from the VAX memory once the format type is known.
//

int decc_apply_format( char * buff, char * fmt, LONGWORD * argv, LONGWORD * pos )
{

    int w, d, dot, size, code, align, z;
    int len, n, v;
    LONGWORD data;
    char ch, * str;
    double dbl;
    
    /* Initialize the scan.  We're going to parse the basic format expression */
    /* In today's version, this data is only used to display a diagnostic when */
    /* the format is incomprehensible.  In the future, we might do more VAX-ish */
    /* formatting rather than cheating with sprintf().... */
    
    w = d = v = dot = 0;
    z = -1;
    size = 2;
    v = 0;
    align = 0;
    len = (int) strlen( fmt );
    code = 0;
    
    /* Scan over the string.  The only state data here is what we do with digits */
    /* that we find - could be leading zero, w, or d specifiers.				*/
    
    for( n = 0; n < len; n++ ) {
    
        ch = fmt[ n ];
        if( ch == '%' )					/* Skip over the leading % */
            continue;
            
        if( ch == '0' && z == -1 ) {			/* If it's leading zero... */
            z = 1;
            continue;
        }
        
        if( ch >= '0' && ch <= '9' && z == -1 )		/* If it's not leading zero */
            z = 0;
            
        if( ch >= '0' && ch <= '9' ) {			/* Record the digit */
            v = v * 10 + ( ch - '0' );
            continue;
        }
        
        if( ch == '.' ) {				/* w.d separator */
            w = v;
            dot = 1;
            v = 0;
            continue;
        }
        
        if( ch == 'l' ) {				/* longword flag */
            size = 4;
            continue;
        }
        
        if( ch == '-' ) {				/* alignment modifier */
            align = 1;
            continue;
        }
        
        if( ch == 'c' )					/* Characters are one byte */
            size = 1;
        
        if( instring( ch, "%dxXfsc" )) {		/* terminating format code */
            code = ch;
            break;
        }
    }
    
    /* Put the number we gathered up into the right field of w or d */
    
    if(  dot )
        d = v;
    else
        w = v;
    
    /* If we never got a leading zero, just set the flag to off */
    
    if( z == -1 )
        z = 0;
    
/*
 *  Based on the data type, fetch the value from memory.  
 */
 
/*
 *	If it's a '%' then this was the odd escape for a percent sign.
 */
 
 	if( code == '%' ) {
 		buff[ 0 ] = '%';
 		buff[ 1 ] = 1;
 		return 1;
 	}
 	
/*  If it's a scalar integer item, the value in the argv[] array is what we use.
 */

    if( code == 'd' || code == 'X' || code == 'x' || code == 'c' ) {
        data = argv[ *pos ];
        *pos = *pos + 1;
        
        return sprintf( buff, fmt, data );
    }

/*
 *  If it's a floating value, more work.  Doubles take two
 *  argument slots.
 */
 
    if( code == 'f' ) {

        fpu_load( argv[ *pos ], argv[ (*pos) + 1 ], &dbl );
        *pos = *pos + 2;
        return sprintf( buff, fmt, dbl );
    }

/*
 *  If it's a string, load the string from the address.
 */
 
    if( code == 's' ) {
    
        str = load_dstring( argv[ *pos ] );
        if( str == 0L ) {
            buff[ 0 ] = 0;
            n = 0;
        }
        else {
            n = sprintf( buff, fmt, str );
            freemem( str );
        }
        
        *pos = *pos + 1;
        return n;
    }

/*
 *	Jeepers, it wasn't anything we understand.  Try to dump out helpful info
 *	about what we found in the format string.  This may or may not be useful...
 */
 
    printf( "%%CRTL-E-FORMAT, unrecognized format \"%s\",", fmt );
    printf( " w=%d d=%d align=%d size=%d z=%d code=\"%c\"\n",
                w, d, align, size, z, code );
                
    printf( "-CRTL-E-FMTARG, argument %08X  %d (dec)\n", 
            argv[ *pos ], argv[ *pos ] );
    return 0;
}





/*----------------------------------------------------------------------*
 *                                                                      *
 *    len = printf( char * fmtstr,... );                                *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_printf( LONGWORD argc, LONGWORD * argv )
{
    char buff[ 1024 ];
    long rc;
    
    rc = decc_format( buff, argc, argv, 0 );
    
    return printf( "%s", buff );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    len = sprintf( char* buff, char * fmtstr,... );                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_sprintf( LONGWORD argc, LONGWORD * argv )
{
    char buff[ 1024 ];
    LONGWORD bp;
    long rc;
    int len, n;
    
    bp = argv[ 0 ];
    rc = decc_format( buff, argc, argv, 1 );
    len = (int)strlen( buff );
    for( n = 0; n <= len; n++ ) {
        rc = store_memory( bp + n, ( void * ) &(buff[ n ]), 1 );
        if( rc )
            break;
    }
    

    return rc;
}




/*
 *	This is the format processor, that processes a format string and an argument list.
 *	It can be called from a variety of places, depending on how the argument list is
 *	to be used (i.e. printf() versus sprintf(), etc.).
 */

LONGWORD decc_format( char * buff, LONGWORD argc, LONGWORD * argv, LONGWORD Pos )
{
    
    LONGWORD fmtaddr, fmtp, pos, rc;
    int parsing = 1;
    int count = 0;
    char ch;
    char dbuff[ 128 ];
    char fmt[ 32 ];
    
    pos = Pos;
    fmtaddr = argv[ pos++];
    rc = VAX_OK;
    
    /* Scan the string.  We basically copy bytes from the format to the output */
    /* buffer.  We also handle special notations like \n in the format string  */
    
    while( parsing ) {
    
        rc = load_byte( fmtaddr++, ( void * ) &ch );
        if( rc )
            break;
        
        switch( ch ) {
        
        case 0:
            parsing = 0;
            break;
        
        /*  Escape character processing.  This may be a bad idea, since a */
        /*  C compiler would have already processed this data.  We'll see... */
        
        case '\\':
            rc = load_byte( fmtaddr++, ( void * ) &ch );
            if( rc )
                break;
            switch( ch ) {

            case 'n':
                buff[ count++ ] = '\n';
                break;
            
            case 'r':
                buff[ count++ ] = '\r';
                break;
            case 't':
                buff[ count++ ] = '\t';
                break;
            case '0':
                buff[ count++ ] = 0;
                break;
            
            default:
                buff[ count++ ] = ch;
                break;              
            }
            
            break;
        
        /*  Uh-oh, it's a format operator.  Capture the format data */
        /*  and then call the format processor to have it eat one or */
        /*  more longwords from the argument list to create a formated */
        /*  data item which we put in the buffer */
        
        case '%':
            
            fmtp = 1;
            fmt[ 0 ] = '%';
            while( 1 ) {
            
                rc = load_byte( fmtaddr++, (void*)&ch );
                if( rc )
                    return rc;
                    
                fmt[ fmtp++ ] = ch;
                
                if( ch == 0 )
                    break;
                if( instring( ch, "dxXfsc" )) {
                    fmt[ fmtp ] = 0;
                    break;
                }
            }
            
            decc_apply_format( dbuff, fmt, argv, &pos );
            buff[ count ] = 0;
            strcat( buff, dbuff );
            count = (int) strlen( buff );
            
            break;
        
        /* Not a format and not an escape, so just copy the byte to the buffer*/
        
        default:
            buff[ count++ ] = ch;
            break;
        }
    }
    
    buff[ count ] = 0;
    return rc;
}

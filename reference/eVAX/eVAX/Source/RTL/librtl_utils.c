//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_utils.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//              This contains miscellaneous utility functions for the runtimes.
//
//  History:    11/05/99    New header format standardization, built from librtl.c
//


#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "memmap.h"         // to instantiate memory map list headers (uses GLOBAL)
#include "shim.h"

#include <errno.h>
LONGWORD load_buffer( LONGWORD addr, char * p, LONGWORD len );

//
//	Store a string in memory.  The string is in a buffer passed by the caller, and
//	we write it to a VAX address.
//

int store_string( char * str, ULONGWORD addr, int len ) 
{
	int count;
	long rc;
	char ch;
	
	if( len == 0 )
		len = 65535;
		
	for( count = 0; count < len; count++ ) {
	
		ch = str[ count ];
		rc = store_memory( addr + count, ( void * ) &ch, 1 );
		if( rc )
			return (int) rc;
		if( ch == 0)
			break;
			
	}
	
	return VAX_OK;
}

//
//  Read a null-terminated string from VAX memory.  This is stored in a dynamically
//  created char * variable that *MUST* be explicitly free'd by the caller when done.
//

char * load_dstring( LONGWORD addr )
{
    char ch, *p;
    long n, len, rc;
    
//  Well, in the interest of fairness, let's see how long this string really is.

    for( n = 0; n < 65535; n++ ) {
        rc = load_byte( addr+n, ( void * ) &ch );
        if( rc )
            return 0L;
        if( ch == 0 )
            break;
    }

//  Allocate some memory.  If there isn't any, return zippo

    len = n + 1;
    p = getmem( len );
    if( p == 0L )
        return 0L;

//  Copy the string to the buffer and return it to the caller.

    for( n = 0; n < len; n++ ) {
        rc = load_byte( addr+n, ( void * ) &ch );
        p[ n ] = ch;
    }
    
    return p;
    
}

//
//  Read a null-terminated string from VAX memory.  This is stored in a buffer
//  passed in by the caller, with a pointer/length pair.  The buffer will never
//  be overflowed.  The return value is the number of bytes actually read.

LONGWORD load_string( LONGWORD addr, char * p, LONGWORD len )
{
    char ch;
    long n, rc;
    
//  Copy the string to the buffer.

    for( n = 0; n < len; n++ ) {
        rc = load_byte( addr+n, ( void * ) &ch );
        if( rc )
            return -1;
        p[ n ] = ch;
        if( ch == 0 )
            break;
    }
    
    return n;
    
}



//
//  Read an arbitrary buffer from VAX memory.  This is stored in a buffer
//  passed in by the caller, with a pointer/length pair.  The buffer will never
//  be overflowed.  The return value is the number of bytes actually read.

LONGWORD load_buffer( LONGWORD addr, char * p, LONGWORD len )
{
    char ch;
    long n, rc;
    
//  Copy the data to the native buffer.

    for( n = 0; n < len; n++ ) {
        rc = load_byte( addr+n, ( void * ) &ch );
        if( rc )
            return -1;
        p[ n ] = ch;
     }
    
    return n;
    
}


//
//  Determine if a character is in a null-terminated string
//

int instring( char ch, char * str ) 
{
    int len, n;
    
    len = (int) strlen( str );
    for( n = 0; n < len; n++ ) {
        if( ch == str[ n ] )
            return 1;
    }
    return 0;
    
}



//
//  Set the value of errno.  This is shared among C runtime library routines
//  to contain the result.  The shim routines use this to set the real errno
//  value into a reserved memory area on the VAX that can be read by programs.

void set_errno( int Code )
{
    long rc;
    char sname[ 32 ], *bp;
    LONGWORD code;
    static LONGWORD errno_addr = 0L;

    code = Code;
    if( errno_addr == 0L ) {

        strcpy( sname, "VAXC$ERRNO" );
        bp = sname;
        
        rc = get_symbol( &bp, &errno_addr );
        if( rc )
            return;
        if( errno_addr == 0L )
            return;
    }
    
    rc = store_memory( errno_addr, ( unsigned char * ) &code, 4 );
    return;
    
}

/*----------------------------------------------------------------------*
 *                                                                      *
 *    len = str_get( struct dsc$descriptor * str, int * retlen,         *
 *                        char * buff );                                *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD str_get( LONGWORD addr, int * retlen, char * buff )
{

    LONGWORD rc, daddr;
    short len, n;
    char ch;
    
//  Get the length of the string

    rc = load_memory( addr, ( void * ) &len, 2 );
    if( rc )
        return rc;
    
    if( len > *retlen ) {
        *retlen = -1;
        return VAX_OK;
    }
    *retlen = len;
    
//  Get the address of the string

    rc = load_memory( addr + 4, ( void * ) &daddr, 4 );

//  Loop over the string, copying bytes to caller's buffer

    for( n = 0; n < len; n++ ) {
    
        rc = load_byte( daddr + n, ( void * ) &ch );
        if( rc )
            return rc;
        buff[ n ] = ch;
    }
    return VAX_OK;
}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    len = str_put( struct dsc$descriptor * str, int len,              *
 *                        char * buff );                                *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD str_put( LONGWORD addr, int len, char * buff )
{

    LONGWORD rc, daddr;
    short dlen, n, attr;
    char ch;
    
//  Get the length of the string

    rc = load_memory( addr, ( void * ) &dlen, 2 );
    if( rc )
        return rc;
//  Get the descriptor type info

    rc = load_memory( addr + 2, ( void * ) &attr, 2 );
    if( rc )
        return rc;
    
//  Get the address of the string

    rc = load_memory( addr + 4, ( void * ) &daddr, 4 );
    
//  Loop over the string, copying bytes from caller's buffer.  If
//  The caller's buffer is too long, pad with blanks.

    for( n = 0; n < dlen; n++ ) {
    
        if( n > len )
            ch = ' ';
        else
            ch = buff[ n ];
        rc = store_memory( daddr + n, ( void * ) &ch, 1 );
        if( rc )
            return rc;
    }
    return VAX_OK;
}


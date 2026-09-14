//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_strings.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//              Supports string functions.  Many string functions are written into
//              the microkernel, but some are here as well.
//
//
//  History:    11/05/99    New header format standardization.  Built from librtl.c
//


#include "vax.h"
#include "shim.h"
#include <ctype.h>


/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isalnum( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isalnum( LONGWORD argc, LONGWORD * argv )
{

    return isalnum( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isalpha( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isalpha( LONGWORD argc, LONGWORD * argv )
{

    return isalpha( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isdigit( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isdigit( LONGWORD argc, LONGWORD * argv )
{

    return isdigit( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$islower( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_islower( LONGWORD argc, LONGWORD * argv )
{

    return islower( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isspace( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isspace( LONGWORD argc, LONGWORD * argv )
{

    return isspace( argv[ 0 ] & 0x0FF );

}



/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isupper( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isupper( LONGWORD argc, LONGWORD * argv )
{

    return isupper( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$iscntrl( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_iscntrl( LONGWORD argc, LONGWORD * argv )
{

    return iscntrl( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isgraph( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isgraph( LONGWORD argc, LONGWORD * argv )
{

    return isgraph( argv[ 0 ] & 0x0FF );

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isprint( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isprint( LONGWORD argc, LONGWORD * argv )
{

    return isprint( argv[ 0 ] & 0x0FF );

}



/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$ispunct( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_ispunct( LONGWORD argc, LONGWORD * argv )
{

    return ispunct( argv[ 0 ] & 0x0FF );

}





/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isxdigit( char ch );                                  *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isxdigit( LONGWORD argc, LONGWORD * argv )
{

    return isxdigit( argv[ 0 ] & 0x0FF );

}





/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$isascii( char ch );                                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_isascii( LONGWORD argc, LONGWORD * argv )
{

    return 1;

}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$strncpy( char * str1, char * str2, int len );         *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_strncpy( LONGWORD argc, LONGWORD * argv )
{

	char * str;
	int alen, len;

	/* Get the source string into host memory */
	
	str = load_dstring( argv[ 1 ] );
	
	/* And the max length of the string as well */
	
	len = (int) argv[ 2 ];
	
	/* Figure out how many bytes to read (lesser of actual len or parameter len ) */
	
	alen = (int) strlen( str );
	if( alen < len )
		len = alen + 1;

	/* Store correct number of bytes in destination area */
	
	store_string( str, argv[ 0 ], len );

	freemem( str );
	
	return len;
}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$strcmp( char * str1, char * str2 );                   *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_strcmp( LONGWORD argc, LONGWORD * argv )
{

	char * str1, * str2;
	int rslt;
	
	str1 = load_dstring( argv[ 0 ] );
	str2 = load_dstring( argv[ 1 ] );
	
	rslt = strcmp( str1, str2 );
	
	freemem( str1 );
	freemem( str2 );
	
	return rslt;
}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    rslt = decc$strncmp( char * str1, char * str2, int len );         *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_strncmp( LONGWORD argc, LONGWORD * argv )
{

	char * str1, * str2;
	long len;
	int rslt;
	
	str1 = load_dstring( argv[ 0 ] );
	str2 = load_dstring( argv[ 1 ] );
	len = argv[ 2 ];
	
	rslt = strncmp( str1, str2, len );
	
	freemem( str1 );
	freemem( str2 );
	
	return rslt;
}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    sts = str$upcase( struct dsc$descriptor * str );                  *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD str_upcase( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD addr, rc, daddr;
    short len, n;
    char ch;
    
    addr = argv[ 0 ];

//  Get the length of the string

    rc = load_memory( addr, ( void * ) &len, 2 );
    if( rc )
        return rc;
    
//  Get the address of the string

    rc = load_memory( addr + 4, ( void * ) &daddr, 4 );

//  Loop over the string, upcasing each byte

    for( n = 0; n < len; n++ ) {
    
        rc = load_byte( daddr + n, ( void * ) &ch );
        if( rc )
            return rc;
            
        if( ch >= 'a' && ch <= 'z' ) {
            ch = ch - 32;
            rc = store_memory( daddr + n, ( void * ) &ch, 1 );
            if( rc )
                return rc;
        }
    }
    return 1;
}

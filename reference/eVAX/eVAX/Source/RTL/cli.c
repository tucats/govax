//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     cli.c
//
//  Purpose:    Runtime support for VMS SYS$CLI callback service
//
//  History:    12/07/01   Created.


#include "vax.h"
#include "shim.h"
#include "services.h"
#include "ss_def.h"
#include "dclrtl.h"


//	As best I can tell, all SYS$CLI callbacks are for a single request, with
//	a static format for the callback.  What we know so far is:
//
//	+-------+-------+-------+-------+
//	|   zeroes?     | flags?| req   |   int_b_req 	request ID
//	+-------+-------+-------+-------+
//      |          in_length            |   int_l_len	length of incoming value
//	+-------+-------+-------+-------+
//      |           in_addr             |   int_l_ptr	addrss of incoming value
//	+-------+-------+-------+-------+
//      |                               |   unknown, currently always zero?
//	+-------+-------+-------+-------+
//      |                               |   unknown, currently always zero?
//	+-------+-------+-------+-------+


SSDEF( sys_cli )
{
    LONGWORD	rc;
    LONGWORD	req_addr, len, ptr;
    char b[ 256 ];
    char	request, subrequest;
    int		reqword;
    
    
    req_addr = argv[ 0 ];
    
    rc = load_memory( req_addr, 	( void * ) &request,	1 );
    if( rc ) return SS_ACCVIO;
    
    rc = load_memory( req_addr+1,	( void * ) &subrequest,	1 );
    if( rc ) return SS_ACCVIO;

    rc = load_memory( req_addr+4,	( void * ) &len,	4 );
    if( rc ) return SS_ACCVIO;
    
    rc = load_memory( req_addr+8,	( void * ) &ptr,	4 );
    if( rc ) return SS_ACCVIO;
    
    reqword = ( subrequest << 8 ) + request;
    rc = SS_NORMAL;
    
    switch( reqword ) {
    
    case 0x1305:   /* get symbol */
            
        load_string( ptr, b, len );
        b[ len ] = 0;
        
        printf( "CLI: Request to get symbol %s\n", b );
        
        /* For now, all are undefined until we learn how to get one back to caller */
        
        rc = 0x38140;  /* CLI$_UNDSYM */
        
        break;
        
    case 0x0105:	/* pause the image */
    case 0x0205:	/* define symbol in local table */
    case 0x0305:	/* define symbol in global table */
    case 0x0405:	/* chain to new image */
    case 0x0505:	/* pass command line to later executive */
    case 0x0605:	/* create process logical name */
    case 0x0705:	/* delete process logical name */
    case 0x0805:	/* disable DCL control-y */
    case 0x0905:	/* enable  DCL control-y */
    case 0x0A05:	/* return value of a symbol */
    case 0x0B05:	/* delete a local symbol */
    case 0x0C05:	/* delete a global symbol */
    case 0x0D05:	/* disable out-of-band characters */
    case 0x0E05:	/* enable out-of-band characters */
    case 0x0F05:	/* spawn a subprocess */
    case 0x1005:	/* attach to a subprocess */
    case 0x1105:	/* define a local  symbol using LIB$SET_SYMBOL */
    case 0x1205:	/* define a global symbol using LIB$SET_SYMBOL */
    case 0x1405:	/* delete a local symbol using LIB$DELETE_SYMBOL */
    case 0x1505:	/* delete a global symbol using LIB$DELETE_SYMBOL */
    case 0x1605:	/* set code set ???? */
    
    default:
        printf( "ERROR: Unknown SYS$CLI request %02X, %02X\n", request, subrequest );
        vax.halted = 1;
        rc = SS_INVARG;
    }
    
    
    return rc;
}


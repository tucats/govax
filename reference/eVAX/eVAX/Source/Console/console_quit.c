//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_quit.c
//
//  Purpose:    This module implements the QUIT or EXIT command which terminates the
//              emulation.
//
//  History:    11/25/99    Header standardization.  This was also adapted 11/17/99
//                          to be based on the DCL dispatch standard.
//



#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

LONGWORD console_exit_dcl( long id )
{
    
	int n;

/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        return VAX_NOVAX;
    }
    n = (int) id;
    vax.console.running = 0;
    
    return VAX_OK;
}

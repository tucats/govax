//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     memory_io.c
//
//  Purpose:    I/O interfaces to memory addresses.  This is called when the
//              physical address is in S2 space (the I/O area).  Currently, 
//              this is called from the storage.c module in the various load*()
//              routines.  
//
//              This is very unstructured as of yet because I haven't decide if
//              I want to keep this bound in here or not.  Since the physical address
//              map clearly can be disjoint on MicroVAX class system, I need to handle
//              references in the contiguous RAM versus the ROM versus the I/O space.
//
//              Originally I planned on allowing I/O references to be slow by virtual
//              of calling here to do the work -- I/O is, after all, often slow.  But
//              if I must also be able to handle reading data out of the ROM for the
//              console this becomes more of a performance issue.
//
//              Note:   The ROM on a MicroVAX 3100 appears to be at 20040000 in the
//                      physical address space.  Need to find out if this is true
//                      for all the teeny-VAXen; i.e. can I generalize this?
//
//
//  History:    09/10/99    Initially created, just to get started.



#include "vax.h"
#include "pte.h"
#include "memmap.h"

#if BIGENDIAN
#define ENDIAN( a,b,c,d) ( d << 24 | c << 16 | b << 8 | a )
#else

#define ENDIAN( a,b,c,d) ( a << 24 | b << 16 | c << 8 | d )
#endif

//
//  Load an I/O location (VAX is a little-endian system) to local storage.  Currently
//  disgustingly brute-force.  Later we'll need to consider ROM versus I/O space, and
//  arranging for some kind of dispatch vector for segments of memory, I think.
//

LONGWORD load_io( LONGWORD address, unsigned char * dest, short count)
{

    if( count != 4 ) {
		set_fault( EXC_RESOP, 0 );
		return VAX_FAULT;
	}

    switch( address ) {
    
    case 0xE0040004:    *(( LONGWORD * ) dest ) = ENDIAN( 0x04, 0x01, 0x00, 0x02 );
                        break;
    
    default:            set_fault( EXC_ACCVIO, 2, address, 1 );
    
                        return VAX_FAULT;
    
    }

    return VAX_OK;
}



//
//  Store an I/O location (VAX is a little-endian system) to local storage
//

LONGWORD store_io( LONGWORD address, unsigned char * src, short count)
{

	if( count != 4 ) {
		set_fault( EXC_RESOP, 0 );
		return VAX_FAULT;
	}

	if( src == 0L ) {
	    set_fault( EXC_ACCVIO, 2, address, 2 );
		return VAX_FAULT;
	}

    set_fault( EXC_ACCVIO, 2, address, 2 );
    

    return VAX_FAULT;
}


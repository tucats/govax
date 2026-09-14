//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_zero.c
//
//  Purpose:    This module implements the ZERO command, which zeroes out all
//              of the processor's physical memory.  It also sets the MAPEN
//              processor register to <0> since if all memory is zeroed then
//              there cannot be any page table data for virtual memory mapping.
//
//  History:    08/06/97    New header format standardization
//
//              05/25/99    When a ZERO command is given, clear the system
//                          symbol table of all non-permanent symbols, since
//                          the storage they presumably define is erased.
//
//              09/22/99    Clear MKVALID state when we zero memory, and clear
//                          the ROM and NVRAM areas.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

LONGWORD console_zero( char ** P )
{
    LONGWORD rc;
    char * p;
    LONGWORD n;
    
    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }


//  You can't do this if you aren't in kernel mode!  This is to prevent accidental
//  screw ups.

    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

    if( MKVALID )
        printf( "%s\n", vaxmsg( VAX_DELMK ));

//  Dump the system and user symbol tables.

    clear_system_symbols();
    clear_all_symbols();

//  Clear out physical memory.

    for( n = 0; (ULONGWORD) n < vax.memsize; n++ )
        vax.memory[ n ] = 0;

//  Reset the privileged registers to show that the memory (and memory-based items)
//  are all zeroed out.

    vax.MAPEN = 0;
    vax.vm_initialized = 0;   // If we've blown away memory then there can be no
                                //  pte entries, and so no vm initalization.
                                
    vax.SBR = 0L;
    vax.SLR = 0L;
    
    vax.P0BR = 0L;
    vax.P0LR = 0L;
    
    vax.P1BR = 0L;
    vax.P1LR = 0L;
    
    vax.SP = vax.memsize - 4;
    vax.PC = 0L;
    vax.FP = vax.SP;
    vax.AP = vax.SP;

    vax.ISP = vax.SP;   /*  Interrupt stack points to same place, and so on */
    vax.KSP = vax.SP;
    vax.SSP = vax.SP;
    vax.ESP = vax.SP;
    vax.USP = vax.SP;

    vax.console.microkernel_valid = 0;    /* No microkernel present! */
    vax.console.vminit_valid = 0;
    
    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return VAX_OK;
}

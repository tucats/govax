//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_push.c
//
//  Purpose:    Emulator handlers for PUSHL, PUSHAx instructions
//
//
//  History:    02/18/98        Initial implementation
//
//


#include "vax.h"

EMULATOR_ENTRY( emul_push )
{

    LONGWORD    value;
    LONGWORD    *addr;
    

    //  If this is PUSHL, then we must dereference a longword and use
    //  that as the object to push.  Otherwise, we just use the VAXaddr
    //  represented by the operand.
    
    if( opcode-> function == 0xDD ) {
        addr = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
        if( addr == 0L )
            return VAX_FAULT;
        value = *addr;
    }
    else {

    //  See if we are being given the address of a register or temporary, which
    //  is an illegal addressing mode.
    
        if( opcode-> is_register[ 0 ] )
            return set_fault( EXC_RESADDR, 0 );

        value = opcode-> VAXaddr[ 0 ];
    }
    
    
    //  Store the value/address on the stack and we're done.
    
    vax.SP -= 4;
    store_memory( vax.SP, ( void * ) &(value), 4 );


    return VAX_OK;
    
}
    
            


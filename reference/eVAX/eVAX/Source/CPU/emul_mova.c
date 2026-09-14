//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_mova.c
//
//  Purpose:    Emulator handlers for MOVAx instructions
//
//
//  History:    02/19/98        Initial implementation
//
//


#include "vax.h"

EMULATOR_ENTRY( emul_mova )
{

    LONGWORD    value, rc;

    //  See if the operand is a a register or temporary, which
    //  is an illegal addressing mode.
    
    if( opcode-> is_register[ 0 ] )
        return set_fault( EXC_RESADDR, 0 );
            

    //  The source is always the address of a dataum, so just get it's
    //  VAXaddress value.
    
    value = opcode-> VAXaddr[ 0 ];

    //  The destination is a "real" operand, so let's write to it.
    
    rc = put_operand( opcode, 1, OP_WR, ( void * ) &value );
    
    return rc;
    
}
    
            


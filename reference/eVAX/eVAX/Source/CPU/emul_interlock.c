//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_interlock.c
//
//  Purpose:    Emulator handlers for interlock instructions
//
//              This single routine handles all interlock instructions. Interlock
//              instructions support SMP environments.  Note that the emulator is
//              _only_ a uniprocessor implementaton, so the interlock aspects of
//              these instructions have no meaning.
//
//  History:    07/28/97    New header format standardization
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//



#include "vax.h"

EMULATOR_ENTRY( emul_interlock )
{


    char * src1, *src2;
    
    LONGWORD data, d1, d2;
    short dataw;
    unsigned char op;
    
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src1 = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src1 == 0L )
        return VAX_FAULT;
    
    
    src2 = ( char * ) get_operand( opcode, 1, OP_RD );
    if( src2 == 0L )
        return VAX_FAULT;
    

    op = opcode-> function;
    
    switch( op ) {
        
    case 0x58: /* ADAWI */
        
        
        //  The destination must be a memory address or it's a fault...
        
        if( opcode-> address[ 1 ] != 0L ) {
            set_fault( EXC_RESOP, 0 );
            return VAX_FAULT;
        }
        
        //  The destination must be an even address or it's a fault...
        
        if( opcode-> VAXaddr[ 1 ] & 0x00000001 ) {
            set_fault( EXC_RESOP, 0 );
            return VAX_FAULT;
        }
        
        d1 = *(( short * ) src1 );
        d2 = *(( short * ) src2 );
        
        data = d1 + d2;
        
        SETCONDITIONBITS( data, 0L );
        vax.pslw.v = ( data > 32767 ) || ( data < -32768 );
        dataw = ( short ) data;
        put_operand( opcode, 1, OP_WR, ( char * ) &dataw );
        break;

    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    


    return VAX_OK;
}



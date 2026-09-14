//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_clr.c
//
//  Purpose:    Emulator handlers for CLR instructions
//
//              The strategy is about the same for all data types.
//
//              1.  Create a zero value in local storage (of whatever the right size is).
//
//              2.  Set the condition bits correctly based on zero.
//
//              3.  Write the value to the output operand
//
//  History:    07/28/97    New header format standardization
//
//		10/08/01    Fixed bug where psl<c> was being cleared incorrectly.
//




#include "vax.h"

EMULATOR_ENTRY( emul_clrb )
{


    signed char data;
        
    data = 0;

    //  Set the condition bits
    
    vax.pslw.n = 0;
    vax.pslw.z = 1;
    vax.pslw.v = 0;
    
    //  Write the data to storage/register
    
    put_operand( opcode, 0, OP_WR, ( char * ) &data );

    return VAX_OK;
}



EMULATOR_ENTRY( emul_clrw )
{


    short data;
    
    data = 0;
    
    //  Set the condition bits
    
    vax.pslw.n = 0;
    vax.pslw.z = 1;
    vax.pslw.v = 0;
    
    //  Write the data to memory
    
    put_operand( opcode, 0, OP_WR, ( char * ) &data );


    return VAX_OK;
}


EMULATOR_ENTRY( emul_clrl )
{


    LONGWORD data;

    data = 0L;
    
    //  Set the condition bits
    
    vax.pslw.n = 0;
    vax.pslw.z = 1;
    vax.pslw.v = 0;
    
    //  Write the data to memory/register
    
    put_operand( opcode, 0, OP_WR, ( char * ) &data );


    return VAX_OK;
}



EMULATOR_ENTRY( emul_clrq )
{
    QUADWORD data;

    data = 0L;
    
    //  Set the condition bits
    
    vax.pslw.n = 0;
    vax.pslw.z = 1;
    vax.pslw.v = 0;
    
    //  Write the data to memory/register
    
    put_operand( opcode, 0, OP_WR, ( char * ) &data );


    return VAX_OK;
}

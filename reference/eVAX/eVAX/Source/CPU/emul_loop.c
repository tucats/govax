//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_loop.c
//
//  Purpose:    Emulator handlers for SOBGTR, SOBGEQ, AOBLSS, and AOBLEQ instructions
//
//
//  History:    07/28/97    New header format standardization
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//



#include "vax.h"



//  The AOBLEQ instruction

EMULATOR_ENTRY( emul_aobleq )
{

    LONGWORD    limit, *limit_a;
    LONGWORD    index, *index_a;
    LONGWORD    new_PC;

//  Get the limit test value.

    limit_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( limit_a == 0L )
        return VAX_FAULT;
    limit = *limit_a;


//  Get the index value to modify.

    index_a = ( LONGWORD * ) get_operand( opcode, 1, OP_MD );
    if( index_a == 0L )
        return VAX_FAULT;
    index = *index_a;

//  Get the branch address -- instruction decoding will have already
//  converted the displacement to a real address.

    new_PC = opcode-> VAXaddr[ 2 ];

//  Do the math, and store the result back away.

    index = index + 1;
    *index_a = index;
    
    SETCONDITIONBITS( index, 0L );

    if( index <= limit )
        vax.PC = new_PC;
    
    return VAX_OK;
    
}



//  The AOBLSS instruction

EMULATOR_ENTRY( emul_aoblss )
{

    LONGWORD    limit, *limit_a;
    LONGWORD    index, *index_a;
    LONGWORD    new_PC;

//  Get the limit test value.

    limit_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( limit_a == 0L )
        return VAX_FAULT;
    limit = *limit_a;


//  Get the index value to modify.

    index_a = ( LONGWORD * ) get_operand( opcode, 1, OP_MD );
    if( index_a == 0L )
        return VAX_FAULT;
    index = *index_a;

//  Get the branch address -- instruction decoding will have already
//  converted the displacement to a real address.

    new_PC = opcode-> VAXaddr[ 2 ];

//  Do the math, and store the result back away.

    index = index + 1;
    *index_a = index;
    
    SETCONDITIONBITS( index, 0L );

    if( index < limit )
        vax.PC = new_PC;
    
    return VAX_OK;
    
}



//  The SOBGTR instruction

EMULATOR_ENTRY( emul_sobgtr )
{

    LONGWORD    index, *index_a;
    LONGWORD    new_PC; 

//  Get the index value to modify.

    index_a = ( LONGWORD * ) get_operand( opcode, 0, OP_MD );
    if( index_a == 0L )
        return VAX_FAULT;

//  Get the branch address -- instruction decoding will have already
//  converted the displacement to a real address.

    new_PC = opcode-> VAXaddr[ 1 ];

//  Do the math, and store the result back away.

    index = *index_a = *index_a - 1;
    
    SETCONDITIONBITS( index, 0L );

    if( index > 0L )
        vax.PC = new_PC;
    
    return VAX_OK;
    
}




//  The SOBGEQ instruction

EMULATOR_ENTRY( emul_sobgeq )
{

    LONGWORD    index, *index_a;
    LONGWORD    new_PC; 

//  Get the index value to modify.

    index_a = ( LONGWORD * ) get_operand( opcode, 0, OP_MD );
    if( index_a == 0L )
        return VAX_FAULT;
    index = *index_a;

//  Get the branch address -- instruction decoding will have already
//  converted the displacement to a real address.

    new_PC = opcode-> VAXaddr[ 1 ];

//  Do the math, and store the result back away.

    index = index - 1;
    *index_a = index;
    
    SETCONDITIONBITS( index, 0L );

    if( index >= 0L )
        vax.PC = new_PC;
    
    return VAX_OK;
    
}



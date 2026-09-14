//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_extended.c
//
//  Purpose:    Emulator handlers for EMUL instructions
//
//
//  History:    07/28/97    New header format standardization
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//



#include "vax.h"



//  The EMUL instruction

EMULATOR_ENTRY( emul_emul )
{

    LONGWORD mulr, *mulr_a;
    LONGWORD muld, *muld_a;
    LONGWORD add,  *add_a;
    
    union XLONG {
        QUADWORD prod;
        LONGWORD reg[ 2 ];
    } xlong;
    
    
    mulr_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( mulr_a == 0L )
        return VAX_FAULT;
    mulr = *mulr_a;
    
    muld_a = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( muld_a == 0L )
        return VAX_FAULT;
    muld = *muld_a;
    
    add_a = ( LONGWORD * ) get_operand( opcode, 2, OP_RD );
    if( add_a == 0L )
        return VAX_FAULT;
    add = *add_a;
    
    xlong.prod = ( QUADWORD ) mulr * ( QUADWORD ) muld;
    xlong.prod = xlong.prod + ( QUADWORD ) add;
    
    opcode-> size[ 3 ] = 8;
    
    //  Have to swap the longwords so they store correctly
    
    add = xlong.reg[ 0 ];
    xlong.reg[ 0 ] = xlong.reg[ 1 ];
    xlong.reg[ 1 ] = add;
    
    return put_operand( opcode, 3, OP_WR, ( void * ) &xlong.prod );
    
}






//  The EDIV instruction

EMULATOR_ENTRY( emul_ediv )
{

    LONGWORD divr, *divr_a;
    LONGWORD quo, rem;
    
    union XLONG {
        QUADWORD prod;
        LONGWORD reg[ 2 ];
    } xlong, *xlong_a;
    QUADWORD xvalue;
    LONGWORD rc;
    
    
    
//  Get the divisor

    divr_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( divr_a == 0L )
        return VAX_FAULT;
    divr = *divr_a;

//  Get the dividend.  Have to swap the longwords so the math
//  works right.

    xlong_a = ( void * ) get_operand( opcode, 1, OP_RD );
    if( xlong_a == 0L )
        return VAX_FAULT;
    xlong = *xlong_a;
    rem = xlong.reg[ 0 ];
    xlong.reg[ 0 ] = xlong.reg[ 1 ];
    xlong.reg[ 1 ] = rem;
    
    if( divr == 0L ) {
    
        quo = xlong.reg[ 1 ];
        rem = 0L;
    }
    else {
    
        quo = ( long ) ( xlong.prod / divr );
        xvalue = ( QUADWORD ) quo * ( QUADWORD ) divr;
        rem = ( LONGWORD ) xlong.prod - ( LONGWORD ) xvalue;
    }

//  Store the operands

    rc = put_operand( opcode, 2, OP_WR, ( void * ) &quo );
    if( rc == VAX_OK )
        rc = put_operand( opcode, 3, OP_WR, ( void * ) &rem );
    
    return rc;
    
}



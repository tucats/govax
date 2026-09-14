//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_ash.c
//
//  Purpose:    Emulator handlers for ROTL, ASHL and ASHQ instructions
//
//
//  History:    07/28/97    New header format standardization
//
//              09/11/99    Fixed bug in ASHL with negative offset
//                          (was shifting wrong direction)
//
//              09/12/99    ASHQ implemented by Carl Fongheiser
//
//              09/22/99    Trivial fix of "LONGWORD LONGWORD" to "QUADWORD" for
//                          portability to Windows compilers
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//


#include "vax.h"

#define LEFT 'L'
#define RIGHT 'R'


//  The ROTL instruction

EMULATOR_ENTRY( emul_rotl )
{

    LONGWORD direction, count, n;
    ULONGWORD value, bit;
    LONGWORD *dest_a, *srcp;
    char bcount;

//  Get the count of bit(s) to move the value

     
    GET_OPERAND( bcount, char, 0, OP_RD );
    count = bcount;

    if( count < 0 ) {
        direction = RIGHT;
        count = -count;
    }
    else
        direction = LEFT;

//  Get the source longword value.

    //GET_OPERAND( value, ULONGWORD, 1, OP_RD );
    srcp = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( srcp == 0L )
        return VAX_FAULT;
    value = *srcp;
    
//  Verify that the destination is accessible

    dest_a = ( LONGWORD * ) get_operand( opcode, 2, OP_WR );
    if( dest_a == 0L )
        return VAX_FAULT;


//  Based on direction, shift the right number of times.

    if( direction == RIGHT ) {
    
        for( n = 0; n < count; n++ ) {
            bit = value & 0x00000001;
            value = value >> 1;
            if( bit )
                value = value + 0x80000000UL;
        }
    }
    else {
    
        for( n = 0; n < count; n++ ) {
        
            bit = value & 0x80000000UL;
            value = value << 1;
            if( bit )
                value = value + 0x00000001UL;
        }
    }

    /* value is ULONGWORD (fixed 32-bit) so this mask is now a no-op;
       kept as a defensive belt-and-suspenders guard -- see AUDIT.md 5.1 */
    value &= 0xFFFFFFFFUL;

    SETCONDITIONBITS( value, 0L );
    vax.pslw.v = 0;
    
    return put_operand( opcode, 2, OP_WR, ( void * ) &value );
    
}


//  The ASHL and ASHQ instruction(s)

EMULATOR_ENTRY( emul_ash )
{


    LONGWORD source, *source_a;
    signed char count, *count_a;
    LONGWORD data;
    QUADWORD qdata, qsource, *qsource_a;
    
    //  Get address of count operand byte
    
    count_a = ( signed char * ) get_operand( opcode, 0, OP_RD );
    if( count_a == 0L )
        return VAX_FAULT;
    count = *count_a;
    
    //  Get address of data to shift.
    
    source_a = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( source_a == 0L )
        return VAX_FAULT;
    

    switch( opcode-> function ) {
    
    case 0x78:  /* ASHL */
    
        source = *source_a;
        
        if( count < 0 ) {
       
            if( count <= -31 )
                data = ( source < 0 ) ? -1 : 0;
            else
                data = sext(( source >> (-count) ), 4 );
        }
        else {
        
            if( count > 32 )
                data = 0L;
            else
                data = source << count;
        }
        
        SETCONDITIONBITS( data, 0L );
    
        put_operand( opcode, 2, OP_WR, ( void * ) &data );
        break;

    case 0x79:        /* ASHQ */

        qsource_a = (QUADWORD *) source_a;
        qsource = *qsource_a;

        if (count < 0) {
           if (count <= -63)
               qdata = (qsource < 0) ? -1 : 0;
           else
               qdata = qsource >> -count;
        }
        else {

           if (count > 64)
               qdata = 0;
           else
               qdata = qsource << count;
        }

        SETCONDITIONBITS(qdata, 0L);

        put_operand(opcode, 2, OP_WR, ( void * ) &qdata);
        break;

    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    


    return VAX_OK;
}



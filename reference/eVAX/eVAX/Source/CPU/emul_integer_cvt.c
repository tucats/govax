//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_integer_cvt.c
//
//  Purpose:    Emulator handlers for integer CVTxy instructions
//
//
//  History:    02/12/98        Initial implementation
//
//              09/27/99        Tweaks to make system 64-bit friendly.
//
//		10/08/01	PSL<V> not always set correctly for
//				CVTxL instructions.





#include "vax.h"

EMULATOR_ENTRY( emul_integer_cvt )
{
    LONGWORD op, dsize;
    LONGWORD data;
    LONGWORD * src, *dst;
    short dataw;
    char datab;
    
/*
 *  Get address of source and destination.
 */

        
    src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;

    dst = ( LONGWORD * ) get_operand( opcode, 1, OP_WR );
    if( dst == 0L )
        return VAX_FAULT;

/*
 *  The instructions handled are:
 *
 *      0 0 1 1   0 0 1 0       CVTWL
 *      0 0 1 1   0 0 1 1       CVTWB
 *
 *      1 0 0 1   1 0 0 0       CVTBL
 *      1 0 0 1   1 0 0 1       CVTBW
 *
 *      1 1 1 1   0 1 1 0       CVTLB
 *      1 1 1 1   0 1 1 1       CVTLW
 *
 */

    op = opcode-> function;
    
/*
 *  Determine the data size that the source is.
 */
 
    dsize = ( op >> 4 ) & 0x0F;     /* 3 = W  9 = B  F = L */

/*
 *  Based on the datasize, read the value of the correct size.
 */

    switch( dsize ) {
    
    case 0x09:  /* Byte */
        data = * ( char * ) src;
        data = sext( data, 1 );
        break;
    
    case 0x03:  /* Word */
        data = * ( short * ) src;
        data = sext( data, 2 );
        break;
    
    case 0x0F:  /* Long */
        data = *src;
        break;
    
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    }
        
/*
 *  Set the condition bits for the data.
 */

    SETCONDITIONBITS( data, 0L );
    vax.pslw.c = 0;
    
/*
 *  Based on destination datasize, do the right thing.
 */
 
    dsize = op & 0x0F;  /* 3,6 = B   7,9 = W  2,8 = L */
    
    switch( dsize ) {
    
    case 3: /* Byte */
    case 6:
        
        vax.pslw.v = ( data > 255 ) || ( data < -256 );
        
        datab = ( char ) data;
        
        put_operand( opcode, 1, OP_WR, ( char * ) &datab );
        break;
    
    case 7: /* Word */
    case 9:
        
        vax.pslw.v = ( data > 32767 ) || ( data < -32768 );
        
        dataw = ( short ) data;
        put_operand( opcode, 1, OP_WR, ( char * ) &dataw );
        break;
    
    case 2: /* Longword */
    case 8:
    
        vax.pslw.v = 0;
        put_operand( opcode,  1, OP_WR, ( char * ) &data );
        break;
    
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    

    return VAX_OK;
}



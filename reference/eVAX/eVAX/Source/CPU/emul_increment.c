//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_increment.c
//
//  Purpose:    Emulator handlers for INCx and DECx instructions
//
//
//  History:    02/12/98        Initial implementation
//
//              09/27/99        Tweaks to make system 64-bit friendly.
//
//		10/08/01	DECL didn't set PSL<C> bit correctly.





#include "vax.h"

EMULATOR_ENTRY( emul_increment )
{
    LONGWORD op, dsize;
    LONGWORD data, increment;
    char * src;
    LONGWORD d1, udata;
    short dataw;
    char datab;
    
/*
 *  Get address of destination.
 */

        
    src = ( char * ) get_operand( opcode, 0, OP_MD );
    if( src == 0L )
        return VAX_FAULT;

/*
 *  The instructions handled are:
 *
 *      1 0 0 1   0 1 1 0       INCB
 *      1 0 0 1   0 1 1 1       DECB
 *      1 0 1 1   0 1 1 0       INCW
 *      1 0 1 1   0 1 1 1       DECW
 *      1 1 0 1   0 1 1 0       INCL
 *      1 1 0 1   0 1 1 1       DECL
 *
 */

    op = opcode-> function;
    
/*
 *  Determine if we are doing decrement or increment.  Also use the
 *  upper bits of the opcode to determine the data size.
 */
 
    increment = ( op & 0x01 ) ? -1L : 1L;
    dsize = ( op >> 5 ) & 0x07;

/*
 *  Based on datasize, do the right thing.
 */
 
    switch( dsize ) {
    
    case 4: /* Byte */
    
        d1 = *(( char * ) src );    //  Get value
        data = d1 + increment;
        
        SETCONDITIONBITS( data, 0L );
        vax.pslw.v = ( data > 255 ) || ( data < -256 );
        vax.pslw.c = ( data & 0x00000100 ) >> 8;
        
        datab = ( char ) data;
        
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &datab );
        break;
    
    case 5: /* Word */
            
        d1 = *(( short * ) src );
        data = d1 + increment;
        
        SETCONDITIONBITS( data, 0L );
        vax.pslw.v = ( data > 32767 ) || ( data < -32768 );
        vax.pslw.c = ( data & 0x00010000 ) >> 16;
        
        dataw = ( short ) data;
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &dataw );
        break;
    
    case 6: /* Longword */
    
    
    
        d1 = *(( LONGWORD * ) src );
             
        data = d1 + increment;
        udata = ( ULONGWORD ) d1 + increment;
        
        SETCONDITIONBITS( data, 0L );

       if(( d1 == 0L ) && ( increment < 0 ))
            vax.pslw.c = 1;
        else
            vax.pslw.c = 0;
        vax.pslw.v = ( data == udata ) ? 0 : 1;
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &data );
        break;
    
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    

    return VAX_OK;
}



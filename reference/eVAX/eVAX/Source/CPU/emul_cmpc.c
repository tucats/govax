//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_cmpc.c
//
//  Purpose:    Emulator handlers for character compare instructions
//              and similar instructions.
//
//  History:    07/28/97    New header format standardization
//
//




#include "vax.h"



//  Handler for CMPC3

EMULATOR_ENTRY( emul_cmpc3 )
{

    char    src1;
    char    src2;

    short   len, *len_a;

    LONGWORD    tmp1, tmp2, tmp3;

//  See if the addresses are registers

    if( opcode-> is_register[ 1 ] || opcode-> is_register[ 2 ])
        return set_fault( EXC_RESOP, 0 );

//  Get addressability to the length.

    len_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    len = *len_a;

//  Load up the working values

    tmp1 = len;
    tmp2 = opcode-> VAXaddr[ 1 ];
    tmp3 = opcode-> VAXaddr[ 2 ];
        
    if( tmp1 == 0 )
        SETCONDITIONBITS( tmp1, 0L );
    
    if( tmp1 > 0 ) {
    
        while( tmp1 != 0 ) {
    
            load_memory( tmp2, ( void * ) &src1, 1 );
            load_memory( tmp3, ( void * ) &src2, 1 );
            
            SETCONDITIONBITS( src1, src2 );
            
            if( src1 == src2 ) {
                tmp1 -= 1;
                tmp2 += 1;
                tmp3 += 1;
                continue;
            }
            break;
        }
    
    }
    
    vax.R0 = tmp1;
    vax.R1 = tmp2;
    vax.R2 = vax.R0;
    vax.R3 = tmp3;

    vax.pslw.v = 0;

    return VAX_OK;
}




EMULATOR_ENTRY( emul_cmpc5 )
{

    char    src1;
    char    src2, fill, *fill_a;

    short   *len_a;

    LONGWORD    tmp1, tmp2, tmp3, tmp4;

//  See if the addresses are registers

    if( opcode-> is_register[ 1 ] || opcode-> is_register[ 4 ])
        return set_fault( EXC_RESOP, 0 );

//  Get addressability to the first length.

    len_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    tmp1 = *len_a;
    
//  Get addressability to the second length.

    len_a = ( short * ) get_operand( opcode, 3, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    tmp3 = *len_a;

//  Get the fill byte

    fill_a = ( char * ) get_operand( opcode, 2, OP_RD );
    if( !fill_a )
        return VAX_FAULT;
    fill = *fill_a;
    
//  Load up the other working values

    tmp2 = opcode-> VAXaddr[ 1 ];
    tmp4 = opcode-> VAXaddr[ 4 ];
        
    SETCONDITIONBITS( tmp1, tmp3 );
    
    if( tmp1 > 0 && tmp3 > 0 ) {
    
        while( tmp1 != 0 && tmp3 != 0 ) {
    
            load_memory( tmp2, ( void * ) &src1, 1 );
            load_memory( tmp4, ( void * ) &src2, 1 );
            
            SETCONDITIONBITS( src1, src2 );
            
            if( src1 == src2 ) {
                tmp1 -= 1;
                tmp2 += 1;
                tmp3 -= 1;
                tmp4 += 1;
                continue;
            }
            break;
        }
        
        while( tmp1 != 0 ) {
            load_memory( tmp2, ( void * ) &src2, 1 );
            SETCONDITIONBITS( src2, fill );
            if( fill == src2 ) {
                tmp1--;
                tmp2++;
                continue;
            }
            break;
        }
        
        while( tmp3 != 0 ) {
            load_memory( tmp4, ( void * ) &src2, 1 );
            SETCONDITIONBITS( fill, src2 );
            if( fill == src2 ) {
                tmp3--;
                tmp4++;
                continue;
            }
            break;
        }
    }
    
    vax.R0 = tmp1;
    vax.R1 = tmp2;
    vax.R2 = tmp3;
    vax.R3 = tmp4;
    
    vax.pslw.v = 0;
    
    return VAX_OK;
}



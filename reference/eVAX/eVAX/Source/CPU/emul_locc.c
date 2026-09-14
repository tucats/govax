//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_locc.c
//
//  Purpose:    Emulator handlers for LOCC instruction
//
//  History:    07/28/97    New header format standardization
//
//              12/09/99    Fixed but in Z bit setting when char found
//


#include "vax.h"


EMULATOR_ENTRY( emul_locc )
{

    unsigned char   match, test;
    unsigned short  len;
    LONGWORD    addr, rc;

    GET_OPERAND( match, unsigned char, 0, OP_RD );
    GET_OPERAND( len, unsigned short, 1, OP_RD );

    if( opcode-> is_register[ 2 ] != OP_MEMORY )
        return set_fault( EXC_RESOP, 0 );

    addr = opcode-> VAXaddr[ 2 ];

    vax.pslw.n = 0;
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    while( len ) {

        rc = load_byte( addr, &test );
        if( test == match ) {
            vax.R0 = len;
            vax.R1 = addr;
            vax.pslw.z = 0;
            return VAX_OK;
        }
        addr++;
        len--;
    }

    vax.R0 = 0;
    vax.R1 = addr;
    vax.pslw.z = 1;

    return VAX_OK;
}


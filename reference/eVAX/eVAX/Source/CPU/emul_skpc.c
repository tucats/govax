//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Carl Fongheiser
//
//  Module:     emul_skpc.c
//
//  Purpose:    This routine contains instruction handler for SKPC
//
//
//  History:    08/27/99    Module integrated into master source
//
//


#include "vax.h"

// SKPC instruction

EMULATOR_ENTRY(emul_skpc)
{
    unsigned short tmp1;
    unsigned char ch, ch2;
    ULONGWORD tmp2;

    GET_OPERAND(ch, unsigned char, 0, OP_RD);
    GET_OPERAND(tmp1, unsigned short, 1, OP_RD);
    tmp2 = opcode->VAXaddr[2];

    vax.pslw.n = 0;
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    while (tmp1) {
        load_byte(tmp2, &ch2);
        if (ch != ch2)
        break;
        tmp1--;
        tmp2++;
    }

    vax.R0 = tmp1;
    vax.R1 = tmp2;
    vax.pslw.z = (tmp1 == 0);

    return VAX_OK;
}

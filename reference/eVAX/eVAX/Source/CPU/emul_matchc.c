//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Carl Fongheiser
//
//  Module:     emul_matchc.c
//
//  Purpose:    This routine contains instruction handler for MATCHC
//
//
//  History:    08/23/99    Module integrated into master source
//
//

#include "vax.h"

EMULATOR_ENTRY(emul_matchc)
{
    unsigned short tmp1, tmp3, tmp5;
    ULONGWORD tmp2, tmp4;

    GET_OPERAND(tmp1, unsigned short, 0, OP_RD);
    tmp2 = opcode->VAXaddr[1];
    GET_OPERAND(tmp3, unsigned short, 2, OP_RD);
    tmp4 = opcode->VAXaddr[3];
    tmp5 = tmp1;

    while (tmp1 && tmp3 >= tmp1) {
            unsigned char a, b;
            LONGWORD rc;

            rc = load_memory(tmp2, (void *) &a, 1);
        if (rc)
        return rc;
        rc = load_memory(tmp4, (void *) &b, 1);
        if (rc)
        return rc;
        if (a == b) {
        tmp1--;
        tmp2++;
        tmp3--;
        tmp4++;
        } else {
        tmp2 -= (tmp5 - tmp1);
        tmp3 += (tmp5 - tmp1 - 1);
        tmp4 -= (tmp5 - tmp1 - 1);
        tmp1 = tmp5;
        }
    }
        if (tmp3 < tmp1) {
        tmp4 += tmp3;
        tmp3 = 0;
    }
    vax.R0 = tmp1;
    vax.R1 = tmp2;
    vax.R2 = tmp3;
    vax.R3 = tmp4;
    vax.pslw.n = 0;
    vax.pslw.z = (tmp1 == 0);
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    return VAX_OK;
}

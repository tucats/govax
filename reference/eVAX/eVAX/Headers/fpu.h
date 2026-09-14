//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     fpu.h
//
//  Purpose:    This header file defines elements of the fpu support in the
//              virtual VAX.
//
//
//  History:    07/28/97    New header format standardization
//
//              09/07/98    Added fault types.
//
//


/*
 *  F A U L T    T Y P E S
 *
 *  These correspond to the SRM fault types.
 */

#define TRAP_INT_OVF    0x01        /* SMI$K_INT_OVF_T */
#define TRAP_INT_DIV    0x02        /* SMI$K_INT_DIV_T */
#define TRAP_FLT_OVF    0x03        /* SMI$K_FLT_OVF_T */
#define TRAP_FLT_DIV    0x04        /* SMI$K_FLT_DIV_T */
#define TRAP_FLT_UND    0x05        /* SMI$K_FLT_UND_T */
#define TRAP_DEC_OVF    0x06        /* SMI$K_DEC_OVF_T */
#define TRAP_SUB_RNG    0x07        /* SMI$K_SUB_RNG_T */

#define FAULT_FLT_OVF   0x08        /* SMI$K_INT_OVF_F */
#define FAULT_FLT_DIV   0x09        /* SMI$K_INT_DIV_F */
#define FAULT_FLT_UND   0x0A        /* SMI$K_INT_UND_F */



/*
 *  P R O T O T Y P E S
 */
 
LONGWORD fpu_load( LONGWORD src1, LONGWORD src2, double * result );

LONGWORD fpu_store( double source, LONGWORD * dst1, LONGWORD * dst2 );

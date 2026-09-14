//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     register.c
//
//  Purpose:    Handles interface to the virtual CPU registers.  This includes
//              formatted dumping of CPU  registers and the formatted PSL value.
//              It also includes the console interfaces for setting or examining
//              register values.
//
//  History:    07/28/97    New header format standardization
//
//              03/17/00    Performance tweaks in PSL management routines.
//
//		10/22/01    Added register tracking functions.


#include "vax.h"
#include "asmproto.h"
#include "regtrack.h"


static struct REGLIST {
    LONGWORD    verb;
    short   n;
    char *  name;
} reglist[] = {
{   CHAR4('R','0',' ',' '), 0,  "R0"    },
{   CHAR4('R','1',' ',' '), 1,  "R1"    },
{   CHAR4('R','2',' ',' '), 2,  "R2"    },
{   CHAR4('R','3',' ',' '), 3,  "R3"    },
{   CHAR4('R','4',' ',' '), 4,  "R4"    },
{   CHAR4('R','5',' ',' '), 5,  "R5"    },
{   CHAR4('R','6',' ',' '), 6,  "R6"    },
{   CHAR4('R','7',' ',' '), 7,  "R7"    },
{   CHAR4('R','8',' ',' '), 8,  "R8"    },
{   CHAR4('R','9',' ',' '), 9,  "R9"    },
{   CHAR4('R','1','0',' '), 10, "R10"   },
{   CHAR4('R','1','1',' '), 11, "R11"   },
{   CHAR4('R','1','2',' '), 12, "AP"    },
{   CHAR4('R','1','3',' '), 13, "FP"    },
{   CHAR4('R','1','4',' '), 14, "SP"    },
{   CHAR4('R','1','5',' '), 15, "PC"    },
{   CHAR4('A','P',' ',' '), 12, "AP"    },
{   CHAR4('F','P',' ',' '), 13, "FP"    },
{   CHAR4('S','P',' ',' '), 14, "SP"    },
{   CHAR4('P','C',' ',' '), 15, "PC"    },
{   0,  0, ""   }};

static char * blanks = "                    ";


/*
 *  Save a register set -- just a snapshot of the current register set.
 */

int save_regset( struct REGSET * rs )
{
    int n;
    for( n = 0; n < 14; n++ )
        rs->reg[ n ] = vax.reg[ n ];

    write_psl_bits();
    rs->psl = vax.psl.reg;
    return 0;
}

int check_regset( struct REGSET * rs )
{
    int n;
    for( n = 0; n < 14; n++ ) {
        if( rs->reg[ n ] != vax.reg[ n ] ) {
            if( n < 12 )
                printf( "%s%3s:  %08X  %u\n", blanks, reglist[n].name, vax.reg[ n ], vax.reg[ n ]);
            else
                printf( "%s%3s:  %08X\n", blanks, reglist[n].name, vax.reg[ n ] );
        }
    }
    write_psl_bits();
    if( rs->psl != vax.psl.reg )
        printf("%sPSL:  %08X\n", blanks, vax.psl.reg );
    return 0;
}

/*
 *  Read the PSL bit array from the stored PSL and update the "Wide"
 *  version kept in shorts that is faster to use.  We call this when
 *  some instruction has modified the PSL register directly.  Normally
 *  we just update the wide versions.
 */

LONGWORD read_psl_bits(void)
{

    /* If we are changing the current mode, then we need to */
    /* flush are cached protection data.                    */
    
    if( vax.psl.bit.cur_mod != vax.pslw.cur_mod )
        invalidate_tb_prot();
    
    /* Now move all the bits */
    
    vax.pslw.cm       = vax.psl.bit.cm;
    vax.pslw.tp       = vax.psl.bit.tp;
    vax.pslw.fpd      = vax.psl.bit.fpd;
    vax.pslw.is       = vax.psl.bit.is;
    vax.pslw.cur_mod  = vax.psl.bit.cur_mod;
    vax.pslw.prv_mod  = vax.psl.bit.prv_mod;
    vax.pslw.ipl      = vax.psl.bit.ipl;
    vax.pslw.dv       = vax.psl.bit.dv;
    vax.pslw.fu       = vax.psl.bit.fu;
    vax.pslw.iv       = vax.psl.bit.iv;
    vax.pslw.t        = vax.psl.bit.t;
    vax.pslw.n        = vax.psl.bit.n;
    vax.pslw.z        = vax.psl.bit.z;
    vax.pslw.v        = vax.psl.bit.v;
    vax.pslw.c        = vax.psl.bit.c;
    
    return VAX_OK;
}



/*
 *  Write the PSL bit array in the stored PSL from the "Wide"
 *  version kept in shorts that is faster to use.  We call this when
 *  some instruction needs to use the PSL register directly, and we
 *  need to resynch the bits in the array from the cached copies.
 */

LONGWORD write_psl_bits(void)
{
    
    /* If we are changing the current mode, then we need to */
    /* flush are cached protection data.                    */
    
    if( vax.psl.bit.cur_mod != vax.pslw.cur_mod )
        invalidate_tb_prot();
    
    /* Now move all the bits */
    
    vax.psl.bit.cm    = (unsigned int) vax.pslw.cm;
    vax.psl.bit.tp    = (unsigned int) vax.pslw.tp;
    vax.psl.bit.fpd   = (unsigned int) vax.pslw.fpd;
    vax.psl.bit.is    = (unsigned int) vax.pslw.is;
    vax.psl.bit.cur_mod   = (unsigned int) vax.pslw.cur_mod;
    vax.psl.bit.prv_mod   = (unsigned int) vax.pslw.prv_mod;
    vax.psl.bit.ipl   = (unsigned int) vax.pslw.ipl;
    vax.psl.bit.dv    = (unsigned int) vax.pslw.dv;
    vax.psl.bit.fu    = (unsigned int) vax.pslw.fu;
    vax.psl.bit.iv    = (unsigned int) vax.pslw.iv;
    vax.psl.bit.t     = (unsigned int) vax.pslw.t;
    vax.psl.bit.n     = (unsigned int) vax.pslw.n;
    vax.psl.bit.z     = (unsigned int) vax.pslw.z;
    vax.psl.bit.v     = (unsigned int) vax.pslw.v;
    vax.psl.bit.c     = (unsigned int) vax.pslw.c;
        
    return VAX_OK;
}


/*
 *  See if the command stream contains a set for a register.  If so, do the
 *  work and return VAX_OK.  If it isn't a SET <register> command, then return
 *  -1
 */
 
LONGWORD set_reg( char ** P )
{
    LONGWORD n, found;
    char * p;
    LONGWORD verb, rc;
    
    p = *P;

    /*  Pick off the verb, which might be a register name */
    
    rc = read_verb( &p, &verb );
    if( rc )
        return rc;
    
    /*  Search the register list to see if it's a known one */
    
    found = -1;
    for( n = 0; reglist[ n ].verb; n++ ) {
        if( reglist[ n ].verb == verb ) {
            found = reglist[ n ].n;
            break;
        }
    }
    
    if( found == -1 )
        return -1;

    flush_blanks( &p );
    if( *p != '=' )
        return -1;
    p++;
    
    rc = asm_expr( &p, ( LONGWORD * ) &n );
    if( rc )
        return rc;
    
    vax.reg[ found ] = n;
    *P = p;
    return VAX_OK;
    
}


/*
 *  Given a verb that contains a register name, display it.
 */

LONGWORD exam_reg( LONGWORD verb )
{

    LONGWORD n;
    
    for( n = 0; reglist[ n ].verb; n++ ) {
        if( reglist[ n ].verb == verb ) {
            printf( "   REG  %6s = %08X (hex)  %12u (dec)\n", reglist[ n ].name, 
                vax.reg[ reglist[ n ].n ], vax.reg[ reglist[ n ].n ]);
            return VAX_OK;
        }
    }
    return -1;
}

/*
 *  Given a verb that contains a register name, display it.
 */

LONGWORD exam_reg_string( char * p )
{

    LONGWORD n;
    
    for( n = 0; reglist[ n ].verb; n++ ) {
        if(strcmp( reglist[ n ].name, p ) == 0 ) {
            printf( "   REG  %6s = %08X (hex)  %12u (dec)\n", reglist[ n ].name, 
                vax.reg[ reglist[ n ].n ], vax.reg[ reglist[ n ].n ]);
            return VAX_OK;
        }
    }
    return -1;
}




//  Register dump routine
//

LONGWORD dump_registers(void)
{

    printf( "\n\n    Registers:\n\n" );
    printf( "    R0:  %08X      R4:  %08X     R8:  %08X     AP:  %08X\n",
        vax.R0, vax.R4, vax.R8, vax.AP );
        
    printf( "    R1:  %08X      R5:  %08X     R9:  %08X     FP:  %08X\n",
        vax.R1, vax.R5, vax.R9, vax.FP );
        
    printf( "    R2:  %08X      R6:  %08X     R10: %08X     SP:  %08X\n",
        vax.R2, vax.R6, vax.R10, vax.SP );
        
    printf( "    R3:  %08X      R7:  %08X     R11: %08X     PC:  %08X\n\n",
        vax.R3, vax.R7, vax.R11, vax.PC );
    
    dump_psl();
    
    return VAX_OK;
    
}



//  Routine to format and display the PSL.

LONGWORD dump_psl(void)
{

    static char * mode_name[] = { "KERNEL", "EXEC", "SUPER", "USER" };

    write_psl_bits();   /* Make bits match processor cache */
    
    printf( "    PSL: %08X\n", vax.psl.reg );
    printf( "         PSW:  C=%d, V=%d, Z=%d, N=%d, T=%d, IV=%d, FU=%d, DV=%d\n",
                                vax.psl.bit.c, 
                                vax.psl.bit.v, 
                                vax.psl.bit.z, 
                                vax.psl.bit.n,
                                vax.psl.bit.t, 
                                vax.psl.bit.iv, 
                                vax.psl.bit.fu, 
                                vax.psl.bit.dv );

    printf( "         PRIV: IPL=%d, CUR_MOD=%s, PRV_MOD=%s, FPD=%d, TP=%d, CM=%d\n",
                                vax.psl.bit.ipl,
                                mode_name[ vax.psl.bit.cur_mod ], 
                                mode_name[ vax.psl.bit.prv_mod ],
                                vax.psl.bit.fpd, 
                                vax.psl.bit.tp,
                                vax.psl.bit.cm );
    return VAX_OK;
}


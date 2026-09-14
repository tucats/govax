//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     init_emulators.c
//
//  Purpose:    Initialize the emulator dispatch array.  For each instruction
//              that has been implemented, the dispatch routine address is set
//              to the appropriate entry point.  Don't add it here until it really
//              works, since the "unimplemented instruction" fault keys on the
//              non-zero routine entry value.
//
//  History:    07/28/97    New header format standardization
//
//

#include "vax.h"
#include "vaxinstr.h"


LONGWORD init_emulators(void)
{

    long    n;
    
    vax.inst_table = instruction;
  
    //  Start by initializing every slot to the unimplemented handler
    
    for( n = 0; n < 500; n++ ) {
        if( instruction[ n ].name[ 0 ] == 0 )
            break;
        instruction[ n ].routine = emul_unimplemented;
    }
    
    
    //  Now go back and explicitly override for the implemented ones
        
    instruction[ 0x00 ].routine = emul_halt;
    instruction[ 0x01 ].routine = emul_nop;
    instruction[ 0x02 ].routine = emul_rei;
    instruction[ 0x03 ].routine = emul_bpt;
    instruction[ 0x04 ].routine = emul_ret;
    instruction[ 0x05 ].routine = emul_branch;              /* RSB   */
    instruction[ 0x06 ].routine = emul_ldpctx;
    instruction[ 0x07 ].routine = emul_svpctx;
    instruction[ 0x0A ].routine = emul_index;
    instruction[ 0x0B ].routine = emul_crc;
    instruction[ 0x0C ].routine = emul_probe;               /* PROBER */
    instruction[ 0x0D ].routine = emul_probe;               /* PROBEW */
    instruction[ 0x0E ].routine = emul_insque;
    instruction[ 0x0F ].routine = emul_remque;

    instruction[ 0x10 ].routine = emul_branch;              /* BSBB  */
    instruction[ 0x11 ].routine = emul_branch_always;       /* BRB   */
    instruction[ 0x12 ].routine = emul_bneq;  
    instruction[ 0x13 ].routine = emul_beql; 
    instruction[ 0x14 ].routine = emul_bgtr; 
    instruction[ 0x15 ].routine = emul_bleq;
    instruction[ 0x16 ].routine = emul_branch;              /* JSB   */
    instruction[ 0x17 ].routine = emul_branch_always;       /* JMP   */
    instruction[ 0x18 ].routine = emul_bgeq;
    instruction[ 0x19 ].routine = emul_blss;
    instruction[ 0x1A ].routine = emul_bgtru;
    instruction[ 0x1B ].routine = emul_blequ;
    instruction[ 0x1C ].routine = emul_bvc;      
    instruction[ 0x1D ].routine = emul_bvs;         
    instruction[ 0x1E ].routine = emul_bgequ;  
    instruction[ 0x1F ].routine = emul_bcs;   
    
    instruction[ 0x28 ].routine = emul_movc3;
    instruction[ 0x29 ].routine = emul_cmpc3;
    instruction[ 0x2A ].routine = emul_scanc;
    instruction[ 0x2B ].routine = emul_scanc;               /* SPANC */
    instruction[ 0x2C ].routine = emul_movc5;
    instruction[ 0x2D ].routine = emul_cmpc5;
    instruction[ 0x2E ].routine = emul_movtc;
    instruction[ 0x2F ].routine = emul_movtuc;
    
    instruction[ 0x30 ].routine = emul_branch;              /* BSBW  */
    instruction[ 0x31 ].routine = emul_branch_always;       /* BRW   */
    instruction[ 0x32 ].routine = emul_integer_cvt;         /* CVTWL */
    instruction[ 0x33 ].routine = emul_integer_cvt;         /* CVTWB */
    instruction[ 0x39 ].routine = emul_matchc;
    instruction[ 0x3A ].routine = emul_locc;
    instruction[ 0x3B ].routine = emul_skpc;
    instruction[ 0x3C ].routine = emul_movzwl;
    instruction[ 0x3D ].routine = emul_acb;                 /* ACBW  */
    instruction[ 0x3E ].routine = emul_mova;                /* MOVAB */
    instruction[ 0x3F ].routine = emul_push;                /* PUSHAB */

    instruction[ 0x40 ].routine = emul_float_math;          /* ADDF2  */
    instruction[ 0x41 ].routine = emul_float_math;          /* ADDF3  */
    instruction[ 0x42 ].routine = emul_float_math;          /* SUBF2  */
    instruction[ 0x43 ].routine = emul_float_math;          /* SUBF3  */
    instruction[ 0x44 ].routine = emul_float_math;          /* MULF2  */
    instruction[ 0x45 ].routine = emul_float_math;          /* MULF3  */
    instruction[ 0x46 ].routine = emul_float_math;          /* DIVF2  */
    instruction[ 0x47 ].routine = emul_float_math;          /* DIVF3  */
    
    instruction[ 0x48 ].routine = emul_float_math;          /* CVTFB  */
    instruction[ 0x49 ].routine = emul_float_math;          /* CVTFW  */
    instruction[ 0x4A ].routine = emul_float_math;          /* CVTFL  */
    instruction[ 0x4B ].routine = emul_float_math;          /* CVTRFL */
    instruction[ 0x4C ].routine = emul_float_math;          /* CVTBF  */
    instruction[ 0x4D ].routine = emul_float_math;          /* CVTWF  */
    instruction[ 0x4E ].routine = emul_float_math;          /* CVTLF  */
    instruction[ 0x4F ].routine = emul_acb;                 /* ACBF   */
    
    instruction[ 0x50 ].routine = emul_movf;
    instruction[ 0x51 ].routine = emul_cmp;                 /* CMPF  */
    instruction[ 0x52 ].routine = emul_movf;                /* MNEGF */
    instruction[ 0x53 ].routine = emul_cmp;                 /* TSTF  */
    
    instruction[ 0x58 ].routine = emul_interlock;           /* ADAWI */
    instruction[ 0x5C ].routine = emul_insqhi;
    instruction[ 0x5D ].routine = emul_insqti;
    instruction[ 0x5E ].routine = emul_remqhi;
    instruction[ 0x5F ].routine = emul_remqti;
    
    instruction[ 0x60 ].routine = emul_float_math;          /* ADDD2 */
    instruction[ 0x61 ].routine = emul_float_math;          /* ADDD3 */
    instruction[ 0x62 ].routine = emul_float_math;          /* SUBD2 */
    instruction[ 0x63 ].routine = emul_float_math;          /* SUBD3 */
    instruction[ 0x64 ].routine = emul_float_math;          /* MULD2 */
    instruction[ 0x65 ].routine = emul_float_math;          /* MULD3 */
    instruction[ 0x66 ].routine = emul_float_math;          /* DIVD2 */
    instruction[ 0x67 ].routine = emul_float_math;          /* DIVD3 */
    
    instruction[ 0x6C ].routine = emul_float_math;          /* CVTBD */
    instruction[ 0x6D ].routine = emul_float_math;          /* CVTWD */
    instruction[ 0x6E ].routine = emul_float_math;          /* CVTLD */
    
    instruction[ 0x70 ].routine = emul_movd;                /* MOVD  */
    instruction[ 0x72 ].routine = emul_movd;                /* MNEGD */
    
    instruction[ 0x78 ].routine = emul_ash;                 /* ASHL  */
    instruction[ 0x79 ].routine = emul_ash;                 /* ASHQ  */
    instruction[ 0x7A ].routine = emul_emul;
    instruction[ 0x7B ].routine = emul_ediv;
    instruction[ 0x7C ].routine = emul_clrq;
    instruction[ 0x7D ].routine = emul_movq;
    instruction[ 0x7E ].routine = emul_mova;                /* MOVAQ */
    instruction[ 0x7F ].routine = emul_push;                /* PUSHAQ */

    instruction[ 0x80 ].routine = emul_integer_math;        /* ADDB2 */
    instruction[ 0x81 ].routine = emul_integer_math;        /* ADDB3 */
    instruction[ 0x82 ].routine = emul_integer_math;        /* SUBB2 */
    instruction[ 0x83 ].routine = emul_integer_math;        /* SUBB3 */
    instruction[ 0x84 ].routine = emul_integer_math;        /* MULB2 */
    instruction[ 0x85 ].routine = emul_integer_math;        /* MULB3 */
    instruction[ 0x86 ].routine = emul_integer_math;        /* DIVB3 */
    instruction[ 0x87 ].routine = emul_integer_math;        /* DIVB3 */
    instruction[ 0x88 ].routine = emul_integer_math;        /* BISB2 */
    instruction[ 0x89 ].routine = emul_integer_math;        /* BISB3 */
    instruction[ 0x8A ].routine = emul_integer_math;        /* BICB2 */
    instruction[ 0x8B ].routine = emul_integer_math;        /* BICB3 */
    instruction[ 0x8C ].routine = emul_xor;                 /* XORB2 */
    instruction[ 0x8D ].routine = emul_xor;                 /* XORB3 */
    instruction[ 0x8E ].routine = emul_movb_negated;        /* MNEGB */
    instruction[ 0x8F ].routine = emul_case;                /* CASEB */

    instruction[ 0x90 ].routine = emul_movb;                /* MOVB  */
    instruction[ 0x91 ].routine = emul_cmp;                 /* CMPB  */
    instruction[ 0x92 ].routine = emul_movb_negated;        /* MCOMB */
    instruction[ 0x93 ].routine = emul_cmp;                 /* BITB  */
    instruction[ 0x94 ].routine = emul_clrb;
    instruction[ 0x95 ].routine = emul_cmp;                 /* TSTB  */
    instruction[ 0x96 ].routine = emul_increment;           /* INCB  */
    instruction[ 0x97 ].routine = emul_increment;           /* DECB  */
    instruction[ 0x98 ].routine = emul_integer_cvt;         /* CVTBL */
    instruction[ 0x99 ].routine = emul_integer_cvt;         /* CVTBW */
    instruction[ 0x9A ].routine = emul_movzbl;
    instruction[ 0x9B ].routine = emul_movzbw;
    instruction[ 0x9C ].routine = emul_rotl;
    instruction[ 0x9D ].routine = emul_acb;                 /* ACBB  */
    instruction[ 0x9E ].routine = emul_mova;                /* MOVAB */
    instruction[ 0x9F ].routine = emul_push;                /* PUSHAB */

    instruction[ 0xA0 ].routine = emul_integer_math;        /* ADDW2 */
    instruction[ 0xA1 ].routine = emul_integer_math;        /* ADDW3 */
    instruction[ 0xA2 ].routine = emul_integer_math;        /* SUBW2 */
    instruction[ 0xA3 ].routine = emul_integer_math;        /* SUBW3 */
    instruction[ 0xA4 ].routine = emul_integer_math;        /* MULW2 */
    instruction[ 0xA5 ].routine = emul_integer_math;        /* MULW3 */
    instruction[ 0xA6 ].routine = emul_integer_math;        /* DIVW3 */
    instruction[ 0xA7 ].routine = emul_integer_math;        /* DIVW3 */
    instruction[ 0xA8 ].routine = emul_integer_math;        /* BISW2 */
    instruction[ 0xA9 ].routine = emul_integer_math;        /* BISW3 */
    instruction[ 0xAA ].routine = emul_integer_math;        /* BICW2 */
    instruction[ 0xAB ].routine = emul_integer_math;        /* BICW3 */
    instruction[ 0xAC ].routine = emul_xor;                 /* XORW2 */
    instruction[ 0xAD ].routine = emul_xor;                 /* XORW3 */
    instruction[ 0xAE ].routine = emul_movw_negated;        /* MNEGW */
    instruction[ 0xAF ].routine = emul_case;                /* CASEW */

    instruction[ 0xB0 ].routine = emul_movw;                /* MOVW  */
    instruction[ 0xB1 ].routine = emul_cmp;                 /* CMPW  */
    instruction[ 0xB2 ].routine = emul_movw_negated;        /* MCOMW */
    instruction[ 0xB3 ].routine = emul_cmp;                 /* BITW  */
    instruction[ 0xB4 ].routine = emul_clrw;
    instruction[ 0xB5 ].routine = emul_cmp;                 /* TSTW  */
    instruction[ 0xB6 ].routine = emul_increment;           /* INCW  */
    instruction[ 0xB7 ].routine = emul_increment;           /* DECW  */
    instruction[ 0xB8 ].routine = emul_bitpsw;              /* BISPSW */
    instruction[ 0xB9 ].routine = emul_bitpsw;              /* BISPSW */
    instruction[ 0xBA ].routine = emul_popr;
    instruction[ 0xBB ].routine = emul_pushr;
    instruction[ 0xBC ].routine = emul_chmx;                /* CHMK  */
    instruction[ 0xBD ].routine = emul_chmx;                /* CHME  */
    instruction[ 0xBE ].routine = emul_chmx;                /* CHMS  */
    instruction[ 0xBF ].routine = emul_chmx;                /* CHMU  */
    
    instruction[ 0xC0 ].routine = emul_integer_math;        /* ADDL2 */
    instruction[ 0xC1 ].routine = emul_integer_math;        /* ADDL3 */
    instruction[ 0xC2 ].routine = emul_integer_math;        /* SUBL2 */
    instruction[ 0xC3 ].routine = emul_integer_math;        /* SUBL3 */
    instruction[ 0xC4 ].routine = emul_integer_math;        /* MULL2 */
    instruction[ 0xC5 ].routine = emul_integer_math;        /* MULL3 */
    instruction[ 0xC6 ].routine = emul_integer_math;        /* DIVL3 */
    instruction[ 0xC7 ].routine = emul_integer_math;        /* DIVL3 */
    instruction[ 0xC8 ].routine = emul_integer_math;        /* BISL2 */
    instruction[ 0xC9 ].routine = emul_integer_math;        /* BISL3 */
    instruction[ 0xCA ].routine = emul_integer_math;        /* BICL2 */
    instruction[ 0xCB ].routine = emul_integer_math;        /* BICL3 */
    instruction[ 0xCC ].routine = emul_xor;                 /* XORL2 */
    instruction[ 0xCD ].routine = emul_xor;                 /* XORL3 */
    instruction[ 0xCE ].routine = emul_movl_negated;        /* MNEGL */
    instruction[ 0xCF ].routine = emul_case;                /* CASEL */

    instruction[ 0xD0 ].routine = emul_movl;                /* MOVL  */
    instruction[ 0xD1 ].routine = emul_cmpl;
    instruction[ 0xD2 ].routine = emul_movl_negated;        /* MCOML */
    instruction[ 0xD3 ].routine = emul_cmp;                 /* BITL  */
    instruction[ 0xD4 ].routine = emul_clrl;
    instruction[ 0xD5 ].routine = emul_cmp;                 /* TSTL  */
    instruction[ 0xD6 ].routine = emul_increment;           /* INCL  */
    instruction[ 0xD7 ].routine = emul_increment;           /* DECL  */
    instruction[ 0xD8 ].routine = emul_integer_math;        /* ADWC  */
    instruction[ 0xD9 ].routine = emul_integer_math;        /* SBWC  */
    instruction[ 0xDA ].routine = emul_mtpr;
    instruction[ 0xDB ].routine = emul_mfpr;
    instruction[ 0xDC ].routine = emul_movpsl;
    instruction[ 0xDD ].routine = emul_push;                /* PUSHL */
    instruction[ 0xDE ].routine = emul_mova;                /* MOVAL */
    instruction[ 0xDF ].routine = emul_push;                /* PUSHAL */

    instruction[ 0xE0 ].routine = emul_bb;                  /* BBS   */
    instruction[ 0xE1 ].routine = emul_bb;                  /* BBC   */
    instruction[ 0xE2 ].routine = emul_bbstate;             /* BBSS  */
    instruction[ 0xE3 ].routine = emul_bbstate;             /* BBCS  */
    instruction[ 0xE4 ].routine = emul_bbstate;             /* BBSC  */
    instruction[ 0xE5 ].routine = emul_bbstate;             /* BBCC  */
    instruction[ 0xE6 ].routine = emul_bbstate;             /* BBSSI */
    instruction[ 0xE7 ].routine = emul_bbstate;             /* BBCCI */
    instruction[ 0xE8 ].routine = emul_branch;              /* BLBS  */
    instruction[ 0xE9 ].routine = emul_branch;              /* BLBC  */
    instruction[ 0xEA ].routine = emul_ff;                  /* FFS   */
    instruction[ 0xEB ].routine = emul_ff;                  /* FFC   */
    instruction[ 0xEC ].routine = emul_cmpv;                /* CMPV  */
    instruction[ 0xED ].routine = emul_cmpv;                /* CMPZV */
    instruction[ 0xEE ].routine = emul_extv;                /* EXTV  */
    instruction[ 0xEF ].routine = emul_extv;                /* EXTZV */

    instruction[ 0xF0 ].routine = emul_insv;
    instruction[ 0xF1 ].routine = emul_acb;                 /* ACBW  */
    instruction[ 0xF2 ].routine = emul_aoblss;
    instruction[ 0xF3 ].routine = emul_aobleq;
    instruction[ 0xF4 ].routine = emul_sobgeq;
    instruction[ 0xF5 ].routine = emul_sobgtr;
    instruction[ 0xF6 ].routine = emul_integer_cvt;         /* CVTWL */
    instruction[ 0xF7 ].routine = emul_integer_cvt;         /* CVTWL */
    instruction[ 0xFA ].routine = emul_call;                /* CALLG */
    instruction[ 0xFB ].routine = emul_call;                /* CALLS */
    instruction[ 0xFC ].routine = emul_xfc;
    
    /* The following initializations are odd in that the index   */
    /* does not match the opcode.  These are after the first     */
    /* 256 instructions in the table, and have a two-byte opcode */

    instruction[ 0x11A ].routine = emul_bug;                /* BUGL  */
    instruction[ 0x11B ].routine = emul_bug;                /* BUGW  */

    return VAX_OK;
}


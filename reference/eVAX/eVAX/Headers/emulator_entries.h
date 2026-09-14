
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     emulator_entries.h
//
//  Purpose:    Entry point declarations for emulator routines... really
//              here for ease of editing and documentation.
//
//  History:    07/28/97    New header format standardization
//
//              Emulator entries are dated by when they were implemented;
//              this provides a (vague) history of the development of the
//              instruction set.  Entries by Tom Cole (TBC) except where
//              noted.  Contributions also from Carl Fongheiser, CMF.
//

#define EMULATOR_ENTRY( n ) LONGWORD n( struct OPCODE * opcode )



EMULATOR_ENTRY( emul_movl );            //  07/06/97
EMULATOR_ENTRY( emul_halt );            //  07/07/97
EMULATOR_ENTRY( emul_movw );            //  07/24/97
EMULATOR_ENTRY( emul_movb );            //  07/24/97
EMULATOR_ENTRY( emul_clrb );            //  07/24/97
EMULATOR_ENTRY( emul_clrw );            //  07/24/97
EMULATOR_ENTRY( emul_clrl );            //  07/24/97
EMULATOR_ENTRY( emul_nop  );            //  07/24/97
EMULATOR_ENTRY( emul_integer_math );    //  01/06/98
EMULATOR_ENTRY( emul_cmp );             //  01/08/98
EMULATOR_ENTRY( emul_branch );          //  01/08/98
EMULATOR_ENTRY( emul_interlock );       //  01/12/98
EMULATOR_ENTRY( emul_ash );             //  01/12/98
EMULATOR_ENTRY( emul_increment );       //  02/12/98
EMULATOR_ENTRY( emul_integer_cvt );     //  02/16/98
EMULATOR_ENTRY( emul_call );            //  02/19/98
EMULATOR_ENTRY( emul_ret );             //  02/19/98
EMULATOR_ENTRY( emul_push );            //  02/19/98
EMULATOR_ENTRY( emul_mova );            //  02/20/98
EMULATOR_ENTRY( emul_rotl );            //  02/20/98
EMULATOR_ENTRY( emul_emul );            //  02/20/98
EMULATOR_ENTRY( emul_ediv );            //  02/20/98
EMULATOR_ENTRY( emul_clrq );            //  02/20/98
EMULATOR_ENTRY( emul_movq );            //  02/20/98
EMULATOR_ENTRY( emul_mtpr );            //  02/21/98
EMULATOR_ENTRY( emul_mfpr );            //  02/21/98
EMULATOR_ENTRY( emul_aobleq );          //  02/24/98
EMULATOR_ENTRY( emul_aoblss );          //  02/24/98
EMULATOR_ENTRY( emul_sobgtr );          //  02/24/98
EMULATOR_ENTRY( emul_sobgeq );          //  02/24/98
EMULATOR_ENTRY( emul_bbstate );         //  02/24/98
EMULATOR_ENTRY( emul_bitpsw );          //  02/25/98
EMULATOR_ENTRY( emul_pushr );           //  02/25/98
EMULATOR_ENTRY( emul_popr );            //  02/25/98
EMULATOR_ENTRY( emul_cmpc3 );           //  03/19/98
EMULATOR_ENTRY( emul_cmpc5 );           //  03/20/98
EMULATOR_ENTRY( emul_locc );            //  03/20/98
EMULATOR_ENTRY( emul_movc3 );           //  03/21/98
EMULATOR_ENTRY( emul_movc5 );           //  03/23/98
EMULATOR_ENTRY( emul_xfc );             //  03/26/98
EMULATOR_ENTRY( emul_ldpctx );          //  08/20/98
EMULATOR_ENTRY( emul_svpctx );          //  08/20/98
EMULATOR_ENTRY( emul_float_math );      //  09/07/98
EMULATOR_ENTRY( emul_movf );            //  09/08/98
EMULATOR_ENTRY( emul_bpt );             //  09/24/98
EMULATOR_ENTRY( emul_case );            //  01/27/99
EMULATOR_ENTRY( emul_crc );             //  02/02/99
EMULATOR_ENTRY( emul_index );           //  02/02/99
EMULATOR_ENTRY( emul_rei );             //  02/03/99
EMULATOR_ENTRY( emul_chmx );            //  02/03/99
EMULATOR_ENTRY( emul_cmpl );            //  03/05/99
EMULATOR_ENTRY( emul_cmpv );            //  05/27/99
EMULATOR_ENTRY( emul_extv );            //  05/27/99
EMULATOR_ENTRY( emul_movpsl );          //  05/27/99
EMULATOR_ENTRY( emul_ff );              //  05/27/99
EMULATOR_ENTRY( emul_bug );             //  05/27/99
EMULATOR_ENTRY( emul_insv );            //  05/28/99
EMULATOR_ENTRY( emul_insqhi );          //  05/28/99
EMULATOR_ENTRY( emul_insqti );          //  05/28/99
EMULATOR_ENTRY( emul_insque );          //  05/28/99
EMULATOR_ENTRY( emul_remqhi );          //  05/28/99
EMULATOR_ENTRY( emul_remqti );          //  05/28/99
EMULATOR_ENTRY( emul_remque );          //  05/28/99
EMULATOR_ENTRY( emul_movzbl );          //  06/11/99
EMULATOR_ENTRY( emul_movzbw );          //  06/11/99
EMULATOR_ENTRY( emul_movzwl );          //  06/11/99
EMULATOR_ENTRY( emul_bb );              //  06/11/99
EMULATOR_ENTRY( emul_probe );           //  07/23/99
EMULATOR_ENTRY( emul_scanc );           //  07/23/99
EMULATOR_ENTRY( emul_movtc );           //  07/23/99
EMULATOR_ENTRY( emul_movtuc );          //  07/23/99
EMULATOR_ENTRY( emul_acb );             //  08/16/99
EMULATOR_ENTRY( emul_xor );             //  08/16/99
EMULATOR_ENTRY( emul_matchc);           //  08/23/99 by CMF
EMULATOR_ENTRY( emul_skpc );            //  08/27/99 by CMF
EMULATOR_ENTRY( emul_unimplemented );   //  01/27/00
EMULATOR_ENTRY( emul_movl_negated );    //  01/27/00
EMULATOR_ENTRY( emul_branch_always );   //  01/27/00
EMULATOR_ENTRY( emul_movb_negated );    //  01/28/00
EMULATOR_ENTRY( emul_movw_negated );    //  01/28/00
EMULATOR_ENTRY( emul_bneq );            //  01/29/00
EMULATOR_ENTRY( emul_beql);             //  01/29/00
EMULATOR_ENTRY( emul_bgtr );            //  01/29/00
EMULATOR_ENTRY( emul_bleq );            //  01/29/00
EMULATOR_ENTRY( emul_bgeq );            //  01/29/00
EMULATOR_ENTRY( emul_bgtru );           //  01/29/00
EMULATOR_ENTRY( emul_blss);             //  01/29/00
EMULATOR_ENTRY( emul_blequ );           //  01/29/00
EMULATOR_ENTRY( emul_bvc );             //  01/29/00
EMULATOR_ENTRY( emul_bvs );             //  01/29/00
EMULATOR_ENTRY( emul_bgequ );           //  01/29/00
EMULATOR_ENTRY( emul_bcs );             //  01/29/00
EMULATOR_ENTRY( emul_movd );            //  11/06/01




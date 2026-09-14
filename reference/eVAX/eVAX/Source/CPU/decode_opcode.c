//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     decode_opcode.c
//
//  Purpose:    This module handles decoding the instruction.  This involves validating
//              and identifying the opcode, and then arranging (via decode_operand.c) for
//              the decoding of the instruction operands.  
//
//              This is used both as a precursor to instruction execution, and as the
//              mechanism for disassembling an instruction (disassembly is controlled
//              by the "disasm" flag).
//
//  History:    07/28/97    New header format standardization
//
//              12/28/98    Add support for disassembling an entry point
//                          mask word.  We determine these from looking to see
//                          if the location we are decoding matches a SYM_ENTRY
//                          symbol entry.
//
//              05/10/99    Add support for system symbol table entries in
//                          disassembly.
//
//              08/03/99    Misc performance tweaks, improved speed by about 15%
//
//              09/26/99    Removed disassembly capability from decode_oprand 
//                          routine; it's handled now by disasm_operand.c


#define VMINLINE 1      /* Do we inline load_byte operations? */

#include "vax.h"
#include "vaxinstr.h"
#include "pte.h"

#include "asmproto.h"       // for get_symbol()


extern char * rom;  /* ROM memory */

//
//  Decode an instruction by addressing modes
//

LONGWORD decode_instruction( struct OPCODE * opcode, short disasm )
{
    struct SYMBOL * sp;
    short j, treg, opcount, found;
    unsigned char * pb;
    unsigned short mask;
    unsigned char f;

#if VMINLINE
    short size;
    ULONGWORD paddr;
#endif

    LONGWORD pc, dpc, rc, inst_index;
    char opname[ 10 ];
    char * scale;
    char label[ 255 ], label2[ 255 ];
    char * disasm_buff;    
    
	opcount = 0;
    vax.instruction_PC = pc = vax.PC;
    


    //  Set up the disassembly if we're doing that.  Check for the 
    //  possibility that this is an .ENTRY mask as a special case.

    if( disasm ) {

        sprintf( opcode-> name, "%08X: ", pc );

        vax.assembler.comment[ 0 ] = 0;

        for( sp = vax.console.symbols; sp; sp = sp-> next )
            if(( sp-> flags & SYM_ENTRY ) && sp-> value == pc )
                break;

        if( !sp )  /* Not in user table, try system */
        for( sp = system_symbols; sp; sp = sp-> next )
            if(( sp-> flags & SYM_ENTRY ) && sp-> value == pc )
                break;

        if( sp ) {
            rc = load_memory( pc, ( void * ) &mask, 2 );
            if( rc )
                return rc;

            strcat( opcode-> name, "              .ENTRY " );
            strcat( opcode-> name, sp-> name );
            strcat( opcode-> name, ", " );
            strcat( opcode-> name, format_mask( mask ));
  
            /* Just in case the user accidentally tries to STEP over an entry mask... */
            
            opcode-> extended = 0;
            opcode-> function = 01; /* NOP */
            opcode-> index = 01;
            opcode-> count = 0;
            
            vax.PC = pc + 2;
            return VAX_STEPENTRY;
        }

        /*  Not a .ENTRY but might be a label... */

        for( sp = vax.console.symbols; sp; sp = sp-> next ) 
            if(( sp-> flags & SYM_LABEL ) && sp-> value == pc )
                break;

        if( !sp )  /* Not in user table, try system table */
        for( sp = system_symbols; sp; sp = sp-> next ) 
            if(( sp-> flags & SYM_LABEL ) && sp-> value == pc )
                break;

        if( sp ) {
            sprintf( label, "%s:", sp-> name );
            sprintf( label2, "%-14s", label );
            strcat( opcode-> name, label2 );
        }
        else
            strcat( opcode-> name, "              " );
    }


    
    //  Get the function opcode.  If it's a two-byte function code, then
    //  save the extended function code bits and fetch the lsb of the opcode.
    //  Brute force inlining follows for performance reasons.
    
#if !VMINLINE
    rc = load_byte( pc, ( unsigned char * ) &f );
#else
    paddr = pc;
    if( vax.MAPEN ) {
        size = 1;
        rc = vm( &paddr, &size, VM_READ );
        if( rc )
            return rc;
    }

    pb = PHYADDR( paddr );
    if( pb == 0L )
        return set_fault( EXC_ACCVIO, 2, pc, 1 );
        
    f = *pb;
#endif

        
    pc++;
    
    //  If it's an extended opcode, we've got to get a second byte.  We also
    //  must do a table lookup to find the actual opcode information.
    
    if( f > 0x00FC ) {
        opcode-> extended = f;
        rc = load_byte( pc, ( unsigned char * ) &f );
        if( rc )
            return rc;
        pc++;
        opcode-> function = f;
        for( found = 0, inst_index = 256; instruction[ inst_index ].operand_count <= 6; inst_index++ ) {
            if( opcode-> extended == instruction[ inst_index ].extended && 
                f == instruction[ inst_index ].opcode ) {
                    opcount = instruction[ inst_index ].operand_count;
                    opcode-> index = ( short ) inst_index;
                    found = 1;
                    break;
            }
        }

        //  If we never found the instruction, then it's a privileged or
        //  reserved-to-digital fault.  It takes a count of zero arguments
        //  This call will redirect the exception to the fault handler, or
        //  will stop the processor if the exception vector is empty.

        if( !found )
            return set_fault( EXC_PRIV, 0 );
        
    }
    
    //  Or it's a single-byte instruction, and we can just index into the
    //  instruction table for the needed info.
    
    else {
        opcode-> extended = 0;
        opcode-> function = f;
     
        opcount = instruction[ f ].operand_count;
        inst_index = f;
        opcode-> index = f;
    }


    
    if( disasm ) {
        sprintf( opname, "%-8s ", instruction[ inst_index ].name );
        strcat( opcode-> name, opname );
    }
        
    opcode-> count = ( unsigned char ) opcount;
    vax.PC = pc;
        
    //  Record the profiling info.  Currently this just counts
    //  the number of times each instruction is used.  Note that
    //  since this code is used for disassembly as well as executing
    //  we only count if we are not halted.

    if( !vax.halted )
        instruction[ inst_index ].use_count++;

    //  If there are no operands, we're done!
    
    if( opcount == 0 ) {
        if( instruction[ inst_index ].debugdata & OP_DBG_BREAK )
            return VAX_BREAK;
        else
            return VAX_OK;
    }
        
    //  Nope, work to be done.  Set up the first available temp register
    //  which is used for literal handling, etc.
        
    treg = 16; 
    
    scale = instruction[ inst_index ].scale;
    disasm_buff = disasm ? opcode-> name : 0L;
    
    //  Let's scan over the operands and scoop them up.

    dpc = pc;
    
    if( !vax.halted )
        for( j = 0; j < opcount; j++ ) {
            
            opcode-> access[ j ] = instruction[ inst_index ].access[ j ];
            rc = decode_operand( opcode, j, &pc, &treg,
                      scale[ j ], 0 /* disasm_buff */ );
                
            if( rc ) {
                vax.PC = pc;
                return rc;
            }            

        }   // End of operand loop
        
        if( disasm_buff ) {
            
            for( j = 0; j < opcount; j++ ) {
                opcode-> access[ j ] = instruction[ inst_index ].access[ j ];
                rc = disasm_operand( opcode, j, &dpc, &treg, 
                            scale[ j ], 0, disasm_buff );
            }
            
            /* If we are disassembling but not running, use the disassembly */
            /* PC to update the stored PC.  This makes dissassembly of      */
            /* sequential memory locations work correctly.                  */
            
            if( vax.halted )
                pc = dpc;
        }
            
    vax.PC = pc;

    //  If we're disassembling, add the comment to the output.

    if( disasm && vax.assembler.comment[ 0 ] != 0 ) {

        pad( opcode-> name, 64 );
        strcat( opcode-> name, vax.assembler.comment );
        vax.assembler.comment[ 0 ] = 0;

    }

    //  If the debug break bit is set for this instruction, break now.
    
    if( instruction[ inst_index ].debugdata & OP_DBG_BREAK )
        return VAX_BREAK;

    //  All done with instruction, clear out.
    
    return VAX_OK;
}

/*
 *  Utility routine to pad a buffer with blanks to a specific length.
 */

void pad( char * buff, int len )
{

    int size, n;

    size = (int) strlen( buff );
    if( size > len ) {
       strcat( buff, " " );
       return;
    }

    for( n = size; n < len ;n++ ) 
        buff[ n ] = ' ';
    buff[ len ] = 0;
    return;
}



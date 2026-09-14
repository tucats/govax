//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_branch.c
//
//  Purpose:    Emulator handlers for relative branch instructions.
//
//
//  History:    07/28/97    New header format standardization
//
//              01/27/99    Added emul_case() handler
//
//              08/16/99    Added ACBx instructions
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//
//              01/27/00    Performance tweaks.  Made separate handler for branches
//                          that "branch-always" such as BRB, JMP, etc.  These have
//                          already had the decode work done so they just put the
//                          operand into the PC and return.
//
//              01/28/00    More of the same. Split out all the simple conditional
//                          branches.


#include "vax.h"
#include "fpu.h"

/*
 *  This handler covers ACBB, ACBW, ACBL, ACBF
 */

EMULATOR_ENTRY( emul_acb )
{
    char    limit_b, addend_b, index_b;
    short   limit_w, addend_w, index_w;
    LONGWORD    limit_l, addend_l, index_l;
    double  limit_f, addend_f, index_f;
    LONGWORD    branch, rc;
    
    branch = 0;
    
    switch( opcode-> function ) {
    
    case    0x9D:   /* ACBB */
    
            GET_OPERAND( limit_b, char, 0, OP_RD );
            GET_OPERAND( addend_b, char, 1, OP_RD );
            GET_OPERAND( index_b, char,  2, OP_RD );
            vax.pslw.v = OVERFLOW( index_b, addend_b, index_b + addend_b );

            index_b = index_b + addend_b;
            rc = put_operand( opcode, 2, OP_WR, &index_b );
            if( rc )
                return rc;
            
            vax.pslw.n = ( index_b < 0 );
            vax.pslw.z = ( index_b == 0 );

            if( addend_b < 0 && index_b >= limit_b )
                branch = 1;
            else
            if( addend_b >= 0 && index_b < limit_b )
                branch = 1;
                
            break;
            
    case    0x3D:   /* ACBW */
    
            GET_OPERAND( limit_w, short, 0, OP_RD );
            GET_OPERAND( addend_w, short, 1, OP_RD );
            GET_OPERAND( index_w, short,  2, OP_RD );

            vax.pslw.v = OVERFLOW( index_w, addend_w, index_w + addend_w );
            
            index_w = index_w + addend_w;
            rc = put_operand( opcode, 2, OP_WR, ( void * )&index_w );
            if( rc )
                return rc;
            
            vax.pslw.n = ( index_w < 0 );
            vax.pslw.z = ( index_w == 0 );

            if( addend_w < 0 && index_w >= limit_w )
                branch = 1;
            else
            if( addend_w >= 0 && index_w < limit_w )
                branch = 1;
                
            break;
            
    
    case    0xF1:   /* ACBL */
    
            GET_OPERAND( limit_l, LONGWORD, 0, OP_RD );
            GET_OPERAND( addend_l, LONGWORD, 1, OP_RD );
            GET_OPERAND( index_l, LONGWORD,  2, OP_RD );

            vax.pslw.v = OVERFLOW( index_l, addend_l, index_l + addend_l );
            
            index_l = index_l + addend_l;
            rc = put_operand( opcode, 2, OP_WR, ( void * ) &index_l );
            if( rc )
                return rc;
        
            vax.pslw.n = ( index_l < 0 );
            vax.pslw.z = ( index_l == 0 );
            
            if( addend_l < 0 && index_l >= limit_l )
                branch = 1;
            else
            if( addend_l >= 0 && index_l < limit_l )
                branch = 1;
                
            break;
            
    
    case    0x4F:   /* ACBF */
    
            GET_OPERAND( limit_l, LONGWORD, 0, OP_RD );
            GET_OPERAND( addend_l, LONGWORD, 1, OP_RD );
            GET_OPERAND( index_l, LONGWORD,  2, OP_RD );
            
            rc = fpu_load( limit_l, 0L, &limit_f );
            if( rc )
                return rc;
            
            rc = fpu_load( addend_l, 0L, &addend_f );
            if( rc )
                return rc;
            
            rc = fpu_load( index_l, 0L, &index_f );
            if( rc )
                return rc;
                
            index_f = index_f + addend_f;
            
            vax.pslw.n = ( index_f < 0.0 );
            vax.pslw.z = ( index_f == 0.0 );
            vax.pslw.v = 0; /* Need overflow detection here */

            rc = fpu_store( index_f, &index_l, 0L /* F_FLOAT */ );
            if( rc )
                return rc;
            rc = put_operand( opcode, 2, OP_WR, ( void * ) &index_l );
            if( rc )
                return rc;
                
            if( addend_f < 0.0 && index_f >= limit_f )
                branch = 1;
            else
            if( addend_f >= 0.0 && index_f < limit_f )
                branch = 1;
                
            break;
    }
    
    if( branch ) {
        vax.PC = opcode-> VAXaddr[ 3 ];
    }

    return VAX_OK;
    
}


/*
 *  This handler covers the CASEB, CASEW, and CASEL instructions.
 */

EMULATOR_ENTRY( emul_case )
{


    ULONGWORD selector =0L, base=0L, limit = 0L;
    ULONGWORD base_displacement, idx;
    LONGWORD rc;
    short branchword;

    LONGWORD  * p_long;
    short * p_word;
    char  * p_byte;

//  Based on the operand sizes, fetch the three primary operands.

    switch( opcode-> function ) {

    case 0x8F:   /* CASEB */

        p_byte = ( char * ) get_operand( opcode, 0, OP_RD );
        if( p_byte == 0L )
            return VAX_FAULT;
        selector = ( LONGWORD ) ( *p_byte );
 
        p_byte = ( char * ) get_operand( opcode, 1, OP_RD );
        if( p_byte == 0L )
            return VAX_FAULT;
        base = ( LONGWORD ) ( *p_byte );
 
        p_byte = ( char * ) get_operand( opcode, 2, OP_RD );
        if( p_byte == 0L )
            return VAX_FAULT;
        limit = ( LONGWORD ) ( *p_byte );
 
        break;


    case 0xAF:   /* CASEW */

        p_word = ( short * ) get_operand( opcode, 0, OP_RD );
        if( p_word == 0L )
            return VAX_FAULT;
        selector = ( LONGWORD ) ( *p_word );
 
        p_word = ( short * ) get_operand( opcode, 1, OP_RD );
        if( p_word == 0L )
            return VAX_FAULT;
        base = ( LONGWORD ) ( *p_word );
 
        p_word = ( short * ) get_operand( opcode, 2, OP_RD );
        if( p_word == 0L )
            return VAX_FAULT;
        limit = ( LONGWORD ) ( *p_word );
 
        break;


    case 0xCF:   /* CASEL */

        p_long = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
        if( p_long == 0L )
            return VAX_FAULT;
        selector = ( LONGWORD ) ( *p_long );
 
        p_long = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
        if( p_long == 0L )
            return VAX_FAULT;
        base = ( LONGWORD ) ( *p_long );
 
        p_long = ( LONGWORD * ) get_operand( opcode, 2, OP_RD );
        if( p_long == 0L )
            return VAX_FAULT;
        limit = ( LONGWORD ) ( *p_long );
 
        break;

    }

//  At this point, we have the selector, base, and limit values.  The
//  displacement table follows the instruction, so let's find the
//  base displacement location.  

    base_displacement = vax.PC;

//  Subtract the selector from the base.  This gives us a zero-based
//  index into the list.  Also, this instruction always clears the
//  overflow bit in the PSL.

    idx = selector - base;
    vax.pslw.v = 0;
    vax.pslw.n = ( ( LONGWORD ) idx < ( LONGWORD ) limit );

//  If the idx is greater than the limit, then we just stride past the
//  table and give it up.

    if( idx > limit ) {

        vax.pslw.z = 0;
        vax.pslw.c = 0;

        vax.PC = base_displacement + 2 + ( limit * 2 );
        return VAX_OK;
    }

//  Otherwise, let's set the PSL bits to reflect the comparison

    vax.pslw.z = ( idx == limit );
    vax.pslw.c = ( idx < limit );

//  Calculate the VAX address of the displacement word we want.  Read
//  the displacement and add to the PC.  Note that the base of the
//  displacement starts at the first byte of the table; i.e. the PC
//  is already advanced past the limit operand when we do this read.

    idx = ( idx * 2 ) + base_displacement;

    rc = load_memory( idx, ( void * ) &branchword, 2 );
    if( rc )
        return rc;

    vax.PC += branchword;
    return VAX_OK;
}


/*
 *  This entry point handles instructions that branch-always, and the
 *  destination has already been decoded.  For example, BRB, BRW, and
 *  JMP all simply branch, so they don't need to do the extra CASE
 *  decode that the generic branch handler does below.  
 */

EMULATOR_ENTRY( emul_branch_always )
{

    vax.PC = opcode-> VAXaddr[ 0 ];
    return VAX_OK;
}

/*
 * This set of handlers does the work of simple conditional branches, one
 * for each instruction.
 */

#define BRANCH_HANDLER( name, test ) \
EMULATOR_ENTRY( name ) { if( test ) vax.PC = opcode-> VAXaddr[0]; return VAX_OK; }

BRANCH_HANDLER( emul_bneq, (!vax.pslw.z))
BRANCH_HANDLER( emul_beql, (vax.pslw.z ))
BRANCH_HANDLER( emul_bgtr, (!( vax.pslw.n | vax.pslw.z )))
BRANCH_HANDLER( emul_bleq, ( vax.pslw.n | vax.pslw.z ))
BRANCH_HANDLER( emul_bgeq, ( !vax.pslw.n ))
BRANCH_HANDLER( emul_blss, ( vax.pslw.n ))
BRANCH_HANDLER( emul_bgtru, (!( vax.pslw.c | vax.pslw.z )))
BRANCH_HANDLER( emul_blequ, ( vax.pslw.c | vax.pslw.z ))
BRANCH_HANDLER( emul_bvc, ( !vax.pslw.v ))
BRANCH_HANDLER( emul_bvs, ( vax.pslw.v ))
BRANCH_HANDLER( emul_bgequ, (! vax.pslw.c ))
BRANCH_HANDLER( emul_bcs, ( vax.pslw.c ))



/*
 *  This entry point handles the generic branch instruction families.
 */
        
        
EMULATOR_ENTRY( emul_branch )
{


    LONGWORD new_pc, rc;
    LONGWORD * src, data;
    
    //  The operand data already contains the desired address to branch to
    //  should we decide to use it.
    
    new_pc = opcode-> VAXaddr[ 0 ];
    rc = VAX_OK;
    
    switch( opcode-> function ) {
    
    case 0x05:  /* RSB */
    
            /* Read the longword at the stack top into the PC (register 15) */
            
            load_register( 15, vax.SP, 4 );
            
            /* Now, update the SP accordingly */
            
            vax.SP = vax.SP + 4;

            break;
            
    case 0x10:  /* BSBW */
    
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &vax.PC, 4 );
            if( !rc )
                vax.PC = new_pc;
            break;
            
                
    case 0x16:  /* JSB */
    
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &(vax.PC), 4 );
            if( rc )
                break;
                
            vax.PC = new_pc;
            break;
                        


    case 0x30:  /* BSBW */
    
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &vax.PC, 4 );
            if( !rc )
                vax.PC = new_pc;
            break;
                
    case 0xE8:  /* BLBS */
    case 0xE9:  /* BLBC */
    
            /* Get the address of the longword we are going to compare */
            
            src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
            if( src == 0L )
                return VAX_FAULT;
            
            /* Read the longword and mask down to the lsb */
            
            data = (*src) & 0x00000001;
            
            /* Depending on which opcode we're doing, test for one or zero */
                        
            if( data == ( opcode-> function == 0xE8 ) ? 1 : 0 )
                vax.PC = opcode-> VAXaddr[ 1 ];
                
            break;
            
    default:
    
        return set_fault( EXC_PRIV, 0 );
    
    }
    
    return rc;
}



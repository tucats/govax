//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_call.c
//
//  Purpose:    Emulator handlers for CALLS, CALLG, and RET instructions
//              Also handler for REI instruction.
//
//
//  History:    07/28/97    New header format standardization
//
//              02/03/99    Added REI instruction
//
//              06/09/99    Make IPL privileged register track PSL<IPL>
//
//              06/10/99    Make REI handle AST's and software interrupts
//
//		07/26/01    Correct errors in handling of CALLG first operand

#include "vax.h"

EMULATOR_ENTRY( emul_call )
{
    LONGWORD * op1, rc;
    LONGWORD calltype, count;
    LONGWORD newPC, mask, n, new_ap;
    LONGWORD savedSP;
    
    union MASKREG mask_union;
    
            
    //  Get which opcode we are dealing with, CALLS or CALLG
    
    calltype = opcode-> function;
    
     
    //  Get address of first operand (which may be literal)
    
    op1 = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( op1 == 0L )
        return VAX_FAULT;

    
    //  Get address of destination.  Must be readable or fault!
    
    newPC = opcode-> VAXaddr[ 1 ];

    //  If this is a CALLS then the op1 argument is a count
    //  of the number of arguments, push it on the stack so 
    //  it sits just below the argument list itself.
    
    if( calltype == 0xFB ) {
            count = *op1;
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &(count), 4 );
            if( rc )
                 return rc;
                 
            new_ap = vax.SP;
    }
    
    //  Otherwise, the op1 argument is the address of the arguments.
    
    else {
        if( opcode-> is_register[0] )
            new_ap = vax.reg[ opcode-> regnum[ 0 ]];
        else
            new_ap = opcode-> VAXaddr[ 0 ];
    }
    
    
    //  Save the SP value, we'll need it later.  Now, coerce the
    //  stack to be longword aligned by zeroing the low order 2 bits.
    
    savedSP = vax.SP;
    vax.SP &= 0xFFFFFFFC;
    
    //  Get the procedure entry mask, located at the destination address.
    
    rc = load_register( ( short ) vax.treg, newPC, 2 );
    if( rc )
        return rc;
        
    newPC += 2;
    mask = vax.reg[ vax.treg ];
    
    //  Loop over the mask in reverse order, and push the R11-R0 registers
    //  on the stack as requested by the mask.
    
    for( n = 11; n >= 0; n-- ) {
    
        if( mask & ( 1 << n )) {
        
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &(vax.reg[ n ]), 4 );
            if( rc )
                return rc;
        }
    }
    
    //  Push the PC, FP, and AP registers
    
    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &( vax.PC), 4 );
    if( rc )
        return rc;
        
    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &( vax.FP), 4 );
    if( rc )
        return rc;

    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &( vax.AP), 4 );
    if( rc )
        return rc;

    
    //  Push the mask, PSW, etc. bits.
    
    write_psl_bits();  /* Make sure bits match cached copy */
    
    mask_union.bits.spa = savedSP & 0x00000003;
    mask_union.bits.calltype = ( calltype == 0xFB );
    mask_union.bits.mbz = 0;
    mask_union.bits.mask = mask & 0x0FFF;
    mask_union.bits.psw = vax.psl.reg & 0x0000FFC0;  /* Bits 5:15 */
    
    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &(mask_union.longword), 4 );
    if( rc )
        return rc;

    //  Push an extra zero to hold the condition handler (if any)
    
    mask_union.longword = 0L;
    vax.SP -= 4;
    rc =store_memory( vax.SP, ( void * ) &(mask_union.longword), 4 );
    if( rc )
        return rc;

    //  Now, set up the registers to the new state.
    
    vax.FP = vax.SP;
    vax.AP = new_ap;
    vax.PC = newPC;
    
    return VAX_OK;
}




//  Return from subroutine, processing frame on the stack.

EMULATOR_ENTRY( emul_ret )
{

    LONGWORD mask, n, count, rc;
    
    union MASKREG mask_union;


//  First, put the SP back to the FP+4, so we discard any automatic storage, and
//  the condition handler longword.

    vax.SP = vax.FP + 4;

//  Fetch the mask word that tell us how to interpret the rest of the frame.

    rc = load_memory( vax.SP, ( unsigned char * ) &( mask_union.longword ), 4 );
    if( rc )
        return rc;

    vax.SP += 4;

//  Fetch the PC, FP, and AP registers

    rc = load_memory( vax.SP, ( unsigned char * ) &( vax.AP ), 4 );
    if( rc )
        return rc;

    vax.SP += 4;
        
    rc = load_memory( vax.SP, ( unsigned char * ) &( vax.FP ), 4 );
    if( rc )
        return rc;

    vax.SP += 4;
        
    rc = load_memory( vax.SP, ( unsigned char * ) &( vax.PC ), 4 );
    if( rc )
        return rc;

    vax.SP += 4;

//  Scan the register storage mask, and pop register values.

    mask = mask_union.bits.mask;
    for( n = 0; n <= 11; n++ ) {
    
        if( mask & ( 1 << n )) {
            rc = load_memory( vax.SP, ( unsigned char * ) &( vax.reg[ n ]), 4 );
            if( rc )
                return rc;

            vax.SP += 4;
        }
    }

//  The stack might have been unaligned before the call, the spa bits
//  contain the low 3 bits of the SP at the time of the call... set 'em back.

    vax.SP += mask_union.bits.spa;
    

//  If it was CALLS then use the arg count to toss away the argument list.  We
//  Must account for the actual count itself.

    if( mask_union.bits.calltype ) {
        rc = load_memory( vax.SP, ( unsigned char * ) &( count ), 4 );
        if( rc )
            return rc;
            
        vax.SP += 4 + ( count * 4 );
    }

    
//  Retrieve the PSW bits that were saved away.  Don't allow bogus bits
//  from the frame to overwrite illegal bits in the PSW.

    write_psl_bits();

    vax.psl.reg &= 0xFFFF003F;    /* We're gonna replace 5:15 */
    vax.psl.reg |= ( mask_union.bits.psw & 0x0000FFC0) ;  /* Bits 5:15 */

    read_psl_bits();  /* Make sure cached copy matches bits */


//  If we are RETurning from a console CALL command as opposed to a CALLx
//  instruction, we must halt the processor so we don't branch into la-la
//  land (no valid PC to return to).  
//
//  Note that we do this if CALL was active and the PC and FP pushed on 
//  the stack contain the magic value of FFFFDEAF.  This is never a valid 
//  PC or FP value, so we should hopefully never hit it in a real execution.
//  This isn't perfect, but I need some way to mark the call frame for this 
//  special case, and still have it be a valid call frame.

    if( vax.console.CALL_active 
           &&  vax.PC == 0xFFFFDEAF 
           &&  vax.FP == 0xFFFFDEAF ) {

        vax.halted = 1;
        vax.PC = vax.console.CALL_PC;
        vax.FP = vax.console.CALL_FP;

        vax.console.CALL_active = 0;
        
    }

//  Our work here is done.

    return VAX_OK;
    
}




//  Return from exception or interrupt.  Restores the mode of the machine
//  as appropriate, and also signals AST's if any are pending.

EMULATOR_ENTRY( emul_rei )
{
    char * mode_name[] = { "KERNEL", "EXEC", "SUPER", "USER" };
    
    LONGWORD rc, new_PC, new_PSL;
    LONGWORD n, value;
    LONGWORD old_mode;

//  Make the cached PSL match up.

    write_psl_bits();
    old_mode = vax.pslw.cur_mod;


//  Fetch the saved PC

    rc = load_memory( vax.SP, ( unsigned char * ) &new_PC, 4 );
    if ( rc )
       return rc;
    vax.SP += 4;


//  Fetch the saved PSL

    rc = load_memory( vax.SP, ( unsigned char * ) &new_PSL, 4 );
    if ( rc )
       return rc;
    vax.SP += 4;


//  Save the stack pointer in the current mode's storage area

    if( vax.pslw.is )
        vax.ISP = vax.SP;
    else    
        vax.preg[ vax.pslw.cur_mod ] = vax.SP;


//  Update the PSL.  The handler MUST have made sure it's correct.

    vax.psl.reg = new_PSL;
    read_psl_bits();
    vax.IPL = vax.pslw.ipl;

    if( vax.debug & DBG_CHM ) {
        if( old_mode != vax.pslw.cur_mod )
            printf( "DEBUG(CHM): CHANGE MODE FROM %s TO %s AT %08X\n",
                mode_name[ old_mode ], 
                mode_name[ vax.pslw.cur_mod ], vax.instruction_PC );
    }


//  Load the stack pointer with the correct new mode's stack.  Set the PC.

    vax.SP = vax.preg[ vax.pslw.cur_mod ];
    vax.PC = new_PC;


//  At this point we should check to see if ASTLVL tells us there are 
//  pending AST's.  We don't do this if on the interrupt stack, though
//  since AST's are handled in process space typically.  ASTLVL is used
//  to prevent AST delivery to a process not in a suitable mode; i.e.
//  don't deliver USER mode AST's when we are in KERNEL mode, etc.

    if( ! vax.pslw.is ) {
        if( vax.pslw.cur_mod >= vax.ASTLVL ) {
            interrupt( 0x00000088, 02, 0 );
            return VAX_OK;
        }
    }


//  Also, check for other pending software interrupts in the SISR that
//  are now eligible to run because IPL (may have) lowered.

    value = vax.IPL;

    for( n = 15; n > value; n-- ) {
        if( vax.SISR & ( 1 << n )) {
            vax.SISR &= ~( 1 << n );
            interrupt( ( 0x000000080 + ( n << 2 )), n, 0 );
        }
    }


    return VAX_OK;
}


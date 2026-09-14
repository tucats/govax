

//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_exec.c
//
//  Purpose:    This console command starts execution of instructions in
//              the virtual VAX processor.  This is with the GO command that
//              simply starts execution, or the CALL command that calls code
//              that expects to execute a RET instruction.
//
//  History:    07/28/97    New header format standardization
//
//              09/24/98    Added the CALL command
//
//              12/11/98    Fixed bug where CALL caused fault because the
//                          return pop'd a bogus PC.  We now save extra
//                          state info in the console to see if this is the
//                          RET from the console, and halt accordingly.
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

/*
 *  GO or EXEC starting at a particular instruction.
 */

LONGWORD console_exec( char ** P )
{
    LONGWORD rc;
    char * p;
    ULONGWORD value;

/*
 *  Get local addressability to virtual VAX and command buffer
 */
 
    /* vax = *Vax;  -- now use global vax */
    p = *P;

/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }
    
/*
 *  Check to see if there are unresolved forward references in the pseudoassembler
 *  element of the console.  If so, then don't allow us to run yet!
 */

    rc = check_unresolved_symbols( 1 );    /* Flag 1 means print the list */
    
    if( rc != VAX_OK )
        return rc;

/*
 *  See if there is a starting address as part of the command.
 */
 
    flush_blanks( &p );
    
    if( !isend( *p )) {
        rc = asm_expr( &p, ( LONGWORD * ) &value );
        if( rc )
            return rc;;
        vax.PC = value;
    }

    rc = execute_vax( ( short ) vax.console.disasm );

/*
 *  If an unhandled fault occurred, we need to reset back to user
 *  mode.  The console user will have to figure out what they want
 *  to do about the arguments, etc. left on the stack.
 */

    if( rc == VAX_UNHANDLED ) {
        set_mode_stack( 3 );
    }

    if( rc == VAX_USERHALT )
	rc = VAX_OK;

/*
 *  Store parameters back to caller.
 */
 

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return rc;
}




/*
 *  CALL the code stream, by creating a call frame on the stack based on the
 *  mask at the designated address.  The code branched to should end with a
 *  RET instruction.
 */

LONGWORD console_call( char ** P )
{
    LONGWORD rc;
    char * p;
    
    ULONGWORD value;

    LONGWORD count, parsing;
    LONGWORD newPC, mask, n, new_ap;
    LONGWORD savedSP;
    LONGWORD step, verb, saved_halt;
    
    LONGWORD argv[ 18 ];    /* Store optional arguments here while parsing (left-to-right) */
    
    union MASKREG {
    
        LONGWORD        longword;
        struct {
            int     spa: 2;
            unsigned int calltype: 1;
            int     mbz: 1;
            int     mask: 12;
            int     psw: 16;
        } bits;
    } mask_union;
    
            
/*
 *  Get local addressability to virtual VAX and command buffer
 */
 
    /* vax = *Vax;  -- now use global vax */
    p = *P;


/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }
    
/*
 *  Check to see if there are unresolved forward references in the pseudoassembler
 *  element of the console.  If so, then don't allow us to run yet!
 */

    rc = check_unresolved_symbols( 1 );    /* Flag 1 means print the list */
    
    if( rc != VAX_OK )
        return rc;

/*
 *  See if the /STEP qualifier (or it's synonym, /BREAK) was used.
 */
 
    flush_blanks( &p );
    step = 0;
    if( *p == '/' ) {
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        if( verb == CHAR4('/','S','T','E') ||
            verb == CHAR4( '/','B','R','E')||
            verb == CHAR4( '/','D','E','B')||
            verb == CHAR4( '/','D','B','G')) {
            step = 1;
        }
        else
            p = *P;
    }

/*
 *  There must be a starting address as part of the command.
 */
 
    flush_blanks( &p );
    
    if( !isend( *p )) {
        rc = asm_expr( &p, ( LONGWORD * ) &value );
        if( rc )
            return rc;;
        newPC = value;
    }
    else
        return VAX_EXPENTRY;

/*
 *  The user might include arguments (my gosh).  Parse 'em
 */

    flush_blanks( &p );
    count = -1L;
    
    if( *p == '(' ) {
    
        p++;
        count = 0;
        parsing = 1;
        while( parsing ) {
        
            /* If EOS, it's an incomplete argument list */
            
            flush_blanks( &p );
            if( isend( *p )) 
                return VAX_INVARGLIST;

            /* If closing parens, we're done */
            
            if( *p == ')' ) {
                p++;
                parsing = 0;
                continue;
            }
            
            /* If 2nd or later argument, must be comma delim */
            
            if( count > 0 ) {
                if( *p != ',' )
                    return VAX_INVARGLIST;
                p++;
                flush_blanks( &p );
            }
            
            /*  See if we've just got to many arguments */
            
            if( count >= 15 )
                return VAX_INVARGLIST;

            /* Must be numeric, get it */                
            rc = asm_expr( &p, ( LONGWORD * ) &value );
            argv[ count ] = value;
            count++;
        }
    
    }

    //  If we had arguments, count is >= 0.  If so we've got to push them
    //  on the stack.
    
    if( count >= 0 ) {
    
        for( n = count ; n >= 0; n-- ) {
            vax.SP -= 4;
            rc = store_memory( vax.SP, ( void * ) &(argv[ n ]), 4 );
            if( rc )
                return rc;
                
        }
        
        /* count = count + 1;*/  /* Should be 1-based, not zero-based */
        
    }
    else
        count = 0L;         /* No arguments, 1 based */
        
        
    //  Assume this is like a CALLS and the arguments are on the stack.  Push
    //  the count on the stack (after optional arguments pushed R->L )
    
    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &(count), 4 );
    if( rc )
        return rc;
        
    new_ap = vax.SP;

    
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

    //  We are going to mark this frame specially, to detect when a RET
    //  should return to the console rather than executing.  So save the
    //  real FP and PC values, and the set up fake ones.

    vax.console.CALL_PC = vax.PC;
    vax.console.CALL_FP = vax.FP;
    vax.console.CALL_active = 1;

    //  Magic value (pronounced "if-def")

    vax.PC = vax.FP = 0xFFFFDEAF;


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
    mask_union.bits.calltype = 0x01;   /* CALLS */
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
    rc = store_memory( vax.SP, ( void * ) &(mask_union.longword), 4 );
        if( rc )
                return rc;

    //  Now, set up the registers to the new state.
    
    vax.FP = vax.SP;
    vax.AP = new_ap;
    vax.PC = newPC;

/*
 *  Special case.  If the caller wants to CALL/STEP then we don't run the
 *  code, we just do a single step to the first location.
 */
 
    if( step ) {
        *P = p;
        return console_step( P );
    }
    
        
/*
 *  Execute code starting at the requested location.
 */

    saved_halt = vax.halted;
    rc = execute_vax( ( short ) vax.console.disasm );

/*
 *  If everything went okay, then we extract the function return
 *  value.  
 */

    if( rc == VAX_USERHALT || rc == VAX_HALT ) {
        rc = VAX_OK;
        vax.halted = rc;
        set_symbol_direct( "$RC", ( LONGWORD ) vax.R0, 0 );
    }
    else
        vax.halted = saved_halt;

/*
 *  If an unhandled fault occurred, we need to reset back to user
 *  mode.  The console user will have to figure out what they want
 *  to do about the arguments, etc. left on the stack.
 */

    if( rc == VAX_UNHANDLED ) {
        set_mode_stack( 3 );
    }

/*
 *  Store console command parameters back to caller and we're done.
 */
 

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return rc;
}

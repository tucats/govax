
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     step.c
//
//  Purpose:    This module implements the STEP command, which single steps
//              the processor starting at a specific address, or the current
//              PC if no address is given.
//
//  History:    06/03/99    New header format standardization
//
//              06/03/99    Added ability to show what registers changed on
//                          a step boundary.  This only works for STEP command,
//                          not disassembly mode.  On by default, this feature
//                          is controlled by SET DEBUG [NO]REGISTERS
//
//              01/13/00    Fixed remaining bugs with STEP/OVER and how it
//                          formats its output compared to STEP/INSTR.  Matches
//                          fixes in vax.c
//
//		10/22/01    Separated out register tracking functionality.



#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "regtrack.h"


LONGWORD console_step( char ** P )
{
    LONGWORD step_mode, rc, verb;
    char * p;
    ULONGWORD value;
    struct REGSET rs;

/*
 *  Get local addressability to virtual VAX.  
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
 *  See if we have the /OVER, /INTO, or /RETURN flags.  If not use the default.
 */

    flush_blanks( &p );
    step_mode = vax.console.stepmode; 
    if( !isend( *p )) {
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        
        if( verb == CHAR4( '/','I','N',' ') ||      /*  STEP/IN             */
            verb == CHAR4( '/','I','N','S') ||      /*  STEP/INSTRUCTION    */
            verb == CHAR4( '/','I','N','T')) {      /*  STEP/INTO           */
            step_mode = STEP_INSTRUCTION;
        }
        else
        if( verb == CHAR4( '/','O','V','E')) {      /*  STEP/OVER           */
            step_mode = STEP_OVER;
        }
        else
        if( verb == CHAR4( '/','R','E','T')) {      /*  STEP/RETURN         */
            step_mode = STEP_RETURN;
        }
        else {
            p = *P; /* Reset parse pointer, use default mode */
        }
    }
    
/*
 *  We basically do a normal execute but with the special console single step flag
 */
 
    vax.console.singlestep = step_mode;

    flush_blanks( &p );
    
    if( !isend( *p )) {
        rc = asm_hex( &p, &value );
        if( rc )
            return rc;;
        vax.PC = value;
    }

/*
 *  Capture the register state before the step.
 */

    if( vax.debug & DBG_REGISTERS ) {
        save_regset( &rs );
    }

/*
 *  Okay, there are two ways to go here.
 */
 
    if( step_mode == STEP_INSTRUCTION ) {
        rc = execute_vax( 1 );  /* Always in trace mode */  
        format_instruction( "Stepped to", vax.PC );
    }
    
    else {
    
/*
 *  It's an /OVER or /RETURN case.  This is a little harder.  We enable "special" single 
 *  step mode, which says capture the PC address *after* decoding and create a break there,
 *  or monitor the opcodes as apprpriate for the mode.  This is all really handled in the 
 *  vax.c module.
 */
        rc = execute_vax( ( short ) vax.console.disasm );

        /* If it was /OVER and we did NOT hit a call/jsb, then we need to */
        /* format the next instruction so it looks like STEP/INSTR did.   */

        if( rc != VAX_BREAK ) {
            format_instruction( "Stepped to", vax.PC );
        }
    }
    

/*
 *  What registers changed?
 */

    if( vax.debug & DBG_REGISTERS ) {
        check_regset( &rs );
     }


    
/*
 *  Store parameters back to caller.
 */
 

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;

    if( rc == VAX_BREAK )
        rc = VAX_OK;

    return rc;
}


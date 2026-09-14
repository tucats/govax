//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     vax.c
//
//  Purpose:    Main VAX execution controller module.  This routine handles the
//              execution of sequential VAX instructions starting at either a
//	        numeric address or at a symbol.  This routine also handles 
//		support for the t-bit to identify when a trace fault should
//		occur.
//
//  History:    07/28/97    New header format standardization
//
//              05/07/99    Add the current SP (and implicitly mode) info
//                          to the trace output.
//
//              06/14/99    Added ability to set breakpoints on FAULTS as well
//                          as instruction addresses.
//
//              09/06/99    Added keyboard polling mechanisms on some hosts to
//			    start experimenting with interrupt-driven keyboard
//                          input.
//
//              10/12/99    Added extra control for STEP/OVER and STEP/RETURN.
//
//              11/04/99    Fixed dumb bug in STEP handling for /RETURN.
//
//              11/08/99    Cleaned up STEP handling somewhat.  Implemented
//			    "temporary"  breakpoints which are one-shot breaks.
//			    STEP/OVER and STEP/RETURN now create one-shot 
//			    breakpoints. This eliminates the expensive
//                          checking of PC against step_pc when we already look
//			    for breakpoints as needed.  
//
//              01/13/00    Finished cleanup of STEP, I think.  Formatting of
//			    output for STEP/OVER is the same as STEP/INSTR, etc. 
//			    Also STEP/OVER now is same as STEP/INSTR when the
//			    current opcode is *not* a procedure or subroutine
//			    call.  Matches fixes in console_step.c
//
//              01/27/00    Performance tweaks.  Removed extra longword compare
//			    when the breakpoint list is empty (typical case). 
//		            Remove unneeded "min quantum" calculation.  Moved
//			    STEPENTRY test to inside fault handling logic so 
//			    normal cases does one less compare. Remove test for
//			    unimplemented instruction, and instead init
//                          all handlers to a special "unimplemented" handler
//			    that faults. Make all post-emulation handler 
//			    checking conditional on a single test of rc rather
//			    than two tests.
//
//              02/10/00    Enabled keyboard detection for Windows.  Still need 
//			    Unix and VMS versions of this, of course.
//

#if macintosh
#include <SIOUX.h>

#include <console.h>
#include <signal.h>

#endif

#define GLOBAL

#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "memmap.h"         // to instantiate memory map list headers (uses GLOBAL)
#include "regtrack.h"


int kbdhit( void );
int getkbd( void );

LONGWORD format_operands(struct OPCODE * op);

//  Begin execution of instructions at a particular address denoted by
//  a symbolic value.

LONGWORD execute_symbol( char * symbol, short disasm )
{

    LONGWORD    pc, rc;
    char * cp;
    
    cp = symbol;
    rc = get_symbol( &cp, &pc );
    if( rc )
        return rc;
    
    vax.PC = pc;
    return execute_vax( disasm );
}



//
//  Begin execution of instructions

LONGWORD execute_vax( short disasm )
{

    struct OPCODE opcode;
    LONGWORD rc, saved_pc, step_pc;
    LONGWORD (*routine)(struct OPCODE * opcode ); 
    struct BREAKSTR * brkp;
    ULONGWORD initial_PC;
    static char * mode_name[] = { "KSP", "ESP", "SSP", "USP" };
    char * modep;
    struct INTERRUPT * ip, *ipnext, *iplast;
    LONGWORD iquantum;
    struct REGSET rs;
    
    vax.halted = 0;
    initial_PC = vax.PC;
    vax.clock_running = vax.ICCS & 1; /* ICCS<0> */
    
    while( !vax.halted ) {

    //	Diagnostic tool.  The privileged register IPL and the IPL field of
    //	the PSL should match before starting the next instruction, always.
    //	If not, then something has gone haywire.

#ifdef DEBUG
        if( vax.IPL != vax.pslw.ipl ) {
            printf( "IPL synch error!\n" );
            vax.halted = 1;
            break;
        }
#endif

        //  If we have a pending fault from a FAULT BREAKPOINT, trigger it now.
        
        if( vax.fault_pending ) {
            rc = handle_fault( );
            if( rc ) {
                vax.clock_running = 1;	/* Not in console mode */
                return rc;
            }
        }
        
        //  Do "quantum" things.  This is work done infrequently, and we want to limit
        //  the time spend evaluating these things for fastest instruction throughput.  As
        //  such, we have notion of a quantum unit.  This is the number of instructions
        //  that will run without interrupt or flow change.  We set a quantum counter, and
        //  decrement to zero.  If the counter becomes zero, then we evaluate lots of more
        //  expensive things,
        //
        //  1.  Updating the quantum counter to it's initial value again.
        //  2.  Update the TOY clock.
        //  3.  Decrementing the UI quantum counter (a multiple of quanta) for UI intervention.
        //  4.  Evaluate the interrupt queue.
        //
        
        /* vax.quantum.current--; */  /* Let the ms timer handle this? */
        
        if( vax.quantum.current <= 0 ) {
        
            vax.quantum.current = vax.quantum.initial;
            
        //  Update the clock if it's enabled.

            if( vax.clock_running ) {

                if( vax.clock == 0xFFFFFFFF || vax.clock < vax.NICR ) {

                    // If an interrupt already happened, set ICCS<ERR> bit.
                    if( vax.ICCS & ( 1<< 7)) 
                        vax.ICCS |= 0x80000000;

                    // Set the interrupt bit.

                    vax.ICCS |= ( 1 << 7 );

                    // If ICCS<IE> then also declare an interrupt at IPL 22

                    if( vax.ICCS & ( 1 << 6 ))
                        interrupt( 0xC0, 22, 0 );

                    // Load up with the next interval timer value.

                    vax.clock = vax.NICR;
                }
                
                /* else 
                    vax.clock++; */

            }

        //  Handle GUI issues.  On cooperative task systems like the Mac, you have to
        //  yield once in a while to let other stuff run, including handling GUI events.
        
            if( vax.uiquantum.initial ) {
            
                vax.uiquantum.current++;
                if( vax.uiquantum.current > vax.uiquantum.initial ) {
                
                    vax.uiquantum.current = 0;
                    
            //  Now's the time to see if a byte is available on the console input
            
                    poll_keyboard();

#if macintosh
            //  If we are allowing the emulator to time slice, check for it now.  On
            //  "quantum" boundaries of instruction executions, we allow the native
            //  console handler (SIOUX, part of CodeWarrior) to handle an event that
            //  it grabs from the event queue.  We call it until it's got all the
            //  events under control again.
            
                      while( SIOUXHandleOneEvent(0L))
                      	continue;
#endif
                    

                }

            }
        
            
            //  We need to see if an interrupt is ready to go.  Note that we don't
            //  do this if there is already an interrupt pending.
    
            if( !vax.interrupt_pending ) {

                iplast = 0L;
                ipnext = 0L;
                iquantum = vax.quantum.initial;
                
                //  Scan the queue of pending interrupts to see if one is ready.

                for( ip = vax.iqueue; ip; ip = ipnext  ) {
                    ipnext = ip-> next;

                    if( vax.debug & DBG_INTERRUPTS ) {
                        printf( "DEBUG(INTERRUPTS): quantum; evaluating interrupt %04X, IPL %d, age=%d, quantum %d\n",
                            ( LONGWORD ) ip-> code, ip-> ipl, ip-> age, ip-> quantum );
                    }

                    //  If the quantum on this interrupt is not yet zero, it hasn't
                    //  aged enough to be taken.  Skip to the next one.

                    if( ip-> age > 0 ) {
                        iplast = ip;
                        ip-> age--;
                        continue;
                    }

                    //  If we've already chosen an interrupt, then we don't do another.  We'll
                    //  have to catch this on the next time.
                    
                    if( vax.interrupt_pending )
                        continue;
                        
                    //  Since this interrupt is ready (age <= 0), see if it's IPL is greater
                    //  than the current IPL.  If so, we take this interrupt.  In taking the
                    //  interrupt, we also remove it from the queue.

                    if( ( ULONGWORD ) ip-> ipl > ( ULONGWORD ) vax.pslw.ipl ) {
                        vax.interrupt_pending = ip-> code;
                        vax.interrupt_ipl = ip-> ipl;

                        if( ip == vax.iqueue )
                            vax.iqueue = ipnext;
                        else
                            iplast-> next = ipnext;

                        if( vax.debug & DBG_INTERRUPTS ) {
                            printf( "DEBUG(INTERRUPTS): set interrupt %04X\n",
                                ( LONGWORD ) vax.interrupt_pending );
                        }

                            
                        freemem( ip );
                        break;
                    }
                    iplast = ip;
                }
                
            }

        }


        //  Whew.  After all that, if we decided to take an interrupt, let's
        //  handle it now.  This will take the interrupt, set it up on the
        //  appropriate stack, and requeue the PC appropriately.

        if( vax.interrupt_pending ) {

            set_fault( vax.interrupt_pending, 0 );

            // Use the IPL register as temp storage of NEW IPL.  This is the
            // only time IPL and PSL<IPL> are allowed to differ.  The handle
            // fault routine will use the IPL register value to store the
            // NEW IPL on return from this routine.

            vax.IPL = vax.interrupt_ipl;
            
            vax.interrupt_pending = 0;
            vax.interrupt_ipl = 0;
            
            for( brkp = vax.console.breakpoint_list; brkp; brkp = brkp -> next ) {
                if(( brkp-> kind == BREAK_FAULT ) && 
                   ( (ULONGWORD) brkp-> pc == (ULONGWORD) vax.fault.code )) {
                    vax.fault_pending = vax.fault.code;
                    vax.halted = 0;
                    if( brkp-> kind & BREAK_TEMPORARY ) {
                        clear_break( brkp );
                    }
                    return VAX_BREAK;
                }
            }
            
            handle_fault(  );
        }
        
        
        //  Move the trace pending bit to the trace bit.  This can have been set by
        //  a previous instruction.
        
        vax.pslw.tp = vax.pslw.t;
        
        //  See if the PC value is a stored break point location.  We don't
        //  do this if it's the PC we started at.  If it's the initial PC
        //  then reset our stored memory of the initial PC to an impossible
        //  value, so subsequent hits on this PC will cause a brkp.

        if( vax.console.breakpoint_list ) {
        
            if( vax.PC == initial_PC )
                initial_PC = 0xFFFFFFFF;
            else
                for( brkp = vax.console.breakpoint_list; brkp; brkp = brkp-> next ) {
                    if(( ( BREAK_TYPE & brkp-> kind ) == BREAK_ADDRESS ) && 
                       ( brkp-> pc == vax.PC )) {

                        if( brkp-> after ) {
                            brkp-> after--;
                            continue;
                        }

                        vax.halted = 0;
                        
                        if( brkp-> kind & BREAK_STEP )
                            format_instruction( "Stepped to", vax.PC );
                        else
                            format_instruction( "Break at  ", vax.PC );
                        
                        if( brkp-> kind & BREAK_TEMPORARY ) {
                            clear_break( brkp );
                            return VAX_BREAK;
                        }
                            
                        return VAX_BREAK;
                    }
                }

        }
          
        //  Decode the instruction (and optionally disassemble it)
        
        saved_pc = vax.PC;
        rc = decode_instruction( &opcode, disasm );
        
        if( rc == VAX_BREAK && ( saved_pc != initial_PC )) {
            vax.halted = 0;
            vax.PC = saved_pc;
            format_instruction( " Instruction break at  ", vax.PC );
            return VAX_BREAK;
        }
        
        //  If we are supposed to STEP/OVER, then see if the instruction is one
        //  of the branches we might step over.  If so, then set a temporary
        //  breakpoint on the new PC (following instruction).  If it was not a
        //  STEP/OVER candidate, then set it to a STEP/INSTR for the rest of the
        //  decode phase.  Note that in this case we must disassemble the 
        //  instruction again to force a DISASM flag so the STEP/OVER will act
        //  like a STEP/INSTR as far as output goes.
        
        if( vax.console.singlestep == STEP_OVER ) {
            if( break_over_instruction( opcode.index )) {
                set_break( vax.PC, 0, BREAK_ADDRESS | BREAK_STEP | BREAK_TEMPORARY );
            }
            else {
                vax.console.singlestep = STEP_INSTRUCTION;
            }
            vax.PC = saved_pc;
            rc = decode_instruction( &opcode, 1 );
            disasm = 2;
        }
        
        //  Okay, the decode phase didn't go quite right.  See if we have a pending
        //  breakpoint for the stored fault, and if so then handle it.
        
        if( rc ) {

            //  Okay, if the DECODE instruction detected that this was stepping
            //  onto an entry point, then we are done (i.e. we were only disassembling,
            //  and this is a .ENTRY point, already formatted and displayed).  This
            //  can only happen if the PC happens to be on an entry mask and the
            //  user chooses STEP.
            
            if( rc == VAX_STEPENTRY )
                return rc;
        
            //  Nope, not STEP on an entry point, so the instruction must not have
            //  decoded right.  See if the stored fault matches a breakpoint.

            vax.PC = saved_pc;
            for( brkp = vax.console.breakpoint_list; brkp; brkp = brkp -> next ) {
                if(( ( BREAK_TYPE & brkp-> kind ) == BREAK_FAULT ) && 
                   (( ULONGWORD ) brkp-> pc == ( ULONGWORD) vax.fault.code )) {
                    vax.fault_pending = vax.fault.code;
                    if( brkp-> kind & BREAK_TEMPORARY )
                        clear_break( brkp );
                    return VAX_BREAK;
                }
            }
            
            //  No breakpoint, just just fault like normal
            
            rc = handle_fault();
            if( rc )
                return rc;
            continue;
        }
        
        
        //  If we're disassembling, print it out now.
                    
        if( disasm ) {

            if( disasm == 2 )    /* If this was special case of STEP/OVER, reset disasm flag */
                disasm = 0;

            if( vax.pslw.is )
                modep = "ISP";
            else
                modep = mode_name[ vax.pslw.cur_mod ];

            printf( "[%s %08X] %s\n", 
                 modep, vax.SP, opcode.name );
            if( vax.debug & DBG_REGISTERS )
                save_regset( &rs );
            if( vax.debug & DBG_FULLDISASM )
                format_operands( &opcode );
            
        }

        //  Fetch the emulator routine for this instruction.
        
        routine = instruction[ opcode.index ].routine;

        //  Initialize the temp register index freshly
        
        vax.treg = MAXREG - 1;
        
        //  Update the count of instructions executed.  This is for the TIME
        //  console function.
        
        vax.console.instruction_count++;
        

        //  Call the emulator routine to handle the opcode.
        
        rc = (*routine)( &opcode );
    
        //  See if there was an anomalous return code
        
        if( rc ) {
        
            //  The vax already has the fault data stored away, so just
            //  handle it.
            
            if( rc == VAX_FAULT ) {
                vax.PC = vax.instruction_PC;
                for( brkp = vax.console.breakpoint_list; brkp; brkp = brkp -> next ) {
                    if((( BREAK_TYPE & brkp-> kind ) == BREAK_FAULT ) && 
                       (( ULONGWORD ) brkp-> pc == ( ULONGWORD ) vax.fault.code )) {
                        vax.fault_pending = vax.fault.code;
                        return VAX_BREAK;
                    }
                }
                rc = handle_fault();
                if( rc )
                    return rc;
                    continue;
            }

            //  If after all that, there's some outstanding error, return it.
                    
            if( rc )
                return rc;
                
        }
        
        //  If disassembly enabled and REGISTER tracking turned on, dump the
        //  changed register(s).
        
        if( disasm && ( vax.debug & DBG_REGISTERS ))
            check_regset( &rs );
            
        //  If the trace  bit was set, then after executing the instruction, we
        //  clear the bit and stop execution, without affecting the HALTED state
        //  of the cpu

        if( vax.pslw.tp ) {
            vax.pslw.tp = 0;
            rc = set_fault( EXC_TP, 0 );
            rc = handle_fault();
            if( rc )
                return rc;
            continue;
        }
        
        //  If the console is single stepping, then let's stop now
        
        if( vax.console.singlestep ) {

            //  See if this is a STEP/RETURN hit point
            
            if( vax.console.singlestep == STEP_RETURN ) {
            
                /* Convert to a STEP_OVER */
                
                vax.console.singlestep = STEP_NONE;
                
                /* Get the return address and make it the temp break point */
                
                rc = get_return( &step_pc );
                set_break( step_pc, 0, BREAK_TEMPORARY | BREAK_STEP | BREAK_ADDRESS );
                
                if( rc ) {
                    return rc;
                }
                
            }
            else
            if( vax.console.singlestep == STEP_INSTRUCTION ) {
                vax.halted = 1;
                vax.console.singlestep = STEP_NONE;
                return VAX_OK;
            }
          
            /* No, it might be the trickier step/over case.  We've saved the pc */
            /* of the next instruction in step_pc, so just keep trucking */
            
            if( vax.console.singlestep == STEP_OVER )
                vax.console.singlestep = STEP_NONE;  
         }
    }
    
    return vax.halted;    /* Reason code stored here by whomever does the halt! */
}



#ifdef macintosh

/*
 *
 *    int kbdhit()
 *
 *    returns true if any keyboard key is pressed without retrieving the key
 *    used for stopping a loop by pressing any key
 */
 
int kbdhit(void)
{
      EventRecord event;  
      return EventAvail(keyDownMask,&event); 
}

/*
 *
 *    int getkbd()
 *
 *    returns the keyboard character pressed when an ascii key is pressed  
 *    used for console style menu selections for immediate actions.
 */
int getkbd(void)
{
   int c;
   EventRecord event;

/* fflush( stdout ) */

         /* Allow SIOUX response for the mouse, drag, zoom, or resize. */
    while(!GetNextEvent(keyDownMask,&event)) 
    {

//      if (GetNextEvent(updateMask | osMask | mDownMask | mUpMask, &event)) /* mm 980506 */
//          SIOUXHandleOneEvent(&event);
    }
    c = event.message&charCodeMask;
    if (c == '.' && (event.modifiers&cmdKey))
        raise( SIGINT );
  
   return c;
}

#else
#ifdef WIN

#define kbdhit _kbhit
#define getkbd _getkbd

#else
/*
 *  If we just don't have the ability to do asynchronous keyboard input yet, then stub it out.
 */

#define kbdhit() 0
#define getkbd() 0

#endif
#endif

/*
 *  This is currently written to support MacOS with Metrowerks and Windows with Visual Studio
 */

int poll_keyboard( void )
{

#if defined( macintosh ) || defined( WIN )
                
    if(  kbdhit() 

#ifdef macintosh
                    || stdin-> buffer_len 
#endif
                                            ) {
    
        int kbd_debug = vax.debug & DBG_KBD;
        
        
        /* If we already had data, sit tight... */
        if( vax.RXCS & 0x80 ) {
            if( kbd_debug )
                printf( "KBD: hit, not read yet, ignoring\n" );
            return 0; /* Do nothing */ ;
        }
        else { 
        
            /*  Put the byte into the data register */

#ifdef macintosh            
            if( stdin-> buffer_len )
                vax.RXDB = getc( stdin );
            else
                vax.RXDB = getkbd();
#else
            vax.RXDB = getch();
#endif
    
            if( kbd_debug )
                printf( "KBD: hit, read byte \"%c\"\n", vax.RXDB );
            
        
            /* Show that a  byte is in the data register */
            vax.RXCS |= 0x80;
            
            /* Should we interrupt the processor? */
            if( vax.RXCS & 0x00000040 ) {
                if( kbd_debug )
                    printf( "KBD: RXCS says interrupt\n" );
                interrupt( EXC_CONREAD, 20, 0 );
            }
            return 1;
        }       
        
    }
#endif

    return 0;
}

/*
 *  See if the current instruction qualifies for a STEP/OVER break.  Basically
 *  is the instruction a CALL or JSB sort of instruction?
 */
 
LONGWORD break_over_instruction( int op )
{
    static int break_list[] = {
        0x10,       /* BSBB  */
        0x16,       /* JSB   */
        0x30,       /* BSBW  */
        0xBC,       /* CHMK  */
        0xBD,       /* CHME  */
        0xBE,       /* CHMS  */
        0xBF,       /* CHMU  */
        0xFA,       /* CALLG */
        0xFB,       /* CALLS */
        0 };        /* end of list */
    
    int n;
        
    for( n = 0; break_list[ n ]; n++ ) {
        if( op == break_list[ n ])
            return 1;
    }
    
    return 0;
    
}

EMULATOR_ENTRY( emul_unimplemented )
{
    printf( "ERROR: Attempt to execute unimplemented instruction at %08X\n", vax.PC);
    set_fault( EXC_PRIV, 0 );
    return VAX_FAULT;
}

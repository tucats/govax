//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     interrupt.c
//
//  Purpose:    Interrupt handling.  This includes the means where any routine
//              can declare a fault condition (With paramaters).  This set_fault()
//              routine is responsible for storing the signal argument data in
//              the virtual CPU control data.  The handle_fault() then takes this
//              information and builds a fault signal frame, etc.  The handle_fault()
//              is called from the appropriate point(s) during execution where a
//              fault can be initiated, such as between instructions or during a
//              LONGWORD character operation.
//
//  History:    07/28/97    New header format standardization
//
//              02/03/99    Add EXCEPTION debugging
//
//              02/03/99    Added CHMK, CHME, CHMS, CHMU instructions.
//
//              05/25/99    Added interrupt generation ability.
//
//              11/08/99    Created set_break and clear_break routines in this
//                          module, shared by both the console SET BREAK commands
//                          and the console STEP processor, which  creates one-shot
//                          breakpoints to support STEP/OVER and STEP/RETURN.
//
//		02/20/02    Added support for a FAULT/EXCEPTION history track, which
//			    keeps up with the last 'n' faults.  The default is 5 
//			    faults, and can be (re)set with the SET FAULT n command.
//
#include "vax.h"
#include "console_proto.h"

#include <stdarg.h>

static LONGWORD chf( void );

static LONGWORD fault_history_p = -1L;
static LONGWORD fault_history_count = 0;
static LONGWORD fault_history_max = 8;
static struct FAULT ** fault_history = 0L;

#define MAX_CHF_DEPTH 4096   /* Max depth we'll hunt for a condition handler on stack */

//  Set new fault history size

LONGWORD set_fault_history( int fsize )
{
    int n;
    
    if( fault_history ) {
        for( n = 0; n < fault_history_max; n++ )
            if( fault_history[n] )
                freemem((void*)fault_history[n]);
                
        freemem((void*)fault_history);
    }
    
    fault_history = 0L;
    fault_history_p = -1L;
    fault_history_max = fsize;

    return VAX_OK;
}

//  Show faults in the history queue

LONGWORD show_faults( void )
{

    struct FAULT * fp;
    
    int	max, i, n, p;
    
    max = ( fault_history_count > fault_history_max ) ?
            (int) fault_history_max : (int) fault_history_count;
    p = (int) fault_history_p + 1;
    if( p >= fault_history_max )
       p = 0;
    
    if( max == 0 || fault_history_p < 0 || fault_history == 0L ) {
        printf( "No exceptions or interrupts have occurred yet\n" );
        return VAX_OK;
    }
    
    printf( "FAULT/EXCEPTION EVENT HISTORY:\n" );
    
    for( i = 0; i < max; i++ ) {
    
        fp = fault_history[ p ];
        if( fp ) {
            printf( "%08X %-40s SEQ(%u)  PC(%08X) PSL(%08X) ",
                    fp-> code, vaxexcept((short)fp-> code), fp-> history_id,
                    fp-> pc, fp-> psl );
            
            if( fp->signal_args[0]) {
                printf( " ARGS(" );
                for( n = 1; ( ULONGWORD ) n <= fp-> signal_args[ 0 ]; n++ ) {
                    if( n > 1 )
                        printf( "," );
                    printf( "%08X",  fp-> signal_args[ n ] );
                }
                printf(") " );
            }
        printf( "\n");
        }
        p = p + 1;
        if( p >= fault_history_max )
            p = 0;

    }
    return VAX_OK;
}

//  Store a fault in the history queue

LONGWORD store_fault( struct FAULT * f )
{
    struct FAULT * fp;
    LONGWORD size, n;
    
    if( fault_history == 0L ) {
        size = sizeof( struct FAULT ) * fault_history_max;
        fault_history = ( struct FAULT **)getmem(size);
        for( n = 0; n < fault_history_max; n++ )
            fault_history[ n ] = 0L;
    }
    
    fault_history_count++;
    
    fault_history_p++;
    if( fault_history_p >= fault_history_max )
        fault_history_p  = 0;
    
    f->history_id = fault_history_count;
    
    fp = ( struct FAULT * ) getmem(sizeof( struct FAULT ));
    *fp = *f;
    
    if( fault_history[ fault_history_p] != 0L )
        freemem( ( void * ) fault_history[fault_history_p]);
        
    fault_history[ fault_history_p ] = fp;
    return VAX_OK;
    
}

//  A fault must be signalled... store the data in the fault data structure
//  in the virtual CPU.  At the appropriate timing boundary, this data is used
//  to invoke a fault.

LONGWORD set_fault( LONGWORD code, LONGWORD Count, ... )
{

    va_list ap;
    LONGWORD    n, count;
    
    va_start( ap, Count );

    vax.fault.code = code;
    vax.fault.signal_args[ 0 ] = Count;
    
    count = Count;
    
    if( count > 6 )
        count = 6;
    
    for( n = 1; n <= count; n++ ) {
        vax.fault.signal_args[ n ] = va_arg( ap, LONGWORD );
    }
    
    va_end( ap );
    
    write_psl_bits();  /* Make bits match processor cache */
    
    vax.fault.pc = vax.instruction_PC;
    vax.fault.psl = vax.psl.reg;
    vax.fault.r0 = vax.R0;
    vax.fault.r1 = vax.R1;

    store_fault( &vax.fault );
    
    if( vax.debug & DBG_EXCEPTIONS ) {

        printf( "DEBUG(EXCEPTION): SET, CODE=%02X  PC=%08X  PSL=%08X  ARGC=%d\n",
            code, vax.fault.pc, vax.fault.psl, count );
        if( count ) {
            printf( "DEBUG(EXCEPTION): ARG%s = ", ( count == 1 ) ? "" : "S" );
            for( n = 1; n <= count; n++ ) {
                if( n > 1 )
                    printf( ", " );
                printf( "%08X", vax.fault.signal_args[ n ] );
            }
            printf( "\n" );
        }
    }

    return VAX_FAULT;
}


//  Create a signal fault frame on the stack and redirect execution
//  to the defined fault handler.

LONGWORD handle_fault(void)
{


    short stack;
    LONGWORD addr, vector;
    LONGWORD saved_mapen;
    LONGWORD rc, new_mode;
    LONGWORD saved_psl;
    static char * stackname[ 4 ] =
       { "KSP", "ESP", "SSP", "USP" };

    //  Get address of the exception vector 
    
    addr = vax.SCBB;
    
    //  Add the offset for the exception code and read the exception address
    
    saved_mapen = vax.MAPEN;
    vax.MAPEN = 0;
    rc = load_memory( addr + vax.fault.code, ( unsigned char * ) &vector, 4 );
    vax.MAPEN = saved_mapen;
    vax.fault_pending = 0;
    
    if( rc )
        return rc;

    //  The native console is able to format and print exceptions in a nice way.
    //  See if the vector value is 0xFFFFFFFF which means let the console handle it.
    
    if( vector == 0xFFFFFFFF)
        return format_exception();
        
    stack = ( short ) ( vector & 0x00000003L );
    vector = vector & 0xFFFFFFF8L;

    //  Figure out the new mode to set.  CMKx instructions set to a
    //  specific mode.  The others are KERNEL mode by default, and 
    //  later figure out if their are on the interrupt stack or not...

    if( vax.fault.code >= EXC_CHMK && vax.fault.code <= EXC_CHMU )
        new_mode = ( vax.fault.code & 0x0000000F ) >> 2;
    else
        new_mode = AM_KERNEL;


    //  Save the PSL.  We'll need it later to push on the stack.

    write_psl_bits();    
    saved_psl = vax.psl.reg;

    //  Now that we've saved the PSL, we can reset the IPL.  The new IPL
    //  was temporarily put in the IPL register by the interrupt detection
    //  logic in the execute_vax() driver.  We can't have set it before
    //  now or the saved PSL will be goofed up (don't want to save the new
    //  IPL, need to save the old IPL instead).  Note that this will only
    //  change the value on an interrupt, never an exception.

    vax.pslw.ipl = (unsigned int) vax.IPL;
    vax.psl.bit.ipl = (unsigned int) vax.IPL;

    //  Based on the nature of the exception, go ahead and put the right
    //  stack pointer in play and change modes.

    if( stack == 0 ) {
        set_mode_stack( new_mode ); 
    }
    else
    if( stack == 1 ) {
        set_mode_stack( 4 );  /* 4 = Interrupt stack, kernel mode */
    }

    invalidate_tb_prot();

    //  If we're debugging, dump the data now.

    if( vax.debug & DBG_EXCEPTIONS ) {
        printf( "DEBUG(EXCEPTION): TAKE, CODE=%04X  VECTOR=%08X  STACK=%08X [%s]\n",
            vax.fault.code, vector, vax.SP,
            ( stack == 0 ) ? stackname[ new_mode ] : "ISP, VM=OFF" ); 
    }

    //  Push the PSL onto the stack

    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &( saved_psl ), 4 );
    if( rc )
        return rc;
        
    //  Push the PC onto the stack
    
    vax.SP -= 4;
    rc = store_memory( vax.SP, ( void * ) &(vax.PC ), 4 );
    if( rc )
        return rc;
    
    if( vax.fault.signal_args[ 0 ] > 0 ) {
        vax.SP -= 4;
        rc = store_memory( vax.SP, ( void * ) &( vax.fault.signal_args[ 1 ]), 4 );
        if( rc )
            return rc;
    }
        
    if( vax.fault.signal_args[ 0 ] > 1 ) {
        vax.SP -= 4;
        rc = store_memory( vax.SP, ( void * ) &( vax.fault.signal_args[ 2 ]), 4 );
        if( rc )
            return rc;
    }

    
    
    vax.PC = vector;
        
    if( vector == 0L )
        return VAX_UNHANDLED;
    
    else
        return VAX_OK;
}

//  Format the current exception.  This is done typically when the default console
//  SCB handler is installed.  The job of this handler is to display what happened,
//  and then stop the emulation so the user can figure out what's going on.


long format_exception(void)
{
    unsigned long n;
    

    write_psl_bits();    

    /* See if there is a condition handler that can deal with it.  If so, done */

    if( chf() == VAX_OK )
        return VAX_OK;

    /* Nope, it's an unhandled exception */

    printf( "%%VAX-E-CONHANDLER, %s, PC=%08X  PSL=%08X\n", 
                 vaxexcept(( short ) vax.fault.code ),
                 vax.PC, vax.psl.reg );
                             
    
    if( vax.fault.signal_args[ 0 ] ) {
        printf( "-VAX-E-CONSIGARG, There are %u signal arguments,\n       ", 
                vax.fault.signal_args[ 0 ]);
        for( n = 1; n <= vax.fault.signal_args[ 0 ]; n++ ) {
            printf( "%08X", vax.fault.signal_args[ n ] );
            if( n < vax.fault.signal_args[ 0 ] )
                printf( ", " );
        }
        printf( "\n" );
    }

    vax.halted = 1;
    return VAX_OK;
}

    
//  Change mode instruction

EMULATOR_ENTRY( emul_chmx )
{

    LONGWORD    tmp1, tmp2, tmp3, tmp4, rc;
    short   code, *code_p;
    LONGWORD    n;

    //  Find out the new mode, based on the function.  CHMK = BC, CHME = BD, etc. */

    tmp1 = opcode-> function - 0x00BC;   /* 0=K, 1=E, 2=S, 3=U */

    //  Determine maximum privilege

    if( ( ULONGWORD ) tmp1 < vax.pslw.cur_mod )
       tmp2 = tmp1;
    else
       tmp2 = vax.pslw.cur_mod;

    //  Get the code

    code_p = ( short * ) get_operand( opcode, 0, OP_RD );
    if( !code_p )
        return VAX_FAULT;
    code = *code_p;
    tmp3 = code;

    //  We're in big trouble if this is the interrupt stack.

    if( vax.pslw.is )
        return VAX_HALT;

    //  Find out what the new SP will be so we can verify we have enough
    //  stack space.

    tmp4 = vax.preg[ tmp1 ];

    //  Verify that there are at least 12 bytes of stack space valid
    //  for the new mode.  (SRM Ch. 6)

    for( n = tmp4; n < tmp4+12; n = n + 4 ) {
        rc = probe( ( ULONGWORD ) n, 4, ( short ) tmp2 );
        if( rc )
            return rc;
    }

    //  Looks good.  Initiate the right exception, which will change the mode.
    //  Note that we update the instruction_PC field so the PC saved is for
    //  the next instruction.

    vax.instruction_PC = vax.PC;

    rc = set_fault( EXC_CHMK + ( tmp1 * 4 ), 1, tmp3 );
    return VAX_FAULT;
}


/*
 *  When we do a mode change, handle the stack,  etc. correctly.
 *  This also sets the is interrupt stack bit in the PSL.
 */

void set_mode_stack( LONGWORD newmode )
{
    ULONGWORD new_sp;
    char * mode_name[] = { "KERNEL", "EXEC", "SUPER", "USER" };

    if( vax.pslw.cur_mod == ( ULONGWORD ) newmode )
        return;

    if( vax.debug & DBG_CHM ) {
        printf( "DEBUG(CHM): CHANGE MODE FROM %s TO %s AT %08X\n", 
            mode_name[ vax.pslw.cur_mod ],
            mode_name[ newmode ], vax.instruction_PC );
    }

    /* Update the previous mode to be the current mode */
    vax.pslw.prv_mod = vax.pslw.cur_mod;

    /* Find out what the new SP will be */

    if( newmode == 4 )
        new_sp = vax.ISP;
    else
        new_sp = vax.preg[ newmode ];


    /* Store away the current SP in the appropriate area */

    if( vax.pslw.is )
        vax.ISP = vax.SP;
    else {
        vax.pslw.is = 0;
        vax.preg[ vax.pslw.cur_mod ] = vax.SP;
    }

    /* Then set the SP and cur_mod field to match the new mode */

    if( newmode == 4 ) {
        vax.SP = new_sp;
        vax.pslw.is = 1;
        vax.pslw.cur_mod = 0;
        vax.MAPEN = 0;
    }
    else {
        vax.pslw.cur_mod = newmode;
        vax.SP = new_sp;
        vax.pslw.is = 0;

        vax.MAPEN = 1;        /* Not sure about this!! */

    }

    return;
}

//  Queue up an interrupt.  This determines if it should be taken now or stored
//  away.

LONGWORD interrupt( LONGWORD code, LONGWORD ipl, LONGWORD quantum )
{

    struct INTERRUPT * ip;
    LONGWORD queue;
    
    queue = 0;

//  There may be an issue where we get an interrupt that is higher priority
//  than the one we are ready to act on.  Should we take the pending one and
//  queue it and replace it with the new one?  Not sure if interrupt ordering
//  like this is critical or not.  Be on the lookout...


    /* If we are already handling an interrupt, or at/above the IPL of the  */
    /* interrupt then we queue it up for later.                             */
    
    if( vax.interrupt_pending || vax.pslw.ipl >= ( ULONGWORD ) ipl )
        queue = 1;
    
    /* If the user has a quantum value (wants to delay the interrupt) then  */
    /* do so now.                                                           */
    
    if( vax.quantum.initial && quantum > 0 )
        queue = 1;
        
    /* If we are already processing an interrupt, or the IPL is wrong, save it */
    
    if( queue ) {
    
        ip = ( struct INTERRUPT * ) getmem( sizeof( struct INTERRUPT ));
        ip-> next = vax.iqueue;
        vax.iqueue = ip;
        
        ip-> code = code;
        ip-> ipl = ipl;
        ip-> quantum = quantum;
        ip-> age = ( quantum / vax.quantum.initial );

        /* If there is a quantum and we are supporting quantums, then reset the */
        /* quantum evaluation now.                                              */
        
        if(( vax.quantum.initial > 0 ) && ( vax.quantum.current > quantum ))
            vax.quantum.current = quantum;
    
        if( vax.debug & DBG_INTERRUPTS ) {
            printf( "DEBUG(INTERRUPTS): queue interrupt %04X, IPL %d, age=%d, quantum %d\n",
                ( LONGWORD ) code, ipl, ip-> age, quantum );
        }
    }
    else {

        if( vax.debug & DBG_INTERRUPTS ) {
            printf( "DEBUG(INTERRUPTS): set interrupt %04X, IPL %d, quantum %d\n",
                ( LONGWORD ) code, ipl, quantum );
        }
    
        vax.interrupt_pending = code;
        vax.interrupt_ipl = ipl;
    }
    
    return VAX_OK;
}



/*
 *  Delete a breakpoint structure.
 */

long clear_break( struct BREAKSTR * match )
{
    struct BREAKSTR * bp, *nextbp;
    
    nextbp = 0L;
    for( bp = vax.console.breakpoint_list; bp; bp = bp-> next ) {
        if( bp == match )
            break;
        nextbp = bp;
        
    }
    
    if( nextbp == 0L ) 
        vax.console.breakpoint_list = vax.console.breakpoint_list -> next;
    else
        nextbp-> next = bp-> next;
    
    freemem( bp );
    return VAX_OK;
}


/*
 *  Create a breakpoint structure.
 */

struct BREAKSTR * set_break( LONGWORD pc, int after, int kind )
{
    struct BREAKSTR * bptr;
    
    bptr = ( struct BREAKSTR * ) getmem( sizeof( struct BREAKSTR ));
    if( bptr == 0L )
        return bptr;
        
    bptr-> pc = pc;
    bptr-> after = after;
    bptr-> kind = kind;     /* Type of breakpoint */

    bptr-> next = vax.console.breakpoint_list;
    vax.console.breakpoint_list = bptr;

    return bptr;
}


/*
 *	Condition Handling Facility
 */

LONGWORD chf(void)
{

        ULONGWORD fp, saved_sp, handler, siglen, sigargs, mechargs;
        ULONGWORD saved_r0, saved_r1;

        LONGWORD v;

        union MASKREG {
            LONGWORD longword;
            struct {
                int   spa: 2 ;
                unsigned int calltype: 1;
                int   mbz: 1 ;
                int   mask: 12 ;
                int   psw: 16 ;
            } bits;
        } mask_union;
        ULONGWORD mask, addr, ap, n, saved_ap, rc;
        ULONGWORD mech_r0, mech_r1, arg_pc;

        long count, saved_halt;
        char cmd[ 80 ], *cmdp;

        count = 0; /* Depth of call frame chase */
        saved_halt = vax.halted;

/*
 *  Start trucking through the stack frames, looking for an FP that
 *  Specifies an exception handler.
 */


        fp = vax.FP;
        ap = vax.AP;

        if( fp == 0L || ap == 0L ) {
            rc = VAX_NOFRAMES;
            return rc;
        }

        while( count < MAX_CHF_DEPTH ) {

            count++;
            addr = fp;

            /* Get the condition handler address */

            rc = load_memory( addr, ( void * ) &handler, 4 );
            if( rc )
                return rc;

            /* "handler" is the address of the handler, if any */

            if( handler ) {

                saved_sp = vax.SP;

                siglen = 3 + vax.fault.signal_args[0];
 
                /* Push the PC and PSL */

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.fault.psl ), 4 );
                if( rc )
                    return rc;

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.fault.pc ), 4 );
                if( rc )
                    return rc;
                arg_pc = vax.SP;

                /* Push the arguments */

                for( n = vax.fault.signal_args[0]; n > 0; n-- ) {
                    vax.SP -= 4;
                    rc = store_memory( vax.SP, ( void * ) &( vax.fault.signal_args[n] ), 4 );
                    if( rc )
                        return rc;
                }

                /* Push the condition value */

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.fault.code ), 4 );
                if( rc )
                    return rc;

                /* Push the signal length */
               
                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( siglen ), 4 );
                if( rc )
                    return rc;
                sigargs = vax.SP;


                /* Now, create the mechanism vector */



                /* Store the R0 and R1 values */

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.fault.r1 ), 4 );
                if( rc )
                    return rc;
                mech_r1 = vax.SP;


                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.fault.r0 ), 4 );
                if( rc )
                    return rc;
                mech_r0 = vax.SP;

                /* Store the depth count */

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( count ), 4 );
                if( rc )
                    return rc;

                /* Store the FP */

                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( vax.FP ), 4 );
                if( rc )
                    return rc;

                siglen = 4; /* Always 4 on VAX */
                vax.SP -= 4;
                rc = store_memory( vax.SP, ( void * ) &( siglen ), 4 );
                if( rc )
                    return rc;

                mechargs = vax.SP;

                /* Set up to call the handler now! */

                sprintf( cmd, "%08X ( %08X, %08X )", handler, sigargs, mechargs );
                if( vax.debug & DBG_EXCEPTIONS )
                    printf( "DEBUG: condition handler executing CALL %s\n", cmd );

                saved_r0 = vax.R0;
                saved_r1 = vax.R1;
                cmdp = cmd;

                rc = console_call( &cmdp );

                vax.SP = saved_sp;
                vax.halted = saved_halt;

                /* Use return code from handler for guidance */

                if( vax.debug & DBG_EXCEPTIONS )
                    printf( "DEBUG: condition handler returns %08X\n", vax.R0 );
                rc = vax.R0;

                vax.R0 = saved_r0;
                vax.R1 = saved_r1;

                /* If the handler was happy, we're done.  But we must restore */
                /* the PC and the R0/R1 from the vectors, in case the handler */
                /* modified them...                                           */


                if( rc && 0x00000001 ) {

                    load_memory( mech_r0, ( void * ) &( vax.R0), 4 );
                    load_memory( mech_r1, ( void * ) &( vax.R1), 4 );
                    load_memory( arg_pc,  ( void * ) &( vax.PC), 4 );

                    return VAX_OK;
                }                
            }



            /* Get the mask longword that defines the frame format */

            addr += 4;
            rc = load_memory( addr, ( void * ) &mask_union.longword, 4 );
            if( rc )
                return rc;

            /* Get the PC, FP, and PC registers */

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;
            saved_ap = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            fp = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            /* Get the saved registers, if any */

            addr += 4;
            mask = mask_union.bits.mask;
            for( n = 0; n <= 11; n++ ) {

                if( mask & ( 1 << n )) {
                    rc = load_memory( addr, ( void * ) &v, 4 );
                    if( rc )
                        return rc;
                    addr += 4;
                }
            }

        /* The value of FP was captured above and will be the new */
        /* frame pointer now.  However, if the FP is zero then we */
        /* are done. Also if the FP is the magic value 'fdef' we  */
        /* are looking at a frame created by the console and are  */
        /* unable to continue.                                    */

            if( fp == 0L || fp == 0xFFFFDEAF )
                break;

        }

        return VAX_UNHANDLED;
}

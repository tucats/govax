//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_procreg.c
//
//  Purpose:    Emulator handlers for MTPR and MFPR instructions
//
//
//  History:    02/21/98    New module
//
//              06/09/99    Make sure IPL register tracks PSL.IPL


#include "vax.h"

static char priv_reg_access[ MAXPRIVREG + 1];
static char first_call = 1;


static void init_reg_access( void );


void init_reg_access(void)
{
    LONGWORD    n;
    
    first_call = 0;

/*
 *  Initially, set all to NO ACCESS
 */

    for( n = 0; n <= MAXPRIVREG; n++ )
        priv_reg_access[ n ] = OP_NL;

/*
 *  Now, for groups of them that are read-write, set them now.
 */

    priv_reg_access[  0 ] = OP_MD;      /* KSP      */
    priv_reg_access[  1 ] = OP_MD;      /* ESP      */
    priv_reg_access[  2 ] = OP_MD;      /* SSP      */
    priv_reg_access[  3 ] = OP_MD;      /* USP      */
    priv_reg_access[  4 ] = OP_MD;      /* ISP      */
    priv_reg_access[  8 ] = OP_MD;      /* P0BR     */
    priv_reg_access[  9 ] = OP_MD;      /* P0LR     */
    priv_reg_access[ 10 ] = OP_MD;      /* P1BR     */
    priv_reg_access[ 11 ] = OP_MD;      /* P1LR     */
    priv_reg_access[ 12 ] = OP_MD;      /* SBR      */
    priv_reg_access[ 13 ] = OP_MD;      /* SLR      */
    priv_reg_access[ 16 ] = OP_MD;      /* PCBB     */
    priv_reg_access[ 17 ] = OP_MD;      /* SCBB     */
    priv_reg_access[ 18 ] = OP_MD;      /* IPL      */
    priv_reg_access[ 19 ] = OP_MD;      /* ASTLVL   */
    priv_reg_access[ 20 ] = OP_WR;      /* SIRR     */
    priv_reg_access[ 21 ] = OP_MD;      /* SISR     */
    priv_reg_access[ 23 ] = OP_MD;      /* MCSR - 11/750 */
    priv_reg_access[ 24 ] = OP_MD;      /* ICCS     */
    priv_reg_access[ 25 ] = OP_WR;      /* NICR     */
    priv_reg_access[ 26 ] = OP_RD;      /* ICR      */
    priv_reg_access[ 27 ] = OP_MD;      /* TODR     */
    priv_reg_access[ 28 ] = OP_MD;      /* CSRS - Console storage R/S */
    priv_reg_access[ 29 ] = OP_MD;      /* CSRD - Console storage R/D */
    priv_reg_access[ 30 ] = OP_MD;      /* CSTS - Console storage T/S */
    priv_reg_access[ 31 ] = OP_MD;      /* CSTD - Console storage T/D */
    priv_reg_access[ 32 ] = OP_MD;      /* RXCS     */
    priv_reg_access[ 33 ] = OP_RD;      /* RXDB     */
    priv_reg_access[ 34 ] = OP_MD;      /* TXCS     */
    priv_reg_access[ 35 ] = OP_WR;      /* TXDB     */
    priv_reg_access[ 36 ] = OP_WR;      /* TBDR - 11/750 */
    priv_reg_access[ 37 ] = OP_WR;      /* CADR - Cache Disable Register - 11/750 */
    priv_reg_access[ 38 ] = OP_MD;      /* MCESR - Machine Check Error Summary Register - 11/750 */
    priv_reg_access[ 39 ] = OP_MD;      /* CAER - Cache Error Register - 11/750 */
    priv_reg_access[ 40 ] = OP_MD;      /* ACCS - Accelerator Control Register */
    priv_reg_access[ 41 ] = OP_MD;      /* SAVISP - Console Saved ISP */
    priv_reg_access[ 42 ] = OP_MD;      /* SAVPC  - Console Saved PC  */
    priv_reg_access[ 43 ] = OP_MD;      /* SAVPSL - Console Saved PSL */
    priv_reg_access[ 44 ] = OP_NL;      /* WCSA - Writable Control Store Address */
    priv_reg_access[ 45 ] = OP_NL;      /* WCSB - Writable Control Store Data */
    priv_reg_access[ 48 ] = OP_NL;      /* SBI Fault/status */
    priv_reg_access[ 49 ] = OP_NL;      /* SBI Silo */
    priv_reg_access[ 50 ] = OP_NL;      /* SBI Silo Comparator */
    priv_reg_access[ 51 ] = OP_NL;      /* SBI Silo Maintenance */
    priv_reg_access[ 52 ] = OP_NL;      /* SBI Error Register */
    priv_reg_access[ 53 ] = OP_NL;      /* SBI Timeout Address Register */
    priv_reg_access[ 54 ] = OP_NL;      /* SBI Quadword Clear */
    priv_reg_access[ 55 ] = OP_NL;      /* Initialize Unibus - 11/750 */
    priv_reg_access[ 56 ] = OP_MD;      /* MAPEN    */
    priv_reg_access[ 57 ] = OP_WR;      /* TBIA     */
    priv_reg_access[ 58 ] = OP_WR;      /* TBIS     */
    priv_reg_access[ 59 ] = OP_NL;      /* TBDATA - Translation Buffer Data */
    priv_reg_access[ 60 ] = OP_NL;      /* MBRK - Microprogram Break */
    priv_reg_access[ 61 ] = OP_MD;      /* PMR      */
    priv_reg_access[ 62 ] = OP_MD;      /* SID      */
    priv_reg_access[ 63 ] = OP_WR;      /* TBCHK    */

    
    return;
}




EMULATOR_ENTRY( emul_mtpr )
{

    LONGWORD value, *value_a;
    LONGWORD reg,   *reg_a;

    
/*
 *  Get the value to move.
 */

    value_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( value_a == 0L )
        return VAX_FAULT;
    value = *value_a;
    
/*
 *  Find out which register it's to.
 */
 
    reg_a = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( reg_a == 0L )
        return VAX_FAULT;
    reg = *reg_a;

/*
 *  Set the value.  This is a separate routine so that the SET command can
 *  use it (and also get the checking and effects desired.
 */

    return set_priv_reg( ( short ) reg, value );
}



/*
 *  Routine that actually sets a value into a privileged register.  If the 
 *  effect of writing to the register is needed, then this routine should be
 *  called rather than just jamming the value into the register.
 */

LONGWORD set_priv_reg( short reg, LONGWORD value )
{
    LONGWORD ibit, n;

/*
 *  If first call, we need to set up the access data for the privileged registers.
 */
    
    if( first_call )
        init_reg_access();


/*
 *  If it's an invalid register number, puke out now.
 */
 
    if( reg < 0 || reg > MAXPRIVREG )
        return set_fault( EXC_RESOP, 0 );

/*
 *  If we are not in kernel mode, then boo-boo.
 */
 
    if( vax.pslw.cur_mod != 0x00 )    /* Kernel mode = 0 */
        return set_fault( EXC_PRIV, 0 );

/*
 *  See if we're allowed to write to this register
 */
 
    if( priv_reg_access[ reg ] != OP_MD &&
        priv_reg_access[ reg ] != OP_WR )
            return set_fault( EXC_RESOP, 0 );
            

/*
 *  Some special things can happen on a write operation.  Let's deal with them.
 */

    switch( reg ) {
    
    case 18:    /* IPL */
    
                value = value & 0x1F;
                vax.pslw.ipl = value;
                vax.IPL = value;
                
                /* Because we might have lowered the IPL, check for pending */
                /* software interrupts.                                     */

                for( n = 15; n > value; n-- ) {
                    if( vax.SISR & ( 1 << n )) {
                        vax.SISR &= ~( 1 << n );
                        interrupt( ( 0x000000080 + ( n << 2 )), n, 0 );
                    }
                }

                break;
    

    case 19:    /* ASTLVL */

                if( value < 0 || value > 4 ) {
                    set_fault( EXC_RESOP, 0 );
                    return VAX_FAULT;
                }
                vax.ASTLVL = value;
                break;


    case 20:    /* SIRR */

                value = value & 0x0000000F; /* Only low order 4 bits count */

                /* If the request is at a higher level than we are at now, take it */

                if( ( ULONGWORD ) value > vax.pslw.ipl ) {
                    interrupt(  ( 0x00000080  + ( value << 2 )), value, 0 );
                    break;
                }

                vax.SISR |= ( 1 << value );
                vax.SIRR = value;

                break;


    case 24:    /* ICCS */

                vax.ICR = vax.clock;

                /* Only certain bits count */

                value = value & 0x800000F1;

                /* Do stuff with each bit */

                /* If write to ICCS<INT>, clear it */

                if( value & 0x00000080 )
                    vax.ICCS &= ~( 0x00000080 );

                /* If write to ICCS<RUN>, set it */

                if( value & 0x00000001 ) {
                    vax.ICCS |= 0x00000001;
                    vax.clock_running = 1;
                }
                else
                    vax.clock_running = 0;

                /* if write to ICCS<XFR>, copy NICR to ICR */

                if( value & 0x00000010 )
                    vax.clock = vax.ICR = vax.NICR;

                /* if write to ICCS<SGL>, increment the clock */

                if( value & 0x00000020 ) {
                    vax.clock++;
                    vax.ICR = vax.ICR + 1;
                }

                /* if write to ICCS<IE>, set it */

                vax.ICCS &= ~(0x00000040);
                vax.ICCS |= ( value & 0x00000040 );

                /* if write to ICCS<ERR>, clear it */
                if( value & 0x80000000 )
                    vax.ICCS &= ( 0x7FFFFFFF );

                break;

    case 32:    /* RXCS */
    
                value = value & 0x40;   /* Can only write the IE bit */
                ibit = vax.RXCS & 0x40;
                
                vax.RXCS = ( vax.RXCS & 0x80 ) | value;     /* Write the bit */
                
                /* If done and IE set, then signal interrupt */
                
                if( ibit && ( vax.RXCS & 0xC0 ))
                    interrupt( EXC_CONREAD, 20, 1 );
                break;
                
                    
    case 34:    /* TXCS */
    
                ibit = vax.TXCS & 0x00000040; /* Save the current IE bit */
                
                value = value & 0x00000040L;    /* Can only write the IE bit */
                
                /* Note that the RDY bit is always set; we never have to */
                /* wait for a console transmit to complete.              */
                
                if( value )
                    vax.TXCS = 0x000000C0;    /* RDY and IE set */
                else
                    vax.TXCS = 0x00000080;    /* Only RDY set */
                
                /*  At this point, if value is non-zero, trigger an IPL 20  */
                /*  interrupt of Console Term Trans since IE and RDY are    */
                /*  both 1 now.                                             */
                
                if( value )
                    interrupt( 0xFC, 0x20, /* 10 */ 0 );

                break;
                
                
    case 35:    /* TXDB */

                value = value & 0x007FL;
                poll_keyboard();    /* Make sure we get any active keys waiting first... */
                
                printf( "%c", ( char ) value );
                fflush( stdout );
                
                ibit = vax.TXCS & 0x00000060; /* Get the IE bit */
                vax.TXCS |= 0x00000080;       /* Set the RDY bit */
                
                /* If the ibit is true, generate an IPL 20 interrupt */
                if( ibit )
                    interrupt( 0xFC, 0x20, /* 30 */ 0 );
                
                break;
        
                
    case 58:    /* TBIS */
    
                invalidate_page( value );
                break;
    
    case 57:    /* TBIA */
    
                invalidate_tb();
                break;
    
    default:    /*  nothing tricky, just store away the value. */

                vax.preg[ reg ] = value;

    }
        
    return VAX_OK;
}



EMULATOR_ENTRY( emul_mfpr )
{

    LONGWORD procreg, *procreg_a;

/*
 *  If first call, we need to set up the access data for the privileged registers.
 */
    
    if( first_call )
        init_reg_access();
    
/*
 *  Since this about the only time we can access the TODR,  let's go
 *  ahead and update it while we're here.
 */
#if 0  /* This is now handled by the millisecond timer */

    if( vax.TODR != 0L ) {

        LONGWORD new_time;

        /* Difference is 60th's of a second, make seconds then 100's */
        
        new_time = (( TickCount() - vax.timebase ) * 100 ) /60;
    
        /* Because instructions happen faster than TickCount cycles, */
        /* if the newly calculated time is the same as the one we    */
        /* already have, just increment it by one (assumes 10ms have */
        /* passed) so it always changes when we look at it.          */
        
        if( ( ULONGWORD ) new_time == vax.TODR )
            vax.TODR += 1;
        else
            vax.TODR = new_time;
    }
#endif

        vax.ICR = vax.clock;
        
/*
 *  Get address of the processor register number
 */

    procreg_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( procreg_a == 0L )
        return VAX_FAULT;
    
    procreg = *procreg_a;

/*
 *  See if it's in range.  If not, it's a reserved operand fault.
 */

    if( procreg < 0 || procreg > MAXPRIVREG ) {
        return set_fault( EXC_RESOP, 0 );
        
    }

/*
 *  If we are not in kernel mode, then boo-boo.
 */
 
    if( vax.pslw.cur_mod != 0x00 )    /* Kernel mode = 0 */
        return set_fault( EXC_PRIV, 0 );

/*
 *  See if we're allowed to read from this register
 */
 
    if( priv_reg_access[ procreg ] != OP_MD &&
        priv_reg_access[ procreg ] != OP_RD )
            return set_fault( EXC_RESOP, 0 );

/*
 *  Since we're reading a register, let's go ahead now and update
 *  some privileged registers that might have stale values.
 */

    vax.IPL = vax.pslw.ipl;
    vax.ICR = vax.clock;
/*
 *  Do we have side effects to reading this we need to deal with?
 */
 
    if( procreg == 33 )     /* RXDB clears done bit */
    
        vax.RXCS &= 0x40; /* Clear DON bit */
        
/*
 *  Otherwise, move the value from the privileged register to the destination.
 */

    return put_operand( opcode, 1, OP_WR, ( void * ) &( vax.preg[ procreg ]));

}


/*
 *  Load process context from the physical memory pointed to by the PCBB register.
 */
 
EMULATOR_ENTRY( emul_ldpctx )
{

    LONGWORD saved_mapen, rc, n;
    ULONGWORD pcb, tmpl;
    
/*
 *  If we're not in kernel mode, then fault.
 */
 
    if( vax.pslw.cur_mod != 0x00 )
        return set_fault( EXC_PRIV, 0 );

/*
 *  Dump the translation buffer cache entries, since we're about to mess
 *  with the P0 and P1 base registers, etc.
 */

    invalidate_tb();

/*
 *  Turn off virtual memory since we locate the PCB data by physical address.
 */
 
    saved_mapen = vax.MAPEN;
    vax.MAPEN = 0;
 
/*
 *  Get the PCB address.
 */

    pcb = vax.PCBB;

/*
 *  Load up the internal stack pointer registers.
 */

    rc = load_memory( pcb, ( void * ) &( vax.KSP ), 4 );
    if( rc )
        return rc;

    rc = load_memory( pcb + 4, ( void * ) &( vax.ESP ), 4 );
    if( rc )
        return rc;

    rc = load_memory( pcb + 8, ( void * ) &( vax.SSP ), 4 );
    if( rc )
        return rc;

    rc = load_memory( pcb + 12, ( void * ) &( vax.USP ), 4 );
    if( rc )
        return rc;
    vax.SP = vax.USP;

/*
 *  Load the GP registers R0 - R11, AP, and FP.
 */

    for( n = 0; n < 14; n++ ) {
    
        rc = load_memory( pcb + 16 + ( n * 4 ), 
                            ( void * ) &( vax.reg[ n ]), 4 );
        if( rc )
            return rc;
    }

/*
 *  Make sure the P0BR is okay.  It must be an S0 address aligned on
 *  a longword boundary.
 */

    rc = load_memory( pcb + 80, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    
    if(( tmpl & 0xC0000003UL ) != 0x80000000 ) 
        return set_fault( EXC_CHECK, 0 );

    vax.P0BR = tmpl;

/*
 *  Verify that the P0LR is okay.  It can only be 21 bits wide. 
 */
 
    rc = load_memory( pcb + 84, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    
    if( tmpl >> 27 != 0L )
        return set_fault( EXC_CHECK, 0 );

    /* Store only the lower 21 bits */
    
    vax.P0LR = tmpl & (( 1 << 21 ) - 1 );

/*
 *  Get the ASTLVL value, out of the same longword.
 */

    tmpl = tmpl >> 24;
    if( tmpl <= 5 )
        return set_fault( EXC_CHECK, 0 );

    vax.ASTLVL = tmpl;

/*
 *  Get the P1BR.
 */

    rc = load_memory( pcb + 88, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    
    vax.P1BR = tmpl;

/*
 *  Get the P1LR.
 */

    rc = load_memory( pcb + 92, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    
    if(( tmpl & 0xC0000003UL ) != 0x80000000 ) 
        return set_fault( EXC_CHECK, 0 );

    vax.P1LR = tmpl & (( 1 << 21 ) - 1 );

/*
 *  If we're in interrupt mode fix the stack up.
 */

    if( vax.pslw.is ) {
        vax.ISP = vax.SP;
        vax.pslw.is = 0;
        vax.SP = vax.KSP;
    }

/*
 *  We push the PSL and PC from the PCB onto the (current) stack, ready for a REI later.
 */

    rc = load_memory( pcb + 76, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    
    vax.MAPEN = saved_mapen;
    rc = push_data( ( void * ) &tmpl, 4 );
    if( rc ) 
        return rc;
    vax.MAPEN = 0;
    
    rc = load_memory( pcb + 72, ( void * ) &tmpl, 4 );
    if( rc )
        return rc;
    rc = push_data( ( void * ) &tmpl, 4 );
    if( rc ) 
        return rc;
 
    vax.IPL = vax.pslw.ipl; 

    return VAX_OK;
}



EMULATOR_ENTRY( emul_svpctx )
{

    LONGWORD saved_mapen, rc, n;
    ULONGWORD pcb, new_PC, new_PSL;
    
/*
 *  If we're not in kernel mode, then fault.
 */
 
    if( vax.pslw.cur_mod != 0x00 )
        return set_fault( EXC_PRIV, 0 );

/*
 *  Dump the translation buffer cache entries, since we're about to mess
 *  with the P0 and P1 base registers, etc.
 */

    invalidate_tb();

/*
 *  Turn off virtual memory since we locate the PCB data by physical address.
 */
 
    saved_mapen = vax.MAPEN;
    vax.MAPEN = 0;
 
/*
 *  Get the PCB address.
 */

    pcb = vax.PCBB;

/*
 *  Write the 4 SP registers.
 */

    for( n = 0; n < 4; n++ ) {
        rc = store_memory( pcb + ( n << 4 ), ( void * ) &( vax.preg[ n ]), 4 );
        if( rc )
            return rc;
    }

/*
 *  Write the 14 regular registers
 */

    for( n = 0; n < 14; n++ ) {
        rc = store_memory( pcb + 16 + ( n << 4 ),
                    ( void * ) &( vax.reg[ n ]), 4 );
        if( rc )
            return rc;
    }

/*
 *  Store the PC and PSL that are on the stack.
 */

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

    rc = store_memory( pcb + 72, ( void * ) &new_PC, 4 );
    if( rc )
        return rc;

    rc = store_memory( pcb + 76, ( void * ) &new_PSL, 4 );
    if( rc )
        return rc;

//  If we are not on the interrupt stack, put us there with an IPL of 1.

    if( vax.pslw.is == 0 ) {

        if( vax.pslw.ipl == 0 )
            vax.pslw.ipl = 1;

        rc = store_memory( pcb, ( void * ) &( vax.SP ), 4 );
        if( rc )
            return rc;

        vax.KSP = vax.SP;
        vax.pslw.is = 1;
        vax.SP = vax.ISP;

    }

    vax.IPL = vax.pslw.ipl;

    return VAX_OK;
}

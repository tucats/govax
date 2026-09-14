//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_misc.c
//
//  Purpose:    Emulator handlers for miscellaneous instructions
//
//  History:    07/28/97    New header format standardization
//
//              05/29/99    Added INSQHI, INSQTI instructions
//
//              06/23/99    Added PROBEx instructions.
//
//              09/25/99    REMQHI, REMQTI added by Carl Fongheiser (CMF)
//
//              09/27/99    Tweaks to make system 64-bit friendly.
//
//              07/09/01    Bug fixes for REMQHI, INSQTI


#include "vax.h"
#include "fpu.h"
#include "pte.h"  /* For PROBEx instructions */

//  Misc utility macro to get operands.

#define GET_OP( n, v ) \
    p32 = ( LONGWORD * ) get_operand( opcode, n, OP_RD );   \
    if( !p32 )                                               \
        return VAX_FAULT;                                    \
    v = *p32



//  BUGL, BUGW - bugcheck

EMULATOR_ENTRY( emul_bug )
{

    LONGWORD code;

//  The BUGL/BUGW instructions takes a single implicit immediate instruction argument.

    code = opcode-> VAXaddr[ 0 ];

//  Use that to define the fault code, and we're done.

    set_fault( EXC_PRIV, 1, code );

    return VAX_FAULT;
}




//  INDEX - compute an index for array references in a HLL.

EMULATOR_ENTRY( emul_index )
{

    LONGWORD subscript, low, high, size, in, out;

    LONGWORD *p32;

    //  Get all the various input parameters

    GET_OP( 0, subscript );
    GET_OP( 1, low );
    GET_OP( 2, high );
    GET_OP( 3, size );
    GET_OP( 4, in );

    //  Make sure the output parameter is addressable

    p32 = ( LONGWORD * ) get_operand( opcode, 5, OP_WR );
    if( !p32 )
        return VAX_FAULT;

    //  Calculate the index result value

    out = ( in + subscript ) * size;

    //  See if the value blows the limits of the range.  IF so,
    //  we signal a fault.

    if(( subscript < low ) || ( subscript > high )) {
        set_fault( EXC_ARITH, 1, TRAP_SUB_RNG );
        return VAX_FAULT;
    }

    vax.pslw.n = ( out <  0L );
    vax.pslw.z = ( out == 0L );
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    *p32 = out;
    return VAX_OK;
}


//  BISPSW, BICPSW - set or clear bits in the PSW (low-order word of PSL) under
//                   user control.

EMULATOR_ENTRY( emul_bitpsw )
{

    short mask;
    LONGWORD longmask;

//  Get the mask value.

    GET_OPERAND( mask, short, 0, OP_RD );

    
//  First, if bits 8:15 are non-zero then it's a fault.

    if( mask & 0xFF00 )
        return set_fault( EXC_RESOP, 0 );

//  Mask the bits to the PSL that we care about.  Depending on
//  the opcode, the mask is used to set or clear the bits...

    longmask = ( LONGWORD ) mask & 0x000000FF;
    
    write_psl_bits( );  /* Make sure bits match cached data */
    
    if( opcode-> function == 0xB8)
        vax.psl.reg |= longmask;      /* BISPSW */
    else
        vax.psl.reg &= ~longmask;     /* BICPSW */
    
    return VAX_OK;
}



//  PUSHR   Use a mask word to define registers to be pushed onto the stack.  You cannot
//          specify the PC with this instruction.

EMULATOR_ENTRY( emul_pushr )
{

    short mask, *mask_a;
    LONGWORD n;
    
//  Get the mask value that defines which registers to process.

    mask_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( mask_a == 0L )
        return VAX_FAULT;
    mask = *mask_a;

//  Loop over the list, pushing the data

    for( n = 14; n >= 0; n-- ) {
    
        if(( mask >> n ) & 0x0001 ) {
        
            vax.SP -= 4;
            store_memory( vax.SP, ( void * ) &( vax.reg[ n ]), 4 );
        }
    }
    
    return VAX_OK;
}




//  POPR    Use a mask word to define registers to be removed from the stack.

EMULATOR_ENTRY( emul_popr )
{

    short mask, *mask_a;
    LONGWORD n;
    
//  Get the mask value that defines which registers to process.

    mask_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( mask_a == 0L )
        return VAX_FAULT;
    mask = *mask_a;

//  Loop over the list, popping the register values

    for( n = 0; n <= 14; n++ ) {
    
        if( mask & ( 1 << n )) {
            load_memory( vax.SP, ( unsigned char * ) &( vax.reg[ n ]), 4 );
            vax.SP += 4;
        }
    }
    
    return VAX_OK;
}








//  HALT    Halts the processor on the next instruction boundary and returns control
//          to the console.

EMULATOR_ENTRY( emul_halt )
{

    if( !( vax.debug & DBG_USERHALT ) && vax.pslw.cur_mod > 0 ) {
        return set_fault( EXC_PRIV, 0 );
    }
    
    vax.halted = VAX_HALT;
    return VAX_OK;

}




//  NOP     Does no work, as cheaply as possible.  Later, we'll short-circuit the
//          decoder to recognize NOP as a special case that should not even be dispatched.

EMULATOR_ENTRY( emul_nop )
{
    return VAX_OK;
}





//  BPT     Signal a breakpoint exception

EMULATOR_ENTRY( emul_bpt )
{
    return set_fault( EXC_BPT, 0L );
}


//  PROBEx  Probe for read or write addressability

EMULATOR_ENTRY( emul_probe )
{

    char    mode, *mode_p;
    short   len, *len_p, kind, size, saved_mode;
    ULONGWORD   addr;
    LONGWORD    rc;
        
    /* Get the mode we are to test at (USER, KERNEL, etc.) */
    
    mode_p = ( char * ) get_operand( opcode, 0, OP_RD );
    if( !mode_p )
        return VAX_FAULT;
    mode = *mode_p;
    
    /* Get the length of the area.  Note that the VAX ARM says */
    /* that we only test the first and last byte of the area,  */
    /* so callers must arrange to stride over page boundaries  */
    /* correctly by using multiple PROBEx instructions.        */
    
    len_p = ( short * ) get_operand( opcode, 1, OP_RD );
    if( !mode_p )
        return VAX_FAULT;
    len = *len_p;
    
    /* The actual mode we'll use is the max (least privileged) of */
    /* the requested mode and the previous mode.  Usually, use    */
    /* zero to effectively test against the previous mode.        */
    
    if( vax.pslw.cur_mod > mode )
        mode = vax.pslw.prv_mod;

    /* See what kind of access this is (read or write).  We also */
    /* ensure that we won't signal on this translation.         */
    
    kind = VM_NOSIGNAL  + (( opcode-> function == 0x0C ) ? OP_RD : OP_WR);
    
    /* Test the first address */
    
    saved_mode = vax.pslw.cur_mod;
    vax.pslw.cur_mod = mode;
    
    addr = opcode-> VAXaddr[ 2 ];
    size = 1;
    rc = vm( &addr, &size, kind );
    
    /*  If that worked okay, test the end address */
    if( rc == VAX_OK ) {
        addr = opcode-> VAXaddr[ 2 ] + len - 1;
        rc = vm( &addr, &size, kind );
    }
    
    /* Restore the real mode, and set the Z bit based on our results */
    
    vax.pslw.cur_mod = saved_mode;
    vax.pslw.z = ( rc == VAX_OK ? 0 : 1 );
    
    return VAX_OK;
}

    


//  MOVPSL  Write the whole PSL somewhere.

EMULATOR_ENTRY( emul_movpsl )
{

    write_psl_bits( );
    return put_operand( opcode, 0, OP_WR, ( char * ) &vax.psl.reg );
}




//  INSQHI

EMULATOR_ENTRY( emul_insqhi )
{


    ULONGWORD   entry, header, offset, hf, hb;
    LONGWORD    rc;

    /*  Get the parameters */

    entry = opcode-> VAXaddr[ 0 ];
    header = opcode-> VAXaddr[ 1 ];

    /*  Get the header forward link */

    rc = load_memory( header, ( void * ) &hf, 4 );
    if( rc )
        return rc;
    hf = header + hf;

    /*  Get the header backwards link */
    rc = load_memory( header+4, ( void * ) &hb, 4 );
    if( rc )
        return rc;
    hb = header+ hb;

    vax.pslw.n = 0;
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    /*  Simplest case is that this is the first added to the queue */

    if( hf == hb ) {  /* Offsets match? */

        offset = entry - header;
        rc = store_memory( header, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        rc = store_memory( header+4, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        offset = header - entry;
        rc = store_memory( entry, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        rc = store_memory( entry+4, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        vax.pslw.z = 1;
        return VAX_OK;
    }


    /* Harder case is actually adding it in to the queue */

    /* Fix current first entry's back link to be new entry */
    offset = entry - hf;
    rc = store_memory( hf+4, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix new entry's back link to be the header */
    offset = header - entry;
    rc = store_memory( entry+4, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix header's forward link to be the new entry. */
    offset = entry - header;
    rc = store_memory( header, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix new entry's forward link to be old first entry */
    offset = hf - entry;
    rc = store_memory( entry, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    return VAX_OK;

}




//  INSQTI

EMULATOR_ENTRY( emul_insqti )
{


    ULONGWORD   entry, header, offset, hf, hb;
    LONGWORD    rc;

    /*  Get the parameters */

    entry = opcode-> VAXaddr[ 0 ];
    header = opcode-> VAXaddr[ 1 ];

    /*  Get the header forward link */

    rc = load_memory( header, ( void * ) &hf, 4 );
    if( rc )
        return rc;
    hf = header + hf;

    /*  Get the header backwards link */
    rc = load_memory( header+4, ( void * ) &hb, 4 );
    if( rc )
        return rc;
    hb = header+ hb;

    vax.pslw.n = 0;
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    /*  Simplest case is that this is the first added to the queue */

    if( /*  hf == 0 && hb == 0 */ hf == hb ) {

        offset = entry - header;
        rc = store_memory( header, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        rc = store_memory( header+4, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        offset = header - entry;
        rc = store_memory( entry, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        rc = store_memory( entry+4, ( void * ) &offset, 4 );
        if( rc )
            return rc;

        vax.pslw.z = 1;
        return VAX_OK;
    }


    /* Harder case is actually adding it in to the queue */

    /* Fix current last entry's forward link to be new entry */
    offset = entry - hb;
    rc = store_memory( hb+4, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix new entry's forward link to be the header */
    offset = header - entry;
    rc = store_memory( entry, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix header's backward link to be the new entry. */
    offset = entry - header;
    rc = store_memory( header+4, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    /* Fix new entry's backward link to be old last entry */
    offset = hb - entry;
    rc = store_memory( entry+4, ( void * ) &offset, 4 );
    if( rc )
        return rc;

    return VAX_OK;

}




//  INSQUE

EMULATOR_ENTRY( emul_insque )
{


    LONGWORD    entry;
    LONGWORD    pred;
    LONGWORD    pred_forw;
    LONGWORD    rc;

    /*  Get the parameters */

    entry = opcode-> VAXaddr[ 0 ];
    pred = opcode-> VAXaddr[ 1 ];

    /* Need to validate that all the memory is accessible! */
    /* TBD */


    /* Step 1.  The entry's forward link is set to pred's forward link */

    /* Get pred's forward link (pred) */
    rc = load_memory( pred, ( void * ) &pred_forw, 4 );
    if( rc )
        return rc;

    /* Store in new entries forward link (entry) */
    rc = store_memory( entry + 0, ( void * ) &pred_forw, 4 );
    if( rc )
        return rc;

    vax.pslw.n = ( pred_forw < pred );
    vax.pslw.z = ( pred_forw == pred );
    vax.pslw.v = 0;
    vax.pslw.c = ( (ULONGWORD) pred_forw < ( ULONGWORD ) pred );

    /* Step 2.  The entry's backward link is set to point to pred */

    /* Store pred as backward link of new entry */

    rc = store_memory( entry + 4, ( void * ) &pred, 4 );


    /* Step 3.  The backward link of the successor is set to the entry */

    /* Store in back link */
    rc = store_memory( pred_forw+4, ( void * ) &entry, 4 );
    if( rc )
        return rc;

    
    /* Step 4.  The forward link of the predecessor is set to the entry */

    rc = store_memory( pred, ( void * ) &entry, 4 );
    if( rc )
        return rc;


    return VAX_OK;

}




//  REMQHI - Implemented by CMF

EMULATOR_ENTRY( emul_remqhi )
{

    LONGWORD    header;
    LONGWORD tmp1, tmp2, tmp3, rc;
    
     /*      Get the parameters */

     header = opcode-> VAXaddr[ 0 ];

     rc = load_memory(header, (void *) &tmp1, 4);
     if (rc)
         return rc;

     vax.pslw.n = 0;
     vax.pslw.c = 0;

     if (tmp1 == 0) {                // Queue empty?
         vax.pslw.z = 1;
         vax.pslw.v = 1;
         rc = put_operand( opcode, 1, OP_WR, (void *) &header);
                                     // Store the address
         return VAX_OK;
     }

     rc = load_memory(header + 4, (void *) &tmp2, 4);
     if (rc)
         return rc;
     if (tmp1 == tmp2) {             // Only one element in the queue
         tmp2 = header + tmp1;       // Get the address of the element
         rc = put_operand( opcode, 1, OP_WR, (void *) &tmp2);
                                     // Store the address
         if (rc)
             return rc;
         tmp2 = 0;
         // Clear out the queue header, since the queue is empty
         rc = store_memory(header, (void *) &tmp2, 4);
         if (rc)
             return rc;
         rc = store_memory(header + 4, (void *) &tmp2, 4);
         if (rc)
             return rc;
         vax.pslw.z = 1;
         vax.pslw.v = 0;

         return VAX_OK;
     }

     // Normal case -- queue has more than one element

     rc = load_memory(header + tmp1, (void *) &tmp2, 4);
                     // Get the forward pointer from the first element
     if (rc)
         return rc;
     tmp3 = tmp1 + tmp2;     // Calculate the offset to second element
     rc = store_memory(header, (void *) &tmp3, 4);
                             // Store it in the header
     tmp2 = header + tmp3;   // Get the address of the second element
     tmp3 = -tmp3;           // Reverse the offset for the backpointer
     rc = store_memory(tmp2 + 4, (void *) &tmp3, 4);
                            // Store the backpointer
     if (rc)
         return rc;
     tmp2 = header + tmp1;   // Get the address of the element
     rc = put_operand( opcode, 1, OP_WR, (void *) &tmp2);
                                     // Store the address
     if (rc)
         return rc;

     vax.pslw.z = 0;
     vax.pslw.v = 0;

     return VAX_OK;


}




//  REMQTI - added by CMF

EMULATOR_ENTRY( emul_remqti )
{


    LONGWORD header, addr;
    LONGWORD tmp1, tmp2, tmp3, rc;

     /*      Get the parameters */

    header = opcode-> VAXaddr[ 0 ];
    addr = opcode-> VAXaddr[ 1 ];

    rc = load_memory(header + 4, (void *) &tmp1, 4);
    if (rc)
       return rc;

    vax.pslw.n = 0;
    vax.pslw.c = 0;

    if (tmp1 == 0) {                // Queue empty?
       vax.pslw.z = 1;
       vax.pslw.v = 1;
       return VAX_OK;
    }

    rc = load_memory(header, (void *) &tmp2, 4);
    if (rc)
       return rc;
    if (tmp1 == tmp2) {             // Only one element in the queue
       tmp2 = header + tmp1;       // Get the address of the element
       rc = put_operand( opcode, 1, OP_WR, (void *) &tmp2 );
                                   // Store the address
       if (rc)
           return rc;
       tmp2 = 0;
       // Clear out the queue header, since the queue is empty
       rc = store_memory(header, (void *) &tmp2, 4);
       if (rc)
           return rc;
       rc = store_memory(header + 4, (void *) &tmp2, 4);
       if (rc)
           return rc;
       vax.pslw.z = 1;
       vax.pslw.v = 0;
       
       return VAX_OK;
    }

    // Normal case -- queue has more than one element

    rc = load_memory(header + tmp1 + 4, (void *) &tmp2, 4);
                   // Get the back pointer from the last element
    if (rc)
       return rc;
    tmp3 = tmp1 + tmp2;     // Calculate the offset to 2nd-to-last element
    rc = store_memory(header + 4, (void *) &tmp3, 4);
                           // Store it in the header
    tmp2 = header + tmp3;   // Get the address of the 2nd-to-last element
    tmp3 = -tmp3;           // Reverse the offset for the forward pointer
    rc = store_memory(tmp2, (void *) &tmp3, 4);
                           // Store the forward pointer
    if (rc)
       return rc;
    tmp2 = header + tmp1;   // Get the address of the element
    rc = put_operand( opcode, 1, OP_WR, (void *) &tmp2 );
                           // Store the address
    if (rc)
       return rc;

    vax.pslw.z = 0;
    vax.pslw.v = 0;

    return VAX_OK;


}




//  REMQUE

EMULATOR_ENTRY( emul_remque )
{


    LONGWORD    entry, addr_succ, addr_pred;
    LONGWORD    rc;

    /*  Get the parameters */

    entry = opcode-> VAXaddr[ 0 ];


    /* Validate that all the memory references will work! */
    /* TBD */


    /*  Step 1. Fix forward link of our predecessor to point */
    /*          to our successor.                            */

    /*  Get address of our predecessor */

    rc = load_memory( entry+4, ( void * ) &addr_pred, 4 );
    if( rc )
        return rc;

    /*  Get address of our successor */

    rc = load_memory( entry,   ( void * ) &addr_succ, 4 );
    if( rc )
        return rc;

    vax.pslw.n = ( addr_succ < addr_pred );
    vax.pslw.z = ( addr_succ == addr_pred );
    vax.pslw.v = ( entry == addr_succ );
    vax.pslw.c = ( ( ULONGWORD ) addr_succ < ( ULONGWORD ) addr_pred );

    /*  Store our successor in our predecessor's forward link */

    rc = store_memory( addr_pred, ( void * ) &addr_succ, 4 );
    if( rc )
        return rc;


    /*  Step 2. Fix backward link of our successor to be    */
    /*          our predecessor.                            */


    rc = store_memory( addr_succ + 4, ( void * ) &addr_pred, 4 );
    if( rc )
        return rc;


    rc = put_operand( opcode, 1, OP_WR, ( void * ) &entry );

    return VAX_OK;

}



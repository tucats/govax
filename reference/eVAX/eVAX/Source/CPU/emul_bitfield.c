//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_bitfield.c
//
//  Purpose:    Emulator handlers for bit field instructions
//
//
//  History:    07/28/97    New header format standardization
//
//              05/27/99    Added EXT[Z]V, CMP[Z]V, FFS, FFC
//
//              05/28/99    Added INSV
//
//              06/11/99    Added BBC, BBS
//
//	        07/08/01    Fixed bugs in get_memory_bitfield where
//	                    bit array was returned completely wrongly.
//
//              12/04/01    Fixed bugs in get_memory_bitfield where
//                          bit array was returned incorrectly if 
//                          offset wasn't on a byte boundary.
//
//		12/06/01    Fixed bug in set_memory_bitfield where
//			    position number was shifted wrong amount,
//			    and data was (duh) never written out.

#include "vax.h"
#include "pte.h"


extern char * rom;  /* ROM memory */

//  Handler for BBSS, BBCS, BBSC, BBCC, BBSSI, BBCCI

EMULATOR_ENTRY( emul_bbstate )
{

    LONGWORD    pos, *pos_a, rc;
    unsigned char bits;
    LONGWORD    offset, vaxaddr;
    short   bit, test, set;
    short   f, is_register, fsize;
    unsigned char   * bits_a;
    
//  Get the position number

    pos_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( pos_a == 0L )
        return VAX_FAULT;
    pos = *pos_a;


//  Get the base address of the bit field

    bits_a = ( unsigned char * ) get_operand( opcode, 1, OP_MD );
    if( bits_a == 0L )
        return VAX_FAULT;


//  If the operand is in a register, offset within a big-endian model.

    vaxaddr = 0L;
    if( opcode-> VAXaddr[ 1 ] == 0L ) {
    
        is_register = 1;
        //  If the position is > 31 then it's a fault
        
        if( pos > 31 )
            return set_fault( EXC_RESOP, 0 );
        offset = pos / 8;
        bits_a -= offset;
        bits = *bits_a;
    }

//  No, it's from memory.  Offset withing a little-endian model.
//  For the code below to writes back to the memory to work, we
//  must resolve the vaxaddr to the physical location here and now.

//  Added (8/19/98) the call to the vm handler to resolve the addr.

    else {
    
        is_register = 0;
        offset = pos / 8;   /*  Which byte would this be in? */
        vaxaddr = opcode-> VAXaddr[ 1 ] + offset;
        fsize = 1;
        rc = vm( ( ULONGWORD * ) &vaxaddr, &fsize, 
                         VM_READ );
        if( rc )
            return rc;

        bits = *(PHYADDR( vaxaddr ));
    }
    
//  Now that we know exactly what byte we are manipulating, mask out the
//  position .


    pos = pos & 0x0007;
    

//  Get the specific bit.

    bit = ( short ) (( bits >> pos ) & 0x01);

//  Based on the operation, we must now arrange to update the bit.  The opcodes
//  are layed out like:
//
//  1 1 1 0   0 0 1 0       BBSS
//  1 1 1 0   0 0 1 1       BBCS
//  1 1 1 0   0 1 0 0       BBSC
//  1 1 1 0   0 1 0 1       BBCC
//  1 1 1 0   0 1 1 0       BBSSI
//  1 1 1 0   0 1 1 1       BBCCI
//
//  So find out what the test case and set cases are.  We don't support
//  multiprocessors so we ignore the INTERLOCK case as anything special.


    f = opcode-> function;
    
    if( f == 0xE6 )     /*  Convert BBSSI   */
        f =  0xE2;      /*  to BBSS         */
        
    if( f == 0xE7 )     /*  Convert BBCCI   */
        f =  0xE5;      /*  to BBCC         */
    
    test = ( short ) (( f & 0x01 ) ? 0 : 1);    /* bit to test for */
    set  = ( short ) (( f & 0x02 ) ? 1 : 0);    /* set value to    */


//  Set the new value and write back to memory.  The write is either to
//  a register or to the virtual VAX memory.  Note that the previous access
//  of the memory would have faulted for a modify operation, so at this
//  point we're guaranteed of a valid writable page.

    if( set )
        bits |= ( 1 << pos );
    else
        bits &= ~( 1 << pos );

    if( is_register )
        *bits_a = bits;
    else
        vax.memory[ vaxaddr ] = bits;

    
//  If the bit matches the test, set the new PC.

    if( test == bit ) {
        rc = load_byte( opcode-> VAXaddr[ 2 ], &bits );
        if( rc )
            return rc;
        vax.PC = opcode-> VAXaddr[ 2 ];   /* Third operand is branch */
                                          /* displacement addr */
    }

    return VAX_OK;
}




//  This routine collects up a bit field from a register.

LONGWORD get_register_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD * result_p )
{

    ULONGWORD result, result2;
    ULONGWORD mask, size2, size3;

    if( size > 32 ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    result = 0L;

/*
 *  For register operations, the value could be in just the first register, 
 *  or it could be in two adjacent registers.
 */

    if( position < 0 || position > 31 ||  
        base < 0 || base > 15 ||
        ( base == 15 && ( position + size > 31 ))) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }


    if( position + size < 32 ) {

        result = vax.reg[ base ];
        result = result >> position;

        mask = ( 1 << size ) - 1;
        result = result & mask;
    }

    else {

        size2 = ( position + size ) - 31;
        size3 = size - size2;

        result = vax.reg[ base ];
        result = result >> position;
        mask = ( 1 << size3 ) - 1;
        result = result & mask;

        result2 = vax.reg[ base + 1 ];
        mask = ( 1 << size2 ) - 1;
        result2 = result2 & mask;

        result = result | ( result2 << size2 );

    }


    *result_p = result;
    return VAX_OK;
}




//  This routine sets a bit field in a register given a longword containing the
//  bit field data.

LONGWORD set_register_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD data )
{

    ULONGWORD result;
    ULONGWORD mask, size2, size3;

    if( size > 32 ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    result = 0L;

/*
 *  For register operations, the value could be in just the first register, 
 *  or it could be in two adjacent registers.
 */

    if( position < 0 || position > 31 ||  
        base < 0 || base > 15 ||
        ( base == 15 && ( position + size > 31 ))) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }


    if( position + size < 32 ) {

        result = vax.reg[ base ];

        mask = (( 1 << size ) - 1 ) << position;

        result &= ~mask;
        result |= ( data << position );
        vax.reg[ base ] = result;
    }

    else {

        size2 = ( position + size ) - 31; /* bits in first register */
        size3 = size - size2;             /* bits in second register */

        /* Do the first register.  */

        result = vax.reg[ base ];
        mask = (( 1 << size2 ) - 1 ) << position;

        result &= ~mask;
        result |= ( data << position );

        vax.reg[ base ] = result;

        /* Do the second register */

        result = vax.reg[ base+1 ];
        mask = ( 1 << size3 ) - 1;

        result &= ~mask;
        result |= (( data >> size2 ) & mask );
        vax.reg[ base+1 ] = result;

    }

    return VAX_OK;
}






//  This routine gets a bit field given a position/size/base triplet.  The result
//  is written to a user-supplied longword.

LONGWORD get_memory_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD * result_p )
{

    ULONGWORD addr;
    ULONGWORD result;
    unsigned char byt;
    ULONGWORD byte_off, bit_off, byt_max, bpos, bit;
    LONGWORD rc, bcount;

    if( size > 32 ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    result = 0L;

/*
 *  First, use the position to adjust the base.
 */

    byte_off = position >> 3;  /* divide by 8 */
    bit_off  = position & 0x00000007;
    addr = (( ULONGWORD ) base ) + byte_off;

/*
 *  Start pulling in bits.
 */
 
    bpos = 0;
    byt_max = 8-bit_off;
    
    while( size ) {

        rc = load_memory( addr, &byt, 1 );
        if( rc )
            return rc;
        byt = (unsigned char ) ( byt >> bit_off );
        bcount = 0;

        while( size ) {
            bcount++;
            bit = byt & 0x01;
            result |= ((ULONGWORD) bit << bpos );
            bpos++;
            byt = ( unsigned char ) ( byt >> 1 );
            size--;
            if( bcount == byt_max )
                break;
        }
        bit_off = 0;  /* Start at first bit of next byte */
        byt_max = 8;  /* Read all bits of next byte */
        addr = addr + 1;

    }

    *result_p = result;
    return VAX_OK;
}






//  This routine sets a bit field in memomry given a position/size/base
//  triplet and a longword containing the bit data to be inserted.

LONGWORD set_memory_field( LONGWORD position, LONGWORD size,
                        LONGWORD base, LONGWORD data )
{

    ULONGWORD addr;
    ULONGWORD result;
    unsigned char byt;
    ULONGWORD bcount, mask;
    ULONGWORD byte_off, bit_off, bit;
    LONGWORD rc;

    if( size > 32 ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    result = 0L;

/*
 *  First, use the position to adjust the base.
 */

    /* How many bytes away from the base do we start?  */
    byte_off = position >> 3;  /* Divide by 8 */

    /* What bit position with that byte is the start?  */
    bit_off  = position & 0x00000007;

    /* Calculate a new starting address                */
    addr = (( ULONGWORD ) base ) + byte_off;
    rc = VAX_OK;
    
/*
 *  Start pulling in bytes of memory to set bits in them.
 */

    while( size ) {

        /* Get the byte we want to work on. */

        rc = load_memory( addr, &byt, 1 );
        if( rc )
            return rc;

        /* Start at the right bit (may be zero if this is nth byte) */

        bcount = bit_off;

        rc = 0;
        while( size ) {

            /* Get the low order bit left in the datum to set */
            bit = data & 0x00000001;
            data = data >> 1;

            /* Shift it to the right position in the byte, and */
            /* make a mask for that position.                  */
            bit = bit << bcount;
            mask = ( ULONGWORD ) ( 1 << bcount );

            /* Force clear the bit at that position, and set   */
            /* the new value.                                  */

            byt &= ~mask;
            byt |= bit;

            /* Move bit position, decrease size.  If we are at */
            /* the end of a byte, reset offset and address to  */
            /* start of next byte.                             */

            bcount++;
            size--;
            if( bcount == 8 ) {
                bcount = 0;
                /* Write the updated byte back out */
                rc = store_memory( addr, ( void * ) &byt, 1 );
                if( rc ) break;
                addr = addr + 1;
                break;
            }
        }
        
        /* If partial byte modified, write it out */
        
        if(( rc == VAX_OK ) && ( bcount ))
            rc = store_memory( addr, ( void * ) &byt, 1 );


    }

    return rc;
}






//  Utility routine to sign extend a bit field value.

ULONGWORD bit_sext( ULONGWORD data, LONGWORD size )
{

    ULONGWORD mask, result;
    LONGWORD sign;

    sign = data & ( 1 << ( size - 1 ));

//  This masks the data we want to keep.

    mask = ( 1 << size ) - 1;

//  Discard unwanted bits.

    result = ( data & mask );

//  If the sign was negative, construct a bit field
//  that fills in the rest of the bits as a 1.

    if( sign ) {
        sign = 0xFFFFFFFF;
        sign &= ~mask;
        result = ( data | sign );
    }

    return result;
}



EMULATOR_ENTRY( emul_bb )
{

    LONGWORD pos, result, size, base, test, rc;
    unsigned char bits;

//  Get the position number

    GET_OPERAND( pos, LONGWORD, 0, OP_RD );

//  The size is always 1

    size = 1;

//  The base can be a register or a memory address.  It can't be a generated
//  temporary.

    if( opcode-> is_register[ 1 ] == OP_TEMPORARY ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    if( opcode-> is_register[ 1 ] == OP_REGISTER ) {
        base = opcode-> regnum[ 1 ];
        rc = get_register_field( pos, ( LONGWORD ) size, base, &result );
        if( rc != VAX_OK )
            return rc;
    }
    else {
        base = opcode-> VAXaddr[ 1 ];
        rc = get_memory_field( pos, ( LONGWORD ) size, base, &result );
        if( rc != VAX_OK )
            return rc;
    }

//  If the bit matches the test, set the new PC.  E0 is BBS, E1 is BBC

    test = ( opcode->function == 0xE0 ) ? 1 : 0;

    if( result == test ) {
        vax.PC = opcode-> VAXaddr[ 2 ];   /* Third operand is disp. addr */
        rc = load_byte( opcode-> VAXaddr[ 2 ], &bits );
        if( rc )
            return rc;
    }

    return VAX_OK;
}




//  CMPV, CMPZV

EMULATOR_ENTRY( emul_cmpv )
{

    LONGWORD pos, * pos_a;
    char size, * size_a;
    ULONGWORD result, data, *data_a;
    LONGWORD base;
    LONGWORD rc;

//  Get the position number

    pos_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( pos_a == 0L )
        return VAX_FAULT;
    pos = *pos_a;


//  Get the size of the bit field

    size_a = ( char * ) get_operand( opcode, 1, OP_RD );
    if( size_a == 0L )
        return VAX_FAULT;
    size = *size_a;

//  The base can be a register or a memory address.  It can't be a generated
//  temporary.

    if( opcode-> is_register[ 2 ] == OP_TEMPORARY ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    if( opcode-> is_register[ 2 ] == OP_REGISTER ) {
        base = opcode-> regnum[ 2 ];
        rc = get_register_field( pos, ( LONGWORD ) size, base, 
        	( LONGWORD * ) &result );
        if( rc != VAX_OK )
            return rc;
    }
    else {
        base = opcode-> VAXaddr[ 2 ];
        rc = get_memory_field( pos, ( LONGWORD ) size, base, 
        	( LONGWORD * ) &result );
        if( rc != VAX_OK )
            return rc;
    }

//  Get the actual item to compare with.

    data_a = ( ULONGWORD * ) get_operand( opcode, 3, OP_RD );
    if( data_a == 0L )
        return VAX_FAULT;
    data = *data_a;

//  If this is CMPV instead of CMPZV, sign extend the bit field

    if( opcode-> function == 0xEC )
        result = bit_sext( result, size );

//  Set the condition bits according to spec and we're done.

    
    SETCONDITIONBITS( result, data );
    vax.pslw.v = 0;    

    return VAX_OK;

}



//  Handler for EXTV, EXTZV


EMULATOR_ENTRY( emul_extv )
{

    LONGWORD pos, * pos_a;
    char size, * size_a;
    LONGWORD result;
    LONGWORD base;
    LONGWORD rc;

//  Get the position number

    pos_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( pos_a == 0L )
        return VAX_FAULT;
    pos = *pos_a;


//  Get the size of the bit field

    size_a = ( char * ) get_operand( opcode, 1, OP_RD );
    if( size_a == 0L )
        return VAX_FAULT;
    size = *size_a;

//  The base can be a register or a memory address.  It can't be a generated
//  temporary.

    if( opcode-> is_register[ 2 ] == OP_TEMPORARY ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    if( opcode-> is_register[ 2 ] == OP_REGISTER ) {
        base = opcode-> regnum[ 2 ];
        rc = get_register_field( pos, ( LONGWORD ) size, base, &result );
        if( rc != VAX_OK )
            return rc;
    }
    else {
        base = opcode-> VAXaddr[ 2 ];
        rc = get_memory_field( pos, ( LONGWORD ) size, base, &result );
        if( rc != VAX_OK )
            return rc;
    }


//  If this is EXTV instead of EXTZV, sign extend the bit field

    if( opcode-> function == 0xEE )
        result = bit_sext( result, size );

//  Set the condition bits and write the result out.

    SETCONDITIONBITS( result, 0L );
    vax.pslw.v = 0;
    vax.pslw.c = 0;

    return put_operand( opcode, 3, OP_WR, ( char * ) &result );

}





//  Handler for INSV


EMULATOR_ENTRY( emul_insv )
{

    LONGWORD pos, * pos_a;
    char size, * size_a;
    LONGWORD data, *data_a;
    LONGWORD base;
    LONGWORD rc;

//  Get the data containing the bits to insert

    data_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( data_a == 0L )
        return VAX_FAULT;
    data = *data_a;


//  Get the position number

    pos_a = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( pos_a == 0L )
        return VAX_FAULT;
    pos = *pos_a;


//  Get the size of the bit field

    size_a = ( char * ) get_operand( opcode, 2, OP_RD );
    if( size_a == 0L )
        return VAX_FAULT;
    size = *size_a;


//  The base can be a register or a memory address.  It can't be a generated
//  temporary.

    if( opcode-> is_register[ 3 ] == OP_TEMPORARY ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    if( opcode-> is_register[ 3 ] == OP_REGISTER ) {
        base = opcode-> regnum[ 3 ];
        rc = set_register_field( pos, ( LONGWORD ) size, base, data );
    }
    else {
        base = opcode-> VAXaddr[ 3 ];
        rc = set_memory_field( pos, ( LONGWORD ) size, base, data );
    }

    return rc;

}




//  Handler for FFS, FFC

EMULATOR_ENTRY( emul_ff )
{

    LONGWORD pos, * pos_a;
    char size, * size_a;
    LONGWORD base;
    LONGWORD rc, n;
    ULONGWORD testbit, result;

    n = vax.PC;
    
//  Get the position number

    pos_a = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( pos_a == 0L )
        return VAX_FAULT;
    pos = *pos_a;


//  Get the base address of the bit field

    size_a = ( char * ) get_operand( opcode, 1, OP_RD );
    if( size_a == 0L )
        return VAX_FAULT;
    size = *size_a;

//  The base can be a register or a memory address.  It can't be a generated
//  temporary.

    if( opcode-> is_register[ 2 ] == OP_TEMPORARY ) {
        set_fault( EXC_RESOP, 0 );
        return VAX_FAULT;
    }

    if( opcode-> is_register[ 2 ] == OP_REGISTER ) {
        base = opcode-> regnum[ 2 ];
        rc = get_register_field( pos, ( LONGWORD ) size, 
                                base, ( LONGWORD * ) &result );
        if( rc != VAX_OK )
            return rc;
    }
    else {
        base = opcode-> VAXaddr[ 2 ];
        rc = get_memory_field( pos, ( LONGWORD ) size, 
                                base, ( LONGWORD * ) &result );
        if( rc != VAX_OK )
            return rc;
    }


//  See if we are looking for set or clear...  Also, set up the
//  condition bits not affected by the results.

    testbit = ( opcode-> function == 0xEB ) ? 0 : 1;    /* FFC or FFS */

    vax.pslw.v = 0;
    vax.pslw.c = 0;
    vax.pslw.n = 0;

//  Size of zero is a special case

    if( size == 0 ) {
        vax.pslw.z = 1;
        return put_operand( opcode, 3, OP_WR, ( char * ) &pos );
    }

//  Search the result field to find the first bit that matches

    for( n = 0; n < ( LONGWORD ) size; n++ ) {
        if(( result & 0x00000001 ) == testbit ) {
            vax.pslw.z = 0;
            n = n + pos;
            return put_operand( opcode, 3, OP_WR, ( char * ) &n );
        }
        result = ( result >> 1 ) & ( 0x7FFFFFFF);
    }

//  Not found, so return next position, etc.

    vax.pslw.z = 1;
    n = pos + ( LONGWORD ) size;
    return put_operand( opcode, 3, OP_WR, ( char * ) &n );

}

//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_mov.c
//
//  Purpose:    Emulator handlers for MOV* instructions
//
//              The strategy is about the same for all data types.
//
//              1.  Get addressability to the source memory or register.
//
//              2.  Read the data into temporary store.
//
//              3.  Set the condition bits correctly based on the value.
//
//              4.  Write the value to the output operand
//
//  History:    07/28/97    New header format standardization
//
//              09/07/98    Added MOVF, MNEGF
//
//              01/27/00    Performance tweak.  MOVL is used so often that I am removing
//                          the special cases for MCOM and MNEG variants and putting them
//                          in their own emul_movl_negated() handler to speed up the
//                          most common case.  Same for MOVW and MOVB versions.
//
//		07/06/01    MOVQ was not writing MSL/LSL longwords in correct order when
//			    destination was register pair.
//		10/08/01    PSL<V> and PSL<C> not being set right for MNEGx instructions.
//			    PSL<C> was getting cleared on MOVx instructions.

#include "vax.h"
#include "fpu.h"



EMULATOR_ENTRY( emul_movzbl )
{
    unsigned char src;
    ULONGWORD dst;

    GET_OPERAND( src, unsigned char, 0, OP_RD );

    dst = ( LONGWORD ) src;
    vax.pslw.n = 0;
    vax.pslw.z = ( dst == 0 ) ? 1 : 0;
    vax.pslw.v = 0;

    return put_operand( opcode, 1, OP_WR, ( void * ) &dst );
}




EMULATOR_ENTRY( emul_movzbw )
{
    unsigned char src;
    unsigned short dst;

    GET_OPERAND( src, unsigned char, 0, OP_RD );

    dst = ( short ) src;
    vax.pslw.n = 0;
    vax.pslw.z = ( dst == 0 ) ? 1 : 0;
    vax.pslw.v = 0;

    return put_operand( opcode, 1, OP_WR, ( void * ) &dst );
}




EMULATOR_ENTRY( emul_movzwl )
{
    unsigned short src;
    ULONGWORD dst;

    GET_OPERAND( src, unsigned short, 0, OP_RD );

    dst = ( LONGWORD ) src;
    vax.pslw.n = 0;
    vax.pslw.z = ( dst == 0 ) ? 1 : 0;
    vax.pslw.v = 0;

    return put_operand( opcode, 1, OP_WR, ( void * ) &dst );
}



EMULATOR_ENTRY( emul_movb )
{


    char * src;
    unsigned char data;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the byte value.  Note that the pointer is a pointer to the
    //  specific byte, even if it's in a virtual register... don't scale.
    
    data = *src;
    vax.pslw.v = 0;

    
    //  Set the condition bits
    
    vax.pslw.n = (( signed char ) data < 0 );
    vax.pslw.z = (data == 0);
    vax.pslw.v = 0;
    
    //  Write the data back to storage/register
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}



EMULATOR_ENTRY( emul_movb_negated )
{


    char * src;
    char data;
    unsigned char udata;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the byte value.  Note that the pointer is a pointer to the
    //  specific byte, even if it's in a virtual register... don't scale.
    
    data = *src;
    udata = data;
    vax.pslw.v = 0;

    //  If it's the MCOM or MNEG version, deal with it.
    
    if( opcode-> function == 0x92 )
        data = ( unsigned char ) ~data;
    else
    if( opcode-> function == 0x8E ) {
        udata = data;
        vax.pslw.c = ( data < 0 ) ? 1 : 0;
        if( udata == 0x80L )
            vax.pslw.v = 1;
        else
            data = ( unsigned char ) ( -data );
    }
    
    //  Set the condition bits
    
    vax.pslw.n = ( data < 0 );
    vax.pslw.z = ( data == 0 );
    vax.pslw.v = 0;
    
    //  Write the data back to storage/register
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}


EMULATOR_ENTRY( emul_movw )
{


    short * src;
    short data;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( short * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the word value.  Note that the pointer is a pointer to the
    //  specific word, even if it's in a virtual register... don't scale.
    
    vax.pslw.v = 0;
    data = *src;
        
    //  Set the condition bits
    
    vax.pslw.n = (data < 0 );
    vax.pslw.z = (data == 0);
    vax.pslw.v = 0;
    
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );

}


EMULATOR_ENTRY( emul_movw_negated )
{


    short * src;
    short data;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( short * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the word value.  Note that the pointer is a pointer to the
    //  specific word, even if it's in a virtual register... don't scale.
    
    vax.pslw.v = 0;
    data = *src;

    //  If it's the MCOM or MNEG version, deal with it.
    
    if( opcode-> function == 0xB2 )
        data = ( short ) ~data;
    else
    if( opcode-> function == 0xAE ) {
        vax.pslw.c = ( data < 0 ) ? 1 : 0;
        if( (unsigned short ) data == 0x8000 )
            vax.pslw.v = 1;
        else
            data = ( short ) (-data);
    }
        
    //  Set the condition bits
    
    vax.pslw.n = ( data < 0 );
    vax.pslw.z = ( data == 0 );
    
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );

}


EMULATOR_ENTRY( emul_movl )
{


    LONGWORD * src;
    LONGWORD data;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the longword value
    
    data = *src;
    
    //  Set the condition bits
    
    vax.pslw.n = (data < 0 );
    vax.pslw.z = (data == 0);
    vax.pslw.v = 0;
    
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}


EMULATOR_ENTRY( emul_movl_negated )
{


    LONGWORD * src;
    LONGWORD data;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the longword value
    
    data = *src;
    vax.pslw.v = 0;

    //  Based on MCOML or MNEGL, do the right kind of negation
    
    if( opcode-> function == 0xD2 )
        data = ~data;
    else
    if( opcode-> function == 0xCE ) {
        vax.pslw.c = ( data < 0 ) ? 1 : 0;
        if( data == 0x80000000 )
            vax.pslw.v = 1;
        else
            data = -data;
    }
    
    //  Set the condition bits
    
    vax.pslw.n = ( data < 0 );
    vax.pslw.z = ( data == 0 );
    
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}



EMULATOR_ENTRY( emul_movf )
{


    LONGWORD * src, rc;
    LONGWORD data;
    double dbl;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the longword value
    
    data = *src;
    vax.pslw.v = 0;

    //  Convert to natural double
    
    rc = fpu_load( data, 0L, &dbl );
    if( rc )
        return rc;

    //  If it's the MNEGF version, handle it.
    
    if( opcode-> function == 0x52 )
        dbl = -dbl;
    
    //  Set the condition bits
    
    vax.pslw.n = ( dbl < 0.0 ) ? 1 : 0;
    vax.pslw.z = ( dbl == 0.0 ) ? 1 : 0;
    
    //  Convert back to a 4-byte f_float
    
    rc = fpu_store( dbl, &data, 0L /* F_FLOAT */ );
    if( rc )
        return rc;
        
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}


EMULATOR_ENTRY( emul_movd )
{


    LONGWORD * src, rc;
    LONGWORD data[2];
    double dbl;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the longword value
    
    data[0] = src[0];  /* Endian sensitivity? */
    data[1] = src[1];
    
    vax.pslw.v = 0;

    //  Convert to natural double
    
    rc = fpu_load( data[0], data[1], &dbl );
    if( rc )
        return rc;

    //  If it's the MNEGD version, handle it.
    
    if( opcode-> function == 0x72 )
        dbl = -dbl;
    
    //  Set the condition bits
    
    vax.pslw.n = ( dbl < 0.0 ) ? 1 : 0;
    vax.pslw.z = ( dbl == 0.0 ) ? 1 : 0;
    
    //  Convert back to a 4-byte f_float
    
    rc = fpu_store( dbl, &(data[0]), &(data[1]) );
    if( rc )
        return rc;
        
    //  Write the data back to memory
    
    return put_operand( opcode, 1, OP_WR, ( char * ) data );


}



EMULATOR_ENTRY( emul_movq )
{

    QUADWORD data;
    QUADWORD * src;
    LONGWORD ls_long, ms_long;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src = ( QUADWORD * ) get_operand( opcode, 0, OP_RD );
    if( src == 0L )
        return VAX_FAULT;
        
    //  Get the longword value
    
    data = *src;
    vax.pslw.v = 0;

    
    //  Set the condition bits
    
    SETCONDITIONBITS( data, 0L );
    
    //  Write the data back to memory
    
    //  If it's a register, we have to split this up.

    if( opcode-> is_register[ 1 ]) {
    
         ls_long = data & 0xFFFFFFFFL;
         ms_long = ( data >> 32 ) & 0xFFFFFFFFL;
         
         vax.reg[ opcode-> regnum[ 1 ]] = ls_long;
         vax.reg[ opcode-> regnum[ 1 ]+1] = ms_long;
         return VAX_OK;
        
    }
    //  not to register, so swap halves so put_operand works right
    
    return put_operand( opcode, 1, OP_WR, ( char * ) &data );


}

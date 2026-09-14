//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     emul_cmp.c
//
//  Purpose:    Emulator handlers for numeric compare instructions
//
//              This single routine handles all numeric compare instructions.  Floating
//              and packed data will be handled elsewhere.  This routine examines the
//              opcode to determine what the data size and type is.
//
//  History:    07/28/97    New header format standardization
//              02/03/98    Added TSTx and BITx instructions.
//              09/08/98    Added CMPF and TSTF instructions.
//              09/27/99    Tweaks to make system 64-bit friendly.
//



#include "vax.h"
#include "fpu.h"


EMULATOR_ENTRY( emul_cmp )
{


    char * src1, *src2;
    
    LONGWORD data, d1, d2;
    unsigned char op, dsize;
    LONGWORD opcnt;
    LONGWORD func, rc, cbit;
    QUADWORD zero;
    double float1, float2, float3;
    
    /*
        The instructions are structured like this:
         
         0 1 0 1  0 0 0 1       CMPF
         0 1 1 1  0 0 0 1       CMPD
         1 0 0 1  0 0 0 1       CMPB
         1 0 0 1  0 0 1 1       BITB
         1 0 0 1  0 1 0 1       TSTB
         1 0 1 1  0 0 0 1       CMPW
         1 0 1 1  0 0 1 1       BITB
         1 0 1 1  0 1 0 1       TSTB
         1 1 0 1  0 0 0 1       CMPL
         1 1 0 1  0 0 1 1       BITL
         1 1 0 1  0 1 0 1       TSTL
         
        
        So let's decompose the opcode to find out what data type
        for this instruction is.
        
     */

    op = opcode-> function;
    opcnt = ( op >> 2 ) & 0x01 ? 1 : 2;
    
    dsize = ( op >> 5 );                /*  2 = F, 3 = D, 4 = B, 5 = W, 6 = L       */
    func =  ( op >> 1 ) & 0x0007;       /*  0 = CMP, 1 = BIT, 2 = TST   */
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src1 = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src1 == 0L )
        return VAX_FAULT;
    
    
    if( opcnt > 1 ) {
        src2 = ( char * ) get_operand( opcode, 1, OP_RD );
        if( src2 == 0L )
            return VAX_FAULT;
    }
    else {
        zero = 0L;          /* 8-byte field of zeros */
        src2 = ( char * ) &zero;
    }
    
    

    /* Based on data size, we must fetch right-sized variables */
    
    switch( dsize ) {
    
    case 2: /* F_FLOAT */
    
        d1 = *(( LONGWORD * ) src1 );
        rc = fpu_load( d1, 0L, &float1 );
        if( rc )
            return rc;
        
        d2 = *(( LONGWORD * ) src2 );
        rc = fpu_load( d2, 0L, &float2 );
        if( rc )
            return rc;
        
        break;
        
    case 4: /* Byte */
    
        d1 = *(( char * ) src1 );
        d2 = *(( char * ) src2 );
        break;
    
    
    case 5: /* Word */
            
        d1 = *(( short * ) src1 );
        d2 = *(( short * ) src2 );
        
        break;
    
    case 6: /* Longword */
    
        d1 = *(( LONGWORD * ) src1 );
        d2 = *(( LONGWORD * ) src2 );
        break;
            
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    
    /* Based on operation, do the right thing with them */
    
    cbit = 0;
    
    switch( func ) {
    
    case 0: /* CMP, subtract two values */
    
        if( dsize > 3 ) {   /* Integer operation */
            data = d1 - d2;
            cbit = (( ULONGWORD ) d1 < ( ULONGWORD ) d2 ) ? 1 : 0;
        }
        else {
            float3 = float1 - float2;   /* Do float compare */
            
            if( float3 < 0.0 )          /* Make integer result */
                data = -1;
            else
            if( float3 == 0.0 )
                data = 0;
            else
                data = 1;
        }
        break;
    
    case 1: /* BIT, do a bit test of the values */
    
        data = d2 & d1;
        break;
    
    case 2: /* TST, just compare data against zero */
    
        if( dsize > 3 ) {   /* Integer operation */
            data = d1;
        }
        else {              /* Float operation */
            
            if( float1 < 0.0 )          /* Make integer result */
                data = -1;
            else
            if( float1 == 0.0 )
                data = 0;
            else
                data = 1;
        
        }
        break;
        
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    }
    
    SETCONDITIONBITS( data, 0L );
    
    vax.pslw.c = cbit;
    vax.pslw.v = 0;

    return VAX_OK;
}

/*
 *  Optimized version for longwords.
 */

EMULATOR_ENTRY( emul_cmpl )
{


    LONGWORD * src1, *src2;
    
    LONGWORD data, d1, d2;
    LONGWORD cbit;
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src1 = ( LONGWORD * ) get_operand( opcode, 0, OP_RD );
    if( src1 == 0L )
        return VAX_FAULT;
    
    
    src2 = ( LONGWORD * ) get_operand( opcode, 1, OP_RD );
    if( src2 == 0L )
        return VAX_FAULT;
    

    d1 = *(( LONGWORD * ) src1 );
    d2 = *(( LONGWORD * ) src2 );
    
    data = d1 - d2;
    cbit = (( ULONGWORD ) d1 < ( ULONGWORD ) d2 ) ? 1 : 0;
    
    SETCONDITIONBITS( data, 0L );
    
    vax.pslw.c = cbit;
    vax.pslw.v = 0;

    return VAX_OK;
}



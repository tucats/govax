//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     emul_float_math.c
//
//  Purpose:    Emulator handlers for floating ponit math instructions
//
//              This single routine handles all F_FLOAT and D_FLOAT math
//              instructions. This routine examines the opcode to determine
//              what the data size and type is, and whether to write back
//              to the second or third operand (ADDF2 vs. ADDF3, etc.).
//
//  History:    09/07/98        Initial implementation.
//
//              10/15/99        Fix bug is selection of integer data size
//                              for conversions.
//
//		12/07/01        Fixed bug where D_FLOAT conversion was written
//				in wrong order (obscured by bug in put_operand
//				that reversed it back).


#include "vax.h"
#include "fpu.h"


EMULATOR_ENTRY( emul_float_math )
{


    char * src1, *src2;
    
    LONGWORD d1, d2, rc, d[2];
    double float1, float2, float3;

    unsigned char op, dsize, dsize2, opcnt, func;
    
    float3 = 0.0;
	dsize2 = 0;
	
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src1 = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src1 == 0L )
        return VAX_FAULT;
    
    
    src2 = ( char * ) get_operand( opcode, 1, OP_RD );
    if( src2 == 0L )
        return VAX_FAULT;
    
    /*
        The instructions are structured like this:
    
         0 1 0 0  - - - -        ---F-
         0 1 0 0  0 0 0 0        ADDF2 
         0 1 0 0  0 0 0 1        ADDF3 
         0 1 0 0  0 0 1 0        SUBF2
         0 1 0 0  0 0 1 1        SUBF3
         0 1 0 0  0 1 0 0        MULF2
         0 1 0 0  0 1 0 1        MULF3
         0 1 0 0  0 1 1 0        DIVF2
         0 1 0 0  0 1 1 1        DIVF3
         
         0 1 0 0  1 0 0 0        CVTFB
         0 1 0 0  1 0 0 1        CVTFW
         0 1 0 0  1 0 1 0        CVTFL
         0 1 0 0  1 0 1 1        CVTRFL
         0 1 0 0  1 1 0 0        CVTBF
         0 1 0 0  1 1 0 1        CVTWF
         0 1 0 0  1 1 1 0        CVTLF
         0 1 0 0  1 1 1 1        ACBF  Supported in emul_branch !
         
         0 1 1 0  - - - -       ---D-
         
        So let's decompose the opcode to find out what the operations
        are that we'll do, and we can overload this routine with all
        the operations.
     */

    op = opcode-> function;
    
    dsize = ( op >> 5 );        /*  2 = F, 3 = D */
    opcnt = ( op & 0x01 ) + 2;  /*  2 or 3 */
    func  = ( op >> 1 ) & 0x07; /*  0 = ADD, 1 = SUB, 2 = MUL, 3 = DIV  */

    /* If the function is >= 4 then it's a conversion, remap them. */
    
    /*  func = 4 | 5     convert from float to integer  */
    /*  func = 6 | 7     convert from integer to float  */
    
    if( func == 4 || func == 5 ) {
        func = 4;               /* convert from float to integer */
        dsize2 = op & 0x03;     /* 0 = B, 1 = W, 2 = L, 3 = RL   */
    }
    else
    if( func == 6 || func == 7 ) {
        func = 5;               /* convert from integer to float */
        dsize2 = op & 0x03;     /* 0 = B, 1 = W, 2 = L, 3 = RL   */
        if(( op & 0x0F ) == 0x0F )
        func = 6;               /* Special case of ACBx */
    }
    
    
    /* Handle conversions separately from math operations */
    
    if( func == 4 ) {           /* convert from float to integer */
    
    
        /* Get the F_FLOAT operand */
        
        d1 = *(( LONGWORD * ) src1 );
        rc = fpu_load( d1, 0L, &float1 );
        if( rc )
            return rc;

        switch( dsize2 ) {
        
        case 0:     /* Byte */
        
                    if( float1 < -128.0 || float1 > 127.0 ) {
        
                        return set_fault( EXC_ARITH, 1, TRAP_INT_OVF ); 
                    }
                    
                    d1 = ( char ) float1;
                    break;
            
        case 1:     /* Word */

                    if( float1 < -32768.0 || float1 > 32767.0 ) {

                        return set_fault( EXC_ARITH, 1, TRAP_INT_OVF );
                    }

                    d1 = ( short ) float1;
                    break;

        case 2:     /* Long */

                    if( float1 < -2147483648.0 || float1 > 2147483647.0 ) {

                        return set_fault( EXC_ARITH, 1, TRAP_INT_OVF );
                    }

                    d1 = ( LONGWORD ) float1;
                    break;

        default:
                    set_fault( EXC_PRIV, 0 );
                    return VAX_FAULT;
        }
        
        SETCONDITIONBITS( d1, 0L );
        
        return put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &d1 );

    }

    if( func == 5 ) {           /* convert from integer to float */

        switch( dsize2 ) {
        
        case 0:     /* Byte */
        
                    d1 = *(( char * ) src1 );
                    break;
            
        case 1:     /* Word */
        
                    d1 = *(( short * ) src1 );
                    break;
            
        case 2:     /* Long */
        
                    d1 = *(( LONGWORD * ) src1 );
                    break;

        default:
                    set_fault( EXC_PRIV, 0 );
                    return VAX_FAULT;
        }
        
        SETCONDITIONBITS( d1, 0L );
        
        float1 = d1;
        if( dsize ==2  ) {
            rc = fpu_store( float1, &d1, 0L /* F_FLOAT */ );
            if( rc )
                return rc;
            return put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &d1 );
        }
        else {  /* D_FLOAT */
            rc = fpu_store( float1, &(d[1]),&(d[0]) ); /* KLUDGE? */
            if( rc )
                return rc;
            return put_operand( opcode, opcode-> count - 1,OP_WR, ( char * ) d );
        }
    }

    
    /* It's basic math.  Based on data size, we must manipulate right-sized variables */
    
    switch( dsize ) {
    
    case 2: /* F_FLOAT */
    case 3: /* D_FLOAT */  /* Both same except for memory read/write */
    
        if( dsize == 2 ) {
            d1 = *(( LONGWORD * ) src1 );
            d2 = *(( LONGWORD * ) src2 );
            
            rc = fpu_load( d1, 0L, &float1 );
            if( rc )
                return rc;
            rc = fpu_load( d2, 0L, &float2 );
            if( rc )
                return rc;
        }
        else {
            d[0] = ((LONGWORD *) src1 )[0];
            d[1] = ((LONGWORD *) src1 )[1];
            
            /* Is this ENDIAN sensitive? */
            rc = fpu_load( d[1],d[0],&float1 );
            if( rc ) return rc;
            
            d[0] = ((LONGWORD *) src2 )[0];
            d[1] = ((LONGWORD *) src2 )[1];
            
            /* Is this ENDIAN sensitive? */
            rc = fpu_load( d[1],d[0],&float2 );
            if( rc ) return rc;
        }
        
        switch( func ) {
        
        case 0:  float3 = float2 + float1;
                 break;
                 
        case 1:  float3 = float2 - float1;
                 break;
                 
        case 2:  float3 = float2 * float1;
                 break;
                 
        case 3:  float3 = float2 / float1;
                 break;
                 
        }
        
        vax.pslw.z = ( float3 == 0.0 ) ? 1 : 0;
        vax.pslw.n = ( float3 <  0.0 ) ? 1 : 0;
        
        if( dsize == 2  ) {
                rc = fpu_store( float3, &d1, 0L /* F_FLOAT */ );
                if( rc )
                    vax.pslw.v = 1;
                return put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &d1 );
            }
            else {  /* D_FLOAT */
                rc = fpu_store( float3, &(d[0]),&(d[1]) );
                if( rc )
                    return rc;
                return put_operand( opcode, opcode-> count - 1,OP_WR, ( char * ) d );
            }
        
         break;
    

    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    


    return VAX_OK;
}



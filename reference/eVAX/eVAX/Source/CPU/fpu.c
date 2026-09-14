//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     fpu.c
//
//  Purpose:    This routine contains the general handlers for floating point
//              math.  They operate on a data type of struct FLOAT which is
//              essentially a D_FLOAT value decomposed into it's constituent
//              bit-fields.
//
//
//  History:    07/28/97    New header format standardization
//
//              09/07/98    Implemented first _working_ version, based on the
//                          floating conversions done by Larry Noe.  Note that
//                          this will work flawlessly for F_FLOAT, but means
//                          that (currently) D_FLOAT values may be inaccurate
//                          in the last 3 bits.
//
//              10/17/99    Fixed bugs in conversions on little-endian ports.
//


#include "vax.h"
#include "fpu.h"

/*
 *  Which longword in a double contains which part of the value?
 */

#if BIGENDIAN 
#define MSB 1
#define LSB 0

#else
#define MSB 0
#define LSB 1
#endif

/*
 *  FPU_STORE()
 *
 *  Given a native double, create a pair of longwords that are the D_FLOAT data
 *  representation.  The second float is the least significant for the D_FLOAT
 *  representation.  Therefore, if you really just want F_FLOAT, then pass a
 *  null pointer as the second parameter to get only a 4-byte float.
 */

LONGWORD fpu_store( double source, LONGWORD * dst1, LONGWORD * dst2  ) 
{

    ULONGWORD * pd, *pv;
    double local;
    LONGWORD dfloat[ 2 ], dfloat2[ 2 ];
    unsigned char * p_src, *p_dst;
    int len;
    
    LONGWORD expon;

/*
 *  What size data are we working on here?
 */
 
    if( dst2 == 0L )
        len = 4;
    else
        len = 8;
        
/*
 *  Short circuit zero.
 */

    if( source == 0.0 ) {
        *dst1 = 0;
        if( len == 8 )
            *dst2 = 0;
        return VAX_OK;
    }

/*
 *  Set up for the awful conversions.
 */
 
    local = source;
    pd = ( ULONGWORD * ) &source;

    pv = ( ULONGWORD * ) dfloat;
    
/*
 *  Let's see how big the exponent is.  VAX can only be -127 to +127.
 */

    /* Mask out irrelevant bits */
    
    expon = pd[ LSB /* 0 */ ] & 0x7FF00000;
    
    /* Slide into lower short */
    
    expon = expon >> 16;
    
    /* Strip off lowest 4 bits */
    
    expon = expon >> 4;
    
    /*  Convert from IEEE to decimal */
    
    expon = ( expon - 0x03FF ) + 1;

    /*  If underflow, then fault if PSL[FU] else assume zero */
    
    if( expon < -127 ) {
        if( vax.pslw.fu )
            return set_fault( EXC_ARITH, 1, FAULT_FLT_UND );
        else {
            local = 0.0;
            expon = 0;
        }
    }

overflow:

    if( expon > 127 )
        return set_fault( EXC_ARITH, 1, FAULT_FLT_OVF );

/*
 *  Make the exponent be excess 128.
 */
 
    expon += 128;
    
    dfloat[ 0 ] = dfloat[ 1 ] = 0L;
 
    pv[ MSB ] = ( pd[ LSB /* 0 */ ] & 0x80000000 ) |          /* MSB */
                ((pd[ LSB /* 0 */ ] & 0x000FFFFF ) << 3 ) |   /* MSB */
                ((expon & 0x000000FF ) << 23 );
    
    pv[ MSB ] |= (pd[ MSB /* 1 */ ] & 0xE0000000) >> 29;      /* LSB */
    
    pv[ LSB ] = pd[ MSB /* 1 */ ] << 3;                       /* LSB */
 
 /*
  * Have to handle rounding now if we are working on a 4-byte value,
  * to emulate the way that the VAX hardware converts 8-byte to 4-byte
  * values.
  */
  
    if( len == 4 ) {
    
        unsigned int incr, ovfl;
        ovfl = 0;
        
        ovfl = pv[ LSB ] & 0x80000000;
        incr = ( unsigned int ) ( 1 << (((4)<<3)-1));
        pv[ LSB ] += incr;
        incr = ( ovfl && !(pv[ LSB ] & 0x80000000));
    
        ovfl = pv[ MSB ] & 0x80000000;
        pv[ MSB ] += incr;      
        
        if(( ovfl && !(pv[ MSB ] & 0x80000000)) ||
           (!ovfl &&  (pv[ MSB ] & 0x80000000)) )
           {
                expon = 255;
                goto overflow;
            }
    }
    
 /*
  * Result is now in dfloat[] array.  Do the wierd byte inversion.
  */
 
    p_src = ( unsigned char * ) dfloat;
    p_dst = ( unsigned char * ) dfloat2;

    p_dst[ 0 ] = p_src[ 6 ];
    p_dst[ 1 ] = p_src[ 7 ];
    p_dst[ 2 ] = p_src[ 4 ];
    p_dst[ 3 ] = p_src[ 5 ];
    p_dst[ 4 ] = p_src[ 2 ];
    p_dst[ 5 ] = p_src[ 3 ];
    p_dst[ 6 ] = p_src[ 0 ];
    p_dst[ 7 ] = p_src[ 1 ];

    *dst1 = dfloat2[ LSB /* 0 */ ];
    if( len == 8 )
        *dst2 = dfloat2[ MSB /* 1 */ ];
    
    
    return VAX_OK;
}


/*
 *  FPU_LOAD()
 *
 *  Given a pair of 32 bit values, convert them as a D_FLOAT datum to a native
 *  double.  If it's an F_FLOAT value, just pass zero as the src2 LSB value.
 *
 */

LONGWORD fpu_load( LONGWORD src1, LONGWORD src2, double * result )
{

    double local, local2;
    unsigned char * pvax;
    ULONGWORD *hvax, *hvax2;
    
    char * source;
    LONGWORD source_data[ 2 ];
    
    int expon, tmp;
    int holdlen, len;
    
    source_data[ LSB /* 0 */ ] = src1;
    source_data[ MSB /* 1 */ ] = src2;
    source = ( char * ) source_data;
    
    local = local2 = 0.0;
    hvax = ( ULONGWORD * ) &local;
    hvax2 = ( ULONGWORD * ) &local2;
    holdlen = len = 8;

/*
 *  This may be bogus.  For some reason the code is sure that the 16-bit values
 *  must be ungrouped this way.
 */
 
    for( pvax = ( unsigned char * ) hvax; len >  1; len -= 2, source += 2 ) {
        *pvax++ = ( unsigned char ) *(source+1);
        *pvax++ = ( unsigned char ) *source;
    }

    if( len )
        *pvax = (unsigned char )*source;

    pvax = ( unsigned char * ) hvax;
    for( len = 0; len < 4; len++ ) {
    
        tmp = pvax[ len ];
        pvax[ len ] = pvax[ 7-len ];
        pvax[ 7-len ] = ( char ) tmp;
    }
    

/*
 *  Capture the exponent.  Convert from excess 128 to decimal.  The convert to IEEE.
 */

    expon = (hvax[ MSB ] & 0x7F800000) >> 23;

    /* If the exponent is zero and the sign bit is zero, assume 0.0 */
    
    if( expon == 0 ) {
        if(( hvax[ MSB ] & 0x7FFFFFFF ) == 0x00000000 ) {
            *result = 0.0;
            return VAX_OK;
        }
        
        /* If exponent zero and sign bit 1, then reserved op fault */
        
        else
            return set_fault( EXC_RESOP, 0 );
    }
    
    expon = expon - 129;

    expon = expon + 1023;

/*
 *  Untangle the mantissa.  Swap the longwords while we're at it into the
 *  alternate buffer.
 */
 
    hvax2[ MSB ] = ((hvax[ MSB ] & 0x00000007) << 29) | (hvax[ LSB ] >> 3 );
    hvax2[ LSB ] = ( hvax[ MSB ] & 0x80000000) |
                  ((hvax[ MSB ] & 0x007FFFFF) >> 3 ) | ( expon << 20 );
    
    *result = local2;
    return VAX_OK;
 }
 

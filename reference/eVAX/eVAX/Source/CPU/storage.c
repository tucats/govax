//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     storage.c
//
//  Purpose:    Interface to main memory for the CPU and the console processor.
//              Reads and writes to memory, or moves to and from a CPU register
//              are handled here.  These routines are responsible for virtual
//              memory translations!
//
//  History:    07/28/97    New header format standardization
//
//              08/19/98    Added full VM support
//
//              12/09/98    Added endian-sensitivity
//
//              07/13/99    Added VAX memory-to-native-structure mapping routines.
//
//              07/13/99    Fixed bug in getting a quadword operand 
//                          (tregs backwards)
//
//              11/09/99    Fixed bug where a read or write that spans pages 
//                          that are not contiguous in physical memory fails.
//
//              03/17/00    Performance tweaks in load_io and store_io usage.
//
//              11/05/01    Fix problems with writing quadwords to memory.
//
//		12/07/01    Fixed additional problems with writing quadwords
//			    to memory (order reversal issue paired with cvtld bug).
//
//		03/01/02    Fix problem when two quadword temporaries are used in
//			    same instruction; longword register overloaded.

#include "vax.h"
#include "pte.h"
#include "memmap.h"
#include "console_proto.h"

//  Watchpoint support

struct WATCHPOINT {
    struct WATCHPOINT * next;
    ULONGWORD	address;
    short 		size;
    char		name[ 64 ];
};

struct WATCHPOINT * watchpoints = 0L;
int watchpoint_hit( ULONGWORD address, int count, struct WATCHPOINT * wp );

//  This is fixed storage area for the console ROM, and it's file name.

char * rom = 0L;
char *rom_name = 0L;

//  And for the NVRAM console storage

char * nvram = 0L;
char * nvram_name = 0L;
char * microvax = 0L;


//  Add a new watchpoint

int add_watchpoint( ULONGWORD addr, short size, char * name )
{
    struct WATCHPOINT * wp;

    for( wp = watchpoints; wp; wp = wp-> next ) {
        if( wp-> address == addr ) {
            wp-> size = size;
            return 1;
        }
    }
    wp = ( struct WATCHPOINT * ) getmem( sizeof( struct WATCHPOINT));
    wp-> next = watchpoints;
    watchpoints = wp;
    wp-> address = addr;
    wp-> size = size;
    if( name ) {
        strncpy( wp-> name, name, 63 );
        wp-> name[63] = 0;
    }
    else
        sprintf( wp-> name, "WATCH_%08lX", ( long ) wp-> address );

    return 0;

}

//  List watchpoints

int show_watchpoints( void )
{
    struct WATCHPOINT * wp;

    if( watchpoints == 0L ) {
        printf( "No watchpoints.\n" ); 
        return 0;
    }

    printf( "Watchpoints:\n" );
    for( wp = watchpoints; wp; wp = wp-> next ) {
        printf( "    %08X:  %d bytes    \"%s\"\n", 
            wp-> address, wp-> size, wp-> name );
    }
    printf( "\n" );
    return 0;
}


//  Delete a watch point

int delete_watchpoint( ULONGWORD addr )
{

    struct WATCHPOINT * wp, *next, *last;

    if( watchpoints == 0L ) 
        return -1;

    for( wp = watchpoints, last = 0L; wp; wp = next ) {

        next = wp-> next;
        if( wp-> address != addr ) {
            last = wp;
	    continue;
        }
        if( wp == watchpoints ) 
            watchpoints = next;
        else
            last-> next = next;

        freemem(( char * ) wp );
        return 0;
 	
    }

    return -1;

}


//  push data onto the current stack


LONGWORD push_data( void * Data, short len )
{

    short j;
    LONGWORD  sp, rc;
    unsigned char * data;

    data = (unsigned char * ) Data;
    
    sp = vax.SP;
    
    if( vax.MAPEN == 0 && ( sp < 0L || ( ULONGWORD ) sp > vax.memsize )) {
        return set_fault( EXC_KSNV, 0 );
    }
    
    for( j = 0; j < len; j++ ) {
        sp--;
        rc = store_memory( sp, ( void * ) &( data[ j ]), 1 );
        if( rc )
            return rc;
    }
    
    vax.SP = sp;
    
    return VAX_OK;
}

//  Store an operand back to memory.  If it's a register, then there's
//  no work to be done.  If memory, then handle byte sex and access vios.

LONGWORD put_operand( struct OPCODE * opcode,
                  short n,
                  short mode,
                  char * addr )
{

    short k;
    LONGWORD rc;
    unsigned char * dst;
    short len;

    len = ( short ) opcode-> size[ n ];
    dst = opcode-> address[ n ];

/*
 *  If the emulator address is zero, then this is a VAX address.  Otherwise we
 *  can just write to the register address.
 */
 
    if( dst == 0L ) {
        rc = store_memory( opcode-> VAXaddr[ n ], ( void * ) addr, len );
    }
    else {
        for( k = 0; k < len ; k++ )
            dst[ k ] = addr[ k ];
        rc = VAX_OK;
    }
    
    return rc;
}


//  Get a pointer to an operand

char * get_operand( struct OPCODE * opcode, short n, short mode )
{
    unsigned char * addr;
    short len;
    LONGWORD rc;
    int k;

	k = mode;

    len = ( short ) opcode-> size[ n ];
    addr = opcode-> address[ n ];
    if( addr == 0L ) {

        if( len <= 4 ) {
            rc = load_register( ( short ) vax.treg, opcode-> VAXaddr[ n ], len );
            if( rc )
                return 0L;
        }
        
        /* If it's greater than 4 it must be 8, which is a quadword load. */
        /* We load this into a pair of registers adjacent in memory, as   */
        /* two 4-byte loads.  */
        
        else {

#if BIGENDIAN        
            rc = load_register( ( short ) ( vax.treg - 1 ), opcode-> VAXaddr[ n ] + 4 , 4 );
            if( rc )
                return 0L;
            rc = load_register( ( short ) vax.treg, opcode-> VAXaddr[ n ] + 0, 4 );
            if( rc )
                return 0L;
#else
            rc = load_register( ( short ) ( vax.treg - 1 ), opcode-> VAXaddr[ n ] + 0 , 4 );
            if( rc )
                return 0L;
            rc = load_register( ( short ) vax.treg, opcode-> VAXaddr[ n ] + 4, 4 );
            if( rc )
                return 0L;
#endif
            vax.treg = vax.treg - 1;
        }
        
        addr = ( unsigned char * ) &( vax.reg[ vax.treg ]);
        
        /* KLUDGE */

#if BIGENDIAN        
        if( len == 1 )
            addr = addr + 3;
        else
        if( len == 2 )
            addr = addr + 2;
#endif

        /* END KLUDGE */
        
        vax.treg = vax.treg - 1;
    }
                
    return ( char * ) addr;
}


//
//  Load a memory location (VAX is a little-endian system)
//

LONGWORD load_register( short regn, ULONGWORD address, short count)
{


    unsigned char * r;
    unsigned char * p;
    ULONGWORD addr, addr2;
    short size, n, spans_pages;
    LONGWORD rc;
#if !BIGENDIAN
    LONGWORD rx;
#endif

    spans_pages = 0;

    ASSERT( regn >= 0, "load_register regn" );
    ASSERT( regn <= MAXREG, "load_register regn" );

/*
 *  We are given a machine address.  If virtual memory is on, we'll need to handle
 *  the possibility of a page fault.  Otherwise, we just point to physical memory
 *  and have at it.
 */
 
    addr = address;

// I think this is unnecessary; we only do this kind of direct
//  addressing when VM is off, so it is caught below the VM
//  translation block...
//
//  if( addr >= 0xC0000000)
//      return load_io( addr,  ( void * ) &vax.reg[ regn], count );

    if( vax.MAPEN ) {
        size = count;
        
        if( size > 1 ) {
            addr2 = addr + size - 1;
            spans_pages = ( short ) (( addr & 0xFFFFFE00 ) != ( addr2 & 0xFFFFFE00 ));
        }
        else
            spans_pages = 0;
        
        /* Resolve the address of the first byte of the data */
        
        rc = vm( &addr, &size, VM_READ );
        if( rc )
            return rc;
    
        /* If the value spans pages, then we must fault in a second page. */
        
        if( spans_pages ) {
            rc = vm( &addr2, &size, VM_READ );
            if( rc )
                return rc;
        }
    }
 
 /*
  * Convert to a physical address.  If this isn't possible then call the
  * load_io() routine which does memory mapped I/O work.
  */
  
    p = PHYADDR( addr );
    if( p == 0L )
        return load_io( addr,  ( void * ) &vax.reg[ regn], count );
                
    r = ( unsigned char * ) &( vax.reg[ regn ]);

/*
 *  If we are reading into a processor register, we only disturb as many bytes
 *  as are read; i.e. if you read a word into a register, only the lsb 2 bytes
 *  of the register are modified.
 *
 *  If you are reading into a scratch register, the entire register is zeroed
 *  on the read operation.  The caller must decide if the value is to be sign-
 *  extended or not.
 */
 

    if( regn > 15 ) {
        vax.reg[ regn ] = 0L;
    }


    /* If the read spans pages, we must do a split read.  These */
    /* are fairly infrequent, so for now I tolerate a slooooow  */
    /* mechanism (byte-for-byte read).                          */
    
    if( spans_pages ) {
    
        for( n = 1; n <= count; n++ )
            rc = load_byte ( address + n - 1, ( void * ) &(
#if BIGENDIAN
                                         r[ count - n ]
#else
                                         r[ n - 1 ]
#endif
                                                       ));
    }
    else {
    

 /*
  * Now, based on data size, load the appropriate number of bytes into the
  * register.
  */

#if BIGENDIAN 
        switch( count ) {
        
        case 1: r[ 3 ] = p[ 0 ];
                break;

        case 2: r[ 3 ] = p[ 0 ];
                r[ 2 ] = p[ 1 ];
                break;

        case 4: r[ 3 ] = p[ 0 ];
                r[ 2 ] = p[ 1 ];
                r[ 1 ] = p[ 2 ];
                r[ 0 ] = p[ 3 ];
                break;

        default:    
                return VAX_FAULT;
                
        }
#else

        for( rx = 0; rx < count; rx++ )
            r[ rx ] = p[ rx ];
#endif

    }
    
    return VAX_OK;
}


//
//  Sign extend a value based on size.  Note that this routine
//  has been carefully organized to put the most common cases
//  first.  Don't re-arrange these incautiously....

LONGWORD sext( LONGWORD data, short size )
{

    if( size == 1 ) {
        if(  data >= 128 )
            return data - 256L;
        else
            return data;
    }

    // If it's a short then extend it else return it.

    if( size == 2 )
        if( data >= 32768 ) 
           return data - 65536L;

    //  Otherwise just return it because it doesn't need extending
    //  *OR* it's a four-byte value already.

    return data;

}


//
//  Load a memory location (VAX is a little-endian system) to local storage
//

LONGWORD load_memory( LONGWORD address, unsigned char * dest, short count)
{

    short   n;
    unsigned char * r;
    unsigned char * p;
    ULONGWORD addr, addr2;
    LONGWORD rc;
    LONGWORD spans_pages = 0;
    short size;

//  If this is a quadword read, then break it into two longword reads to
//  ensure proper behavior on little-endian hosts.  **DO WE NEED TO MAKE 
//  THE ORDER OF THIS ENDIAN-SENSITIVE A-LA STORE_MEMORY?  PROBABLY.  TBC.

    if( count == 8 ) {

        rc = load_memory( address, dest, 4 );
        if( rc )
            return rc;
        rc = load_memory( address+4, dest+4, 4 );
        return rc;
    }

    addr = address;
        
    if( vax.MAPEN ) {
        size = count;
 
        if( size > 1 ) {
            addr2 = addr + size - 1;
            spans_pages = ( addr & 0xFFFFFE00 ) != ( addr2 & 0xFFFFFE00 );
        }
        else
            spans_pages = 0;

        rc = vm( &addr, &size, VM_READ );
        if( rc )
            return rc;
            
        if( spans_pages )
            rc = vm( &addr2, &size, VM_READ );
            
        if( rc )
            return rc;
    }

    /*
     *  Zero the destination before a partial (4-byte) load so that any
     *  slot wider than a real VAX longword gets deterministic zero bits
     *  above the loaded value instead of whatever garbage was already
     *  there.  Unconditional since LONGWORD/ULONGWORD are fixed 32-bit
     *  types on every build now -- see AUDIT.md 5.1 (this used to be
     *  gated behind HAS64BITLONGS, which was never defined here).
     */
    if( count == 4 )
        * (LONGWORD * ) dest = 0L;

    p = PHYADDR( addr );
    if( p == 0L )
        return load_io( addr,  dest, count );

    r = ( unsigned char * ) dest;

    /* If the read spans pages, we must do a split read.  These */
    /* are fairly infrequent, so for now I tolerate a slooooow  */
    /* mechanism (byte-for-byte read).                          */
    
    if( spans_pages ) {
    
        for( n = 1; n <= count; n++ )
            rc = load_byte ( address + n - 1, ( void * ) &(
       
#if BIGENDIAN
                              r[ count - n ]
#else
                              r[ n - 1]
#endif
                                               ));
    }
    else {
    
    /* No it doesn't span pages, so read contiguous bytes from memory */
    
#if BIGENDIAN    
        for( n = 1; n <= count; n++ ) {
            r[ count - n ]  = *(p++);
        }
#else
        for( n = 0; n < count; n++ ) {
            r[ n ] = *(p++);
        }
#endif
    }
    return VAX_OK;
}


//
//  Load a single byte from a memory location (VAX is a little-endian system) to 
//  local storage.  This is a special case of load_memory since we do this a lot
//  while decoding instructions.
//

LONGWORD load_byte( LONGWORD address, unsigned char * dest )
{

    ULONGWORD addr;
    short size;
    LONGWORD rc;
    unsigned char * p;
    
    addr = address;
    if( vax.MAPEN ) {
        size = 1;
        rc = vm( &addr, &size, VM_READ );
        if( rc )
            return rc;
    }

    p = PHYADDR( addr );
    if( p == 0L )
        return load_io( addr,  dest, 1 );

    *dest = *p;
        
    return VAX_OK;
}

//
//  Determine if a write will hit a watchpoint
//

int watchpoint_hit( ULONGWORD address, int count, struct WATCHPOINT * wp )
{

    if( address+count <= wp-> address )
        return 0;
    
    if( address >= wp-> address+ wp-> size )
        return 0;
    
    return 1;
}

//
//  Store a memory location (VAX is a little-endian system) to local storage
//
//  Note that later this will need to handle translation via the page tables.
//

LONGWORD store_memory( LONGWORD address, unsigned char * src, short count)
{

    short   n;
    unsigned char * r;
    unsigned char * p;

    ULONGWORD addr, addr2;
    LONGWORD rc;
    short size;
    LONGWORD spans_pages = 0;
    struct WATCHPOINT * wp;


//  In order to correctly support endian-ness issues on various host
//  ports, for quadword operations, break it into two longword operations.

    if( count == 8 ) {
        long rc;

#ifdef BIGENDIAN
        rc = store_memory( address, src+4, 4 );
        if( rc )
            return rc;
        rc = store_memory( address+4, src, 4 );

#else
        rc = store_memory( address, src, 4 );
        if( rc )
            return rc;
        rc = store_memory( address+4, src+4, 4 );
#endif
        return rc;
    }

// I think this is unnecessary; we only do this kind of direct
//  addressing when VM is off, so it is caught below the VM
//  translation block...
//
//    if( ( ULONGWORD ) address >= 0xC0000000UL )
//        return store_io(  address,  src,  count );
    
    addr = address;

    if( watchpoints ) {

        for( wp = watchpoints; wp; wp = wp-> next ) {

            if( watchpoint_hit( address, count, wp )) {
            
                char cmd[ 32 ];
                
                printf( "%%VAX-I-WATCH, watchpoint %s hit, watching %d byte%s at %08X,\n",
                            wp-> name, wp-> size, wp-> size == 1 ? "" : "s", wp-> address );
                printf( "-VAX-I-WRITE, at PC=%08X, %d byte%s at %08X changed to ",
                    vax.instruction_PC, count, count == 1 ? "" : "s", address );
                if( count == 1 ) 
                    printf( "%02X\n", *src );
                else
                if( count == 2 )
	            printf( "%04X\n", *(short *)src );
                else
                    printf( "%08lX\n", *( long*) src );
                
                if( !vax.halted ) {
                    printf( "-VAX-I-INSTR, " );
                    sprintf( cmd, "EXAM/INSTR %08X", vax.instruction_PC );
                    console_dispatch( cmd );
                }
            }

        }

    }

//  If VM enabled (usually true) then translate the address from V to P memory.

    if( vax.MAPEN ) {
        size = count;
        if( size > 1 ) {
            addr2 = addr + size - 1;
            spans_pages = ( addr & 0xFFFFFE00 ) != ( addr2 & 0xFFFFFE00 );
        }
        else
            spans_pages = 0;

        rc = vm( &addr, &size, VM_WRITE );
        if( rc )
            return rc;
        if( spans_pages )
            rc = vm( &addr2, &size, VM_WRITE );
        if( rc )
            return rc;
    }

    p = PHYADDR( addr );
    if( p == 0L )
        return store_io( addr,  src, count );

    r = ( unsigned char * ) src;

    if( spans_pages ) {
    
        for( n = 1; n <= count; n++ )
            rc = store_memory( address + n - 1, ( void * ) 
#if BIGENDIAN
            &( r[ count - n ]), 1 );
#else
            &( r[ n - 1 ]), 1 );
#endif

    }
    else {
    
#if BIGENDIAN    
        for( n = 1; n <= count; n++ ) {
            *(p++) = r[ count - n ];
        }
#else
        for( n = 0; n < count; n++ ) {
            *(p++) = r[ n ];
        }
#endif
    }

    return VAX_OK;
}


#if 0
LONGWORD pmvalid( ULONGWORD addr )
{

    if( rom && ( addr >= vax.rom_base ) &&  ( addr <= vax.rom_end ))
        return 1;
    
    if( addr < vax.memsize )
        return 1;
    
    return 0;
}
#endif
    


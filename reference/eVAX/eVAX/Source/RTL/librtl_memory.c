//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_memory.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//  History:    12/14/99    First written.  In it's initial form this is not as
//                          robust as ultimately will be needed.  It meets the
//                          basic needs of malloc() and related calls, but does
//                          not yet adequately protect against fragmentation, etc.
//
//                          To-do list:
//
//                          1.  Sort the free list in ascending size order so it
//                              satisfies new allocations from the smallest free 
//                              block that will work, not the last-free'd block 
//                              that works.
//
//                          2.  When a free block is co-joined with another block,
//                              it should recursively call free() again to see if 
//                              it joins with yet another block.  This will allow 
//                              re-aggregation to be more complete.
//
//              12/15/99    Added dump routine to support SHOW MEMORY/RUNTIME
//


#include "vax.h"
#include "console_proto.h"   /* For region handling routines */
#include "shim.h"
#include "ss_def.h"


#define LIBVM_MALLOC   1
#define LIBVM_LIBRTL   2
#define LIBVM_VIRGIN   4
#define LIBVM_CARVED   8
#define LIBVM_ZEROED  16

struct MEMBLK {
    struct MEMBLK * next;
    LONGWORD        flags;
    LONGWORD        zone;
    LONGWORD        size;
    LONGWORD        req_size;
    LONGWORD        addr;
};


struct MEMBLK * mem_allocated = 0L;
struct MEMBLK * mem_freed = 0L;

/*
 *	Called from lib_initialize() during runtime initialization
 *	before loading an image.  Reset the memory lists!
 *
 *      Note that we only need to free the descriptive data structures
 *      for the memory; the memory itself is from the virtual VAX P0
 *      space and is reset during image loads.
 */

LONGWORD decc_init_memory(void)
{
    struct MEMBLK * p, *next;

    for( p = mem_allocated; p; p = next ) {
        next = p-> next;
        freemem( p );
    }

    for( p = mem_freed; p; p = next ) {
        next = p-> next;
        freemem( p );
    }

    mem_allocated = 0L;
    mem_freed = 0L;
    return VAX_OK;
}

/*
 *  Diagnostic routine to dump info about memory allocations
 */

LONGWORD decc_dump_memory( int flag )
{
    LONGWORD min, max, total, count;
    struct MEMBLK * p;
    char fbuff[ 80 ];

    if( mem_allocated == 0L && mem_freed == 0L ) {
        printf( "No runtime memory allocations\n" );
        return VAX_OK;
    }

    printf( "\nRUNTIME MEMORY ALLOCATIONS:\n" );
    printf( "\tALLOCATED MEMORY:\n" );

    min = 0x7FFFFFFFL;
    max = total = count = 0;

    if( flag && mem_allocated ) 
        printf( "\t\t Entry    Base     End      Size Actual     Zone   Flags\n" );

    for( p = mem_allocated; p; p = p-> next ) {

        if( p-> size < min )
            min = p-> size;
        if( p-> size > max )
            max = p-> size;
        total = total + p-> size;
        count++;
        fbuff[ 0 ] = 0;

        if( p-> flags & LIBVM_LIBRTL )
            strcat( fbuff, "LIBRTL " );

        if( p-> flags & LIBVM_MALLOC )
            strcat( fbuff, "MALLOC " );

        if( p-> flags & LIBVM_ZEROED )
            strcat( fbuff, "ZEROED " );

        if( flag )
            printf( "\t\t(%4d) %08X %08X  %6d %6d %08X  %s\n",
                    count, p-> addr, p-> addr + p-> size - 1, 
                    p-> req_size, p-> size, p-> zone, fbuff );

    }

    if( count == 0 ) {
        printf( "\t\tNO MEMORY ALLOCATED\n" );
    }
    else {
        printf( "\t\tBlocks:   %6d        Bytes:  %6d\n", count, total );
        printf( "\t\tSmallest: %6d       Largest: %6d\n", min, max );
    }


    printf( "\n\tFREED MEMORY:\n" );

    min = 0x7FFFFFFFL;
    max = total = count = 0;

    if( flag && mem_freed ) 
        printf( "\t\t Entry    Base     End      Size   Flags\n" );

    for( p = mem_freed; p; p = p-> next ) {

        if( p-> size < min )
            min = p-> size;
        if( p-> size > max )
            max = p-> size;
        total = total + p-> size;
        count++;
        fbuff[ 0 ] = 0;

        if( p-> flags & LIBVM_VIRGIN )
            strcat( fbuff, "VIRGIN " );

        if( p-> flags & LIBVM_CARVED )
            strcat( fbuff, "CARVED " );

        if( flag )
            printf( "\t\t(%4d) %08X %08X  %6d  %s\n",
                    count, p-> addr, p-> addr + p-> size - 1, 
                    p-> size, fbuff );

    }

    if( count == 0 ) {
        printf( "\t\tNO MEMORY FREED\n" );
    }
    else {
        printf( "\t\tBlocks:   %6d        Bytes:  %6d\n", count, total );
        printf( "\t\tSmallest: %6d       Largest: %6d\n\n", min, max );
    }

    return VAX_OK;
}



/*----------------------------------------------------------------------*
 *                                                                      *
 *    ptr = malloc( size );                                             *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_malloc( LONGWORD argc, LONGWORD * argv )
{
    LONGWORD size, rounded_size, region;
    struct MEMBLK * p, * q, * last;
    LONGWORD flag, zone, n, zero, rc;

    zero = 0;
    size = argv[ 0 ];
    rounded_size = ( size + 16 ) & 0xFFFFFFF0UL;

    flag = LIBVM_MALLOC;
    zone = 0;

    /* Kludge.  Malloc is only called with one argument.  */
    /* If we get called with two, it's from LIB$GET_VM    */
    /* and related routines.  If so, mark the block.      */

    if( argc > 1 )
        flag = argv[ 1 ];


    if( argc > 2 )
        zone = argv[ 2 ];
    

    /* Search the free list to see if we have one that will do */

    last = 0L;
    for( p = mem_freed; p; p = p-> next ) {

        if( p-> size >= size ) {

            /* Take it off the free list */

            if( last == 0L )
                mem_freed = p-> next;
            else
                last-> next = p-> next;

            /* Put it on the allocated list */

            p-> next = mem_allocated;
            mem_allocated = p;

            /* If there is leftover from the allocation, make a new free block for it */

            if( p-> size > rounded_size ) {
                q = ( struct MEMBLK * ) getmem( sizeof( struct MEMBLK ));
                q-> size = p-> size - rounded_size;
                q-> addr = p-> addr + rounded_size;
                q-> next = mem_freed;
                q-> flags |= LIBVM_CARVED;
                mem_freed = q;
            }

            p-> size = rounded_size;
            p-> req_size = size;
            p-> flags = flag;
            p-> zone = zone;

            if( p-> flags & LIBVM_ZEROED ) {
                for( n = 0; n < size; n++ ) {
                    rc = store_memory( p-> addr + n, ( unsigned char * ) &zero, 1 );
                    if( rc )
                        return -1;
                }
            }

            /* Return the address to the caller */

            return p-> addr;
        }
        last = p;
    }

    /* It's not available on the free list, so make more space */

    get_region_size( 0, &region );
    if( region == 0L )
        region = 0x0200;

    q = ( struct MEMBLK * ) getmem( sizeof( struct MEMBLK ));
    q-> size = rounded_size;
    q-> req_size = size;
    q-> addr = region;
    q-> next = mem_allocated;
    q-> zone = zone;
    q-> flags = flag;

    mem_allocated = q;
    vax.R0 = region;

    if( q-> flags & LIBVM_ZEROED ) {
        for( n = 0; n < size; n++ ) {
            rc = store_memory( q-> addr + n, ( unsigned char * ) &zero, 1 );
            if( rc )
                return -1;
        }
    }

    /* Round up to a page boundary.  If there is free space */
    /* then put it on the free list for future use.         */

    rounded_size = ( size + 512 ) & 0xFFFFFE00;

    if( q-> size < rounded_size ) {
        p = ( struct MEMBLK * ) getmem( sizeof( struct MEMBLK ));
        p-> size = rounded_size - q-> size;
        p-> addr = vax.R0 + q-> size;
        p-> next = mem_freed;
        p-> flags = LIBVM_VIRGIN;
        p-> zone = 0;
        mem_freed = p;
    }

    /* Update the region size */

    region = region + rounded_size;
    set_region_size( 0, region );


    return vax.R0;

}



/*----------------------------------------------------------------------*
 *                                                                      *
 *    free( char * ptr );                                               *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD decc_free( LONGWORD argc, LONGWORD * argv )
{
	struct MEMBLK * p, *q, * last;
        int front;

	LONGWORD addr;

	addr = argv[ 0 ];

	/* Try to find the memory block that we are free'ing up */

        last = 0L;
        for( p = mem_allocated; p; p = p-> next ) {
            if(( addr >= p-> addr ) && ( addr < p-> addr + p-> size )) 
                break;
            last = p;
        }

        /* If we didn't find one then bad... */

        if( p == 0L ) {
            return -1;
        }

        p-> flags = 0L;

        /* Unlink it from the allocated list */

        if( last == 0L )
            mem_allocated = p-> next;
        else
            last-> next = p-> next;

        /* See if we can find a block on the free list that this joins to */

        last = 0L;
        front = 0;
        for( q = mem_freed; q; q = q-> next ) {
            if( q-> addr + q-> size == addr ) {
                break;
            }
            if( p-> addr + p-> size == q-> addr ) {
                front = 1;
                break;
            }
            last = q;
        }

	/* If fragmented, etc. then just put it on the free list */

        if( !q ) {
            p-> next = mem_freed;
            mem_freed = p;
            return 0;
        }

        /* Otherwise, add the blocks together on the existing free list entry */

        if( front )
            q-> addr = p-> addr; /* Back up the base pointer */
            
        q-> size = q-> size + p-> size;  /* And aggregate the size */
        freemem( p );

	return 0;
}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    sts = lib$get_vm( long * size, char ** ptr );                     *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD lib_get_vm( LONGWORD argc, LONGWORD * argv )
{
	long rc;
        LONGWORD local_argv[ 3 ];

        LONGWORD size, size_addr, ret_addr, addr, zone;

        size_addr = argv[ 0 ];
        ret_addr = argv[ 1 ];

        rc = load_memory( size_addr, ( void * ) &size, 4 );
        if( rc )
            return SS_ACCVIO;

        zone = 0L;
        if( argc == 3 ) {
            if( argv[ 2 ] ) {
                rc = load_memory( argv[ 2 ], ( void * ) &zone, 4 );
                if( rc )
                    return SS_ACCVIO;
            }
        }


        /* Note kludge - malloc() takes one argument, but if   */
        /* we send a second one it's special flags.            */

        local_argv[ 0 ] = size;
        local_argv[ 1 ] = LIBVM_LIBRTL;
        local_argv[ 2 ] = zone;

        addr = decc_malloc( 2, local_argv );
        if( addr == 0L )
            return SS_INSFMEM;

        rc = store_memory( ret_addr, ( unsigned char * ) &addr, 4 );

        return SS_NORMAL;
}





/*----------------------------------------------------------------------*
 *                                                                      *
 *    sts = lib$free_vm( long * size, char ** ptr );                    *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD lib_free_vm( LONGWORD argc, LONGWORD * argv )
{
	long rc;

        LONGWORD size, size_addr, ret_addr, addr, zone;

        size_addr = argv[ 0 ];
        ret_addr = argv[ 1 ];

        rc = load_memory( size_addr, ( void * ) &size, 4 );
        if( rc )
            return SS_ACCVIO;

        rc = load_memory( ret_addr, ( void * ) &addr, 4 );
        if( rc )
            return SS_ACCVIO;

        if( argc == 3 ) {
            zone = 0L;
            if( argv[ 2 ] ) {
                rc = load_memory( argv[ 2 ], ( void * ) &zone, 4 );
                if( rc )
                    return SS_ACCVIO;
            }
            if( zone )
            printf( "Warning: attempt to use non-default LIB$FREE_VM zone %08X\n", zone );
        }


        if( decc_free( 1, &addr ) == -1)
            return SS_INVARG;

        return SS_NORMAL;
}





/*----------------------------------------------------------------------*
 *                                                                      *
 *    sts = lib$delete_vm_zone( long * zoneid );                        *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD lib_delete_vm_zone( LONGWORD argc, LONGWORD * argv )
{
	struct MEMBLK * p, * last;
	long rc;

        LONGWORD zone_addr, zone, addr;

        zone_addr = argv[ 0 ];

        rc = load_memory( zone_addr, ( void * ) &zone, 4 );
        if( rc )
            return SS_ACCVIO;

        last = 0L;
        p = mem_allocated;

        while( p != 0L ) {

            /* If this is from the target zone, free it up */

            if( p-> zone == zone ) {

                /* Free the memory */

                addr = p-> addr;
                decc_free( 1, &addr );

                /* Because the free disturbes the chain, back */
                /* up to the previous item to continue the    */
                /* scan...                                    */

                if( last == 0L )
                   p = mem_allocated;
                else
                   p = last;

                continue;
            }

            /* Otherwise just look to the next block */

            last = p;
            p = p-> next;
        }

        return SS_NORMAL;
}


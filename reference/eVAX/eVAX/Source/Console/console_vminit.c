//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_vminit.c
//
//  Purpose:    This module implements the VMINIT command.  This command 
//              sets up a virtual memory mapping system for all pages of
//              physical memory.
//
//  History:    08/19/98        Initial implementation
//
//              01/06/99        Clear the symbol table when a VMINIT is
//                              given since the symbols point to zero'd
//                              locations now.  Note that this doesn't
//                              include "reserved" symbols with a $
//                              character in the name.
//
//              02/15/99        Make the system pages PTE$K_UREW so user
//                              and supervisor code can't modify page tables.
//
//              04/29/99        When the VMINIT is executed, flush the TB
//                              statistics as well.
//
//              05/03/99        Make a PTE$K_NONE page at the top of each
//                              stack to mark it's end point.
//
//              05/11/99        Create the string pool area and define the
//                              CONSOLE$STRINGPOOL* symbols that describe it.
//
//              05/25/99        When a VMINIT command is given, clear the system
//                              symbol table of all non-permanent symbols, since
//                              the storage they presumably define is erased.
//
//              06/09/99        Disallow if you are not in kernel mode.  And 
//                              when done, make sure we are still in kernel mode.
//
//              03/31/00        Modified to use DCL syntax.  Also added ability to
//                              specify KSP, SSP, ESP, and ISP sizes, and string pool
//                              size.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "pte.h"
#include "dclrtl.h"

LONGWORD setpte( char ** P );
LONGWORD setpte_multiple( char ** P );


LONGWORD console_vminit_dcl( long id )
{

    extern char * page_map;
    
    LONGWORD rc, i, pcount;
    char b[ 64 ], *bp;
    ULONGWORD scratch, stringpool;
    union PTE pte;
    extern LONGWORD tb_hit, tb_try, tb_pflush;    
    LONGWORD size[ 4 ], idx, total, zcount, n;
    LONGWORD ptecount, page;
    ULONGWORD base, paddr;
    LONGWORD stacksize[ 4 ], stringsize, debug;
    
//  Validate that we have a VAX... this is required for all console commands.

    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

//  You can't do this if you aren't in kernel mode!  This is to prevent accidental
//  screw ups.

    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;


//  We will need to build a list of the sizes of each of the regions of the address
//  space.  Let's initialize the list to zeros.

    size[ 3 ] = 0;    
    for( idx = 0; idx < 3; idx++ )
        size[ idx ] = DCLgetinteger( DCL_CALLBACK_QUALIFIER, 1801 + idx );

    for( i = 0; i < 4; i++ )
        stacksize[ i ] = DCLgetinteger( DCL_CALLBACK_QUALIFIER, 1804 + i );

    stringsize = DCLgetinteger( DCL_CALLBACK_QUALIFIER, 1808 );
    debug = DCLpresent( DCL_CALLBACK_QUALIFIER, 1809 ) > 0;

//  Let's see how many pages total the user has explicitly requested.
//  Note that he may specify 0-3 regions of data explicitly.
    
    for( zcount = 0, total = 0, idx = 0; idx < 3; idx++ ) {
        if( size[ idx ] == 0 )
            zcount++;
        total += size[ idx ];
    }

//  If the total number of pages explicity requested is too darn big,
//  then give it up.

#ifndef DYNVM   

    if( (ULONGWORD) total > (ULONGWORD) (vax.memsize >> 9 ))
        return VAX_INVVMSIZE;
#endif


//  Convert the physical memory sizes to a page count, and subtract the
//  number claimed by the user.  We divide the leftover pages by the
//  number of regions not explicity sized by the command.  This is
//  allocated to each of the still-zero-sized regions.  This generally
//  has the effect of taking unclaimed memory and dividing it evenly
//  over the regions.
    
    total = ( vax.memsize >> 9 ) - total;
    if( zcount )
        total = total / zcount;
    else
        total = 0;
        
    for( idx = 0; idx < 3; idx++ )
        if( size[ idx ] == 0 )
            size[ idx ] = total;


//  Often, the page count won't divide perfectly evenly given the number of physical
//  pages of memory.  If so, then allocate the remaining page or two to the S0 region.

    pcount =  ( vax.memsize >> 9 ) -  ( size[ 0 ] + size[ 1 ] + size[ 2 ] );
    if( pcount > 0 )
        size[ 2 ] += pcount;
            

//  Figure out how many pages will be taken up by the page tables.  The
//  S0 system region must be AT LEAST this big to hold all the page tables,
//  plus an extra 16 pages for the special stacks (KSP, ISP, etc.).

    page = 0;
    for( idx = 0; idx < 3; idx++ ) {
        ptecount = ( size[ idx ] * 4 ) / 512 + 1;
        page += ptecount;
    }
    
    if( page + 32 > size[ 2 ])
        return VAX_INVS0SIZE;
        
    paddr = 0;  /*  First PTE goes at location 0 */
    page = 0;   /*  First physical page mapped is page 0 */
    
    
//  Turn off virtual memory, since we're gonna screw with it a lot.

    vax.MAPEN = 0;

//  We're going to make really serious changes to memory, so initialize the
//  physical memory to all zeroes before we begin.
    
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "VM initializing\n\t%u pages of physical memory\n",
            vax.memsize >> 9 );

    for( n = 0; (ULONGWORD) n < vax.memsize; n++ )
        vax.memory[ n ] = 0;

    if( debug ) {
        printf( "         P0 = %5d pages\n", size[ 0 ] );
        printf( "         P1 = %5d pages\n", size[ 1 ] );
        printf( "         S0 = %5d pages\n", size[ 2 ] );
        printf( "        KSP = %5d pages\n", stacksize[ 0 ]);
        printf( "        ESP = %5d pages\n", stacksize[ 1 ]);
        printf( "        SSP = %5d pages\n", stacksize[ 2 ]);
        printf( "        ISP = %5d pages\n", stacksize[ 3 ]);
        printf( "STRING SIZE = %5d pages\n", stringsize );
    }

//  Dump the translation buffer

    invalidate_tb();
    tb_hit = tb_try = tb_pflush = 0L;

//  Also, since we've pretty much invalidated any symbols that point to
//  memory locations, zap the user symbol table.

    clear_all_symbols();
    clear_system_symbols();

//  First, build the S0 system space.


    ptecount = ( size[ 2 ] * 4 ) / 512 + 1;
    
//  Save the info about S0 space for later use.

    vax.region[ 2 ].pte_count = ptecount;
    strcpy( vax.region[ 2 ].name, "S0" );
    vax.region[ 2 ].size = size[ 2 ];

    vax.region[ 2 ].v_start = 0x80000000UL;
    vax.region[ 2 ].v_end =   0x80000000UL + ( size[ 2 ] << 9 );
    
    vax.region[ 2 ].p_start = 0L;
    vax.region[ 2 ].p_end = size[ 2 ] << 9;
    
    //  The base register is hard coded here to zero, since we happen to create
    //  the system page table here by default.
    
    vax.SBR = 0L;
    vax.SLR = size[ 2 ];
    
        
    //  Create the PFN for the system region.  Write as many PTE's as needed
    
    for( i = 0; i < size[ 2 ]; i++ ) {
    
        pte.longword = 0L;

        pte.bit.v = 1;
        pte.bit.prot = PTE_K_URKW;    /* All read, K write */
        pte.bit.pfn = (unsigned int) page;     /* Start pages at zero and go up */
        
        page_map[ page ] = 1;
        
        page++;
        
        store_memory( paddr, ( void * ) &pte, 4 );
        paddr += 4;
    }

//  Create the P0 page table.  It resides in system space, so keep writing
//  from where we are in memory, since this physical memory is all part
//  of the system space.

    //  Roll up the first physical addr where we'll write the first PTE
    //  to the next even page boundary
    
    paddr = ( paddr >> 9 );
    paddr = ( paddr + 1 ) << 9;

    vax.region[ 0 ].size = size[ 0 ];
    
    ptecount = ( size[ 0 ] * 4 ) / 512 + 1;
    
    //  The P0 base register will need to be a virtual address eventually, but 
    //  since the S0 and physical memory overlap perfectly, we can convert this
    //  later.  For now it's the physical address of the first PTE.
    
    vax.P0BR = paddr;
    vax.P0LR = size[ 0 ];

    vax.region[ 0 ].size = size[ 0 ];
    
    vax.region[ 0 ].pte_count = ptecount;
    strcpy( vax.region[ 0 ].name, "P0" );

    vax.region[ 0 ].v_start = 0x0000000UL;
    vax.region[ 0 ].v_end = vax.P0LR << 9;
    
    vax.region[ 0 ].p_start = page << 9;
    vax.region[ 0 ].p_end = ( page << 9 ) + ( vax.P0LR << 9 );
    
    
        
    //  Create the PFN for the P0 region.  Write as many PTE's as needed
    
    for( i = 0; i < size[ 0 ]; i++ ) {
    
        pte.longword = 0L;

#ifdef DYNVM
        pte.bit.v = 0;          /* No P0 page is valid initially */
        pte.bit.prot = 0x04;    /* But is read/write */
        pte.bit.pfn = 0;        /* Not mapped to a physical page yet */
#else
        pte.bit.v = 1;
        pte.bit.prot = 0x04;    /* All read/write */
        pte.bit.pfn = page;     /* Start pages at zero and go up */
#endif

        page++;
        
        if( i == 0 )
            pte.bit.prot = 0;   /* Bottom-most page is protected from any access */
            
        store_memory( paddr, ( void * ) &pte, 4 );
        paddr += 4;
    }

    //  The P0 base register must be a virtual address.  Since the PFN database starts at
    //  page 0, we can convert physical to virtual pretty easily.
    
    vax.P0BR += 0x80000000UL;
    
//  Now create the P1 address space.  The base register is a littly funky.  This is the
//  lowest P1 address permitted by the size of the PFN for P1.  This is because P1 grows
//  down towards lower addresses, unlike S0 and P0 which grow up.

#define MAX_P1 0x80000000UL
#define SP_P1  0x7FE00000UL

    base = MAX_P1 - ( ULONGWORD ) ( size[ 1 ] * 512 );

    //  Roll up the physical address we'll write the PTE to a page boundary.
    
    paddr = ( paddr >> 9 );
    paddr = ( paddr + 1 ) << 9;

    //  Let's see how many PTE's total could exist in the region.
    
    i = 0x40000000 >> 9;    /* Physical address range divided by 512 */
    
    //  The length register shows the number of pages NOT on the list.  This is a quirk
    //  of the VAX architecture.
    
    vax.P1LR = i - size[ 1 ];

    //  Each PTE takes four bytes.  We need to figure out, given the address where
    //  we know the first real PTE will actually be (the paddr value), what the
    //  effective base register value is, by subtracting this from the real paddr.
    //  We offset the paddr to be an S0 address since it must be virtual, not
    //  physical.
    
    vax.P1BR = ( paddr + 0x80000000UL ) - ( vax.P1LR * 4 );

    ptecount = ( size[ 1 ] * 4 ) / 512 + 1;
    
    vax.region[ 1 ].pte_count = ptecount;
    vax.region[ 1 ].size = size[ 1 ];
    
    vax.region[ 1 ].v_start = base;
    vax.region[ 1 ].v_end = base + ( size[ 1 ] << 9 );
    
    vax.region[ 1 ].p_start = page << 9;
    vax.region[ 1 ].p_end = ( page << 9 ) + ( size[ 1 ] << 9 );
    
    strcpy( vax.region[ 1 ].name, "P1" );
        
    //  Create the PFN for the P1 region.  Write as many PTE's as needed
    
    for( i = 0; i < size[ 1 ]; i++ ) {
    
        pte.longword = 0L;

#ifdef DYNVM
        pte.bit.v = 0;          /* No P1 page is initially valid */
        pte.bit.prot = 0x04;    /* but is read/write */
        pte.bit.pfn = 0;        /* Not mapped to a page yet */
#else
        pte.bit.v = 1;
        pte.bit.prot = 0x04;    /* All read/write */
        pte.bit.pfn = page;     /* Start pages at zero and go up */
#endif

        page++;
        
        store_memory( paddr, ( void * ) &pte, 4 );
        paddr += 4;
    }

//  Let's set up all the stack pointers correctly.

    //  Roll up the paddr to the next page.  We need this so we know where we
    //  can allocate some space for the various stacks.
    
    paddr = ( paddr >> 9 );
    paddr = ( paddr + 1 ) << 9;

    //  The user stack starts at the top of the P1 space. 
    
    vax.USP = SP_P1;
 
    //  Each of the privileged mode stacks must live in S0 space somewhere,
    //  so we allocate 4 pages each for them after the end of the PFN DB.
    
    vax.KSP = 0x80000000 + paddr + stacksize[ 0 ] * 512 - 4;
    paddr = paddr + stacksize[ 0 ] * 512;

    vax.ESP = 0x80000000 + paddr + stacksize[ 1 ] * 512 - 4;
    paddr = paddr + stacksize[ 1 ] * 512;

    vax.SSP = 0x80000000 + paddr + stacksize[ 2 ] * 512 - 4;
    paddr = paddr + stacksize[ 2 ] * 512;

    
    //  The ISP stack also lives in the same area, but is a physical address
    //  not a virtual one since VM translation doesn't happen at interrupt
    //  time.
    
    vax.ISP = paddr + stacksize[ 3 ] * 512 - 4;
    paddr = paddr + stacksize[ 3 ] * 512;

    //  While we are here, seal up that we are in kernel mode.

    vax.SP = vax.KSP;
    vax.pslw.cur_mod = AM_KERNEL;
    vax.pslw.prv_mod = AM_USER;
    vax.pslw.is = 0;

    //  Define the console "Scratch page" which is used to assemble and
    //  execute immediate mode instructions.

    scratch = 0x80000000 + paddr + 512;
    paddr = paddr + 512;
    set_symbol_direct( "CONSOLE$SCRATCH", scratch, 0 );


    //  A handfull of other registers need to be set up to not create the
    //  mistaken impression of a valid call frame.

    vax.FP = 0L;
    vax.AP = 0L;
    
    //  The SCB must be initialized, and the SCBB register points to it's
    //  physical address.  This is a single page in memory, and the SCBB is
    //  it's base.  This contains the interrupt vector addresses (which are
    //  themselves virtual addresses).  These are all initially zero which
    //  results in an unhandled exception.
    
    
    for( vax.SCBB = i = paddr + 512; i < 512; i++ )
        vax.memory[ i ] = 0;
    paddr = paddr + 512;

    //  For the convenience of assembler writers, we also set the SCBB$BASE
    //  symbol to be the virtual address base location where we write the 
    //  SCB vector.   You can also set an exception handler with the .SCB
    //  pseudo opcode.

    set_symbol_direct( "SCBB$BASE",vax.SCBB + 0x80000000, 0 );

    vax.console.p0_deposit = 0x00000200;

//  Show what we've done, unless we've already put it out in debug mode.

    vax.vm_initialized = 1;
    
    if( !debug && ( vax.console.flags & CONSOLE_VERBOSE )) 
        printf( "\tVM region sizes(decimal pages):  P0=%d  P1=%d  S0=%d\n\n",
            size[ 0 ], size[ 1 ], size[ 2 ] );
    
//  Okay, we have (hopefully) set up all the page tables correctly.  Turn VM on.

    vax.console.deposit = 0x00000200;     /* Initial deposit/asm location */    
    vax.MAPEN = 1;
    rc = VAX_OK;

//  Okay, we need to fiddle with the stack pages to make an unreadable
//  page at the end of each stack.  Set the PTE (using the PTE parser)
//  and then move the stack down one page.

    sprintf( b, "KSP-4 PROT=PTE$K_NONE" );
    bp = b;
    rc = setpte( &bp );
    vax.KSP = vax.KSP - 512;
    vax.SP = vax.KSP;

    sprintf( b, "ESP-4 PROT=PTE$K_NONE" );
    bp = b;
    rc = setpte( &bp );
    vax.ESP = vax.ESP - 512;

    sprintf( b, "SSP-4 PROT=PTE$K_NONE" );
    bp = b;
    rc = setpte( &bp );
    vax.SSP = vax.SSP - 512;

    /* Note that the USP is already in the last page (not sitting */
    /* just above it like the other stack pointers), so we don't  */
    /* offset by 4 bytes */

    sprintf( b, "USP PROT=PTE$K_NONE" );
    bp = b;
    rc = setpte( &bp );
    vax.USP = vax.USP - 512;

//  Define the string pool area, after the SCB and such.

    stringsize = stringsize * 512;
    stringpool = 0x80000000 + paddr + stringsize;
    set_symbol_direct( "CONSOLE$STRINGPOOL", stringpool, 0 );

    set_symbol_direct( "CONSOLE$STRINGPOOL_BASE", stringpool, 0 );

    set_symbol_direct( "CONSOLE$STRINGPOOL_SIZE", stringsize, 0 );

    sprintf( b, "%08X TO %08X PROT=PTE$K_ALL", 
             stringpool, stringpool + stringsize - 1 );
    bp = b;
    rc = setpte_multiple( &bp );
    if( rc )
        return rc;
    n = 0L;
    rc = store_memory( stringpool, ( void * ) &n, 4 );

    vax.console.s0_deposit = stringpool + stringsize;  /* Next page */
                                                 /* after string pool */

//  Last deal, let's set the page characteristic of the scratch page
//  so it's read/write for everyone.

    sprintf( b, "%08X PROT=PTE$K_ALL", scratch );
    bp = b;
    rc = setpte( &bp );

    vax.console.vminit_valid = 1;
      
    return rc;
}

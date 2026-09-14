//
//  Copyright (C) 1997,1998,1999, 2000 Forest Edge Software
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     vm.c
//
//  Purpose:    These routines are responsible for virtual memory translations.
//              Given a virtual address, convert it to a physical address, and
//              signal page faults, access violations, etc.
//
//              Also contains the routines that handle the translation buffer
//              cache, used to speed up VM translations.  TBIA and TBIS 
//              privileged processor registers are used to control this.
//
//  History:    08/31/98    Extracted from storage.c and made new module.
//
//              01/01/99    Make member alignment pragma VMS-only.
//
//              02/01/99    Make VM and TB tracing use the new SET DEBUG
//                          flags.
//
//              04/10/99    Support alternate algorithm for PTE evaluation of
//                          protect schemes, based on a table lookup.
//
//              04/26/99    Misc performance tweaks to the vm() routine.  Also
//                          added the invalidate_tp_prot() routine which allows 
//                          the translator to assume the protection for a page
//                          is valid if it's been checked once and we haven't
//                          changed psl.cur_mod yet. This short-circuits most of
//                          the PTE protection checks completely for normal memory
//                          operations.
//
//              04/29/99    More tweaks.  The translation buffer caching
//                          strategy was flawed and allowed a mode change to
//                          leave stale PTE protection data in the TB arrays.
//                          Now, when a PTE is changed the TB is invalidated
//                          and when a mode change occurs the protection valid
//                          field is cleared.  Finally, this had to be stored
//                          based on mode; i.e. the protection only reflects
//                          the success of the last operation, if we try a
//                          new kind of operation (read versus write) then
//                          the protection must be re-checked.
//
//              09/21/99    Added "sequential translation cache."  Over 80% of
//                          the VM translation requests appear to be on the very
//                          same page as the last one.  So we have a one-slot
//                          cache of the virtual->physical page mapping of the
//                          last translation.  This is cleared whenever the TB
//                          is cleared or on any error.  Note that STC is enabled
//                          with a #define of STC in the code below. This is
//                          mostly for debugging and timing comparisons.
//
//              11/09/99    Added ability to map invalid pages to physical memory
//                          when under control of the microkernel.  This is not
//                          used when a "real" operating system is booted.
//
//              02/15/00    The PTE(M) bit was not being set on a write
//                          operation; this is required for OS level paging
//                          support.  Adding this reduced the efficiency of
//                          the TB a little bit, since it now will fail to
//                          use the TB if the cached TB mode differs from 
//                          the current mode.  This is needed so the "real"
//                          PTE in memory can't get stale.

#include "vax.h"
#include "pte.h"


extern char * rom;      /* ROM memory               */
extern char * nvram;    /* Non-volatile RAM memory  */


//  This is a low-level translation cache.  The index to the cache is a combination
//  of the low-level 2 bits of the page number, and the region number, to form a
//  128-entry translation cache.


struct TB {
    LONGWORD        page;
    LONGWORD        paddr;
    LONGWORD        code;
    LONGWORD        m_bit;
    LONGWORD        prot_valid;
};

struct TB tb[ 128 ];

LONGWORD   tb_try = 0L;
LONGWORD   tb_hit = 0L;
LONGWORD   tb_flush = 0L;
LONGWORD   tb_pflush = 0L;

void dump_tb(void);

//
//  This map is used to define the physical pages allocated to specific pages
//  of memory.  Supports VMINIT console memory mapping.

LONGWORD mapsize = 0L;
char * page_map = 0L;

//  Local prototypes...

LONGWORD tracevm( ULONGWORD Addr, short mode );
LONGWORD validate_page( union PTE * pte, LONGWORD paddr );
LONGWORD mapped_pages( void );

//  These values are used to keep a copy of the last translation done,
//  which is often in the same page as the current one (think of it
//  as a one-slot L1 cache for the VM.

#define STC 1

LONGWORD cached_virtual_page = -1L;
LONGWORD cached_physical_page = 0L;
LONGWORD cached_page_hit = 0;
LONGWORD cached_page_try = 0;
LONGWORD cached_mbit = 0;

#if STC
#define STC_FLUSH cached_virtual_page = -1L
#else
#define STC_FLUSH 
#endif

//  This table is used to define the PTE access code and functional
//  mask for each access mode.

#define VM_TABLE_ACCESS   0   /* 0 = algorithmic, 1 = table driven */

#define PTE_NA        0       /* 00 */
#define PTE_R         1       /* 01 */
#define PTE_RW        3       /* 11 */

#if VM_TABLE_ACCESS
static unsigned short access_map[16][4] = {     /*  PTE CODE    */
{   PTE_NA, PTE_NA, PTE_NA, PTE_NA  },          /*  0   NA      */
{   PTE_NA, PTE_NA, PTE_NA, PTE_NA  },          /*  1   <undef> */
{   PTE_RW, PTE_NA, PTE_NA, PTE_NA  },          /*  2   KW      */
{   PTE_R,  PTE_NA, PTE_NA, PTE_NA  },          /*  3   KR      */
{   PTE_RW, PTE_RW, PTE_RW, PTE_RW  },          /*  4   UW      */
{   PTE_RW, PTE_RW, PTE_NA, PTE_NA  },          /*  5   EW      */
{   PTE_RW, PTE_R,  PTE_NA, PTE_NA  },          /*  6   ERKW    */
{   PTE_R,  PTE_R,  PTE_NA, PTE_NA  },          /*  7   ER      */
{   PTE_RW, PTE_RW, PTE_RW, PTE_NA  },          /*  8   SW      */
{   PTE_RW, PTE_RW, PTE_R,  PTE_NA  },          /*  9   SREW    */
{   PTE_RW, PTE_R,  PTE_R,  PTE_NA  },          /* 10   SRKW    */
{   PTE_R,  PTE_R,  PTE_R,  PTE_NA  },          /* 11   SR      */
{   PTE_RW, PTE_RW, PTE_RW, PTE_R   },          /* 12   URSW    */
{   PTE_RW, PTE_RW, PTE_R,  PTE_R   },          /* 13   UREW    */
{   PTE_RW, PTE_R,  PTE_R,  PTE_R   },          /* 14   URKW    */
{   PTE_R,  PTE_R,  PTE_R,  PTE_R   }           /* 15   UR      */
};
#endif

#define TB_READ    0
#define TB_WRITE   1
#define TB_INVALID 2

static char * mode_name[] = { "READ", "WRITE", "-NONE-" };
static char * cur_name[] = { "KERNEL ", "EXEC ", "SUPER ", "USER " };


static char * prot_name[] =
        { "NONE",    "RESERVED",    "KW",       "KR", 
          "ALL",     "EW",          "ERKW",     "ER", 
          "SW",      "SREW",        "SRKW",     "SR", 
          "URSW",    "UREW",        "URKW",     "UR" };

//  This is called from the SHOW TB command to display the translation
//  buffer contents.

void dump_tb(void)
{
    short n;
    ULONGWORD va;
    
    for( n = 0; n < 128; n++ ) {

        if( tb[ n ].page == -1L )
            continue;
        va = (( ULONGWORD ) n >> 5 ) & 0x00000003;
        va = va << 30;
        va = va + ( tb[ n ].page << 9 );
        
        printf( "    TB(%02X)  VA=%08X  PA=%08X  PROT=%-4s  MODE=%s%s\n",
                 n, va, tb[ n ].paddr, prot_name[ tb[ n ].code],
                    tb[ n ].prot_valid != TB_INVALID ?
                        cur_name[ vax.pslw.cur_mod ] : "",
                        mode_name[ tb[ n ].prot_valid ] );

    }

    return;
}



//  This call will invalidate the protection valid bit for the cache.  This 
//  means that the next time we use the TB we must manually re-check the
//  protection mask.  This lets us save time with a protection check on most
//  reads.  You must invalidate the TB protection data whenever the PSL cur_mod
//  field changes, which changes the user's mode.  This is taken care of in the
//  write_psl_bits() routine which makes sure the bits are current.

void invalidate_tb_prot(void)
{
    short n;
    
    for( n = 0; n < 128; n++ ) {
        tb[ n ].prot_valid = TB_INVALID;
    }

    STC_FLUSH;

    return;
}



//  This call will invalidate the translation buffer cache if needed.  This is
//  done whenever a context switch occurs, or any of the base registers, etc.
//  are modified.

void invalidate_tb(void)
{
    short n;
    
    tb_flush++;
    for( n = 0; n < 128; n++ ) {
        tb[ n ].page = -1L;
    }
    
    STC_FLUSH;
    
    return;
}

//  This call will invalidate the translation cache for a particular address.
//  This is required when the operating system wants to modify a PTE; it writes
//  an address in the offendin page to the privileged register TBIA.  This causes
//  a call to this routine to invalidate the cache for that page slot.

void invalidate_page( ULONGWORD addr )
{

    LONGWORD region, page, tb_idx;
    
    region = ( addr >> 30 ) & 0x03L;
    page   = ( addr & 0x3FFFFFFFL ) >> 9;

    tb_idx = ( region << 5 ) + ( page & 0x0000001FL );
    tb[ tb_idx ].page = -1L;
    
    STC_FLUSH;

}


//  Routine to convert a virtual address to a physical address.  If the
//  translation can't be done (page fault) then set up the fault and 
//  return VAX_FAULT.  Otherwise, modify the caller's argument to be the
//  new physical address.
//
//  Size is number of bytes that we must ensure are valid.  If the request
//  can't be satisfied from one page, then size is adjusted to show how many
//  bytes remain to be satisfied.


LONGWORD vm( ULONGWORD * Addr, short * size, short mode )
{

    LONGWORD region, page, byte;
    ULONGWORD addr, va, pteaddr;
    LONGWORD the_lr, the_br, rc;
    short   n, valid, fault;
    unsigned char * r;
    unsigned char * p, * pte_addr;
    short tb_idx;
    float ratio;
    ULONGWORD vpage;
    union PTE pte;    
    unsigned short cm, rm, wm, code;
    LONGWORD paddr;
    struct TB * tbp;
    LONGWORD signal = 1;
    
	n = *size;

/*
 *  First, if virtual memory isn't on, then we have no work to do.
 */
 
    if( vax.MAPEN == 0L )
        return VAX_OK;

/*
 *  Okay, work to do.  First, let's break apart the address.
 */

    va = addr = *Addr;
    
    vpage = ( addr & 0xFFFFFE00 );
    byte   = ( addr & 0x1FF );

/*
 *  Many address operations are effectively sequential.  This is true of
 *  instruction decoding, character data reads, etc.  So let's start by
 *  seeing if this is the same as the last page we just did.  Note that
 *  the mode must also exactly match; i.e. if this is a write but the
 *  last was read, etc. then we can't use the STC cache.
 */

#if STC
    cached_page_try++;
    if( (ULONGWORD) vpage == (ULONGWORD) cached_virtual_page  &&
        ( cached_mbit == mode )) {
            *Addr = ( cached_physical_page + byte );
            cached_page_hit++;
            return VAX_OK;
    }
#endif

/*
 *  Sequential translation cache didnt' find anything, so finish getting
 *  ready for the translation.  We might be given the mode with extra bits 
 *  set.  Check it out.  Also, finish breaking up the address fields we need.
 */

    if( mode & VM_NOSIGNAL ) {
        mode = ( short )( mode & ~VM_NOSIGNAL );
        signal = 0;
    }

#if STC
    if( signal ) {                      /* If this is real (non PROBEx),   */
        cached_virtual_page = vpage;    /* access, remember for next time! */
        cached_mbit = mode;
    }
#endif

    region = ( addr >> 30 ) & 0x00000003L;
    page   = ( addr & 0x3FFFFFFFL ) >> 9;
    
/*
 *  Let's do a little translation buffer caching.  Since the work of
 *  doing a translation is VERY costly, let's see if we already have
 *  done this particular translation.  If we ever get a translation
 *  fault then we flush the buffer and do the next translations the
 *  hard way.
 */


    tb_idx =  ( short ) (( region << 5 ) + ( page & 0x00001FL ));    /* Index into TB cache */
    tbp = &( tb[ tb_idx ]);
    
    if( !vax.TBDR ) {

        tb_try++;

        /* To support PTE(M) setting, we only honor the cache if the mode */
        /* matches what we already did.  This prevents the M bit in the   */
        /* TB from getting updated while the one in the real PTE stales.  */
    
        if( tbp-> page == page && tbp-> prot_valid == mode ) {
            addr = tbp-> paddr + byte;
            *Addr = addr;

#if STC    
            cached_virtual_page = vpage;
            cached_physical_page = tbp-> paddr;
            cached_mbit = mode;
#endif            

            /* Track how we did with the TB for accounting    */
            /* and then let's get out of here.                */
            
            tb_hit++;            
            
            if( vax.debug & DBG_TB ) {
                if( tb_try )
                    ratio = ( float ) (( float ) tb_hit / ( float ) tb_try * 100.0);
                else
                    ratio = 0.0;
                printf( "DEBUG(TB):  TB CACHE HIT IDX=%02X; VA=%08X  PA=%08X RATIO=%d%%\n",
                        tb_idx, va, addr, ( LONGWORD ) ratio );
            }
            return VAX_OK;
        }
    }
    
/*
 *  Depending on which region it's in, do the work.
 */

    switch( region ) {
    
    case 0: /* P0 */
    case 1: /* P1 */

            /*  Depending on which region, look at the LR and BR values.            */
            /*  See if the page number violates the base register length.  Note     */
            /*  that for P1 space, the base register is the list of INVALID page    */
            /*  entries, so the sense of the test must be inverted.                 */
           
            fault = 0;
            if( region == 0 ) {
                the_br = vax.P0BR;
                the_lr = vax.P0LR;
                if( page > the_lr )
                    fault = 1;
            }
            else {
                the_br = vax.P1BR;
                the_lr = vax.P1LR;
                if( page <= the_lr ) /* was <= the_lr */
                    fault = 1;
            }
               
            if( fault ) {
                if( signal )
                    set_fault( EXC_ACCVIO, 2, addr, 0x0001 );
                tbp-> page = -1L;    /* Invalidate cache */
                return VAX_FAULT;
            }

    /*  Get the address of the page table entry.  This is actually a */
    /*  system virtual address, so we must recursively translate it, */
    /*  accessing the page in kernel mode.                           */
    
                        
            paddr = pteaddr = the_br + ( page * 4 );

            n = sizeof( pte );
            rc = vm( ( ULONGWORD * ) &paddr, &n, TB_READ );
            if( rc ) {
                tbp-> page = -1L;    /* Invalidate cache */  

                STC_FLUSH;
                return rc;
            }
            
    /*  Now we have the actual physical address of the page table entry */

            pte_addr = p = PHYADDR( paddr );               
            r = ( unsigned char * ) &pte.longword;
            

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif



    /*  If we are tracing the vm operations, do so now. */
    
            if( vax.debug & DBG_VM ) 
                printf( "DEBUG(VM): VA=%08X  R=%02d PTEA=%08X PTE=%08X P=%02X M=%02X PA=%08X\n",
                        va, region, pteaddr, pte.longword,
                        pte.bit.prot, mode,
                        ((( LONGWORD ) ( pte.bit.pfn )) << 9 ) + byte );

    /*  See if the protection bits allow this access            */
    
#if VM_TABLE_ACCESS
            code = access_map[ pte.bit.prot ][ vax.pslw.cur_mod ];
            if( mode == 0 )
                valid = ( code > PTE_NA );  /* for Read just need access */
            else
                valid = ( code > PTE_R  );  /* for write, must be > read */
#else
            code = ( unsigned short ) pte.bit.prot;
            rm = ( unsigned short ) (( code >> 2 ) & 0x03 );
            wm = ( unsigned short ) (~( code & 0x03 ) & 0x03 );
            cm = ( unsigned short ) vax.pslw.cur_mod;
            valid = 0;
            if(
                ( code != 0 ) &&
                (   ( code == 4 ) ||
                    ( cm < wm ) ||
                    ( mode == 0 && ( cm <= rm )))) 
                    valid = 1;
#endif


            if( !valid ) {
            
                if( signal )
                    set_fault( EXC_ACCVIO, 2, addr, 0x0002 );
                
                tbp-> page = -1L;    /* Invalidate cache */
                STC_FLUSH;         
                return VAX_FAULT;
            }
            
    /*  See if the page is valid.  If not, then translation fault */
    
            if( pte.bit.v == 0 ) {
            
                rc = validate_page( &pte, paddr );
                tbp-> page = -1L;    /* Invalidate cache */
                STC_FLUSH;         
                
                if( rc ) {
                    if( signal )
                        set_fault( EXC_TNV, 2, addr, 0x0000 );
                    
                    return VAX_FAULT;
                }
            }
    
    /*   Construct the real physical address of the page we want.  */
    /*   Also, save it in the translation buffer for this region.  */
    

            paddr = ((( LONGWORD ) ( pte.bit.pfn )) << 9 );
            tbp-> paddr = paddr;
            tbp-> page = page;
            tbp-> code = pte.bit.prot;
            tbp-> prot_valid = mode;   /* we just checked it */
            *Addr = paddr + byte;
#if STC    
            cached_virtual_page = vpage;
            cached_physical_page = paddr;
            cached_mbit = mode;

#endif

    /*  If this is a write operation and the page has not been written */
    /*  to yet, set the M bit and update the PTE */

            if( pte.bit.m == 0 && mode == 1 ) {  /* WRITE */

                pte.bit.m = 1;
                p = pte_addr;
                r = ( unsigned char * ) &pte.longword;
            
#if BIGENDIAN
                for( n = 1; n < 5; n++ ) {
                    *(p++) = r[ 4-n ];
                }
#else
                for( n = 0; n < 4; n++ ) {
                    *(p++) = r[ n ];
                 }
#endif
             }


            break;  
    
    
    
    case 2: /* S0 */

    /*  See if the page number violates the SBR length */
    
            if( ( ULONGWORD ) page > vax.SLR ) {
                if( signal )
                    set_fault( EXC_ACCVIO, 2, addr, 0x0001 );
                
                tbp-> page = -1L;    /* Invalidate cache */
                STC_FLUSH;
                return VAX_FAULT;
            }
    
    /*  Find the VAXaddr that is the base address of the page table */
    /*  Each PTE is four bytes LONGWORD, so scale appropriately         */
    
            paddr = vax.SBR + ( page * 4 );
            p = pte_addr = PHYADDR( paddr );
            r = ( unsigned char * ) &pte.longword;

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif


    /*  If we are tracing the vm operations, do so now. */
    
            if( vax.debug & DBG_VM ) 
                printf( "DEBUG(VM): VA=%08X  R=%02d PTEA=%08X PTE=%08X P=%02X M=%02X PA=%08X\n",
                        va, region, paddr, pte.longword,
                        pte.bit.prot, mode,
                        ((( LONGWORD ) ( pte.bit.pfn )) << 9 ) + byte );

    /*  See if the protection bits allow this access            */

#if VM_TABLE_ACCESS
            code = access_map[ pte.bit.prot ][ vax.pslw.cur_mod ];
            if( mode == 0 )
                valid = ( code > PTE_NA );  /* for Read just need access */
            else
                valid = ( code > PTE_R  );  /* for write, must be > read */
#else
            code = ( unsigned short ) pte.bit.prot;
            rm = ( unsigned short ) (( code >> 2 ) & 0x03 );
            wm = ( unsigned short ) (~( code & 0x03 ) & 0x03 );
            cm = ( unsigned short ) vax.pslw.cur_mod;
            valid = 0;
            if(
                ( code != 0 ) &&
                (   ( code == 4 ) ||
                    ( cm < wm ) ||
                    ( mode == 0 && ( cm <= rm )))) 
                    valid = 1;
#endif

            if( !valid ) {
                if( signal )
                    set_fault( EXC_ACCVIO, 2, addr, 0x0002 );
                tbp-> page = -1L;    /* Invalidate cache */
                STC_FLUSH;
                return VAX_FAULT;
            }
            
    /*  See if the page is valid.  If not, then translation fault */
    
            if( pte.bit.v == 0 ) {
            
#ifdef DYNVM
                validate_page( &pte, paddr );
                tbp-> page = -1L;
                STC_FLUSH;
#else
                if( signal )
                    set_fault( EXC_TNV, 2, addr, 0x0000 );
                
                tbp-> page = -1L;    /* Invalidate cache */
                STC_FLUSH;
                return VAX_FAULT;
#endif
            }
    
    /*   Construct the real physical address of the page we want */
    
            paddr = ((( LONGWORD ) ( pte.bit.pfn )) << 9 );
            tbp-> paddr = paddr;
            tbp-> page = page;
            tbp-> code = pte.bit.prot;
            tbp-> prot_valid = mode;   /* we just checked it */

            *Addr = paddr + byte;
#if STC    
            cached_virtual_page = vpage;
            cached_physical_page = paddr;
            cached_mbit = mode;
#endif            

    /*  If this is a write operation and the page has not been written */
    /*  to yet, set the M bit and update the PTE */

            if( pte.bit.m == 0 && mode == 1 ) {  /* WRITE */

                pte.bit.m = 1;
                p = pte_addr;
                r = ( unsigned char * ) &pte.longword;
            
#if BIGENDIAN
                for( n = 1; n < 5; n++ ) {
                    *(p++) = r[ 4-n ];
                }
#else
                for( n = 0; n < 4; n++ ) {
                    *(p++) = r[ n ];
                 }
#endif
             }


            break;

    default:  /* S1 */
    
            /* ERROR */
            if( signal )
               set_fault( EXC_ACCVIO, 2, addr, 0x0001 );
                
            tbp-> page = -1L;    /* Invalidate cache */
            STC_FLUSH;
            return VAX_FAULT;
    }
    
    return VAX_OK;
}
    

//  Routine to probe addressability of arguments

LONGWORD probe( ULONGWORD addr, short size, short mode )
{
    ULONGWORD va;
    short sz;
    LONGWORD rc;
    
    if( mode == 0  )
        return VAX_OK;

    
    if( vax.MAPEN == 0 ) {
        unsigned char * pb;
        pb = PHYADDR( addr );
        if( pb == 0L ) 
            return VAX_ACCVIO + ( mode << 2 );
    }
    else {
        va = addr;
        sz = size;
        rc = vm( &addr, &sz, ( short ) ( mode | VM_NOSIGNAL ));
        return rc;
    } 
    
    return VAX_OK;
}

//  This is nearly a clone of the vm() routine, but used to trace a
//  page table entry resolution.  This is used by SHOW PTE, for example.

LONGWORD tracevm( ULONGWORD Addr, short mode )
{

    LONGWORD region, page, byte;
    ULONGWORD addr, va, pteaddr;
    LONGWORD the_lr, the_br, rc;
    short   n, valid, fault;
    unsigned char * r;
    unsigned char * p;
    short tb_idx;
    
    unsigned short cm, rm, wm, code;
    LONGWORD paddr;
    
    union PTE pte;

    static char * prot_name[] =
        { "NONE",    "RESERVED",    "KW",       "KR", 
          "ALL",     "EW",          "ERKW",     "ER", 
          "SW",      "SREW",        "SRKW",     "SR", 
          "URSW",    "UREW",        "URKW",     "UR" };

        
/*
 *  First, if virtual memory isn't on, then we have no work to do.
 */
 
    if( vax.MAPEN == 0L ) {
        printf( "Virtual memory is disabled.\n" );
        return VAX_OK;
    }
 
/*
 *  Okay, work to do.  First, let's break apart the address.
 */

    va = addr = Addr;
    
    region = ( addr >> 30 ) & 0x00000003L;
    page   = ( addr & 0x3FFFFFFFL ) >> 9;
    byte   = ( addr & 0x1FF );

/*
 *  Let's do a little translation buffer caching.  Since the work of
 *  doing a translation is VERY costly, let's see if we already have
 *  done this particular translation.  If we ever get a translation
 *  fault then we flush the buffer and do the next translations the
 *  hard way.
 */

    tb_idx =  ( short ) (( region << 5 ) + ( page & 0x00001FL ));    /* Index into TB cache */
    
/*
 *  Depending on which region it's in, do the work.
 */

    switch( region ) {
    
    case 0: /* P0 */
    case 1: /* P1 */

            /* Depending on which region, get the LR and BR values */
            
            if( region == 0 ) {
                the_br = vax.P0BR;
                the_lr = vax.P0LR;
            }
            else {
                the_br = vax.P1BR;
                the_lr = vax.P1LR;
            }

    /*  See if the page number violates the base register length.  Note     */
    /*  that for P1 space, the base register is the list of INVALID page    */
    /*  entries, so the sense of the test must be inverted.                 */
    
            fault = 0;
            if( region == 0 && page > the_lr ) {
                fault = 1;
            }
            else
            if( region == 1 && page <= the_lr )
                fault = 1;
                
            if( fault ) {
                printf( "ACCVIO, page table P%d length violation\n", region );
                return VAX_OK;
            }

    /*  Get the address of the page table entry.  This is actually a */
    /*  system virtual address, so we must recursively translate it, */
    /*  accessing the page in kernel mode.                           */
    
                        
            paddr = pteaddr = the_br + ( page * 4 );

            n = sizeof( pte );
            rc = vm( ( ULONGWORD * ) &paddr, &n, 
                                VM_READ | VM_NOSIGNAL );
            if( rc ) {
                return rc;
            }
            
    /*  Now we have the actual physical address of the page table entry */

            p = PHYADDR( paddr );
            r = ( unsigned char * ) &pte.longword;
            

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif

            printf( "    Virtual address:   %08X\n", va );
            printf( "    Region:            %02d\n",  region );
            printf( "    PTE Address:       %08X\n", pteaddr );
            printf( "    PTE Longword:      %08X\n", pte.longword );
            printf( "        VALID: %d\n",    pte.bit.v );
            printf( "        PROT:  %02X (PTE$K_%s) \n",  pte.bit.prot,
                               prot_name[ pte.bit.prot] );
            printf( "        M:     %d\n",    pte.bit.m );
            printf( "        OWNER: %1X\n",   pte.bit.own );
            printf( "        S:     %1X\n",   pte.bit.s );
            printf( "        PFN:   %08X\n", pte.bit.pfn );
            printf( "    Physical address:  %08X\n", 
                        ((( LONGWORD ) ( pte.bit.pfn )) << 9 ) + byte );

    /*  See if the protection bits allow this access            */
    

            code = ( unsigned short ) pte.bit.prot;
            
            rm = ( unsigned short ) (( code >> 2 ) &0x03 );
            wm = ( unsigned short ) (~( code & 0x03 ) & 0x03 );
            cm = ( unsigned short ) vax.pslw.cur_mod;
            
            printf( "    Read  mode:  %02x\n", rm );
            printf( "    Write mode:  %02X\n", wm );
            printf( "    PSL cur_mod: %02X\n", cm );
            printf( "    Access:      %s\n", mode ? "VM_WRITE" : "VM_READ" );
            if(
                ( code != 0 ) &&
                (   ( code == 4 ) ||
                    ( cm < wm ) ||
                    ( mode == 0 && ( cm <= rm )))) 
                    valid = 1;
            else {
            
                printf( "ACCVIO, protection violation\n" );
                
                return VAX_OK;
            }
            
    /*  See if the page is valid.  If not, then translation fault */
    
            if( pte.bit.v == 0 ) {
            
                STC_FLUSH;         
                printf( "TNV, valid bit not set\n" );
                return VAX_OK;
            }
    
            break;  
    
    
    
    case 2: /* S0 */

    /*  See if the page number violates the SBR length */
    
            if( ( ULONGWORD ) page > vax.SLR ) {
                printf( "ACCVIO, page table S0 length violation\n" );
                return VAX_OK;
            }
    
    /*  Find the VAXaddr that is the base address of the page table */
    /*  Each PTE is four bytes LONGWORD, so scale appropriately         */
    
            pteaddr = paddr = vax.SBR + ( page * 4 );

            p = PHYADDR( paddr );
            r = ( unsigned char * ) &pte.longword;

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif


    /*  If we are tracing the vm operations, do so now. */

            printf( "    Virtual address:   %08X\n", va );
            printf( "    Region:            %02d\n",  region );
            printf( "    PTE Address:       %08X\n", pteaddr );
            printf( "    PTE Longword:      %08X\n", pte.longword );
            printf( "        VALID: %d\n",    pte.bit.v );
            printf( "        PROT:  %02X (PTE$K_%s) \n",  pte.bit.prot,
                               prot_name[ pte.bit.prot] );
            printf( "        M:     %d\n",    pte.bit.m );
            printf( "        OWNER: %1X\n",   pte.bit.own );
            printf( "        S:     %1X\n",   pte.bit.s );
            printf( "        PFN:   %08X\n", pte.bit.pfn );
            printf( "    Physical address:  %08X\n", 
                        ((( LONGWORD ) ( pte.bit.pfn )) << 9 ) + byte );

    /*  See if the protection bits allow this access            */
    
            code = ( unsigned short ) pte.bit.prot;
            

            rm = ( unsigned short ) (( code >> 2 ) & 0x03 );
            wm = ( unsigned short ) (~( code & 0x03 ) &0x03 );
            cm = ( unsigned short ) vax.pslw.cur_mod;
            
            printf( "    Read  mode:  %02x\n", rm );
            printf( "    Write mode:  %02X\n", wm );
            printf( "    PSL cur_mod: %02X\n", cm );
            printf( "    Access:      %s\n", mode ? "VM_WRITE" : "VM_READ" );

            if(
                ( code != 0 ) &&
                (   ( code == 4 ) ||
                    ( cm < wm ) ||
                    ( mode == 0 && ( cm <= rm )))) 
                    valid = 1;
            else {
            
                printf( "ACCVIO, protection violation\n" );
                return VAX_FAULT;
            }
            
    /*  See if the page is valid.  If not, then translation fault */
    
            if( pte.bit.v == 0 ) {
                printf( "TNV, valid bit not set\n" );
                return VAX_OK;
            }
    
            
            break;

    default:  /* S1 */
    
            /* ERROR */
            printf( "ACCVIO, invalid S1 region\n" );
            return VAX_OK;
                
    }
    
    return VAX_OK;
}
    

/*
 *  Routine that converts a physical VAX address into a native host address.
 *
 *  Currently this includes:
 *
 *      main memory         vax.memory[]
 *      ROM memory          rom[]
 *      NVRAM memory        nvram[]
 * 
 */

unsigned char * get_address( ULONGWORD addr )
{

    extern char * microvax;
    
    /*  If it's in the console ROM area, handle that.   */
    
    if( rom && addr >= vax.rom_base && addr <= vax.rom_end )
        return ( unsigned char * ) &( rom[ addr - vax.rom_base ]);
    
    /*  If it's in the NVRAM area, handle that.         */
    
    if( nvram && addr >= vax.nvram_base && addr <= vax.nvram_end )
        return ( unsigned char * ) &( nvram[ addr - vax.nvram_base ]);
    
    /* If it's in regular physical memory, do that.  Normally this won't    */
    /* be the case because the macro PHYADDR will have gotten this already  */
    
    if( addr < vax.memsize )
        return (unsigned char * ) &( vax.memory[ addr ]);
    
    /*  See if it's the special microvax area.  Not sure what this is yet... */
    
    if( addr >= 0x20080000 && addr <= 0x200800ff )
        return ( unsigned char * ) &( microvax[ addr - 0x20080000]);
        
    /*  It's not a RAM or ROM location.  Return null, which causes */
    /*  the caller to use load_io or store_io instead.              */
    
    return 0L;
}


/*
 *  Microkernel support routine.  If we get a TNV fault, this routine is called 
 *  to determine if the emulator owns making the page valid.  If it doesn't then
 *  it just returns VAX_FAULT and the TNV fault continues.
 *
 *  If we are in the microkernel with VM initalized, then we can try to find a
 *  physical page to map to the virtual page.
 */
 
LONGWORD validate_page( union PTE * pte, LONGWORD paddr )
{
    extern LONGWORD mapsize;
    extern char * page_map;
    int n, saved_mapen;

#ifndef DYNVM
    return VAX_FAULT;
#endif

    if( !VMVALID )
        return VAX_FAULT;
        
    if( page_map == 0L )
        return VAX_FAULT;
        
    /* Search for an un-mapped page */
            
    for( n = 1; n < mapsize; n++ ) {
    
        if( page_map[ n ] == 0 ) {
            pte-> bit.v = 1;     /* Make PFN entry valid */
            pte-> bit.pfn = n;   /* And map to a physical page */
            page_map[ n ] = 1;  /* which we mark as in-use */
            
            saved_mapen = (unsigned int) vax.MAPEN;
            vax.MAPEN = 0;
            
            /* Update the PTE in memory with the PFN info */
            
            store_memory( paddr, ( void * ) &(pte->longword), 4 );
            vax.MAPEN = saved_mapen;
            
            if( vax.debug & DBG_VM )
                printf( "DEBUG(VMPAGE): mapping physical page %08X in PTE at %08X\n", n, paddr );
            
            return VAX_OK;
        }
        
    }
    
    /* Never found one, bad */
    
    return VAX_FAULT;
}


LONGWORD mapped_pages( void )
{
    LONGWORD count = 0;
    int n;
    
    if( !VMVALID )
        return -1L;
    
    for( n = 1; n < mapsize; n++ ) {
        if( page_map[ n ] )
            count++;
    }
    
    return count;
}

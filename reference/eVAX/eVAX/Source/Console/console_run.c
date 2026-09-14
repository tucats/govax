
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_run.c
//
//  Purpose:    This module implements the RUN command which loads images
//              into P0 space as user-mode code and executes them.
//
//  History:    07/16/99    Extracted from console_load.c
//
//              07/21/99    Fixed numerous problems with recursive processing
//                          of sharable images
//
//              12/09/99    Major bug fixes to image loading, basing of
//                          shareable images, and inter-image fixups that
//                          do not use shims.
//
//              01/10/00    If the sharable image prefix is an escape (0x1b)
//                          then don't try to find the sharable images.
//
//		07/06/01    Added awareness of LIB$INITIALIZE entry points, 
//                          ability to optionally run them.
//
//		07/26/01    When writing back region sizes, do it in kernel mode
//			    so we have write access to region memory slots.
//
//		12/04/01    To better debug how LIB$INITILIZE happens, changing how
//			    the init modules are called.  We now write a small
//			    program at CONSOLE$SCRATCH that calls each of the entry
//			    points, and then calls the main program.  This lets
//			    breakpoints function correctly when placed in the
//			    initializion areas.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "memmap.h"
#include "imgdef.h"
#include "shim.h"

#include <time.h>
static long init_region_addr( void );

LONGWORD region_size_addr[ 4 ] = { 0L, 0L, 0L, 0L };
struct ICB * icb_list = 0L;
static char * image_file_found( char * fn );

/*
 *  Find main ICB - finds the ICB marked as the main/primary image
 */

struct ICB * find_main_icb(void)
{

    struct ICB * icb;
    
    for( icb = icb_list; icb; icb = icb-> next ) {
        if( icb-> flags & ICB_MAIN )
            break;
    }

    if( vax.debug & DBG_IMAGES )
        printf( "Main image is %s\n", icb-> name );
        
    return icb;
}

/*
 *  dump the current ICB list
 */

void dump_icb_list( int full )
{

    struct ICB * icb;
    struct SHR * shr;
    int n;
    
    if( icb_list == 0L ) {

        printf( "No VMS images loaded in memory.\n" );
        return;
    }

    printf( "ACTIVE IMAGES IN MEMORY:\n" );

    for( icb = icb_list; icb; icb = icb->next ) {

        printf( "    %-39s  %08X  %08X\n", icb-> name, icb-> base, icb-> end );

        if( full ) {
        
            if( icb-> transfer[ 0 ] == 0 )
                printf( "        no transfer addresses\n" );
            else {
                printf( "        transfer address%s ",
                    icb-> transfer[1] ? "es":"" );
                for( n = 0; n < 4; n++ )
                    if( icb-> transfer[ n ] )
                        printf( "%08X  ", icb-> transfer[ n ]);
                printf( "\n" );
            }
            if( icb-> shr_list ) {
    
                /* Must be at least two entries to count */
    
                if( icb-> shr_list-> next ) {
                    printf( "        depends on " );
    
                    for( shr = icb-> shr_list; shr; shr = shr-> next ) {
    
                        /* Skip over the self-reference SHR entry */
                        if( shr-> id == 0 )
                            continue;
    
                        if( shr != icb-> shr_list )
                            printf( ", " );
    
                        printf( "%s", shr-> name );
                    }
                    printf( "\n" );
                }
            }
	    }
    }
    return;

}

/*
 *  reset_icb_list
 *
 *  Reset the ICB list if it's got data in it.
 */

void reset_icb_list( void )
{
    extern LONGWORD vms_exit_handler;

    struct ICB * icb, * icb_next;
    struct SHR * shr, * shr_next;
    struct ISD * isd, * isd_next;
    
    for( icb = icb_list; icb; icb = icb_next ) {
        icb_next = icb-> next;
        
        for( isd = icb-> isd_list; isd; isd = isd_next ) {
            isd_next = isd-> next;
            if( isd-> valid != CHAR4('U','S','E','D'))
                continue;
            isd-> valid = CHAR4('F','R','E','E');
            freemem( isd );
        }
        
        for( shr = icb-> shr_list; shr; shr = shr_next ) {
            shr_next = shr-> next;
            freemem( shr );
        }
        icb-> valid = CHAR4('F','R','E','E');
        freemem( icb );
    }
    
    icb_list = 0L;

    /* While we're here, initialize the other "user process" attributes */

    vms_exit_handler = 0L;              /* No exit handler declared     */
    set_region_size( 0, 0L );           /* P0 is empty                  */
    vax.console.image_load = 0L;      /* Next load location is 0      */
    
    return;
}


/*
 *  RUN <filename>
 */

 
LONGWORD console_run( char ** P )
{
    LONGWORD debug;
    char * p, *fn;
    char sb[ 256 ], *sbp;
    struct ICB * main_image, *icb;
    LONGWORD rc;
    LONGWORD addr;
    LONGWORD saved_mode;
    LONGWORD verb, step, load_only, run_inits;
    char * savedp;
        
    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        return VAX_NOVAX;
    }
    
    if( !MKVALID )
        return VAX_NOMK;
    
    debug = vax.debug & DBG_IMAGES;
    run_inits = vax.debug & DBG_LIBINIT;
    
    
//  Initialize the structure mapping, if necessary

    init_ihd_maps();

//  If the ICB list is non-empty, first take time to clear it out.

    reset_icb_list();

//  Initialize the fake runtime environment.  We don't know if we'll 
//  use it yet, of course.

    lib_initialize();
    
//  Do the work.  Start by fetching the filename.
    

    flush_blanks( &p );
    step = 0;
    load_only = 0;
    
    if( *p == '/' ) {
        savedp = p;
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        if( verb == CHAR4( '/','N','O','I' )) {
            run_inits = 0;
            flush_blanks( &p );
        }
        if( verb == CHAR4( '/','I','N','I' )) {
            run_inits = 1;
            flush_blanks( &p );
        }
        else
        if( verb == CHAR4( '/','B','R','E' ) || 
            verb == CHAR4( '/','D','E','B' ) ||
            verb == CHAR4( '/','S','T','E' )) {
            step = 1;
            flush_blanks( &p );
        }
        else
        if( verb == CHAR4( '/', 'N','O','E')) {     /* /NOEXECUTE */
            load_only = 1;
            flush_blanks( &p );
        }
        else
            p = savedp;
    }
    
    if( isend( *p )) {
        printf( "Missing file name to run\n" );
        return VAX_SYNTAX;
    }
    else {
        fn = p;
        if( *p == '"' )
            p++;
        for( fn = p; *p; p++ )
            if( *p == '\n' || *p == '"' )
                *p = 0;
    }
    
    *P = p;

//  Save our current mode (should be user) and get into kernel mode.

    saved_mode = vax.pslw.cur_mod;
    set_mode_stack( 0 );

//  Try to load the image.  This will recursively call itself as needed
//  for secondary images.  The first image is based off of zero, and we
//  load each ISD in successive pages as we go.
    
    vax.console.image_load = 0L;
    set_region_size( 0, 0L );

    rc = image_load( fn, ICB_MAIN, 0L );
    if( rc ) {
        printf( "Unabled to activate %s, %08X\n", fn, rc );
        goto abort;
    }

//  Now walk the list of images again, doing fixups on each one.  Since
//  images are stored on the ICB list in the reverse order that they were
//  loaded (insert-at-front) then we can walk the list front to back and
//  fixup each image before we fixup the image(s) that depend on it.

    for( icb = icb_list; icb; icb = icb-> next ) {
        rc = image_fixup( icb );
        if( rc ) {
            printf( "Error doing relocation/fixups for image %s, %08X\n",
                    icb-> name, rc );
            goto abort;
        }
    }
        
//  Find the main image

    main_image = find_main_icb();
    
        
//  Set the mode back to what it was when we started, probably USER mode...

    set_mode_stack( saved_mode );

//  If there was no main image, we screwed up somehow.  Bail.

    if( !main_image ) {
        printf( "No main image loaded\n" );
        goto abort;
    }

//  Search the ICB list (skipping the MAIN program) and call everyone's LIB$INITIALIZE
//	entry point if they have one.  This is done by writing a small program into the
// 	kernel scratch area that calls each init function if there is one.  It is 
//	ended by calling the program's real main function.
    
    //  If the symbol already exists, toss it away.
    clear_symbol( "IMAGE$MAIN" );
    clear_symbol( "IMAGE$INIT" );
    
    //  Make a valid procedure out of the initialization code.  Leave a little room in
    //  the scratch area and then write a little program for ourselves.
    
    assemble_direct( ".BASE CONSOLE$SCRATCH+08" );
    rc = assemble_direct( ".ENTRY  IMAGE$INIT, ^m<r2,r3>" );
    if( rc )
        assemble_direct( ".word 0" );
        
    if( run_inits ) {
        for( icb = icb_list-> next; icb; icb = icb-> next ) {
            if( icb-> transfer[ 0 ] ) {
                if( vax.debug & DBG_IMAGES ) {
                    printf( "Preparing call to LIB$INITIALIZE entry %08X for image %s\n",
                            icb-> transfer[ 0 ], icb-> name );
                }
                
                assemble_direct( "PUSHL #0" );
                assemble_direct( "PUSHL #0" );
                
                /* I don't yet know why, but apparantly LIBRTL gets 100 as a */
                /* special flag for some reason. */
                
                if( strcmp( icb-> name, "LIBRTL" ) == 0 )
                    assemble_direct( "PUSHL #100" );
                else {
                    sprintf( sb, "PUSHL #0%08lX", 0L /* icb-> transfer[ 0 ] */);
                    assemble_direct( sb );
                    }
                sprintf( sb, "calls #3, @#SHARE$%s_INITIALIZE", icb->name );
                assemble_direct( sb );
                
              //  rc = console_call(  &sbp );
            }
        }
    }
            
//  Now that we've loaded it all in memory, set up a symbol that defines the entry point.
//  For now, we look for the entry point in user mode, and ignore the LIB$INITIALIZE
//  or debugger entry points.

    addr = 0L;
    if( main_image-> transfer[ 0 ] > 0L && main_image-> transfer[ 0 ] < 0x3FFFFFFF )
        addr = main_image-> transfer[ 0 ];
    else
    if( main_image-> transfer[ 1 ] > 0L && main_image-> transfer[ 1 ] < 0x3FFFFFFF )
        addr = main_image-> transfer[ 1 ];
    else
    if( main_image-> transfer[ 2 ] > 0L && main_image-> transfer[ 2 ] < 0x3FFFFFFF )
        addr = main_image-> transfer[ 2 ];
    
    if( debug ) {
        printf( "\nImage execution starts at IMAGE$INIT,\n");
        printf( "   main program at address %08x\n\n", addr );
    }
    
//  Well, the user did say to RUN it, so set up a call to the entry point...

    if( !addr )
        printf( "No transfer address!\n" );
    else {
    
        rc = set_symbol_direct( "IMAGE$MAIN", addr, SYM_ENTRY );
        if( rc != VAX_OK ) {
            printf( "Error creating IMAGE$MAIN symbol, %08X\n", rc );
            goto abort;
        }
        
        assemble_direct( "calls #0, @#image$main" );
        assemble_direct( "ret" );

        /* If we are to step at the first instruction, add flag to CALL command */
        
        if( !load_only ) {
            sprintf( sb, "%sIMAGE$INIT",
                step ? "/STEP " : "" );
            sbp = sb;
            rc = console_call(  &sbp );
        }
    }


    return rc;
    
abort:
    set_mode_stack( saved_mode );
    return rc;

}




/**************************************************************************\
 *                                                                        *
 *                            LOAD AN IMAGE                               *
 *                                                                        *
\**************************************************************************/

/*
 *
 *  Here is the general strategy of the image loader.  Note that this is
 *  very distinct from the image fixup which handles load-time linking!
 *
 *
 *  1.  Read the first block of the image file into CONSOLE$SCRATCH.
 *      This lets us find out how many blocks the header really is.
 *
 *  2.  Starting at vax.console.image_load, read in the entire
 *      header.
 *
 *  3.  Scoop up the static info (transfer address, etc.) and if we
 *      are in debug mode, dump it out.  This is all stored in an
 *      image control block (ICB) that is stored on the icb_list.
 *
 *  4.  Stride over the image section descriptors (ISD's) in the
 *      header and make a list.  This list is hung off the current ICB.
 *
 *  5.  Re-scan the ICB list after it's created, processing each one.
 *      At this point we are done with the image header, so we start
 *      reading headers in at vax.console.image_load again.  This 
 *      time, as we read pages we advance image_load to track the
 *      memory high water mark.
 *
 *  6.  We skip GBL, DZRO, and USERSTACK sections.  When we find the
 *      (one and only) fixup section, we note which one it is in the
 *      icb-> fixup_isd pointer for later use.
 *
 *  7.  After reading in all sections, if there was a fixup_isd section,
 *      we read the VAX memory where it was written to build a list of
 *      sharable images (the SHR list) that must also be activated.  
 *
 *  8.  After building the entire SHR list, we re-scan it, recursively
 *      calling image_load() for each one on the list.  Note that we
 *      do this before doing any fixups, so that all possible shared
 *      images are actually in memory before fixups start.
 *
 *  Note that it is sort of important that when new ICB's are created
 *  by the recursive process, they actually are stored at the -front-
 *  of the ICB list as it's built.  In this way, when the image_fixup()
 *  routine runs, it will do fixups starting on the last (leaf-node)
 *  images in the sharable image hierarchy if possible, and will do the
 *  main image fixups last.
 *
 */

 
LONGWORD image_load( char * fn, LONGWORD flag, struct ICB ** icbptr )
{
    LONGWORD debug;
    char *sbp;
    
    LONGWORD size, rc;
    LONGWORD addr, paddr, naddr, base;
    LONGWORD saved_mode, n, zp;
    LONGWORD section_blocks;
    char nblocks;
    struct IHD ihd;
    struct IHI ihi;
    struct ISD * isd, static_isd;
    LONGWORD j;
    char shrname[ 40 ];
    struct SHR * shr;
    char sname[ 64 ];
    
    ULONGWORD file_addr;
    LONGWORD imageid = 0;
 
    
    struct ICB * icb, *icb_p;
    
    
    FILE *  fp;
    short bsize = 512;
  
    
    if( !vax_init ) {
        return VAX_NOVAX;
    }

    debug = vax.debug & DBG_IMAGES;

/*  Let's see if this one is already loaded. */

    for( icb = icb_list; icb; icb = icb-> next ) {
        if( strcmp( fn, icb-> name ) == 0 ) {
            if( debug ) {
                printf( "\nLOADING %s IMAGE \"%s\" -- ALREADY LOADED\n",
                    flag == ICB_MAIN ? "PRIMARY" : "SECONDARY", fn );
            }

            if( icbptr )
                *icbptr = icb;
            return VAX_OK;
        }
    }

/*  Not already loaded, so let's try to open it */

    sbp = find_image( fn );
    if( sbp == 0L ) {
        if( debug )
            printf( "Attempt to load %s image %s failed.\n",
                flag == ICB_MAIN ? "primary" : "secondary", fn );
                        
        return VAX_FNF;
    }
   
    fp = xfopen( sbp, "rb" );

//  We have reserved the use of the scratch page for reading in image blocks.  Get
//  it's address now.

    rc = get_symbol_direct( "CONSOLE$SCRATCH", &addr );
    if( rc ) {
        printf( "Unable to load image (no CONSOLE$SCRATCH area)\n" );
        return rc;
    }

//  Convert to physical address.  We'll need to be in Kernel mode for all this to
//  work correctly.
    
    saved_mode = vax.pslw.cur_mod;
    set_mode_stack( 0 );
    paddr = addr;
    rc = vm( ( ULONGWORD * ) &paddr, &bsize, OP_MD );
    if( rc ) {
        printf( "Unable to resolve physical address of SYS$SCRATCH for I/O\n" );
        set_mode_stack( saved_mode );
        return rc;
    }
    
//  Read in first header block.

    size = fread( &( vax.memory[ paddr ]), 512L, 1, fp );
    if( size <= 0 ) {
        printf( "Unable to read image header\n" );
        return VAX_OK;
    }
    
//  Read the fixed portion of the header.  This is sort of ugly, but we have to do
//  it this way to get endian stuff right, etc.

    rc = load_memory( addr + 16, ( void * ) &nblocks, 1 );
    if( rc ) {
        printf( "Error getting image header size, %08X\n", rc );
        goto abort;
    }

//  Now that we know how big the goober is, let's reload it all at a better location.

    /* We must add a page because the first load is based at ZERO  */
    /* and you can't write to page zero on VMS.  So we really load */
    /* the header one page into the un-used area.                  */

    base = vax.console.image_load + 0x200L;

    fseek( fp, 0L, 0 );
    if( debug ) {
        printf( "\n\n" );
        printf( "LOADING %s IMAGE %s (FILE \"%s\")\n",
		flag == ICB_MAIN ? "PRIMARY" : "SECONDARY", fn, sbp );
        printf( "\tImage Header (%d blocks) loaded at %08X\n",
                     nblocks, base );
    }

    for( n = 0; n < nblocks; n++ ) {
    
        addr = base + ( n * 512 );
        paddr = addr;
        rc = vm( ( ULONGWORD * ) &paddr, &bsize, OP_MD );
        if( rc ) {
            printf( "Unable to read entire header into memory, %08X\n", rc );
            goto abort;
        }
        
        size = fread( &( vax.memory[ paddr ]), 512L, 1, fp );
        if( size <= 0 ) {
            printf( "Unable to read block %d into memory at %08X\n", n, addr );
            goto abort;
        }
        
    }

//  Make an ICB entry for this, and put it on the list.

    icb = ( struct ICB * ) getmem( sizeof( struct ICB ));
    icb-> next = 0L;
    
    if( icb_list == 0L ) {
        icb_list = icb;
    }
    else {
        for( icb_p = icb_list; icb_p; icb_p = icb_p -> next ) 
            if( icb_p-> next == 0L )
                break;
        icb_p-> next = icb;
     }   
    
    if( icb-> valid == CHAR4('U','S','E','D')) {
        printf( "Invalid re-use of ICB memory!\n" );
    }
    icb-> valid = CHAR4('U','S','E','D');
    
    /* Note that here we use the real base addr (zero for first image) */
    icb-> base = vax.console.image_load;
    icb-> end = 0L;
    icb-> flags = flag | ICB_INCOMPLETE;
    icb-> isd_count = 0;
    icb-> isd_list = 0;
    icb-> fixup_isd = 0;
    icb-> shr_list = 0;

    if( flag == ICB_MAIN )
        strcpy( icb-> name, "<MAIN>" );
    else {
        if( strlen( fn ) < 39 )    
            strcpy( icb-> name, fn );
        else
            strcpy( icb-> name, "<INVALID>" );
    }
    
    if( icbptr )
        *icbptr = icb;
        
//  Map the fixed portion of the header, and copy it to the ICB as well.

    rc = map( "IHD", ( void * ) &ihd, base );
    if( rc ) {
        printf( "Error mapping IHD %08X\n", rc );
        goto abort;
    }

    icb-> ihd = ihd;
    
//  Read in the transfer array data.

    for( n = 0; n < 4; n++ ) {
    
        rc = load_memory( base + ihd.offset_transfer + ( n * 4 ),
                 ( void * ) &(icb-> transfer[n]), 4 );
        if( rc ) {
            printf( "Error reading transfer array, %08X\n", rc );
            goto abort;
        }
        if( icb-> transfer[ n ] ) {
            icb-> transfer[ n ] += icb-> base;
            if( icb-> name[0] ) {
                if( n )
                    sprintf( sname, "SHARE$%s_TRANSFER_%d", icb-> name, n );
                else
                    sprintf( sname, "SHARE$%s_INITIALIZE", icb-> name );
            }
            else
                strcpy( sname, "MAIN");
            set_symbol_direct( sname, icb-> transfer[ n ], SYM_ENTRY );
        }
    }


//  Get the image ident stuff

    addr = base + ihd.offset_ident;
    rc = map( "IHI", ( void * ) &ihi, addr );
    if( rc ) {
        printf( "Error mapping IHI, %08X\n", rc );
        goto abort;
    }
    
    ihi.imgnam[ (int) ihi.imgnaml ] = 0;
    ihi.imgid [ (int) ihi.imgidl  ] = 0;
    ihi.linkid[ (int) ihi.linkidl ] = 0;
    
    if( debug ) {
        printf( "\tImage Identification:\n\t\t%s %s\n\t\tLinker: %s\n", 
            ihi.imgnam, ihi.imgid, ihi.linkid );
            
        printf( "\n\tTransfer array:\n\t\t%08X\n\t\t%08X\n\t\t%08X\n",
            icb-> transfer[ 0 ], icb-> transfer[ 1 ], icb-> transfer[ 2 ]);
    }
    
    
//  Walk the ISD list.

    addr = base + ihd.offset_ident + IHI_SIZE;

    for( n = 0; n < 1000; n++ ) {
    
        rc = map( "ISD", ( void * ) &static_isd, addr );
        if( rc ) {
            printf( "Error mapping ISD, %08X\n", rc );
            goto abort;
        }
            
        if( static_isd.size == 0 )
            break;
        addr = addr + static_isd.size;
        
        isd = ( struct ISD * ) getmem( sizeof( struct ISD ));
        *isd = static_isd;
        isd-> next = icb-> isd_list;
        icb-> isd_list = isd;
        if( isd-> valid == CHAR4('U','S','E','D')) {
            printf( "Internal error; re-used ICB structure!\n" );
        }
        isd-> valid = CHAR4('U','S','E','D');
        
    }
    
    icb-> fixup_isd = 0L;
    icb-> isd_count = 0;
    
    for( isd = icb-> isd_list; isd; isd = isd-> next ) {
        icb-> isd_count++;
        if( isd-> count < 1 || isd-> count > 39 ) {
            strcpy( isd-> name, "<NONE>" );
            isd-> count = 6;
        }
        else
            isd-> name[ (int) isd-> count ] = 0;
    
        if( debug ) {
            addr = ( isd-> vpn << 9 );
            printf( "\n\tImage Section Descriptor %d\n", icb-> isd_count );
            printf( "\t\t%d page%s mapped at %08X (cluster=%d)\n",
                     ( LONGWORD ) isd-> pages, 
                     isd-> pages == 1 ? "" : "s",
                     icb-> base + addr, (LONGWORD) isd-> pfc );

            /* As long as it's not a global section or dzero, where is it? */

            if( !( isd-> flags & ( ISD_M_GBL | ISD_M_DZRO )))
                printf( "\t\tMapped to file page %08X (byte %08lX)\n", 
                     isd-> vbn, ( long ) isd-> vbn<<9 );

            printf( "\t\tFlags: %08X  ", ( LONGWORD ) isd-> flags );
            
            if( isd-> flags & ISD_M_GBL )
                printf( " GBL" );
            
            if( isd-> flags & ISD_M_CRF )
                printf( "  CRF" );
            
            if( isd-> flags & ISD_M_DZRO )
                printf( " DZRO" );
            
            if( isd-> flags & ISD_M_WRT )
                printf( " WRT" );
            
            if( isd-> flags & ISD_M_LASTCLU )
                printf( " LASTCLU" );

            if( isd-> flags & ISD_M_COPYALWAY )
                printf( " COPYALWAY" );
            
            if( isd-> flags & ISD_M_BASED )
                printf( " BASED" );
            
            if( isd-> flags & ISD_M_FIXUPVEC )
                printf( " FIXUPVEC" );
            
            if( isd-> flags & ISD_M_PROTECT )
                printf( " PROTECT" );
            
            
            printf( "\n\t\t" );
            
            n = ISD_B_TYPE( isd-> flags );
            switch( n ) {
                case 0:     printf( "Normal section" );     break;
                case 1:     printf( "Fixed shared" );       break;
                case 2:     printf( "Fixed private" );      break;
                case 3:     printf( "Shared" );             break;
                case 4:     printf( "Private" );            break;
                case 253:
                case -3:    printf( "User stack" );         break;
                default:    printf( "Unknown image type %d", n );       
            }
        
            if( n > 0 && n < 253 ) {
                printf( " image section" );
		if( isd-> name[ 0 ] != 0 &&
                    isd-> name[ 0 ] != '<' )
			printf( ", name = %s", isd-> name );
            }
            
            printf( "\n" );

        }

//  If this section contains the fixup data for the image (there will
//  be only one of these) then make a note of it.
        
        if( isd-> flags & ISD_M_FIXUPVEC )
            icb-> fixup_isd = isd;

        
//  Load this section if appropriate.


        n = ISD_B_TYPE( isd-> flags );      

        if( n == -3 )			/* No user stack loads */
            continue;
        
        if( isd-> flags & ISD_M_GBL )   /* No global sections */
            continue;

//	If it's DZRO we must account for the memory and zero it out while
//	we're at it.  We can translate to a physical address (which gets
//	a VM check, and maps if it was unmapped memory, and the just zero
//	the page by writing to physical memory.

        if( isd-> flags & ISD_M_DZRO ) {
        
            for( n = 0; n < isd-> pages; n++ ) {
                addr = icb-> base + ((n+ isd->vpn)<<9);
                
                /* Advance the highwater mark as needed */
                if( addr >= vax.console.image_load ) {
                    vax.console.image_load = addr + 512;
                    set_region_size( 0, vax.console.image_load );
                }
                
                if( addr + 0x1FF > icb->end )
                    icb-> end = addr + 0x1FF;
                
                /* Find the physical address in memory */
                
                paddr = addr;
                rc = vm( ( ULONGWORD * ) &paddr, &bsize, OP_MD );
                if( rc ) {
                    printf( "VM translation error creating DZRO image section into memory at VA %08X\n",
                        paddr );
                    goto abort;
                }
                
                /* Write zeroes over the page */
                
                for( zp = 0; zp < 512; zp++ )
                    vax.memory[ paddr+zp ] = 0;
            }

            continue;
        }

//  Let's load it up.  Find the memory and file address...


        /* Files are zero-based, but the Virtual Block Number is */
        /* one-based so offset it before mapping page to byte.   */

        file_addr = ( isd-> vbn - 1 ) * 512;

        rc = fseek( fp, file_addr, SEEK_SET );
        if( rc ) {
            printf( "Unabled to seek to byte %08X\n", file_addr );
        }
        
        section_blocks= isd-> pages;
        if( debug ) {
            addr = icb-> base + ( isd-> vpn << 9 );
            printf( "\t\tReading %d page%s into %08X - %08X\n", 
		section_blocks, section_blocks == 1 ? "" : "s",
                addr, addr + (section_blocks << 9 ) - 1 );
        }

        for( n = 0; n < section_blocks; n++ ) {
        
            addr = icb-> base + (( n + isd-> vpn ) << 9 );
            
            /* Advance the "high water mark" for the image.  We  */
            /* want "image_load" to point to the next free page. */

            if( addr >= vax.console.image_load ) {
                vax.console.image_load = addr + 512;
                set_region_size( 0, vax.console.image_load );
            }
            
            if( addr + 0x1FF > icb->end )
                icb-> end = addr + 0x1FF;
                
            paddr = addr;
            rc = vm( ( ULONGWORD * ) &paddr, &bsize, OP_MD );
            if( rc ) {
                printf( "VM translation error reading image section into memory at VA %08X, section block %d\n",
                    paddr, n );
                goto abort;
            }

            size = fread( &( vax.memory[ paddr ]), 1, 512, fp );
            if( size <= 0 ) {
                printf( "Unable to read block %d into memory at %08X (PA %08X)\n", 
                           n, addr, paddr );
                goto abort;
            }
        }

    }
    
//  Close the file, and mark that the ICB represents a completely loaded
//  image.

    fclose( fp );
    icb-> flags &= ~ICB_INCOMPLETE;

//  Okay, this image is likely to have bound sharable libraries on it, so
//  if so, let's go ahead and recursively call ourselves to load those if
//  possible.

    
    if( icb-> fixup_isd ) {
    
        /* Use the VPN of the ISD offset by the image base addr */

        addr = icb-> base + ( icb-> fixup_isd-> vpn << 9 );

        rc = map( "IAF", &( icb-> iaf ), addr );
        if( rc ) {
            printf( "Error mapping IAF, %08X\n", rc );
            goto abort;
        }
        
        // Let's make the list of sharable images first before we 
        // start recursively calling ourselves.
    
        naddr = addr + icb-> iaf.offset_shl;
        naddr = naddr + 16;  /* Hack, there are four longs I don't understand */
        rc = load_memory( naddr, ( void * ) &( icb->iaf.offset_names ), 4 );
        if( rc ) {
            printf( "Error reading IAF.OFFSET_SHL, %08X\n", rc );
            goto abort;
        }

        naddr = naddr + icb-> iaf.offset_names;
        
        /* Scoop up each name from the SHR structures */
        
        if( debug ) {
            printf( "\n\tSHARABLE IMAGE LIST:\n" );
            printf( "\t\t(%d) %s\n", 0, "this image" );
        }

        /*  Create the one for the image itself */
            shr = ( struct SHR * ) getmem( sizeof( struct SHR ));
            shr-> next = icb-> shr_list;
            icb-> shr_list = shr;
            shr-> id = 0;
            shr-> icb = icb;	/* Self reference */
            shr-> base = icb-> base;
            strcpy( shr-> name, icb-> name );

        /*  And one for each image on the list */

        for( n = 1; n < icb-> iaf.shrimgcnt; n++ ) {
        
            paddr = naddr + 0x08;  /* Two longs that seem to be always zero, maybe queue slots? */
            naddr = naddr + 0x40;  /* Stride over each one */
            
            for( j = 0; j < 40; j++ ) {
                rc = load_byte( paddr + j, ( void * ) &shrname[ j ] );
            }
            
            shrname[ shrname[ 0 ] + 1 ] = 0;
            
            shr = ( struct SHR * ) getmem( sizeof( struct SHR ));
            shr-> next = icb-> shr_list;
            icb-> shr_list = shr;
            imageid++;
            shr-> id = imageid;
            shr-> icb = 0L;
            shr-> base = 0L;
            strcpy( shr-> name, &(shrname[ 1 ]));
            if( debug ) 
                printf( "\t\t(%d) %s\n", shr-> id, shr-> name );
        }

        /* Now that we have the list, try to recursively load each one */

        for( shr = icb-> shr_list; shr; shr = shr-> next ) {

            if( shr-> id == 0 )
                continue; /* Don't load ourselves! */

            rc = image_load( shr-> name, ICB_SECONDARY, &shr-> icb );
            if( rc == VAX_FNF )
                rc = VAX_OK;
            
            if( rc )
                break;

            if( shr-> icb )
                shr->base = shr-> icb-> base;
        }
    }
    
    
    
    return rc;
    
abort:
    fclose( fp );
    printf( "Error reading image header\n" );
    set_mode_stack( saved_mode );
    return rc;

}



/**************************************************************************\
 *                                                                        *
 *                           FIX UP AN IMAGE                              *
 *                                                                        *
\**************************************************************************/

/*
 *
 *  Here is the general strategy of the image fixup phase.
 *
 *
 *  1.  This is called for each ICB in the icb_list chain.  These
 *      are created in reverse order, so the first fixup is to the
 *      last image loaded.
 *
 *  2.  If the image has already been fixed up (not likely) or had
 *      no fixup records itself (also not usually the case) then
 *      we do no work.
 *
 *  3.  Otherwise use the fixup_isd pointer in the ICB to locate the
 *      section containing the fixup data.  Re-map an IAF structure
 *      to the fixed header of this memory.
 *
 *  4.  Traverse the list of G^ and .ADDRESS fixups stored in the
 *      list.  For each list, there is an ID of the sharable image
 *      that it must fix up against.  This is used to find the base
 *      address of the sharable images that were previously recursively
 *      loaded.  This is used to store a fixup value in memory.  If
 *      there is no  image to bind to, then SHIM$ symbols are used to
 *      point to microkernel replacements for specific entry points.
 *
 */
 
LONGWORD image_fixup( struct ICB * icb )
{
    LONGWORD debug;
    char sb[ 256 ], *sbp;
    struct ICB * ip;    
    LONGWORD rc;
    LONGWORD addr, paddr, naddr, vaddr;
    LONGWORD n, value, disp;
    LONGWORD fixup_count;
    struct IAF iaf;
    LONGWORD imageid = 0;
    
    struct SHR * shr;
            
    if( !vax_init ) {
        return VAX_NOVAX;
    }
    debug = vax.debug & DBG_IMAGES;
    

    if( icb == 0L ) {
        printf( "No main image to fix up.\n" );
        return VAX_OK;
    }
    
    if( !icb-> fixup_isd ) {
        if( debug )
            printf( "No fixups for image %s\n", icb-> name );
        return VAX_OK;
    }

    /* If we've already done this one then no more work here */
    if( icb-> flags & ICB_FIXED )
        return VAX_OK;
    
    addr = icb-> base + ( icb-> fixup_isd-> vpn << 9 );

    rc = map( "IAF", &iaf, addr );
    if( rc ) {
        printf( "Error mapping IAF, %08X\n", rc );
        goto abort;
    }
    
    /*  Run the G^ list and bind each one to a sharable image */
    
    if( iaf.offset_g_fix ) {
    
        naddr = addr + iaf.offset_g_fix;
        
        /* Get the number of entries in the next list */
        rc = load_memory( naddr, ( void * ) &fixup_count, 4 );
        if( rc ) {
            printf( "Error reading G^ count, %08X\n", rc );
            goto abort;
        }

        if( debug )
            printf( "There %s %d G^ fixup record%s for image %s\n",
                fixup_count == 1 ? "is" : "are",
                fixup_count,
                fixup_count == 1 ? "" : "s",
                icb-> name );
                
        while( fixup_count ) {
            
            
            /* Get which image we're doing fixup's for */
            
            naddr += 4;
            rc = load_memory( naddr, ( void * ) &imageid, 4 );
            if( rc ) {
                printf( "Error reading G^ image id, %08X\n", rc );
                goto abort;
            }

            for( shr = icb-> shr_list; shr; shr = shr-> next )
                if( imageid == shr-> id )
                    break;

            if( shr == 0L ) {
                if( debug )
                    printf( "\nImage %s needs G^ fixups to image id %d that didn't load\n", 
                        icb-> name, imageid );
                fixup_count = 0;
            }
            else
            if( debug )
                printf( "\nThere %s %d G^ fixup%s for image %d, %s\n", 
                    fixup_count == 1 ? "is" : "are",
                    fixup_count,
                    fixup_count == 1 ? "" : "s", imageid, shr-> name );
            
            /* Now loop over all the fixups */
            
            for( n = 0; n < fixup_count; n++ ) {

                naddr += 4;
                rc = load_memory( naddr, ( void * ) &paddr, 4 );
                if( rc ) {
                    printf( "Error reading fixup address, %08X\n", rc );
                    goto abort;
                }
                                
                if( debug )
                    printf( "\tAt address %08X, fixup to offset %08X\n", 
                        naddr, paddr );

                /* First, see if it's on an ICB list we already have */

                for( ip = icb_list; ip; ip = ip-> next ) {
                    if( strcmp( ip-> name, shr-> name ) == 0 ) {
                        value = ip-> base + paddr;
                        if( debug )
                            printf( "\t\t%s based at %08X, resolves to %08X\n",
                                shr-> name, ip-> base, value );
                        break;
                    }
                }

                /* If not an ICB, look for a shim */

                if( !ip ) {
                    sprintf( sb, "SHIM$%s_%08X", shr->name, paddr );
                    sbp = sb;
                
                    disp = K_ADDR_L;
                    vax.console.deposit = naddr;
                    rc = asm_value( &sbp, ( ULONGWORD * )  &value, disp );
                    if( rc ) {
                        printf( "Error referencing SHIM symbol, %08X\n", rc );
                        goto abort;
                    }
                }
                    
                rc = store_memory( naddr, ( void * ) &value, 4 );
                if( rc ) {
                    printf( "Error storing SHIM fixup, %08X\n", rc );
                    goto abort;
                }
            }
            
            /* Get length of next fixup list and loop if non-zero */
            
            naddr += 4;
            rc = load_memory( naddr, ( void * ) &fixup_count, 4 );
            if( rc ) {
                printf( "Error reading next G^ fixup count, %08X\n", rc );
                goto abort;
            }
        }
    }   /* if there are G^ fixups */
        
    /*  Run the .ADDRESS list and bind each one to a sharable image */
    
    if( iaf.offset_addr ) {
    
        naddr = addr + iaf.offset_addr;
        
        /* Get the number of entries in the next list */
        rc = load_memory( naddr, ( void * ) &fixup_count, 4 );
        if( rc ) {
            printf( "Error reading .ADDRESS fixup count, %08X\n", rc );
            goto abort;
        }

        if( debug )
            printf( "There %s %d .ADDRESS fixup record%s for image %s\n",
                fixup_count == 1 ? "is" : "are",
                fixup_count,
                fixup_count == 1 ? "" : "s",
                icb-> name );
                
        while( fixup_count ) {
            
            
            /* Get which image we're doing fixup's for */
            
            naddr += 4;
            rc = load_memory( naddr, ( void * ) &imageid, 4 );
            if( rc ) {
                printf( "Error reading .ADDRESS image id, %08X\n", rc );
                goto abort;
            }

            for( shr = icb-> shr_list; shr; shr = shr-> next )
                if( imageid == shr-> id )
                    break;
            
            if( shr == 0L ) {
                if( debug )
                    printf( "\nImage %s needs .ADDRESS fixups to image id %d that didn't load\n", 
                        icb-> name, imageid );
                fixup_count = 0;
            }
            else
            if( debug )
                printf( "\nThere %s %d .ADDRESS fixup%s for image %d, %s\n", 
                    fixup_count == 1 ? "is" : "are",
                    fixup_count,
                    fixup_count == 1 ? "" : "s", imageid, shr-> name );
            
            /* Now loop over all the fixups */
    
        
            for( n = 0; n < fixup_count; n++ ) {

                /* Get the offset in memory from the image base */
                naddr += 4;
                rc = load_memory( naddr, ( void * ) &paddr, 4 );
                if( rc ) {
                    printf( "Error loading .ADDRESS fixup address, %08X\n", rc);
                    goto abort;
                }
                
                vaddr = icb-> base + paddr;
                rc = load_memory( vaddr, ( void * ) &paddr, 4 );
                if( rc ) {
                    printf( "Error loading .ADDRESS fixup dest, %08X\n", rc );
                    goto abort;
                }

                paddr = paddr + shr-> base;
                        
                if( debug )
                    printf( "\tAt address %08X, fixup to offset %08X\n", 
                        vaddr, paddr );
                
                rc = store_memory( vaddr, ( void * ) &paddr, 4 );
                if( rc ) {
                    printf( "Error writing .ADDRESS fixup, %08X\n", rc );
                    goto abort;
                }
            }
            
            /* Get length of next fixup list and loop if non-zero */
            
            naddr += 4;
            rc = load_memory( naddr, ( void * ) &fixup_count, 4 );
            if( rc ) {
                printf( "Error reading next .ADDRESS fixup count, %08X\n", rc );
                goto abort;
            }
        }
        
    }   /* If there are .ADDRESS fixups */
    /* Success case falls through here to exit point */

    icb-> flags |= ICB_FIXED;
    
abort:

    return rc;

}



void init_ihd_maps( void )
{

    struct IHD * ihd;
    struct ISD * isd;
    struct IHI * ihi;
    struct IAF * iaf;
    
    if( find_map( "IHD" ))
        return;
    
    ihd = 0L;
    isd = 0L;
    ihi = 0L;
    iaf = 0L;
    
    map_init( "IAF" );
    
    map_add( "IAF", "OFFSET_G_FIX",
                0x0C, STROFF( iaf, offset_g_fix ),
                4, 0 );

    map_add( "IAF", "OFFSET_SHL",
                0x18, STROFF( iaf, offset_shl ),
                4, 0 );

    map_add( "IAF", "OFFSET_ADDR",
                0x10, STROFF( iaf, offset_addr ),
                4, 0 );

    map_add( "IAF", "OFFSET_CHGPROT",
                0x14, STROFF( iaf, offset_chgprot ),
                4, 0 );

    map_add( "IAF", "SHRIMGCNT",
                0x1C, STROFF( iaf, shrimgcnt ),
                4, 0 );


    map_init( "IHI" );
    
    map_add( "IHI", "IMGNAML",
                0, STROFF( ihi, imgnaml ),
                1, 0 );
    
    map_add( "IHI", "IMGNAM",
                1, STROFF( ihi, imgnam ),
                39, 1 );
    
    map_add( "IHI", "IMGIDL",
                40, STROFF( ihi, imgidl ),
                1, 0 );
    
    map_add( "IHI", "IMGID",
                41, STROFF( ihi, imgid ),
                15, 1 );
    
    map_add( "IHI", "TIME_0",
                56, STROFF( ihi, time[ 0 ] ),
                4, 0 );
    
    map_add( "IHI", "TIME_1",
                60, STROFF( ihi, time[ 1 ] ),
                4, 0 );

    map_add( "IHI", "LINKIDL",
                64, STROFF( ihi, linkidl ),
                1, 0 );
    
    map_add( "IHI", "LINKID",
                65, STROFF( ihi, linkid ),
                15, 1 );
    
    
    map_init( "IHD" );
    
    map_add( "IHD", "SIZE", 
                0, STROFF( ihd, size ), 
                2, 0 );
                
    map_add( "IHD", "OFFSET_TRANSFER", 
                2, STROFF( ihd, offset_transfer ), 
                2, 0 );
                
    map_add( "IHD", "OFFSET_DST", 
                4, STROFF( ihd, offset_dst ), 
                2, 0 );
                
    map_add( "IHD", "OFFSET_IDENT", 
                6, STROFF( ihd, offset_ident ), 
                2, 0 );
    
    map_add( "IHD", "OFFSET_PATCH", 
                8, STROFF( ihd, offset_patch ), 
                2, 0 );
    
    map_add( "IHD", "MAJOR_ID", 
                12, STROFF( ihd, major_id ), 
                2, 0 );
    
    map_add( "IHD", "MINOR_ID", 
                14, STROFF( ihd, minor_id ), 
                2, 0 );
    
    map_add( "IHD", "HEADER_BLOCKS", 
                16, STROFF( ihd, header_blocks ), 
                1, 0 );

    map_add( "IHD", "IMAGE_TYPE", 
                17, STROFF( ihd,image_type ), 
                1, 0 );
    
    map_add( "IHD", "MASK_0", 
                20, STROFF( ihd, mask[0] ), 
                4, 0 );
    
    map_add( "IHD", "MASK_1", 
                24, STROFF( ihd, mask[1] ), 
                4, 0 );

    map_add( "IHD", "CHANNELS", 
                28, STROFF( ihd, channels ), 
                2, 0 );
    
    map_add( "IHD", "IO_PAGES", 
                30, STROFF( ihd, io_pages ), 
                2, 0 );
    
    map_add( "IHD", "FLAGS", 
                32, STROFF( ihd, flags ), 
                4, 0 );

    map_add( "IHD", "SECTION_ID", 
                36, STROFF( ihd, section_id ), 
                4, 0 );
    
    map_add( "IHD", "VERSION", 
                40, STROFF( ihd, version ), 
                4, 0 );
    
    map_init( "ISD" );
    
    map_add( "ISD", "SIZE", 
                0, STROFF( isd, size ), 
                2, 0 );

    map_add( "ISD", "PAGES", 
                2, STROFF( isd, pages ), 
                2, 0 );

    map_add( "ISD", "VPN", 
                4, STROFF( isd, vpn ), 
                2, 0 );

    map_add( "ISD", "PFC", 
                6, STROFF( isd, pfc ), 
                2, 0 );

    map_add( "ISD", "FLAGS", 
                8, STROFF( isd, flags ), 
                4, 0 );

    map_add( "ISD", "VBN", 
                12, STROFF( isd, vbn ), 
                4, 0 );

    map_add( "ISD", "SECTION_ID", 
                16, STROFF( isd, section_id ), 
                4, 0 );

    map_add( "ISD", "COUNT", 
                20, STROFF( isd, count ), 
                1, 0 );

    map_add( "ISD", "NAME", 
                21, STROFF( isd, name ), 
                15, 1 );


    return; 
}

/*
 *   FIND_IMAGE
 *
 *   This routine is given a file name and must locate it on the
 *   native file system.  This is done by making sure that it has
 *   a .EXE suffix, and then looking in the path as-passed.  If not
 *   found then a prefix is added for looking for sharable images.
 */

char * find_image( char * fp )
{
    static char fn[ 256 ];
    int n;
    char * cp, *fnp;
    char * suffix;

    /* See if it has a .EXE already */

#if defined( VMS ) || defined( WIN )
    suffix = ".EXE";
#else
    suffix = ".exe";
#endif

    n = (int) strlen( fp );
    if( n > 4 ) {
        cp = &( fp[ n - 4 ] );
        if( strcmp( cp, ".exe" ) == 0 ||
            strcmp( cp, ".EXE" ) == 0 )
            suffix = 0L;
    }

    strcpy( fn, fp );
    if( suffix )
        strcat( fn, suffix );

	fnp = image_file_found( fn );
    if( fnp )
        return fnp;

    fn[ 0 ] = 0;
    if( vax.console.share_prefix[0] ) {
        if( vax.console.share_prefix[ 0 ] != 0x1B )
            strcpy( fn, vax.console.share_prefix );
    }
    strcat( fn, fp );
    if( suffix )
        strcat( fn, suffix );

	fnp = image_file_found( fn );
    if( fnp )
        return fnp;

    for( n = 0; n < 255; n++ ) {
        if( fn[ n ] == 0 )
            break;
        if( fn[ n ] >= 'A' && fn[ n ] <= 'Z' )
            fn[ n ] += 32;
    }

	fnp = image_file_found( fn );
    if( fnp )
        return fnp;
    return 0L;
}

static char * image_file_found( char * fn )
{
    static char fnl[ 256 ];
    char ch;
    int n;

    FILE * f;

    f = xfopen( fn, "rb" );
    if( f ) {
        fclose( f );
        return fn;
    }

    for( n = 0; n < 256; n++ ) {
        ch = fn[ n ];
        if( ch >= 'A' && ch <= 'Z' )
            ch = ch+32;
        fnl[ n ] = ch;
        if( ch == 0 )
            break;
    }

    f = xfopen( fnl, "rb" );
    if( f ) {
        fclose( f );
        return fnl;
    }

    return 0L;
}
    

/*
 *  Given a region number (0-P0, 1=P1, 2=S0), get the current region
 *  size value.
 */
    
long get_region_size( int region, LONGWORD * size )
{
    long rc;

    if( region_size_addr[0] == 0 )
        init_region_addr();

    rc = load_memory( region_size_addr[ region ], ( void * ) size, 4 );
    return rc;
}


/*
 *  Given a region number (0-P0, 1=P1, 2=S0), set the current region
 *  size value.
 */
    
long set_region_size( int region, LONGWORD size )
{
    long rc;
    int saved_mode;
    
    if( region_size_addr[0] == 0 )
        init_region_addr();

    saved_mode = (int) vax.pslw.cur_mod;
    vax.pslw.cur_mod = 0;
    rc = store_memory( region_size_addr[ region ], ( void * ) &size, 4 );
    vax.pslw.cur_mod = saved_mode;
    
    return rc;
}


static long init_region_addr(void)
{

    long rc;

    rc = get_symbol_direct( "EXE$P0_RGN", &(region_size_addr[ 0 ]));
    if( rc )
        return rc;

    rc = get_symbol_direct( "EXE$P1_RGN", &(region_size_addr[ 1 ]));
    if( rc )
        return rc;

    rc = get_symbol_direct( "EXE$S0_RGN", &(region_size_addr[ 2 ]));
    return rc;

}

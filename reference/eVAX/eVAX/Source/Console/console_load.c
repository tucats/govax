
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_load.c
//
//  Purpose:    This module implements the LOAD command, which reads a
//              memory image into the current system.
//
//  History:    08/06/97    New header format standardization
//
//              06/09/99    Disallow if not in kernel mode.
//
//              06/11/99    Add BOOT command
//
//              07/13/99    Add RUN command
//
//              07/16/99    Moved RUN function to console_run.c
//
//              09/23/99    Added LOAD/ROM command.  This loads a console ROM into separate
//                          address space from physical memory.  Supports /BASE and /SIZE
//                          qualifiers to define memory range for text ROM files, or can
//                          read a binary ROM file created by SAVE/ROM.  Also support /NOERROR
//                          qualifier that ignores if file is not found.
//
//              09/26/99    Added NVRAM to match ROM capabilities.  Set console symbols
//                          CONSOLE$ROM_BASE, etc. to describe where ROM (or NVRAM) was
//                          loaded.
//              01/10/00    Fixed small bug in LOAD command.  However, it seems that the
//                          LOAD command is pretty antique given all the changes in the
//                          last year.  Put on the "to-do" list creating a more useful
//                          LOAD and SAVE pair that captures the entire context of the
//                          emulation.  We can also probably dump the SAVE/TEXT which
//                          is not very useful.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "memmap.h"
#include "imgdef.h"

#include <time.h>
void init_ihd_maps( void );
LONGWORD load_rom( FILE * fp );
LONGWORD load_nvram( char ** P );
int swap_bytes( void * addr, int size );

    
//  LOAD <filename>

LONGWORD console_load( char ** P )
{
    struct VAX fvax;
    
    char * p, *fn;
    struct SYMBOL * sym;
    LONGWORD size, n, rc, reading, verb;
    LONGWORD addr, pagecount;
    char buff[ 512 ];
    FILE * fp;
    int valid, oldmkv;
    
    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        return VAX_NOVAX;
    }

//  Maybe LOAD/IMAGE, which we convert to RUN/NOEXEC

    flush_blanks( &p );
    rc = read_verb( &p, &verb );

    if( rc == VAX_OK && verb == CHAR4('/','I','M','A')) {
        strcpy( buff, "/NOEXEC " );
        strcat( buff, p );
        while( !isend( *p )) p++;
        
        *P = p;
        p = buff;
        rc = console_run( &p );
        if( rc == VAX_OK ) {
            strcpy( buff, "IMAGE$MAIN" );
            p = buff;
            if( VAX_OK == get_symbol( &p, &addr)) {
                if( vax.console.flags & CONSOLE_VERBOSE )
                    printf( "Image main program located at %08X\n", addr );
            }
            strcpy( buff, "IMAGE$INIT" );
            p = buff;
            if( VAX_OK == get_symbol( &p, &addr)) {
                if( vax.console.flags & CONSOLE_VERBOSE )
                    printf( "Image initialization entry point IMAGE$INIT is %08X\n", addr );
                vax.PC = addr;
            }
        }
        return rc;
        
    }
    

//  You can't do other LOAD forms if you aren't in kernel mode!  This is to 
//  prevent accidental screw ups.

    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

//  See if this is LOAD/ROM.

    if( rc == VAX_OK && verb == CHAR4('/','R','O','M')) {
        *P = p;
        return console_rom( P );
    }

//  Okay, how about LOAD/NVRAM
    if( rc == VAX_OK && verb == CHAR4('/','N','V','R')) {
        *P = p;
        return load_nvram( P );
    }

    p = *P;
    
//  If there are unresolved forward references, disallow the
//  load operation.  The load could interfere with forward
//  references inappropriately.

    for( sym = vax.console.symbols; sym; sym = sym-> next ) {
        if( sym-> forward != 0L ) 
            return VAX_ASMFORWARD;
    }
    
    
//  Do the work.  Start by fetching the filename.
    

    flush_blanks( &p );
    if( isend( *p ))
        fn = "saved.vax";
    else {
        fn = p;
        for( fn = p; *p; p++ )
            if( *p == '\n' )
                *p = 0;
    }
    
    fp = xfopen( fn, "rb" );
    if( fp == 0L ) {
        printf( "Can't open file \"%s\"\n", fn );
        return VAX_OK;
    }

//  Read the first eight bytes.  They must match a "magic number".

    size = fread( buff, 1, 8, fp );
    valid = 0;
    if( strncmp( buff, ";//EOF*\n", 8 ) == 0 )
        valid = 1;
    if( strncmp( buff, ";//EOF*\r", 8 ) == 0 )
        valid = 1;
        
    if( size != 8 || !valid ) {
        printf( "File \"%s\" is not a saved memory image.\n", fn );
        return VAX_OK;
    }

//  Read the version number.  Currently we only accept a version of 2.1

    size = fread( buff, 1, 2, fp );
    if( size != 2 )
        goto eof_exit;

    if( buff[ 0 ] != 2  || buff[ 1 ] != 1 ) {
        printf( "Unsupported file version %d.%d\n", buff[ 0 ], buff[ 1 ] );
        return VAX_OK;
    }

//  Read the struct VAX layout size the file was written with, and refuse
//  the file if it doesn't match this build's layout -- a saved.vax file
//  written by a build with a different `struct VAX` (e.g. before AUDIT.md's
//  LONGWORD fix, or on a different platform) is not safe to read field-by-
//  field, and would otherwise desync every field that follows.

    {
        int32_t structsize = 0;
        size = fread( &structsize, 4, 1, fp );
        if( size != 1 )
            goto eof_exit;
        if( structsize != (int32_t) sizeof( struct VAX )) {
            printf( "File \"%s\" was saved by a build with a different VAX layout "
                "(%d bytes vs. %d expected); refusing to load.\n",
                fn, structsize, (int) sizeof( struct VAX ));
            fclose( fp );
            return VAX_OK;
        }
    }

//  Read the copy of the VAX header at the time.

    size = fread( &fvax, sizeof( struct VAX ), 1, fp );
    if( size != 1 )
        goto eof_exit;

//  Blow away the vax we have, and make a new one.  This free's up memory,
//  cleans up the processor state, etc.

    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "Reading in a virtual VAX with %08X (hex) bytes of memory.\n",
        fvax.memsize );
        
    free_vax( &vax );
    rc = alloc_vax( fvax.memsize );
    /* *Vax = vax; -- use global vax instead -- */
    if( rc )
        return rc;

    vax.console.vminit_valid = 0;
    if( MKVALID )
        printf( "%s\n", vaxmsg( VAX_DELMK ));
        
    vax.console.microkernel_valid = 0;
    

//  Copy the VM data

    for( n = 0; n < 3; n++ )
        vax.region[ n ] = fvax.region[ n ];
    vax.vm_initialized =  fvax.vm_initialized;

//  Copy the console data.  Empty out the pointers and
//  such that we don't want.

    sym = system_symbols;

    vax.console = fvax.console;
    vax.console.scale = 0L;
    vax.console.breakpoint_list = 0L;
    vax.console.prompt = 0L;
    vax.console.CALL_active = 0;

    vax.console.last_symbol = 0L;
    vax.console.symbols = 0L;

    system_symbols = sym;

//  Copy the assembler flag data

    vax.assembler.flags = fvax.assembler.flags;

//  Copy the register data.

    for( n = 0; n < MAXREG; n++ )
        vax.reg[ n ] = fvax.reg[ n ];
    
    for( n = 0; n < MAXPRIVREG; n++ )
        vax.preg[ n ] = fvax.preg[ n ];
    
    vax.psl = fvax.psl;


//  Start reading page data

    reading = 1;
    while( reading ) {
    
        size = fread( &addr, 4, 1, fp );
        if( size != 1 )
            goto eof_exit;

        size = fread( &pagecount, 4, 1, fp );
        if( size != 1 )
            goto eof_exit;

        if( pagecount == 0L ) {
            reading = 0;
            break;
        }

        if( (ULONGWORD) ( addr + pagecount * 512 - 1 ) > ( ULONGWORD ) vax.memsize ) {
            printf( "Saved memory image too large; error loading page at %08X\n", addr );
            reading = 0;
            break;
        }
        
        size = fread( &( vax.memory[ addr ]), 1, 512 * pagecount, fp );
        if( size != 512 )
            goto eof_exit;
        
        // printf( "Read %ld bytes at %08lx\n", pagecount * 512L, addr );
        
    
    }
        
    fclose( fp );

    vax.console.vminit_valid = fvax.console.vminit_valid;

    oldmkv = MKVALID;
    vax.console.microkernel_valid = fvax.console.microkernel_valid;

    if( oldmkv & !MKVALID )
        printf( "%s\n", vaxmsg( VAX_DELMK ));

    return VAX_OK;

eof_exit:

    fclose( fp );
    printf( "Unexpected end-of-file reading \"%s\"\n", fn );
    return VAX_OK;
    
}

/*
 *  LOAD/ROM <filename>
 */

LONGWORD console_rom( char ** P )
{
    extern char * rom;
    extern char * rom_name;
    
    char * p, *fn;
    FILE * fp;
    char quote;
    int n, saved_mapen, parsing, base_set, size_set, silent;
    char *b, buff[ 80 ];
    ULONGWORD base, addr, size, data, paddr;
    LONGWORD rc, verb, line;
    
    p = *P;
    if( !vax_init )
        return VAX_NOVAX;
    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

    flush_blanks( &p );
    parsing = ( *p == '/' );
    
/*
 *  Look for qualifiers. 
 *
 *      /BASE=base-address
 *      /SIZE=size (in Kilobytes)
 *      /IGNORE or /SILENT      
 */
 
    base_set = size_set = 0;
    
    base = 0x20040000;                  /* Default base address */
    size = 256;                         /* Default is 256K ROM size */
    silent = 0;                         /* Display errors.  */
    
    while( parsing ) {
    
        flush_blanks( &p );
        if( isend( *p )) {
            parsing = 0;
            continue;
        };
        
        if( *p != '/' ) {
            parsing = 0;
            continue;
        }
        
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        
        if( verb == CHAR4( '/','S','I','L' ) || verb == CHAR4( '/','N','O','E')) {
            silent = 1;
            continue;
        }

        flush_blanks( &p );
        if( *p != '=' ) {
            return VAX_SYNTAX;
        }
        p++;
        
        rc = asm_expr( &p, ( LONGWORD * ) &data );
        if( rc )
            return rc;
        
        if( verb == CHAR4( '/','B','A','S' )) {
            base = data;
            base_set = 1;
        }
        else
        if( verb == CHAR4( '/','S','I','Z' )) {
            size = data;
            size_set = 1;
        }
        else {
            return VAX_SYNTAX;
        }
    }
    
    
    if( *p == '"' ) {
        p++;
        quote = '"';
    }
    else
        quote = 0;
        
    if( isend( *p ))
        fn = "default.rom";
    else {
        fn = p;
        for( fn = p; *p; p++ )
            if( *p == '\n' || *p == quote )
                *p = 0;
    }
    
    *P = p;
    
    fp = xfopen( fn, "rb" );
    if( fp == 0L ) {
    
        if( silent )    
            return VAX_OK;
            
        printf( "Can't open ROM file \"%s\"\n", fn );
        return VAX_BADROM;
    }

    /* See if this is a binary ROM image */
    b = fgets( buff, 8, fp );
    buff[ 8 ] = 0;
    if( strcmp( b, ";ROMIMG" ) == 0 ) {
    
        if( base_set || size_set ) {
            printf( "/BASE or /SIZE illegal with binary ROM loads\n" );
            return VAX_BADROM;
        }
        strcpy( rom_name, fn );
        
        return load_rom( fp );
    }
    else {
    
        /* Nah, back it up and read like "normal" */
        fseek( fp, 0, 0 );
    }
    
        
    
    if( base < vax.memsize ) {
        printf( "ROM address space overlaps physical memory (last physical address is %08X\n",
                    vax.memsize );
        return VAX_BADROM;
    }
    if( size > 2048 ) {
        printf( "ROM size too large (max 2MB)\n" );
        return VAX_BADROM;
    }
    
    /* Destroy the old ROM if any */
    
    if( rom )
        freemem( rom );
    
    /* Set up for the new ROM */
    
    size = size * 1024;  /* Convert kilobytes to bytes */
    rom = getmem( size );
    if( rom == 0L )
        return VAX_MEM;
    
    /* Zero the ROM memory */
    for( paddr = 0; paddr < size; paddr++ )
        rom[ paddr ] = 0;
        
    vax.rom_base = base;
    vax.rom_end = base + size - 1;
    strcpy( rom_name, fn );

    set_symbol_direct( "CONSOLE$ROM_BASE", vax.rom_base, 0 );
    set_symbol_direct( "CONSOLE$ROM_END", vax.rom_end, 0 );
    set_symbol_direct( "CONSOLE$ROM_SIZE", size, 0 );
    
    /*  Set up to read in the data */
    
    b = buff;
    base = ( ULONGWORD ) -1L;
    paddr = 0;
    saved_mapen = (int) vax.MAPEN;
    vax.MAPEN = 0;
    rc = VAX_OK;
    line = 0;
    while( b ) {
    
        b = fgets( buff, 80, fp );
        if( b == 0L )
            break;
        line++;
        while( *b == ' ' )
            b++;
        
        if( b[ 0 ] == '\n' || b[ 0 ] == '\r' )
            continue;
            
        /* Ignore blank lines and lines with a comment */
        
        if( strlen( b ) == 0 )
            continue;
            
        if( b[ 0 ] == ';' )
            continue;
            
        /* Command has a very specific format.  P aaaaaaaa dddddddd  */
        /* where P is literal, aaaaaaaa is the physical address, and */
        /* dddddddd is the data in hex.                              */
        
        n = sscanf( buff, "  P %X %X",  &addr, &data );
        if( n != 2 ) {
            printf( "Error reading input at line %d\n  \"%s\"\n", line, buff );
            rc = VAX_BADROM;
            break;
        }

        if( addr <  vax.rom_base || addr > vax.rom_end ) {
            printf( "Error storing ROM data at %08X, outside ROM address space (%08X-%08X)\n",
                        addr, vax.rom_base, vax.rom_end );
            rc = VAX_BADROM;
            break;
        }
        
        rc = store_memory( addr, ( void * ) &data, 4 );
        if( rc ) {
            printf( "Error storing ROM data at %08X\n", addr );
            break;
        }
        
        if( addr > paddr )
            paddr = addr;
        if( addr < base )
            base = addr;
            
        
    }
    vax.MAPEN = saved_mapen;
    
    fclose( fp );
    
    if( rc == VAX_OK ) {
        if( paddr > 0 && ( vax.console.flags & CONSOLE_VERBOSE ))
            printf( "Loaded data from %08X to %08X\n", base, paddr );
    }
    else {
        freemem( rom );
        strcpy( rom_name, "<no ROM loaded>" );
        vax.rom_base = 0xFFFFFFFFUL;
        vax.rom_end = 0xFFFFFFFFUL;
        rom = 0L;
        set_symbol_direct( "CONSOLE$ROM_BASE", vax.rom_base, 0 );
        set_symbol_direct( "CONSOLE$ROM_END", vax.rom_end, 0 );
        set_symbol_direct( "CONSOLE$ROM_SIZE", 0, 0 );
    }
    
    return rc;
}


/*
 *  BOOT <filename>
 */

 
LONGWORD console_boot( char ** P )
{
    
    char * p, *fn;

    LONGWORD size, n, rc, reading;
    LONGWORD addr, base;
    ULONGWORD flags;    /* Boot flags */
    unsigned char buff[ 512 ];
    FILE *  fp;
    
    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        return VAX_NOVAX;
    }


//  You can't do this if you aren't in kernel mode!  This is to prevent accidental
//  screw ups.

    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

//  Zero out everything.

    rc = console_zero( P );
    if( rc )
        return rc;

    flush_blanks( &p );
    if( *p == '/' ) {
        p++;
        rc = asm_value( &p, &flags, 0 );
        if( rc )
            return rc;
    }
    else
        flags = 0;


//  Do the work.  Start by fetching the filename.
    

    flush_blanks( &p );
    if( isend( *p ))
        fn = "vmb.boot";
    else {
        fn = p;
        for( fn = p; *p; p++ )
            if( *p == '\n' )
                *p = 0;
    }
    
    *P = p;

    fp = xfopen( fn, "rb" );
    if( fp == 0L ) {
        printf( "Can't open file \"%s\"\n", fn );
        return VAX_OK;
    }

//  Read in blocks of data.

    addr = base = 0x00001000L;

    reading = 1;
    rc = VAX_OK;
    while( reading ) {

        for( n = 0; n < 512; n++ )
            buff[ n ] = 0;

        size = fread( &(buff[ 0 ]), sizeof( unsigned char ), 512, fp );
    
        if( size <= 0 ) {
            reading = 0;
            for( n = 0; n< 512; n++ )
                if( !buff[ n ] )
                    break;

#if 0
            printf( "n = %ld size = %ld ferror = %ld, feof = %ld\n", 
                n, size, ( LONGWORD ) ferror( fp ), ( LONGWORD ) feof( fp ));
#endif

            continue;
        }

        for( n = 0; n < size; n++ ) {
            vax.memory[ addr++ ] = buff[ n ];
        }
        if( rc )
            break;
    }
        
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "Bootstrap code %s loaded from address %08X to %08X\n", 
            fn, base, addr - 1 );

    fclose( fp );

    vax.R0 = 17;          /* UDA-50 boot device */
    vax.R1 = 0;           /* TR number of adapter */
    vax.R2 = 0x65001000;  /* Controller "A", unibus adapter ID 1000 */
    vax.R3 = 0;           /* Boot device unit number */
    vax.R4 = 0;           /* Boot LBN (0 for VMS) */
    vax.R5 = flags;       /* Boot flags */
    vax.R10 = 0;          /* Halt PC */
    vax.R11 = 0;          /* Halt PSL */
    vax.AP = 0;           /* Halt code */
    vax.PC = base;        /* Base of VMB */
    vax.SP = base;        /* Base of VMB */

    /* Turn off some stuff that the booter wouldn't expect or want */

    vax.ICCS = 0;             /* No interval timer */
    vax.clock_running = 0;    /* ditto */
    vax.NICR = 0;             /* No next timer value */
    vax.clock = 0;            /* No clock value */
    vax.ICR = 0;              /* Ditto */
    vax.TXCS = 0x00000080;    /* Console ready to xmit */
    vax.SCBB = addr;          /* SCB defaults to after the VMB loader */

    vax.IPL = 31;             /* Booter expects no interrupts at start */
    vax.pslw.ipl = 31;
    
    /* Note that the 512 byte block before the SP and VMB load point is the */
    /* Restart Parameter Block (RPB).  This is zeroed in this case.  Need   */
    /* find out later if this must be filled in some way.                   */

    /* We've loaded the booter, let 'er rip */
    
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "Executing bootstrap code starting at %08X\n", vax.PC );
    
    execute_vax( (short) vax.console.disasm );
    
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "\n" );

    return rc;

}


LONGWORD load_rom( FILE * fp )
{
    extern char * rom;
    char junk;
    LONGWORD size, rc, reading;
    ULONGWORD addr;
    ULONGWORD pagecount;
    ULONGWORD rom_base, rom_end, rom_size;
    
//  Free the current ROM memory

    if( rom )
        freemem( rom );
    rom = 0L;
    vax.rom_base = 0xFFFFFFFFUL;
    vax.rom_end = 0xFFFFFFFFUL;

//  Read the base and size

    size = fread( &junk, 1, 1, fp );
    
    rc = VAX_BADROM;
    size = fread( &rom_base, 4, 1, fp );
    if( size != 1 ) {
        printf( "Unable to read ROM base address from saved image\n" );
        goto err_exit;
    }

    size = fread( &rom_end, 4, 1, fp );
    if( size != 1 ) {
        printf( "Unable to read ROM end address from saved image\n" );
        goto err_exit;
    }
        
#if !BIGENDIAN
    swap_bytes( &rom_base, sizeof( rom_base ));
    swap_bytes( &rom_end, sizeof( rom_end ));
#endif

    rom_size = ( rom_end + 1 ) - rom_base;
    
    rom = getmem( rom_size );
    if( rom == 0L ) {
        rc = VAX_MEM;
        goto err_exit;
    }
    
    vax.rom_base = rom_base;
    vax.rom_end = rom_end;
    
//  Start reading page data

    reading = 1;
    while( reading ) {
    
        size = fread( &addr, 4, 1, fp );
        if( size != 1 )
            goto eof_exit;

        size = fread( &pagecount, 4, 1, fp );
        if( size != 1 )
            goto eof_exit;

#if !BIGENDIAN
        swap_bytes( &addr, sizeof( addr ));
        swap_bytes( &pagecount, sizeof( addr ));
#endif
        
        if( pagecount == 0L ) {
            reading = 0;
            break;
        }
        
        if( addr + pagecount * 512 - 1 >rom_size ) {
            printf( "Saved ROM memory image too large; error loading page at %08X\n", addr );
            reading = 0;
            break;
        }
        
        size = fread( &(rom[ addr ]), 1, 512 * pagecount, fp );
        if( size != 512 )
            goto eof_exit;
        
        // printf( "Read %ld bytes at %08lx\n", pagecount * 512L, addr );
        
    
    }

eof_exit:
    set_symbol_direct( "CONSOLE$ROM_BASE", vax.rom_base, 0 );
    set_symbol_direct( "CONSOLE$ROM_END", vax.rom_end, 0 );
    set_symbol_direct( "CONSOLE$ROM_SIZE", size, 0 );

    fclose( fp );
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "Read ROM image at %08X - %08X\n", 
            vax.rom_base, vax.rom_end );
    
    return VAX_OK;

err_exit:
    printf( "Error reading binary ROM image\n" );
    fclose( fp );
    if( rom )
        freemem( rom );
    vax.rom_base = 0xFFFFFFFFUL;
    vax.rom_end = 0xFFFFFFFFUL;
    rom = 0L;
    set_symbol_direct( "CONSOLE$ROM_BASE", vax.rom_base, 0 );
    set_symbol_direct( "CONSOLE$ROM_END", vax.rom_end, 0 );
    set_symbol_direct( "CONSOLE$ROM_SIZE", 0, 0 );
    return rc;
    
}

int swap_bytes( void * addr, int size )
{
     char * d_addr, tmp;
     int n, dlen;

     dlen = size/2;

     d_addr = ( char * ) addr;
     for( n = 0; n < dlen; n++ ) {
         tmp = d_addr[ n ];
         d_addr[ n ] = d_addr[ (size-n)-1 ];
         d_addr[ (size-n)-1 ] = tmp;
     }
     return 0;
}
         

/*
 *  LOAD/NVRAM <filename>
 */

LONGWORD load_nvram( char ** P )
{
    extern char * nvram, * nvram_name;
    
    char * p, *fn;
    FILE * fp;
    char quote;
    int silent;
    ULONGWORD count, base, size;
    LONGWORD rc, verb;
    
    p = *P;
    if( !vax_init )
        return VAX_NOVAX;
    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

    
/*
 *  Look for qualifiers. 
 *      /IGNORE or /SILENT      
 */
 
    silent = 0;                         /* Display errors.  */
    
    flush_blanks( &p );
    rc = read_verb( &p, &verb );
    if( verb == CHAR4( '/','S','I','L' ) || verb == CHAR4( '/','N','O','E'))
        silent = 1;
    else
        p = *P;
        
    
    
    if( *p == '"' ) {
        p++;
        quote = '"';
    }
    else
        quote = 0;
        
    if( isend( *p ))
        fn = "default.nvram";
    else {
        fn = p;
        for( fn = p; *p; p++ )
            if( *p == '\n' || *p == quote )
                *p = 0;
    }
    
    *P = p;
    
    fp = xfopen( fn, "rb" );
    if( fp == 0L ) {
    
        if( silent )    
            return VAX_OK;
            
        printf( "Can't open NVRAM file \"%s\"\n", fn );
        return VAX_BADROM;
    }

    if( nvram )
        freemem( nvram );
    vax.nvram_base = 0xFFFFFFFFUL;
    vax.nvram_end = 0xFFFFFFFFUL;
    set_symbol_direct( "CONSOLE$NVRAM_BASE", -1L, 0 );
    set_symbol_direct( "CONSOLE$NVRAM_END", -1L, 0 );
    set_symbol_direct( "CONSOLE$NVRAM_SIZE", 0, 0 );
    
    /* Read the base address and size */

    count = fread( &base, 4, 1, fp );
    if( count != 1 ) {
        fclose(fp );
        return VAX_BADNVR;
    }

    count = fread( &size, 4, 1, fp );
    if( count != 1 ) {
        fclose( fp );
        return VAX_BADNVR;
    }

#if !BIGENDIAN
    swap_bytes( &base, sizeof(base));
    swap_bytes( &size, sizeof(size));
#endif
    
    nvram = getmem( size );
    if( nvram == 0L ) {
        printf( "Unable to allocate %uK NVRAM area\n", size/1024 );
        fclose( fp );
        return VAX_MEM;
    }
    
    /*  Read the data into the NVRAM area */
    
    count = fread( nvram, 1, size, fp );
    if( count != size ) {
        fclose( fp );
        return VAX_BADNVR;
    }
    
    vax.nvram_base = base;
    vax.nvram_end = base + size - 1;
    strcpy( nvram_name, fn );
    
    set_symbol_direct( "CONSOLE$NVRAM_BASE", base, 0 );
    set_symbol_direct( "CONSOLE$NVRAM_END", vax.nvram_end, 0 );
    set_symbol_direct( "CONSOLE$NVRAM_SIZE", size, 0 );
    
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "NVRAM loaded from %08X to %08X\n", 
             vax.nvram_base, vax.nvram_end );
    fclose( fp );
    return VAX_OK;
    
    
    return rc;
}

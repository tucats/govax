//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, 
//                see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     logical_names.c
//
//  Purpose:    Runtime support for VMS logical names.
//
//  History:    07/05/01    Created.


#include "vax.h"
#include "shim.h"
#include "services.h"
#include "ss_def.h"
#include "dclrtl.h"
#include "logicals.h"

struct LNM * tables = 0L;

/*
 *	SHOW LOGICAL <name> console command
 */

long show_logical( long id )
{
    struct LNM * lnm_t, * lnm;
    char * tabname, * logname;
    char * q, * v;
    int count;
    
    count = 0;
    if( DCLpresent( DCL_CALLBACK_QUALIFIER, 300 ))
        tabname = DCLgetstring( DCL_CALLBACK_QUALIFIER, 300 );
    else
        tabname = 0L;

    if( DCLpresent( DCL_CALLBACK_PARAMETER, 301 ))
        logname = DCLgetstring( DCL_CALLBACK_PARAMETER, 301 );
    else
        logname = 0L;

    for( lnm_t = tables; lnm_t; lnm_t = lnm_t-> next ) {
    
        /* If a logical name table was given, test for a match */
        if( tabname && ( strcmp( lnm_t-> name, tabname ) != 0 ))
            continue;
        
        for( lnm = lnm_t-> tables; lnm; lnm = lnm-> next ) {
            if( logname && ( strcmp( lnm-> name, logname ) != 0 ))
                continue;
            
            if( lnm-> value ) {
                q = "\"";
                v = lnm-> value;
            }
            else {
                q = "";
                v = "<undefined>";
            }
            
            printf( "%s [%s] = %s%s%s\n", lnm-> name, lnm_t-> name, q, v, q );
            count++;
        }
    }
    
    if( !count ) 
        printf( "No matching logical names.\n" );
        
    return VAX_OK;
}

            

    
/*
 *	DEFINE/LOGICAL console command
 *
 *	Syntax:
 *
 *		DEFINE/LOGICAL <logical_name> [/TABLE=<table_name>] "<string_value>"
 $
 */
 
long define_logical( long id )
{
    
    char * table;
    char * name;
    char * value;
    long sts;
    
    if( DCLpresent( DCL_CALLBACK_QUALIFIER, 200 ))
        table = DCLgetstring( DCL_CALLBACK_QUALIFIER, 200 );
    else
        table = "LNM_PROCESS";
        
    name =  DCLgetstring( DCL_CALLBACK_PARAMETER, 201 );
    value = DCLgetstring( DCL_CALLBACK_PARAMETER, 202 );
    
    
    sts = set_logical( table, name, value, 0L );
    if( sts == SS_NORMAL )
        sts = VAX_OK;
    return sts;
}

LONGWORD init_logicals(void)
{

    set_logical( "LNM$TABLE",    "LNM$FILE_DEV", "<TABLE>", LNM_M_TABLE );
    
    set_logical( "LNM$FILE_DEV", "SYS$COMMAND", "TTA0:", 0L );
    set_logical( "LNM$FILE_DEV", "SYS$INPUT",   "TTA0:", 0L );
    set_logical( "LNM$FILE_DEV", "SYS$OUTPUT",  "TTA0:", 0L );
    set_logical( "LNM$FILE_DEV", "SYS$ERROR",   "TTA0:", 0L );
    
    return VAX_OK;
}

struct LNM * get_logical( char * Tabnam, char * lognam, long attr )
{
    struct LNM * lnm_t, * lnm;
    char * tabnam;
    
/*
 *  Find the table
 */
 
    if( Tabnam == 0L || *Tabnam == 0 )
        tabnam = "LNM$ROOT";
    else
        tabnam = Tabnam;

    
    for( lnm_t = tables; lnm_t; lnm_t = lnm_t-> next ) {
    
        if( strcmp( tabnam, lnm_t-> name ) == 0 )
            break;
    }
    
    if(  !lnm_t && !( attr & LNM_M_TERMINAL ))
        lnm_t = get_logical( "LNM$TABLE", tabnam, LNM_M_TERMINAL );

    if( !lnm_t ) {
        if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: get_logical(%s,%s), table not found.\n", 
                tabnam, lognam );
        return 0L;
    }

/*
 *  Find the logical in the table
 */
 
    for( lnm = lnm_t-> tables; lnm; lnm = lnm-> next )
        if( strcmp( lognam, lnm-> name ) == 0 ) {
            if( attr )
                if( lnm-> attr != attr )
                    continue;
            break;
        }
            
    if( !lnm ) {
        if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: get_logical(%s,%s), logical name not found.\n", 
                tabnam, lognam );
        return 0L;
    }

    if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: get_logical(%s,%s), value=\"%s\"\n", 
                tabnam, lognam,
                lnm-> value ? lnm-> value : "<undefined>");
    return lnm;
}



LONGWORD set_logical( char * table, char * name, char * value, long attr ) 
{
    struct LNM * lnm_t, * lnm;
    
    if( vax.debug & DBG_LOGICALS )
        printf( "DEBUG: DEFINE/LOGICAL %s/TABLE=%s \"%s\"\n", 
            name, table, value );
            
    for( lnm_t = tables; lnm_t; lnm_t = lnm_t-> next ) 
        if( strcmp( lnm_t-> name, table ) == 0 )
            break;
    
    if( !lnm_t ) {
        lnm_t = ( struct LNM * ) getmem( sizeof( struct LNM ));
        lnm_t-> next = tables;
        tables = lnm_t;
        lnm_t-> tables = 0L;
        lnm_t-> value = 0L;
        strcpy( lnm_t-> name, table );
        lnm_t-> attr = LNM_M_TABLE;
        lnm_t-> accmode= 0;
    }
    
    for( lnm = lnm_t-> tables; lnm; lnm = lnm-> next )
        if( strcmp( lnm-> name, name ) == 0 )
            break;

    if( !lnm ) {
        lnm = ( struct LNM * ) getmem( sizeof( struct LNM ) + strlen( value ));
        lnm-> next = lnm_t -> tables;
        lnm_t-> tables = lnm;
        lnm-> tables = 0L;
        lnm-> value = 0L;
        strcpy( lnm-> name, name );
        lnm-> attr = attr;
        lnm-> accmode= 0;
    }
    
    if( lnm-> value ) {
        freemem( lnm-> value );
    }
    
    lnm-> value = getmem( strlen( value ) + 1 );
    strcpy( lnm-> value, value );
    return SS_NORMAL;
    
}

/*
 *	SYS$TRNLNM System Service
 *
 *	Given a table and logical name, and an item list of things to do with it,
 *	translate the name and return result(s).
 */


SSDEF( sys_trnlnm )
{
    LONGWORD attr;
    char lognam[ 64 ];
    char tabnam[ 64 ];
    int len, accmode;
    LONGWORD rc, ptr;
    struct LNM * lnm, * lnm_t;
    short bufflen, itemcode;
    LONGWORD buffaddr, retaddr;
    LONGWORD size;
    short retsize;
    
    if( argc < 5 )
        return SS_INSFARG;
    if( argc > 5 )
        return SS_TOO_MANY_ARGS;
        
    /* Get address of attribute list, if given */
    
    if( argv[ 0 ] )
        rc = load_memory( argv[ 0 ], ( void * ) &attr, 4 );
    else
        attr = 0;
        
    if( argv[ 1 ] ) {
        len = 63;
        lognam[ 0 ] = 0;
        rc = str_get( argv[ 1 ], &len, tabnam );
        if( len > 0 )
            tabnam[ len ] = 0;
        else
            return SS_IVLOGTAB;
    }
    else
        return SS_IVLOGTAB;
        
    len = 63;
    lognam[ 0 ] = 0;
    rc = str_get( argv[ 2 ], &len, lognam );
    if( len > 0 )
        lognam[ len ] = 0;
    else
        return SS_IVLOGNAM;
    
    /* If we are supposed to be case blind, then upcase the name */
    
    if( attr & LNM_M_CASE_BLIND )
        uppercase( lognam );
        
    if( argv[ 3 ] )
        rc = load_memory( argv[ 3 ], (void*) &accmode, 1 );
    else
        accmode = (unsigned int) vax.pslw.cur_mod;
    
/*
 *  Find the table
 */
 
    for( lnm_t = tables; lnm_t; lnm_t = lnm_t-> next ) {
    
        if( strcmp( tabnam, lnm_t-> name ) == 0 )
            break;
    }
    if( !lnm_t ) {
        if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: $TRNLNM(%s,%s), table not found.\n", 
                tabnam, lognam );
        return SS_NOLOGTAB;
    }

/*
 *  Find the logical in the table
 */
 
    for( lnm = lnm_t-> tables; lnm; lnm = lnm-> next )
        if( strcmp( lognam, lnm-> name ) == 0 )
            break;
            
    if( !lnm ) {
        if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: $TRNLNM(%s,%s), logical name not found.\n", 
                tabnam, lognam );
        return SS_NOLOGNAM;
    }

    if( vax.debug & DBG_LOGICALS )
            printf( "DEBUG: $TRNLNM(%s,%s), mode=%d value=\"%s\"\n", 
                tabnam, lognam, accmode,
                lnm-> value ? lnm-> value : "<undefined>");


/* If they didn't give an arg list, return success */
    
    if( argv[ 4 ] == 0L )
        return SS_NORMAL;

/*  Scan the item list looking for things we know how to set. */

    ptr = argv[ 4 ];

    while( 1 ) {
    
        /* Get the next item list entry elements */
        
        rc = load_memory( ptr, ( void * ) &bufflen, 2 );
        if( rc )
            return SS_ACCVIO;
        rc = load_memory( ptr+2, ( void * ) &itemcode, 2 );
        if( rc )
            return SS_ACCVIO;
        
        /* If this is the end of the list then break out */
        if( bufflen == 0 && itemcode == 0 )
            break;
        
        rc = load_memory( ptr+4, ( void * ) &buffaddr, 4 );
        if( rc )
            return SS_ACCVIO;
        
        rc = load_memory( ptr+8, ( void * ) &retaddr, 4 );
        if( rc )
            return SS_ACCVIO;
        
        /* Now, based on item code, do the right thing */
     
        switch( itemcode ) {
        
        case LNM__ACMODE:
        
            if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__ACMODE = %d\n", accmode );
                
            rc = store_memory( buffaddr, ( void * ) &accmode, 1 );
            if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 1;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
       
        case LNM__ATTRIBUTES:
           if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__ATTRIBUTES = %08X\n", lnm-> attr );
                
            rc = store_memory( buffaddr, ( void * ) &lnm-> attr, 4 );
            if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 4;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
           
        case LNM__LENGTH:
             if( lnm-> value )
                size = strlen( lnm-> value );
            else
                size = 0;

           if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__LENGTH = %d\n", size );
            retsize = size;    
            rc = store_memory( buffaddr, ( void * ) &retsize, 2 );
            if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 4;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;

        case LNM__MAX_INDEX:

        /* Currently we don't support lists, so max index is always 1 */
        
           retsize = size = 1;

           if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__MAX_INDEX = %d\n", size );

            rc = store_memory( buffaddr, ( void * ) &retsize, 2 );
            if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 4;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;

        case LNM__STRING:

           if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__STRING = \"%s\"\n", 
                    lnm-> value ? lnm-> value : "<undefined>" );
                    
            if( lnm-> value ) {
                size = strlen( lnm-> value );
                rc = store_string( lnm-> value, buffaddr,(int) size );
                if( rc )
                    return SS_ACCVIO;
            }
            else
                size = 0;
                        
            if( retaddr ) {
                retsize = size;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
 
        case LNM__TABLE:

           if( vax.debug & DBG_LOGICALS ) 
                printf( "DEBUG: $TRNLNM item list LNM__TABLE = %s\n", 
                    lnm_t-> name );

    
            size = strlen( lnm_t-> name );
            rc = str_put( buffaddr, (int) size, lnm_t-> name );
            if( rc )
                return SS_ACCVIO;
                    
            if( retaddr ) {
                retsize = size;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
  
        }
    
        /* Advance to next item in itmlst */
        
        ptr = ptr + 12;
    }
    
    return SS_NORMAL;
}



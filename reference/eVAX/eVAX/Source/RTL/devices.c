//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     devices.c
//
//  Purpose:    Runtime support for VMS system services for handling devices.
//
//  History:    11/08/99    Created.


#include "vax.h"
#include "shim.h"
#include "services.h"
#include "ss_def.h"
#include "dclrtl.h"

#define DEV_CLASS_NONE  0
#define DEV_CLASS_TT	66
#define DEV_CLASS_DISK	1

#define DVI_M_SECONDARY  0x00000001
#define DVI_M_NOREDIRECT 0x00008000
#define DVI_M_CODEBITS   0x00007FFE

#define DVI__DEVCLASS      4
#define DVI__DEVTYPE       6
#define DVI__DEVBUFSIZE    8

#define DVI__ACPPID       64

static struct DEV_CLASS_MAP {
    long	class;
    char *	name;
} dev_class_map[] = {
{ DEV_CLASS_TT,	"terminal" },
{ DEV_CLASS_DISK, "disk" },
{ DEV_CLASS_NONE, "none" }};

struct DEVICE {
    struct DEVICE * 	next;
    char		name[ 64 ];
    LONGWORD		acppid;
    LONGWORD		cluster;
    LONGWORD		cylinders;
    LONGWORD		devbufsiz;
    LONGWORD		devchar;
    LONGWORD		devchar2;
    char		devclass;
    LONGWORD		devdepend;
    LONGWORD		devdepend2;
    LONGWORD		devsts;
    char		devtype;
    LONGWORD		errcnt;
    LONGWORD		freeblocks;
    LONGWORD		lockid;
    char		volname[ 64 ];
    LONGWORD		maxblock;
    LONGWORD		maxfiles;
    char		medianame[64 ];
    char		mediatype[64];
    LONGWORD		mountcount;
    LONGWORD		opcnt;
    LONGWORD		ownuic;
    LONGWORD		pid;
    LONGWORD		recsize;
    LONGWORD		refcnt;
    char		rootdevname[ 64 ];
    LONGWORD		sectors;
    LONGWORD		serial;
    LONGWORD		sts;
};


struct CHAN {
    struct CHAN *	next;
    char		name[ 64 ];
    char		mbx[ 255 ];
    char		physname[ 255 ];
    short		chan;
    short		class;
    LONGWORD		flags;
    struct DEVICE *	device;
};

struct DEVICE * find_device( char * name );
char * get_dev_class_name( long class );

int next_channel = 0x0100;

struct CHAN * channels = 0L;
struct DEVICE * devices = 0L;

char * get_dev_class_name( long class )
{
    int n;
    
    for( n = 0; n < 100; n++ )  {
        if( dev_class_map[ n ].class == class )
            return dev_class_map[ n ].name;
    }
    return "<unknown>";
}


struct DEVICE * find_device( char * name )
{
    struct DEVICE * dp;
    int n;
    char dname[ 64 ];
    
    strcpy( dname, name );
    uppercase( dname );
    
    n = (int) strlen( dname );
    if( dname[ n-1 ] == ':' )
        dname[ n-1 ] = 0;
    
    for( dp = devices; dp; dp = dp-> next ) {
        if( strcmp( dname, dp-> name ) == 0 )
            return dp;
    }
    return 0L;
}


long show_device( long verb )
{
    char * name;
    int full;
    struct DEVICE * dp;
    
    full = (int) DCLpresent( DCL_CALLBACK_QUALIFIER, 101 );
    if( DCLpresent( DCL_CALLBACK_PARAMETER, 100 ))
        name = DCLgetstring( DCL_CALLBACK_PARAMETER, 100 );
    else
        name = 0L;

    for( dp = devices; dp; dp = dp-> next ) {
    
        if( name && ( strcmp( name, dp-> name ) != 0  ))
            continue;
        
        printf( "Device %s\n", dp-> name );
        if( !full )
            continue;
        
        printf( "    DEVCLASS=%d (%s)   DEVTYPE=%d\n",  
            dp-> devclass,
            get_dev_class_name( dp-> devclass ),
             dp-> devtype );
        
        if( dp-> devclass == 1 ) {
            printf( "    APCPID=%08X\n",   dp-> acppid );
            printf( "    CLUSTER=%d\n",    dp-> cluster );
            printf( "    CYLINDERS=%d\n",  dp-> cylinders );
            printf( "    FREEBLOCKS=%d\n", dp-> freeblocks );
            printf( "    MAXBLOCK=%d\n",   dp-> maxblock );
            printf( "    MAXFILEs=%d\n",   dp-> maxfiles );
            printf( "    SECTORS=%d\n",    dp-> sectors );
            printf( "    SERIAL=%d\n",     dp-> serial );
            printf( "    VOLNAME=%s\n",     dp-> volname );
            printf( "    MEDIANAME=%s\n",   dp-> medianame );
            printf( "    MEDIATYPE=%s\n",   dp-> mediatype );
            printf( "    ROOTDEVNAME=%s\n", dp-> rootdevname );
        }
        
        printf( "    DEVBUFSIZE=%d\n", dp-> devbufsiz );
        printf( "    RECSIZE=%d\n",    dp-> recsize );

        printf( "    DEVCHAR=%08X    DEVCHAR2=%08X\n",  
                dp-> devchar, dp-> devchar2 );
        printf( "    DEVDEPEND=%08X  DEVDEPEND2=%08X\n", 
                dp-> devdepend, dp-> devdepend2 );
        
        printf( "    PID=%08X        OWNUIC=%08X\n", 
                dp-> pid, dp-> ownuic );
        printf( "    LOCKID=%08X\n",   dp-> lockid );
        printf( "    REFCNT=%d\n",     dp-> refcnt );
        
        
    }
    
    return VAX_OK;
}



long define_device( long verb )
{
    long rc;
    struct DEVICE * dp;
    
    dp = ( struct DEVICE * ) getmem( sizeof( struct DEVICE ));
    if( dp == 0L )
        return VAX_MEM;
    rc = 0;
    
/*
 *	Fill in stuff we know without being told.
 */
 
    dp-> pid = 0;
    dp-> devsts = 0;
    dp-> errcnt = 0;
    dp-> acppid = 0;
    dp-> refcnt = 0;
    dp-> mountcount = 0;
    dp-> opcnt = 0;
    dp-> pid = 0;
    dp-> sts = 0;
    

/*
 *	Get stuff from the command as needed.
 */
 
    strcpy( dp-> name, DCLgetstring( DCL_CALLBACK_PARAMETER, 400 ));

#define GET_INT_FIELD( ptr, field, num ) \
    if( DCLpresent( DCL_CALLBACK_QUALIFIER, num )) \
        ptr -> field = DCLgetinteger( DCL_CALLBACK_QUALIFIER, num ); \
    else \
        ptr -> field = 0

#define GET_STR_FIELD( ptr, field, num ) \
    if( DCLpresent( DCL_CALLBACK_QUALIFIER, num )) \
        strcpy( ptr -> field, DCLgetstring( DCL_CALLBACK_QUALIFIER, num )); \
    else \
        ptr -> field[ 0 ] = 0

    GET_INT_FIELD( dp, cluster, 401 );
    GET_INT_FIELD( dp, cylinders, 402 );
    GET_INT_FIELD( dp, devbufsiz, 403 );
    GET_INT_FIELD( dp, devchar, 404 );
    GET_INT_FIELD( dp, devchar2, 405 );
    GET_INT_FIELD( dp, devclass, 406 );
    GET_INT_FIELD( dp, devdepend, 407 );
    GET_INT_FIELD( dp, devdepend2, 408 );
    GET_INT_FIELD( dp, devtype, 409 );
    GET_INT_FIELD( dp, freeblocks, 410 );
    GET_INT_FIELD( dp, lockid, 411 );
    GET_INT_FIELD( dp, maxblock, 412 );
    GET_INT_FIELD( dp, maxfiles, 413 );
    GET_INT_FIELD( dp, ownuic, 414 );
    GET_INT_FIELD( dp, recsize, 415 );
    GET_INT_FIELD( dp, sectors, 416 );
    GET_INT_FIELD( dp, serial, 417 );
    
    GET_STR_FIELD( dp, volname, 420 );
    GET_STR_FIELD( dp, medianame, 421 );
    GET_STR_FIELD( dp, mediatype, 422 );
    GET_STR_FIELD( dp, rootdevname, 423 );

    if( rc )
        freemem(( void *  ) dp );
    else {
        dp-> next = devices;
        devices = dp;
    }
    
    return VAX_OK;
    
}

/*
 *	SYS$GETDVIW system service
 */

SSDEF( sys_getdviw )
{
    char devname[ 64 ];
    int size;
    int	efn;
    int chan;
    int noredirect, secondary;
    
    LONGWORD ptr, buffaddr, retaddr;
    short retsize;
    short itemcode, bufflen;
    long rc;
    
    
    struct CHAN * cp;
    struct DEVICE * dp;
    
    if( argc != 8 )
        return SS_INSFARG;
    
    efn = (int) argv[ 0 ];
    chan = (int) argv[ 1 ];
    
    if( argv[ 2 ] ) {
        size = 63;
        str_get( argv[ 2 ], &size, devname );
        devname[ size ] = 0;
    }
    else
        size = 0;
        
    if( chan ) {
        for( cp = channels; cp; cp = cp-> next ) {
            if( cp-> chan == chan ) 
                break;
        }
        if( !cp )
            return SS_IVCHAN;
        dp = cp-> device;
    }
    else
    if( size ) {
        dp = find_device( devname );
        if( !dp )
            return SS_NOSUCHDEV;
    }
    else
        return SS_IVDEVNAM;
        
/*
 *	At this point, the dp pointer points to the device in question.
 */

    if( vax.debug & DBG_DEVICES )
        printf( "DEBUG: SYS$GETDVIW looks up device %s\n", dp-> name );
    
/*  Scan the item list looking for things we know how to process. */

    ptr = argv[ 3 ];

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
     
        noredirect = itemcode & DVI_M_NOREDIRECT;
        secondary  = itemcode & DVI_M_SECONDARY;
        
        /* itemcode &= DVI_M_CODEBITS; */
        
        switch( itemcode ) {
       
         case DVI__DEVCLASS:
        
            if( vax.debug & DBG_DEVICES ) 
                printf( "DEBUG: SYS$GETDVIW returns DEVCLASS=%d\n", dp-> devclass );
                
            rc = store_memory( buffaddr, ( void * ) &dp-> devclass, 1 );
            if( rc )
                return SS_ACCVIO;
            if( retaddr ) {
                retsize = 1;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
       
       case DVI__DEVTYPE:
        
           if( vax.debug & DBG_DEVICES ) 
                printf( "DEBUG: SYS$GETDVIW returns DEVTYPE=%d\n", dp-> devtype );
                
           rc = store_memory( buffaddr, ( void * ) &dp-> devtype, 1 );
            if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 1;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
       
        case DVI__DEVBUFSIZE:
            if( vax.debug & DBG_DEVICES ) 
                printf( "DEBUG: SYS$GETDVIW returns DEVBUFSIZ=%d\n", dp-> devbufsiz );
                
            rc = store_memory( buffaddr, ( void * ) &dp-> devbufsiz, 4 );
             if( rc )
                return SS_ACCVIO;
            
            if( retaddr ) {
                retsize = 4;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
           
        default:
            if( vax.debug & DBG_DEVICES ) 
                printf( "DEBUG: SYS$GETDVIW for %s found unrecognized item code %d\n",
                        dp-> name, itemcode );
            return SS_BADPARAM;
        }
    
        /* Advance to next item in itmlst */
        
        ptr = ptr + 12;
    }
    
    return SS_NORMAL;
    
}

/*
 *	SYS$ASSIGN system service
 *
 *	Given a device name, assign a channel to the device.
 */
 
SSDEF( sys_assign ) 
{
    char name[ 64 ];
    int size;
    long rc;
    struct CHAN * cp;
    struct DEVICE * dp;
    
/*  Do some basic argument checking */

    if( argc < 2 )
        return SS_INSFARG;
    if( argc > 5 )
        return SS_TOO_MANY_ARGS;
    
    if(( argv[ 0 ] == 0L ) || ( argv[ 1 ] == 0L ))
        return SS_INSFARG;

/* Get the device name, passed as a string descriptor */
        
    size = 64;
    rc = str_get( argv[ 0 ], &size, name );
    if( rc )
        return SS_ACCVIO;
    name[ size ] = 0;
    dp = find_device( name );
    if( dp == 0L )
        return SS_IVDEVNAM;
        
/*  Create a channel */

    cp = ( struct CHAN * ) getmem( sizeof( struct CHAN ));
    cp-> next = channels;
    channels = cp;
    next_channel += 8;
    
    strcpy( cp-> name, name );
    cp-> chan = next_channel;

    cp-> device = dp;
    
    rc = store_memory( argv[ 1 ], ( void * ) &( cp-> chan ), 2 );
    if( rc )
        return SS_ACCVIO;
    
    if(( argc > 3 ) && ( argv[ 3 ] != 0L )) {
        size = 255;
        rc = str_get( argv[ 3 ], &size, cp-> mbx );
        if( rc  )
            return SS_ACCVIO;
    }
    
    if( argc > 4 ) 
        cp-> flags = argv[ 4 ];

    cp-> class = cp-> device-> devclass;
    cp-> device-> refcnt++;

    cp-> device-> pid = vax.console.pid;
    cp-> device-> ownuic = vax.console.uic;
    
    return SS_NORMAL;
    
}


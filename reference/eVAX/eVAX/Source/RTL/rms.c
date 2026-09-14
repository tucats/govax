//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     rms.c
//
//  Purpose:    Runtime support for RMS system services.
//
//  History:    12/05/2001    Created.


#include "vax.h"
#include "shim.h"
#include "services.h"
#include "ss_def.h"
#include "dclrtl.h"
#include "memmap.h"
#include "logicals.h"

#include "fab.h"
#include "rab.h"

int rmsinit( void );
int get_free_ifi( void );
LONGWORD load_buffer( LONGWORD addr, char * p, LONGWORD len );

FILE * ifi[ 256 ];

//	Initialize the RMS world on the first call made.

int rmsinit( void )
{
    struct FAB * fab = 0L;
    struct RAB * rab = 0L;
    static int initialized = 0;
    int n;
    
    if( initialized )
        return VAX_OK;

    for( n = 0; n < 256; n++ )
        ifi[ n ] = 0L;
    
    ifi[ 0 ] = ( FILE * ) -1;
    ifi[ 1 ] = stdout;
    ifi[ 2 ] = stdin;
    ifi[ 3 ] = stderr;
    
    map_init( "FAB" );
    map_int( "FAB", "FAB$B_BID", STROFF( fab, fab_b_bid ), 1 );
    map_int( "FAB", "FAB$B_BLN", STROFF( fab, fab_b_bln ), 1 );
    map_int( "FAB", "FAB$W_IFI", STROFF( fab, fab_r_ifi_overlay.fab_w_ifi ), 2 );
    map_int( "FAB", "FAB$L_FOP", STROFF( fab, fab_r_fop_overlay.fab_l_fop ), 4 );
    map_int( "FAB", "FAB$L_STS", STROFF( fab, fab_l_sts ), 4 );
    map_int( "FAB", "FAB$L_STV", STROFF( fab, fab_l_stv ), 4 );
    map_int( "FAB", "FAB$L_ALQ", STROFF( fab, fab_l_alq ), 4 );
    map_int( "FAB", "FAB$W_DEQ", STROFF( fab, fab_w_deq ), 2 );
    map_int( "FAB", "FAB$B_FAC", STROFF( fab, fab_r_fac_overlay.fab_b_fac ), 1 );
    map_int( "FAB", "FAB$B_SHR", STROFF( fab, fab_r_shr_overlay.fab_b_shr ), 1 );
    map_int( "FAB", "FAB$L_CTX", STROFF( fab, fab_l_ctx ), 4 );
    map_int( "FAB", "FAB$B_RTV", STROFF( fab, fab_b_rtv ), 1 );
    map_int( "FAB", "FAB$B_ORG", STROFF( fab, fab_r_org_overlay.fab_b_org ), 1 );
    map_int( "FAB", "FAB$B_RAT", STROFF( fab, fab_r_rat_overlay.fab_b_rat ), 1 );
    map_int( "FAB", "FAB$B_RFM", STROFF( fab, fab_b_rfm ), 1 );
    map_int( "FAB", "FAB$L_JNL", STROFF( fab, fab_l_jnl_overlay.fab_l_jnl ), 4 );
    map_int( "FAB", "FAB$L_XAB", STROFF( fab, fab_l_xab ), 4 );
    map_int( "FAB", "FAB$L_NAM", STROFF( fab, fab_l_nam ), 4 );
    map_int( "FAB", "FAB$L_FNA", STROFF( fab, fab_l_fna ), 4 );
    map_int( "FAB", "FAB$B_FNS", STROFF( fab, fab_b_fns ), 1 );
    map_int( "FAB", "FAB$B_DNS", STROFF( fab, fab_b_dns ), 1 );
    map_int( "FAB", "FAB$W_MRS", STROFF( fab, fab_w_mrs ), 2 );
    map_int( "FAB", "FAB$L_MRN", STROFF( fab, fab_l_mrn ), 4 );
    map_int( "FAB", "FAB$W_BLS", STROFF( fab, fab_w_bls ), 2 );
    map_int( "FAB", "FAB$B_BKS", STROFF( fab, fab_b_bks ), 1 );
    map_int( "FAB", "FAB$B_FSZ", STROFF( fab, fab_b_fsz ), 1 );
    map_int( "FAB", "FAB$L_DEV", STROFF( fab, fab_l_dev ), 4 );
    map_int( "FAB", "FAB$L_SDC", STROFF( fab, fab_l_sdc ), 4 );
    map_int( "FAB", "FAB$W_GBC", STROFF( fab, fab_w_gbc ), 2 );
    map_int( "FAB", "FAB$B_ACMODES", STROFF( fab, fab_r_acmodes_overlay.fab_b_acmodes ), 1 );
    map_int( "FAB", "FAB$B_RCF", STROFF( fab, fab_r_rcf_overlay.fab_b_rcf ), 1 );
    
    map_init( "RAB" );
    map_int( "RAB", "RAB$B_BID", STROFF( rab, rab_b_bid ), 1 );
    map_int( "RAB", "RAB$B_BLN", STROFF( rab, rab_b_bln ), 1 );
    map_int( "RAB", "RAB$W_ISI", STROFF( rab, rab_r_isi_overlay.rab_w_isi ), 2 );
    map_int( "RAB", "RAB$L_ROP", STROFF( rab, rab_r_rop_overlay.rab_l_rop ), 4 );
    map_int( "RAB", "RAB$L_STS", STROFF( rab, rab_l_sts ), 4 );
    map_int( "RAB", "RAB$L_STV", STROFF( rab, rab_r_stv_overlay.rab_l_stv ), 4 );
    map_int( "RAB", "RAB$W_RFA0", STROFF( rab, rab_r_rfa_overlay.rab_r_rfa_fields.rab_l_rfa0 ), 4 );
    map_int( "RAB", "RAB$W_RFA4", STROFF( rab, rab_r_rfa_overlay.rab_r_rfa_fields.rab_w_rfa4 ), 2 );
    map_int( "RAB", "RAB$W_FILL4",STROFF( rab, rabdef___fill_4 ), 2 );
    map_int( "RAB", "RAB$L_CTX", STROFF( rab, rab_l_ctx ), 4 );
    map_int( "RAB", "RAB$L_FILL5",STROFF( rab, rabdef___fill_5 ), 2 );
    map_int( "RAB", "RAB$B_RAC", STROFF( rab, rab_b_rac ), 1 );
    map_int( "RAB", "RAB$B_TMO", STROFF( rab, rab_b_tmo ), 1 );
    map_int( "RAB", "RAB$W_USZ", STROFF( rab, rab_w_usz ), 2 );
    map_int( "RAB", "RAB$W_RSZ", STROFF( rab, rab_w_rsz ), 2 );
    map_int( "RAB", "RAB$L_UBF", STROFF( rab, rab_l_ubf ), 4 );
    map_int( "RAB", "RAB$L_RBF", STROFF( rab, rab_l_rbf ), 4 );
    map_int( "RAB", "RAB$L_RHB", STROFF( rab, rab_l_rhb ), 4 );
    map_int( "RAB", "RAB$L_KBF", STROFF( rab, rab_r_kbf_overlay.rab_l_kbf ), 4 );
    map_int( "RAB", "RAB$B_KSZ", STROFF( rab, rab_r_ksz_overlay.rab_b_ksz ), 1 );
    map_int( "RAB", "RAB$B_KRF", STROFF( rab, rab_b_krf ), 1 );
    map_int( "RAB", "RAB$B_MBF", STROFF( rab, rab_b_mbf ), 1 );
    map_int( "RAB", "RAB$B_MBC", STROFF( rab, rab_b_mbc ), 1 );
    map_int( "RAB", "RAB$L_BKT", STROFF( rab, rab_r_bkt_overlay.rab_l_bkt ), 4);
    map_int( "RAB", "RAB$L_FAB", STROFF( rab, rab_l_fab ), 4 );
    map_int( "RAB", "RAB$L_XAB", STROFF( rab, rab_l_xab ), 4 );
    
    
    initialized = 1;
    return VAX_OK;
}

int get_free_ifi( void )
{
    int n;
    for( n = 1; n < 255; n++ ) {
        if( ifi[ n ] == 0L )
            return n;
    }
    return 0;
}

SSDEF( rms_put )
{
    struct RAB rab;
    struct FAB fab;
    FILE * f;
    char * b;
    LONGWORD rc;
    
    rmsinit();
    if( vax.debug & DBG_RMS )
        printf( "RMS: In SYS$PUT function, RAB=%08X, ", argv[ 0 ]);
    
    rc = map( "RAB", &rab, argv[ 0 ]);
    if( rc )
        return SS_ACCVIO;

    rc = map( "FAB", &fab, rab.rab_l_fab );
    if( rc )
        return SS_ACCVIO;

    if( vax.debug & DBG_RMS ) {
        printf( " FAB=%08X  IFI=%d\n", rab.rab_l_fab,
            fab.fab_r_ifi_overlay.fab_w_ifi );
    }

    f = ifi[ fab.fab_r_ifi_overlay.fab_w_ifi ];
    if( (LONGWORD) f == 0 || (LONGWORD) f == -1 ) {
        if( vax.debug & DBG_RMS )
            printf( "RMS: invalid IFI %08X\n", (LONGWORD) f );
        return SS_INVARG; /* NEED RMS RETURN CODES */
    }
    
    switch( rab.rab_b_rac ) {
    
    case RAB_C_SEQ:
    
        if( vax.debug & DBG_RMS )
            printf( "RMS: SEQ write %d bytes from %08X\n",
                rab.rab_w_rsz, rab.rab_l_rbf );
        b = getmem( rab.rab_w_rsz );
        load_buffer( rab.rab_l_rbf, b, rab.rab_w_rsz );
        fwrite( b, 1, rab.rab_w_rsz, f );
        if( fab.fab_r_ifi_overlay.fab_w_ifi <= 3 ) {
            b[ 0 ] = '\n';
            fwrite( b, 1, 1, f );
        }
        freemem( b );
        
        break;
        
    default:
        if( vax.debug & DBG_RMS ) {
            printf( "RMS: invalid RAC code %d\n", rab.rab_b_rac );
            return SS_INVARG; /* NEED RMS RETURN CODES */
        }
    }
    
    rc = SS_NORMAL;
    store_field( argv[0], "RAB", "RAB$L_STS", ( void * ) &rc, 4 );
    store_field( argv[0], "RAB", "RAB$L_STV", ( void * ) &rc, 4 );
    
    return SS_NORMAL;
}


SSDEF( rms_connect )
{
    struct RAB rab;
    struct FAB fab;
    LONGWORD rc;
    
    rmsinit();
    if( vax.debug & DBG_RMS )
        printf("RMS: In SYS$CONNECT function, RAB=%08X.\n", argv[ 0 ]);
    
    rc = map( "RAB", &rab, argv[ 0 ]);
    rc = map( "FAB", &fab, rab.rab_l_fab );
    
    store_field( argv[0], "RAB", "RAB$W_ISI", 
        ( void * ) &fab.fab_r_ifi_overlay.fab_w_ifi, 2 );

    store_field( argv[0], "RAB", "RAB$L_STS", ( void * ) &rc, 4 );
    store_field( argv[0], "RAB", "RAB$L_STV", ( void * ) &rc, 4 );
        
    return SS_NORMAL;
}


SSDEF( rms_create )
{
    struct FAB fab;
    struct LNM * lnm;
    LONGWORD rc;
    char fn[ 256 ];
    
    rmsinit();
    rc = map( "FAB", &fab, argv[0]);

    if( vax.debug & DBG_RMS ) {
        printf( "RMS: In SYS$CREATE function, FAB=%08X,", argv[0] );    
        printf( " FAC=%d, ", fab.fab_r_fac_overlay.fab_b_fac );
    }
    
    if( !fab.fab_b_fns )
        fab.fab_b_fns = 255;
        
    fab.fab_b_fns = load_string( fab.fab_l_fna, fn, fab.fab_b_fns );
    fn[ fab.fab_b_fns ] = 0;
    if( vax.debug & DBG_RMS )
        printf( "  FNS=%d FNA=%08X \"%s\"\n",
        fab.fab_b_fns, fab.fab_l_fna, fn );

//	See if this is a logical name

    lnm = get_logical( "LNM$FILE_DEV", fn, 0 );
    if( lnm )
        strcpy( fn, lnm-> value );
    
    switch( fab.fab_r_fac_overlay.fab_b_fac ) {
    
    case FAB_M_PUT:
    
        if( strcmp( fn, "TTA0:" ) == 0 ) {
            fab.fab_r_ifi_overlay.fab_w_ifi = 1;
            break;
        }
        
        fab.fab_r_ifi_overlay.fab_w_ifi = get_free_ifi();
        ifi[ fab.fab_r_ifi_overlay.fab_w_ifi ] = fopen( fn, "w" );
        if( !ifi[ fab.fab_r_ifi_overlay.fab_w_ifi ] ) {
            return SS_NOSUCHFILE;
        }
        break;
        
    default:
        if( vax.debug & DBG_RMS )
            printf( "RMS: Unknown FAC %d\n", fab.fab_r_fac_overlay.fab_b_fac );
        return SS_NOSUCHFAC;
    }
    
    if( vax.debug & DBG_RMS )
        printf( "RMS: FN=\"%s\", writing to IFI[%d]\n", 
            fn, fab.fab_r_ifi_overlay.fab_w_ifi );
        
    store_field( argv[0], "FAB", "FAB$W_IFI",
                    ( void * ) &fab.fab_r_ifi_overlay.fab_w_ifi, 2 );
                    
    store_field( argv[0], "FAB", "FAB$L_STS", ( void * ) &rc, 4 );
    store_field( argv[0], "FAB", "FAB$L_STV", ( void * ) &rc, 4 );
    
    return SS_NORMAL;
}

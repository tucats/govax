//
//  IMGDEF.H
//
//  Definitions for VAX/VMS image activations
//


struct IAF {
    LONGWORD        offset_g_fix;
    LONGWORD        offset_shl;
    LONGWORD        offset_addr;
    LONGWORD        offset_chgprot;
    LONGWORD        offset_names;
    LONGWORD        shrimgcnt;
};


struct SHR {
    struct SHR *    next;
    char            name[ 40 ];
    LONGWORD            id;
    LONGWORD            base;
    struct ICB *    icb;
};


struct IHD {
    short       size;
    short       offset_transfer;
    short       offset_dst;
    short       offset_ident;
    short       offset_patch;
    short       minor_id;
    short       major_id;
    char        header_blocks;
    char        image_type;
    LONGWORD        mask[ 2 ];
    short       channels;
    short       io_pages;
    LONGWORD        flags;
    LONGWORD        section_id;
    LONGWORD        version;
};

struct IHI {
    char        imgnaml;
    char        imgnam[ 39 ];
    char        imgidl;
    char        imgid[ 15 ];
    LONGWORD        time[ 2 ];
    char        linkidl;
    char        linkid[ 15 ];
};

#define IHI_SIZE 80

struct ISD { 
    struct ISD *next;
    LONGWORD    valid;
    short       size;
    short       pages;
    short       vpn;
    short       pfc;
    LONGWORD        flags;
    LONGWORD        vbn;
    LONGWORD        section_id;
    char        count;
    char        name[ 15 ];
};

/*
 *  Values for the ISD flags field.
 */
 
#define ISD_V_GBL   0
#define ISD_V_CRF   1
#define ISD_V_DZRO  2
#define ISD_V_WRT   3
#define ISD_V_LASTCLU   7
#define ISD_V_COPYALWAY 8
#define ISD_V_BASED 9
#define ISD_V_FIXUPVEC  10
#define ISD_V_VECTOR    17
#define ISD_V_PROTECT   18

#define ISD_M_GBL   (1<<ISD_V_GBL)
#define ISD_M_CRF   (1<<ISD_V_CRF)
#define ISD_M_DZRO  (1<<ISD_V_DZRO)
#define ISD_M_WRT   (1<<ISD_V_WRT)
#define ISD_M_LASTCLU   (1<<ISD_V_LASTCLU)
#define ISD_M_COPYALWAY (1<<ISD_V_COPYALWAY)
#define ISD_M_BASED (1<<ISD_V_BASED)
#define ISD_M_FIXUPVEC  (1<<ISD_V_FIXUPVEC)
#define ISD_M_PROTECT   (1<<ISD_V_PROTECT)

/*
 *  Type field is high byte of flags field
 */
#define ISD_V_TYPE  24
#define ISD_M_TYPE  (0xff<<ISD_V_TYPE)
#define ISD_B_TYPE( isd_l_flags )   (((isd_l_flags)&ISD_M_TYPE)>>ISD_V_TYPE)

#define ISD_K_NORMAL    0
#define ISD_K_SHRFXD    1
#define ISD_K_PRVFXD    2
#define ISD_K_SHRPIC    3
#define ISD_K_PRVPIC    4
#define ISD_K_USRSTACK  253

/*
 *  Image control blocks -- this models the VMS ICB list, which keeps a list of
 *  each image that we know about.
 */
 
#define ICB_MAIN        0x00000001  /* This is the "main" image                 */
#define ICB_PRIMARY     0x00000001  /*    "ditto"                               */
#define ICB_SECONDARY   0x00000002  /* This is a secondary image                */
#define ICB_FIXED       0x00000004  /* Fixups are completed for this image      */
#define ICB_INCOMPLETE  0x80000000  /* This image load isn't finished yet       */

struct ICB {
    struct ICB *    next;           /* Linked list                              */
    LONGWORD        valid;          /* Used to track many<>1 mappings           */
    LONGWORD        flags;          /* Flags - are we resolved, etc.            */
    char            name[ 40 ];     /* Name of shared image                     */
    LONGWORD        base;           /* Base address of image                    */
    LONGWORD        end;            /* Last address of image                    */
    struct IHD      ihd;            /* Static part of image header              */
    LONGWORD        isd_count;      /* Number of ISD's in the list              */
    struct ISD *    isd_list;       /* ISD list for this image                  */
    struct ISD *    fixup_isd;      /* The ISD in the list for fixups           */
    LONGWORD        transfer[ 4 ];  /* Transfer vector                          */
    struct IAF      iaf;            /* Image fixup data                         */
    struct SHR  *   shr_list;       /* List of sharable images this image needs */
};

/*
 *  Misc support prototypes here as well.
 */

void reset_icb_list( void );
LONGWORD image_load( char * fn, LONGWORD flag, struct ICB ** icbptr );
LONGWORD image_fixup( struct ICB * icb );
struct ICB * find_main_icb(void);

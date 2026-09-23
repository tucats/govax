/********************************************************************************************************************************/
/* Created: 24-NOV-1996 13:48:43 by OpenVMS SDL EV1-33     */
/* Source:  24-NOV-1996 13:46:27 _$22$DIA11:[BUILD.SDL]RMSUSR.SDI;1 */
/********************************************************************************************************************************/
/*** MODULE $RABDEF ***/
#ifndef __RABDEF_LOADED
#define __RABDEF_LOADED 1
 
#pragma nostandard
 
#ifdef __cplusplus
    extern "C" {
#define __unknown_params ...
#else
#define __unknown_params
#endif
 
#if !defined(__VAXC) && !defined(VAXC)
#define __struct struct
#define __union union
#else
#define __struct variant_struct
#define __union variant_union
#endif
 
/*                                                                          */
/*         record access block (rab) definitions                            */
/*                                                                          */
/*  there is one rab per connected stream                                   */
/*  it is used for all communications between the user                      */
/*  and rms concerning operations on the stream                             */
/*                                                                          */
/*+++++*****                                                                */
/*  the fields thru ctx cannot be changed due to commonality                */
/*  with the fab                                                            */
/*                                                                          */
#define RAB$C_BID 1                     /* code for rab                     */
#define RAB$M_PPF_RAT 0x3FC0
#define RAB$M_PPF_IND 0x4000
#define RAB$M_PPISI 0x8000
#define RAB$M_ASY 0x1
#define RAB$M_TPT 0x2
#define RAB$M_REA 0x4
#define RAB$M_RRL 0x8
#define RAB$M_UIF 0x10
#define RAB$M_MAS 0x20
#define RAB$M_FDL 0x40
#define RAB$M_REV 0x80
#define RAB$M_EOF 0x100
#define RAB$M_RAH 0x200
#define RAB$M_WBH 0x400
#define RAB$M_BIO 0x800
#define RAB$M_CDK 0x1000
#define RAB$M_LOA 0x2000
#define RAB$M_LIM 0x4000
#define RAB$M_SYNCSTS 0x8000
#define RAB$M_LOC 0x10000
#define RAB$M_WAT 0x20000
#define RAB$M_ULK 0x40000
#define RAB$M_RLK 0x80000
#define RAB$M_NLK 0x100000
#define RAB$M_KGE 0x200000
#define RAB$M_KGT 0x400000
#define RAB$M_NXR 0x800000
#define RAB$M_RNE 0x1000000
#define RAB$M_TMO 0x2000000
#define RAB$M_CVT 0x4000000
#define RAB$M_RNF 0x8000000
#define RAB$M_ETO 0x10000000
#define RAB$M_PTA 0x20000000
#define RAB$M_PMT 0x40000000
#define RAB$M_CCO 0x80000000
#define RAB$M_EQNXT 0x200000
#define RAB$M_NXT 0x400000
#define RAB$C_SEQ 0                     /* sequential access                */
#define RAB$C_KEY 1                     /* keyed access                     */
#define RAB$C_RFA 2                     /* rfa access                       */
#define RAB$C_STM 3                     /* stream access (valid only for sequential org)  */
#define RAB$C_MAXRAC 2                  /* Maximum RAC value currently supported by RMS */
#define RAB$K_BLN 68                    /* length of rab                    */
#define RAB$C_BLN 68                    /* length of rab                    */
struct rabdef {
    unsigned char rab$b_bid;            /* block id                         */
    unsigned char rab$b_bln;            /* block length                     */
    __union  {
        unsigned short int rab$w_isi;   /* internal stream index            */
/* (ifi in fab)                                                             */
        __struct  {
            unsigned rabdef$$_fill_1 : 6; /* move to bit 6                  */
            unsigned rab$v_ppf_rat : 8; /* rat value for process-permanent files  */
            unsigned rab$v_ppf_ind : 1; /* indirect access to process-permanent file  */
/* (i.e., restricted operations)                                            */
            unsigned rab$v_ppisi : 1;   /* indicates that this is process-permanent stream */
            } rab$r_isi_bits;
        } rab$r_isi_overlay;
    __union  {
        unsigned int rab$l_rop;         /* record options                   */
        __struct  {
            unsigned rab$v_asy : 1;     /* asynchronous operations          */
            unsigned rab$v_tpt : 1;     /* truncate put - allow sequential put not at  */
/*   eof, thus truncating file (seq. org only)                              */
/*                                                                          */
/* these next two should be in the byte for bits                            */
/* input to $find or $get, but there is no room there                       */
/*                                                                          */
            unsigned rab$v_rea : 1;     /* lock record for read only, allow other readers  */
            unsigned rab$v_rrl : 1;     /* read record regardless of lock   */
/*                                                                          */
            unsigned rab$v_uif : 1;     /* update if existent               */
            unsigned rab$v_mas : 1;     /* mass-insert mode                 */
            unsigned rab$v_fdl : 1;     /* fast record deletion             */
            unsigned rab$v_rev : 1;     /* reverse-search - can only be set with NXT or EQNXT */
/*                                                                          */
            unsigned rab$v_eof : 1;     /* connect to eof                   */
            unsigned rab$v_rah : 1;     /* read ahead                       */
            unsigned rab$v_wbh : 1;     /* write behind                     */
            unsigned rab$v_bio : 1;     /* connect for bio only             */
            unsigned rab$v_cdk : 1;     /* check for duplicate keys on $GET */
            unsigned rab$v_loa : 1;     /* use bucket fill percentage       */
            unsigned rab$v_lim : 1;     /* compare for key limit reached on $get/$find seq. (idx only)  */
            unsigned rab$v_syncsts : 1; /* Synchronous status notification for asynchronous routines. */
/*                                                                          */
/* the following bits are input to                                          */
/* $find or $get, (see above also REA and RRL)                              */
/* (separate byte)                                                          */
/*                                                                          */
            unsigned rab$v_loc : 1;     /* use locate mode                  */
            unsigned rab$v_wat : 1;     /* wait if record not available     */
            unsigned rab$v_ulk : 1;     /* manual unlocking                 */
            unsigned rab$v_rlk : 1;     /* allow readers for this locked record  */
            unsigned rab$v_nlk : 1;     /* do not lock record               */
            unsigned rab$v_kge : 1;     /* key > or =                       */
            unsigned rab$v_kgt : 1;     /* key greater than                 */
            unsigned rab$v_nxr : 1;     /* get non-existent record          */
/*                                                                          */
/*  the following bits are terminal qualifiers only                         */
/*  (separate byte)                                                         */
/*                                                                          */
            unsigned rab$v_rne : 1;     /* read no echo                     */
            unsigned rab$v_tmo : 1;     /* use time-out period              */
            unsigned rab$v_cvt : 1;     /* convert to upper case            */
            unsigned rab$v_rnf : 1;     /* read no filter                   */
            unsigned rab$v_eto : 1;     /* extended terminal operation      */
            unsigned rab$v_pta : 1;     /* purge type ahead                 */
            unsigned rab$v_pmt : 1;     /* use prompt buffer                */
            unsigned rab$v_cco : 1;     /* cancel control o on output       */
            } rab$r_rop_bits0;
        __struct  {
            unsigned rabdef$$_fill_6 : 21;
            unsigned rab$v_eqnxt : 1;   /* Synonyms for KGE and             */
            unsigned rab$v_nxt : 1;     /*   KGT                            */
            unsigned rab$v_fill_4 : 1;
            } rab$r_rop_bits1;
/*                                                                          */
/* the following bits may be                                                */
/* input to various rab-related                                             */
/* operations                                                               */
/*                                                                          */
        __struct  {
            char rabdef$$_fill_3;
            unsigned char rab$b_rop1;   /* various options                  */
            unsigned char rab$b_rop2;   /* get/find options (use of this field discouraged  */
/* due to REA and RRL being in a different byte)                            */
            unsigned char rab$b_rop3;   /* terminal read options            */
/*                                                                          */
            } rab$r_rop_fields;
        } rab$r_rop_overlay;
    unsigned int rab$l_sts;             /* status                           */
    __union  {
        unsigned int rab$l_stv;         /* status value                     */
        __struct  {
            unsigned short int rab$w_stv0; /* low word of stv               */
            unsigned short int rab$w_stv2; /* high word of stv              */
            } rab$r_stv_fields;
        } rab$r_stv_overlay;
    __union  {
        unsigned short int rab$w_rfa [3]; /* record's file address          */
        __struct  {
            unsigned int rab$l_rfa0;
            unsigned short int rab$w_rfa4;
            } rab$r_rfa_fields;
        } rab$r_rfa_overlay;
    short int rabdef$$_fill_4;          /* (reserved - rms release 1 optimizes stores  */
/*  to the rfa field to be a move quad, overwriting                         */
/*  this reserved word)                                                     */
    unsigned int rab$l_ctx;             /* user context                     */
/*-----*****                                                                */
    short int rabdef$$_fill_5;          /* (spare)                          */
    unsigned char rab$b_rac;            /* record access                    */
    unsigned char rab$b_tmo;            /* time-out period                  */
    unsigned short int rab$w_usz;       /* user buffer size                 */
    unsigned short int rab$w_rsz;       /* record buffer size               */
    unsigned int rab$l_ubf;             /* user buffer address              */
    unsigned int rab$l_rbf;             /* record buffer address            */
    unsigned int rab$l_rhb;             /* record header buffer addr        */
    __union  {
        unsigned int rab$l_kbf;         /* key buffer address               */
        unsigned int rab$l_pbf;         /* prompt buffer addr               */
        } rab$r_kbf_overlay;
    __union  {
        unsigned char rab$b_ksz;        /* key buffer size                  */
        unsigned char rab$b_psz;        /* prompt buffer size               */
        } rab$r_ksz_overlay;
    unsigned char rab$b_krf;            /* key of reference                 */
    char rab$b_mbf;                     /* multi-buffer count               */
    unsigned char rab$b_mbc;            /* multi-block count                */
    __union  {
        unsigned int rab$l_bkt;         /* bucket hash code, vbn, or rrn    */
        unsigned int rab$l_dct;         /* duplicates count on key accessed on alternate key  */
        } rab$r_bkt_overlay;
    unsigned int rab$l_fab;             /* related fab for connect          */
    unsigned int rab$l_xab;             /* XAB address                      */
    } ;
 
#if !defined(__VAXC) && !defined(VAXC)
#define rab$w_isi rab$r_isi_overlay.rab$w_isi
#define rab$v_ppf_rat rab$r_isi_overlay.rab$r_isi_bits.rab$v_ppf_rat
#define rab$v_ppf_ind rab$r_isi_overlay.rab$r_isi_bits.rab$v_ppf_ind
#define rab$v_ppisi rab$r_isi_overlay.rab$r_isi_bits.rab$v_ppisi
#define rab$l_rop rab$r_rop_overlay.rab$l_rop
#define rab$v_asy rab$r_rop_overlay.rab$r_rop_bits0.rab$v_asy
#define rab$v_tpt rab$r_rop_overlay.rab$r_rop_bits0.rab$v_tpt
#define rab$v_rea rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rea
#define rab$v_rrl rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rrl
#define rab$v_uif rab$r_rop_overlay.rab$r_rop_bits0.rab$v_uif
#define rab$v_mas rab$r_rop_overlay.rab$r_rop_bits0.rab$v_mas
#define rab$v_fdl rab$r_rop_overlay.rab$r_rop_bits0.rab$v_fdl
#define rab$v_rev rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rev
#define rab$v_eof rab$r_rop_overlay.rab$r_rop_bits0.rab$v_eof
#define rab$v_rah rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rah
#define rab$v_wbh rab$r_rop_overlay.rab$r_rop_bits0.rab$v_wbh
#define rab$v_bio rab$r_rop_overlay.rab$r_rop_bits0.rab$v_bio
#define rab$v_cdk rab$r_rop_overlay.rab$r_rop_bits0.rab$v_cdk
#define rab$v_loa rab$r_rop_overlay.rab$r_rop_bits0.rab$v_loa
#define rab$v_lim rab$r_rop_overlay.rab$r_rop_bits0.rab$v_lim
#define rab$v_syncsts rab$r_rop_overlay.rab$r_rop_bits0.rab$v_syncsts
#define rab$v_loc rab$r_rop_overlay.rab$r_rop_bits0.rab$v_loc
#define rab$v_wat rab$r_rop_overlay.rab$r_rop_bits0.rab$v_wat
#define rab$v_ulk rab$r_rop_overlay.rab$r_rop_bits0.rab$v_ulk
#define rab$v_rlk rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rlk
#define rab$v_nlk rab$r_rop_overlay.rab$r_rop_bits0.rab$v_nlk
#define rab$v_kge rab$r_rop_overlay.rab$r_rop_bits0.rab$v_kge
#define rab$v_kgt rab$r_rop_overlay.rab$r_rop_bits0.rab$v_kgt
#define rab$v_nxr rab$r_rop_overlay.rab$r_rop_bits0.rab$v_nxr
#define rab$v_rne rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rne
#define rab$v_tmo rab$r_rop_overlay.rab$r_rop_bits0.rab$v_tmo
#define rab$v_cvt rab$r_rop_overlay.rab$r_rop_bits0.rab$v_cvt
#define rab$v_rnf rab$r_rop_overlay.rab$r_rop_bits0.rab$v_rnf
#define rab$v_eto rab$r_rop_overlay.rab$r_rop_bits0.rab$v_eto
#define rab$v_pta rab$r_rop_overlay.rab$r_rop_bits0.rab$v_pta
#define rab$v_pmt rab$r_rop_overlay.rab$r_rop_bits0.rab$v_pmt
#define rab$v_cco rab$r_rop_overlay.rab$r_rop_bits0.rab$v_cco
#define rab$v_eqnxt rab$r_rop_overlay.rab$r_rop_bits1.rab$v_eqnxt
#define rab$v_nxt rab$r_rop_overlay.rab$r_rop_bits1.rab$v_nxt
#define rab$b_rop1 rab$r_rop_overlay.rab$r_rop_fields.rab$b_rop1
#define rab$b_rop2 rab$r_rop_overlay.rab$r_rop_fields.rab$b_rop2
#define rab$b_rop3 rab$r_rop_overlay.rab$r_rop_fields.rab$b_rop3
#define rab$l_stv rab$r_stv_overlay.rab$l_stv
#define rab$w_stv0 rab$r_stv_overlay.rab$r_stv_fields.rab$w_stv0
#define rab$w_stv2 rab$r_stv_overlay.rab$r_stv_fields.rab$w_stv2
#define rab$w_rfa rab$r_rfa_overlay.rab$w_rfa
#define rab$l_rfa0 rab$r_rfa_overlay.rab$r_rfa_fields.rab$l_rfa0
#define rab$w_rfa4 rab$r_rfa_overlay.rab$r_rfa_fields.rab$w_rfa4
#define rab$l_kbf rab$r_kbf_overlay.rab$l_kbf
#define rab$l_pbf rab$r_kbf_overlay.rab$l_pbf
#define rab$b_ksz rab$r_ksz_overlay.rab$b_ksz
#define rab$b_psz rab$r_ksz_overlay.rab$b_psz
#define rab$l_bkt rab$r_bkt_overlay.rab$l_bkt
#define rab$l_dct rab$r_bkt_overlay.rab$l_dct
#endif		/* #if !defined(__VAXC) && !defined(VAXC) */
 
 
#ifdef __cplusplus
    }
#endif
#pragma standard
 
#endif /* __RABDEF_LOADED */
 

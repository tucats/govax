/********************************************************************************************************************************/
/* Created: 24-NOV-1996 13:48:43 by OpenVMS SDL EV1-33     */
/* Source:  24-NOV-1996 13:46:27 _$22$DIA11:[BUILD.SDL]RMSUSR.SDI;1 */
/********************************************************************************************************************************/
/*** MODULE $FABDEF ***/
#ifndef __FABDEF_LOADED
#define __FABDEF_LOADED 1
 
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
 
/*+++++*****                                                                */
/*   the fields thru ctx must not be modified due to                        */
/*   commonality between fab/rab/xab                                        */
#define FAB$C_BID 3                     /* code for fab                     */
#define FAB$M_PPF_RAT 0x3FC0
#define FAB$M_PPF_IND 0x4000
#define FAB$M_PPIFI 0x8000
#define FAB$M_ASY 0x1
#define FAB$M_MXV 0x2
#define FAB$M_SUP 0x4
#define FAB$M_TMP 0x8
#define FAB$M_TMD 0x10
#define FAB$M_DFW 0x20
#define FAB$M_SQO 0x40
#define FAB$M_RWO 0x80
#define FAB$M_POS 0x100
#define FAB$M_WCK 0x200
#define FAB$M_NEF 0x400
#define FAB$M_RWC 0x800
#define FAB$M_DMO 0x1000
#define FAB$M_SPL 0x2000
#define FAB$M_SCF 0x4000
#define FAB$M_DLT 0x8000
#define FAB$M_NFS 0x10000
#define FAB$M_UFO 0x20000
#define FAB$M_PPF 0x40000
#define FAB$M_INP 0x80000
#define FAB$M_CTG 0x100000
#define FAB$M_CBT 0x200000
#define FAB$M_SYNCSTS 0x400000
#define FAB$M_RCK 0x800000
#define FAB$M_NAM 0x1000000
#define FAB$M_CIF 0x2000000
#define FAB$M_ESC 0x8000000
#define FAB$M_TEF 0x10000000
#define FAB$M_OFP 0x20000000
#define FAB$M_KFO 0x40000000
#define FAB$M_PUT 0x1
#define FAB$M_GET 0x2
#define FAB$M_DEL 0x4
#define FAB$M_UPD 0x8
#define FAB$M_TRN 0x10
#define FAB$M_BIO 0x20
#define FAB$M_BRO 0x40
#define FAB$M_EXE 0x80
#define FAB$M_SHRPUT 0x1
#define FAB$M_SHRGET 0x2
#define FAB$M_SHRDEL 0x4
#define FAB$M_SHRUPD 0x8
#define FAB$M_MSE 0x10
#define FAB$M_NIL 0x20
#define FAB$M_UPI 0x40
#define FAB$M_ORG 0xF0
#define FAB$C_SEQ 0                     /* sequential                       */
#define FAB$C_REL 16                    /* relative                         */
#define FAB$C_IDX 32                    /* indexed                          */
#define FAB$C_HSH 48                    /* hashed                           */
#define FAB$M_FTN 0x1
#define FAB$M_CR 0x2
#define FAB$M_PRN 0x4
#define FAB$M_BLK 0x8
#define FAB$M_MSB 0x10
#define FAB$C_RFM_DFLT 2                /* var len is default               */
#define FAB$C_UDF 0                     /* undefined (also stream binary)   */
#define FAB$C_FIX 1                     /* fixed length records             */
#define FAB$C_VAR 2                     /* variable length records          */
#define FAB$C_VFC 3                     /* variable fixed control           */
#define FAB$C_STM 4                     /* RMS-11 stream (valid only for sequential org)  */
#define FAB$C_STMLF 5                   /* LF stream (valid only for sequential org)  */
#define FAB$C_STMCR 6                   /* CR stream (valid only for sequential org)  */
#define FAB$C_MAXRFM 6                  /* maximum rfm supported            */
#define FAB$M_ONLY_RU 0x1
#define FAB$M_RU 0x2
#define FAB$M_BI 0x4
#define FAB$M_AI 0x8
#define FAB$M_AT 0x10
#define FAB$M_NEVER_RU 0x20
#define FAB$M_JOURNAL_FILE 0x40
#define FAB$M_RCF_RU 0x1
#define FAB$M_RCF_AI 0x2
#define FAB$M_RCF_BI 0x4
#define FAB$K_BLN 80                    /* length of fab                    */
#define FAB$C_BLN 80                    /* length of fab                    */
struct fabdef {
    unsigned char fab$b_bid;            /* block id                         */
    unsigned char fab$b_bln;            /* block len                        */
    __union  {
        unsigned short int fab$w_ifi;   /* internal file index              */
        __struct  {
            unsigned fabdef$$_fill_1 : 6; /* move to bit 6                  */
            unsigned fab$v_ppf_rat : 8; /* rat value for process-permanent files  */
            unsigned fab$v_ppf_ind : 1; /* indirect access to process-permanent file */
/* (i.e., restricted operations)                                            */
            unsigned fab$v_ppifi : 1;   /* indicates that this is PPF file  */
            } fab$r_ifi_bits;
        } fab$r_ifi_overlay;
    __union  {
        unsigned int fab$l_fop;         /* file options                     */
        __struct  {
            unsigned fab$v_asy : 1;     /* asynchronous operations          */
            unsigned fab$v_mxv : 1;     /* maximize version number          */
            unsigned fab$v_sup : 1;     /* supersede existing file          */
            unsigned fab$v_tmp : 1;     /* create temporary file            */
            unsigned fab$v_tmd : 1;     /* create temp file marked for delete  */
            unsigned fab$v_dfw : 1;     /* deferred write (rel and idx)     */
            unsigned fab$v_sqo : 1;     /* sequential access only           */
            unsigned fab$v_rwo : 1;     /* rewind mt on open                */
            unsigned fab$v_pos : 1;     /* use next magtape position        */
            unsigned fab$v_wck : 1;     /* write checking                   */
            unsigned fab$v_nef : 1;     /* inhibit end of file positioning  */
            unsigned fab$v_rwc : 1;     /* rewind mt on close               */
            unsigned fab$v_dmo : 1;     /* dismount mt on close (not implemented)  */
            unsigned fab$v_spl : 1;     /* spool file on close              */
            unsigned fab$v_scf : 1;     /* submit command file on close     */
            unsigned fab$v_dlt : 1;     /* delete sub-option                */
            unsigned fab$v_nfs : 1;     /* non-file structured operation    */
            unsigned fab$v_ufo : 1;     /* user file open - no rms operations  */
            unsigned fab$v_ppf : 1;     /* process permanent file (pio segment)  */
            unsigned fab$v_inp : 1;     /* process-permanent file is 'input'  */
            unsigned fab$v_ctg : 1;     /* contiguous extension             */
            unsigned fab$v_cbt : 1;     /* contiguous best try              */
            unsigned fab$v_syncsts : 1; /* Synchronous status notification for asynchronous routines. */
            unsigned fab$v_rck : 1;     /* read checking                    */
            unsigned fab$v_nam : 1;     /* use name block dvi, did, and/or fid fields for open  */
            unsigned fab$v_cif : 1;     /* create if non-existent           */
            unsigned fabdef$$_fill_3 : 1; /* reserved (was UFM bitfield)    */
            unsigned fab$v_esc : 1;     /* 'escape' to non-standard function ($modify)  */
            unsigned fab$v_tef : 1;     /* truncate at eof on close (write-accessed seq. disk file only)  */
            unsigned fab$v_ofp : 1;     /* output file parse (only name type sticky)  */
            unsigned fab$v_kfo : 1;     /* known file open (image activator only release 1)  */
            unsigned fabdef$$_fill_4 : 1; /* reserved (not implemented)     */
            } fab$r_fop_bits;
        } fab$r_fop_overlay;
    unsigned int fab$l_sts;             /* status                           */
    unsigned int fab$l_stv;             /* status value                     */
    unsigned int fab$l_alq;             /* allocation quantity              */
    unsigned short int fab$w_deq;       /* default allocation quantity      */
    __union  {
        unsigned char fab$b_fac;        /* file access                      */
        __struct  {
            unsigned fab$v_put : 1;     /* put access                       */
            unsigned fab$v_get : 1;     /* get access                       */
            unsigned fab$v_del : 1;     /* delete access                    */
            unsigned fab$v_upd : 1;     /* update access                    */
            unsigned fab$v_trn : 1;     /* truncate access                  */
            unsigned fab$v_bio : 1;     /* block i/o access                 */
            unsigned fab$v_bro : 1;     /* block and record i/o access      */
            unsigned fab$v_exe : 1;     /* execute access (caller must be exec or kernel mode,  */
/*  ufo must also be set)                                                   */
            } fab$r_fac_bits;
        } fab$r_fac_overlay;
    __union  {
        unsigned char fab$b_shr;        /* file sharing                     */
        __struct  {
            unsigned fab$v_shrput : 1;  /* put access                       */
            unsigned fab$v_shrget : 1;  /* get access                       */
            unsigned fab$v_shrdel : 1;  /* delete access                    */
            unsigned fab$v_shrupd : 1;  /* update access                    */
            unsigned fab$v_mse : 1;     /* multi-stream connects enabled    */
            unsigned fab$v_nil : 1;     /* no sharing                       */
            unsigned fab$v_upi : 1;     /* user provided interlocking (allows multiple  */
/*  writers to seq. files)                                                  */
            unsigned fab$v_fill_0 : 1;
            } fab$r_shr_bits;
        } fab$r_shr_overlay;
    unsigned int fab$l_ctx;             /* user context                     */
/*-----*****                                                                */
    char fab$b_rtv;                     /* retrieval window size            */
    __union  {
        unsigned char fab$b_org;        /* file organization                */
        __struct  {
            unsigned fabdef$$_fill_5 : 4;
            unsigned fab$v_org : 4;
            } fab$r_org_bits;
        } fab$r_org_overlay;
    __union  {
        unsigned char fab$b_rat;        /* record format                    */
        __struct  {
            unsigned fab$v_ftn : 1;     /* fortran carriage-ctl             */
            unsigned fab$v_cr : 1;      /* lf-record-cr carriage ctl        */
            unsigned fab$v_prn : 1;     /* print-file carriage ctl          */
            unsigned fab$v_blk : 1;     /* records don't cross block boundaries  */
            unsigned fab$v_msb : 1;     /* MSB formatted byte count         */
            unsigned fab$v_fill_1 : 3;
            } fab$r_rat_bits;
        } fab$r_rat_overlay;
    unsigned char fab$b_rfm;            /* record format                    */
    __union  {
        unsigned char fab$b_journal;    /* journaling options (from FH2$B_JOURNAL) */
        __struct  {                     /* note: only one of RU, ONLY_RU, NEVER_RU */
/* may be set at a time                                                     */
            unsigned fab$v_only_ru : 1; /* file is accessible only in recovery unit  */
            unsigned fab$v_ru : 1;      /* enable recovery unit journal     */
            unsigned fab$v_bi : 1;      /* enable before image journal      */
            unsigned fab$v_ai : 1;      /* enable after image journal       */
            unsigned fab$v_at : 1;      /* enable audit trail journal       */
            unsigned fab$v_never_ru : 1; /* file is never accessible in recovery unit */
            unsigned fab$v_journal_file : 1; /* this is a journal file      */
            unsigned fab$v_fill_2 : 1;
            } fab$r_journal_bits;
        } fab$r_journal_overlay;
    unsigned char fab$b_ru_facility;    /* recoverable facility id number   */
    short int fabdef$$_fill_7;          /* (spare)                          */
    unsigned int fab$l_xab;             /* xab address                      */
    unsigned int fab$l_nam;             /* nam block address                */
    unsigned int fab$l_fna;             /* file name string address         */
    unsigned int fab$l_dna;             /* default file name string addr    */
    unsigned char fab$b_fns;            /* file name string size            */
    unsigned char fab$b_dns;            /* default name string size         */
    unsigned short int fab$w_mrs;       /* maximum record size              */
    unsigned int fab$l_mrn;             /* maximum record number            */
    unsigned short int fab$w_bls;       /* blocksize for tape               */
    unsigned char fab$b_bks;            /* bucket size                      */
    unsigned char fab$b_fsz;            /* fixed header size                */
    unsigned int fab$l_dev;             /* device characteristics           */
    unsigned int fab$l_sdc;             /* spooling device characteristics  */
    unsigned short int fab$w_gbc;       /* Global buffer count              */
    __union  {
        unsigned char fab$b_acmodes;    /* agent access modes               */
        __struct  {
            unsigned fab$v_lnm_mode : 2; /* ACMODE for log nams             */
            unsigned fab$v_chan_mode : 2; /* ACMODE for channel             */
            unsigned fab$v_file_mode : 2; /* ACMODE to use for determining file accessibility */
            unsigned fab$v_callers_mode : 2; /* ACMODE for user structure probing; */
/* maximized with actual mode of caller                                     */
            } fab$r_acmodes_bits;
        } fab$r_acmodes_overlay;
    __union  {                          /* recovery control flags           */
        unsigned char fab$b_rcf;        /* (only for use by RMS Recovery)   */
        __struct  {
            unsigned fab$v_rcf_ru : 1;  /* recovery unit recovery           */
            unsigned fab$v_rcf_ai : 1;  /* after image recovery             */
            unsigned fab$v_rcf_bi : 1;  /* before image recovery            */
            unsigned fab$v_fill_3 : 5;
            } fab$r_rcf_bits;
        } fab$r_rcf_overlay;
    int fabdef$$_fill_9;                /* (spare)                          */
    } ;
 
#if !defined(__VAXC) && !defined(VAXC)
#define fab$w_ifi fab$r_ifi_overlay.fab$w_ifi
#define fab$v_ppf_rat fab$r_ifi_overlay.fab$r_ifi_bits.fab$v_ppf_rat
#define fab$v_ppf_ind fab$r_ifi_overlay.fab$r_ifi_bits.fab$v_ppf_ind
#define fab$v_ppifi fab$r_ifi_overlay.fab$r_ifi_bits.fab$v_ppifi
#define fab$l_fop fab$r_fop_overlay.fab$l_fop
#define fab$v_asy fab$r_fop_overlay.fab$r_fop_bits.fab$v_asy
#define fab$v_mxv fab$r_fop_overlay.fab$r_fop_bits.fab$v_mxv
#define fab$v_sup fab$r_fop_overlay.fab$r_fop_bits.fab$v_sup
#define fab$v_tmp fab$r_fop_overlay.fab$r_fop_bits.fab$v_tmp
#define fab$v_tmd fab$r_fop_overlay.fab$r_fop_bits.fab$v_tmd
#define fab$v_dfw fab$r_fop_overlay.fab$r_fop_bits.fab$v_dfw
#define fab$v_sqo fab$r_fop_overlay.fab$r_fop_bits.fab$v_sqo
#define fab$v_rwo fab$r_fop_overlay.fab$r_fop_bits.fab$v_rwo
#define fab$v_pos fab$r_fop_overlay.fab$r_fop_bits.fab$v_pos
#define fab$v_wck fab$r_fop_overlay.fab$r_fop_bits.fab$v_wck
#define fab$v_nef fab$r_fop_overlay.fab$r_fop_bits.fab$v_nef
#define fab$v_rwc fab$r_fop_overlay.fab$r_fop_bits.fab$v_rwc
#define fab$v_dmo fab$r_fop_overlay.fab$r_fop_bits.fab$v_dmo
#define fab$v_spl fab$r_fop_overlay.fab$r_fop_bits.fab$v_spl
#define fab$v_scf fab$r_fop_overlay.fab$r_fop_bits.fab$v_scf
#define fab$v_dlt fab$r_fop_overlay.fab$r_fop_bits.fab$v_dlt
#define fab$v_nfs fab$r_fop_overlay.fab$r_fop_bits.fab$v_nfs
#define fab$v_ufo fab$r_fop_overlay.fab$r_fop_bits.fab$v_ufo
#define fab$v_ppf fab$r_fop_overlay.fab$r_fop_bits.fab$v_ppf
#define fab$v_inp fab$r_fop_overlay.fab$r_fop_bits.fab$v_inp
#define fab$v_ctg fab$r_fop_overlay.fab$r_fop_bits.fab$v_ctg
#define fab$v_cbt fab$r_fop_overlay.fab$r_fop_bits.fab$v_cbt
#define fab$v_syncsts fab$r_fop_overlay.fab$r_fop_bits.fab$v_syncsts
#define fab$v_rck fab$r_fop_overlay.fab$r_fop_bits.fab$v_rck
#define fab$v_nam fab$r_fop_overlay.fab$r_fop_bits.fab$v_nam
#define fab$v_cif fab$r_fop_overlay.fab$r_fop_bits.fab$v_cif
#define fab$v_esc fab$r_fop_overlay.fab$r_fop_bits.fab$v_esc
#define fab$v_tef fab$r_fop_overlay.fab$r_fop_bits.fab$v_tef
#define fab$v_ofp fab$r_fop_overlay.fab$r_fop_bits.fab$v_ofp
#define fab$v_kfo fab$r_fop_overlay.fab$r_fop_bits.fab$v_kfo
#define fab$b_fac fab$r_fac_overlay.fab$b_fac
#define fab$v_put fab$r_fac_overlay.fab$r_fac_bits.fab$v_put
#define fab$v_get fab$r_fac_overlay.fab$r_fac_bits.fab$v_get
#define fab$v_del fab$r_fac_overlay.fab$r_fac_bits.fab$v_del
#define fab$v_upd fab$r_fac_overlay.fab$r_fac_bits.fab$v_upd
#define fab$v_trn fab$r_fac_overlay.fab$r_fac_bits.fab$v_trn
#define fab$v_bio fab$r_fac_overlay.fab$r_fac_bits.fab$v_bio
#define fab$v_bro fab$r_fac_overlay.fab$r_fac_bits.fab$v_bro
#define fab$v_exe fab$r_fac_overlay.fab$r_fac_bits.fab$v_exe
#define fab$b_shr fab$r_shr_overlay.fab$b_shr
#define fab$v_shrput fab$r_shr_overlay.fab$r_shr_bits.fab$v_shrput
#define fab$v_shrget fab$r_shr_overlay.fab$r_shr_bits.fab$v_shrget
#define fab$v_shrdel fab$r_shr_overlay.fab$r_shr_bits.fab$v_shrdel
#define fab$v_shrupd fab$r_shr_overlay.fab$r_shr_bits.fab$v_shrupd
#define fab$v_mse fab$r_shr_overlay.fab$r_shr_bits.fab$v_mse
#define fab$v_nil fab$r_shr_overlay.fab$r_shr_bits.fab$v_nil
#define fab$v_upi fab$r_shr_overlay.fab$r_shr_bits.fab$v_upi
#define fab$b_org fab$r_org_overlay.fab$b_org
#define fab$v_org fab$r_org_overlay.fab$r_org_bits.fab$v_org
#define fab$b_rat fab$r_rat_overlay.fab$b_rat
#define fab$v_ftn fab$r_rat_overlay.fab$r_rat_bits.fab$v_ftn
#define fab$v_cr fab$r_rat_overlay.fab$r_rat_bits.fab$v_cr
#define fab$v_prn fab$r_rat_overlay.fab$r_rat_bits.fab$v_prn
#define fab$v_blk fab$r_rat_overlay.fab$r_rat_bits.fab$v_blk
#define fab$v_msb fab$r_rat_overlay.fab$r_rat_bits.fab$v_msb
#define fab$b_journal fab$r_journal_overlay.fab$b_journal
#define fab$v_only_ru fab$r_journal_overlay.fab$r_journal_bits.fab$v_only_ru
#define fab$v_ru fab$r_journal_overlay.fab$r_journal_bits.fab$v_ru
#define fab$v_bi fab$r_journal_overlay.fab$r_journal_bits.fab$v_bi
#define fab$v_ai fab$r_journal_overlay.fab$r_journal_bits.fab$v_ai
#define fab$v_at fab$r_journal_overlay.fab$r_journal_bits.fab$v_at
#define fab$v_never_ru fab$r_journal_overlay.fab$r_journal_bits.fab$v_never_ru
#define fab$v_journal_file fab$r_journal_overlay.fab$r_journal_bits.fab$v_journal_file
#define fab$b_acmodes fab$r_acmodes_overlay.fab$b_acmodes
#define fab$v_lnm_mode fab$r_acmodes_overlay.fab$r_acmodes_bits.fab$v_lnm_mode
#define fab$v_chan_mode fab$r_acmodes_overlay.fab$r_acmodes_bits.fab$v_chan_mode
#define fab$v_file_mode fab$r_acmodes_overlay.fab$r_acmodes_bits.fab$v_file_mode
#define fab$v_callers_mode fab$r_acmodes_overlay.fab$r_acmodes_bits.fab$v_callers_mode
#define fab$b_rcf fab$r_rcf_overlay.fab$b_rcf
#define fab$v_rcf_ru fab$r_rcf_overlay.fab$r_rcf_bits.fab$v_rcf_ru
#define fab$v_rcf_ai fab$r_rcf_overlay.fab$r_rcf_bits.fab$v_rcf_ai
#define fab$v_rcf_bi fab$r_rcf_overlay.fab$r_rcf_bits.fab$v_rcf_bi
#endif		/* #if !defined(__VAXC) && !defined(VAXC) */
 
 
#ifdef __cplusplus
    }
#endif
#pragma standard
 
#endif /* __FABDEF_LOADED */
 

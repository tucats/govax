package vmserrors

// RMS facility message IDs -- file I/O, matching real VMS's RMS (Record
// Management Services) facility. Private: only the composite RMS_* codes
// below are part of this package's public API.
//
// Grouped by originating subsystem: internal/asm's .INCLUDE file
// resolution first, then internal/console's ROM/NVRAM image loading and
// sharable-image file lookup.
const (
	rmsNoResolver uint32 = iota + 1
	rmsInclude
	rmsNoImage
	rmsReadField
	rmsReadPage
	rmsBadMagic
	rmsImageTooLarge
	rmsImageNotFound
	rmsShortHeader
)

// RMS facility status codes -- RMS_ prefix, matching real VMS's RMS$_
// status codes.
const (
	// RMS_NORESOLVER reports .INCLUDE used with no include resolver
	// configured (a batch-assembler configuration gap, not a real file
	// I/O failure).
	RMS_NORESOLVER = RMSFacility<<FacilityPosition | rmsNoResolver<<MessagePosition | StatusError
	// RMS_INCLUDE wraps the underlying error from resolving/reading a
	// .INCLUDE target file.
	RMS_INCLUDE = RMSFacility<<FacilityPosition | rmsInclude<<MessagePosition | StatusError
	// RMS_NOIMAGE/RMS_READFIELD/RMS_READPAGE/RMS_BADMAGIC/
	// RMS_IMAGETOOLARGE report ROM/NVRAM loading failures
	// (internal/console's rom.go).
	RMS_NOIMAGE       = RMSFacility<<FacilityPosition | rmsNoImage<<MessagePosition | StatusError
	RMS_READFIELD     = RMSFacility<<FacilityPosition | rmsReadField<<MessagePosition | StatusError
	RMS_READPAGE      = RMSFacility<<FacilityPosition | rmsReadPage<<MessagePosition | StatusError
	RMS_BADMAGIC      = RMSFacility<<FacilityPosition | rmsBadMagic<<MessagePosition | StatusError
	RMS_IMAGETOOLARGE = RMSFacility<<FacilityPosition | rmsImageTooLarge<<MessagePosition | StatusError
	// RMS_IMAGENOTFOUND/RMS_SHORTHEADER report a sharable-image load
	// failure (internal/console's image.go).
	RMS_IMAGENOTFOUND = RMSFacility<<FacilityPosition | rmsImageNotFound<<MessagePosition | StatusError
	RMS_SHORTHEADER   = RMSFacility<<FacilityPosition | rmsShortHeader<<MessagePosition | StatusError
)

func init() {
	DefineMessage(RMS_NORESOLVER, RMSFacility, "NORESOLVER", ".INCLUDE !Q: no include resolver configured")
	DefineMessage(RMS_INCLUDE, RMSFacility, "INCLUDE", ".INCLUDE !Q")
	DefineMessage(RMS_NOIMAGE, RMSFacility, "NOIMAGE", "No !S image loaded")
	DefineMessage(RMS_READFIELD, RMSFacility, "READFIELD", "Reading !S !S")
	DefineMessage(RMS_READPAGE, RMSFacility, "READPAGE", "Reading !S page at !XL")
	DefineMessage(RMS_BADMAGIC, RMSFacility, "BADMAGIC", "!S is not a binary ROM image")
	DefineMessage(RMS_IMAGETOOLARGE, RMSFacility, "IMAGETOOLARGE", "!S image too large; error loading page at !XL")
	DefineMessage(RMS_IMAGENOTFOUND, RMSFacility, "IMAGENOTFOUND", "Image !S not found")
	DefineMessage(RMS_SHORTHEADER, RMSFacility, "SHORTHEADER", "Image !S: file too short for a header")
}

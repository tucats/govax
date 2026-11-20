package build

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/link"
)

// TestCommittedImage checks that the committed ftnrtl.exe is what its
// sources build: run "go generate ./internal/ftnrtl" after changing them.
func TestCommittedImage(t *testing.T) {
	built, err := Build(ftnrtl.Files)
	if err != nil {
		t.Fatal(err)
	}

	committed, ok := ftnrtl.Image("ftnrtl")
	if !ok {
		t.Fatal("no built-in FTNRTL")
	}

	if !bytes.Equal(built, committed) {
		t.Error("ftnrtl.exe isn't what its sources build; run go generate ./internal/ftnrtl")
	}
}

// TestImage checks the image's global symbol table: its name, its
// match control, and that each entry is a universal symbol at its
// transfer vector offset, which must never change.
func TestImage(t *testing.T) {
	data, _ := ftnrtl.Image(ftnrtl.Name)

	src, name, err := link.ReadShareableImage(data)
	if err != nil || name != ftnrtl.Name {
		t.Fatalf("ReadShareableImage: %q, %v", name, err)
	}

	image, ok := src.Image(ftnrtl.Name)
	if !ok || image.Match != link.MatchLEQ || image.MajorID != 1 || image.MinorID != 30 {
		t.Errorf("image %+v", image)
	}

	for symbol, offset := range map[string]uint32{
		"FTN$VERSION": 0, "FTN$STOP": 8, "FTN$ADJARRDIM": 656, "FTN$RAB": 664, "FTN$DIVCH": 672, "FTN$NOTCONFOR": 680, "FTN$ALLOCATE": 688, "FTN$DEALLOCATE": 696, "FTN$GET_TEMP": 704, "FTN$BADARRDESC": 712, "FTN$ALLOC_SCALAR": 720, "FTN$DEALLOC_SCALAR": 728, "FTN$BMVBITS": 736, "FTN$ADJUSTL": 744, "FTN$REPEAT": 760, "FTN$INDEX_BACK": 784, "FTN$SCALE_F": 856, "FTN$SET_EXPONENT_H": 912, "FTN$NEAREST_H": 1008, "FTN$DATE_AND_TIME": 1016, "FTN$SYSTEM_CLOCK_W": 1040, "FTN$RANDOM_F": 1048, "FTN$RANDOM_H": 1072, "FTN$RANDOM_SEED_W": 1088,
		"FTN$REWIND": 416, "FTN$BACKSPACE": 424, "FTN$ENDFILE": 432,
		"FTN$READ_SU": 440, "FTN$WRITE_SU": 448,
		"FTN$READ_DF": 456, "FTN$READ_DU": 464, "FTN$WRITE_DF": 472, "FTN$WRITE_DU": 480,
		"FTN$FIND": 488, "FTN$DEF_FILE": 496, "FTN$DEF_FILE_W": 504,
		"FTN$DELETE": 512, "FTN$DELETE_D": 520, "FTN$UNLOCK": 528,
		"FTN$REWRITE_SF": 536, "FTN$REWRITE_SU": 544, "FTN$READ_KF": 552, "FTN$READ_KU": 560,
		"FTN$PRINT_LIST": 16, "FTN$WRITE_LIST": 24, "FTN$IO_ITEM": 32, "FTN$IO_END": 40,
		"FTN$IISHFTC": 48, "FTN$JISHFTC": 56,
		"FTN$LEN": 64, "FTN$INDEX": 72, "FTN$ICHAR": 80, "FTN$CHAR": 88,
		"FTN$LLT": 96, "FTN$LLE": 104, "FTN$LGT": 112, "FTN$LGE": 120,
		"FTN$BITEST": 128, "FTN$BJTEST": 136, "FTN$IIBITS": 144, "FTN$JIBITS": 152,
		"FTN$IIBSET": 160, "FTN$JIBSET": 168, "FTN$IIBCLR": 176, "FTN$JIBCLR": 184,
		"FTN$IZEXT": 192, "FTN$JZEXT": 200, "FTN$SNGLQ": 208, "FTN$DBLEQ": 216,
		"FTN$GBLEQ": 224, "FTN$QEXT": 232, "FTN$QEXTD": 240, "FTN$QEXTG": 248,
		"FTN$QFLOAT": 256, "FTN$NWORKERS": 264, "FTN$PAUSE": 272,
		"FTN$READ_SF": 280, "FTN$WRITE_SF": 288, "FTN$READ_SL": 296, "FTN$WRITE_SL": 304,
		"FTN$READ_SN": 312, "FTN$WRITE_SN": 320, "FTN$READ_IF": 328, "FTN$WRITE_IF": 336,
		"FTN$READ_IL": 344, "FTN$WRITE_IL": 352, "FTN$DECODE": 360, "FTN$ENCODE": 368,
		"FTN$ERRSNS": 376, "FTN$ERRSNS_W": 384, "FTN$OPEN": 392, "FTN$CLOSE": 400,
		"FTN$DATE": 568, "FTN$DATE_T_DS": 576, "FTN$TIME": 584, "FTN$TIME_T_DS": 592,
		"FTN$IDATE": 600, "FTN$JDATE": 608, "FTN$SECNDS": 616, "FTN$EXIT": 624,
		"FTN$EXIT_W": 632, "FTN$IMVBITS": 640, "FTN$JMVBITS": 648,
		"FTN$CHABS": 792, "FTN$CHSQRT": 800, "FTN$CHEXP": 808, "FTN$CHLOG": 816,
		"FTN$CHSIN": 824, "FTN$CHCOS": 832, "FTN$CHPOWJ": 840, "FTN$CHPOW": 848,
	} {
		d, ok, _ := src.Lookup(symbol)
		if !ok || d.Image != ftnrtl.Name || d.Value != offset {
			t.Errorf("%s: %+v, want offset %d in FTNRTL", symbol, d, offset)
		}
	}

	// Every routine the compiler calls or passes for an intrinsic function
	// is an entry.
	for _, symbol := range sema.FTNRoutines {
		if d, ok, _ := src.Lookup(symbol); !ok || d.Image != ftnrtl.Name {
			t.Errorf("%s, which the compiler uses, isn't FTNRTL's: %+v", symbol, d)
		}
	}

	if _, ok := ftnrtl.Image("LIBRTL"); ok {
		t.Error("Image answered for LIBRTL")
	}
}

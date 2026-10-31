package mthrtl

import (
	"fmt"
	"regexp"
	"slices"
	"sync"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// Library is the name of the shareable image these routines stand for, as
// a program's image names it and SHIM$<library>_<offset> spells it.
const Library = "MTHRTL"

// codeBase is the XFC$SHIM code of the transfer vector's first entry.
// LIBRTL's codes (its missing JSB entries start at 2000) and the traceback
// handler's (3000) are below it, and the stubs the image activator makes
// for other libraries' routines are far above it (machine's
// unresolvedCodeBase).
const codeBase = 4000

// entrySize is the size of one transfer vector entry, in bytes. A CALL
// entry is a 2-byte entry mask and a 6-byte JMP, a JSB entry a JMP and two
// bytes of padding.
const entrySize = 8

// Routine is one routine reached by CALLS or CALLG. Its code isn't written
// here: it follows from the routine's place in the transfer vector
// (codeOf), so the tables can't disagree with the vector, and a routine
// listed under either of two names that share an entry (MTH$IINT is
// MTH$IIFIX) is the same routine.
type Routine struct {
	// Name is the routine's universal symbol, which must be one of
	// MTHRTL.EXE's.
	Name string

	// Fn is the routine.
	Fn corevms.ShimFunc
}

// JSBRoutine is one routine reached by JSB, its arguments in registers.
type JSBRoutine struct {
	Name string
	Fn   corevms.JSBShimFunc
}

// Routines are the routines reached by CALL that govax provides. The
// transfer vector's other CALL entries signal NOTIMPL.
var Routines = []Routine{
	{"MTH$SQRT", callEntry(vaxfloat.F, squareRoot)},
	{"MTH$DSQRT", callEntry(vaxfloat.D, squareRoot)},
	{"MTH$GSQRT", callEntry(vaxfloat.G, squareRoot)},
	{"MTH$HSQRT", callEntry(vaxfloat.H, squareRoot)},

	{"MTH$EXP", callEntry(vaxfloat.F, exp)},
	{"MTH$DEXP", callEntry(vaxfloat.D, exp)},
	{"MTH$GEXP", callEntry(vaxfloat.G, exp)},
	{"MTH$HEXP", callEntry(vaxfloat.H, exp)},

	{"MTH$ALOG", callEntry(vaxfloat.F, naturalLog)},
	{"MTH$DLOG", callEntry(vaxfloat.D, naturalLog)},
	{"MTH$GLOG", callEntry(vaxfloat.G, naturalLog)},
	{"MTH$HLOG", callEntry(vaxfloat.H, naturalLog)},

	{"MTH$ALOG2", callEntry(vaxfloat.F, base2Log)},
	{"MTH$DLOG2", callEntry(vaxfloat.D, base2Log)},
	{"MTH$GLOG2", callEntry(vaxfloat.G, base2Log)},
	{"MTH$HLOG2", callEntry(vaxfloat.H, base2Log)},

	{"MTH$ALOG10", callEntry(vaxfloat.F, base10Log)},
	{"MTH$DLOG10", callEntry(vaxfloat.D, base10Log)},
	{"MTH$GLOG10", callEntry(vaxfloat.G, base10Log)},
	{"MTH$HLOG10", callEntry(vaxfloat.H, base10Log)},

	{"MTH$SIN", callEntry(vaxfloat.F, sine)},
	{"MTH$DSIN", callEntry(vaxfloat.D, sine)},
	{"MTH$GSIN", callEntry(vaxfloat.G, sine)},
	{"MTH$HSIN", callEntry(vaxfloat.H, sine)},

	{"MTH$COS", callEntry(vaxfloat.F, cosine)},
	{"MTH$DCOS", callEntry(vaxfloat.D, cosine)},
	{"MTH$GCOS", callEntry(vaxfloat.G, cosine)},
	{"MTH$HCOS", callEntry(vaxfloat.H, cosine)},

	{"MTH$TAN", callEntry(vaxfloat.F, tangent)},
	{"MTH$DTAN", callEntry(vaxfloat.D, tangent)},
	{"MTH$GTAN", callEntry(vaxfloat.G, tangent)},
	{"MTH$HTAN", callEntry(vaxfloat.H, tangent)},

	{"MTH$SINCOS", callPairEntry(vaxfloat.F, sineCosine)},
	{"MTH$DSINCOS", callPairEntry(vaxfloat.D, sineCosine)},
	{"MTH$GSINCOS", callPairEntry(vaxfloat.G, sineCosine)},
	{"MTH$HSINCOS", callPairEntry(vaxfloat.H, sineCosine)},

	{"MTH$SIND", callEntry(vaxfloat.F, sineDegrees)},
	{"MTH$DSIND", callEntry(vaxfloat.D, sineDegrees)},
	{"MTH$GSIND", callEntry(vaxfloat.G, sineDegrees)},
	{"MTH$HSIND", callEntry(vaxfloat.H, sineDegrees)},

	{"MTH$COSD", callEntry(vaxfloat.F, cosineDegrees)},
	{"MTH$DCOSD", callEntry(vaxfloat.D, cosineDegrees)},
	{"MTH$GCOSD", callEntry(vaxfloat.G, cosineDegrees)},
	{"MTH$HCOSD", callEntry(vaxfloat.H, cosineDegrees)},

	{"MTH$TAND", callEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$DTAND", callEntry(vaxfloat.D, tangentDegrees)},
	{"MTH$GTAND", callEntry(vaxfloat.G, tangentDegrees)},
	{"MTH$HTAND", callEntry(vaxfloat.H, tangentDegrees)},

	{"MTH$SINCOSD", callPairEntry(vaxfloat.F, sineCosineDegrees)},
	{"MTH$DSINCOSD", callPairEntry(vaxfloat.D, sineCosineDegrees)},
	{"MTH$GSINCOSD", callPairEntry(vaxfloat.G, sineCosineDegrees)},
	{"MTH$HSINCOSD", callPairEntry(vaxfloat.H, sineCosineDegrees)},

	{"MTH$ASIN", callEntry(vaxfloat.F, arcSine)},
	{"MTH$DASIN", callEntry(vaxfloat.D, arcSine)},
	{"MTH$GASIN", callEntry(vaxfloat.G, arcSine)},
	{"MTH$HASIN", callEntry(vaxfloat.H, arcSine)},

	{"MTH$ACOS", callEntry(vaxfloat.F, arcCosine)},
	{"MTH$DACOS", callEntry(vaxfloat.D, arcCosine)},
	{"MTH$GACOS", callEntry(vaxfloat.G, arcCosine)},
	{"MTH$HACOS", callEntry(vaxfloat.H, arcCosine)},

	{"MTH$ATAN", callEntry(vaxfloat.F, arcTangent)},
	{"MTH$DATAN", callEntry(vaxfloat.D, arcTangent)},
	{"MTH$GATAN", callEntry(vaxfloat.G, arcTangent)},
	{"MTH$HATAN", callEntry(vaxfloat.H, arcTangent)},

	{"MTH$ASIND", callEntry(vaxfloat.F, arcSineDegrees)},
	{"MTH$DASIND", callEntry(vaxfloat.D, arcSineDegrees)},
	{"MTH$GASIND", callEntry(vaxfloat.G, arcSineDegrees)},
	{"MTH$HASIND", callEntry(vaxfloat.H, arcSineDegrees)},

	{"MTH$ACOSD", callEntry(vaxfloat.F, arcCosineDegrees)},
	{"MTH$DACOSD", callEntry(vaxfloat.D, arcCosineDegrees)},
	{"MTH$GACOSD", callEntry(vaxfloat.G, arcCosineDegrees)},
	{"MTH$HACOSD", callEntry(vaxfloat.H, arcCosineDegrees)},

	{"MTH$ATAND", callEntry(vaxfloat.F, arcTangentDegrees)},
	{"MTH$DATAND", callEntry(vaxfloat.D, arcTangentDegrees)},
	{"MTH$GATAND", callEntry(vaxfloat.G, arcTangentDegrees)},
	{"MTH$HATAND", callEntry(vaxfloat.H, arcTangentDegrees)},

	{"MTH$ATAN2", callBinaryEntry(vaxfloat.F, arcTangent2)},
	{"MTH$DATAN2", callBinaryEntry(vaxfloat.D, arcTangent2)},
	{"MTH$GATAN2", callBinaryEntry(vaxfloat.G, arcTangent2)},
	{"MTH$HATAN2", callBinaryEntry(vaxfloat.H, arcTangent2)},

	{"MTH$ATAND2", callBinaryEntry(vaxfloat.F, arcTangent2Degrees)},
	{"MTH$DATAND2", callBinaryEntry(vaxfloat.D, arcTangent2Degrees)},
	{"MTH$GATAND2", callBinaryEntry(vaxfloat.G, arcTangent2Degrees)},
	{"MTH$HATAND2", callBinaryEntry(vaxfloat.H, arcTangent2Degrees)},

	{"MTH$SINH", callEntry(vaxfloat.F, hyperbolicSine)},
	{"MTH$DSINH", callEntry(vaxfloat.D, hyperbolicSine)},
	{"MTH$GSINH", callEntry(vaxfloat.G, hyperbolicSine)},
	{"MTH$HSINH", callEntry(vaxfloat.H, hyperbolicSine)},

	{"MTH$COSH", callEntry(vaxfloat.F, hyperbolicCosine)},
	{"MTH$DCOSH", callEntry(vaxfloat.D, hyperbolicCosine)},
	{"MTH$GCOSH", callEntry(vaxfloat.G, hyperbolicCosine)},
	{"MTH$HCOSH", callEntry(vaxfloat.H, hyperbolicCosine)},

	{"MTH$TANH", callEntry(vaxfloat.F, hyperbolicTangent)},
	{"MTH$DTANH", callEntry(vaxfloat.D, hyperbolicTangent)},
	{"MTH$GTANH", callEntry(vaxfloat.G, hyperbolicTangent)},
	{"MTH$HTANH", callEntry(vaxfloat.H, hyperbolicTangent)},

	{"MTH$ATANH", callEntry(vaxfloat.F, arcHyperbolicTangent)},
	{"MTH$DATANH", callEntry(vaxfloat.D, arcHyperbolicTangent)},
	{"MTH$GATANH", callEntry(vaxfloat.G, arcHyperbolicTangent)},
	{"MTH$HATANH", callEntry(vaxfloat.H, arcHyperbolicTangent)},

	{"MTH$ABS", callEntry(vaxfloat.F, absolute)},
	{"MTH$DABS", callEntry(vaxfloat.D, absolute)},
	{"MTH$GABS", callEntry(vaxfloat.G, absolute)},
	{"MTH$HABS", callEntry(vaxfloat.H, absolute)},
	{"MTH$IIABS", integerEntry(word, 1, integerAbsolute)},
	{"MTH$JIABS", integerEntry(long, 1, integerAbsolute)},

	{"MTH$DIM", callBinaryEntry(vaxfloat.F, positiveDifference)},
	{"MTH$DDIM", callBinaryEntry(vaxfloat.D, positiveDifference)},
	{"MTH$GDIM", callBinaryEntry(vaxfloat.G, positiveDifference)},
	{"MTH$HDIM", callBinaryEntry(vaxfloat.H, positiveDifference)},
	{"MTH$IIDIM", integerEntry(word, 2, integerDifference)},
	{"MTH$JIDIM", integerEntry(long, 2, integerDifference)},

	{"MTH$SIGN", callBinaryEntry(vaxfloat.F, transferSign)},
	{"MTH$DSIGN", callBinaryEntry(vaxfloat.D, transferSign)},
	{"MTH$GSIGN", callBinaryEntry(vaxfloat.G, transferSign)},
	{"MTH$HSIGN", callBinaryEntry(vaxfloat.H, transferSign)},
	{"MTH$IISIGN", integerEntry(word, 2, integerSign)},
	{"MTH$JISIGN", integerEntry(long, 2, integerSign)},
	{"MTH$SGN", wholeEntry(vaxfloat.F, long, signOf)},

	{"MTH$FLOOR", callEntry(vaxfloat.F, wholeNumber(floorOf))},
	{"MTH$DFLOOR", callEntry(vaxfloat.D, wholeNumber(floorOf))},
	{"MTH$GFLOOR", callEntry(vaxfloat.G, wholeNumber(floorOf))},
	{"MTH$HFLOOR", callEntry(vaxfloat.H, wholeNumber(floorOf))},

	{"MTH$AINT", callEntry(vaxfloat.F, wholeNumber(truncated))},
	{"MTH$DINT", callEntry(vaxfloat.D, wholeNumber(truncated))},
	{"MTH$GINT", callEntry(vaxfloat.G, wholeNumber(truncated))},
	{"MTH$HINT", callEntry(vaxfloat.H, wholeNumber(truncated))},
	{"MTH$IIFIX", wholeEntry(vaxfloat.F, word, truncated)}, // MTH$IINT too
	{"MTH$JIFIX", wholeEntry(vaxfloat.F, long, truncated)}, // MTH$JINT too
	{"MTH$IIDINT", wholeEntry(vaxfloat.D, word, truncated)},
	{"MTH$JIDINT", wholeEntry(vaxfloat.D, long, truncated)},
	{"MTH$IIGINT", wholeEntry(vaxfloat.G, word, truncated)},
	{"MTH$JIGINT", wholeEntry(vaxfloat.G, long, truncated)},
	{"MTH$IIHINT", wholeEntry(vaxfloat.H, word, truncated)},
	{"MTH$JIHINT", wholeEntry(vaxfloat.H, long, truncated)},

	{"MTH$ANINT", callEntry(vaxfloat.F, wholeNumber(nearest))},
	{"MTH$DNINT", callEntry(vaxfloat.D, wholeNumber(nearest))},
	{"MTH$GNINT", callEntry(vaxfloat.G, wholeNumber(nearest))},
	{"MTH$HNINT", callEntry(vaxfloat.H, wholeNumber(nearest))},
	{"MTH$ININT", wholeEntry(vaxfloat.F, word, nearest)},
	{"MTH$JNINT", wholeEntry(vaxfloat.F, long, nearest)},
	{"MTH$IIDNNT", wholeEntry(vaxfloat.D, word, nearest)},
	{"MTH$JIDNNT", wholeEntry(vaxfloat.D, long, nearest)},
	{"MTH$IIGNNT", wholeEntry(vaxfloat.G, word, nearest)},
	{"MTH$JIGNNT", wholeEntry(vaxfloat.G, long, nearest)},
	{"MTH$IIHNNT", wholeEntry(vaxfloat.H, word, nearest)},
	{"MTH$JIHNNT", wholeEntry(vaxfloat.H, long, nearest)},

	{"MTH$AMOD", callBinaryEntry(vaxfloat.F, remainder)},
	{"MTH$DMOD", callBinaryEntry(vaxfloat.D, remainder)},
	{"MTH$GMOD", callBinaryEntry(vaxfloat.G, remainder)},
	{"MTH$HMOD", callBinaryEntry(vaxfloat.H, remainder)},
	{"MTH$IMOD", integerRemainderEntry(word)},
	{"MTH$JMOD", integerRemainderEntry(long)},

	{"MTH$AIMAX0", extremeFloatEntry(integerArgs(word), vaxfloat.F, wantLargest)},
	{"MTH$AJMAX0", extremeFloatEntry(integerArgs(long), vaxfloat.F, wantLargest)},
	{"MTH$IMAX0", extremeIntegerEntry(integerArgs(word), word, wantLargest)},
	{"MTH$JMAX0", extremeIntegerEntry(integerArgs(long), long, wantLargest)},
	{"MTH$AMAX1", extremeFloatEntry(floatArgs(vaxfloat.F), vaxfloat.F, wantLargest)},
	{"MTH$DMAX1", extremeFloatEntry(floatArgs(vaxfloat.D), vaxfloat.D, wantLargest)},
	{"MTH$GMAX1", extremeFloatEntry(floatArgs(vaxfloat.G), vaxfloat.G, wantLargest)},
	{"MTH$HMAX1", extremeFloatEntry(floatArgs(vaxfloat.H), vaxfloat.H, wantLargest)},
	{"MTH$IMAX1", extremeIntegerEntry(floatArgs(vaxfloat.F), word, wantLargest)},
	{"MTH$JMAX1", extremeIntegerEntry(floatArgs(vaxfloat.F), long, wantLargest)},
	{"MTH$UMAX", extremeIntegerEntry(integerArgs(unsignedLong), unsignedLong, wantLargest)},

	{"MTH$AIMIN0", extremeFloatEntry(integerArgs(word), vaxfloat.F, wantSmallest)},
	{"MTH$AJMIN0", extremeFloatEntry(integerArgs(long), vaxfloat.F, wantSmallest)},
	{"MTH$IMIN0", extremeIntegerEntry(integerArgs(word), word, wantSmallest)},
	{"MTH$JMIN0", extremeIntegerEntry(integerArgs(long), long, wantSmallest)},
	{"MTH$AMIN1", extremeFloatEntry(floatArgs(vaxfloat.F), vaxfloat.F, wantSmallest)},
	{"MTH$DMIN1", extremeFloatEntry(floatArgs(vaxfloat.D), vaxfloat.D, wantSmallest)},
	{"MTH$GMIN1", extremeFloatEntry(floatArgs(vaxfloat.G), vaxfloat.G, wantSmallest)},
	{"MTH$HMIN1", extremeFloatEntry(floatArgs(vaxfloat.H), vaxfloat.H, wantSmallest)},
	{"MTH$IMIN1", extremeIntegerEntry(floatArgs(vaxfloat.F), word, wantSmallest)},
	{"MTH$JMIN1", extremeIntegerEntry(floatArgs(vaxfloat.F), long, wantSmallest)},
	{"MTH$UMIN", extremeIntegerEntry(integerArgs(unsignedLong), unsignedLong, wantSmallest)},

	{"MTH$DBLE", conversionEntry(floatArgs(vaxfloat.F), vaxfloat.D, convert)},
	{"MTH$GDBLE", conversionEntry(floatArgs(vaxfloat.F), vaxfloat.G, convert)},
	{"MTH$SNGL", conversionEntry(floatArgs(vaxfloat.D), vaxfloat.F, convertTrapping)},
	{"MTH$SNGLG", conversionEntry(floatArgs(vaxfloat.G), vaxfloat.F, convertTrapping)},
	{"MTH$FLOATI", conversionEntry(integerArgs(word), vaxfloat.F, convert)},
	{"MTH$DFLOTI", conversionEntry(integerArgs(word), vaxfloat.D, convert)},
	{"MTH$GFLOTI", conversionEntry(integerArgs(word), vaxfloat.G, convert)},
	{"MTH$FLOATJ", conversionEntry(integerArgs(long), vaxfloat.F, convert)},
	{"MTH$DFLOTJ", conversionEntry(integerArgs(long), vaxfloat.D, convert)},
	{"MTH$GFLOTJ", conversionEntry(integerArgs(long), vaxfloat.G, convert)},
	{"MTH$DPROD", productEntry(vaxfloat.D)},
	{"MTH$GPROD", productEntry(vaxfloat.G)},
	{"MTH$CVT_D_G", conversionEntry(floatArgs(vaxfloat.D), vaxfloat.G, convert)},
	{"MTH$CVT_G_D", conversionEntry(floatArgs(vaxfloat.G), vaxfloat.D, convert)},
	{"MTH$CVT_DA_GA", convertArrayEntry(vaxfloat.D, vaxfloat.G)},
	{"MTH$CVT_GA_DA", convertArrayEntry(vaxfloat.G, vaxfloat.D)},

	{"MTH$IIAND", integerEntry(word, 2, integerAnd)},
	{"MTH$JIAND", integerEntry(long, 2, integerAnd)},
	{"MTH$IIOR", integerEntry(word, 2, integerOr)},
	{"MTH$JIOR", integerEntry(long, 2, integerOr)},
	{"MTH$IIEOR", integerEntry(word, 2, integerExclusiveOr)},
	{"MTH$JIEOR", integerEntry(long, 2, integerExclusiveOr)},
	{"MTH$INOT", integerEntry(word, 1, integerNot)},
	{"MTH$JNOT", integerEntry(long, 1, integerNot)},
	{"MTH$IISHFT", integerEntry(word, 2, integerShift)},
	{"MTH$JISHFT", integerEntry(long, 2, integerShift)},

	{"MTH$RANDOM", random},

	{"MTH$CABS", modulusEntry(vaxfloat.F)},
	{"MTH$CDABS", modulusEntry(vaxfloat.D)},
	{"MTH$CGABS", modulusEntry(vaxfloat.G)},

	{"MTH$CEXP", complexEntry(vaxfloat.F, complexAt, complexExponential)},
	{"MTH$CDEXP", complexEntry(vaxfloat.D, complexAt, complexExponential)},
	{"MTH$CGEXP", complexEntry(vaxfloat.G, complexAt, complexExponential)},

	{"MTH$CLOG", complexEntry(vaxfloat.F, complexAt, complexLog)},
	{"MTH$CDLOG", complexEntry(vaxfloat.D, complexAt, complexLog)},
	{"MTH$CGLOG", complexEntry(vaxfloat.G, complexAt, complexLog)},

	{"MTH$CSIN", complexEntry(vaxfloat.F, complexAt, complexSine)},
	{"MTH$CDSIN", complexEntry(vaxfloat.D, complexAt, complexSine)},
	{"MTH$CGSIN", complexEntry(vaxfloat.G, complexAt, complexSine)},

	{"MTH$CCOS", complexEntry(vaxfloat.F, complexAt, complexCosine)},
	{"MTH$CDCOS", complexEntry(vaxfloat.D, complexAt, complexCosine)},
	{"MTH$CGCOS", complexEntry(vaxfloat.G, complexAt, complexCosine)},

	{"MTH$CSQRT", complexEntry(vaxfloat.F, complexAt, complexSquareRoot)},
	{"MTH$CDSQRT", complexEntry(vaxfloat.D, complexAt, complexSquareRoot)},
	{"MTH$CGSQRT", complexEntry(vaxfloat.G, complexAt, complexSquareRoot)},

	{"MTH$CMPLX", complexEntry(vaxfloat.F, complexFromParts, asComplex)},
	{"MTH$DCMPLX", complexEntry(vaxfloat.D, complexFromParts, asComplex)},
	{"MTH$GCMPLX", complexEntry(vaxfloat.G, complexFromParts, asComplex)},

	{"MTH$CONJG", complexEntry(vaxfloat.F, complexAt, conjugate)},
	{"MTH$DCONJG", complexEntry(vaxfloat.D, complexAt, conjugate)},
	{"MTH$GCONJG", complexEntry(vaxfloat.G, complexAt, conjugate)},

	{"MTH$REAL", callEntry(vaxfloat.F, convert)},
	{"MTH$DREAL", callEntry(vaxfloat.D, convert)},
	{"MTH$GREAL", callEntry(vaxfloat.G, convert)},

	{"MTH$AIMAG", conversionEntry(imaginaryArgs(vaxfloat.F), vaxfloat.F, convert)},
	{"MTH$DIMAG", conversionEntry(imaginaryArgs(vaxfloat.D), vaxfloat.D, convert)},
	{"MTH$GIMAG", conversionEntry(imaginaryArgs(vaxfloat.G), vaxfloat.G, convert)},

	{"OTS$POWRR", powerEntry(vaxfloat.F, vaxfloat.F, vaxfloat.F)},
	{"OTS$POWRD", powerEntry(vaxfloat.F, vaxfloat.D, vaxfloat.D)},
	{"OTS$POWDR", powerEntry(vaxfloat.D, vaxfloat.F, vaxfloat.D)},
	{"OTS$POWDD", powerEntry(vaxfloat.D, vaxfloat.D, vaxfloat.D)},
	{"OTS$POWGG", powerEntry(vaxfloat.G, vaxfloat.G, vaxfloat.G)},
	{"OTS$POWHH_R3", powerEntry(vaxfloat.H, vaxfloat.H, vaxfloat.H)},

	{"OTS$POWRJ", integerPowerEntry(vaxfloat.F, long)},
	{"OTS$POWDJ", integerPowerEntry(vaxfloat.D, long)},
	{"OTS$POWGJ", integerPowerEntry(vaxfloat.G, long)},
	{"OTS$POWHJ_R3", integerPowerEntry(vaxfloat.H, long)},

	{"OTS$POWRLU", integerPowerEntry(vaxfloat.F, unsignedLong)},
	{"OTS$POWDLU", integerPowerEntry(vaxfloat.D, unsignedLong)},
	{"OTS$POWGLU", integerPowerEntry(vaxfloat.G, unsignedLong)},
	{"OTS$POWHLU_R3", integerPowerEntry(vaxfloat.H, unsignedLong)},

	{"OTS$POWII", integerPowerOfEntry(word)},
	{"OTS$POWJJ", integerPowerOfEntry(long)},
	{"OTS$POWLULU", integerPowerOfEntry(unsignedLong)},

	{"OTS$POWCC", complexOperationEntry(vaxfloat.F, complexPower)},
	{"OTS$POWCDCD_R3", complexOperationEntry(vaxfloat.D, complexPower)},
	{"OTS$POWCGCG_R3", complexOperationEntry(vaxfloat.G, complexPower)},

	{"OTS$POWCJ", complexIntegerPowerEntry(vaxfloat.F)},
	{"OTS$POWCDJ_R3", complexIntegerPowerEntry(vaxfloat.D)},
	{"OTS$POWCGJ_R3", complexIntegerPowerEntry(vaxfloat.G)},

	{"OTS$DIVC", complexOperationEntry(vaxfloat.F, complexQuotient)},
	{"OTS$DIVCD_R3", complexOperationEntry(vaxfloat.D, complexQuotient)},
	{"OTS$DIVCG_R3", complexOperationEntry(vaxfloat.G, complexQuotient)},

	{"OTS$MULCD_R3", complexOperationEntry(vaxfloat.D, complexProduct)},
	{"OTS$MULCG_R3", complexOperationEntry(vaxfloat.G, complexProduct)},
}

// JSBRoutines are the routines reached by JSB that govax provides. The
// vector's other JSB entries signal NOTIMPL, among them MTH$SQRT_R2,
// MTH$HTAN_R7, MTH$HTAND_R7, the hyperbolic functions' (MTH$SINH_R5, ...), and
// the two-argument MTH$ATAN2_R4's and MTH$AMOD_R5's kind, the sign's (MTH$SIGN_R1,
// MTH$SGN_R1, ...), and the complex functions' (MTH$CABS_R4, MTH$CDSIN_R9, ...), of
// which section 1.5 says there are none. No manual gives these, and their registers
// wait for the VMS probe (Decision 7). A name that has a neighbor with a smaller
// register number, such as MTH$DEXP_R7 beside MTH$DEXP_R6 or MTH$ASIN_R5 beside
// MTH$ASIN_R4, is a second entry: in VMS's image the two jump to the same
// address.
var JSBRoutines = []JSBRoutine{
	{"MTH$SQRT_R3", jsbEntry(vaxfloat.F, squareRoot)},
	{"MTH$DSQRT_R5", jsbEntry(vaxfloat.D, squareRoot)},
	{"MTH$GSQRT_R5", jsbEntry(vaxfloat.G, squareRoot)},
	{"MTH$HSQRT_R8", jsbEntry(vaxfloat.H, squareRoot)},

	{"MTH$EXP_R4", jsbEntry(vaxfloat.F, exp)},
	{"MTH$DEXP_R6", jsbEntry(vaxfloat.D, exp)},
	{"MTH$DEXP_R7", jsbEntry(vaxfloat.D, exp)},
	{"MTH$GEXP_R6", jsbEntry(vaxfloat.G, exp)},
	{"MTH$GEXP_R7", jsbEntry(vaxfloat.G, exp)},
	{"MTH$HEXP_R6", jsbEntry(vaxfloat.H, exp)},

	{"MTH$ALOG_R5", jsbEntry(vaxfloat.F, naturalLog)},
	{"MTH$DLOG_R8", jsbEntry(vaxfloat.D, naturalLog)},
	{"MTH$GLOG_R8", jsbEntry(vaxfloat.G, naturalLog)},
	{"MTH$HLOG_R8", jsbEntry(vaxfloat.H, naturalLog)},

	{"MTH$ALOG10_R5", jsbEntry(vaxfloat.F, base10Log)},
	{"MTH$DLOG10_R8", jsbEntry(vaxfloat.D, base10Log)},
	{"MTH$GLOG10_R8", jsbEntry(vaxfloat.G, base10Log)},
	{"MTH$HLOG10_R8", jsbEntry(vaxfloat.H, base10Log)},

	{"MTH$SIN_R4", jsbEntry(vaxfloat.F, sine)},
	{"MTH$DSIN_R7", jsbEntry(vaxfloat.D, sine)},
	{"MTH$GSIN_R7", jsbEntry(vaxfloat.G, sine)},
	{"MTH$HSIN_R5", jsbEntry(vaxfloat.H, sine)},

	{"MTH$COS_R4", jsbEntry(vaxfloat.F, cosine)},
	{"MTH$DCOS_R7", jsbEntry(vaxfloat.D, cosine)},
	{"MTH$GCOS_R7", jsbEntry(vaxfloat.G, cosine)},
	{"MTH$HCOS_R5", jsbEntry(vaxfloat.H, cosine)},

	{"MTH$TAN_R4", jsbEntry(vaxfloat.F, tangent)},
	{"MTH$TAN_R5", jsbEntry(vaxfloat.F, tangent)},
	{"MTH$DTAN_R7", jsbEntry(vaxfloat.D, tangent)},
	{"MTH$GTAN_R7", jsbEntry(vaxfloat.G, tangent)},
	{"MTH$HTAN_R5", jsbEntry(vaxfloat.H, tangent)},

	{"MTH$SINCOS_R5", jsbPairEntry(vaxfloat.F, sineCosine)},
	{"MTH$DSINCOS_R7", jsbPairEntry(vaxfloat.D, sineCosine)},
	{"MTH$GSINCOS_R7", jsbPairEntry(vaxfloat.G, sineCosine)},
	{"MTH$HSINCOS_R7", jsbPairEntry(vaxfloat.H, sineCosine)},

	{"MTH$SIND_R4", jsbEntry(vaxfloat.F, sineDegrees)},
	{"MTH$DSIND_R7", jsbEntry(vaxfloat.D, sineDegrees)},
	{"MTH$GSIND_R7", jsbEntry(vaxfloat.G, sineDegrees)},
	{"MTH$HSIND_R5", jsbEntry(vaxfloat.H, sineDegrees)},

	{"MTH$COSD_R4", jsbEntry(vaxfloat.F, cosineDegrees)},
	{"MTH$DCOSD_R7", jsbEntry(vaxfloat.D, cosineDegrees)},
	{"MTH$GCOSD_R7", jsbEntry(vaxfloat.G, cosineDegrees)},
	{"MTH$HCOSD_R5", jsbEntry(vaxfloat.H, cosineDegrees)},

	{"MTH$TAND_R4", jsbEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$TAND_R5", jsbEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$DTAND_R7", jsbEntry(vaxfloat.D, tangentDegrees)},
	{"MTH$GTAND_R7", jsbEntry(vaxfloat.G, tangentDegrees)},
	{"MTH$HTAND_R5", jsbEntry(vaxfloat.H, tangentDegrees)},

	{"MTH$SINCOSD_R5", jsbPairEntry(vaxfloat.F, sineCosineDegrees)},
	{"MTH$DSINCOSD_R7", jsbPairEntry(vaxfloat.D, sineCosineDegrees)},
	{"MTH$GSINCOSD_R7", jsbPairEntry(vaxfloat.G, sineCosineDegrees)},
	{"MTH$HSINCOSD_R7", jsbPairEntry(vaxfloat.H, sineCosineDegrees)},

	{"MTH$ASIN_R4", jsbEntry(vaxfloat.F, arcSine)},
	{"MTH$ASIN_R5", jsbEntry(vaxfloat.F, arcSine)},
	{"MTH$DASIN_R7", jsbEntry(vaxfloat.D, arcSine)},
	{"MTH$DASIN_R9", jsbEntry(vaxfloat.D, arcSine)},
	{"MTH$GASIN_R7", jsbEntry(vaxfloat.G, arcSine)},
	{"MTH$GASIN_R9", jsbEntry(vaxfloat.G, arcSine)},
	{"MTH$HASIN_R8", jsbEntry(vaxfloat.H, arcSine)},

	{"MTH$ACOS_R4", jsbEntry(vaxfloat.F, arcCosine)},
	{"MTH$ACOS_R5", jsbEntry(vaxfloat.F, arcCosine)},
	{"MTH$DACOS_R7", jsbEntry(vaxfloat.D, arcCosine)},
	{"MTH$DACOS_R9", jsbEntry(vaxfloat.D, arcCosine)},
	{"MTH$GACOS_R7", jsbEntry(vaxfloat.G, arcCosine)},
	{"MTH$GACOS_R9", jsbEntry(vaxfloat.G, arcCosine)},
	{"MTH$HACOS_R8", jsbEntry(vaxfloat.H, arcCosine)},

	{"MTH$ATAN_R4", jsbEntry(vaxfloat.F, arcTangent)},
	{"MTH$DATAN_R7", jsbEntry(vaxfloat.D, arcTangent)},
	{"MTH$GATAN_R7", jsbEntry(vaxfloat.G, arcTangent)},
	{"MTH$HATAN_R8", jsbEntry(vaxfloat.H, arcTangent)},

	{"MTH$ASIND_R4", jsbEntry(vaxfloat.F, arcSineDegrees)},
	{"MTH$DASIND_R7", jsbEntry(vaxfloat.D, arcSineDegrees)},
	{"MTH$GASIND_R7", jsbEntry(vaxfloat.G, arcSineDegrees)},
	{"MTH$HASIND_R8", jsbEntry(vaxfloat.H, arcSineDegrees)},

	{"MTH$ACOSD_R4", jsbEntry(vaxfloat.F, arcCosineDegrees)},
	{"MTH$DACOSD_R7", jsbEntry(vaxfloat.D, arcCosineDegrees)},
	{"MTH$GACOSD_R7", jsbEntry(vaxfloat.G, arcCosineDegrees)},
	{"MTH$HACOSD_R8", jsbEntry(vaxfloat.H, arcCosineDegrees)},

	{"MTH$ATAND_R4", jsbEntry(vaxfloat.F, arcTangentDegrees)},
	{"MTH$DATAND_R7", jsbEntry(vaxfloat.D, arcTangentDegrees)},
	{"MTH$GATAND_R7", jsbEntry(vaxfloat.G, arcTangentDegrees)},
	{"MTH$HATAND_R8", jsbEntry(vaxfloat.H, arcTangentDegrees)},

	{"MTH$FLOOR_R1", jsbEntry(vaxfloat.F, wholeNumber(floorOf))},
	{"MTH$DFLOOR_R3", jsbEntry(vaxfloat.D, wholeNumber(floorOf))},
	{"MTH$GFLOOR_R3", jsbEntry(vaxfloat.G, wholeNumber(floorOf))},
	{"MTH$HFLOOR_R7", jsbEntry(vaxfloat.H, wholeNumber(floorOf))},

	// The entries no manual gives, whose registers are VMS 7.1's (testdata/mth77):
	// arguments in R0 up, one after the other, and the result from R0.
	{"MTH$SQRT_R2", jsbEntry(vaxfloat.F, squareRoot)},
	{"MTH$HTAN_R7", jsbEntry(vaxfloat.H, tangent)},
	{"MTH$HTAND_R7", jsbEntry(vaxfloat.H, tangentDegrees)},

	{"MTH$SINH_R5", jsbEntry(vaxfloat.F, hyperbolicSine)},
	{"MTH$DSINH_R8", jsbEntry(vaxfloat.D, hyperbolicSine)},
	{"MTH$GSINH_R8", jsbEntry(vaxfloat.G, hyperbolicSine)},
	{"MTH$COSH_R4", jsbEntry(vaxfloat.F, hyperbolicCosine)},
	{"MTH$DCOSH_R7", jsbEntry(vaxfloat.D, hyperbolicCosine)},
	{"MTH$GCOSH_R7", jsbEntry(vaxfloat.G, hyperbolicCosine)},
	{"MTH$TANH_R5", jsbEntry(vaxfloat.F, hyperbolicTangent)},
	{"MTH$DTANH_R9", jsbEntry(vaxfloat.D, hyperbolicTangent)},
	{"MTH$GTANH_R9", jsbEntry(vaxfloat.G, hyperbolicTangent)},

	{"MTH$ATAN2_R4", jsbBinaryEntry(vaxfloat.F, arcTangent2)},
	{"MTH$DATAN2_R7", jsbBinaryEntry(vaxfloat.D, arcTangent2)},
	{"MTH$GATAN2_R7", jsbBinaryEntry(vaxfloat.G, arcTangent2)},
	{"MTH$ATAND2_R4", jsbBinaryEntry(vaxfloat.F, arcTangent2Degrees)},
	{"MTH$DATAND2_R7", jsbBinaryEntry(vaxfloat.D, arcTangent2Degrees)},
	{"MTH$GATAND2_R7", jsbBinaryEntry(vaxfloat.G, arcTangent2Degrees)},

	{"MTH$AMOD_R5", jsbBinaryEntry(vaxfloat.F, remainder)},
	{"MTH$DMOD_R9", jsbBinaryEntry(vaxfloat.D, remainder)},
	{"MTH$GMOD_R9", jsbBinaryEntry(vaxfloat.G, remainder)},
	{"MTH$SIGN_R1", jsbBinaryEntry(vaxfloat.F, transferSign)},
	{"MTH$DSIGN_R3", jsbBinaryEntry(vaxfloat.D, transferSign)},
	{"MTH$GSIGN_R3", jsbBinaryEntry(vaxfloat.G, transferSign)},
	{"MTH$SGN_R1", jsbSignEntry},

	{"MTH$CABS_R4", jsbModulusEntry(vaxfloat.F)},
	{"MTH$CDABS_R6", jsbModulusEntry(vaxfloat.D)},
	{"MTH$CGABS_R6", jsbModulusEntry(vaxfloat.G)},
	{"MTH$CEXP_R6", jsbComplexEntry(vaxfloat.F, complexExponential)},
	{"MTH$CDEXP_R9", jsbComplexEntry(vaxfloat.D, complexExponential)},
	{"MTH$CGEXP_R9", jsbComplexEntry(vaxfloat.G, complexExponential)},
	{"MTH$CLOG_R6", jsbComplexEntry(vaxfloat.F, complexLog)},
	{"MTH$CDLOG_R9", jsbComplexEntry(vaxfloat.D, complexLog)},
	{"MTH$CGLOG_R9", jsbComplexEntry(vaxfloat.G, complexLog)},
	{"MTH$CSIN_R8", jsbComplexEntry(vaxfloat.F, complexSine)},
	{"MTH$CDSIN_R9", jsbComplexEntry(vaxfloat.D, complexSine)},
	{"MTH$CGSIN_R9", jsbComplexEntry(vaxfloat.G, complexSine)},
	{"MTH$CCOS_R8", jsbComplexEntry(vaxfloat.F, complexCosine)},
	{"MTH$CDCOS_R9", jsbComplexEntry(vaxfloat.D, complexCosine)},
	{"MTH$CGCOS_R9", jsbComplexEntry(vaxfloat.G, complexCosine)},
	{"MTH$CSQRT_R7", jsbComplexEntry(vaxfloat.F, complexSquareRoot)},
	{"MTH$CDSQRT_R9", jsbComplexEntry(vaxfloat.D, complexSquareRoot)},
	{"MTH$CGSQRT_R9", jsbComplexEntry(vaxfloat.G, complexSquareRoot)},

	{"MTH$AINT_R2", jsbEntry(vaxfloat.F, wholeNumber(truncated))},
	{"MTH$DINT_R4", jsbEntry(vaxfloat.D, wholeNumber(truncated))},
	{"MTH$GINT_R4", jsbEntry(vaxfloat.G, wholeNumber(truncated))},
	{"MTH$HINT_R8", jsbEntry(vaxfloat.H, wholeNumber(truncated))},
}

// jsbName matches the names of the JSB entries: a name like MTH$SIN_R4 is
// reached by JSB. OTS$POWHH_R3 is not: it's a CALL entry whose result comes
// back in R0:R3 (the survey read each entry's first bytes, and this rule
// holds for all 404 names).
var jsbName = regexp.MustCompile(`^MTH\$.*_R[0-9]+$`)

// entry is one entry of the transfer vector.
type entry struct {
	name   string // the first name in alphabetical order, if it has two
	offset uint32
	jsb    bool
}

// entries are the transfer vector's entries, in offset order. It's a
// function made once, on first use (sync.OnceValue), since a new System
// asks for it each time and the answer can't change.
var entries = sync.OnceValue(func() []entry {
	byOffset := map[uint32]string{}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library {
			continue
		}

		if other, ok := byOffset[sym.Value]; !ok || name < other {
			byOffset[sym.Value] = name
		}
	}

	list := make([]entry, 0, len(byOffset))
	for offset, name := range byOffset {
		list = append(list, entry{name: name, offset: offset, jsb: jsbName.MatchString(name)})
	}

	slices.SortFunc(list, func(a, b entry) int { return int(a.offset) - int(b.offset) })

	return list
})

// codeOf returns the XFC$SHIM code of name's stub. It panics for a name
// MTHRTL.EXE doesn't have, a mistake in the tables that TestTables reports
// by name.
func codeOf(name string) uint32 {
	sym, ok := vmsdef.ImageSymbols[name]
	if !ok || sym.Image != Library {
		panic(fmt.Sprintf("mthrtl: %s is not in %s's transfer vector", name, Library))
	}

	return codeBase + sym.Value/entrySize
}

// Register installs every entry's routine in t: the tables' routines, and
// for each other entry one that signals NOTIMPL. The console does this for
// each new System.
func Register(t *corevms.ShimTable) {
	for _, e := range entries() {
		code := codeBase + e.offset/entrySize

		if e.jsb {
			t.RegisterJSB(code, e.name, librtl.NotImplementedJSB(e.name))
		} else {
			t.Register(code, e.name, librtl.NotImplemented(e.name))
		}
	}

	for _, r := range Routines {
		t.Register(codeOf(r.Name), r.Name, r.Fn)
	}

	for _, r := range JSBRoutines {
		t.RegisterJSB(codeOf(r.Name), r.Name, r.Fn)
	}
}

// Links are the transfer vector's entries as corevms.ShimLinks, for their
// stubs: one for each of the 402 entries, whatever the table holds, in
// offset order, so a stub's address doesn't change when its routine is
// written. Two names that share an entry have one stub, under the name
// that sorts first.
func Links() []corevms.ShimLink {
	list := entries()
	links := make([]corevms.ShimLink, len(list))

	for i, e := range list {
		links[i] = corevms.ShimLink{
			Name: e.name, Library: Library, Offset: e.offset,
			Code: codeBase + e.offset/entrySize, JSB: e.jsb,
		}
	}

	return links
}

// arg returns argument i of argv, or 0 when the call passed fewer. The
// MTH$ routines' arguments are addresses, and 0 is one no program can
// reach, so a missing argument is an access violation, as on VMS.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}

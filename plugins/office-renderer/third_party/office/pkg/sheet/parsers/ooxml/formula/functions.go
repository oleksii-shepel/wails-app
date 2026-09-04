package formula

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// evalFunction evaluates a function call.
func evalFunction(name string, args []Expression, ctx Context) Value {
	switch name {
	case "SUM":
		return fnSum(args, ctx)
	case "PRODUCT":
		return fnProduct(args, ctx)
	case "SUMSQ":
		return fnSumSq(args, ctx)
	case "MEDIAN":
		return fnMedian(args, ctx)
	case "LARGE":
		return fnLarge(args, ctx)
	case "RANK", "RANK.EQ":
		return fnRank(args, ctx)
	case "SMALL":
		return fnSmall(args, ctx)
	case "COUNTBLANK":
		return fnCountBlank(args, ctx)
	case "MODE":
		return fnMode(args, ctx)
	case "AVERAGEIF":
		return fnAverageIf(args, ctx)
	case "AVERAGEIFS":
		return fnAverageIfs(args, ctx)
	case "COUNTIFS":
		return fnCountIfs(args, ctx)
	case "SUMIF":
		return fnSumIf(args, ctx)
	case "SUMIFS":
		return fnSumIfs(args, ctx)
	case "DSUM":
		return fnDSum(args, ctx)
	case "DCOUNT":
		return fnDCount(args, ctx)
	case "DCOUNTA":
		return fnDCountA(args, ctx)
	case "DAVERAGE":
		return fnDAverage(args, ctx)
	case "DGET":
		return fnDGet(args, ctx)
	case "DMAX":
		return fnDMax(args, ctx)
	case "DMIN":
		return fnDMin(args, ctx)
	case "DPRODUCT":
		return fnDProduct(args, ctx)
	case "DSTDEV":
		return fnDStdev(args, ctx)
	case "DSTDEVP":
		return fnDStdevp(args, ctx)
	case "DVAR":
		return fnDVar(args, ctx)
	case "DVARP":
		return fnDVarp(args, ctx)
	case "SUMPRODUCT":
		return fnSumProduct(args, ctx)
	case "AVERAGE":
		return fnAverage(args, ctx)
	case "COUNT":
		return fnCount(args, ctx)
	case "COUNTA":
		return fnCountA(args, ctx)
	case "MAX":
		return fnMax(args, ctx)
	case "MIN":
		return fnMin(args, ctx)
	case "COUNTIF":
		return fnCountIf(args, ctx)
	case "IF":
		return fnIf(args, ctx)
	case "IFS":
		return fnIfs(args, ctx)
	case "IFERROR":
		return fnIfError(args, ctx)
	case "IFNA":
		return fnIfNA(args, ctx)
	case "SWITCH":
		return fnSwitch(args, ctx)
	case "CHOOSE":
		return fnChoose(args, ctx)
	case "CHOOSECOLS":
		return fnChooseCols(args, ctx)
	case "CHOOSEROWS":
		return fnChooseRows(args, ctx)
	case "UNIQUE":
		return fnUnique(args, ctx)
	case "SORT":
		return fnSort(args, ctx)
	case "SORTBY":
		return fnSortBy(args, ctx)
	case "SEQUENCE":
		return fnSequence(args, ctx)
	case "RANDARRAY":
		return fnRandArray(args, ctx)
	case "EXPAND":
		return fnExpand(args, ctx)
	case "MAKEARRAY":
		return fnMakeArray(args, ctx)
	case "BYROW":
		return fnByRow(args, ctx)
	case "BYCOL":
		return fnByCol(args, ctx)
	case "MAP":
		return fnMap(args, ctx)
	case "REDUCE":
		return fnReduce(args, ctx)
	case "SCAN":
		return fnScan(args, ctx)
	case "LET":
		return fnLet(args, ctx)
	case "MATCH":
		return fnMatch(args, ctx)
	case "LOOKUP":
		return fnLookup(args, ctx)
	case "XMATCH":
		return fnXMatch(args, ctx)
	case "XLOOKUP":
		return fnXLookup(args, ctx)
	case "FILTER":
		return fnFilter(args, ctx)
	case "HSTACK":
		return fnHStack(args, ctx)
	case "VSTACK":
		return fnVStack(args, ctx)
	case "TAKE":
		return fnTake(args, ctx)
	case "DROP":
		return fnDrop(args, ctx)
	case "TOROW":
		return fnToRow(args, ctx)
	case "TOCOL":
		return fnToCol(args, ctx)
	case "WRAPROWS":
		return fnWrapRows(args, ctx)
	case "WRAPCOLS":
		return fnWrapCols(args, ctx)
	case "INDEX":
		return fnIndex(args, ctx)
	case "OFFSET":
		return fnOffset(args, ctx)
	case "TRANSPOSE":
		return fnTranspose(args, ctx)
	case "ABS":
		return fnAbs(args, ctx)
	case "ROUND":
		return fnRound(args, ctx)
	case "ROUNDUP":
		return fnRoundUp(args, ctx)
	case "ROUNDDOWN":
		return fnRoundDown(args, ctx)
	case "INT":
		return fnInt(args, ctx)
	case "MOD":
		return fnMod(args, ctx)
	case "CEILING":
		return fnCeiling(args, ctx)
	case "FLOOR":
		return fnFloor(args, ctx)
	case "POWER":
		return fnPower(args, ctx)
	case "SQRT":
		return fnSqrt(args, ctx)
	case "TRUNC":
		return fnTrunc(args, ctx)
	case "SIGN":
		return fnSign(args, ctx)
	case "LOG":
		return fnLog(args, ctx)
	case "LOG10":
		return fnLog10(args, ctx)
	case "LN":
		return fnLn(args, ctx)
	case "EXP":
		return fnExp(args, ctx)
	case "RANDBETWEEN":
		return fnRandBetween(args, ctx)
	case "XOR":
		return fnXor(args, ctx)
	case "ROWS":
		return fnRows(args, ctx)
	case "COLUMNS":
		return fnColumns(args, ctx)
	case "ADDRESS":
		return fnAddress(args, ctx)
	case "INDIRECT":
		return fnIndirect(args, ctx)
	case "LEN":
		return fnLen(args, ctx)
	case "LEFT":
		return fnLeft(args, ctx)
	case "RIGHT":
		return fnRight(args, ctx)
	case "MID":
		return fnMid(args, ctx)
	case "TRIM":
		return fnTrim(args, ctx)
	case "UPPER":
		return fnUpper(args, ctx)
	case "LOWER":
		return fnLower(args, ctx)
	case "PROPER":
		return fnProper(args, ctx)
	case "REPT":
		return fnRept(args, ctx)
	case "EXACT":
		return fnExact(args, ctx)
	case "CHAR":
		return fnChar(args, ctx)
	case "CODE":
		return fnCode(args, ctx)
	case "FIND":
		return fnFind(args, ctx)
	case "SEARCH":
		return fnSearch(args, ctx)
	case "SUBSTITUTE":
		return fnSubstitute(args, ctx)
	case "REPLACE":
		return fnReplace(args, ctx)
	case "TEXTJOIN":
		return fnTextJoin(args, ctx)
	case "CONCATENATE", "CONCAT":
		return fnConcatenate(args, ctx)
	case "AND":
		return fnAnd(args, ctx)
	case "OR":
		return fnOr(args, ctx)
	case "NOT":
		return fnNot(args, ctx)
	case "ISNUMBER":
		return fnIsNumber(args, ctx)
	case "ISTEXT":
		return fnIsText(args, ctx)
	case "ISBLANK":
		return fnIsBlank(args, ctx)
	case "ISERR":
		return fnIsErr(args, ctx)
	case "ISERROR":
		return fnIsError(args, ctx)
	case "ISNA":
		return fnIsNA(args, ctx)
	case "ISLOGICAL":
		return fnIsLogical(args, ctx)
	case "ISNONTEXT":
		return fnIsNonText(args, ctx)
	case "ISREF":
		return fnIsRef(args, ctx)
	case "AREAS":
		return fnAreas(args, ctx)
	case "ISFORMULA":
		return fnIsFormula(args, ctx)
	case "ISEVEN":
		return fnIsEven(args, ctx)
	case "ISODD":
		return fnIsOdd(args, ctx)
	case "ERROR.TYPE":
		return fnErrorType(args, ctx)
	case "TYPE":
		return fnType(args, ctx)
	case "T":
		return fnT(args, ctx)
	case "N":
		return fnN(args, ctx)
	case "NA":
		return Error(ErrNA)
	case "VALUE":
		return fnValue(args, ctx)
	case "TEXT":
		return fnText(args, ctx)
	case "FORMULATEXT":
		return fnFormulaText(args, ctx)
	case "TODAY":
		return fnToday(args, ctx)
	case "NOW":
		return fnNow(args, ctx)
	case "SHEET":
		return fnSheet(args, ctx)
	case "SHEETS":
		return fnSheets(args, ctx)
	case "DATE":
		return fnDate(args, ctx)
	case "YEAR":
		return fnYear(args, ctx)
	case "MONTH":
		return fnMonth(args, ctx)
	case "DAY":
		return fnDay(args, ctx)
	case "HOUR":
		return fnHour(args, ctx)
	case "MINUTE":
		return fnMinute(args, ctx)
	case "SECOND":
		return fnSecond(args, ctx)
	case "WEEKDAY":
		return fnWeekday(args, ctx)
	case "DAYS":
		return fnDays(args, ctx)
	case "VLOOKUP":
		return fnVLookup(args, ctx)
	case "HLOOKUP":
		return fnHLookup(args, ctx)
	case "MINIFS":
		return fnMinIfs(args, ctx)
	case "MAXIFS":
		return fnMaxIfs(args, ctx)
	case "SIN":
		return fnSin(args, ctx)
	case "COS":
		return fnCos(args, ctx)
	case "TAN":
		return fnTan(args, ctx)
	case "ASIN":
		return fnAsin(args, ctx)
	case "ACOS":
		return fnAcos(args, ctx)
	case "ATAN":
		return fnAtan(args, ctx)
	case "ATAN2":
		return fnAtan2(args, ctx)
	case "DEGREES":
		return fnDegrees(args, ctx)
	case "RADIANS":
		return fnRadians(args, ctx)
	case "SINH":
		return fnSinh(args, ctx)
	case "COSH":
		return fnCosh(args, ctx)
	case "TANH":
		return fnTanh(args, ctx)
	case "ASINH":
		return fnAsinh(args, ctx)
	case "ACOSH":
		return fnAcosh(args, ctx)
	case "ATANH":
		return fnAtanh(args, ctx)
	case "STDEV", "STDEV.S":
		return fnStdev(args, ctx)
	case "STDEVP", "STDEV.P":
		return fnStdevp(args, ctx)
	case "VAR", "VAR.S":
		return fnVar(args, ctx)
	case "VARP", "VAR.P":
		return fnVarp(args, ctx)
	case "MAXA":
		return fnMaxA(args, ctx)
	case "MINA":
		return fnMinA(args, ctx)
	case "EDATE":
		return fnEdate(args, ctx)
	case "EOMONTH":
		return fnEomonth(args, ctx)
	case "WORKDAY":
		return fnWorkday(args, ctx)
	case "NETWORKDAYS":
		return fnNetworkdays(args, ctx)
	case "WEEKNUM":
		return fnWeeknum(args, ctx)
	case "ISOWEEKNUM":
		return fnIsoweeknum(args, ctx)
	case "TIME":
		return fnTime(args, ctx)
	case "DATEVALUE":
		return fnDatevalue(args, ctx)
	case "TIMEVALUE":
		return fnTimevalue(args, ctx)
	case "DATEDIF":
		return fnDatedif(args, ctx)
	case "GCD":
		return fnGcd(args, ctx)
	case "LCM":
		return fnLcm(args, ctx)
	case "FACT":
		return fnFact(args, ctx)
	case "COMBIN":
		return fnCombin(args, ctx)
	case "PERMUT":
		return fnPermut(args, ctx)
	case "MROUND":
		return fnMround(args, ctx)
	case "QUOTIENT":
		return fnQuotient(args, ctx)
	case "CEILING.MATH":
		return fnCeilingMath(args, ctx)
	case "FLOOR.MATH":
		return fnFloorMath(args, ctx)
	case "TEXTSPLIT":
		return fnTextsplit(args, ctx)
	case "TEXTBEFORE":
		return fnTextbefore(args, ctx)
	case "TEXTAFTER":
		return fnTextafter(args, ctx)
	case "NUMBERVALUE":
		return fnNumbervalue(args, ctx)
	case "CLEAN":
		return fnClean(args, ctx)
	case "FIXED":
		return fnFixed(args, ctx)
	case "DOLLAR":
		return fnDollar(args, ctx)
	case "PMT":
		return fnPmt(args, ctx)
	case "PV":
		return fnPv(args, ctx)
	case "FV":
		return fnFv(args, ctx)
	case "NPER":
		return fnNper(args, ctx)
	case "RATE":
		return fnRate(args, ctx)
	case "IPMT":
		return fnIpmt(args, ctx)
	case "PPMT":
		return fnPpmt(args, ctx)
	case "CUMIPMT":
		return fnCumIpmt(args, ctx)
	case "CUMPRINC":
		return fnCumPrinc(args, ctx)
	case "SLN":
		return fnSln(args, ctx)
	case "SYD":
		return fnSyd(args, ctx)
	case "DDB":
		return fnDdb(args, ctx)
	case "DB":
		return fnDb(args, ctx)
	case "IRR":
		return fnIrr(args, ctx)
	case "MIRR":
		return fnMirr(args, ctx)
	case "NPV":
		return fnNpv(args, ctx)
	case "EFFECT":
		return fnEffect(args, ctx)
	case "NOMINAL":
		return fnNominal(args, ctx)
	case "CORREL":
		return fnCorrel(args, ctx)
	case "COVAR", "COVARIANCE.P":
		return fnCovar(args, ctx)
	case "COVARIANCE.S":
		return fnCovarS(args, ctx)
	case "SLOPE":
		return fnSlope(args, ctx)
	case "INTERCEPT":
		return fnIntercept(args, ctx)
	case "FORECAST", "FORECAST.LINEAR":
		return fnForecast(args, ctx)
	case "PERCENTILE", "PERCENTILE.INC":
		return fnPercentile(args, ctx)
	case "PERCENTILE.EXC":
		return fnPercentileExc(args, ctx)
	case "QUARTILE", "QUARTILE.INC":
		return fnQuartile(args, ctx)
	case "QUARTILE.EXC":
		return fnQuartileExc(args, ctx)
	case "PERCENTRANK", "PERCENTRANK.INC":
		return fnPercentRank(args, ctx)
	case "NORMDIST", "NORM.DIST":
		return fnNormDist(args, ctx)
	case "NORMSDIST", "NORM.S.DIST":
		return fnNormSDist(args, ctx)
	case "NORMINV", "NORM.INV":
		return fnNormInv(args, ctx)
	case "NORMSINV", "NORM.S.INV":
		return fnNormSInv(args, ctx)
	case "CONFIDENCE", "CONFIDENCE.NORM":
		return fnConfidenceNorm(args, ctx)
	case "FACTDOUBLE":
		return fnFactDouble(args, ctx)
	case "COMBINA":
		return fnCombinA(args, ctx)
	case "MULTINOMIAL":
		return fnMultinomial(args, ctx)
	case "SUMX2MY2":
		return fnSumX2MY2(args, ctx)
	case "SUMX2PY2":
		return fnSumX2PY2(args, ctx)
	case "SUMXMY2":
		return fnSumXMY2(args, ctx)
	case "SUBTOTAL":
		return fnSubtotal(args, ctx)
	case "AGGREGATE":
		return fnAggregate(args, ctx)
	case "MMULT":
		return fnMmult(args, ctx)
	case "MUNIT":
		return fnMunit(args, ctx)
	case "MDETERM":
		return fnMdeterm(args, ctx)
	case "MINVERSE":
		return fnMinverse(args, ctx)
	case "XNPV":
		return fnXnpv(args, ctx)
	case "XIRR":
		return fnXirr(args, ctx)
	case "FVSCHEDULE":
		return fnFvSchedule(args, ctx)
	case "PDURATION":
		return fnPduration(args, ctx)
	case "RRI":
		return fnRri(args, ctx)
	case "ISPMT":
		return fnIspmt(args, ctx)
	case "DOLLARDE":
		return fnDollarDe(args, ctx)
	case "DOLLARFR":
		return fnDollarFr(args, ctx)
	case "ACCRINT":
		return fnAccrint(args, ctx)
	case "ACCRINTM":
		return fnAccrintM(args, ctx)
	case "PRICE":
		return fnPrice(args, ctx)
	case "PRICEDISC":
		return fnPriceDisc(args, ctx)
	case "PRICEMAT":
		return fnPriceMat(args, ctx)
	case "YIELD":
		return fnYield(args, ctx)
	case "YIELDDISC":
		return fnYieldDisc(args, ctx)
	case "YIELDMAT":
		return fnYieldMat(args, ctx)
	case "DISC":
		return fnDisc(args, ctx)
	case "INTRATE":
		return fnIntRate(args, ctx)
	case "RECEIVED":
		return fnReceived(args, ctx)
	case "TBILLEQ":
		return fnTbillEq(args, ctx)
	case "TBILLPRICE":
		return fnTbillPrice(args, ctx)
	case "TBILLYIELD":
		return fnTbillYield(args, ctx)
	case "COUPDAYBS":
		return fnCoupDayBs(args, ctx)
	case "COUPDAYS":
		return fnCoupDays(args, ctx)
	case "COUPDAYSNC":
		return fnCoupDaysNc(args, ctx)
	case "COUPNCD":
		return fnCoupNcd(args, ctx)
	case "COUPPCD":
		return fnCoupPcd(args, ctx)
	case "ODDFPRICE":
		return fnOddFPrice(args, ctx)
	case "ODDFYIELD":
		return fnOddFYield(args, ctx)
	case "ODDLPRICE":
		return fnOddLPrice(args, ctx)
	case "ODDLYIELD":
		return fnOddLYield(args, ctx)
	case "HYPERLINK":
		return fnHyperlink(args, ctx)
	case "GETPIVOTDATA":
		return fnGetPivotData(args, ctx)
	case "BESSELI":
		return fnBesselI(args, ctx)
	case "BESSELJ":
		return fnBesselJ(args, ctx)
	case "BESSELK":
		return fnBesselK(args, ctx)
	case "BESSELY":
		return fnBesselY(args, ctx)
	case "BITAND":
		return fnBitAnd(args, ctx)
	case "BITOR":
		return fnBitOr(args, ctx)
	case "BITXOR":
		return fnBitXor(args, ctx)
	case "BITLSHIFT":
		return fnBitLShift(args, ctx)
	case "BITRSHIFT":
		return fnBitRShift(args, ctx)
	case "COMPLEX":
		return fnComplex(args, ctx)
	case "CONVERT":
		return fnConvert(args, ctx)
	case "IMABS":
		return fnImAbs(args, ctx)
	case "IMAGINARY":
		return fnImaginary(args, ctx)
	case "IMARGUMENT":
		return fnImArgument(args, ctx)
	case "IMCONJUGATE":
		return fnImConjugate(args, ctx)
	case "IMDIV":
		return fnImDiv(args, ctx)
	case "IMEXP":
		return fnImExp(args, ctx)
	case "IMLN":
		return fnImLn(args, ctx)
	case "IMLOG10":
		return fnImLog10(args, ctx)
	case "IMLOG2":
		return fnImLog2(args, ctx)
	case "IMPOWER":
		return fnImPower(args, ctx)
	case "IMPRODUCT":
		return fnImProduct(args, ctx)
	case "IMREAL":
		return fnImReal(args, ctx)
	case "IMSUB":
		return fnImSub(args, ctx)
	case "IMSUM":
		return fnImSum(args, ctx)
	case "BETA.DIST", "BETADIST":
		return fnBetaDist(args, ctx)
	case "BETA.INV", "BETAINV":
		return fnBetaInv(args, ctx)
	case "BINOM.DIST", "BINOMDIST":
		return fnBinomDist(args, ctx)
	case "BINOM.INV", "CRITBINOM":
		return fnBinomInv(args, ctx)
	case "CHISQ.DIST", "CHIDIST":
		return fnChisqDist(args, ctx)
	case "CHISQ.INV", "CHIINV":
		return fnChisqInv(args, ctx)
	case "F.DIST", "FDIST":
		return fnFDist(args, ctx)
	case "F.INV", "FINV":
		return fnFInv(args, ctx)
	case "GAMMA.DIST", "GAMMADIST":
		return fnGammaDist(args, ctx)
	case "GAMMA.INV", "GAMMAINV":
		return fnGammaInv(args, ctx)
	case "HYPGEOM.DIST", "HYPGEOMDIST":
		return fnHypgeomDist(args, ctx)
	case "LOGNORM.DIST", "LOGNORMDIST":
		return fnLognormDist(args, ctx)
	case "LOGNORM.INV", "LOGINV":
		return fnLognormInv(args, ctx)
	case "NEGBINOM.DIST", "NEGBINOMDIST":
		return fnNegbinomDist(args, ctx)
	case "POISSON.DIST", "POISSON":
		return fnPoissonDist(args, ctx)
	case "T.DIST", "TDIST":
		return fnTDist(args, ctx)
	case "T.INV", "TINV":
		return fnTInv(args, ctx)
	case "WEIBULL.DIST", "WEIBULL":
		return fnWeibullDist(args, ctx)
	// Batch 7: Cube And OLAP Functions — return #REF! until OLAP data sources are supported
	case "CUBEMEMBER":
		return Error(ErrRef)
	case "CUBEVALUE":
		return Error(ErrRef)
	case "CUBERANKEDMEMBER":
		return Error(ErrRef)
	case "CUBEKPIMEMBER":
		return Error(ErrRef)
	case "CUBEMEMBERPROPERTY":
		return Error(ErrRef)
	// Batch 8: Macro And External-Call Functions — return #VALUE! until macro support is available
	case "CALL":
		return Error(ErrValue)
	case "REGISTER":
		return Error(ErrValue)
	case "REGISTER.ID":
		return Error(ErrValue)
	case "ROW":
		return fnRow(args, ctx)
	case "COLUMN":
		return fnColumn(args, ctx)
	case "TRUE":
		return Bool(true)
	case "FALSE":
		return Bool(false)
	case "PI":
		return Number(math.Pi)
	default:
		if resolver, ok := ctx.(lambdaBindingResolver); ok {
			if lambda, ok := resolver.ResolveLambdaBinding(name); ok {
				return invokeLambdaDefinition(lambda, args, ctx)
			}
		}
		return Error(ErrName)
	}
}

func expandArgumentValues(arg Expression, ctx Context) []Value {
	switch expr := arg.(type) {
	case *CellRefExpr:
		if resolver, ok := ctx.(bindingResolver); ok {
			if values, _, ok := resolver.ResolveArrayBinding(expr.Ref); ok {
				return values
			}
		}
		return []Value{arg.Eval(ctx)}
	case *RangeExpr:
		return ctx.GetRange(expr.Start, expr.End)
	case *UnaryExpr:
		return evalUnaryArgumentValues(expr, ctx)
	case *BinaryExpr:
		return evalBinaryArgumentValues(expr, ctx)
	case *FuncCallExpr:
		if strings.EqualFold(expr.Name, "IF") {
			return evalIfArgumentValues(expr.Args, ctx)
		}
		if strings.EqualFold(expr.Name, "INDIRECT") {
			start, end, ok, errValue := indirectTargetRange(expr.Args, ctx)
			if !ok {
				return []Value{errValue}
			}
			if start == end {
				return []Value{ctx.GetCell(start)}
			}
			return ctx.GetRange(start, end)
		}
		if shape, ok := inferExpressionArrayShape(expr); ok && (shape.rows > 1 || shape.cols > 1) {
			return evalArrayExpressionValues(expr, shape, ctx)
		}
		if shape, ok := inferExpressionArrayShapeWithContext(expr, ctx); ok && (shape.rows > 1 || shape.cols > 1) {
			return evalArrayExpressionValues(expr, shape, ctx)
		}
		return []Value{arg.Eval(ctx)}
	default:
		return []Value{arg.Eval(ctx)}
	}
}

type arrayExpansionContext struct {
	base           Context
	currentRef     string
	arrayAnchorRef string
}

type lambdaBinding struct {
	values []Value
	shape  arrayShape
	lambda *lambdaClosure
}

type lambdaBindingResolver interface {
	ResolveLambdaBinding(ref string) (lambdaClosure, bool)
}

type lambdaClosure struct {
	def lambdaDefinition
	ctx Context
}

// PivotDataCriterion is one GETPIVOTDATA field/item filter pair.
type PivotDataCriterion struct {
	Field string
	Item  Value
}

type pivotDataResolver interface {
	ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool)
}

type lambdaContext struct {
	base           Context
	currentRef     string
	arrayAnchorRef string
	bindings       map[string]lambdaBinding
}

func (c *arrayExpansionContext) GetCell(ref string) Value {
	return c.base.GetCell(ref)
}

func (c *arrayExpansionContext) GetRange(start, end string) []Value {
	return c.base.GetRange(start, end)
}

func (c *arrayExpansionContext) CurrentRef() string {
	return c.currentRef
}

func (c *arrayExpansionContext) ArrayAnchorRef() string {
	return c.arrayAnchorRef
}

func (c *arrayExpansionContext) GetFormula(ref string) (string, bool) {
	return c.base.GetFormula(ref)
}

func (c *arrayExpansionContext) ResolveSheetOrdinal(token string) (int, bool) {
	return c.base.ResolveSheetOrdinal(token)
}

func (c *arrayExpansionContext) SheetCount() int {
	return c.base.SheetCount()
}

func (c *arrayExpansionContext) GetSpillRange(ref string) (string, string, bool) {
	return c.base.GetSpillRange(ref)
}

func (c *arrayExpansionContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	if resolver, ok := c.base.(pivotDataResolver); ok {
		return resolver.ResolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
	}
	return Value{}, false
}

func (c *lambdaContext) GetCell(ref string) Value {
	return c.base.GetCell(ref)
}

func (c *lambdaContext) GetRange(start, end string) []Value {
	return c.base.GetRange(start, end)
}

func (c *lambdaContext) CurrentRef() string {
	if c.currentRef != "" {
		return c.currentRef
	}
	return c.base.CurrentRef()
}

func (c *lambdaContext) ArrayAnchorRef() string {
	if c.arrayAnchorRef != "" {
		return c.arrayAnchorRef
	}
	return c.base.ArrayAnchorRef()
}

func (c *lambdaContext) GetFormula(ref string) (string, bool) {
	return c.base.GetFormula(ref)
}

func (c *lambdaContext) ResolveSheetOrdinal(token string) (int, bool) {
	return c.base.ResolveSheetOrdinal(token)
}

func (c *lambdaContext) SheetCount() int {
	return c.base.SheetCount()
}

func (c *lambdaContext) GetSpillRange(ref string) (string, string, bool) {
	return c.base.GetSpillRange(ref)
}

func (c *lambdaContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	if resolver, ok := c.base.(pivotDataResolver); ok {
		return resolver.ResolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
	}
	return Value{}, false
}

func (c *lambdaContext) ResolveScalarBinding(ref string) (Value, bool) {
	binding, ok := c.lookupBinding(ref)
	if ok {
		if len(binding.values) == 0 {
			return Value{}, false
		}
		if len(binding.values) == 1 {
			return binding.values[0], true
		}
		if value, ok := bindingValueAt(binding, c.CurrentRef(), c.ArrayAnchorRef()); ok {
			return value, true
		}
		return binding.values[0], true
	}
	if resolver, ok := c.base.(bindingResolver); ok {
		return resolver.ResolveScalarBinding(ref)
	}
	return Value{}, false
}

func (c *lambdaContext) ResolveArrayBinding(ref string) ([]Value, arrayShape, bool) {
	binding, ok := c.lookupBinding(ref)
	if ok {
		return binding.values, binding.shape, true
	}
	if resolver, ok := c.base.(bindingResolver); ok {
		return resolver.ResolveArrayBinding(ref)
	}
	return nil, arrayShape{}, false
}

func (c *lambdaContext) ResolveLambdaBinding(ref string) (lambdaClosure, bool) {
	binding, ok := c.lookupBinding(ref)
	if ok && binding.lambda != nil {
		return *binding.lambda, true
	}
	if resolver, ok := c.base.(lambdaBindingResolver); ok {
		return resolver.ResolveLambdaBinding(ref)
	}
	return lambdaClosure{}, false
}

func (c *lambdaContext) lookupBinding(ref string) (lambdaBinding, bool) {
	binding, ok := c.bindings[strings.ToUpper(ref)]
	return binding, ok
}

func evalArrayExpressionValues(expr Expression, shape arrayShape, ctx Context) []Value {
	anchorRef := ctx.CurrentRef()
	if anchorRef == "" {
		anchorRef = "A1"
	}
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if anchorRow == 0 || anchorCol == 0 {
		anchorRow, anchorCol = 1, 1
		anchorRef = "A1"
	}
	values := make([]Value, 0, shape.rows*shape.cols)
	for rowOffset := 0; rowOffset < shape.rows; rowOffset++ {
		for colOffset := 0; colOffset < shape.cols; colOffset++ {
			currentRef := fmt.Sprintf("%s%d", colToLetters(anchorCol+colOffset), anchorRow+rowOffset)
			evalCtx := &arrayExpansionContext{base: ctx, currentRef: currentRef, arrayAnchorRef: anchorRef}
			values = append(values, expr.Eval(evalCtx))
		}
	}
	return values
}

func normalizeArgumentValues(values []Value) []Value {
	if len(values) == 0 {
		return []Value{Empty()}
	}
	return values
}

func bindingValueAt(binding lambdaBinding, currentRef, anchorRef string) (Value, bool) {
	if len(binding.values) == 0 {
		return Empty(), true
	}
	if len(binding.values) == 1 {
		return binding.values[0], true
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Value{}, false
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || colOffset < 0 || rowOffset >= binding.shape.rows || colOffset >= binding.shape.cols {
		return Value{}, false
	}
	index := rowOffset*binding.shape.cols + colOffset
	if index < 0 || index >= len(binding.values) {
		return Value{}, false
	}
	return binding.values[index], true
}

func scalarLambdaBinding(value Value) lambdaBinding {
	return lambdaBinding{values: []Value{value}, shape: arrayShape{rows: 1, cols: 1}}
}

func normalizedLambdaBinding(values []Value, shape arrayShape) lambdaBinding {
	values = normalizeArgumentValues(values)
	if shape.rows <= 0 || shape.cols <= 0 {
		shape = arrayShape{rows: 1, cols: len(values)}
		if len(values) == 1 {
			shape = arrayShape{rows: 1, cols: 1}
		}
	}
	return lambdaBinding{values: values, shape: shape}
}

type lambdaDefinition struct {
	params []string
	body   Expression
}

func parseLambda(expr Expression, minParams, maxParams int) (lambdaDefinition, bool, Value) {
	call, ok := expr.(*FuncCallExpr)
	if !ok || !strings.EqualFold(call.Name, "LAMBDA") || len(call.Args) == 0 {
		return lambdaDefinition{}, false, Error(ErrValue)
	}
	paramCount := len(call.Args) - 1
	if paramCount < minParams || (maxParams >= 0 && paramCount > maxParams) {
		return lambdaDefinition{}, false, Error(ErrValue)
	}
	params := make([]string, 0, paramCount)
	for _, arg := range call.Args[:paramCount] {
		ref, ok := arg.(*CellRefExpr)
		if !ok || ref.Ref == "" {
			return lambdaDefinition{}, false, Error(ErrValue)
		}
		params = append(params, strings.ToUpper(ref.Ref))
	}
	return lambdaDefinition{params: params, body: call.Args[len(call.Args)-1]}, true, Value{}
}

func parseBindingName(expr Expression) (string, bool) {
	ref, ok := expr.(*CellRefExpr)
	if !ok || ref.Ref == "" {
		return "", false
	}
	return strings.ToUpper(ref.Ref), true
}

func resolveLambdaDefinition(expr Expression, ctx Context) (lambdaClosure, bool) {
	if lambda, ok, _ := parseLambda(expr, 0, -1); ok {
		return lambdaClosure{def: lambda, ctx: ctx}, true
	}
	switch expr := expr.(type) {
	case *CellRefExpr:
		if expr.Ref == "" {
			return lambdaClosure{}, false
		}
		if resolver, ok := ctx.(lambdaBindingResolver); ok {
			return resolver.ResolveLambdaBinding(expr.Ref)
		}
	case *ApplyExpr:
		lambdaCtx, body, ok, _ := lambdaInvocationContext(expr.Callee, expr.Args, ctx)
		if !ok {
			return lambdaClosure{}, false
		}
		return resolveLambdaDefinition(body, lambdaCtx)
	case *FuncCallExpr:
		if strings.EqualFold(expr.Name, "LET") {
			letCtx, finalExpr, ok, _ := letContextForArgs(expr.Args, ctx)
			if !ok {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(finalExpr, letCtx)
		}
		if strings.EqualFold(expr.Name, "IF") {
			if len(expr.Args) < 2 {
				return lambdaClosure{}, false
			}
			condition := expr.Args[0].Eval(ctx)
			if condition.IsError() {
				return lambdaClosure{}, false
			}
			boolVal, err := condition.ToBool()
			if err != nil {
				return lambdaClosure{}, false
			}
			if boolVal {
				return resolveLambdaDefinition(expr.Args[1], ctx)
			}
			if len(expr.Args) >= 3 {
				return resolveLambdaDefinition(expr.Args[2], ctx)
			}
			return lambdaClosure{}, false
		}
		if strings.EqualFold(expr.Name, "CHOOSE") {
			if len(expr.Args) < 2 {
				return lambdaClosure{}, false
			}
			selector, ok := inferStaticIndexSelector(expr.Args[0])
			if !ok {
				selectorValue := expr.Args[0].Eval(ctx)
				if selectorValue.IsError() {
					return lambdaClosure{}, false
				}
				num, err := selectorValue.ToNumber()
				if err != nil {
					return lambdaClosure{}, false
				}
				selector = int(num)
			}
			if selector < 1 || selector >= len(expr.Args) {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(expr.Args[selector], ctx)
		}
		if strings.EqualFold(expr.Name, "IFS") {
			selected, _, ok := selectIfsBranch(expr.Args, ctx)
			if !ok {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(selected, ctx)
		}
		if strings.EqualFold(expr.Name, "SWITCH") {
			selected, _, ok := selectSwitchBranch(expr.Args, ctx)
			if !ok {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(selected, ctx)
		}
		if strings.EqualFold(expr.Name, "IFERROR") {
			selected, _, ok := selectIfErrorBranch(expr.Args, ctx)
			if !ok {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(selected, ctx)
		}
		if strings.EqualFold(expr.Name, "IFNA") {
			selected, _, ok := selectIfNaBranch(expr.Args, ctx)
			if !ok {
				return lambdaClosure{}, false
			}
			return resolveLambdaDefinition(selected, ctx)
		}
	}
	return lambdaClosure{}, false
}

func letContextForArgs(args []Expression, ctx Context) (*lambdaContext, Expression, bool, Value) {
	if len(args) < 3 || len(args)%2 == 0 {
		return nil, nil, false, Error(ErrValue)
	}
	bindings := make(map[string]lambdaBinding, len(args)/2)
	letCtx := &lambdaContext{base: ctx, currentRef: ctx.CurrentRef(), arrayAnchorRef: ctx.ArrayAnchorRef(), bindings: bindings}
	for index := 0; index < len(args)-1; index += 2 {
		name, ok := parseBindingName(args[index])
		if !ok {
			return nil, nil, false, Error(ErrValue)
		}
		if lambda, ok := resolveLambdaDefinition(args[index+1], letCtx); ok {
			lambdaCopy := lambda
			bindings[name] = lambdaBinding{lambda: &lambdaCopy}
			continue
		}
		values := normalizeArgumentValues(expandArgumentValues(args[index+1], letCtx))
		shape, ok := inferExpressionArrayShapeWithContext(args[index+1], letCtx)
		if !ok {
			shape = arrayShape{rows: 1, cols: len(values)}
			if len(values) == 1 {
				shape = arrayShape{rows: 1, cols: 1}
			}
		}
		bindings[name] = normalizedLambdaBinding(values, shape)
	}
	return letCtx, args[len(args)-1], true, Value{}
}

func lambdaInvocationContextFromDefinition(lambda lambdaClosure, args []Expression, ctx Context) (*lambdaContext, Expression, bool, Value) {
	baseCtx := lambda.ctx
	if baseCtx == nil {
		baseCtx = ctx
	}
	bindings := make(map[string]lambdaBinding, len(args))
	lambdaCtx := &lambdaContext{base: baseCtx, currentRef: ctx.CurrentRef(), arrayAnchorRef: ctx.ArrayAnchorRef(), bindings: bindings}
	for index, arg := range args {
		values := normalizeArgumentValues(expandArgumentValues(arg, ctx))
		shape, ok := inferExpressionArrayShapeWithContext(arg, ctx)
		if !ok {
			shape = arrayShape{rows: 1, cols: len(values)}
			if len(values) == 1 {
				shape = arrayShape{rows: 1, cols: 1}
			}
		}
		bindings[lambda.def.params[index]] = normalizedLambdaBinding(values, shape)
	}
	return lambdaCtx, lambda.def.body, true, Value{}
}

func lambdaInvocationContext(callee Expression, args []Expression, ctx Context) (*lambdaContext, Expression, bool, Value) {
	lambda, ok := resolveLambdaDefinition(callee, ctx)
	if !ok {
		return nil, nil, false, Error(ErrValue)
	}
	if len(lambda.def.params) != len(args) {
		return nil, nil, false, Error(ErrValue)
	}
	return lambdaInvocationContextFromDefinition(lambda, args, ctx)
}

func evalApplyExpression(callee Expression, args []Expression, ctx Context) Value {
	lambdaCtx, body, ok, errValue := lambdaInvocationContext(callee, args, ctx)
	if !ok {
		return errValue
	}
	return body.Eval(lambdaCtx)
}

func invokeLambdaDefinition(lambda lambdaClosure, args []Expression, ctx Context) Value {
	lambdaCtx, body, ok, errValue := lambdaInvocationContextFromDefinition(lambda, args, ctx)
	if !ok {
		return errValue
	}
	return body.Eval(lambdaCtx)
}

func evaluateLambda(def lambdaDefinition, bindings map[string]lambdaBinding, currentRef, anchorRef string, ctx Context) Value {
	return def.body.Eval(&lambdaContext{base: ctx, currentRef: currentRef, arrayAnchorRef: anchorRef, bindings: bindings})
}

func evaluateScalarLambda(def lambdaDefinition, bindings map[string]lambdaBinding, currentRef, anchorRef string, ctx Context) Value {
	lambdaCtx := &lambdaContext{base: ctx, currentRef: currentRef, arrayAnchorRef: anchorRef, bindings: bindings}
	if _, ok := inferLambdaBodyArrayShape(def.body, lambdaCtx); ok {
		return Error(ErrValue)
	}
	return def.body.Eval(lambdaCtx)
}

func arrayEvalAnchor(ctx Context) (string, int, int) {
	anchorRef := ctx.CurrentRef()
	if anchorRef == "" {
		anchorRef = "A1"
	}
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if anchorRow == 0 || anchorCol == 0 {
		anchorRef = "A1"
		anchorRow, anchorCol = 1, 1
	}
	return anchorRef, anchorRow, anchorCol
}

func arrayCurrentRef(anchorRow, anchorCol, rowOffset, colOffset int) string {
	return fmt.Sprintf("%s%d", colToLetters(anchorCol+colOffset), anchorRow+rowOffset)
}

func arrayResultValue(values []Value, shape arrayShape, ctx Context) Value {
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

func argumentBroadcastLength(arrays ...[]Value) (int, bool) {
	targetLen := 1
	for _, values := range arrays {
		values = normalizeArgumentValues(values)
		if len(values) > targetLen {
			targetLen = len(values)
		}
	}
	for _, values := range arrays {
		values = normalizeArgumentValues(values)
		if len(values) != 1 && len(values) != targetLen {
			return 0, false
		}
	}
	return targetLen, true
}

func argumentValueAt(values []Value, idx int) Value {
	values = normalizeArgumentValues(values)
	if len(values) == 1 {
		return values[0]
	}
	return values[idx]
}

func evalUnaryArgumentValues(expr *UnaryExpr, ctx Context) []Value {
	values := expandArgumentValues(expr.Expr, ctx)
	result := make([]Value, len(values))
	for idx, value := range values {
		if value.IsError() {
			result[idx] = value
			continue
		}
		switch expr.Op {
		case "+":
			num, err := value.ToNumber()
			if err != nil {
				result[idx] = Error(ErrValue)
				continue
			}
			result[idx] = Number(num)
		case "-":
			num, err := value.ToNumber()
			if err != nil {
				result[idx] = Error(ErrValue)
				continue
			}
			result[idx] = Number(-num)
		default:
			result[idx] = Error(ErrValue)
		}
	}
	return result
}

func evalBinaryArgumentValues(expr *BinaryExpr, ctx Context) []Value {
	leftValues := expandArgumentValues(expr.Left, ctx)
	rightValues := expandArgumentValues(expr.Right, ctx)
	targetLen, ok := argumentBroadcastLength(leftValues, rightValues)
	if !ok {
		return []Value{Error(ErrValue)}
	}

	result := make([]Value, targetLen)
	for idx := 0; idx < targetLen; idx++ {
		leftValue := argumentValueAt(leftValues, idx)
		rightValue := argumentValueAt(rightValues, idx)
		if leftValue.IsError() {
			result[idx] = leftValue
			continue
		}
		if rightValue.IsError() {
			result[idx] = rightValue
			continue
		}
		result[idx] = evalBinaryOp(leftValue, expr.Op, rightValue)
	}
	return result
}

func evalIfArgumentValues(args []Expression, ctx Context) []Value {
	if len(args) < 2 {
		return []Value{Error(ErrValue)}
	}

	conditionValues := expandArgumentValues(args[0], ctx)
	trueValues := expandArgumentValues(args[1], ctx)
	falseValues := []Value{Bool(false)}
	if len(args) >= 3 {
		falseValues = expandArgumentValues(args[2], ctx)
	}

	targetLen, ok := argumentBroadcastLength(conditionValues, trueValues, falseValues)
	if !ok {
		return []Value{Error(ErrValue)}
	}

	result := make([]Value, targetLen)
	for idx := 0; idx < targetLen; idx++ {
		condition := argumentValueAt(conditionValues, idx)
		if condition.IsError() {
			result[idx] = condition
			continue
		}
		boolVal, err := condition.ToBool()
		if err != nil {
			result[idx] = Error(ErrValue)
			continue
		}
		if boolVal {
			result[idx] = argumentValueAt(trueValues, idx)
			continue
		}
		result[idx] = argumentValueAt(falseValues, idx)
	}
	return result
}

// collectValues collects all numeric values from arguments, expanding ranges.
func collectValues(args []Expression, ctx Context) []Value {
	var values []Value
	for _, arg := range args {
		values = append(values, expandArgumentValues(arg, ctx)...)
	}
	return values
}

type criteriaEval struct {
	values  []Value
	matcher func(Value) bool
}

func buildCriteriaEvaluators(args []Expression, start int, ctx Context) ([]criteriaEval, int, Value, bool) {
	remaining := len(args) - start
	if remaining <= 0 || remaining%2 != 0 {
		return nil, 0, Error(ErrValue), false
	}

	criteria := make([]criteriaEval, 0, remaining/2)
	targetLen := 0
	for i := start; i < len(args); i += 2 {
		values := expandArgumentValues(args[i], ctx)
		if targetLen == 0 {
			targetLen = len(values)
		} else if len(values) != targetLen {
			return nil, 0, Error(ErrValue), false
		}

		criteriaVal := args[i+1].Eval(ctx)
		if criteriaVal.IsError() {
			return nil, 0, criteriaVal, false
		}
		matcher, ok := buildCountIfMatcher(criteriaVal)
		if !ok {
			return nil, 0, Error(ErrValue), false
		}

		criteria = append(criteria, criteriaEval{values: values, matcher: matcher})
	}

	return criteria, targetLen, Value{}, true
}

func criteriaMatchAt(criteria []criteriaEval, idx int) (bool, Value) {
	for _, criterion := range criteria {
		value := criterion.values[idx]
		if value.IsError() {
			return false, value
		}
		if !criterion.matcher(value) {
			return false, Value{}
		}
	}
	return true, Value{}
}

type databaseCriterion struct {
	col     int
	matcher func(Value) bool
}

type valueExpression struct {
	value Value
}

func (e valueExpression) Eval(ctx Context) Value {
	return e.value
}

func (e valueExpression) String() string {
	return e.value.ToText()
}

func databaseMatrix(expr Expression, ctx Context) ([]Value, int, int, Value, bool) {
	vals, rows, cols := rangeToMatrix(expr, ctx)
	if vals == nil || rows == 0 || cols == 0 {
		return nil, 0, 0, Error(ErrValue), false
	}
	return vals, rows, cols, Value{}, true
}

func databaseHeaders(vals []Value, cols int) []string {
	headers := make([]string, cols)
	for col := 0; col < cols; col++ {
		headers[col] = strings.TrimSpace(vals[col].ToText())
	}
	return headers
}

func databaseFieldIndex(field Value, headers []string) (int, bool) {
	if field.IsError() {
		return -1, false
	}
	if num, err := field.ToNumber(); err == nil {
		index := int(math.Trunc(num)) - 1
		if float64(index+1) == num && index >= 0 && index < len(headers) {
			return index, true
		}
	}
	name := strings.TrimSpace(field.ToText())
	if name == "" {
		return -1, false
	}
	for index, header := range headers {
		if strings.EqualFold(header, name) {
			return index, true
		}
	}
	return -1, false
}

func buildDatabaseCriteria(criteriaVals []Value, rows, cols int, headers []string) ([][]databaseCriterion, Value, bool) {
	if rows == 0 || cols == 0 {
		return nil, Error(ErrValue), false
	}
	criteriaHeaders := databaseHeaders(criteriaVals, cols)
	if rows == 1 {
		return nil, Value{}, true
	}
	out := make([][]databaseCriterion, 0, rows-1)
	for row := 1; row < rows; row++ {
		current := make([]databaseCriterion, 0, cols)
		for col := 0; col < cols; col++ {
			header := criteriaHeaders[col]
			criterion := criteriaVals[row*cols+col]
			if criterion.IsEmpty() || (criterion.IsText() && criterion.Text == "") {
				continue
			}
			if header == "" {
				continue
			}
			databaseCol, ok := databaseFieldIndex(Text(header), headers)
			if !ok {
				return nil, Error(ErrValue), false
			}
			matcher, ok := buildCountIfMatcher(criterion)
			if !ok {
				return nil, Error(ErrValue), false
			}
			current = append(current, databaseCriterion{col: databaseCol, matcher: matcher})
		}
		out = append(out, current)
	}
	return out, Value{}, true
}

func databaseRowMatches(row []Value, criteriaRows [][]databaseCriterion) (bool, Value) {
	if len(criteriaRows) == 0 {
		return true, Value{}
	}
	for _, criteria := range criteriaRows {
		matched := true
		for _, criterion := range criteria {
			value := row[criterion.col]
			if value.IsError() {
				return false, value
			}
			if !criterion.matcher(value) {
				matched = false
				break
			}
		}
		if matched {
			return true, Value{}
		}
	}
	return false, Value{}
}

func databaseQuery(args []Expression, ctx Context) ([][]Value, int, Value, bool) {
	if len(args) != 3 {
		return nil, 0, Error(ErrValue), false
	}
	dbVals, dbRows, dbCols, errValue, ok := databaseMatrix(args[0], ctx)
	if !ok {
		return nil, 0, errValue, false
	}
	if dbRows < 1 {
		return nil, 0, Error(ErrValue), false
	}
	headers := databaseHeaders(dbVals, dbCols)
	field := args[1].Eval(ctx)
	if field.IsError() {
		return nil, 0, field, false
	}
	fieldCol, ok := databaseFieldIndex(field, headers)
	if !ok {
		return nil, 0, Error(ErrValue), false
	}
	criteriaVals, criteriaRowsCount, criteriaCols, errValue, ok := databaseMatrix(args[2], ctx)
	if !ok {
		return nil, 0, errValue, false
	}
	criteriaRows, errValue, ok := buildDatabaseCriteria(criteriaVals, criteriaRowsCount, criteriaCols, headers)
	if !ok {
		return nil, 0, errValue, false
	}
	matches := make([][]Value, 0, dbRows-1)
	for row := 1; row < dbRows; row++ {
		rowValues := dbVals[row*dbCols : (row+1)*dbCols]
		matched, errValue := databaseRowMatches(rowValues, criteriaRows)
		if errValue.IsError() {
			return nil, 0, errValue, false
		}
		if matched {
			matches = append(matches, rowValues)
		}
	}
	return matches, fieldCol, Value{}, true
}

// fnSumProduct multiplies aligned array elements and returns the total sum.
func fnSumProduct(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}

	arrays := make([][]Value, len(args))
	targetLen := 1
	for i, arg := range args {
		values := expandArgumentValues(arg, ctx)
		if len(values) == 0 {
			values = []Value{Empty()}
		}
		arrays[i] = values
		if len(values) > targetLen {
			targetLen = len(values)
		}
	}
	for _, values := range arrays {
		if len(values) != 1 && len(values) != targetLen {
			return Error(ErrValue)
		}
	}

	var sum float64
	for idx := 0; idx < targetLen; idx++ {
		product := 1.0
		for _, values := range arrays {
			value := values[0]
			if len(values) > 1 {
				value = values[idx]
			}
			if value.IsError() {
				return value
			}
			num, err := value.ToNumber()
			if err != nil {
				num = 0
			}
			product *= num
		}
		sum += product
	}
	return Number(sum)
}

// fnSum calculates the sum of values.
func fnSum(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var sum float64
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			sum += num
		}
	}
	return Number(sum)
}

// fnProduct multiplies numeric values.
func fnProduct(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	product := 1.0
	var hasValue bool
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			product *= num
			hasValue = true
		}
	}
	if !hasValue {
		return Number(0)
	}
	return Number(product)
}

// fnSumSq sums the squares of numeric values.
func fnSumSq(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var sum float64
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			sum += num * num
		}
	}
	return Number(sum)
}

func numericValues(args []Expression, ctx Context) ([]float64, Value, bool) {
	values := collectValues(args, ctx)
	numbers := make([]float64, 0, len(values))
	for _, v := range values {
		if v.IsError() {
			return nil, v, false
		}
		num, err := v.ToNumber()
		if err == nil {
			numbers = append(numbers, num)
		}
	}
	return numbers, Value{}, true
}

// fnMedian returns the median numeric value.
func fnMedian(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrNum)
	}
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) == 0 {
		return Error(ErrNum)
	}
	sort.Float64s(numbers)
	mid := len(numbers) / 2
	if len(numbers)%2 == 1 {
		return Number(numbers[mid])
	}
	return Number((numbers[mid-1] + numbers[mid]) / 2)
}

func rankValues(args []Expression, ctx Context) ([]float64, int, Value, bool) {
	if len(args) != 2 {
		return nil, 0, Error(ErrValue), false
	}

	values := collectValues([]Expression{args[0]}, ctx)
	rankValue := args[1].Eval(ctx)
	if rankValue.IsError() {
		return nil, 0, rankValue, false
	}
	rankNum, err := rankValue.ToNumber()
	if err != nil {
		return nil, 0, Error(ErrValue), false
	}
	rank := int(rankNum)
	if float64(rank) != rankNum || rank < 1 {
		return nil, 0, Error(ErrNum), false
	}

	numbers := make([]float64, 0, len(values))
	for _, v := range values {
		if v.IsError() {
			return nil, 0, v, false
		}
		num, err := v.ToNumber()
		if err == nil {
			numbers = append(numbers, num)
		}
	}
	if rank > len(numbers) {
		return nil, 0, Error(ErrNum), false
	}
	return numbers, rank, Value{}, true
}

// fnRank returns the descending rank of a number in a range.
func fnRank(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	needle := args[0].Eval(ctx)
	if needle.IsError() {
		return needle
	}
	needleNum, err := needle.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	numbers, errValue, ok := numericValues([]Expression{args[1]}, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) == 0 {
		return Error(ErrNA)
	}
	sort.Float64s(numbers)
	rank := 1
	for idx := len(numbers) - 1; idx >= 0; idx-- {
		if numbers[idx] == needleNum {
			return Number(float64(rank))
		}
		rank++
	}
	return Error(ErrNA)
}

// fnLarge returns the k-th largest numeric value.
func fnLarge(args []Expression, ctx Context) Value {
	numbers, rank, errValue, ok := rankValues(args, ctx)
	if !ok {
		return errValue
	}
	sort.Float64s(numbers)
	return Number(numbers[len(numbers)-rank])
}

// fnSmall returns the k-th smallest numeric value.
func fnSmall(args []Expression, ctx Context) Value {
	numbers, rank, errValue, ok := rankValues(args, ctx)
	if !ok {
		return errValue
	}
	sort.Float64s(numbers)
	return Number(numbers[rank-1])
}

// fnCountBlank counts empty or empty-string values.
func fnCountBlank(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}
	values := collectValues(args, ctx)
	var count int
	for _, v := range values {
		if v.IsError() {
			return v
		}
		if v.IsEmpty() || (v.IsText() && v.Text == "") {
			count++
		}
	}
	return Number(float64(count))
}

// fnMode returns the most frequently occurring numeric value.
func fnMode(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrNA)
	}
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) == 0 {
		return Error(ErrNA)
	}
	freq := make(map[float64]int, len(numbers))
	bestValue := 0.0
	bestCount := 0
	for _, num := range numbers {
		freq[num]++
		count := freq[num]
		if count > bestCount || (count == bestCount && num < bestValue) {
			bestValue = num
			bestCount = count
		}
	}
	if bestCount < 2 {
		return Error(ErrNA)
	}
	return Number(bestValue)
}

// fnSumIf sums matching values from a range using COUNTIF-style criteria.
func fnSumIf(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}

	matchValues := collectValues([]Expression{args[0]}, ctx)
	if len(matchValues) == 0 {
		return Number(0)
	}
	criteriaVal := args[1].Eval(ctx)
	if criteriaVal.IsError() {
		return criteriaVal
	}
	matcher, ok := buildCountIfMatcher(criteriaVal)
	if !ok {
		return Error(ErrValue)
	}

	sumValues := matchValues
	if len(args) == 3 {
		sumValues = expandArgumentValues(args[2], ctx)
		if len(sumValues) != len(matchValues) {
			return Error(ErrValue)
		}
	}

	var sum float64
	for idx, matchValue := range matchValues {
		if matchValue.IsError() {
			return matchValue
		}
		if !matcher(matchValue) {
			continue
		}
		sumValue := sumValues[idx]
		if sumValue.IsError() {
			return sumValue
		}
		num, err := sumValue.ToNumber()
		if err == nil {
			sum += num
		}
	}
	return Number(sum)
}

// fnSumIfs sums values that satisfy all criteria ranges.
func fnSumIfs(args []Expression, ctx Context) Value {
	if len(args) < 3 {
		return Error(ErrValue)
	}

	sumValues := expandArgumentValues(args[0], ctx)
	criteria, targetLen, errVal, ok := buildCriteriaEvaluators(args, 1, ctx)
	if !ok {
		return errVal
	}
	if len(sumValues) != targetLen {
		return Error(ErrValue)
	}

	var sum float64
	for idx := 0; idx < targetLen; idx++ {
		matched, errValue := criteriaMatchAt(criteria, idx)
		if errValue.IsError() {
			return errValue
		}
		if !matched {
			continue
		}

		value := sumValues[idx]
		if value.IsError() {
			return value
		}
		num, err := value.ToNumber()
		if err == nil {
			sum += num
		}
	}
	return Number(sum)
}

// fnAverageIf averages values matching a COUNTIF-style criterion.
func fnAverageIf(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}

	matchValues := collectValues([]Expression{args[0]}, ctx)
	if len(matchValues) == 0 {
		return Error(ErrDiv0)
	}
	criteriaVal := args[1].Eval(ctx)
	if criteriaVal.IsError() {
		return criteriaVal
	}
	matcher, ok := buildCountIfMatcher(criteriaVal)
	if !ok {
		return Error(ErrValue)
	}

	averageValues := matchValues
	if len(args) == 3 {
		averageValues = expandArgumentValues(args[2], ctx)
		if len(averageValues) != len(matchValues) {
			return Error(ErrValue)
		}
	}

	var sum float64
	var count int
	for idx, matchValue := range matchValues {
		if matchValue.IsError() {
			return matchValue
		}
		if !matcher(matchValue) {
			continue
		}

		value := averageValues[idx]
		if value.IsError() {
			return value
		}
		num, err := value.ToNumber()
		if err == nil {
			sum += num
			count++
		}
	}
	if count == 0 {
		return Error(ErrDiv0)
	}
	return Number(sum / float64(count))
}

// fnCountIfs counts positions that satisfy every criteria range.
func fnCountIfs(args []Expression, ctx Context) Value {
	criteria, targetLen, errVal, ok := buildCriteriaEvaluators(args, 0, ctx)
	if !ok {
		return errVal
	}

	var count int
	for idx := 0; idx < targetLen; idx++ {
		matched, errValue := criteriaMatchAt(criteria, idx)
		if errValue.IsError() {
			return errValue
		}
		if matched {
			count++
		}
	}
	return Number(float64(count))
}

// fnAverageIfs averages values that satisfy all criteria ranges.
func fnAverageIfs(args []Expression, ctx Context) Value {
	if len(args) < 3 {
		return Error(ErrValue)
	}

	averageValues := expandArgumentValues(args[0], ctx)
	criteria, targetLen, errVal, ok := buildCriteriaEvaluators(args, 1, ctx)
	if !ok {
		return errVal
	}
	if len(averageValues) != targetLen {
		return Error(ErrValue)
	}

	var sum float64
	var count int
	for idx := 0; idx < targetLen; idx++ {
		matched, errValue := criteriaMatchAt(criteria, idx)
		if errValue.IsError() {
			return errValue
		}
		if !matched {
			continue
		}

		value := averageValues[idx]
		if value.IsError() {
			return value
		}
		num, err := value.ToNumber()
		if err == nil {
			sum += num
			count++
		}
	}
	if count == 0 {
		return Error(ErrDiv0)
	}
	return Number(sum / float64(count))
}

func fnDSum(args []Expression, ctx Context) Value {
	rows, fieldCol, errValue, ok := databaseQuery(args, ctx)
	if !ok {
		return errValue
	}
	var sum float64
	for _, row := range rows {
		value := row[fieldCol]
		if value.IsError() {
			return value
		}
		num, err := value.ToNumber()
		if err == nil {
			sum += num
		}
	}
	return Number(sum)
}

func fnDCount(args []Expression, ctx Context) Value {
	rows, fieldCol, errValue, ok := databaseQuery(args, ctx)
	if !ok {
		return errValue
	}
	count := 0
	for _, row := range rows {
		value := row[fieldCol]
		if value.IsError() {
			return value
		}
		if _, err := value.ToNumber(); err == nil {
			count++
		}
	}
	return Number(float64(count))
}

func fnDCountA(args []Expression, ctx Context) Value {
	rows, fieldCol, errValue, ok := databaseQuery(args, ctx)
	if !ok {
		return errValue
	}
	count := 0
	for _, row := range rows {
		value := row[fieldCol]
		if value.IsError() {
			return value
		}
		if !value.IsEmpty() && !(value.IsText() && value.Text == "") {
			count++
		}
	}
	return Number(float64(count))
}

func databaseFieldValues(args []Expression, ctx Context) ([]Value, Value, bool) {
	rows, fieldCol, errValue, ok := databaseQuery(args, ctx)
	if !ok {
		return nil, errValue, false
	}
	values := make([]Value, 0, len(rows))
	for _, row := range rows {
		value := row[fieldCol]
		if value.IsError() {
			return nil, value, false
		}
		values = append(values, value)
	}
	return values, Value{}, true
}

func fnDAverage(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	var sum float64
	var count int
	for _, value := range values {
		num, err := value.ToNumber()
		if err == nil {
			sum += num
			count++
		}
	}
	if count == 0 {
		return Error(ErrDiv0)
	}
	return Number(sum / float64(count))
}

func fnDGet(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) != 1 {
		return Error(ErrNum)
	}
	return values[0]
}

func fnDMax(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnMax(fnArgs, ctx)
}

func fnDMin(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnMin(fnArgs, ctx)
}

func fnDProduct(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnProduct(fnArgs, ctx)
}

func fnDStdev(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnStdev(fnArgs, ctx)
}

func fnDStdevp(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnStdevp(fnArgs, ctx)
}

func fnDVar(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnVar(fnArgs, ctx)
}

func fnDVarp(args []Expression, ctx Context) Value {
	values, errValue, ok := databaseFieldValues(args, ctx)
	if !ok {
		return errValue
	}
	fnArgs := make([]Expression, len(values))
	for i, value := range values {
		fnArgs[i] = valueExpression{value: value}
	}
	return fnVarp(fnArgs, ctx)
}

// fnAverage calculates the average of values.
func fnAverage(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrDiv0)
	}

	values := collectValues(args, ctx)
	var sum float64
	var count int
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			sum += num
			count++
		}
	}

	if count == 0 {
		return Error(ErrDiv0)
	}
	return Number(sum / float64(count))
}

// fnCount counts numeric values.
func fnCount(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var count int
	for _, v := range values {
		if v.IsError() {
			return v
		}
		if _, err := v.ToNumber(); err == nil {
			count++
		}
	}
	return Number(float64(count))
}

// fnCountA counts non-empty values.
func fnCountA(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var count int
	for _, v := range values {
		if v.IsError() {
			return v
		}
		if !v.IsEmpty() && !(v.IsText() && v.Text == "") {
			count++
		}
	}
	return Number(float64(count))
}

// fnMax finds the maximum value.
func fnMax(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var max float64
	var hasValue bool
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			if !hasValue || num > max {
				max = num
				hasValue = true
			}
		}
	}

	if !hasValue {
		return Number(0)
	}
	return Number(max)
}

// fnMin finds the minimum value.
func fnMin(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Number(0)
	}

	values := collectValues(args, ctx)
	var min float64
	var hasValue bool
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err == nil {
			if !hasValue || num < min {
				min = num
				hasValue = true
			}
		}
	}

	if !hasValue {
		return Number(0)
	}
	return Number(min)
}

// fnCountIf counts cells that match a criteria expression.
func fnCountIf(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}

	values := collectValues([]Expression{args[0]}, ctx)
	if len(values) == 0 {
		return Number(0)
	}
	criteriaVal := args[1].Eval(ctx)
	if criteriaVal.IsError() {
		return criteriaVal
	}
	matcher, ok := buildCountIfMatcher(criteriaVal)
	if !ok {
		return Error(ErrValue)
	}
	var count int
	for _, v := range values {
		if v.IsError() {
			return v
		}
		if matcher(v) {
			count++
		}
	}
	return Number(float64(count))
}

func buildCountIfMatcher(criteria Value) (func(Value) bool, bool) {
	if criteria.IsNumber() || criteria.IsBool() {
		return func(v Value) bool {
			return valuesEqual(v, criteria)
		}, true
	}
	if criteria.IsEmpty() {
		return func(v Value) bool {
			return v.IsEmpty() || (v.IsText() && v.Text == "")
		}, true
	}

	text := criteria.ToText()
	op := "="
	operand := text
	for _, candidate := range []string{"<>", "<=", ">=", "<", ">", "="} {
		if strings.HasPrefix(text, candidate) {
			op = candidate
			operand = strings.TrimSpace(text[len(candidate):])
			break
		}
	}
	operandValue := Text(operand)
	if num, err := strconv.ParseFloat(operand, 64); err == nil {
		operandValue = Number(num)
	} else {
		upper := strings.ToUpper(operand)
		switch upper {
		case "TRUE":
			operandValue = Bool(true)
		case "FALSE":
			operandValue = Bool(false)
		case "":
			operandValue = Empty()
		default:
			if errType := ParseError(operand); errType != ErrNone {
				operandValue = Error(errType)
			}
		}
	}
	return func(v Value) bool {
		switch op {
		case "=":
			return valuesEqual(v, operandValue)
		case "<>":
			return !valuesEqual(v, operandValue)
		default:
			result := compareValues(v, op, operandValue)
			if result.IsError() {
				return false
			}
			boolVal, err := result.ToBool()
			return err == nil && boolVal
		}
	}, true
}

// fnIf implements conditional logic.
func fnIf(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}

	condition := args[0].Eval(ctx)
	if condition.IsError() {
		return condition
	}

	boolVal, err := condition.ToBool()
	if err != nil {
		return Error(ErrValue)
	}

	if boolVal {
		return args[1].Eval(ctx)
	}

	if len(args) >= 3 {
		return args[2].Eval(ctx)
	}

	return Bool(false)
}

func expressionValueAtOffset(expr Expression, ctx Context, rowOffset, colOffset int) Value {
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(expr, sourceCtx))
	if len(values) == 0 {
		return Error(ErrValue)
	}
	if len(values) == 1 {
		return values[0]
	}
	shape, ok := inferExpressionArrayShapeWithContext(expr, sourceCtx)
	if !ok || shape.rows <= 0 || shape.cols <= 0 || shape.rows*shape.cols != len(values) {
		shape = arrayShape{rows: 1, cols: len(values)}
	}
	valueIndex := 0
	switch {
	case shape.rows == 1:
		if colOffset < 0 || colOffset >= shape.cols {
			return Error(ErrValue)
		}
		valueIndex = colOffset
	case shape.cols == 1:
		if rowOffset < 0 || rowOffset >= shape.rows {
			return Error(ErrValue)
		}
		valueIndex = rowOffset
	default:
		if rowOffset < 0 || colOffset < 0 || rowOffset >= shape.rows || colOffset >= shape.cols {
			return Error(ErrValue)
		}
		valueIndex = rowOffset*shape.cols + colOffset
	}
	if valueIndex < 0 || valueIndex >= len(values) {
		return Error(ErrValue)
	}
	return values[valueIndex]
}

func expressionOffsetsForLinearIndex(expr Expression, ctx Context, linearIndex int) (int, int, bool) {
	if linearIndex < 0 {
		return 0, 0, false
	}
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(expr, sourceCtx))
	if len(values) == 0 {
		return 0, 0, false
	}
	if len(values) == 1 {
		return 0, 0, true
	}
	shape, ok := inferExpressionArrayShapeWithContext(expr, sourceCtx)
	if !ok || shape.rows <= 0 || shape.cols <= 0 || shape.rows*shape.cols != len(values) {
		shape = arrayShape{rows: 1, cols: len(values)}
	}
	if linearIndex >= len(values) {
		return 0, 0, false
	}
	if shape.rows == 1 {
		return 0, linearIndex, true
	}
	if shape.cols == 1 {
		return linearIndex, 0, true
	}
	return linearIndex / shape.cols, linearIndex % shape.cols, true
}

type referenceShapeOffsetContext struct {
	Context
	currentRef     string
	arrayAnchorRef string
}

func (c *referenceShapeOffsetContext) CurrentRef() string {
	return c.currentRef
}

func (c *referenceShapeOffsetContext) ArrayAnchorRef() string {
	return c.arrayAnchorRef
}

func (c *referenceShapeOffsetContext) GetSpillRange(ref string) (string, string, bool) {
	return c.Context.GetSpillRange(ref)
}

func (c *referenceShapeOffsetContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	if resolver, ok := c.Context.(pivotDataResolver); ok {
		return resolver.ResolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
	}
	return Value{}, false
}

func contextForArrayOffset(ctx Context, rowOffset, colOffset int) (Context, bool) {
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = ctx.CurrentRef()
	}
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if anchorRow == 0 || anchorCol == 0 || rowOffset < 0 || colOffset < 0 {
		return nil, false
	}
	currentRow := anchorRow + rowOffset
	currentCol := anchorCol + colOffset
	if currentRow < 1 || currentCol < 1 {
		return nil, false
	}
	currentRef := fmt.Sprintf("%s%d", colToLetters(currentCol), currentRow)
	return &referenceShapeOffsetContext{Context: ctx, currentRef: currentRef, arrayAnchorRef: anchorRef}, true
}

func expressionValueAtCurrentOffset(expr Expression, ctx Context) Value {
	rowOffset, colOffset, _ := currentArrayOffsets(ctx)
	if rowOffset < 0 || colOffset < 0 {
		return Error(ErrValue)
	}
	return expressionValueAtOffset(expr, ctx, rowOffset, colOffset)
}

func expressionValueAtProjectedCurrentOffset(expr Expression, ctx Context) Value {
	rowOffset, colOffset, ok := currentArrayOffsets(ctx)
	if !ok {
		rowOffset, colOffset = 0, 0
	}
	if rowOffset < 0 || colOffset < 0 {
		return Error(ErrValue)
	}
	sourceCtx := arraySourceContext(ctx)
	shape, ok := inferExpressionArrayShapeWithContext(expr, sourceCtx)
	if !ok || shape.rows <= 0 || shape.cols <= 0 {
		return expressionValueAtOffset(expr, ctx, rowOffset, colOffset)
	}
	if shape.rows == 1 {
		rowOffset = 0
	}
	if shape.cols == 1 {
		colOffset = 0
	}
	return expressionValueAtOffset(expr, ctx, rowOffset, colOffset)
}

func selectIfsBranch(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 2 || len(args)%2 != 0 {
		return nil, Error(ErrValue), false
	}
	for i := 0; i < len(args); i += 2 {
		condition := expressionValueAtCurrentOffset(args[i], ctx)
		if condition.IsError() {
			return nil, condition, false
		}
		boolVal, err := condition.ToBool()
		if err != nil {
			return nil, Error(ErrValue), false
		}
		if boolVal {
			return args[i+1], Value{}, true
		}
	}
	return nil, Error(ErrNA), false
}

func fnIfs(args []Expression, ctx Context) Value {
	selected, errValue, ok := selectIfsBranch(args, ctx)
	if !ok {
		return errValue
	}
	return selected.Eval(ctx)
}

func selectIfErrorBranch(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Error(ErrValue), false
	}
	value := args[0].Eval(ctx)
	if !value.IsError() {
		return args[0], Value{}, true
	}
	if len(args) == 1 {
		return &StringExpr{Value: ""}, Value{}, true
	}
	return args[1], Value{}, true
}

func fnIfError(args []Expression, ctx Context) Value {
	selected, errValue, ok := selectIfErrorBranch(args, ctx)
	if !ok {
		return errValue
	}
	return selected.Eval(ctx)
}

func selectIfNaBranch(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Error(ErrValue), false
	}
	value := args[0].Eval(ctx)
	if !value.IsError() || value.Err != ErrNA {
		return args[0], Value{}, true
	}
	if len(args) == 1 {
		return &StringExpr{Value: ""}, Value{}, true
	}
	return args[1], Value{}, true
}

func fnIfNA(args []Expression, ctx Context) Value {
	selected, errValue, ok := selectIfNaBranch(args, ctx)
	if !ok {
		return errValue
	}
	return selected.Eval(ctx)
}

func selectSwitchBranch(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 3 {
		return nil, Error(ErrValue), false
	}
	selector := expressionValueAtCurrentOffset(args[0], ctx)
	if selector.IsError() {
		return nil, selector, false
	}
	pairEnd := len(args)
	var defaultExpr Expression
	if len(args)%2 == 0 {
		defaultExpr = args[len(args)-1]
		pairEnd = len(args) - 1
	}
	for i := 1; i < pairEnd; i += 2 {
		candidate := expressionValueAtCurrentOffset(args[i], ctx)
		if candidate.IsError() {
			return nil, candidate, false
		}
		if valuesEqual(selector, candidate) {
			return args[i+1], Value{}, true
		}
	}
	if defaultExpr != nil {
		return defaultExpr, Value{}, true
	}
	return nil, Error(ErrNA), false
}

func fnSwitch(args []Expression, ctx Context) Value {
	selected, errValue, ok := selectSwitchBranch(args, ctx)
	if !ok {
		return errValue
	}
	return selected.Eval(ctx)
}

// fnChoose returns the value of the selected argument.
func fnChoose(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	selectorValue := chooseSelectorValue(args[0], ctx)
	if selectorValue.IsError() {
		return selectorValue
	}
	num, err := selectorValue.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	selector := int(num)
	if selector < 1 || selector >= len(args) {
		return Error(ErrValue)
	}
	return args[selector].Eval(ctx)
}

func chooseSelectorValue(expr Expression, ctx Context) Value {
	if selector, ok := inferStaticIndexSelector(expr); ok {
		return Number(float64(selector))
	}
	return expressionValueAtCurrentOffset(expr, ctx)
}

func chooseTargetExpression(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 2 {
		return nil, Error(ErrValue), false
	}
	selectorValue := chooseSelectorValue(args[0], ctx)
	if selectorValue.IsError() {
		return nil, selectorValue, false
	}
	num, err := selectorValue.ToNumber()
	if err != nil {
		return nil, Error(ErrValue), false
	}
	selector := int(num)
	if selector < 1 || selector >= len(args) {
		return nil, Error(ErrValue), false
	}
	return args[selector], Value{}, true
}

func ifTargetExpression(args []Expression, ctx Context) (Expression, Value, bool) {
	if len(args) < 2 || len(args) > 3 {
		return nil, Error(ErrValue), false
	}
	conditionValue := expressionValueAtCurrentOffset(args[0], ctx)
	if conditionValue.IsError() {
		return nil, conditionValue, false
	}
	isTrue, err := conditionValue.ToBool()
	if err != nil {
		return nil, Error(ErrValue), false
	}
	if isTrue {
		return args[1], Value{}, true
	}
	if len(args) >= 3 {
		return args[2], Value{}, true
	}
	return nil, Value{}, true
}

// fnChooseCols selects and reorders columns from a range.
func fnChooseCols(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return Error(ErrRef)
	}
	indices, ok := chooseAxisSelectors(args[1:], ctx)
	if !ok {
		return Error(ErrValue)
	}
	targetRef, ok := chooseAxisTargetRef(start, end, indices, false, ctx)
	if !ok || targetRef == "" {
		return Error(ErrRef)
	}
	return ctx.GetCell(targetRef)
}

// fnChooseRows selects and reorders rows from a range.
func fnChooseRows(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return Error(ErrRef)
	}
	indices, ok := chooseAxisSelectors(args[1:], ctx)
	if !ok {
		return Error(ErrValue)
	}
	targetRef, ok := chooseAxisTargetRef(start, end, indices, true, ctx)
	if !ok || targetRef == "" {
		return Error(ErrRef)
	}
	return ctx.GetCell(targetRef)
}

// fnTake selects a leading/trailing slice of rows and optional columns.
func fnTake(args []Expression, ctx Context) Value {
	targetRef, ok, errValue := takeDropTargetRef(args, true, ctx)
	if !ok {
		return errValue
	}
	return ctx.GetCell(targetRef)
}

// fnDrop removes leading/trailing rows and optional columns.
func fnDrop(args []Expression, ctx Context) Value {
	targetRef, ok, errValue := takeDropTargetRef(args, false, ctx)
	if !ok {
		return errValue
	}
	return ctx.GetCell(targetRef)
}

// fnFilter filters rows or columns from an array using a boolean include array.
func fnFilter(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := filterArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

// fnUnique returns the unique rows or columns from an array.
func fnUnique(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := uniqueArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

// fnSort sorts rows or columns from an array while preserving the source shape.
func fnSort(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := sortArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

// fnSortBy sorts rows or columns from an array using one or more external key arrays.
func fnSortBy(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := sortByArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

// fnSequence returns a sequential array with configurable shape, start, and step.
func fnSequence(args []Expression, ctx Context) Value {
	rows, cols, start, step, ok, errValue := sequenceArgs(args, ctx)
	if !ok {
		return errValue
	}
	if rows <= 1 && cols <= 1 {
		return Number(start)
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= rows || colOffset < 0 || colOffset >= cols {
		return Error(ErrNA)
	}
	index := rowOffset*cols + colOffset
	return Number(start + step*float64(index))
}

// fnRandArray returns a spilled array of random numbers.
func fnRandArray(args []Expression, ctx Context) Value {
	_, _, minValue, maxValue, wholeNumber, ok, errValue := randArrayArgs(args, ctx)
	if !ok {
		return errValue
	}
	if maxValue < minValue {
		return Error(ErrValue)
	}
	if wholeNumber {
		lower := int(math.Ceil(minValue))
		upper := int(math.Floor(maxValue))
		if upper < lower {
			return Error(ErrValue)
		}
		if upper == lower {
			return Number(float64(lower))
		}
		return Number(float64(lower + rand.Intn(upper-lower+1)))
	}
	if maxValue == minValue {
		return Number(minValue)
	}
	return Number(minValue + rand.Float64()*(maxValue-minValue))
}

// fnExpand pads an array to the requested size.
func fnExpand(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := expandArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnMakeArray generates an array by invoking a lambda for each row/column pair.
func fnMakeArray(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := makeArrayResult(args, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnByRow evaluates a lambda once for each row in an array.
func fnByRow(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := byAxisLambdaResult(args, true, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnByCol evaluates a lambda once for each column in an array.
func fnByCol(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := byAxisLambdaResult(args, false, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnMap applies a lambda element-wise across one or more arrays.
func fnMap(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := mapLambdaResult(args, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnReduce folds an array into a scalar accumulator.
func fnReduce(args []Expression, ctx Context) Value {
	value, ok, errValue := reduceLambdaValue(args, ctx)
	if !ok {
		return errValue
	}
	return value
}

// fnScan returns the running accumulator across an array.
func fnScan(args []Expression, ctx Context) Value {
	values, shape, ok, errValue := scanLambdaResult(args, ctx)
	if !ok {
		return errValue
	}
	return arrayResultValue(values, shape, ctx)
}

// fnLet evaluates bindings left-to-right and returns the final expression value.
func fnLet(args []Expression, ctx Context) Value {
	letCtx, finalExpr, ok, errValue := letContextForArgs(args, ctx)
	if !ok {
		return errValue
	}
	return finalExpr.Eval(letCtx)
}

// fnMatch returns the relative position of an item in a lookup vector.
func fnMatch(args []Expression, ctx Context) Value {
	lookupValue, lookupArray, matchMode, searchMode, ok, errValue := matchArgs(args, ctx)
	if !ok {
		return errValue
	}
	index, errValue, ok := xmatchIndex(lookupValue, lookupArray, matchMode, searchMode)
	if !ok {
		return errValue
	}
	return Number(float64(index + 1))
}

// fnLookup returns a matched value using LOOKUP compatibility defaults.
func fnLookup(args []Expression, ctx Context) Value {
	lookupValue, lookupArray, returnArray, ok, errValue := lookupArgs(args, ctx)
	if !ok {
		return errValue
	}
	index, errValue, ok := xmatchIndex(lookupValue, lookupArray, -1, 2)
	if !ok {
		return errValue
	}
	if index < 0 || index >= len(returnArray) {
		return Error(ErrNA)
	}
	value := returnArray[index]
	if value.IsError() {
		return value
	}
	return value
}

// fnXMatch returns the relative position of a matching item in a lookup array.
func fnXMatch(args []Expression, ctx Context) Value {
	lookupValue, lookupArray, matchMode, searchMode, ok, errValue := xmatchArgs(args, ctx)
	if !ok {
		return errValue
	}
	index, errValue, ok := xmatchIndex(lookupValue, lookupArray, matchMode, searchMode)
	if !ok {
		return errValue
	}
	return Number(float64(index + 1))
}

// fnXLookup returns a matched value from a parallel return array.
func fnXLookup(args []Expression, ctx Context) Value {
	lookupValue, lookupArray, returnArray, ifNotFound, matchMode, searchMode, ok, errValue := xlookupArgs(args, ctx)
	if !ok {
		return errValue
	}
	index, errValue, ok := xmatchIndex(lookupValue, lookupArray, matchMode, searchMode)
	if !ok {
		if errValue.IsError() && errValue.Err == ErrNA && ifNotFound.Type != TypeEmpty {
			return ifNotFound
		}
		return errValue
	}
	lookupSource, lookupShape, ok, shapeErr := stackSourceValues(args[1], ctx)
	if !ok {
		return shapeErr
	}
	_, returnShape, ok, shapeErr := stackSourceValues(args[2], ctx)
	if !ok {
		return shapeErr
	}
	if lookupShape.cols == 1 {
		if index < 0 || index >= returnShape.rows {
			return Error(ErrNA)
		}
		if returnShape.cols == 1 {
			value := returnArray[index*returnShape.cols]
			if value.IsError() {
				return value
			}
			return value
		}
		currentRef := ctx.CurrentRef()
		anchorRef := ctx.ArrayAnchorRef()
		if anchorRef == "" {
			anchorRef = currentRef
		}
		currentRow, currentCol := parseCellRef(currentRef)
		anchorRow, anchorCol := parseCellRef(anchorRef)
		if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
			return Error(ErrNA)
		}
		rowOffset := currentRow - anchorRow
		colOffset := currentCol - anchorCol
		if rowOffset < 0 || colOffset < 0 || colOffset >= returnShape.cols {
			return Error(ErrNA)
		}
		value := returnArray[index*returnShape.cols+colOffset]
		if value.IsError() {
			return value
		}
		return value
	}
	if len(lookupSource) == 0 {
		return Error(ErrNA)
	}
	if index < 0 || index >= returnShape.cols {
		return Error(ErrNA)
	}
	if returnShape.rows == 1 {
		value := returnArray[index]
		if value.IsError() {
			return value
		}
		return value
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if colOffset < 0 || rowOffset < 0 || rowOffset >= returnShape.rows {
		return Error(ErrNA)
	}
	value := returnArray[rowOffset*returnShape.cols+index]
	if value.IsError() {
		return value
	}
	return value
}

// fnHStack appends arrays horizontally, padding shorter inputs with #N/A.
func fnHStack(args []Expression, ctx Context) Value {
	return stackValue(args, true, ctx)
}

// fnVStack appends arrays vertically, padding shorter inputs with #N/A.
func fnVStack(args []Expression, ctx Context) Value {
	return stackValue(args, false, ctx)
}

// fnToRow flattens an array into a single row.
func fnToRow(args []Expression, ctx Context) Value {
	return toVectorValue(args, false, ctx)
}

// fnToCol flattens an array into a single column.
func fnToCol(args []Expression, ctx Context) Value {
	return toVectorValue(args, true, ctx)
}

// fnWrapRows wraps a flattened array into rows of a fixed width.
func fnWrapRows(args []Expression, ctx Context) Value {
	return wrapVectorValue(args, true, ctx)
}

// fnWrapCols wraps a flattened array into columns of a fixed height.
func fnWrapCols(args []Expression, ctx Context) Value {
	return wrapVectorValue(args, false, ctx)
}

// fnIndex returns a referenced value or selector view from a range.
func fnIndex(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 4 {
		return Error(ErrValue)
	}
	if len(args) == 4 {
		areaNum, ok := indexSelectorNumber(args[3], ctx)
		if !ok {
			return Error(ErrValue)
		}
		if areaNum != 1 {
			return Error(ErrRef)
		}
	}
	rowNum, ok := indexSelectorNumber(args[1], ctx)
	if !ok {
		return Error(ErrValue)
	}
	colNum := 1
	if len(args) >= 3 {
		colValue, ok := indexSelectorNumber(args[2], ctx)
		if !ok {
			return Error(ErrValue)
		}
		colNum = colValue
	}
	if rowNum < 0 || colNum < 0 {
		return Error(ErrValue)
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		if rowNum == 1 && colNum == 1 {
			return args[0].Eval(ctx)
		}
		return Error(ErrRef)
	}
	targetRef, ok := indexRangeTargetRef(start, end, rowNum, colNum, ctx)
	if !ok || targetRef == "" {
		return Error(ErrRef)
	}
	return ctx.GetCell(targetRef)
}

// fnOffset returns a value from a shifted reference or range.
func fnOffset(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return Error(ErrRef)
	}
	rowOffset, ok := indexSelectorNumber(args[1], ctx)
	if !ok {
		return Error(ErrValue)
	}
	colOffset, ok := indexSelectorNumber(args[2], ctx)
	if !ok {
		return Error(ErrValue)
	}
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return Error(ErrRef)
	}
	height := endCoords.row - startCoords.row + 1
	width := endCoords.col - startCoords.col + 1
	if len(args) >= 4 {
		height, ok = indexSelectorNumber(args[3], ctx)
		if !ok || height <= 0 {
			return Error(ErrValue)
		}
	}
	if len(args) >= 5 {
		width, ok = indexSelectorNumber(args[4], ctx)
		if !ok || width <= 0 {
			return Error(ErrValue)
		}
	}
	targetStart, targetEnd, ok := offsetRangeRefs(start, end, rowOffset, colOffset, height, width)
	if !ok {
		return Error(ErrRef)
	}
	targetRef, ok := offsetRangeTargetRef(targetStart, targetEnd, ctx)
	if !ok || targetRef == "" {
		return Error(ErrRef)
	}
	return ctx.GetCell(targetRef)
}

// fnTranspose returns the transposed member of a range for array evaluation.
func fnTranspose(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	switch expr := args[0].(type) {
	case *RangeExpr:
		targetRef, ok := transposeRangeTargetRef(expr.Start, expr.End, ctx)
		if !ok || targetRef == "" {
			return Error(ErrNA)
		}
		return ctx.GetCell(targetRef)
	default:
		return args[0].Eval(ctx)
	}
}

func toVectorValue(args []Expression, asColumn bool, ctx Context) Value {
	values, ok, errValue := toVectorArgs(args, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 0 {
		return Error(ErrNA)
	}
	if len(values) == 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	linearIndex := 0
	if asColumn {
		if colOffset != 0 || rowOffset < 0 || rowOffset >= len(values) {
			return Error(ErrNA)
		}
		linearIndex = rowOffset
	} else {
		if rowOffset != 0 || colOffset < 0 || colOffset >= len(values) {
			return Error(ErrNA)
		}
		linearIndex = colOffset
	}
	return values[linearIndex]
}

func wrapVectorValue(args []Expression, wrapRows bool, ctx Context) Value {
	values, wrapCount, padValue, ok, errValue := wrapArgs(args, ctx)
	if !ok {
		return errValue
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	linearIndex := 0
	if wrapRows {
		if rowOffset < 0 || colOffset < 0 || colOffset >= wrapCount {
			return Error(ErrNA)
		}
		linearIndex = rowOffset*wrapCount + colOffset
	} else {
		if rowOffset < 0 || rowOffset >= wrapCount || colOffset < 0 {
			return Error(ErrNA)
		}
		linearIndex = colOffset*wrapCount + rowOffset
	}
	if linearIndex < len(values) {
		return values[linearIndex]
	}
	return padValue
}

func toVectorArgs(args []Expression, ctx Context) ([]Value, bool, Value) {
	if len(args) < 1 || len(args) > 3 {
		return nil, false, Error(ErrValue)
	}
	ignoreMode, scanByColumn, ok := linearizeArrayOptions(args[1:], ctx)
	if !ok {
		return nil, false, Error(ErrValue)
	}
	values, ok, errValue := linearizeSourceValues(args[0], ignoreMode, scanByColumn, ctx)
	if !ok {
		return nil, false, errValue
	}
	return values, true, Value{}
}

func inferLinearizeSourceShape(expr Expression) (arrayShape, bool) {
	if shape, ok := inferExpressionArrayShape(expr); ok {
		return shape, true
	}
	start, end, ok := indexTargetRange(expr)
	if !ok {
		return arrayShape{}, false
	}
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return arrayShape{}, false
	}
	return arrayShape{rows: endCoords.row - startCoords.row + 1, cols: endCoords.col - startCoords.col + 1}, true
}

func inferLinearizeSourceShapeWithContext(expr Expression, ctx Context) (arrayShape, bool) {
	if shape, ok := inferLinearizeSourceShape(expr); ok {
		return shape, true
	}
	return inferExpressionArrayShapeWithContext(expr, ctx)
}

func linearizeArrayOptions(args []Expression, ctx Context) (int, bool, bool) {
	ignoreMode := 0
	if len(args) >= 1 {
		ignoreValue, ok := indexSelectorNumber(args[0], ctx)
		if !ok || ignoreValue < 0 || ignoreValue > 3 {
			return 0, false, false
		}
		ignoreMode = ignoreValue
	}
	scanByColumn := false
	if len(args) >= 2 {
		scanValue, ok := indexSelectorNumber(args[1], ctx)
		if !ok {
			return 0, false, false
		}
		scanByColumn = scanValue != 0
	}
	return ignoreMode, scanByColumn, true
}

func linearizeSourceValues(expr Expression, ignoreMode int, scanByColumn bool, ctx Context) ([]Value, bool, Value) {
	sourceCtx := arraySourceContext(ctx)
	values := expandArgumentValues(expr, sourceCtx)
	if len(values) == 0 {
		values = []Value{Empty()}
	}
	shape, ok := inferLinearizeSourceShapeWithContext(expr, sourceCtx)
	if !ok {
		shape = arrayShape{rows: 1, cols: len(values)}
	}
	values = reorderLinearizedValues(values, shape, scanByColumn)
	values = filterLinearizedValues(values, ignoreMode)
	return values, true, Value{}
}

func wrapArgs(args []Expression, ctx Context) ([]Value, int, Value, bool, Value) {
	if len(args) < 2 || len(args) > 3 {
		return nil, 0, Value{}, false, Error(ErrValue)
	}
	wrapCount, ok := indexSelectorNumber(args[1], ctx)
	if !ok || wrapCount <= 0 {
		return nil, 0, Value{}, false, Error(ErrValue)
	}
	padValue := Error(ErrNA)
	if len(args) == 3 {
		padValue = args[2].Eval(ctx)
	}
	values, ok, errValue := linearizeSourceValues(args[0], 0, false, ctx)
	if !ok {
		return nil, 0, Value{}, false, errValue
	}
	return values, wrapCount, padValue, true, Value{}
}

func uniqueArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 1 || len(args) > 3 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceCtx := arraySourceContext(ctx)
	arrayValues := expandArgumentValues(args[0], sourceCtx)
	shape, ok := inferExpressionArrayShapeWithContext(args[0], sourceCtx)
	if !ok {
		if len(arrayValues) == 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		shape = arrayShape{rows: 1, cols: len(arrayValues)}
	}
	if shape.rows <= 0 || shape.cols <= 0 || len(arrayValues) != shape.rows*shape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	byColumn, exactlyOnce, ok, errValue := uniqueOptions(args[1:], ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	axis := uniqueSelectionAxis(shape, byColumn)
	selectors := uniqueSelectors(arrayValues, shape, axis, exactlyOnce)
	if len(selectors) == 0 {
		return []Value{Error(ErrNA)}, arrayShape{rows: 1, cols: 1}, true, Value{}
	}
	if axis == filterAxisRows {
		filtered := make([]Value, 0, len(selectors)*shape.cols)
		for _, rowIndex := range selectors {
			start := rowIndex * shape.cols
			filtered = append(filtered, arrayValues[start:start+shape.cols]...)
		}
		return filtered, arrayShape{rows: len(selectors), cols: shape.cols}, true, Value{}
	}
	filtered := make([]Value, 0, shape.rows*len(selectors))
	for row := 0; row < shape.rows; row++ {
		base := row * shape.cols
		for _, colIndex := range selectors {
			filtered = append(filtered, arrayValues[base+colIndex])
		}
	}
	return filtered, arrayShape{rows: shape.rows, cols: len(selectors)}, true, Value{}
}

func sortArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 1 || len(args) > 4 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceCtx := arraySourceContext(ctx)
	arrayValues := expandArgumentValues(args[0], sourceCtx)
	shape, ok := inferExpressionArrayShapeWithContext(args[0], sourceCtx)
	if !ok {
		if len(arrayValues) == 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		shape = arrayShape{rows: 1, cols: len(arrayValues)}
	}
	if shape.rows <= 0 || shape.cols <= 0 || len(arrayValues) != shape.rows*shape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sortIndex := 1
	if len(args) >= 2 {
		value, ok := indexSelectorNumber(args[1], ctx)
		if !ok || value == 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		sortIndex = value
	}
	sortOrder := 1
	if len(args) >= 3 {
		value, ok := indexSelectorNumber(args[2], ctx)
		if !ok || (value != 1 && value != -1) {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		sortOrder = value
	}
	byColumn := false
	if len(args) == 4 {
		value := args[3].Eval(ctx)
		if value.IsError() {
			return nil, arrayShape{}, false, value
		}
		boolValue, err := value.ToBool()
		if err != nil {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		byColumn = boolValue
	}
	axis := sortSelectionAxis(shape, byColumn)
	keyLimit := shape.cols
	selectorCount := shape.rows
	if axis == filterAxisCols {
		keyLimit = shape.rows
		selectorCount = shape.cols
	}
	if sortIndex < 1 || sortIndex > keyLimit {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	selectors := make([]int, selectorCount)
	for index := range selectors {
		selectors[index] = index
		if key := sortKeyValue(arrayValues, shape, axis, index, sortIndex-1); key.IsError() {
			return nil, arrayShape{}, false, key
		}
	}
	sort.SliceStable(selectors, func(i, j int) bool {
		left := sortKeyValue(arrayValues, shape, axis, selectors[i], sortIndex-1)
		right := sortKeyValue(arrayValues, shape, axis, selectors[j], sortIndex-1)
		if sortOrder < 0 {
			left, right = right, left
		}
		return valueLessThan(left, right)
	})
	if axis == filterAxisRows {
		filtered := make([]Value, 0, len(arrayValues))
		for _, rowIndex := range selectors {
			start := rowIndex * shape.cols
			filtered = append(filtered, arrayValues[start:start+shape.cols]...)
		}
		return filtered, shape, true, Value{}
	}
	filtered := make([]Value, 0, len(arrayValues))
	for row := 0; row < shape.rows; row++ {
		base := row * shape.cols
		for _, colIndex := range selectors {
			filtered = append(filtered, arrayValues[base+colIndex])
		}
	}
	return filtered, shape, true, Value{}
}

func sortByArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 2 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceCtx := arraySourceContext(ctx)
	arrayValues := expandArgumentValues(args[0], sourceCtx)
	shape, ok := inferExpressionArrayShapeWithContext(args[0], sourceCtx)
	if !ok {
		if len(arrayValues) == 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		shape = arrayShape{rows: 1, cols: len(arrayValues)}
	}
	if shape.rows <= 0 || shape.cols <= 0 || len(arrayValues) != shape.rows*shape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	keys, axis, ok, errValue := sortByKeys(args[1:], shape, sourceCtx, ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	selectorCount := shape.rows
	if axis == filterAxisCols {
		selectorCount = shape.cols
	}
	selectors := make([]int, selectorCount)
	for index := range selectors {
		selectors[index] = index
	}
	sort.SliceStable(selectors, func(i, j int) bool {
		leftIndex := selectors[i]
		rightIndex := selectors[j]
		for _, key := range keys {
			left := key.values[leftIndex]
			right := key.values[rightIndex]
			if valuesEqual(left, right) {
				continue
			}
			if key.order < 0 {
				left, right = right, left
			}
			return valueLessThan(left, right)
		}
		return false
	})
	if axis == filterAxisRows {
		filtered := make([]Value, 0, len(arrayValues))
		for _, rowIndex := range selectors {
			start := rowIndex * shape.cols
			filtered = append(filtered, arrayValues[start:start+shape.cols]...)
		}
		return filtered, shape, true, Value{}
	}
	filtered := make([]Value, 0, len(arrayValues))
	for row := 0; row < shape.rows; row++ {
		base := row * shape.cols
		for _, colIndex := range selectors {
			filtered = append(filtered, arrayValues[base+colIndex])
		}
	}
	return filtered, shape, true, Value{}
}

type sortByKey struct {
	values []Value
	order  int
}

func sequenceArgs(args []Expression, ctx Context) (int, int, float64, float64, bool, Value) {
	if len(args) < 1 || len(args) > 4 {
		return 0, 0, 0, 0, false, Error(ErrValue)
	}
	rows, ok := indexSelectorNumber(args[0], ctx)
	if !ok || rows <= 0 {
		return 0, 0, 0, 0, false, Error(ErrValue)
	}
	cols := 1
	if len(args) >= 2 {
		cols, ok = indexSelectorNumber(args[1], ctx)
		if !ok || cols <= 0 {
			return 0, 0, 0, 0, false, Error(ErrValue)
		}
	}
	start := 1.0
	if len(args) >= 3 {
		value := args[2].Eval(ctx)
		if value.IsError() {
			return 0, 0, 0, 0, false, value
		}
		num, err := value.ToNumber()
		if err != nil {
			return 0, 0, 0, 0, false, Error(ErrValue)
		}
		start = num
	}
	step := 1.0
	if len(args) >= 4 {
		value := args[3].Eval(ctx)
		if value.IsError() {
			return 0, 0, 0, 0, false, value
		}
		num, err := value.ToNumber()
		if err != nil {
			return 0, 0, 0, 0, false, Error(ErrValue)
		}
		step = num
	}
	return rows, cols, start, step, true, Value{}
}

func randArrayArgs(args []Expression, ctx Context) (int, int, float64, float64, bool, bool, Value) {
	if len(args) > 5 {
		return 0, 0, 0, 0, false, false, Error(ErrValue)
	}
	rows := 1
	cols := 1
	var ok bool
	if len(args) >= 1 {
		rows, ok = indexSelectorNumber(args[0], ctx)
		if !ok || rows <= 0 {
			return 0, 0, 0, 0, false, false, Error(ErrValue)
		}
	}
	if len(args) >= 2 {
		cols, ok = indexSelectorNumber(args[1], ctx)
		if !ok || cols <= 0 {
			return 0, 0, 0, 0, false, false, Error(ErrValue)
		}
	}
	minValue := 0.0
	if len(args) >= 3 {
		value := args[2].Eval(ctx)
		if value.IsError() {
			return 0, 0, 0, 0, false, false, value
		}
		num, err := value.ToNumber()
		if err != nil {
			return 0, 0, 0, 0, false, false, Error(ErrValue)
		}
		minValue = num
	}
	maxValue := 1.0
	if len(args) >= 4 {
		value := args[3].Eval(ctx)
		if value.IsError() {
			return 0, 0, 0, 0, false, false, value
		}
		num, err := value.ToNumber()
		if err != nil {
			return 0, 0, 0, 0, false, false, Error(ErrValue)
		}
		maxValue = num
	}
	wholeNumber := false
	if len(args) >= 5 {
		value := args[4].Eval(ctx)
		if value.IsError() {
			return 0, 0, 0, 0, false, false, value
		}
		boolValue, err := value.ToBool()
		if err != nil {
			return 0, 0, 0, 0, false, false, Error(ErrValue)
		}
		wholeNumber = boolValue
	}
	return rows, cols, minValue, maxValue, wholeNumber, true, Value{}
}

func expandArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 2 || len(args) > 4 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceValues, sourceShape, ok, errValue := stackSourceValues(args[0], ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	rows, ok := indexSelectorNumber(args[1], ctx)
	if !ok || rows <= 0 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	cols := sourceShape.cols
	if len(args) >= 3 {
		cols, ok = indexSelectorNumber(args[2], ctx)
		if !ok || cols <= 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
	}
	if rows < sourceShape.rows || cols < sourceShape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	padValue := Error(ErrNA)
	if len(args) == 4 {
		padValue = args[3].Eval(ctx)
		if padValue.IsError() && padValue.Err != ErrNA {
			return nil, arrayShape{}, false, padValue
		}
	}
	shape := arrayShape{rows: rows, cols: cols}
	values := make([]Value, rows*cols)
	for index := range values {
		values[index] = padValue
	}
	for row := 0; row < sourceShape.rows; row++ {
		copy(values[row*cols:row*cols+sourceShape.cols], sourceValues[row*sourceShape.cols:(row+1)*sourceShape.cols])
	}
	return values, shape, true, Value{}
}

func makeArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) != 3 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	rows, ok := indexSelectorNumber(args[0], ctx)
	if !ok || rows <= 0 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	cols, ok := indexSelectorNumber(args[1], ctx)
	if !ok || cols <= 0 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	lambda, ok, errValue := parseLambda(args[2], 2, 2)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	shape := arrayShape{rows: rows, cols: cols}
	values := make([]Value, 0, rows*cols)
	anchorRef, anchorRow, anchorCol := arrayEvalAnchor(ctx)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			bindings := map[string]lambdaBinding{
				lambda.params[0]: scalarLambdaBinding(Number(float64(row + 1))),
				lambda.params[1]: scalarLambdaBinding(Number(float64(col + 1))),
			}
			value := evaluateScalarLambda(lambda, bindings, arrayCurrentRef(anchorRow, anchorCol, row, col), anchorRef, ctx)
			if value.IsError() {
				return nil, arrayShape{}, false, value
			}
			values = append(values, value)
		}
	}
	return values, shape, true, Value{}
}

func byAxisLambdaResult(args []Expression, byRow bool, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) != 2 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceValues, sourceShape, ok, errValue := stackSourceValues(args[0], ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	lambda, ok, errValue := parseLambda(args[1], 1, 1)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	shape := arrayShape{rows: sourceShape.rows, cols: 1}
	count := sourceShape.rows
	if !byRow {
		shape = arrayShape{rows: 1, cols: sourceShape.cols}
		count = sourceShape.cols
	}
	values := make([]Value, 0, count)
	anchorRef, anchorRow, anchorCol := arrayEvalAnchor(ctx)
	for index := 0; index < count; index++ {
		bindingValues := make([]Value, 0)
		bindingShape := arrayShape{rows: 1, cols: sourceShape.cols}
		currentRef := arrayCurrentRef(anchorRow, anchorCol, index, 0)
		if !byRow {
			bindingShape = arrayShape{rows: sourceShape.rows, cols: 1}
			currentRef = arrayCurrentRef(anchorRow, anchorCol, 0, index)
		}
		if byRow {
			start := index * sourceShape.cols
			bindingValues = append(bindingValues, sourceValues[start:start+sourceShape.cols]...)
		} else {
			for row := 0; row < sourceShape.rows; row++ {
				bindingValues = append(bindingValues, sourceValues[row*sourceShape.cols+index])
			}
		}
		bindings := map[string]lambdaBinding{lambda.params[0]: normalizedLambdaBinding(bindingValues, bindingShape)}
		value := evaluateScalarLambda(lambda, bindings, currentRef, anchorRef, ctx)
		if value.IsError() {
			return nil, arrayShape{}, false, value
		}
		values = append(values, value)
	}
	return values, shape, true, Value{}
}

func mapLambdaResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 2 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	lambda, ok, errValue := parseLambda(args[len(args)-1], len(args)-1, len(args)-1)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	type lambdaSource struct {
		values []Value
		shape  arrayShape
	}
	sources := make([]lambdaSource, 0, len(args)-1)
	shapes := make([]arrayShape, 0, len(args)-1)
	for _, arg := range args[:len(args)-1] {
		values, shape, ok, errValue := stackSourceValues(arg, ctx)
		if !ok {
			return nil, arrayShape{}, false, errValue
		}
		sources = append(sources, lambdaSource{values: values, shape: shape})
		shapes = append(shapes, shape)
	}
	targetShape, ok := broadcastArrayShapes(shapes...)
	if !ok {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	values := make([]Value, 0, targetShape.rows*targetShape.cols)
	anchorRef, anchorRow, anchorCol := arrayEvalAnchor(ctx)
	for row := 0; row < targetShape.rows; row++ {
		for col := 0; col < targetShape.cols; col++ {
			bindings := make(map[string]lambdaBinding, len(lambda.params))
			for index, source := range sources {
				value, ok := broadcastSourceValue(source.values, source.shape, row, col)
				if !ok {
					return nil, arrayShape{}, false, Error(ErrValue)
				}
				bindings[lambda.params[index]] = scalarLambdaBinding(value)
			}
			value := evaluateScalarLambda(lambda, bindings, arrayCurrentRef(anchorRow, anchorCol, row, col), anchorRef, ctx)
			if value.IsError() {
				return nil, arrayShape{}, false, value
			}
			values = append(values, value)
		}
	}
	return values, targetShape, true, Value{}
}

func reduceLambdaValue(args []Expression, ctx Context) (Value, bool, Value) {
	if len(args) != 3 {
		return Value{}, false, Error(ErrValue)
	}
	accumulator := args[0].Eval(ctx)
	if accumulator.IsError() {
		return Value{}, false, accumulator
	}
	values, _, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return Value{}, false, errValue
	}
	lambda, ok, errValue := parseLambda(args[2], 2, 2)
	if !ok {
		return Value{}, false, errValue
	}
	for _, value := range values {
		result := evaluateScalarLambda(lambda, map[string]lambdaBinding{
			lambda.params[0]: scalarLambdaBinding(accumulator),
			lambda.params[1]: scalarLambdaBinding(value),
		}, ctx.CurrentRef(), ctx.ArrayAnchorRef(), ctx)
		if result.IsError() {
			return Value{}, false, result
		}
		accumulator = result
	}
	return accumulator, true, Value{}
}

func scanLambdaResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) != 3 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	accumulator := args[0].Eval(ctx)
	if accumulator.IsError() {
		return nil, arrayShape{}, false, accumulator
	}
	inputValues, shape, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	lambda, ok, errValue := parseLambda(args[2], 2, 2)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	values := make([]Value, 0, len(inputValues))
	anchorRef, anchorRow, anchorCol := arrayEvalAnchor(ctx)
	for index, inputValue := range inputValues {
		result := evaluateScalarLambda(lambda, map[string]lambdaBinding{
			lambda.params[0]: scalarLambdaBinding(accumulator),
			lambda.params[1]: scalarLambdaBinding(inputValue),
		}, arrayCurrentRef(anchorRow, anchorCol, index/shape.cols, index%shape.cols), anchorRef, ctx)
		if result.IsError() {
			return nil, arrayShape{}, false, result
		}
		accumulator = result
		values = append(values, result)
	}
	return values, shape, true, Value{}
}

func broadcastSourceValue(values []Value, shape arrayShape, row, col int) (Value, bool) {
	targetRow := row
	if shape.rows == 1 {
		targetRow = 0
	}
	targetCol := col
	if shape.cols == 1 {
		targetCol = 0
	}
	index := targetRow*shape.cols + targetCol
	if index < 0 || index >= len(values) {
		return Value{}, false
	}
	return values[index], true
}

func matchArgs(args []Expression, ctx Context) (Value, []Value, int, int, bool, Value) {
	if len(args) < 2 || len(args) > 3 {
		return Value{}, nil, 0, 0, false, Error(ErrValue)
	}
	lookupValue := expressionValueAtCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return Value{}, nil, 0, 0, false, lookupValue
	}
	lookupArray := expandArgumentValues(args[1], ctx)
	if len(lookupArray) == 0 {
		return Value{}, nil, 0, 0, false, Error(ErrNA)
	}
	matchType := 1
	if len(args) == 3 {
		value, ok := indexSelectorNumberAtCurrentOffset(args[2], ctx)
		if !ok || (value != -1 && value != 0 && value != 1) {
			return Value{}, nil, 0, 0, false, Error(ErrValue)
		}
		matchType = value
	}
	switch matchType {
	case -1:
		return lookupValue, lookupArray, 1, -2, true, Value{}
	case 0:
		matchMode := 0
		if hasLookupWildcard(lookupValue) {
			matchMode = 2
		}
		return lookupValue, lookupArray, matchMode, 1, true, Value{}
	default:
		return lookupValue, lookupArray, -1, 2, true, Value{}
	}
}

func lookupArgs(args []Expression, ctx Context) (Value, []Value, []Value, bool, Value) {
	if len(args) < 2 || len(args) > 3 {
		return Value{}, nil, nil, false, Error(ErrValue)
	}
	lookupValue := expressionValueAtCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return Value{}, nil, nil, false, lookupValue
	}
	if len(args) == 3 {
		lookupArray := expandArgumentValues(args[1], ctx)
		returnArray := expandArgumentValues(args[2], ctx)
		if len(lookupArray) == 0 || len(returnArray) == 0 || len(lookupArray) != len(returnArray) {
			return Value{}, nil, nil, false, Error(ErrValue)
		}
		return lookupValue, lookupArray, returnArray, true, Value{}
	}
	values, shape, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return Value{}, nil, nil, false, errValue
	}
	if shape.rows == 1 || shape.cols == 1 {
		return lookupValue, values, values, true, Value{}
	}
	if shape.rows <= shape.cols {
		lookupArray := make([]Value, 0, shape.cols)
		returnArray := make([]Value, 0, shape.cols)
		lastRowBase := (shape.rows - 1) * shape.cols
		for col := 0; col < shape.cols; col++ {
			lookupArray = append(lookupArray, values[col])
			returnArray = append(returnArray, values[lastRowBase+col])
		}
		return lookupValue, lookupArray, returnArray, true, Value{}
	}
	lookupArray := make([]Value, 0, shape.rows)
	returnArray := make([]Value, 0, shape.rows)
	lastCol := shape.cols - 1
	for row := 0; row < shape.rows; row++ {
		base := row * shape.cols
		lookupArray = append(lookupArray, values[base])
		returnArray = append(returnArray, values[base+lastCol])
	}
	return lookupValue, lookupArray, returnArray, true, Value{}
}

func xmatchArgs(args []Expression, ctx Context) (Value, []Value, int, int, bool, Value) {
	if len(args) < 2 || len(args) > 4 {
		return Value{}, nil, 0, 0, false, Error(ErrValue)
	}
	lookupValue := expressionValueAtCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return Value{}, nil, 0, 0, false, lookupValue
	}
	lookupArray := expandArgumentValues(args[1], ctx)
	if len(lookupArray) == 0 {
		return Value{}, nil, 0, 0, false, Error(ErrNA)
	}
	matchMode, searchMode, ok, errValue := xmatchModes(args, 2, 3, ctx)
	if !ok {
		return Value{}, nil, 0, 0, false, errValue
	}
	return lookupValue, lookupArray, matchMode, searchMode, true, Value{}
}

func xlookupArgs(args []Expression, ctx Context) (Value, []Value, []Value, Value, int, int, bool, Value) {
	if len(args) < 3 || len(args) > 6 {
		return Value{}, nil, nil, Value{}, 0, 0, false, Error(ErrValue)
	}
	lookupValue := expressionValueAtProjectedCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return Value{}, nil, nil, Value{}, 0, 0, false, lookupValue
	}
	lookupArray, lookupShape, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return Value{}, nil, nil, Value{}, 0, 0, false, errValue
	}
	if (lookupShape.rows != 1 && lookupShape.cols != 1) || (lookupShape.rows == 1 && lookupShape.cols == 1) {
		return Value{}, nil, nil, Value{}, 0, 0, false, Error(ErrValue)
	}
	returnArray, returnShape, ok, errValue := stackSourceValues(args[2], ctx)
	if !ok {
		return Value{}, nil, nil, Value{}, 0, 0, false, errValue
	}
	if lookupShape.cols == 1 {
		if returnShape.rows != lookupShape.rows {
			return Value{}, nil, nil, Value{}, 0, 0, false, Error(ErrValue)
		}
	} else {
		if returnShape.cols != lookupShape.cols {
			return Value{}, nil, nil, Value{}, 0, 0, false, Error(ErrValue)
		}
	}
	ifNotFound := Empty()
	if len(args) >= 4 {
		ifNotFound = expressionValueAtProjectedCurrentOffset(args[3], ctx)
		if ifNotFound.IsError() {
			return Value{}, nil, nil, Value{}, 0, 0, false, ifNotFound
		}
	}
	matchMode, searchMode, ok, errValue := xmatchModes(args, 4, 5, ctx)
	if !ok {
		return Value{}, nil, nil, Value{}, 0, 0, false, errValue
	}
	if lookupShape.cols == 1 {
		lookupValues := make([]Value, lookupShape.rows)
		for row := 0; row < lookupShape.rows; row++ {
			lookupValues[row] = lookupArray[row*lookupShape.cols]
		}
		return lookupValue, lookupValues, returnArray, ifNotFound, matchMode, searchMode, true, Value{}
	}
	lookupValues := make([]Value, lookupShape.cols)
	for col := 0; col < lookupShape.cols; col++ {
		lookupValues[col] = lookupArray[col]
	}
	return lookupValue, lookupValues, returnArray, ifNotFound, matchMode, searchMode, true, Value{}
}

func xmatchModes(args []Expression, matchModeIndex, searchModeIndex int, ctx Context) (int, int, bool, Value) {
	matchMode := 0
	if len(args) > matchModeIndex {
		value, ok := indexSelectorValueNumber(expressionValueAtProjectedCurrentOffset(args[matchModeIndex], ctx))
		if !ok || (value != -1 && value != 0 && value != 1 && value != 2) {
			return 0, 0, false, Error(ErrValue)
		}
		matchMode = value
	}
	searchMode := 1
	if len(args) > searchModeIndex {
		value, ok := indexSelectorValueNumber(expressionValueAtProjectedCurrentOffset(args[searchModeIndex], ctx))
		if !ok || (value != -2 && value != -1 && value != 1 && value != 2) {
			return 0, 0, false, Error(ErrValue)
		}
		searchMode = value
	}
	return matchMode, searchMode, true, Value{}
}

func xmatchIndex(lookupValue Value, lookupArray []Value, matchMode, searchMode int) (int, Value, bool) {
	if searchMode == 2 || searchMode == -2 {
		return xmatchBinaryIndex(lookupValue, lookupArray, matchMode, searchMode)
	}
	return xmatchLinearIndex(lookupValue, lookupArray, matchMode, searchMode)
}

func xmatchLinearIndex(lookupValue Value, lookupArray []Value, matchMode, searchMode int) (int, Value, bool) {
	bestIndex := -1
	for _, idx := range xmatchSearchOrder(len(lookupArray), searchMode) {
		candidate := lookupArray[idx]
		if candidate.IsError() {
			return -1, candidate, false
		}
		if lookupValuesMatch(candidate, lookupValue, matchMode) {
			return idx, Value{}, true
		}
		switch matchMode {
		case -1:
			if lookupValueLessThan(candidate, lookupValue) && (bestIndex < 0 || lookupValueLessThan(lookupArray[bestIndex], candidate)) {
				bestIndex = idx
			}
		case 1:
			if lookupValueLessThan(lookupValue, candidate) && (bestIndex < 0 || lookupValueLessThan(candidate, lookupArray[bestIndex])) {
				bestIndex = idx
			}
		}
	}
	if bestIndex >= 0 {
		return bestIndex, Value{}, true
	}
	return -1, Error(ErrNA), false
}

func xmatchBinaryIndex(lookupValue Value, lookupArray []Value, matchMode, searchMode int) (int, Value, bool) {
	if matchMode == 2 {
		return xmatchLinearIndex(lookupValue, lookupArray, matchMode, binarySearchDirection(searchMode))
	}
	low := 0
	high := len(lookupArray) - 1
	bestIndex := -1
	ascending := searchMode == 2
	for low <= high {
		mid := low + (high-low)/2
		candidate := lookupArray[mid]
		if candidate.IsError() {
			return -1, candidate, false
		}
		cmp, ok := lookupCompare(candidate, lookupValue)
		if !ok {
			return xmatchLinearIndex(lookupValue, lookupArray, matchMode, binarySearchDirection(searchMode))
		}
		if cmp == 0 {
			return mid, Value{}, true
		}
		if cmp < 0 && matchMode == -1 {
			bestIndex = mid
		}
		if cmp > 0 && matchMode == 1 {
			bestIndex = mid
		}
		orderedCmp := cmp
		if !ascending {
			orderedCmp = -orderedCmp
		}
		if orderedCmp < 0 {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if bestIndex >= 0 {
		return bestIndex, Value{}, true
	}
	return -1, Error(ErrNA), false
}

func xmatchSearchOrder(length, searchMode int) []int {
	order := make([]int, 0, length)
	if searchMode < 0 {
		for idx := length - 1; idx >= 0; idx-- {
			order = append(order, idx)
		}
		return order
	}
	for idx := 0; idx < length; idx++ {
		order = append(order, idx)
	}
	return order
}

type stackSection struct {
	values []Value
	shape  arrayShape
	row    int
	col    int
}

func stackValue(args []Expression, horizontal bool, ctx Context) Value {
	values, shape, ok, errValue := stackArrayResult(args, horizontal, ctx)
	if !ok {
		return errValue
	}
	if len(values) == 1 {
		return values[0]
	}
	currentRef := ctx.CurrentRef()
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = currentRef
	}
	currentRow, currentCol := parseCellRef(currentRef)
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return Error(ErrNA)
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if rowOffset < 0 || rowOffset >= shape.rows || colOffset < 0 || colOffset >= shape.cols {
		return Error(ErrNA)
	}
	index := rowOffset*shape.cols + colOffset
	if index < 0 || index >= len(values) {
		return Error(ErrNA)
	}
	return values[index]
}

func stackArrayResult(args []Expression, horizontal bool, ctx Context) ([]Value, arrayShape, bool, Value) {
	sections, shape, ok, errValue := stackSections(args, horizontal, ctx)
	if !ok {
		return nil, arrayShape{}, false, errValue
	}
	values := make([]Value, shape.rows*shape.cols)
	for index := range values {
		values[index] = Error(ErrNA)
	}
	for _, section := range sections {
		for row := 0; row < section.shape.rows; row++ {
			for col := 0; col < section.shape.cols; col++ {
				targetRow := section.row + row
				targetCol := section.col + col
				targetIndex := targetRow*shape.cols + targetCol
				sourceIndex := row*section.shape.cols + col
				values[targetIndex] = section.values[sourceIndex]
			}
		}
	}
	return values, shape, true, Value{}
}

func stackSections(args []Expression, horizontal bool, ctx Context) ([]stackSection, arrayShape, bool, Value) {
	if len(args) == 0 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sections := make([]stackSection, 0, len(args))
	totalRows := 0
	totalCols := 0
	offsetRow := 0
	offsetCol := 0
	for _, arg := range args {
		values, shape, ok, errValue := stackSourceValues(arg, ctx)
		if !ok {
			return nil, arrayShape{}, false, errValue
		}
		sections = append(sections, stackSection{values: values, shape: shape, row: offsetRow, col: offsetCol})
		if horizontal {
			if shape.rows > totalRows {
				totalRows = shape.rows
			}
			totalCols += shape.cols
			offsetCol += shape.cols
		} else {
			totalRows += shape.rows
			if shape.cols > totalCols {
				totalCols = shape.cols
			}
			offsetRow += shape.rows
		}
	}
	if totalRows <= 0 || totalCols <= 0 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	return sections, arrayShape{rows: totalRows, cols: totalCols}, true, Value{}
}

func stackSourceValues(expr Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(expr, sourceCtx))
	shape, ok := inferExpressionArrayShapeWithContext(expr, sourceCtx)
	if !ok {
		shape = arrayShape{rows: 1, cols: len(values)}
		if len(values) == 1 {
			shape = arrayShape{rows: 1, cols: 1}
		}
	}
	if shape.rows <= 0 || shape.cols <= 0 || len(values) != shape.rows*shape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	return values, shape, true, Value{}
}

func binarySearchDirection(searchMode int) int {
	if searchMode < 0 {
		return -1
	}
	return 1
}

func lookupValuesMatch(candidate, lookupValue Value, matchMode int) bool {
	if matchMode == 2 {
		return lookupWildcardMatch(candidate, lookupValue)
	}
	return lookupValuesEqual(candidate, lookupValue)
}

func lookupValuesEqual(left, right Value) bool {
	if left.Type == TypeText && right.Type == TypeText {
		return strings.EqualFold(left.Text, right.Text)
	}
	return valuesEqual(left, right)
}

func lookupValueLessThan(left, right Value) bool {
	cmp, ok := lookupCompare(left, right)
	return ok && cmp < 0
}

func lookupCompare(left, right Value) (int, bool) {
	if left.Type == TypeText && right.Type == TypeText {
		cmp := strings.Compare(strings.ToUpper(left.Text), strings.ToUpper(right.Text))
		switch {
		case cmp < 0:
			return -1, true
		case cmp > 0:
			return 1, true
		default:
			return 0, true
		}
	}
	leftNum, leftErr := left.ToNumber()
	rightNum, rightErr := right.ToNumber()
	if leftErr == nil && rightErr == nil {
		switch {
		case leftNum < rightNum:
			return -1, true
		case leftNum > rightNum:
			return 1, true
		default:
			return 0, true
		}
	}
	if valuesEqual(left, right) {
		return 0, true
	}
	return 0, false
}

func hasLookupWildcard(value Value) bool {
	if !value.IsText() {
		return false
	}
	escaped := false
	for _, ch := range value.Text {
		if escaped {
			escaped = false
			continue
		}
		if ch == '~' {
			escaped = true
			continue
		}
		if ch == '*' || ch == '?' {
			return true
		}
	}
	return false
}

func lookupWildcardMatch(candidate, lookupValue Value) bool {
	if !candidate.IsText() || !lookupValue.IsText() {
		return lookupValuesEqual(candidate, lookupValue)
	}
	return wildcardMatch(lookupValue.Text, candidate.Text)
}

func wildcardMatch(pattern, text string) bool {
	patternRunes := []rune(strings.ToUpper(pattern))
	textRunes := []rune(strings.ToUpper(text))
	type wildcardState struct {
		patternIndex int
		textIndex    int
	}
	seen := make(map[wildcardState]bool)
	var match func(int, int) bool
	match = func(patternIndex, textIndex int) bool {
		state := wildcardState{patternIndex: patternIndex, textIndex: textIndex}
		if seen[state] {
			return false
		}
		seen[state] = true
		for patternIndex < len(patternRunes) {
			ch := patternRunes[patternIndex]
			switch ch {
			case '~':
				patternIndex++
				if patternIndex >= len(patternRunes) {
					return textIndex == len(textRunes)
				}
				if textIndex >= len(textRunes) || patternRunes[patternIndex] != textRunes[textIndex] {
					return false
				}
				patternIndex++
				textIndex++
			case '?':
				if textIndex >= len(textRunes) {
					return false
				}
				patternIndex++
				textIndex++
			case '*':
				for skip := textIndex; skip <= len(textRunes); skip++ {
					if match(patternIndex+1, skip) {
						return true
					}
				}
				return false
			default:
				if textIndex >= len(textRunes) || ch != textRunes[textIndex] {
					return false
				}
				patternIndex++
				textIndex++
			}
		}
		return textIndex == len(textRunes)
	}
	return match(0, 0)
}

func uniqueOptions(args []Expression, ctx Context) (bool, bool, bool, Value) {
	byColumn := false
	if len(args) >= 1 {
		value := args[0].Eval(ctx)
		if value.IsError() {
			return false, false, false, value
		}
		boolValue, err := value.ToBool()
		if err != nil {
			return false, false, false, Error(ErrValue)
		}
		byColumn = boolValue
	}
	exactlyOnce := false
	if len(args) >= 2 {
		value := args[1].Eval(ctx)
		if value.IsError() {
			return false, false, false, value
		}
		boolValue, err := value.ToBool()
		if err != nil {
			return false, false, false, Error(ErrValue)
		}
		exactlyOnce = boolValue
	}
	return byColumn, exactlyOnce, true, Value{}
}

func uniqueSelectionAxis(shape arrayShape, byColumn bool) filterAxis {
	if byColumn || (shape.rows == 1 && shape.cols > 1) {
		return filterAxisCols
	}
	return filterAxisRows
}

func sortSelectionAxis(shape arrayShape, byColumn bool) filterAxis {
	if byColumn || (shape.rows == 1 && shape.cols > 1) {
		return filterAxisCols
	}
	return filterAxisRows
}

func sortByKeys(args []Expression, sourceShape arrayShape, sourceCtx Context, ctx Context) ([]sortByKey, filterAxis, bool, Value) {
	if len(args) == 0 {
		return nil, filterAxisRows, false, Error(ErrValue)
	}
	keys := make([]sortByKey, 0, len(args))
	axis := filterAxisRows
	axisSet := false
	for index := 0; index < len(args); index++ {
		keyValues, keyAxis, ok, errValue := sortByKeyValues(args[index], sourceShape, sourceCtx)
		if !ok {
			return nil, filterAxisRows, false, errValue
		}
		if !axisSet {
			axis = keyAxis
			axisSet = true
		} else if keyAxis != axis {
			return nil, filterAxisRows, false, Error(ErrValue)
		}
		order := 1
		if index+1 < len(args) {
			if _, _, nextIsKey, _ := sortByKeyValues(args[index+1], sourceShape, sourceCtx); !nextIsKey {
				value, ok := indexSelectorNumber(args[index+1], ctx)
				if !ok || (value != 1 && value != -1) {
					return nil, filterAxisRows, false, Error(ErrValue)
				}
				order = value
				index++
			}
		}
		keys = append(keys, sortByKey{values: keyValues, order: order})
	}
	if len(keys) == 0 {
		return nil, filterAxisRows, false, Error(ErrValue)
	}
	return keys, axis, true, Value{}
}

func sortByKeyValues(expr Expression, sourceShape arrayShape, ctx Context) ([]Value, filterAxis, bool, Value) {
	keyValues := expandArgumentValues(expr, ctx)
	if len(keyValues) == 0 {
		return nil, filterAxisRows, false, Error(ErrValue)
	}
	keyShape, ok := inferExpressionArrayShapeWithContext(expr, ctx)
	if !ok {
		if len(keyValues) == 1 {
			return nil, filterAxisRows, false, Error(ErrValue)
		}
		keyShape = arrayShape{rows: 1, cols: len(keyValues)}
	}
	axis, count, ok := sortByKeyAxis(sourceShape, keyShape, len(keyValues))
	if !ok || count != len(keyValues) {
		return nil, filterAxisRows, false, Error(ErrValue)
	}
	for _, value := range keyValues {
		if value.IsError() {
			return nil, filterAxisRows, false, value
		}
	}
	return keyValues, axis, true, Value{}
}

func sortByKeyAxis(sourceShape, keyShape arrayShape, keyCount int) (filterAxis, int, bool) {
	if keyShape.rows == sourceShape.rows && keyShape.cols == 1 {
		return filterAxisRows, sourceShape.rows, true
	}
	if keyShape.rows == 1 && keyShape.cols == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	if sourceShape.cols == 1 && keyShape.rows == sourceShape.rows && keyShape.cols == sourceShape.cols {
		return filterAxisRows, sourceShape.rows, true
	}
	if sourceShape.rows == 1 && keyShape.rows == sourceShape.rows && keyShape.cols == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	if sourceShape.rows != sourceShape.cols {
		if keyCount == sourceShape.rows {
			return filterAxisRows, sourceShape.rows, true
		}
		if keyCount == sourceShape.cols {
			return filterAxisCols, sourceShape.cols, true
		}
	}
	if sourceShape.cols == 1 && keyCount == sourceShape.rows {
		return filterAxisRows, sourceShape.rows, true
	}
	if sourceShape.rows == 1 && keyCount == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	return filterAxisRows, 0, false
}

func sortKeyValue(values []Value, shape arrayShape, axis filterAxis, selectorIndex, keyIndex int) Value {
	if axis == filterAxisRows {
		return values[selectorIndex*shape.cols+keyIndex]
	}
	return values[keyIndex*shape.cols+selectorIndex]
}

func uniqueSelectors(values []Value, shape arrayShape, axis filterAxis, exactlyOnce bool) []int {
	limit := shape.rows
	if axis == filterAxisCols {
		limit = shape.cols
	}
	selectors := make([]int, 0, limit)
	for index := 0; index < limit; index++ {
		firstMatch := index
		matchCount := 0
		for other := 0; other < limit; other++ {
			if uniqueRecordsEqual(values, shape, axis, index, other) {
				matchCount++
				if other < firstMatch {
					firstMatch = other
				}
			}
		}
		if exactlyOnce {
			if matchCount == 1 {
				selectors = append(selectors, index)
			}
			continue
		}
		if firstMatch == index {
			selectors = append(selectors, index)
		}
	}
	return selectors
}

func uniqueRecordsEqual(values []Value, shape arrayShape, axis filterAxis, leftIndex, rightIndex int) bool {
	if axis == filterAxisRows {
		leftBase := leftIndex * shape.cols
		rightBase := rightIndex * shape.cols
		for col := 0; col < shape.cols; col++ {
			if !valuesEqual(values[leftBase+col], values[rightBase+col]) {
				return false
			}
		}
		return true
	}
	for row := 0; row < shape.rows; row++ {
		leftValue := values[row*shape.cols+leftIndex]
		rightValue := values[row*shape.cols+rightIndex]
		if !valuesEqual(leftValue, rightValue) {
			return false
		}
	}
	return true
}

func filterArrayResult(args []Expression, ctx Context) ([]Value, arrayShape, bool, Value) {
	if len(args) < 2 || len(args) > 3 {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	sourceCtx := arraySourceContext(ctx)
	arrayValues := expandArgumentValues(args[0], sourceCtx)
	shape, ok := inferExpressionArrayShapeWithContext(args[0], sourceCtx)
	if !ok {
		if len(arrayValues) == 0 {
			return nil, arrayShape{}, false, Error(ErrValue)
		}
		shape = arrayShape{rows: 1, cols: len(arrayValues)}
	}
	if shape.rows <= 0 || shape.cols <= 0 || len(arrayValues) != shape.rows*shape.cols {
		return nil, arrayShape{}, false, Error(ErrValue)
	}
	includeValues := expandArgumentValues(args[1], sourceCtx)
	includeShape, includeOk := inferExpressionArrayShapeWithContext(args[1], sourceCtx)
	if !includeOk {
		includeShape = arrayShape{}
	}
	axis, selectors, selectorErr, ok := filterSelectors(shape, includeValues, includeShape)
	if !ok {
		return nil, arrayShape{}, false, selectorErr
	}
	if len(selectors) == 0 {
		if len(args) == 3 {
			fallbackValues := expandArgumentValues(args[2], sourceCtx)
			fallbackShape, ok := inferExpressionArrayShapeWithContext(args[2], sourceCtx)
			if !ok {
				if len(fallbackValues) == 0 {
					fallbackValues = []Value{Empty()}
				}
				fallbackShape = arrayShape{rows: 1, cols: len(fallbackValues)}
			}
			return fallbackValues, fallbackShape, true, Value{}
		}
		return []Value{Error(ErrNA)}, arrayShape{rows: 1, cols: 1}, true, Value{}
	}
	if axis == filterAxisRows {
		filtered := make([]Value, 0, len(selectors)*shape.cols)
		for _, rowIndex := range selectors {
			start := rowIndex * shape.cols
			filtered = append(filtered, arrayValues[start:start+shape.cols]...)
		}
		return filtered, arrayShape{rows: len(selectors), cols: shape.cols}, true, Value{}
	}
	filtered := make([]Value, 0, shape.rows*len(selectors))
	for row := 0; row < shape.rows; row++ {
		base := row * shape.cols
		for _, colIndex := range selectors {
			filtered = append(filtered, arrayValues[base+colIndex])
		}
	}
	return filtered, arrayShape{rows: shape.rows, cols: len(selectors)}, true, Value{}
}

type filterAxis int

const (
	filterAxisRows filterAxis = iota
	filterAxisCols
)

func filterSelectors(sourceShape arrayShape, includeValues []Value, includeShape arrayShape) (filterAxis, []int, Value, bool) {
	axis, count, ok := filterSelectionAxis(sourceShape, includeValues, includeShape)
	if !ok {
		return filterAxisRows, nil, Error(ErrValue), false
	}
	selectors := make([]int, 0, count)
	for index := 0; index < count; index++ {
		boolValue, errValue, ok := filterIncludeAt(includeValues, index)
		if !ok {
			return filterAxisRows, nil, errValue, false
		}
		if boolValue {
			selectors = append(selectors, index)
		}
	}
	return axis, selectors, Value{}, true
}

func filterSelectionAxis(sourceShape arrayShape, includeValues []Value, includeShape arrayShape) (filterAxis, int, bool) {
	if includeShape.rows == sourceShape.rows && includeShape.cols == 1 {
		return filterAxisRows, sourceShape.rows, true
	}
	if includeShape.rows == 1 && includeShape.cols == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	if sourceShape.cols == 1 && includeShape.rows == sourceShape.rows && includeShape.cols == sourceShape.cols {
		return filterAxisRows, sourceShape.rows, true
	}
	if sourceShape.rows == 1 && includeShape.rows == sourceShape.rows && includeShape.cols == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	if len(includeValues) == sourceShape.rows && sourceShape.rows != sourceShape.cols {
		return filterAxisRows, sourceShape.rows, true
	}
	if len(includeValues) == sourceShape.cols {
		return filterAxisCols, sourceShape.cols, true
	}
	if len(includeValues) == sourceShape.rows {
		return filterAxisRows, sourceShape.rows, true
	}
	return filterAxisRows, 0, false
}

func filterIncludeAt(includeValues []Value, index int) (bool, Value, bool) {
	if index < 0 || index >= len(includeValues) {
		return false, Error(ErrValue), false
	}
	value := includeValues[index]
	if value.IsError() {
		return false, value, false
	}
	boolValue, err := value.ToBool()
	if err != nil {
		return false, Error(ErrValue), false
	}
	return boolValue, Value{}, true
}

func arraySourceContext(ctx Context) Context {
	anchorRef := ctx.ArrayAnchorRef()
	if anchorRef == "" {
		anchorRef = ctx.CurrentRef()
	}
	if anchorRef == "" {
		return ctx
	}
	return &arrayExpansionContext{base: ctx, currentRef: anchorRef, arrayAnchorRef: anchorRef}
}

func reorderLinearizedValues(values []Value, shape arrayShape, scanByColumn bool) []Value {
	if !scanByColumn || shape.rows <= 1 || shape.cols <= 1 || shape.rows*shape.cols != len(values) {
		return values
	}
	reordered := make([]Value, 0, len(values))
	for col := 0; col < shape.cols; col++ {
		for row := 0; row < shape.rows; row++ {
			reordered = append(reordered, values[row*shape.cols+col])
		}
	}
	return reordered
}

func filterLinearizedValues(values []Value, ignoreMode int) []Value {
	if ignoreMode == 0 {
		return values
	}
	filtered := make([]Value, 0, len(values))
	for _, value := range values {
		if shouldIgnoreLinearizedValue(value, ignoreMode) {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func shouldIgnoreLinearizedValue(value Value, ignoreMode int) bool {
	switch ignoreMode {
	case 1:
		return value.IsEmpty()
	case 2:
		return value.IsError()
	case 3:
		return value.IsEmpty() || value.IsError()
	default:
		return false
	}
}

// fnAbs returns the absolute value.
func fnAbs(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	num, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	return Number(math.Abs(num))
}

// fnRound rounds a number.
func fnRound(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	num, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	digits := 0
	if len(args) == 2 {
		digitsVal := args[1].Eval(ctx)
		if digitsVal.IsError() {
			return digitsVal
		}
		d, err := digitsVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		digits = int(d)
	}

	factor := math.Pow(10, float64(digits))
	return Number(math.Round(num*factor) / factor)
}

func roundArgs(args []Expression, ctx Context) (float64, int, Value, bool) {
	if len(args) < 1 || len(args) > 2 {
		return 0, 0, Error(ErrValue), false
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return 0, 0, val, false
	}
	num, err := val.ToNumber()
	if err != nil {
		return 0, 0, Error(ErrValue), false
	}
	digits := 0
	if len(args) == 2 {
		digitsVal := args[1].Eval(ctx)
		if digitsVal.IsError() {
			return 0, 0, digitsVal, false
		}
		d, err := digitsVal.ToNumber()
		if err != nil {
			return 0, 0, Error(ErrValue), false
		}
		digits = int(d)
	}
	return num, digits, Value{}, true
}

// fnRoundUp rounds away from zero.
func fnRoundUp(args []Expression, ctx Context) Value {
	num, digits, errValue, ok := roundArgs(args, ctx)
	if !ok {
		return errValue
	}
	factor := math.Pow(10, float64(digits))
	return Number(math.Ceil(math.Abs(num)*factor) / factor * math.Copysign(1, num))
}

// fnRoundDown rounds toward zero.
func fnRoundDown(args []Expression, ctx Context) Value {
	num, digits, errValue, ok := roundArgs(args, ctx)
	if !ok {
		return errValue
	}
	factor := math.Pow(10, float64(digits))
	return Number(math.Floor(math.Abs(num)*factor) / factor * math.Copysign(1, num))
}

func significanceArgs(args []Expression, ctx Context) (float64, float64, Value, bool) {
	if len(args) != 2 {
		return 0, 0, Error(ErrValue), false
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return 0, 0, val, false
	}
	num, err := val.ToNumber()
	if err != nil {
		return 0, 0, Error(ErrValue), false
	}
	sigVal := args[1].Eval(ctx)
	if sigVal.IsError() {
		return 0, 0, sigVal, false
	}
	sig, err := sigVal.ToNumber()
	if err != nil {
		return 0, 0, Error(ErrValue), false
	}
	if sig == 0 {
		return 0, 0, Error(ErrDiv0), false
	}
	return num, math.Abs(sig), Value{}, true
}

// fnCeiling rounds up to the nearest significance.
func fnCeiling(args []Expression, ctx Context) Value {
	num, sig, errValue, ok := significanceArgs(args, ctx)
	if !ok {
		return errValue
	}
	return Number(math.Ceil(num/sig) * sig)
}

// fnFloor rounds down to the nearest significance.
func fnFloor(args []Expression, ctx Context) Value {
	num, sig, errValue, ok := significanceArgs(args, ctx)
	if !ok {
		return errValue
	}
	return Number(math.Floor(num/sig) * sig)
}

// fnInt returns the integer part of a number.
func fnInt(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	num, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	return Number(math.Trunc(num))
}

// fnMod returns the remainder of division.
func fnMod(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}

	val1 := args[0].Eval(ctx)
	if val1.IsError() {
		return val1
	}
	num1, err := val1.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	val2 := args[1].Eval(ctx)
	if val2.IsError() {
		return val2
	}
	num2, err := val2.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	if num2 == 0 {
		return Error(ErrDiv0)
	}

	return Number(math.Mod(num1, num2))
}

// fnPower raises a number to a power.
func fnPower(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}

	val1 := args[0].Eval(ctx)
	if val1.IsError() {
		return val1
	}
	num1, err := val1.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	val2 := args[1].Eval(ctx)
	if val2.IsError() {
		return val2
	}
	num2, err := val2.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	return Number(math.Pow(num1, num2))
}

// fnSqrt returns the square root.
func fnSqrt(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	num, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	if num < 0 {
		return Error(ErrNum)
	}

	return Number(math.Sqrt(num))
}

// fnTrunc truncates a number toward zero with optional digit precision.
func fnTrunc(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	digits := 0
	if len(args) == 2 {
		dv := args[1].Eval(ctx)
		if dv.IsError() {
			return dv
		}
		d, err := dv.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		digits = int(d)
	}
	scale := math.Pow(10, float64(digits))
	return Number(math.Trunc(n*scale) / scale)
}

// fnSign returns the sign of a number: -1, 0, or 1.
func fnSign(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	switch {
	case n > 0:
		return Number(1)
	case n < 0:
		return Number(-1)
	default:
		return Number(0)
	}
}

// fnLog returns the logarithm to a given base (default 10).
func fnLog(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil || n <= 0 {
		return Error(ErrNum)
	}
	base := 10.0
	if len(args) == 2 {
		bv := args[1].Eval(ctx)
		if bv.IsError() {
			return bv
		}
		b, err := bv.ToNumber()
		if err != nil || b <= 0 || b == 1 {
			return Error(ErrNum)
		}
		base = b
	}
	return Number(math.Log(n) / math.Log(base))
}

// fnLog10 returns the base-10 logarithm.
func fnLog10(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil || n <= 0 {
		return Error(ErrNum)
	}
	return Number(math.Log10(n))
}

// fnLn returns the natural logarithm.
func fnLn(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil || n <= 0 {
		return Error(ErrNum)
	}
	return Number(math.Log(n))
}

// fnExp returns e raised to the power of the argument.
func fnExp(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(math.Exp(n))
}

// fnRandBetween returns a random integer between bottom and top (inclusive).
func fnRandBetween(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	bottomVal := args[0].Eval(ctx)
	if bottomVal.IsError() {
		return bottomVal
	}
	topVal := args[1].Eval(ctx)
	if topVal.IsError() {
		return topVal
	}
	bottom, err := bottomVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	top, err := topVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if bottom > top {
		return Error(ErrNum)
	}
	lo := math.Floor(bottom)
	hi := math.Floor(top)
	return Number(lo + math.Floor(rand.Float64()*(hi-lo+1)))
}

// fnXor returns TRUE if an odd number of arguments are TRUE.
func fnXor(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	count := 0
	for _, arg := range args {
		for _, val := range expandArgumentValues(arg, ctx) {
			if val.IsError() {
				return val
			}
			b, err := val.ToBool()
			if err != nil {
				return Error(ErrValue)
			}
			if b {
				count++
			}
		}
	}
	return Bool(count%2 == 1)
}

// fnRows returns the number of rows in a range reference.
func fnRows(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	if rows, _, ok := referenceShape(args[0], ctx); ok {
		return Number(float64(rows))
	}
	if val := args[0].Eval(ctx); val.IsError() {
		return val
	}
	return Number(1)
}

// fnColumns returns the number of columns in a range reference.
func fnColumns(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	if _, cols, ok := referenceShape(args[0], ctx); ok {
		return Number(float64(cols))
	}
	if val := args[0].Eval(ctx); val.IsError() {
		return val
	}
	return Number(1)
}

// fnLen returns the length of a string.
func fnLen(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	return Number(float64(len(text)))
}

// fnLeft returns the leftmost characters.
func fnLeft(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	numChars := 1
	if len(args) == 2 {
		numVal := args[1].Eval(ctx)
		if numVal.IsError() {
			return numVal
		}
		n, err := numVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		numChars = int(n)
		if numChars < 0 {
			return Error(ErrValue)
		}
	}

	if numChars > len(text) {
		numChars = len(text)
	}
	return Text(text[:numChars])
}

// fnRight returns the rightmost characters.
func fnRight(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	numChars := 1
	if len(args) == 2 {
		numVal := args[1].Eval(ctx)
		if numVal.IsError() {
			return numVal
		}
		n, err := numVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		numChars = int(n)
		if numChars < 0 {
			return Error(ErrValue)
		}
	}

	if numChars > len(text) {
		numChars = len(text)
	}
	start := len(text) - numChars
	return Text(text[start:])
}

// fnMid returns characters from the middle.
func fnMid(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	text := val.ToText()

	startVal := args[1].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	startNum, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	start := int(startNum) - 1 // Excel is 1-indexed
	if start < 0 {
		start = 0
	}

	numChars := len(text) - start
	if len(args) == 3 {
		numVal := args[2].Eval(ctx)
		if numVal.IsError() {
			return numVal
		}
		n, err := numVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		numChars = int(n)
		if numChars < 0 {
			return Error(ErrValue)
		}
	}

	if start >= len(text) {
		return Text("")
	}

	end := start + numChars
	if end > len(text) {
		end = len(text)
	}
	return Text(text[start:end])
}

// fnTrim removes extra spaces.
func fnTrim(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	// Remove leading/trailing spaces and collapse multiple spaces
	text = strings.TrimSpace(text)
	var result strings.Builder
	inSpace := false
	for _, ch := range text {
		if ch == ' ' || ch == '\t' {
			if !inSpace {
				result.WriteRune(' ')
				inSpace = true
			}
		} else {
			result.WriteRune(ch)
			inSpace = false
		}
	}
	return Text(result.String())
}

// fnUpper converts to uppercase.
func fnUpper(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	return Text(strings.ToUpper(text))
}

// fnLower converts to lowercase.
func fnLower(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	return Text(strings.ToLower(text))
}

// fnConcatenate joins text strings.
func fnConcatenate(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Text("")
	}

	var result strings.Builder
	for _, arg := range args {
		for _, val := range expandArgumentValues(arg, ctx) {
			if val.IsError() {
				return val
			}
			result.WriteString(val.ToText())
		}
	}
	return Text(result.String())
}

// fnAnd returns true if all arguments are true.
func fnAnd(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}

	for _, arg := range args {
		for _, val := range expandArgumentValues(arg, ctx) {
			if val.IsError() {
				return val
			}
			boolVal, err := val.ToBool()
			if err != nil {
				return Error(ErrValue)
			}
			if !boolVal {
				return Bool(false)
			}
		}
	}
	return Bool(true)
}

// fnOr returns true if any argument is true.
func fnOr(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}

	for _, arg := range args {
		for _, val := range expandArgumentValues(arg, ctx) {
			if val.IsError() {
				return val
			}
			boolVal, err := val.ToBool()
			if err != nil {
				return Error(ErrValue)
			}
			if boolVal {
				return Bool(true)
			}
		}
	}
	return Bool(false)
}

// fnNot negates a boolean value.
func fnNot(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	boolVal, err := val.ToBool()
	if err != nil {
		return Error(ErrValue)
	}
	return Bool(!boolVal)
}

// fnIsNumber checks if value is a number.
func fnIsNumber(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeNumber)
}

// fnIsText checks if value is text.
func fnIsText(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeText)
}

// fnIsBlank checks if value is blank.
func fnIsBlank(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeEmpty)
}

// fnIsErr checks if value is an error other than #N/A.
func fnIsErr(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeError && val.Err != ErrNA)
}

// fnIsError checks if value is an error.
func fnIsError(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeError)
}

// fnProper converts text to title case.
func fnProper(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	s := val.ToText()
	runes := []rune(s)
	result := make([]rune, len(runes))
	wordStart := true
	for i, r := range runes {
		if unicode.IsLetter(r) {
			if wordStart {
				result[i] = unicode.ToUpper(r)
				wordStart = false
			} else {
				result[i] = unicode.ToLower(r)
			}
		} else {
			result[i] = r
			wordStart = true
		}
	}
	return Text(string(result))
}

// fnRept repeats text n times.
func fnRept(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	numVal := args[1].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	n, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	count := int(n)
	if count < 0 {
		return Error(ErrValue)
	}
	return Text(strings.Repeat(val.ToText(), count))
}

// fnExact checks if two strings are identical (case-sensitive).
func fnExact(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	a := args[0].Eval(ctx)
	if a.IsError() {
		return a
	}
	b := args[1].Eval(ctx)
	if b.IsError() {
		return b
	}
	return Bool(a.ToText() == b.ToText())
}

// fnChar returns the character for the given code point.
func fnChar(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil || n < 1 || n > 255 {
		return Error(ErrValue)
	}
	return Text(string(rune(int(n))))
}

// fnCode returns the code point of the first character.
func fnCode(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	s := val.ToText()
	if len(s) == 0 {
		return Error(ErrValue)
	}
	return Number(float64([]rune(s)[0]))
}

// fnFind returns the 1-based position of find_text within within_text (case-sensitive).
func fnFind(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	findText := args[0].Eval(ctx)
	if findText.IsError() {
		return findText
	}
	withinText := args[1].Eval(ctx)
	if withinText.IsError() {
		return withinText
	}
	start := 1
	if len(args) == 3 {
		startVal := args[2].Eval(ctx)
		if startVal.IsError() {
			return startVal
		}
		n, err := startVal.ToNumber()
		if err != nil || n < 1 {
			return Error(ErrValue)
		}
		start = int(n)
	}
	within := withinText.ToText()
	find := findText.ToText()
	runes := []rune(within)
	if start > len(runes) {
		return Error(ErrValue)
	}
	sub := string(runes[start-1:])
	idx := strings.Index(sub, find)
	if idx < 0 {
		return Error(ErrValue)
	}
	// Convert byte offset back to rune offset
	runeOffset := len([]rune(string([]byte(sub)[:idx])))
	return Number(float64(start + runeOffset - 1 + 1))
}

// fnSearch returns the 1-based position of find_text within within_text (case-insensitive).
func fnSearch(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	findText := args[0].Eval(ctx)
	if findText.IsError() {
		return findText
	}
	withinText := args[1].Eval(ctx)
	if withinText.IsError() {
		return withinText
	}
	start := 1
	if len(args) == 3 {
		startVal := args[2].Eval(ctx)
		if startVal.IsError() {
			return startVal
		}
		n, err := startVal.ToNumber()
		if err != nil || n < 1 {
			return Error(ErrValue)
		}
		start = int(n)
	}
	within := strings.ToLower(withinText.ToText())
	find := strings.ToLower(findText.ToText())
	runes := []rune(within)
	if start > len(runes) {
		return Error(ErrValue)
	}
	sub := string(runes[start-1:])
	idx := strings.Index(sub, find)
	if idx < 0 {
		return Error(ErrValue)
	}
	runeOffset := len([]rune(string([]byte(sub)[:idx])))
	return Number(float64(start + runeOffset - 1 + 1))
}

// fnSubstitute replaces occurrences of old_text with new_text.
func fnSubstitute(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	textVal := args[0].Eval(ctx)
	if textVal.IsError() {
		return textVal
	}
	oldText := args[1].Eval(ctx)
	if oldText.IsError() {
		return oldText
	}
	newText := args[2].Eval(ctx)
	if newText.IsError() {
		return newText
	}
	s := textVal.ToText()
	old := oldText.ToText()
	new := newText.ToText()
	if len(args) == 4 {
		instVal := args[3].Eval(ctx)
		if instVal.IsError() {
			return instVal
		}
		n, err := instVal.ToNumber()
		if err != nil || n < 1 {
			return Error(ErrValue)
		}
		instNum := int(n)
		count := 0
		var buf strings.Builder
		remain := s
		for {
			idx := strings.Index(remain, old)
			if idx < 0 {
				buf.WriteString(remain)
				break
			}
			count++
			if count == instNum {
				buf.WriteString(remain[:idx])
				buf.WriteString(new)
				buf.WriteString(remain[idx+len(old):])
				break
			}
			buf.WriteString(remain[:idx+len(old)])
			remain = remain[idx+len(old):]
		}
		return Text(buf.String())
	}
	return Text(strings.ReplaceAll(s, old, new))
}

// fnReplace replaces a portion of text by position.
func fnReplace(args []Expression, ctx Context) Value {
	if len(args) != 4 {
		return Error(ErrValue)
	}
	oldText := args[0].Eval(ctx)
	if oldText.IsError() {
		return oldText
	}
	startVal := args[1].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	numCharsVal := args[2].Eval(ctx)
	if numCharsVal.IsError() {
		return numCharsVal
	}
	newText := args[3].Eval(ctx)
	if newText.IsError() {
		return newText
	}
	s := []rune(oldText.ToText())
	startN, err := startVal.ToNumber()
	if err != nil || startN < 1 {
		return Error(ErrValue)
	}
	numCharsN, err := numCharsVal.ToNumber()
	if err != nil || numCharsN < 0 {
		return Error(ErrValue)
	}
	start := int(startN) - 1
	numChars := int(numCharsN)
	if start > len(s) {
		start = len(s)
	}
	end := start + numChars
	if end > len(s) {
		end = len(s)
	}
	result := string(s[:start]) + newText.ToText() + string(s[end:])
	return Text(result)
}

// fnTextJoin joins text values with a delimiter.
func fnTextJoin(args []Expression, ctx Context) Value {
	if len(args) < 3 {
		return Error(ErrValue)
	}
	delimVal := args[0].Eval(ctx)
	if delimVal.IsError() {
		return delimVal
	}
	ignoreEmptyVal := args[1].Eval(ctx)
	if ignoreEmptyVal.IsError() {
		return ignoreEmptyVal
	}
	delim := delimVal.ToText()
	ignoreEmpty, err := ignoreEmptyVal.ToBool()
	if err != nil {
		return Error(ErrValue)
	}
	var parts []string
	for _, arg := range args[2:] {
		for _, v := range expandArgumentValues(arg, ctx) {
			if v.IsError() {
				return v
			}
			s := v.ToText()
			if ignoreEmpty && s == "" {
				continue
			}
			parts = append(parts, s)
		}
	}
	return Text(strings.Join(parts, delim))
}

// fnIsNA checks if value is a #N/A error.
func fnIsNA(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeError && val.Err == ErrNA)
}

// fnIsLogical checks if value is a boolean.
func fnIsLogical(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	return Bool(val.Type == TypeBool)
}

// fnIsNonText checks if value is not text (includes errors and empty).
func fnIsNonText(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	return Bool(val.Type != TypeText)
}

// fnIsRef checks whether the argument is a reference expression.
func fnIsRef(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	return Bool(resolvesReferenceExpression(args[0], ctx))
}

// fnAreas returns the number of reference areas in a reference expression.
func fnAreas(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	if resolvesReferenceExpression(args[0], ctx) {
		return Number(1)
	}
	if val := args[0].Eval(ctx); val.IsError() {
		return val
	}
	return Error(ErrValue)
}

func resolvesReferenceExpression(expr Expression, ctx Context) bool {
	if fn, ok := expr.(*FuncCallExpr); ok {
		if strings.EqualFold(fn.Name, "CHOOSE") {
			if resolved, handled := resolvesChooseReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
		if strings.EqualFold(fn.Name, "IF") {
			if resolved, handled := resolvesIfReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
		if strings.EqualFold(fn.Name, "IFS") {
			if resolved, handled := resolvesIfsReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
		if strings.EqualFold(fn.Name, "SWITCH") {
			if resolved, handled := resolvesSwitchReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
		if strings.EqualFold(fn.Name, "IFERROR") {
			if resolved, handled := resolvesIfErrorReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
		if strings.EqualFold(fn.Name, "IFNA") {
			if resolved, handled := resolvesIfNaReferenceExpression(fn.Args, ctx); handled {
				return resolved
			}
		}
	}
	if isReferenceExpression(expr) {
		return true
	}
	targetRef, errValue := referenceTargetRef(expr, ctx)
	return targetRef != "" && errValue.Type == TypeEmpty
}

func resolvesIfReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) < 2 || len(args) > 3 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	conditionValues := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(conditionValues) == 1 {
		return false, false
	}
	for pos := 0; pos < len(conditionValues); pos++ {
		rowOffset, colOffset, ok := expressionOffsetsForLinearIndex(args[0], ctx, pos)
		if !ok {
			return false, true
		}
		conditionValue := expressionValueAtOffset(args[0], ctx, rowOffset, colOffset)
		if conditionValue.IsError() {
			return false, true
		}
		isTrue, err := conditionValue.ToBool()
		if err != nil {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, rowOffset, colOffset)
		if !ok {
			return false, true
		}
		if isTrue {
			if _, _, ok := referenceShape(args[1], offsetCtx); !ok {
				return false, true
			}
			continue
		}
		if len(args) < 3 {
			return false, true
		}
		if _, _, ok := referenceShape(args[2], offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func resolvesChooseReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) < 2 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	selectorValues := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(selectorValues) <= 1 {
		return false, false
	}
	for pos := 0; pos < len(selectorValues); pos++ {
		selectorRow, selectorCol, ok := expressionOffsetsForLinearIndex(args[0], ctx, pos)
		if !ok {
			return false, true
		}
		selectorValue := expressionValueAtOffset(args[0], ctx, selectorRow, selectorCol)
		if selectorValue.IsError() {
			return false, true
		}
		num, err := selectorValue.ToNumber()
		if err != nil {
			return false, true
		}
		selector := int(num)
		if selector < 1 || selector >= len(args) {
			return false, true
		}
		selected := args[selector]
		selectedRow, selectedCol, ok := expressionOffsetsForLinearIndex(selected, ctx, pos)
		if !ok {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, selectedRow, selectedCol)
		if !ok {
			return false, true
		}
		if _, _, ok := referenceShape(selected, offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func resolvesIfsReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) < 2 || len(args)%2 != 0 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	maxLen := 1
	for i := 0; i < len(args); i += 2 {
		conditionValues := normalizeArgumentValues(expandArgumentValues(args[i], sourceCtx))
		if len(conditionValues) > maxLen {
			maxLen = len(conditionValues)
		}
	}
	if maxLen <= 1 {
		return false, false
	}
	for pos := 0; pos < maxLen; pos++ {
		selected := -1
		for i := 0; i < len(args); i += 2 {
			rowOffset, colOffset, ok := expressionOffsetsForLinearIndex(args[i], ctx, pos)
			if !ok {
				return false, true
			}
			conditionValue := expressionValueAtOffset(args[i], ctx, rowOffset, colOffset)
			if conditionValue.IsError() {
				return false, true
			}
			isTrue, err := conditionValue.ToBool()
			if err != nil {
				return false, true
			}
			if isTrue {
				selected = i + 1
				break
			}
		}
		if selected < 0 {
			return false, true
		}
		selectedRow, selectedCol, ok := expressionOffsetsForLinearIndex(args[selected], ctx, pos)
		if !ok {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, selectedRow, selectedCol)
		if !ok {
			return false, true
		}
		if _, _, ok := referenceShape(args[selected], offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func resolvesSwitchReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) < 3 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	selectorValues := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(selectorValues) <= 1 {
		return false, false
	}
	pairEnd := len(args)
	var defaultExpr Expression
	if len(args)%2 == 0 {
		defaultExpr = args[len(args)-1]
		pairEnd = len(args) - 1
	}
	for pos := 0; pos < len(selectorValues); pos++ {
		selectorRow, selectorCol, ok := expressionOffsetsForLinearIndex(args[0], ctx, pos)
		if !ok {
			return false, true
		}
		selector := expressionValueAtOffset(args[0], ctx, selectorRow, selectorCol)
		if selector.IsError() {
			return false, true
		}
		selected := -1
		for i := 1; i < pairEnd; i += 2 {
			candidateRow, candidateCol, ok := expressionOffsetsForLinearIndex(args[i], ctx, pos)
			if !ok {
				return false, true
			}
			candidate := expressionValueAtOffset(args[i], ctx, candidateRow, candidateCol)
			if candidate.IsError() {
				return false, true
			}
			if valuesEqual(selector, candidate) {
				selected = i + 1
				break
			}
		}
		if selected >= 0 {
			selectedRow, selectedCol, ok := expressionOffsetsForLinearIndex(args[selected], ctx, pos)
			if !ok {
				return false, true
			}
			offsetCtx, ok := contextForArrayOffset(ctx, selectedRow, selectedCol)
			if !ok {
				return false, true
			}
			if _, _, ok := referenceShape(args[selected], offsetCtx); !ok {
				return false, true
			}
			continue
		}
		if defaultExpr == nil {
			return false, true
		}
		defaultRow, defaultCol, ok := expressionOffsetsForLinearIndex(defaultExpr, ctx, pos)
		if !ok {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, defaultRow, defaultCol)
		if !ok {
			return false, true
		}
		if _, _, ok := referenceShape(defaultExpr, offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func resolvesIfErrorReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) != 2 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(values) <= 1 {
		return false, false
	}
	for pos, value := range values {
		rowOffset, colOffset, ok := expressionOffsetsForLinearIndex(args[0], ctx, pos)
		if !ok {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, rowOffset, colOffset)
		if !ok {
			return false, true
		}
		selected := args[0]
		if value.IsError() {
			selected = args[1]
		}
		if _, _, ok := referenceShape(selected, offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func resolvesIfNaReferenceExpression(args []Expression, ctx Context) (bool, bool) {
	if len(args) != 2 {
		return false, true
	}
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(values) <= 1 {
		return false, false
	}
	for pos, value := range values {
		rowOffset, colOffset, ok := expressionOffsetsForLinearIndex(args[0], ctx, pos)
		if !ok {
			return false, true
		}
		offsetCtx, ok := contextForArrayOffset(ctx, rowOffset, colOffset)
		if !ok {
			return false, true
		}
		selected := args[0]
		if value.IsError() && value.Err == ErrNA {
			selected = args[1]
		}
		if _, _, ok := referenceShape(selected, offsetCtx); !ok {
			return false, true
		}
	}
	return true, true
}

func isReferenceExpression(expr Expression) bool {
	switch expr.(type) {
	case *CellRefExpr, *RangeExpr:
		return true
	}
	return false
}

func rangeRefShape(start, end string) (int, int, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return 0, 0, false
	}
	return endCoords.row - startCoords.row + 1, endCoords.col - startCoords.col + 1, true
}

func indexReferenceShape(args []Expression, ctx Context) (int, int, bool) {
	if len(args) < 2 || len(args) > 4 {
		return 0, 0, false
	}
	if len(args) == 4 {
		areaNum, ok := indexSelectorNumber(args[3], ctx)
		if !ok || areaNum != 1 {
			return 0, 0, false
		}
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return 0, 0, false
	}
	rowCount, colCount, ok := rangeRefShape(start, end)
	if !ok {
		return 0, 0, false
	}
	rowNum, ok := indexSelectorNumber(args[1], ctx)
	if !ok {
		return 0, 0, false
	}
	colNum := 1
	if len(args) >= 3 {
		colValue, ok := indexSelectorNumber(args[2], ctx)
		if !ok {
			return 0, 0, false
		}
		colNum = colValue
	}
	if rowNum < 0 || colNum < 0 {
		return 0, 0, false
	}
	switch {
	case rowNum == 0 && colNum == 0:
		return rowCount, colCount, true
	case rowNum == 0:
		if colNum < 1 || colNum > colCount {
			return 0, 0, false
		}
		return rowCount, 1, true
	case colNum == 0:
		if rowNum < 1 || rowNum > rowCount {
			return 0, 0, false
		}
		return 1, colCount, true
	default:
		if rowNum < 1 || rowNum > rowCount || colNum < 1 || colNum > colCount {
			return 0, 0, false
		}
		return 1, 1, true
	}
}

func offsetReferenceShape(args []Expression, ctx Context) (int, int, bool) {
	if len(args) < 3 || len(args) > 5 {
		return 0, 0, false
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return 0, 0, false
	}
	baseRows, baseCols, ok := rangeRefShape(start, end)
	if !ok {
		return 0, 0, false
	}
	if _, ok := indexSelectorNumber(args[1], ctx); !ok {
		return 0, 0, false
	}
	if _, ok := indexSelectorNumber(args[2], ctx); !ok {
		return 0, 0, false
	}
	height, width := baseRows, baseCols
	if len(args) >= 4 {
		height, ok = indexSelectorNumber(args[3], ctx)
		if !ok || height <= 0 {
			return 0, 0, false
		}
	}
	if len(args) >= 5 {
		width, ok = indexSelectorNumber(args[4], ctx)
		if !ok || width <= 0 {
			return 0, 0, false
		}
	}
	return height, width, true
}

func chooseAxisReferenceShape(args []Expression, ctx Context, selectRows bool) (int, int, bool) {
	if len(args) < 2 {
		return 0, 0, false
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return 0, 0, false
	}
	rowCount, colCount, ok := rangeRefShape(start, end)
	if !ok {
		return 0, 0, false
	}
	indices, ok := chooseAxisSelectors(args[1:], ctx)
	if !ok {
		return 0, 0, false
	}
	if selectRows {
		for _, idx := range indices {
			if _, ok := resolveChooseAxisIndex(idx, rowCount); !ok {
				return 0, 0, false
			}
		}
		return len(indices), colCount, true
	}
	for _, idx := range indices {
		if _, ok := resolveChooseAxisIndex(idx, colCount); !ok {
			return 0, 0, false
		}
	}
	return rowCount, len(indices), true
}

func takeDropReferenceShape(args []Expression, ctx Context, take bool) (int, int, bool) {
	if len(args) < 2 || len(args) > 3 {
		return 0, 0, false
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return 0, 0, false
	}
	rowCount, ok := indexSelectorNumber(args[1], ctx)
	if !ok {
		return 0, 0, false
	}
	colCount := 0
	if len(args) == 3 {
		colValue, ok := indexSelectorNumber(args[2], ctx)
		if !ok {
			return 0, 0, false
		}
		colCount = colValue
	}
	targetStart, targetEnd, ok := takeDropRangeRefs(start, end, rowCount, colCount, take)
	if !ok {
		return 0, 0, false
	}
	return rangeRefShape(targetStart, targetEnd)
}

func referenceShape(expr Expression, ctx Context) (int, int, bool) {
	switch expr := expr.(type) {
	case *CellRefExpr:
		return 1, 1, true
	case *RangeExpr:
		return rangeRefShape(expr.Start, expr.End)
	case *FuncCallExpr:
		switch expr.Name {
		case "INDIRECT":
			start, end, ok, errValue := indirectTargetRange(expr.Args, ctx)
			if !ok || errValue.Type != TypeEmpty {
				return 0, 0, false
			}
			return rangeRefShape(start, end)
		case "INDEX":
			return indexReferenceShape(expr.Args, ctx)
		case "OFFSET":
			return offsetReferenceShape(expr.Args, ctx)
		case "TRANSPOSE":
			if len(expr.Args) != 1 {
				return 0, 0, false
			}
			rows, cols, ok := referenceShape(expr.Args[0], ctx)
			if !ok {
				return 0, 0, false
			}
			return cols, rows, true
		case "CHOOSECOLS":
			return chooseAxisReferenceShape(expr.Args, ctx, false)
		case "CHOOSEROWS":
			return chooseAxisReferenceShape(expr.Args, ctx, true)
		case "TAKE":
			return takeDropReferenceShape(expr.Args, ctx, true)
		case "DROP":
			return takeDropReferenceShape(expr.Args, ctx, false)
		case "CHOOSE":
			selected, _, ok := chooseTargetExpression(expr.Args, ctx)
			if !ok {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		case "IF":
			selected, _, ok := ifTargetExpression(expr.Args, ctx)
			if !ok || selected == nil {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		case "IFS":
			selected, _, ok := selectIfsBranch(expr.Args, ctx)
			if !ok || selected == nil {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		case "SWITCH":
			selected, _, ok := selectSwitchBranch(expr.Args, ctx)
			if !ok || selected == nil {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		case "IFERROR":
			selected, _, ok := selectIfErrorBranch(expr.Args, ctx)
			if !ok || selected == nil {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		case "IFNA":
			selected, _, ok := selectIfNaBranch(expr.Args, ctx)
			if !ok || selected == nil {
				return 0, 0, false
			}
			return referenceShape(selected, ctx)
		}
	}
	return 0, 0, false
}

func referenceTargetRef(expr Expression, ctx Context) (string, Value) {
	switch expr := expr.(type) {
	case *CellRefExpr:
		return expr.Ref, Empty()
	case *RangeExpr:
		targetRef, ok := arrayRangeTargetRef(expr.Start, expr.End, ctx)
		if !ok || targetRef == "" {
			return "", Error(ErrNA)
		}
		return targetRef, Empty()
	case *FuncCallExpr:
		switch expr.Name {
		case "IF":
			selected, errValue, ok := ifTargetExpression(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if selected == nil {
				return "", Empty()
			}
			return referenceTargetRef(selected, ctx)
		case "IFS":
			selected, errValue, ok := selectIfsBranch(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if selected == nil {
				return "", Empty()
			}
			return referenceTargetRef(selected, ctx)
		case "SWITCH":
			selected, errValue, ok := selectSwitchBranch(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if selected == nil {
				return "", Empty()
			}
			return referenceTargetRef(selected, ctx)
		case "IFERROR":
			selected, errValue, ok := selectIfErrorBranch(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if selected == nil {
				return "", Empty()
			}
			return referenceTargetRef(selected, ctx)
		case "IFNA":
			selected, errValue, ok := selectIfNaBranch(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if selected == nil {
				return "", Empty()
			}
			return referenceTargetRef(selected, ctx)
		case "CHOOSE":
			selected, errValue, ok := chooseTargetExpression(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			return referenceTargetRef(selected, ctx)
		case "INDIRECT":
			start, end, ok, errValue := indirectTargetRange(expr.Args, ctx)
			if !ok {
				return "", errValue
			}
			if start == end {
				return start, Empty()
			}
			targetRef, ok := arrayRangeTargetRef(start, end, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrRef)
			}
			return targetRef, Empty()
		case "INDEX":
			if len(expr.Args) < 2 || len(expr.Args) > 4 {
				return "", Error(ErrValue)
			}
			if len(expr.Args) == 4 {
				areaNum, ok := indexSelectorNumber(expr.Args[3], ctx)
				if !ok {
					return "", Error(ErrValue)
				}
				if areaNum != 1 {
					return "", Error(ErrRef)
				}
			}
			rowNum, ok := indexSelectorNumber(expr.Args[1], ctx)
			if !ok {
				return "", Error(ErrValue)
			}
			colNum := 1
			if len(expr.Args) >= 3 {
				colValue, ok := indexSelectorNumber(expr.Args[2], ctx)
				if !ok {
					return "", Error(ErrValue)
				}
				colNum = colValue
			}
			if rowNum < 0 || colNum < 0 {
				return "", Error(ErrValue)
			}
			start, end, ok := indexTargetRange(expr.Args[0])
			if !ok {
				return "", Error(ErrRef)
			}
			targetRef, ok := indexRangeTargetRef(start, end, rowNum, colNum, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrRef)
			}
			return targetRef, Empty()
		case "OFFSET":
			if len(expr.Args) < 3 || len(expr.Args) > 5 {
				return "", Error(ErrValue)
			}
			start, end, ok := indexTargetRange(expr.Args[0])
			if !ok {
				return "", Error(ErrRef)
			}
			rowOffset, ok := indexSelectorNumber(expr.Args[1], ctx)
			if !ok {
				return "", Error(ErrValue)
			}
			colOffset, ok := indexSelectorNumber(expr.Args[2], ctx)
			if !ok {
				return "", Error(ErrValue)
			}
			startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
			if !ok {
				return "", Error(ErrRef)
			}
			height := endCoords.row - startCoords.row + 1
			width := endCoords.col - startCoords.col + 1
			if len(expr.Args) >= 4 {
				height, ok = indexSelectorNumber(expr.Args[3], ctx)
				if !ok || height <= 0 {
					return "", Error(ErrValue)
				}
			}
			if len(expr.Args) >= 5 {
				width, ok = indexSelectorNumber(expr.Args[4], ctx)
				if !ok || width <= 0 {
					return "", Error(ErrValue)
				}
			}
			targetStart, targetEnd, ok := offsetRangeRefs(start, end, rowOffset, colOffset, height, width)
			if !ok {
				return "", Error(ErrRef)
			}
			targetRef, ok := offsetRangeTargetRef(targetStart, targetEnd, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrRef)
			}
			return targetRef, Empty()
		case "TRANSPOSE":
			if len(expr.Args) != 1 {
				return "", Error(ErrValue)
			}
			start, end, ok := indexTargetRange(expr.Args[0])
			if !ok {
				return "", Error(ErrRef)
			}
			targetRef, ok := transposeRangeTargetRef(start, end, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrNA)
			}
			return targetRef, Empty()
		case "CHOOSECOLS":
			if len(expr.Args) < 2 {
				return "", Error(ErrValue)
			}
			start, end, ok := indexTargetRange(expr.Args[0])
			if !ok {
				return "", Error(ErrRef)
			}
			indices, ok := chooseAxisSelectors(expr.Args[1:], ctx)
			if !ok {
				return "", Error(ErrValue)
			}
			targetRef, ok := chooseAxisTargetRef(start, end, indices, false, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrRef)
			}
			return targetRef, Empty()
		case "CHOOSEROWS":
			if len(expr.Args) < 2 {
				return "", Error(ErrValue)
			}
			start, end, ok := indexTargetRange(expr.Args[0])
			if !ok {
				return "", Error(ErrRef)
			}
			indices, ok := chooseAxisSelectors(expr.Args[1:], ctx)
			if !ok {
				return "", Error(ErrValue)
			}
			targetRef, ok := chooseAxisTargetRef(start, end, indices, true, ctx)
			if !ok || targetRef == "" {
				return "", Error(ErrRef)
			}
			return targetRef, Empty()
		case "TAKE":
			targetRef, ok, errValue := takeDropTargetRef(expr.Args, true, ctx)
			if !ok {
				return "", errValue
			}
			return targetRef, Empty()
		case "DROP":
			targetRef, ok, errValue := takeDropTargetRef(expr.Args, false, ctx)
			if !ok {
				return "", errValue
			}
			return targetRef, Empty()
		}
	}
	return "", Empty()
}

// fnIsEven checks whether the truncated numeric value is even.
func fnIsEven(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Bool(math.Mod(math.Abs(math.Trunc(n)), 2) == 0)
}

// fnIsOdd checks whether the truncated numeric value is odd.
func fnIsOdd(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Bool(math.Mod(math.Abs(math.Trunc(n)), 2) == 1)
}

// fnErrorType returns the numeric code for an error value or #N/A when no error exists.
func fnErrorType(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.Type != TypeError {
		return Error(ErrNA)
	}

	switch val.Err {
	case ErrNull:
		return Number(1)
	case ErrDiv0:
		return Number(2)
	case ErrValue:
		return Number(3)
	case ErrRef:
		return Number(4)
	case ErrName:
		return Number(5)
	case ErrNum:
		return Number(6)
	case ErrNA:
		return Number(7)
	case ErrSpill:
		return Number(9)
	default:
		return Error(ErrNA)
	}
}

// fnType returns the Excel TYPE code for the evaluated value.
func fnType(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	if shape, ok := inferExpressionArrayShapeWithContext(args[0], ctx); ok && (shape.rows > 1 || shape.cols > 1) {
		return Number(64)
	}

	switch expr := args[0].(type) {
	case *RangeExpr:
		return Number(64)
	case *FuncCallExpr:
		if rows, cols, ok := referenceShape(expr, ctx); ok && (rows > 1 || cols > 1) {
			return Number(64)
		}
	}

	val := args[0].Eval(ctx)
	switch val.Type {
	case TypeNumber:
		return Number(1)
	case TypeText:
		return Number(2)
	case TypeBool:
		return Number(4)
	case TypeError:
		return Number(16)
	case TypeEmpty:
		return Number(1)
	default:
		return Error(ErrValue)
	}
}

// fnT returns text values unchanged, errors unchanged, and empty text otherwise.
func fnT(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.Type == TypeError {
		return val
	}
	if val.Type == TypeText {
		return val
	}
	return Text("")
}

// fnN converts values to numbers using Excel's compatibility rules.
func fnN(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	switch val.Type {
	case TypeNumber:
		return val
	case TypeBool:
		if val.Bool {
			return Number(1)
		}
		return Number(0)
	case TypeError:
		return val
	default:
		return Number(0)
	}
}

// fnValue converts text to number.
func fnValue(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	text := val.ToText()
	num, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return Error(ErrValue)
	}
	return Number(num)
}

// fnText converts number to text.
func fnText(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}

	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}

	num, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}

	formatVal := args[1].Eval(ctx)
	if formatVal.IsError() {
		return formatVal
	}
	format := formatVal.ToText()

	// Simple format handling - just return the number as string
	// Full format support would require a format parser
	_ = format
	return Text(fmt.Sprintf("%g", num))
}

// fnFormulaText returns the text of a referenced cell's formula, including '='.
func fnFormulaText(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	targetRef, errValue := referenceTargetRef(args[0], ctx)
	if targetRef == "" {
		if errValue.Type != TypeEmpty {
			return errValue
		}
		return Error(ErrValue)
	}
	formula, ok := ctx.GetFormula(targetRef)
	if !ok {
		return Error(ErrNA)
	}
	return Text("=" + formula)
}

// excelEpoch is the base date for Excel serial numbers (day 0 = Dec 30, 1899).
// Using Dec 30, 1899 accounts for Excel's intentional 1900 leap-year bug.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// timeToSerial converts a time.Time to an Excel serial number.
func timeToSerial(t time.Time) float64 {
	days := t.Sub(excelEpoch).Hours() / 24
	return days
}

// serialToTime converts an Excel serial number to time.Time.
func serialToTime(serial float64) time.Time {
	duration := time.Duration(serial * 24 * float64(time.Hour))
	return excelEpoch.Add(duration)
}

// fnToday returns current date.
func fnToday(args []Expression, ctx Context) Value {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return Number(timeToSerial(today))
}

// fnNow returns current date and time.
func fnNow(args []Expression, ctx Context) Value {
	return Number(timeToSerial(time.Now().UTC()))
}

func currentSheetOrdinal(ctx Context) int {
	if index, ok := ctx.ResolveSheetOrdinal(ctx.CurrentRef()); ok && index > 0 {
		return index
	}
	return 1
}

func isQualifiedSheetToken(token string) bool {
	return strings.LastIndex(token, "!") >= 0
}

func sheetOrdinalFromArg(arg Expression, ctx Context) (int, bool, Value) {
	switch expr := arg.(type) {
	case *CellRefExpr:
		if index, ok := ctx.ResolveSheetOrdinal(expr.Ref); ok && index > 0 {
			return index, true, Value{}
		}
		if !isQualifiedSheetToken(expr.Ref) {
			return currentSheetOrdinal(ctx), true, Value{}
		}
		return 0, false, Error(ErrRef)
	case *RangeExpr:
		if index, ok := ctx.ResolveSheetOrdinal(expr.Start); ok && index > 0 {
			return index, true, Value{}
		}
		if !isQualifiedSheetToken(expr.Start) {
			return currentSheetOrdinal(ctx), true, Value{}
		}
		return 0, false, Error(ErrRef)
	case *FuncCallExpr:
		if resolvesReferenceExpression(expr, ctx) {
			targetRef, errValue := referenceTargetRef(expr, ctx)
			if targetRef != "" {
				if index, ok := ctx.ResolveSheetOrdinal(targetRef); ok && index > 0 {
					return index, true, Value{}
				}
				if !isQualifiedSheetToken(targetRef) {
					return currentSheetOrdinal(ctx), true, Value{}
				}
				return 0, false, Error(ErrRef)
			}
			if errValue.Type != TypeEmpty {
				return 0, false, errValue
			}
			return currentSheetOrdinal(ctx), true, Value{}
		}
	}

	val := arg.Eval(ctx)
	if val.IsError() {
		return 0, false, val
	}
	if val.IsText() {
		if index, ok := ctx.ResolveSheetOrdinal(val.Text); ok && index > 0 {
			return index, true, Value{}
		}
		return 0, false, Error(ErrNA)
	}
	return 0, false, Error(ErrRef)
}

// fnSheet returns the sheet number of a sheet name or reference.
func fnSheet(args []Expression, ctx Context) Value {
	if len(args) > 1 {
		return Error(ErrValue)
	}
	if len(args) == 0 {
		return Number(float64(currentSheetOrdinal(ctx)))
	}
	index, ok, errValue := sheetOrdinalFromArg(args[0], ctx)
	if !ok {
		return errValue
	}
	return Number(float64(index))
}

// fnSheets returns the number of sheets in the workbook or reference.
func fnSheets(args []Expression, ctx Context) Value {
	if len(args) > 1 {
		return Error(ErrValue)
	}
	if len(args) == 0 {
		return Number(float64(ctx.SheetCount()))
	}
	if resolvesReferenceExpression(args[0], ctx) {
		return Number(1)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	return Error(ErrRef)
}

// fnDate returns the serial number for the given year, month, day.
func fnDate(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	yearVal := args[0].Eval(ctx)
	if yearVal.IsError() {
		return yearVal
	}
	monthVal := args[1].Eval(ctx)
	if monthVal.IsError() {
		return monthVal
	}
	dayVal := args[2].Eval(ctx)
	if dayVal.IsError() {
		return dayVal
	}
	yearN, err := yearVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	monthN, err := monthVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	dayN, err := dayVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	t := time.Date(int(yearN), time.Month(int(monthN)), int(dayN), 0, 0, 0, 0, time.UTC)
	serial := timeToSerial(t)
	if serial < 0 {
		return Error(ErrNum)
	}
	return Number(serial)
}

// fnYear extracts the year from an Excel serial date.
func fnYear(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Year()))
}

// fnIsFormula reports whether a reference points to a formula cell.
func fnIsFormula(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	targetRef, errValue := referenceTargetRef(args[0], ctx)
	if targetRef == "" {
		if errValue.Type != TypeEmpty {
			return errValue
		}
		return Bool(false)
	}
	_, ok := ctx.GetFormula(targetRef)
	return Bool(ok)
}

// fnMonth extracts the month (1–12) from an Excel serial date.
func fnMonth(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Month()))
}

// fnDay extracts the day-of-month from an Excel serial date.
func fnDay(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Day()))
}

// fnHour extracts the hour (0–23) from an Excel serial datetime.
func fnHour(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Hour()))
}

// fnMinute extracts the minute (0–59) from an Excel serial datetime.
func fnMinute(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Minute()))
}

// fnSecond extracts the second (0–59) from an Excel serial datetime.
func fnSecond(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(float64(serialToTime(n).Second()))
}

// fnWeekday returns the day of week. return_type 1 (default): 1=Sun…7=Sat.
func fnWeekday(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	returnType := 1
	if len(args) == 2 {
		rtVal := args[1].Eval(ctx)
		if rtVal.IsError() {
			return rtVal
		}
		rt, err := rtVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		returnType = int(rt)
	}
	weekday := serialToTime(n).Weekday() // 0=Sun
	switch returnType {
	case 1: // 1=Sun, 2=Mon, …, 7=Sat
		return Number(float64(int(weekday) + 1))
	case 2: // 1=Mon, …, 7=Sun
		return Number(float64((int(weekday)+6)%7 + 1))
	case 3: // 0=Mon, …, 6=Sun
		return Number(float64((int(weekday) + 6) % 7))
	default:
		return Error(ErrNum)
	}
}

// fnDays returns the number of days between two dates.
func fnDays(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	endVal := args[0].Eval(ctx)
	if endVal.IsError() {
		return endVal
	}
	startVal := args[1].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	endN, err := endVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(math.Trunc(endN - startN))
}

// fnRow returns the row number of a cell reference.
func fnRow(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		if row, _ := parseCellRef(ctx.CurrentRef()); row > 0 {
			return Number(float64(row))
		}
		return Number(1)
	}

	targetRef, errValue := referenceTargetRef(args[0], ctx)
	if targetRef != "" {
		_, ref := splitQualifiedRangeRef(targetRef)
		row, _ := parseCellRef(ref)
		if row > 0 {
			return Number(float64(row))
		}
	}
	if errValue.Type != TypeEmpty {
		return errValue
	}

	return Number(1)
}

// fnColumn returns the column number of a cell reference.
func fnColumn(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		if _, col := parseCellRef(ctx.CurrentRef()); col > 0 {
			return Number(float64(col))
		}
		return Number(1)
	}

	targetRef, errValue := referenceTargetRef(args[0], ctx)
	if targetRef != "" {
		_, ref := splitQualifiedRangeRef(targetRef)
		_, col := parseCellRef(ref)
		if col > 0 {
			return Number(float64(col))
		}
	}
	if errValue.Type != TypeEmpty {
		return errValue
	}

	return Number(1)
}

// fnAddress returns a cell reference string for the given row/column.
func fnAddress(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 5 {
		return Error(ErrValue)
	}
	rowVal := args[0].Eval(ctx)
	if rowVal.IsError() {
		return rowVal
	}
	colVal := args[1].Eval(ctx)
	if colVal.IsError() {
		return colVal
	}
	rowNum, err := rowVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	colNum, err := colVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	row := int(rowNum)
	col := int(colNum)
	if row <= 0 || col <= 0 {
		return Error(ErrValue)
	}

	absNum := 1
	if len(args) >= 3 {
		absVal := args[2].Eval(ctx)
		if absVal.IsError() {
			return absVal
		}
		n, err := absVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		absNum = int(n)
		if absNum < 1 || absNum > 4 {
			return Error(ErrValue)
		}
	}

	a1Style := true
	if len(args) >= 4 {
		a1Val := args[3].Eval(ctx)
		if a1Val.IsError() {
			return a1Val
		}
		b, err := a1Val.ToBool()
		if err != nil {
			return Error(ErrValue)
		}
		a1Style = b
	}

	prefix := ""
	if len(args) == 5 {
		sheetVal := args[4].Eval(ctx)
		if sheetVal.IsError() {
			return sheetVal
		}
		sheetName := sheetVal.ToText()
		if sheetName != "" {
			prefix = formatAddressSheetPrefix(sheetName)
		}
	}

	return Text(prefix + formatAddressRef(row, col, absNum, a1Style))
}

// fnIndirect resolves an A1-style text reference to its referenced value.
func fnIndirect(args []Expression, ctx Context) Value {
	start, end, ok, errValue := indirectTargetRange(args, ctx)
	if !ok {
		return errValue
	}
	if start == end {
		return ctx.GetCell(start)
	}
	targetRef, ok := arrayRangeTargetRef(start, end, ctx)
	if !ok || targetRef == "" {
		return Error(ErrRef)
	}
	return ctx.GetCell(targetRef)
}

func formatAddressRef(row, col, absNum int, a1Style bool) string {
	if a1Style {
		colRef := colToLetters(col)
		rowRef := strconv.Itoa(row)
		switch absNum {
		case 1:
			return "$" + colRef + "$" + rowRef
		case 2:
			return colRef + "$" + rowRef
		case 3:
			return "$" + colRef + rowRef
		default:
			return colRef + rowRef
		}
	}

	rowRef := fmt.Sprintf("R[%d]", row)
	if absNum == 1 || absNum == 2 {
		rowRef = fmt.Sprintf("R%d", row)
	}
	colRef := fmt.Sprintf("C[%d]", col)
	if absNum == 1 || absNum == 3 {
		colRef = fmt.Sprintf("C%d", col)
	}
	return rowRef + colRef
}

func formatAddressSheetPrefix(name string) string {
	needsQuote := false
	for _, ch := range name {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '.' {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return "'" + strings.ReplaceAll(name, "'", "''") + "'!"
	}
	return name + "!"
}

func indirectTargetRange(args []Expression, ctx Context) (string, string, bool, Value) {
	if len(args) < 1 || len(args) > 2 {
		return "", "", false, Error(ErrValue)
	}
	refVal := args[0].Eval(ctx)
	if refVal.IsError() {
		return "", "", false, refVal
	}
	refText := strings.TrimSpace(refVal.ToText())
	if refText == "" {
		return "", "", false, Error(ErrRef)
	}
	a1Style := true
	if len(args) == 2 {
		styleVal := args[1].Eval(ctx)
		if styleVal.IsError() {
			return "", "", false, styleVal
		}
		b, err := styleVal.ToBool()
		if err != nil {
			return "", "", false, Error(ErrValue)
		}
		a1Style = b
	}
	if !a1Style {
		return "", "", false, Error(ErrRef)
	}
	expr, err := Parse(refText)
	if err != nil {
		return "", "", false, Error(ErrRef)
	}
	switch refExpr := expr.(type) {
	case *CellRefExpr:
		_, cellRef := splitQualifiedRangeRef(refExpr.Ref)
		row, col := parseCellRef(cellRef)
		if row == 0 || col == 0 {
			return "", "", false, Error(ErrRef)
		}
		return refExpr.Ref, refExpr.Ref, true, Empty()
	case *RangeExpr:
		if _, _, ok := parseRangeRef(refExpr.Start + ":" + refExpr.End); !ok {
			return "", "", false, Error(ErrRef)
		}
		return refExpr.Start, refExpr.End, true, Empty()
	default:
		return "", "", false, Error(ErrRef)
	}
}

func arrayRangeTargetCoords(start, end string, ctx Context) (int, int, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		_, startRef := splitQualifiedRangeRef(start)
		row, col := parseCellRef(startRef)
		return row, col, row > 0 && col > 0
	}
	row, col := startCoords.row, startCoords.col
	currentRow, currentCol := parseCellRef(ctx.CurrentRef())
	anchorRow, anchorCol := parseCellRef(ctx.ArrayAnchorRef())
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return row, col, true
	}
	rowOffset := currentRow - anchorRow
	colOffset := currentCol - anchorCol
	if startCoords.row != endCoords.row {
		row += rowOffset
	}
	if startCoords.col != endCoords.col {
		col += colOffset
	}
	if row < startCoords.row || row > endCoords.row || col < startCoords.col || col > endCoords.col {
		return 0, 0, false
	}
	return row, col, true
}

func arrayRangeTargetRef(start, end string, ctx Context) (string, bool) {
	row, col, ok := arrayRangeTargetCoords(start, end, ctx)
	if !ok || row == 0 || col == 0 {
		return "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(col), row), true
}

func currentArrayOffsets(ctx Context) (int, int, bool) {
	currentRow, currentCol := parseCellRef(ctx.CurrentRef())
	anchorRow, anchorCol := parseCellRef(ctx.ArrayAnchorRef())
	if currentRow == 0 || currentCol == 0 || anchorRow == 0 || anchorCol == 0 {
		return 0, 0, false
	}
	return currentRow - anchorRow, currentCol - anchorCol, true
}

func indexTargetRange(expr Expression) (string, string, bool) {
	switch expr := expr.(type) {
	case *RangeExpr:
		return expr.Start, expr.End, true
	case *CellRefExpr:
		return expr.Ref, expr.Ref, true
	default:
		return "", "", false
	}
}

func indexSelectorNumber(expr Expression, ctx Context) (int, bool) {
	val := expr.Eval(ctx)
	if val.IsError() {
		return 0, false
	}
	return indexSelectorValueNumber(val)
}

func indexSelectorNumberAtCurrentOffset(expr Expression, ctx Context) (int, bool) {
	val := expressionValueAtCurrentOffset(expr, ctx)
	if val.IsError() {
		return 0, false
	}
	return indexSelectorValueNumber(val)
}

func indexSelectorValueNumber(val Value) (int, bool) {
	num, err := val.ToNumber()
	if err != nil {
		return 0, false
	}
	return int(num), true
}

func indexRangeTargetRef(start, end string, rowNum, colNum int, ctx Context) (string, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", false
	}
	rowCount := endCoords.row - startCoords.row + 1
	colCount := endCoords.col - startCoords.col + 1
	sourceRow := startCoords.row
	sourceCol := startCoords.col
	switch {
	case rowNum == 0 && colNum == 0:
		rowOffset, colOffset, _ := currentArrayOffsets(ctx)
		sourceRow += rowOffset
		sourceCol += colOffset
	case rowNum == 0:
		if colNum < 1 || colNum > colCount {
			return "", false
		}
		rowOffset, _, _ := currentArrayOffsets(ctx)
		sourceRow += rowOffset
		sourceCol += colNum - 1
	case colNum == 0:
		if rowNum < 1 || rowNum > rowCount {
			return "", false
		}
		_, colOffset, _ := currentArrayOffsets(ctx)
		sourceRow += rowNum - 1
		sourceCol += colOffset
	default:
		if rowNum < 1 || rowNum > rowCount || colNum < 1 || colNum > colCount {
			return "", false
		}
		sourceRow += rowNum - 1
		sourceCol += colNum - 1
	}
	if sourceRow < startCoords.row || sourceRow > endCoords.row || sourceCol < startCoords.col || sourceCol > endCoords.col {
		return "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(sourceCol), sourceRow), true
}

func transposeRangeTargetRef(start, end string, ctx Context) (string, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", false
	}
	rowOffset, colOffset, _ := currentArrayOffsets(ctx)
	sourceRow := startCoords.row + colOffset
	sourceCol := startCoords.col + rowOffset
	if sourceRow < startCoords.row || sourceRow > endCoords.row || sourceCol < startCoords.col || sourceCol > endCoords.col {
		return "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(sourceCol), sourceRow), true
}

func offsetRangeRefs(start, end string, rowOffset, colOffset, height, width int) (string, string, bool) {
	startCoords, _, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", "", false
	}
	topRow := startCoords.row + rowOffset
	leftCol := startCoords.col + colOffset
	bottomRow := topRow + height - 1
	rightCol := leftCol + width - 1
	if topRow <= 0 || leftCol <= 0 || bottomRow < topRow || rightCol < leftCol {
		return "", "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(leftCol), topRow), startPrefix + fmt.Sprintf("%s%d", colToLetters(rightCol), bottomRow), true
}

func offsetRangeTargetRef(start, end string, ctx Context) (string, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", false
	}
	currentRow, currentCol := parseCellRef(ctx.CurrentRef())
	anchorRow, anchorCol := parseCellRef(ctx.ArrayAnchorRef())
	sourceRow, sourceCol := startCoords.row, startCoords.col
	if currentRow != 0 && currentCol != 0 && anchorRow != 0 && anchorCol != 0 {
		sourceRow += currentRow - anchorRow
		sourceCol += currentCol - anchorCol
	}
	if sourceRow < startCoords.row || sourceRow > endCoords.row || sourceCol < startCoords.col || sourceCol > endCoords.col {
		return "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(sourceCol), sourceRow), true
}

func chooseAxisSelectors(args []Expression, ctx Context) ([]int, bool) {
	indices := make([]int, 0, len(args))
	for _, arg := range args {
		values := expandArgumentValues(arg, arraySourceContext(ctx))
		if len(values) == 0 {
			return nil, false
		}
		for _, value := range values {
			if value.IsError() {
				return nil, false
			}
			num, err := value.ToNumber()
			if err != nil {
				return nil, false
			}
			idx := int(num)
			if idx == 0 {
				return nil, false
			}
			indices = append(indices, idx)
		}
	}
	if len(indices) == 0 {
		return nil, false
	}
	return indices, true
}

func chooseAxisTargetRef(start, end string, indices []int, selectRows bool, ctx Context) (string, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", false
	}
	rowCount := endCoords.row - startCoords.row + 1
	colCount := endCoords.col - startCoords.col + 1
	rowOffset, colOffset, _ := currentArrayOffsets(ctx)
	sourceRow := startCoords.row + rowOffset
	sourceCol := startCoords.col + colOffset
	if selectRows {
		if rowOffset < 0 || rowOffset >= len(indices) {
			return "", false
		}
		resolved, ok := resolveChooseAxisIndex(indices[rowOffset], rowCount)
		if !ok || colOffset < 0 || colOffset >= colCount {
			return "", false
		}
		sourceRow = startCoords.row + resolved
		sourceCol = startCoords.col + colOffset
	} else {
		if colOffset < 0 || colOffset >= len(indices) {
			return "", false
		}
		resolved, ok := resolveChooseAxisIndex(indices[colOffset], colCount)
		if !ok || rowOffset < 0 || rowOffset >= rowCount {
			return "", false
		}
		sourceRow = startCoords.row + rowOffset
		sourceCol = startCoords.col + resolved
	}
	if sourceRow < startCoords.row || sourceRow > endCoords.row || sourceCol < startCoords.col || sourceCol > endCoords.col {
		return "", false
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(sourceCol), sourceRow), true
}

func takeDropTargetRef(args []Expression, take bool, ctx Context) (string, bool, Value) {
	if len(args) < 2 || len(args) > 3 {
		return "", false, Error(ErrValue)
	}
	start, end, ok := indexTargetRange(args[0])
	if !ok {
		return "", false, Error(ErrRef)
	}
	rowCount, ok := indexSelectorNumber(args[1], ctx)
	if !ok {
		return "", false, Error(ErrValue)
	}
	colCount := 0
	if len(args) == 3 {
		colValue, ok := indexSelectorNumber(args[2], ctx)
		if !ok {
			return "", false, Error(ErrValue)
		}
		colCount = colValue
	}
	targetStart, targetEnd, ok := takeDropRangeRefs(start, end, rowCount, colCount, take)
	if !ok {
		return "", false, Error(ErrValue)
	}
	targetRef, ok := offsetRangeTargetRef(targetStart, targetEnd, ctx)
	if !ok || targetRef == "" {
		return "", false, Error(ErrRef)
	}
	return targetRef, true, Value{}
}

func takeDropRangeRefs(start, end string, rowCount, colCount int, take bool) (string, string, bool) {
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return "", "", false
	}
	rows := endCoords.row - startCoords.row + 1
	cols := endCoords.col - startCoords.col + 1
	rowStart, rowEnd, ok := resolveTakeDropBounds(startCoords.row, rows, rowCount, take)
	if !ok {
		return "", "", false
	}
	colStart, colEnd := startCoords.col, endCoords.col
	if colCount != 0 {
		colStart, colEnd, ok = resolveTakeDropBounds(startCoords.col, cols, colCount, take)
		if !ok {
			return "", "", false
		}
	}
	startPrefix, _ := splitQualifiedRangeRef(start)
	endPrefix, _ := splitQualifiedRangeRef(end)
	if endPrefix == "" {
		endPrefix = startPrefix
	}
	if startPrefix != endPrefix {
		return "", "", false
	}
	return startPrefix + fmt.Sprintf("%s%d", colToLetters(colStart), rowStart), startPrefix + fmt.Sprintf("%s%d", colToLetters(colEnd), rowEnd), true
}

func resolveTakeDropBounds(base, size, count int, take bool) (int, int, bool) {
	if size <= 0 || count == 0 || absInt(count) > size {
		return 0, 0, false
	}
	if take {
		if count > 0 {
			return base, base + count - 1, true
		}
		return base + size + count, base + size - 1, true
	}
	if count > 0 {
		return base + count, base + size - 1, true
	}
	return base, base + size + count - 1, true
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func resolveChooseAxisIndex(index int, size int) (int, bool) {
	if size <= 0 || index == 0 {
		return 0, false
	}
	if index < 0 {
		index = size + index + 1
	}
	if index < 1 || index > size {
		return 0, false
	}
	return index - 1, true
}

// scalarMath1 applies a single-argument math function element-wise.
func scalarMath1(args []Expression, ctx Context, fn func(float64) float64) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(fn(n))
}

// fnSin returns the sine of an angle in radians.
func fnSin(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Sin) }

// fnCos returns the cosine of an angle in radians.
func fnCos(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Cos) }

// fnTan returns the tangent of an angle in radians.
func fnTan(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Tan) }

// fnAtan returns the arc tangent of a value (result in radians).
func fnAtan(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Atan) }

// fnDegrees converts radians to degrees.
func fnDegrees(args []Expression, ctx Context) Value {
	return scalarMath1(args, ctx, func(r float64) float64 { return r * 180 / math.Pi })
}

// fnRadians converts degrees to radians.
func fnRadians(args []Expression, ctx Context) Value {
	return scalarMath1(args, ctx, func(d float64) float64 { return d * math.Pi / 180 })
}

// fnSinh returns the hyperbolic sine.
func fnSinh(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Sinh) }

// fnCosh returns the hyperbolic cosine.
func fnCosh(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Cosh) }

// fnTanh returns the hyperbolic tangent.
func fnTanh(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Tanh) }

// fnAsinh returns the inverse hyperbolic sine.
func fnAsinh(args []Expression, ctx Context) Value { return scalarMath1(args, ctx, math.Asinh) }

// fnAsin returns the arc sine of a value (result in radians).
func fnAsin(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if n < -1 || n > 1 {
		return Error(ErrNum)
	}
	return Number(math.Asin(n))
}

// fnAcos returns the arc cosine of a value (result in radians).
func fnAcos(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if n < -1 || n > 1 {
		return Error(ErrNum)
	}
	return Number(math.Acos(n))
}

// fnAcosh returns the inverse hyperbolic cosine (requires x >= 1).
func fnAcosh(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if n < 1 {
		return Error(ErrNum)
	}
	return Number(math.Acosh(n))
}

// fnAtanh returns the inverse hyperbolic tangent (requires -1 < x < 1).
func fnAtanh(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if n <= -1 || n >= 1 {
		return Error(ErrNum)
	}
	return Number(math.Atanh(n))
}

// fnAtan2 returns the arc tangent of y/x (Excel argument order: x, y).
func fnAtan2(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xVal := args[0].Eval(ctx)
	if xVal.IsError() {
		return xVal
	}
	yVal := args[1].Eval(ctx)
	if yVal.IsError() {
		return yVal
	}
	x, err := xVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	y, err := yVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if x == 0 && y == 0 {
		return Error(ErrDiv0)
	}
	return Number(math.Atan2(y, x))
}

// fnVLookup performs a vertical lookup in a table range.
func fnVLookup(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	lookupValue := expressionValueAtProjectedCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return lookupValue
	}
	tableValues, shape, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return errValue
	}
	colIdxValue := expressionValueAtProjectedCurrentOffset(args[2], ctx)
	if colIdxValue.IsError() {
		return Error(ErrValue)
	}
	colIdx, ok := indexSelectorValueNumber(colIdxValue)
	if !ok || colIdx < 1 {
		return Error(ErrValue)
	}
	if colIdx > shape.cols {
		return Error(ErrRef)
	}
	rangeLookup := true
	if len(args) == 4 {
		rl := expressionValueAtProjectedCurrentOffset(args[3], ctx)
		if rl.IsError() {
			return rl
		}
		b, err := rl.ToBool()
		if err != nil {
			return Error(ErrValue)
		}
		rangeLookup = b
	}
	firstCol := make([]Value, shape.rows)
	for i := 0; i < shape.rows; i++ {
		firstCol[i] = tableValues[i*shape.cols]
	}
	matchMode := 0
	searchMode := 1
	if rangeLookup {
		matchMode = -1
		searchMode = 2
	} else if hasLookupWildcard(lookupValue) {
		matchMode = 2
	}
	idx, errVal, matched := xmatchIndex(lookupValue, firstCol, matchMode, searchMode)
	if !matched {
		return errVal
	}
	return tableValues[idx*shape.cols+(colIdx-1)]
}

// fnHLookup performs a horizontal lookup in a table range.
func fnHLookup(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	lookupValue := expressionValueAtProjectedCurrentOffset(args[0], ctx)
	if lookupValue.IsError() {
		return lookupValue
	}
	tableValues, shape, ok, errValue := stackSourceValues(args[1], ctx)
	if !ok {
		return errValue
	}
	rowIdxValue := expressionValueAtProjectedCurrentOffset(args[2], ctx)
	if rowIdxValue.IsError() {
		return Error(ErrValue)
	}
	rowIdx, ok := indexSelectorValueNumber(rowIdxValue)
	if !ok || rowIdx < 1 {
		return Error(ErrValue)
	}
	if rowIdx > shape.rows {
		return Error(ErrRef)
	}
	rangeLookup := true
	if len(args) == 4 {
		rl := expressionValueAtProjectedCurrentOffset(args[3], ctx)
		if rl.IsError() {
			return rl
		}
		b, err := rl.ToBool()
		if err != nil {
			return Error(ErrValue)
		}
		rangeLookup = b
	}
	firstRow := tableValues[:shape.cols]
	matchMode := 0
	searchMode := 1
	if rangeLookup {
		matchMode = -1
		searchMode = 2
	} else if hasLookupWildcard(lookupValue) {
		matchMode = 2
	}
	idx, errVal, matched := xmatchIndex(lookupValue, firstRow, matchMode, searchMode)
	if !matched {
		return errVal
	}
	return tableValues[(rowIdx-1)*shape.cols+idx]
}

// fnMinIfs returns the minimum value in a range where corresponding criteria are met.
func fnMinIfs(args []Expression, ctx Context) Value {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return Error(ErrValue)
	}
	minValues := expandArgumentValues(args[0], ctx)
	criteria, targetLen, errValue, ok := buildCriteriaEvaluators(args, 1, ctx)
	if !ok {
		return errValue
	}
	if len(minValues) != targetLen {
		return Error(ErrValue)
	}
	result := math.Inf(1)
	matched := false
	for idx := 0; idx < targetLen; idx++ {
		match, errVal := criteriaMatchAt(criteria, idx)
		if errVal.IsError() {
			return errVal
		}
		if !match {
			continue
		}
		v := minValues[idx]
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err != nil {
			continue
		}
		if !matched || num < result {
			result = num
			matched = true
		}
	}
	if !matched {
		return Number(0)
	}
	return Number(result)
}

// fnMaxIfs returns the maximum value in a range where corresponding criteria are met.
func fnMaxIfs(args []Expression, ctx Context) Value {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return Error(ErrValue)
	}
	maxValues := expandArgumentValues(args[0], ctx)
	criteria, targetLen, errValue, ok := buildCriteriaEvaluators(args, 1, ctx)
	if !ok {
		return errValue
	}
	if len(maxValues) != targetLen {
		return Error(ErrValue)
	}
	result := math.Inf(-1)
	matched := false
	for idx := 0; idx < targetLen; idx++ {
		match, errVal := criteriaMatchAt(criteria, idx)
		if errVal.IsError() {
			return errVal
		}
		if !match {
			continue
		}
		v := maxValues[idx]
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err != nil {
			continue
		}
		if !matched || num > result {
			result = num
			matched = true
		}
	}
	if !matched {
		return Number(0)
	}
	return Number(result)
}

// sampleStats computes the mean and sample variance (n-1 denominator).
func sampleStats(numbers []float64) (float64, float64) {
	n := float64(len(numbers))
	mean := 0.0
	for _, v := range numbers {
		mean += v
	}
	mean /= n
	variance := 0.0
	for _, v := range numbers {
		d := v - mean
		variance += d * d
	}
	variance /= n - 1
	return mean, variance
}

// fnStdev returns the sample standard deviation (n-1 denominator).
func fnStdev(args []Expression, ctx Context) Value {
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) < 2 {
		return Error(ErrDiv0)
	}
	_, variance := sampleStats(numbers)
	return Number(math.Sqrt(variance))
}

// fnStdevp returns the population standard deviation (n denominator).
func fnStdevp(args []Expression, ctx Context) Value {
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) == 0 {
		return Error(ErrDiv0)
	}
	mean := 0.0
	for _, v := range numbers {
		mean += v
	}
	mean /= float64(len(numbers))
	variance := 0.0
	for _, v := range numbers {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(numbers))
	return Number(math.Sqrt(variance))
}

// fnVar returns the sample variance (n-1 denominator).
func fnVar(args []Expression, ctx Context) Value {
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) < 2 {
		return Error(ErrDiv0)
	}
	_, variance := sampleStats(numbers)
	return Number(variance)
}

// fnVarp returns the population variance (n denominator).
func fnVarp(args []Expression, ctx Context) Value {
	numbers, errValue, ok := numericValues(args, ctx)
	if !ok {
		return errValue
	}
	if len(numbers) == 0 {
		return Error(ErrDiv0)
	}
	mean := 0.0
	for _, v := range numbers {
		mean += v
	}
	mean /= float64(len(numbers))
	variance := 0.0
	for _, v := range numbers {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(numbers))
	return Number(variance)
}

// fnMaxA returns the maximum value, treating TRUE as 1 and FALSE as 0.
func fnMaxA(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	values := collectValues(args, ctx)
	result := math.Inf(-1)
	hasValue := false
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err != nil {
			continue
		}
		if num > result {
			result = num
		}
		hasValue = true
	}
	if !hasValue {
		return Number(0)
	}
	return Number(result)
}

// fnMinA returns the minimum value, treating TRUE as 1 and FALSE as 0.
func fnMinA(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	values := collectValues(args, ctx)
	result := math.Inf(1)
	hasValue := false
	for _, v := range values {
		if v.IsError() {
			return v
		}
		num, err := v.ToNumber()
		if err != nil {
			continue
		}
		if num < result {
			result = num
		}
		hasValue = true
	}
	if !hasValue {
		return Number(0)
	}
	return Number(result)
}

// edateAddMonths adds months to a year/month pair, returning a clamped day.
func edateAddMonths(t time.Time, months int) time.Time {
	year := t.Year()
	month := int(t.Month()) - 1 + months // 0-based month
	year += month / 12
	month = month % 12
	if month < 0 {
		month += 12
		year--
	}
	month++ // back to 1-based
	day := t.Day()
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// fnEdate returns the serial number of the date a given number of months away from start_date.
func fnEdate(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	startVal := args[0].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	monthsVal := args[1].Eval(ctx)
	if monthsVal.IsError() {
		return monthsVal
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	monthsN, err := monthsVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	result := edateAddMonths(serialToTime(startN), int(math.Trunc(monthsN)))
	serial := timeToSerial(result)
	if serial < 0 {
		return Error(ErrNum)
	}
	return Number(serial)
}

// fnEomonth returns the serial number of the last day of the month, months offset from start_date.
func fnEomonth(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	startVal := args[0].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	monthsVal := args[1].Eval(ctx)
	if monthsVal.IsError() {
		return monthsVal
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	monthsN, err := monthsVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	t := serialToTime(startN)
	months := int(math.Trunc(monthsN))
	year := t.Year()
	month := int(t.Month()) - 1 + months // 0-based
	year += month / 12
	month = month % 12
	if month < 0 {
		month += 12
		year--
	}
	month++ // 1-based
	// Day 0 of the next month = last day of this month
	result := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
	serial := timeToSerial(result)
	if serial < 0 {
		return Error(ErrNum)
	}
	return Number(serial)
}

// fnTime returns the serial fraction for the given hour, minute, second.
func fnTime(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	hourVal := args[0].Eval(ctx)
	if hourVal.IsError() {
		return hourVal
	}
	minVal := args[1].Eval(ctx)
	if minVal.IsError() {
		return minVal
	}
	secVal := args[2].Eval(ctx)
	if secVal.IsError() {
		return secVal
	}
	h, err := hourVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	m, err := minVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	s, err := secVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	totalSeconds := math.Trunc(h)*3600 + math.Trunc(m)*60 + math.Trunc(s)
	fraction := math.Mod(totalSeconds/86400, 1)
	if fraction < 0 {
		fraction += 1
	}
	return Number(fraction)
}

// fnDatevalue parses a date text string into an Excel serial date.
func fnDatevalue(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	text := strings.TrimSpace(val.ToText())
	if text == "" {
		return Error(ErrValue)
	}
	formats := []string{
		"1/2/2006", "01/02/2006", "1-2-2006", "01-02-2006",
		"2006-01-02", "January 2, 2006", "Jan 2, 2006",
		"2 January 2006", "2 Jan 2006",
		"2006January2", "2January2006",
		"2006Jan2", "2Jan2006",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, text); err == nil {
			serial := timeToSerial(t)
			if serial < 0 {
				return Error(ErrValue)
			}
			return Number(serial)
		}
	}
	// Try with all spaces removed (handles e.g. "2022APRIL 1" from concatenated cells)
	noSpace := strings.ReplaceAll(text, " ", "")
	if noSpace != text {
		for _, layout := range formats {
			if t, err := time.Parse(layout, noSpace); err == nil {
				serial := timeToSerial(t)
				if serial < 0 {
					return Error(ErrValue)
				}
				return Number(serial)
			}
		}
	}
	return Error(ErrValue)
}

// fnTimevalue parses a time text string into an Excel serial fraction.
func fnTimevalue(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	text := strings.TrimSpace(val.ToText())
	formats := []string{"3:04 PM", "3:04:05 PM", "15:04", "15:04:05", "3:04PM", "3:04:05PM"}
	for _, layout := range formats {
		if t, err := time.Parse(layout, text); err == nil {
			h := float64(t.Hour())
			m := float64(t.Minute())
			s := float64(t.Second())
			return Number((h*3600 + m*60 + s) / 86400)
		}
	}
	return Error(ErrValue)
}

// fnDatedif computes the difference between two dates in the specified unit.
func fnDatedif(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	startVal := args[0].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	endVal := args[1].Eval(ctx)
	if endVal.IsError() {
		return endVal
	}
	unitVal := args[2].Eval(ctx)
	if unitVal.IsError() {
		return unitVal
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	endN, err := endVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if startN > endN {
		return Error(ErrNum)
	}
	start := serialToTime(startN)
	end := serialToTime(endN)
	unit := strings.ToUpper(strings.TrimSpace(unitVal.ToText()))
	switch unit {
	case "Y":
		years := end.Year() - start.Year()
		if end.Month() < start.Month() || (end.Month() == start.Month() && end.Day() < start.Day()) {
			years--
		}
		return Number(float64(years))
	case "M":
		months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
		if end.Day() < start.Day() {
			months--
		}
		return Number(float64(months))
	case "D":
		return Number(math.Trunc(endN - startN))
	case "MD":
		d := end.Day() - start.Day()
		if d < 0 {
			prevMonth := time.Date(end.Year(), end.Month(), 0, 0, 0, 0, 0, time.UTC)
			d += prevMonth.Day()
		}
		return Number(float64(d))
	case "YM":
		months := int(end.Month()) - int(start.Month())
		if months < 0 {
			months += 12
		}
		if end.Day() < start.Day() {
			months--
			if months < 0 {
				months += 12
			}
		}
		return Number(float64(months))
	case "YD":
		startAdj := time.Date(end.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		if startAdj.After(end) {
			startAdj = time.Date(end.Year()-1, start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		}
		days := end.Sub(startAdj).Hours() / 24
		return Number(math.Trunc(days))
	default:
		return Error(ErrValue)
	}
}

// fnWeeknum returns the week number of the year for a given date.
// return_type 1 (default): week begins Sunday; 2: week begins Monday.
func fnWeeknum(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	returnType := 1
	if len(args) == 2 {
		rtVal := args[1].Eval(ctx)
		if rtVal.IsError() {
			return rtVal
		}
		rt, err := rtVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		returnType = int(rt)
	}
	t := serialToTime(n)
	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	dayOfYear := t.YearDay()
	jan1Weekday := int(jan1.Weekday()) // 0=Sun
	weekStart := 0
	if returnType == 2 {
		weekStart = 1
	}
	offset := (jan1Weekday - weekStart + 7) % 7
	return Number(float64((dayOfYear+offset-1)/7 + 1))
}

// fnIsoweeknum returns the ISO 8601 week number for a date.
func fnIsoweeknum(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	_, week := serialToTime(n).ISOWeek()
	return Number(float64(week))
}

// isWeekend reports whether t falls on a Saturday or Sunday.
func isWeekend(t time.Time) bool {
	w := t.Weekday()
	return w == time.Saturday || w == time.Sunday
}

// fnWorkday returns the workday count days from start_date, skipping weekends and holidays.
func fnWorkday(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	startVal := args[0].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	daysVal := args[1].Eval(ctx)
	if daysVal.IsError() {
		return daysVal
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	daysN, err := daysVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	days := int(math.Trunc(daysN))
	holidays := make(map[float64]bool)
	if len(args) == 3 {
		for _, hv := range expandArgumentValues(args[2], ctx) {
			if hv.IsError() {
				return hv
			}
			h, err := hv.ToNumber()
			if err == nil {
				holidays[math.Trunc(h)] = true
			}
		}
	}
	current := serialToTime(math.Trunc(startN))
	dir := 1
	if days < 0 {
		dir = -1
		days = -days
	}
	for days > 0 {
		current = current.AddDate(0, 0, dir)
		if isWeekend(current) {
			continue
		}
		if holidays[math.Trunc(timeToSerial(current))] {
			continue
		}
		days--
	}
	return Number(timeToSerial(current))
}

// fnNetworkdays returns the number of working days between two dates (inclusive).
func fnNetworkdays(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	startVal := args[0].Eval(ctx)
	if startVal.IsError() {
		return startVal
	}
	endVal := args[1].Eval(ctx)
	if endVal.IsError() {
		return endVal
	}
	startN, err := startVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	endN, err := endVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	holidays := make(map[float64]bool)
	if len(args) == 3 {
		for _, hv := range expandArgumentValues(args[2], ctx) {
			if hv.IsError() {
				return hv
			}
			h, err := hv.ToNumber()
			if err == nil {
				holidays[math.Trunc(h)] = true
			}
		}
	}
	sign := 1.0
	if startN > endN {
		startN, endN = endN, startN
		sign = -1.0
	}
	start := serialToTime(math.Trunc(startN))
	end := serialToTime(math.Trunc(endN))
	count := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if isWeekend(d) {
			continue
		}
		if holidays[math.Trunc(timeToSerial(d))] {
			continue
		}
		count++
	}
	return Number(sign * float64(count))
}

// gcdInt returns the greatest common divisor of two non-negative integers.
func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// fnGcd returns the greatest common divisor of the given values.
func fnGcd(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	values := collectValues(args, ctx)
	result := 0
	for _, v := range values {
		if v.IsError() {
			return v
		}
		n, err := v.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		if n < 0 || math.Trunc(n) != n {
			return Error(ErrNum)
		}
		result = gcdInt(result, int(n))
	}
	return Number(float64(result))
}

// fnLcm returns the least common multiple of the given values.
func fnLcm(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	values := collectValues(args, ctx)
	result := 1
	for _, v := range values {
		if v.IsError() {
			return v
		}
		n, err := v.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		if n < 0 || math.Trunc(n) != n {
			return Error(ErrNum)
		}
		in := int(n)
		if in == 0 {
			return Number(0)
		}
		g := gcdInt(result, in)
		result = result / g * in
	}
	return Number(float64(result))
}

// fnFact returns the factorial of n.
func fnFact(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	n, err := val.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	ni := int(math.Trunc(n))
	if ni < 0 || ni > 170 {
		return Error(ErrNum)
	}
	result := 1.0
	for i := 2; i <= ni; i++ {
		result *= float64(i)
	}
	return Number(result)
}

// fnCombin returns the number of combinations C(n, k).
func fnCombin(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	nVal := args[0].Eval(ctx)
	if nVal.IsError() {
		return nVal
	}
	kVal := args[1].Eval(ctx)
	if kVal.IsError() {
		return kVal
	}
	n, err := nVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	k, err := kVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	ni, ki := int(math.Trunc(n)), int(math.Trunc(k))
	if ni < 0 || ki < 0 || ki > ni {
		return Error(ErrNum)
	}
	if ki > ni-ki {
		ki = ni - ki
	}
	result := 1.0
	for i := 0; i < ki; i++ {
		result = result * float64(ni-i) / float64(i+1)
	}
	return Number(math.Round(result))
}

// fnPermut returns the number of permutations P(n, k).
func fnPermut(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	nVal := args[0].Eval(ctx)
	if nVal.IsError() {
		return nVal
	}
	kVal := args[1].Eval(ctx)
	if kVal.IsError() {
		return kVal
	}
	n, err := nVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	k, err := kVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	ni, ki := int(math.Trunc(n)), int(math.Trunc(k))
	if ni < 0 || ki < 0 || ki > ni {
		return Error(ErrNum)
	}
	result := 1.0
	for i := 0; i < ki; i++ {
		result *= float64(ni - i)
	}
	return Number(result)
}

// fnMround rounds a number to the nearest multiple of significance.
func fnMround(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	mulVal := args[1].Eval(ctx)
	if mulVal.IsError() {
		return mulVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	mul, err := mulVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if mul == 0 {
		return Number(0)
	}
	if (num < 0) != (mul < 0) {
		return Error(ErrNum)
	}
	return Number(math.Round(num/mul) * mul)
}

// fnQuotient returns the integer portion of a division.
func fnQuotient(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	divVal := args[1].Eval(ctx)
	if divVal.IsError() {
		return divVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	div, err := divVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if div == 0 {
		return Error(ErrDiv0)
	}
	return Number(math.Trunc(num / div))
}

// fnCeilingMath rounds a number up toward positive infinity (or toward zero for negatives when mode≠0).
func fnCeilingMath(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 3 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	significance := 1.0
	if num < 0 {
		significance = -1.0
	}
	if len(args) >= 2 {
		sigVal := args[1].Eval(ctx)
		if sigVal.IsError() {
			return sigVal
		}
		significance, err = sigVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		if significance == 0 {
			return Number(0)
		}
	}
	mode := 0
	if len(args) == 3 {
		modeVal := args[2].Eval(ctx)
		if modeVal.IsError() {
			return modeVal
		}
		modeN, err := modeVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		mode = int(modeN)
	}
	sig := math.Abs(significance)
	if num < 0 && mode != 0 {
		return Number(math.Floor(num/sig) * sig)
	}
	return Number(math.Ceil(num/sig) * sig)
}

// fnFloorMath rounds a number down toward negative infinity (or toward zero for negatives when mode≠0).
func fnFloorMath(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 3 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	significance := 1.0
	if num < 0 {
		significance = -1.0
	}
	if len(args) >= 2 {
		sigVal := args[1].Eval(ctx)
		if sigVal.IsError() {
			return sigVal
		}
		significance, err = sigVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		if significance == 0 {
			return Number(0)
		}
	}
	mode := 0
	if len(args) == 3 {
		modeVal := args[2].Eval(ctx)
		if modeVal.IsError() {
			return modeVal
		}
		modeN, err := modeVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		mode = int(modeN)
	}
	sig := math.Abs(significance)
	if num < 0 && mode != 0 {
		return Number(math.Ceil(num/sig) * sig)
	}
	return Number(math.Floor(num/sig) * sig)
}

// fnNumbervalue parses a number string using optional decimal and group separators.
func fnNumbervalue(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 3 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	if val.IsNumber() {
		return val
	}
	text := strings.TrimSpace(val.ToText())
	decSep := "."
	if len(args) >= 2 {
		d := args[1].Eval(ctx)
		if d.IsError() {
			return d
		}
		ds := d.ToText()
		if len(ds) != 1 {
			return Error(ErrValue)
		}
		decSep = ds
	}
	groupSep := ""
	if len(args) == 3 {
		g := args[2].Eval(ctx)
		if g.IsError() {
			return g
		}
		groupSep = g.ToText()
		if len(groupSep) > 1 {
			return Error(ErrValue)
		}
	}
	if groupSep != "" {
		text = strings.ReplaceAll(text, groupSep, "")
	}
	if decSep != "." {
		text = strings.ReplaceAll(text, decSep, ".")
	}
	// Handle trailing %
	isPct := strings.HasSuffix(text, "%")
	if isPct {
		text = text[:len(text)-1]
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return Error(ErrValue)
	}
	if isPct {
		n /= 100
	}
	return Number(n)
}

// fnClean removes all non-printable characters (code points below 32, except tab) from text.
func fnClean(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	var b strings.Builder
	for _, r := range val.ToText() {
		if r >= 32 || r == 9 {
			b.WriteRune(r)
		}
	}
	return Text(b.String())
}

// addThousandsSeparator inserts comma separators into the integer portion of a number string.
func addThousandsSeparator(s string) string {
	dotIdx := strings.Index(s, ".")
	intPart := s
	fracPart := ""
	if dotIdx >= 0 {
		intPart = s[:dotIdx]
		fracPart = s[dotIdx:]
	}
	negative := strings.HasPrefix(intPart, "-")
	if negative {
		intPart = intPart[1:]
	}
	var result []byte
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	out := string(result)
	if negative {
		out = "-" + out
	}
	return out + fracPart
}

// fnFixed formats a number as a fixed-point string with optional thousand separators.
func fnFixed(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 3 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	decimals := 2
	if len(args) >= 2 {
		dVal := args[1].Eval(ctx)
		if dVal.IsError() {
			return dVal
		}
		d, err := dVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		decimals = int(math.Trunc(d))
	}
	noCommas := false
	if len(args) == 3 {
		ncVal := args[2].Eval(ctx)
		if ncVal.IsError() {
			return ncVal
		}
		b, err := ncVal.ToBool()
		if err != nil {
			return Error(ErrValue)
		}
		noCommas = b
	}
	if decimals < 0 {
		factor := math.Pow(10, float64(-decimals))
		num = math.Round(num/factor) * factor
		decimals = 0
	}
	formatted := strconv.FormatFloat(num, 'f', decimals, 64)
	if !noCommas {
		formatted = addThousandsSeparator(formatted)
	}
	return Text(formatted)
}

// fnDollar formats a number as a currency string prefixed with "$".
func fnDollar(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	numVal := args[0].Eval(ctx)
	if numVal.IsError() {
		return numVal
	}
	num, err := numVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	decimals := 2
	if len(args) == 2 {
		dVal := args[1].Eval(ctx)
		if dVal.IsError() {
			return dVal
		}
		d, err := dVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		decimals = int(math.Trunc(d))
	}
	if decimals < 0 {
		factor := math.Pow(10, float64(-decimals))
		num = math.Round(num/factor) * factor
		decimals = 0
	}
	negative := num < 0
	formatted := strconv.FormatFloat(math.Abs(num), 'f', decimals, 64)
	formatted = addThousandsSeparator(formatted)
	if negative {
		return Text("($" + formatted + ")")
	}
	return Text("$" + formatted)
}

func financialNumberArg(expr Expression, ctx Context) (float64, Value, bool) {
	val := expr.Eval(ctx)
	if val.IsError() {
		return 0, val, false
	}
	num, err := val.ToNumber()
	if err != nil {
		return 0, Error(ErrValue), false
	}
	return num, Value{}, true
}

func financialOptionalNumberArg(args []Expression, index int, defaultValue float64, ctx Context) (float64, Value, bool) {
	if index >= len(args) {
		return defaultValue, Value{}, true
	}
	return financialNumberArg(args[index], ctx)
}

func financialPaymentTypeArg(args []Expression, index int, ctx Context) (float64, Value, bool) {
	paymentType, errValue, ok := financialOptionalNumberArg(args, index, 0, ctx)
	if !ok {
		return 0, errValue, false
	}
	paymentType = math.Trunc(paymentType)
	if paymentType != 0 && paymentType != 1 {
		return 0, Error(ErrNum), false
	}
	return paymentType, Value{}, true
}

// fnPmt returns the periodic payment for an annuity.
func fnPmt(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 3, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 4, ctx)
	if !ok {
		return errValue
	}
	if nper == 0 {
		return Error(ErrNum)
	}
	if rate == 0 {
		return Number(-(pv + fv) / nper)
	}
	factor := math.Pow(1+rate, nper)
	denom := (1 + rate*paymentType) * (factor - 1)
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number(-(fv + pv*factor) * rate / denom)
}

// fnPv returns the present value of an annuity.
func fnPv(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pmt, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 3, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 4, ctx)
	if !ok {
		return errValue
	}
	if rate == 0 {
		return Number(-(fv + pmt*nper))
	}
	factor := math.Pow(1+rate, nper)
	return Number(-(fv + pmt*(1+rate*paymentType)*(factor-1)/rate) / factor)
}

// fnFv returns the future value of an annuity.
func fnFv(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pmt, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialOptionalNumberArg(args, 3, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 4, ctx)
	if !ok {
		return errValue
	}
	if rate == 0 {
		return Number(-(pv + pmt*nper))
	}
	factor := math.Pow(1+rate, nper)
	return Number(-(pv*factor + pmt*(1+rate*paymentType)*(factor-1)/rate))
}

// fnNper returns the number of periods for an annuity.
func fnNper(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	pmt, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 3, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 4, ctx)
	if !ok {
		return errValue
	}
	if rate == 0 {
		if pmt == 0 {
			return Error(ErrDiv0)
		}
		return Number(-(pv + fv) / pmt)
	}
	base := 1 + rate
	if base <= 0 {
		return Error(ErrNum)
	}
	term := pmt * (1 + rate*paymentType)
	num := term - fv*rate
	den := pv*rate + term
	if num == 0 || den == 0 || num/den <= 0 {
		return Error(ErrNum)
	}
	return Number(math.Log(num/den) / math.Log(base))
}

func annuityBalanceEquation(rate, nper, pmt, pv, fv, paymentType float64) float64 {
	if rate == 0 {
		return pv + pmt*nper + fv
	}
	factor := math.Pow(1+rate, nper)
	return pv*factor + pmt*(1+rate*paymentType)*(factor-1)/rate + fv
}

// fnRate returns the periodic interest rate for an annuity using Newton iteration.
func fnRate(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 6 {
		return Error(ErrValue)
	}
	nper, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	pmt, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 3, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 4, ctx)
	if !ok {
		return errValue
	}
	guess, errValue, ok := financialOptionalNumberArg(args, 5, 0.1, ctx)
	if !ok {
		return errValue
	}
	if nper <= 0 {
		return Error(ErrNum)
	}
	rate := guess
	for i := 0; i < 100; i++ {
		value := annuityBalanceEquation(rate, nper, pmt, pv, fv, paymentType)
		if math.Abs(value) < 1e-10 {
			return Number(rate)
		}
		step := 1e-7
		if math.Abs(rate) > 1 {
			step = math.Abs(rate) * 1e-7
		}
		derivative := (annuityBalanceEquation(rate+step, nper, pmt, pv, fv, paymentType) -
			annuityBalanceEquation(rate-step, nper, pmt, pv, fv, paymentType)) / (2 * step)
		if derivative == 0 || math.IsNaN(derivative) || math.IsInf(derivative, 0) {
			break
		}
		next := rate - value/derivative
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= -1 {
			break
		}
		if math.Abs(next-rate) < 1e-12 {
			return Number(next)
		}
		rate = next
	}
	return Error(ErrNum)
}

func annuityInterestAndPrincipal(rate, period, nper, pv, fv, paymentType float64) (float64, float64, bool) {
	if period < 1 || period > nper {
		return 0, 0, false
	}
	paymentVal := fnPmt([]Expression{
		&NumberExpr{Value: rate},
		&NumberExpr{Value: nper},
		&NumberExpr{Value: pv},
		&NumberExpr{Value: fv},
		&NumberExpr{Value: paymentType},
	}, ctxless())
	if !paymentVal.IsNumber() {
		return 0, 0, false
	}
	pmt := paymentVal.Num
	balance := pv
	targetPeriod := int(math.Trunc(period))
	totalPeriods := int(math.Trunc(nper))
	if float64(targetPeriod) != period || float64(totalPeriods) != nper {
		return 0, 0, false
	}
	for current := 1; current <= totalPeriods; current++ {
		if paymentType == 1 {
			if current == 1 {
				if current == targetPeriod {
					return 0, pmt, true
				}
				balance += pmt
				continue
			}
			interest := -balance * rate
			balance = balance*(1+rate) + pmt
			principal := pmt - interest
			if current == targetPeriod {
				return interest, principal, true
			}
			continue
		}
		interest := -balance * rate
		balance = balance*(1+rate) + pmt
		principal := pmt - interest
		if current == targetPeriod {
			return interest, principal, true
		}
	}
	return 0, 0, false
}

type stubContext struct{}

func ctxless() Context { return stubContext{} }

func (stubContext) GetCell(ref string) Value                        { return Empty() }
func (stubContext) GetRange(start, end string) []Value              { return nil }
func (stubContext) GetSpillRange(ref string) (string, string, bool) { return "", "", false }
func (stubContext) CurrentRef() string                              { return "" }
func (stubContext) ArrayAnchorRef() string                          { return "" }
func (stubContext) GetFormula(ref string) (string, bool)            { return "", false }
func (stubContext) ResolveSheetOrdinal(token string) (int, bool)    { return 0, false }
func (stubContext) SheetCount() int                                 { return 0 }

// fnIpmt returns the interest portion of a payment for a given period.
func fnIpmt(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 6 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	period, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 4, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 5, ctx)
	if !ok {
		return errValue
	}
	interest, _, ok := annuityInterestAndPrincipal(rate, period, nper, pv, fv, paymentType)
	if !ok {
		return Error(ErrNum)
	}
	return Number(interest)
}

// fnPpmt returns the principal portion of a payment for a given period.
func fnPpmt(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 6 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	period, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	fv, errValue, ok := financialOptionalNumberArg(args, 4, 0, ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 5, ctx)
	if !ok {
		return errValue
	}
	_, principal, ok := annuityInterestAndPrincipal(rate, period, nper, pv, fv, paymentType)
	if !ok {
		return Error(ErrNum)
	}
	return Number(principal)
}

// fnNpv returns the net present value of a series of cash flows.
func fnNpv(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	total := 0.0
	period := 1.0
	for _, arg := range args[1:] {
		for _, value := range expandArgumentValues(arg, ctx) {
			if value.IsError() {
				return value
			}
			num, err := value.ToNumber()
			if err != nil {
				continue
			}
			total += num / math.Pow(1+rate, period)
			period++
		}
	}
	return Number(total)
}

// fnEffect converts a nominal annual rate to an effective annual rate.
func fnEffect(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	nominalRate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	periods, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	periods = math.Trunc(periods)
	if nominalRate <= 0 || periods < 1 {
		return Error(ErrNum)
	}
	return Number(math.Pow(1+nominalRate/periods, periods) - 1)
}

// fnNominal converts an effective annual rate to a nominal annual rate.
func fnNominal(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	effectiveRate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	periods, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	periods = math.Trunc(periods)
	if effectiveRate <= 0 || periods < 1 {
		return Error(ErrNum)
	}
	return Number(periods * (math.Pow(1+effectiveRate, 1/periods) - 1))
}

// fnCumIpmt returns the cumulative interest paid between start_period and end_period.
func fnCumIpmt(args []Expression, ctx Context) Value {
	if len(args) != 6 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	startPeriod, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	endPeriod, errValue, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 5, ctx)
	if !ok {
		return errValue
	}
	if rate <= 0 || nper <= 0 || pv <= 0 {
		return Error(ErrNum)
	}
	start := int(math.Trunc(startPeriod))
	end := int(math.Trunc(endPeriod))
	total := int(math.Trunc(nper))
	if start < 1 || end < start || end > total {
		return Error(ErrNum)
	}
	sum := 0.0
	for p := start; p <= end; p++ {
		interest, _, ok := annuityInterestAndPrincipal(rate, float64(p), nper, pv, 0, paymentType)
		if !ok {
			return Error(ErrNum)
		}
		sum += interest
	}
	return Number(sum)
}

// fnCumPrinc returns the cumulative principal paid between start_period and end_period.
func fnCumPrinc(args []Expression, ctx Context) Value {
	if len(args) != 6 {
		return Error(ErrValue)
	}
	rate, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	nper, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	pv, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	startPeriod, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	endPeriod, errValue, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errValue
	}
	paymentType, errValue, ok := financialPaymentTypeArg(args, 5, ctx)
	if !ok {
		return errValue
	}
	if rate <= 0 || nper <= 0 || pv <= 0 {
		return Error(ErrNum)
	}
	start := int(math.Trunc(startPeriod))
	end := int(math.Trunc(endPeriod))
	total := int(math.Trunc(nper))
	if start < 1 || end < start || end > total {
		return Error(ErrNum)
	}
	sum := 0.0
	for p := start; p <= end; p++ {
		_, principal, ok := annuityInterestAndPrincipal(rate, float64(p), nper, pv, 0, paymentType)
		if !ok {
			return Error(ErrNum)
		}
		sum += principal
	}
	return Number(sum)
}

// fnSln returns the straight-line depreciation of an asset for one period.
func fnSln(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	cost, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	salvage, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	life, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	if life == 0 {
		return Error(ErrDiv0)
	}
	return Number((cost - salvage) / life)
}

// fnSyd returns the sum-of-years' digits depreciation for a given period.
func fnSyd(args []Expression, ctx Context) Value {
	if len(args) != 4 {
		return Error(ErrValue)
	}
	cost, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	salvage, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	life, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	per, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	if life <= 0 || per <= 0 || per > life {
		return Error(ErrNum)
	}
	denom := life * (life + 1)
	if denom == 0 {
		return Error(ErrNum)
	}
	return Number((cost - salvage) * (life - per + 1) * 2 / denom)
}

// fnDdb returns the double-declining balance depreciation for a given period.
func fnDdb(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	cost, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	salvage, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	life, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	period, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	factor, errValue, ok := financialOptionalNumberArg(args, 4, 2, ctx)
	if !ok {
		return errValue
	}
	if cost < 0 || salvage < 0 || life <= 0 || period <= 0 || period > life || factor <= 0 {
		return Error(ErrNum)
	}
	rate := factor / life
	bookValue := cost
	var dep float64
	for p := 1; p <= int(math.Trunc(period)); p++ {
		dep = math.Min(bookValue-salvage, bookValue*rate)
		if dep < 0 {
			dep = 0
		}
		if p < int(math.Trunc(period)) {
			bookValue -= dep
		}
	}
	return Number(dep)
}

// fnDb returns the declining-balance depreciation for a given period.
func fnDb(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	cost, errValue, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errValue
	}
	salvage, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	life, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	period, errValue, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errValue
	}
	month, errValue, ok := financialOptionalNumberArg(args, 4, 12, ctx)
	if !ok {
		return errValue
	}
	month = math.Trunc(month)
	if cost <= 0 || salvage < 0 || life <= 0 || period <= 0 || period > life+1 || month < 1 || month > 12 {
		return Error(ErrNum)
	}
	if salvage >= cost {
		return Number(0)
	}
	// DB rate rounded to 3 decimal places as per Excel spec.
	dbRate := math.Round((1-math.Pow(salvage/cost, 1/life))*1000) / 1000
	per := int(math.Trunc(period))
	totalPeriods := int(math.Trunc(life))
	cum := 0.0
	var dep float64
	for p := 1; p <= per; p++ {
		if p == 1 {
			dep = cost * dbRate * month / 12
		} else if p == totalPeriods+1 {
			dep = (cost - cum) * dbRate * (12 - month) / 12
		} else {
			dep = (cost - cum) * dbRate
		}
		if p < per {
			cum += dep
		}
	}
	return Number(dep)
}

// irrNpv evaluates NPV of irregular cash flows at a given rate for IRR iteration.
func irrNpv(rate float64, cashFlows []float64) float64 {
	total := 0.0
	for i, cf := range cashFlows {
		total += cf / math.Pow(1+rate, float64(i))
	}
	return total
}

// fnIrr returns the internal rate of return for a series of cash flows.
func fnIrr(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	var cashFlows []float64
	for _, value := range expandArgumentValues(args[0], ctx) {
		if value.IsError() {
			return value
		}
		if value.IsEmpty() {
			continue
		}
		num, err := value.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cashFlows = append(cashFlows, num)
	}
	if len(cashFlows) < 2 {
		return Error(ErrNum)
	}
	guess, errValue, ok := financialOptionalNumberArg(args, 1, 0.1, ctx)
	if !ok {
		return errValue
	}
	rate := guess
	for i := 0; i < 100; i++ {
		npvVal := irrNpv(rate, cashFlows)
		if math.Abs(npvVal) < 1e-10 {
			return Number(rate)
		}
		step := 1e-7
		deriv := (irrNpv(rate+step, cashFlows) - irrNpv(rate-step, cashFlows)) / (2 * step)
		if deriv == 0 || math.IsNaN(deriv) || math.IsInf(deriv, 0) {
			break
		}
		next := rate - npvVal/deriv
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= -1 {
			break
		}
		if math.Abs(next-rate) < 1e-12 {
			return Number(next)
		}
		rate = next
	}
	return Error(ErrNum)
}

// fnMirr returns the modified internal rate of return.
func fnMirr(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	var cashFlows []float64
	for _, value := range expandArgumentValues(args[0], ctx) {
		if value.IsError() {
			return value
		}
		if value.IsEmpty() {
			continue
		}
		num, err := value.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cashFlows = append(cashFlows, num)
	}
	if len(cashFlows) < 2 {
		return Error(ErrNum)
	}
	financeRate, errValue, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errValue
	}
	reinvestRate, errValue, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errValue
	}
	n := len(cashFlows)
	// PV of negative cash flows at finance_rate.
	pvNeg := 0.0
	// FV of positive cash flows at reinvest_rate.
	fvPos := 0.0
	for i, cf := range cashFlows {
		if cf < 0 {
			pvNeg += cf / math.Pow(1+financeRate, float64(i))
		} else if cf > 0 {
			fvPos += cf * math.Pow(1+reinvestRate, float64(n-1-i))
		}
	}
	if pvNeg == 0 || fvPos == 0 {
		return Error(ErrDiv0)
	}
	return Number(math.Pow(fvPos/-pvNeg, 1/float64(n-1)) - 1)
}

// ---------------------------------------------------------------------------
// Statistical gap functions
// ---------------------------------------------------------------------------

// collectTwoRanges gathers matching numeric pairs from two parallel argument
// lists (range vs range, or individual values).  Returns nil, nil on mismatch.
func collectTwoRanges(args1, args2 []Expression, ctx Context) ([]float64, []float64) {
	xs := collectNumericValues(args1, ctx)
	ys := collectNumericValues(args2, ctx)
	if len(xs) != len(ys) || len(xs) == 0 {
		return nil, nil
	}
	return xs, ys
}

func collectNumericValues(args []Expression, ctx Context) []float64 {
	var out []float64
	for _, arg := range args {
		for _, v := range expandArgumentValues(arg, ctx) {
			if v.IsError() || v.IsEmpty() {
				continue
			}
			n, err := v.ToNumber()
			if err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

func mean(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// fnCorrel returns the Pearson correlation coefficient.
func fnCorrel(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	if len(xs) < 2 {
		return Error(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	var num, dx2, dy2 float64
	for i := range xs {
		dx := xs[i] - mx
		dy := ys[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number(num / denom)
}

// fnCovar returns population covariance (COVAR / COVARIANCE.P).
func fnCovar(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	if len(xs) == 0 {
		return Error(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	s := 0.0
	for i := range xs {
		s += (xs[i] - mx) * (ys[i] - my)
	}
	return Number(s / float64(len(xs)))
}

// fnCovarS returns sample covariance (COVARIANCE.S).
func fnCovarS(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	n := len(xs)
	if n < 2 {
		return Error(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	s := 0.0
	for i := range xs {
		s += (xs[i] - mx) * (ys[i] - my)
	}
	return Number(s / float64(n-1))
}

// fnSlope returns the slope of the linear-regression line.
func fnSlope(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	// SLOPE(known_y, known_x)
	ys, xs := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	if len(xs) < 2 {
		return Error(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	var num, denom float64
	for i := range xs {
		dx := xs[i] - mx
		num += dx * (ys[i] - my)
		denom += dx * dx
	}
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number(num / denom)
}

// fnIntercept returns the intercept of the linear-regression line.
func fnIntercept(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	// INTERCEPT(known_y, known_x)
	ys, xs := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	if len(xs) < 2 {
		return Error(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	var num, denom float64
	for i := range xs {
		dx := xs[i] - mx
		num += dx * (ys[i] - my)
		denom += dx * dx
	}
	if denom == 0 {
		return Error(ErrDiv0)
	}
	slope := num / denom
	return Number(my - slope*mx)
}

// fnForecast returns the forecast for x using linear regression.
func fnForecast(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	xVal := args[0].Eval(ctx)
	if xVal.IsError() {
		return xVal
	}
	xNum, err := xVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	// FORECAST(x, known_y, known_x)
	ys, xs := collectTwoRanges(args[1:2], args[2:3], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	if len(xs) < 2 {
		return Error(ErrNA)
	}
	mx, my := mean(xs), mean(ys)
	var num, denom float64
	for i := range xs {
		dx := xs[i] - mx
		num += dx * (ys[i] - my)
		denom += dx * dx
	}
	if denom == 0 {
		return Error(ErrDiv0)
	}
	slope := num / denom
	intercept := my - slope*mx
	return Number(slope*xNum + intercept)
}

// fnPercentile returns the k-th percentile (0≤k≤1), inclusive.
func fnPercentile(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	vals := collectNumericValues(args[:1], ctx)
	if len(vals) == 0 {
		return Error(ErrNum)
	}
	kVal := args[1].Eval(ctx)
	if kVal.IsError() {
		return kVal
	}
	k, err := kVal.ToNumber()
	if err != nil || k < 0 || k > 1 {
		return Error(ErrNum)
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := float64(len(sorted))
	idx := k * (n - 1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return Number(sorted[lo])
	}
	frac := idx - float64(lo)
	return Number(sorted[lo]*(1-frac) + sorted[hi]*frac)
}

// fnPercentileExc returns the k-th percentile, exclusive (k strictly between 0 and 1).
func fnPercentileExc(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	vals := collectNumericValues(args[:1], ctx)
	n := len(vals)
	if n == 0 {
		return Error(ErrNum)
	}
	kVal := args[1].Eval(ctx)
	if kVal.IsError() {
		return kVal
	}
	k, err := kVal.ToNumber()
	if err != nil {
		return Error(ErrNum)
	}
	// exclusive: k must be in (0, 1) strictly; position = k*(n+1)-1 (0-based)
	if k <= 0 || k >= 1 {
		return Error(ErrNum)
	}
	sorted := make([]float64, n)
	copy(sorted, vals)
	sort.Float64s(sorted)
	idx := k*float64(n+1) - 1
	if idx < 0 || idx > float64(n-1) {
		return Error(ErrNum)
	}
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return Number(sorted[lo])
	}
	frac := idx - float64(lo)
	return Number(sorted[lo]*(1-frac) + sorted[hi]*frac)
}

// fnQuartile returns the k-th quartile (k = 0..4), inclusive.
func fnQuartile(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	qVal := args[1].Eval(ctx)
	if qVal.IsError() {
		return qVal
	}
	q, err := qVal.ToNumber()
	if err != nil {
		return Error(ErrNum)
	}
	q = math.Trunc(q)
	if q < 0 || q > 4 {
		return Error(ErrNum)
	}
	return fnPercentile([]Expression{args[0], &NumberExpr{Value: q / 4}}, ctx)
}

// fnQuartileExc returns the k-th quartile (k = 1..3), exclusive.
func fnQuartileExc(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	qVal := args[1].Eval(ctx)
	if qVal.IsError() {
		return qVal
	}
	q, err := qVal.ToNumber()
	if err != nil {
		return Error(ErrNum)
	}
	q = math.Trunc(q)
	if q < 1 || q > 3 {
		return Error(ErrNum)
	}
	return fnPercentileExc([]Expression{args[0], &NumberExpr{Value: q / 4}}, ctx)
}

// fnPercentRank returns the rank of a value as a percentage of the data set (inclusive).
func fnPercentRank(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	vals := collectNumericValues(args[:1], ctx)
	n := len(vals)
	if n == 0 {
		return Error(ErrNum)
	}
	xVal := args[1].Eval(ctx)
	if xVal.IsError() {
		return xVal
	}
	x, err := xVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	significance := 3
	if len(args) == 3 {
		sVal := args[2].Eval(ctx)
		if sVal.IsError() {
			return sVal
		}
		sv, err2 := sVal.ToNumber()
		if err2 != nil || sv < 1 {
			return Error(ErrNum)
		}
		significance = int(math.Trunc(sv))
	}
	sorted := make([]float64, n)
	copy(sorted, vals)
	sort.Float64s(sorted)
	// value must be in range
	if x < sorted[0] || x > sorted[n-1] {
		return Error(ErrNA)
	}
	// count values strictly less than x
	less := 0
	for _, v := range sorted {
		if v < x {
			less++
		}
	}
	// linear interpolation for non-exact match
	rank := float64(less) / float64(n-1)
	// round to significance digits
	factor := math.Pow(10, float64(significance))
	return Number(math.Floor(rank*factor) / factor)
}

// ---------------------------------------------------------------------------
// Normal distribution functions
// ---------------------------------------------------------------------------

// normCDF is the cumulative standard normal distribution.
func normCDF(z float64) float64 {
	return 0.5 * math.Erfc(-z/math.Sqrt2)
}

// normPDF is the standard normal probability density function.
func normPDF(z float64) float64 {
	return math.Exp(-0.5*z*z) / math.Sqrt(2*math.Pi)
}

// normInv is the inverse of the standard normal CDF (rational approximation).
func normInvApprox(p float64) float64 {
	// Beasley-Springer-Moro algorithm
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	if p == 0.5 {
		return 0
	}
	const (
		a0, a1, a2, a3 = 2.515517, 0.802853, 0.010328, 0.0
		b1, b2, b3     = 1.432788, 0.189269, 0.001308
	)
	q := p
	if p > 0.5 {
		q = 1 - p
	}
	t := math.Sqrt(-2 * math.Log(q))
	num := ((a3*t+a2)*t+a1)*t + a0
	den := (((b3*t+b2)*t+b1)*t + 1)
	z := t - num/den
	if p < 0.5 {
		z = -z
	}
	return z
}

// fnNormDist returns the normal distribution value for x (cumulative or PDF).
func fnNormDist(args []Expression, ctx Context) Value {
	if len(args) != 4 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	mu, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	sigma, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if sigma <= 0 {
		return Error(ErrNum)
	}
	cumVal := args[3].Eval(ctx)
	if cumVal.IsError() {
		return cumVal
	}
	cumulative, err := cumVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	z := (x - mu) / sigma
	if cumulative != 0 {
		return Number(normCDF(z))
	}
	return Number(normPDF(z) / sigma)
}

// fnNormSDist returns the standard normal CDF for z.
func fnNormSDist(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	z, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	// NORM.S.DIST has a cumulative argument; NORMSDIST (legacy) is always cumulative
	if len(args) == 2 {
		cumVal := args[1].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		cum, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		if cum == 0 {
			return Number(normPDF(z))
		}
	}
	return Number(normCDF(z))
}

// fnNormInv returns the inverse of the normal CDF.
func fnNormInv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	mu, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	sigma, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if p <= 0 || p >= 1 || sigma <= 0 {
		return Error(ErrNum)
	}
	return Number(mu + sigma*normInvApprox(p))
}

// fnNormSInv returns the inverse of the standard normal CDF.
func fnNormSInv(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	if p <= 0 || p >= 1 {
		return Error(ErrNum)
	}
	return Number(normInvApprox(p))
}

// fnConfidenceNorm returns the confidence interval half-width using normal distribution.
func fnConfidenceNorm(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	alpha, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	sigma, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	n = math.Trunc(n)
	if alpha <= 0 || alpha >= 1 || sigma <= 0 || n < 1 {
		return Error(ErrNum)
	}
	z := normInvApprox(1 - alpha/2)
	return Number(z * sigma / math.Sqrt(n))
}

// ---------------------------------------------------------------------------
// Math gap functions
// ---------------------------------------------------------------------------

// fnFactDouble returns the double factorial n!! = n*(n-2)*(n-4)*...
func fnFactDouble(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	nVal := args[0].Eval(ctx)
	if nVal.IsError() {
		return nVal
	}
	nf, err := nVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	n := int(math.Trunc(nf))
	if n < -1 {
		return Error(ErrNum)
	}
	if n == -1 || n == 0 {
		return Number(1)
	}
	result := 1.0
	for i := n; i >= 1; i -= 2 {
		result *= float64(i)
	}
	return Number(result)
}

// fnCombinA returns the number of combinations with repetition C(n+k-1, k).
func fnCombinA(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	nVal := args[0].Eval(ctx)
	if nVal.IsError() {
		return nVal
	}
	kVal := args[1].Eval(ctx)
	if kVal.IsError() {
		return kVal
	}
	nf, err := nVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	kf, err := kVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	n := int(math.Trunc(nf))
	k := int(math.Trunc(kf))
	if n < 0 || k < 0 {
		return Error(ErrNum)
	}
	if k == 0 {
		return Number(1)
	}
	// C(n+k-1, k)
	return Number(combinFloat(n+k-1, k))
}

func combinFloat(n, k int) float64 {
	if k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n-k {
		k = n - k
	}
	result := 1.0
	for i := 0; i < k; i++ {
		result = result * float64(n-i) / float64(i+1)
	}
	return result
}

// fnMultinomial returns the multinomial coefficient n!/(k1!*k2!*...).
func fnMultinomial(args []Expression, ctx Context) Value {
	if len(args) == 0 {
		return Error(ErrValue)
	}
	vals := collectNumericValues(args, ctx)
	if len(vals) == 0 {
		return Error(ErrValue)
	}
	total := 0
	for _, v := range vals {
		n := int(math.Trunc(v))
		if n < 0 {
			return Error(ErrNum)
		}
		total += n
	}
	// result = total! / (k1! * k2! * ...)
	result := 1.0
	remaining := total
	for _, v := range vals {
		k := int(math.Trunc(v))
		result *= combinFloat(remaining, k)
		remaining -= k
	}
	return Number(result)
}

// collectPairedRanges gathers matching numeric pairs from exactly two expression arguments.
func collectPairedRanges(arg1, arg2 Expression, ctx Context) ([]float64, []float64) {
	xVals := expandArgumentValues(arg1, ctx)
	yVals := expandArgumentValues(arg2, ctx)
	var xs, ys []float64
	for _, v := range xVals {
		if !v.IsError() && !v.IsEmpty() {
			n, err := v.ToNumber()
			if err == nil {
				xs = append(xs, n)
			}
		}
	}
	for _, v := range yVals {
		if !v.IsError() && !v.IsEmpty() {
			n, err := v.ToNumber()
			if err == nil {
				ys = append(ys, n)
			}
		}
	}
	if len(xs) != len(ys) || len(xs) == 0 {
		return nil, nil
	}
	return xs, ys
}

// fnSumX2MY2 returns sum of (x^2 - y^2) for paired arrays.
func fnSumX2MY2(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	s := 0.0
	for i := range xs {
		s += xs[i]*xs[i] - ys[i]*ys[i]
	}
	return Number(s)
}

// fnSumX2PY2 returns sum of (x^2 + y^2) for paired arrays.
func fnSumX2PY2(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	s := 0.0
	for i := range xs {
		s += xs[i]*xs[i] + ys[i]*ys[i]
	}
	return Number(s)
}

// fnSumXMY2 returns sum of (x - y)^2 for paired arrays.
func fnSumXMY2(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xs, ys := collectPairedRanges(args[0], args[1], ctx)
	if xs == nil {
		return Error(ErrNA)
	}
	s := 0.0
	for i := range xs {
		d := xs[i] - ys[i]
		s += d * d
	}
	return Number(s)
}

// fnSubtotal evaluates a subtotal using one of 11 (or 111) function codes.
// Codes 1-11 include hidden rows; codes 101-111 exclude hidden rows.
// Since the engine has no visibility of hidden rows, both behave identically here.
func fnSubtotal(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	codeVal := args[0].Eval(ctx)
	if codeVal.IsError() {
		return codeVal
	}
	codeF, err := codeVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	code := int(math.Trunc(codeF))
	if code > 100 {
		code -= 100
	}
	rangeArgs := args[1:]
	switch code {
	case 1: // AVERAGE
		return fnAverage(rangeArgs, ctx)
	case 2: // COUNT
		return fnCount(rangeArgs, ctx)
	case 3: // COUNTA
		return fnCountA(rangeArgs, ctx)
	case 4: // MAX
		return fnMax(rangeArgs, ctx)
	case 5: // MIN
		return fnMin(rangeArgs, ctx)
	case 6: // PRODUCT
		return fnProduct(rangeArgs, ctx)
	case 7: // STDEV
		return fnStdev(rangeArgs, ctx)
	case 8: // STDEVP
		return fnStdevp(rangeArgs, ctx)
	case 9: // SUM
		return fnSum(rangeArgs, ctx)
	case 10: // VAR
		return fnVar(rangeArgs, ctx)
	case 11: // VARP
		return fnVarp(rangeArgs, ctx)
	default:
		return Error(ErrValue)
	}
}

// fnAggregate evaluates AGGREGATE by routing supported function codes to
// existing aggregate implementations. Option flags are currently accepted and
// validated but do not alter evaluation because the engine has no hidden-row or
// nested-subtotal model yet.
func fnAggregate(args []Expression, ctx Context) Value {
	if len(args) < 3 {
		return Error(ErrValue)
	}
	codeVal := args[0].Eval(ctx)
	if codeVal.IsError() {
		return codeVal
	}
	codeF, err := codeVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	optionVal := args[1].Eval(ctx)
	if optionVal.IsError() {
		return optionVal
	}
	if _, err := optionVal.ToNumber(); err != nil {
		return Error(ErrValue)
	}
	fnArgs := args[2:]
	switch int(math.Trunc(codeF)) {
	case 1:
		return fnAverage(fnArgs, ctx)
	case 2:
		return fnCount(fnArgs, ctx)
	case 3:
		return fnCountA(fnArgs, ctx)
	case 4:
		return fnMax(fnArgs, ctx)
	case 5:
		return fnMin(fnArgs, ctx)
	case 6:
		return fnProduct(fnArgs, ctx)
	case 7:
		return fnStdev(fnArgs, ctx)
	case 8:
		return fnStdevp(fnArgs, ctx)
	case 9:
		return fnSum(fnArgs, ctx)
	case 10:
		return fnVar(fnArgs, ctx)
	case 11:
		return fnVarp(fnArgs, ctx)
	case 12:
		return fnMedian(fnArgs, ctx)
	case 13:
		return fnMode(fnArgs, ctx)
	case 14:
		return fnLarge(fnArgs, ctx)
	case 15:
		return fnSmall(fnArgs, ctx)
	case 16:
		return fnPercentile(fnArgs, ctx)
	case 17:
		return fnQuartile(fnArgs, ctx)
	case 18:
		return fnPercentileExc(fnArgs, ctx)
	case 19:
		return fnQuartileExc(fnArgs, ctx)
	default:
		return Error(ErrValue)
	}
}

// fnHyperlink evaluates HYPERLINK to its display text. The evaluator does not
// currently attach hyperlink metadata from formula results, so this follows the
// visible cell-value behavior only.
func fnHyperlink(args []Expression, ctx Context) Value {
	if len(args) < 1 || len(args) > 2 {
		return Error(ErrValue)
	}
	linkLocation := args[0].Eval(ctx)
	if linkLocation.IsError() {
		return linkLocation
	}
	if len(args) == 2 {
		friendlyName := args[1].Eval(ctx)
		if friendlyName.IsError() {
			return friendlyName
		}
		return Text(friendlyName.ToText())
	}
	return Text(linkLocation.ToText())
}

// fnGetPivotData resolves workbook-backed pivot lookups when the current
// evaluation context provides a pivot-data resolver.
func fnGetPivotData(args []Expression, ctx Context) Value {
	if len(args) < 2 {
		return Error(ErrValue)
	}
	dataField := args[0].Eval(ctx)
	if dataField.IsError() {
		return dataField
	}
	pivotTableValue := args[1].Eval(ctx)
	if pivotTableValue.IsError() {
		return pivotTableValue
	}
	if (len(args)-2)%2 != 0 {
		return Error(ErrRef)
	}
	criteria := make([]PivotDataCriterion, 0, (len(args)-2)/2)
	for i := 2; i < len(args); i += 2 {
		field := args[i].Eval(ctx)
		if field.IsError() {
			return field
		}
		item := args[i+1].Eval(ctx)
		if item.IsError() {
			return item
		}
		criteria = append(criteria, PivotDataCriterion{
			Field: field.ToText(),
			Item:  item,
		})
	}
	if resolver, ok := ctx.(pivotDataResolver); ok {
		if value, ok := resolver.ResolvePivotData(dataField.ToText(), pivotArgumentRef(args[1]), pivotTableValue, criteria); ok {
			return value
		}
	}
	return Error(ErrRef)
}

func pivotArgumentRef(expr Expression) string {
	switch e := expr.(type) {
	case *CellRefExpr:
		return e.Ref
	case *RangeExpr:
		return e.Start
	case *StringExpr:
		return e.Value
	default:
		return strings.TrimSpace(expr.String())
	}
}

// rangeToMatrix evaluates an expression and returns flat values plus row/col counts.
// It uses inferExpressionArrayShape to determine dimensions of range arguments.
func rangeToMatrix(expr Expression, ctx Context) ([]Value, int, int) {
	shape, ok := inferExpressionArrayShape(expr)
	if ok && (shape.rows > 1 || shape.cols > 1) {
		vals := expandArgumentValues(expr, ctx)
		expected := shape.rows * shape.cols
		if len(vals) != expected {
			return nil, 0, 0
		}
		return vals, shape.rows, shape.cols
	}
	// scalar or single-cell
	v := expr.Eval(ctx)
	if v.IsError() {
		return nil, 0, 0
	}
	return []Value{v}, 1, 1
}

// fnMmult multiplies two matrices stored in 2D cell ranges.
func fnMmult(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVals, aRows, aCols := rangeToMatrix(args[0], ctx)
	if aVals == nil {
		return Error(ErrValue)
	}
	bVals, bRows, bCols := rangeToMatrix(args[1], ctx)
	if bVals == nil {
		return Error(ErrValue)
	}
	if aCols != bRows {
		return Error(ErrValue)
	}
	// result is aRows x bCols
	result := make([]Value, aRows*bCols)
	for r := 0; r < aRows; r++ {
		for c := 0; c < bCols; c++ {
			sum := 0.0
			for k := 0; k < aCols; k++ {
				an, err1 := aVals[r*aCols+k].ToNumber()
				bn, err2 := bVals[k*bCols+c].ToNumber()
				if err1 != nil || err2 != nil {
					return Error(ErrValue)
				}
				sum += an * bn
			}
			result[r*bCols+c] = Number(sum)
		}
	}
	return arrayResultValue(result, arrayShape{rows: aRows, cols: bCols}, ctx)
}

// fnMunit returns the n×n identity matrix.
func fnMunit(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	nVal := args[0].Eval(ctx)
	if nVal.IsError() {
		return nVal
	}
	nf, err := nVal.ToNumber()
	if err != nil || nf < 1 {
		return Error(ErrValue)
	}
	n := int(math.Trunc(nf))
	vals := make([]Value, n*n)
	for i := 0; i < n; i++ {
		vals[i*n+i] = Number(1)
	}
	return arrayResultValue(vals, arrayShape{rows: n, cols: n}, ctx)
}

// fnMdeterm returns the determinant of a square matrix via LU decomposition.
func fnMdeterm(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	vals, rows, cols := rangeToMatrix(args[0], ctx)
	if vals == nil || rows != cols || rows == 0 {
		return Error(ErrValue)
	}
	n := rows
	// Copy to float64 matrix
	m := make([]float64, n*n)
	for i, v := range vals {
		f, err := v.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		m[i] = f
	}
	det := luDeterminant(m, n)
	return Number(det)
}

func luDeterminant(m []float64, n int) float64 {
	a := make([]float64, n*n)
	copy(a, m)
	det := 1.0
	for col := 0; col < n; col++ {
		// find pivot
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(a[row*n+col]) > math.Abs(a[pivot*n+col]) {
				pivot = row
			}
		}
		if pivot != col {
			for k := 0; k < n; k++ {
				a[col*n+k], a[pivot*n+k] = a[pivot*n+k], a[col*n+k]
			}
			det = -det
		}
		if a[col*n+col] == 0 {
			return 0
		}
		det *= a[col*n+col]
		for row := col + 1; row < n; row++ {
			factor := a[row*n+col] / a[col*n+col]
			for k := col; k < n; k++ {
				a[row*n+k] -= factor * a[col*n+k]
			}
		}
	}
	return det
}

// fnMinverse returns the inverse of a square matrix via Gauss-Jordan elimination.
func fnMinverse(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	vals, rows, cols := rangeToMatrix(args[0], ctx)
	if vals == nil || rows != cols || rows == 0 {
		return Error(ErrValue)
	}
	n := rows
	// Augmented matrix [A | I]
	aug := make([]float64, n*2*n)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			f, err := vals[r*n+c].ToNumber()
			if err != nil {
				return Error(ErrValue)
			}
			aug[r*(2*n)+c] = f
		}
		aug[r*(2*n)+n+r] = 1
	}
	// Gauss-Jordan
	for col := 0; col < n; col++ {
		pivot := -1
		for row := col; row < n; row++ {
			if math.Abs(aug[row*(2*n)+col]) > 1e-12 {
				pivot = row
				break
			}
		}
		if pivot == -1 {
			return Error(ErrNum) // singular
		}
		if pivot != col {
			for k := 0; k < 2*n; k++ {
				aug[col*(2*n)+k], aug[pivot*(2*n)+k] = aug[pivot*(2*n)+k], aug[col*(2*n)+k]
			}
		}
		div := aug[col*(2*n)+col]
		for k := 0; k < 2*n; k++ {
			aug[col*(2*n)+k] /= div
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug[row*(2*n)+col]
			for k := 0; k < 2*n; k++ {
				aug[row*(2*n)+k] -= factor * aug[col*(2*n)+k]
			}
		}
	}
	result := make([]Value, n*n)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			result[r*n+c] = Number(aug[r*(2*n)+n+c])
		}
	}
	return arrayResultValue(result, arrayShape{rows: n, cols: n}, ctx)
}

// ---------------------------------------------------------------------------
// Remaining financial gap functions
// ---------------------------------------------------------------------------

// fnXnpv returns the net present value for a schedule of cash flows with dates.
// Values and dates must be parallel ranges.
func fnXnpv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	rate, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	cashFlows := collectNumericValues(args[1:2], ctx)
	dateSeries := collectNumericValues(args[2:3], ctx)
	if len(cashFlows) != len(dateSeries) || len(cashFlows) == 0 {
		return Error(ErrValue)
	}
	if rate <= -1 {
		return Error(ErrNum)
	}
	base := dateSeries[0]
	total := 0.0
	for i, cf := range cashFlows {
		days := dateSeries[i] - base
		total += cf / math.Pow(1+rate, days/365)
	}
	return Number(total)
}

// fnXirr returns the internal rate of return for irregular cash flows using Newton iteration.
func fnXirr(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	cashFlows := collectNumericValues(args[:1], ctx)
	dateSeries := collectNumericValues(args[1:2], ctx)
	if len(cashFlows) != len(dateSeries) || len(cashFlows) < 2 {
		return Error(ErrValue)
	}
	guess, errVal, ok := financialOptionalNumberArg(args, 2, 0.1, ctx)
	if !ok {
		return errVal
	}
	base := dateSeries[0]
	xnpvAt := func(r float64) float64 {
		total := 0.0
		for i, cf := range cashFlows {
			days := dateSeries[i] - base
			total += cf / math.Pow(1+r, days/365)
		}
		return total
	}
	rate := guess
	for i := 0; i < 100; i++ {
		v := xnpvAt(rate)
		if math.Abs(v) < 1e-10 {
			return Number(rate)
		}
		step := 1e-7
		deriv := (xnpvAt(rate+step) - xnpvAt(rate-step)) / (2 * step)
		if deriv == 0 || math.IsNaN(deriv) || math.IsInf(deriv, 0) {
			break
		}
		next := rate - v/deriv
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= -1 {
			break
		}
		if math.Abs(next-rate) < 1e-12 {
			return Number(next)
		}
		rate = next
	}
	return Error(ErrNum)
}

// fnFvSchedule returns the future value of an investment after applying a schedule of rates.
func fnFvSchedule(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	principal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	rates := collectNumericValues(args[1:], ctx)
	result := principal
	for _, r := range rates {
		result *= 1 + r
	}
	return Number(result)
}

// fnPduration returns the number of periods required for an investment to reach a target.
func fnPduration(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	rate, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	pv, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	fv, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if rate <= 0 || pv <= 0 || fv <= 0 {
		return Error(ErrNum)
	}
	return Number((math.Log(fv) - math.Log(pv)) / math.Log(1+rate))
}

// fnRri returns the equivalent interest rate for the growth of an investment.
func fnRri(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	nper, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	pv, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	fv, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if nper <= 0 || pv == 0 {
		return Error(ErrNum)
	}
	return Number(math.Pow(fv/pv, 1/nper) - 1)
}

// fnIspmt returns the interest paid during a specific period of an investment.
// ISPMT(rate, per, nper, pv) — simple straight-line interest, not amortized.
func fnIspmt(args []Expression, ctx Context) Value {
	if len(args) != 4 {
		return Error(ErrValue)
	}
	rate, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	per, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	nper, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	pv, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	if nper == 0 {
		return Error(ErrDiv0)
	}
	return Number(pv * rate * (per/nper - 1))
}

// fnDollarDe converts a dollar price expressed as a fraction to a decimal.
// DOLLARDE(fractional_dollar, fraction)
func fnDollarDe(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	fd, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	frac, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	frac = math.Trunc(frac)
	if frac < 0 {
		return Error(ErrNum)
	}
	if frac == 0 {
		return Error(ErrDiv0)
	}
	intPart := math.Trunc(fd)
	fracPart := fd - intPart
	// fracPart digits represent numerator; denominator = frac
	digits := math.Ceil(math.Log10(frac))
	if digits < 1 {
		digits = 1
	}
	fracPart = fracPart * math.Pow(10, digits) / frac
	return Number(intPart + fracPart)
}

// fnDollarFr converts a dollar price expressed as a decimal to a fraction.
// DOLLARFR(decimal_dollar, fraction)
func fnDollarFr(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	dec, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	frac, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	frac = math.Trunc(frac)
	if frac < 0 {
		return Error(ErrNum)
	}
	if frac == 0 {
		return Error(ErrDiv0)
	}
	intPart := math.Trunc(dec)
	fracPart := dec - intPart
	digits := math.Ceil(math.Log10(frac))
	if digits < 1 {
		digits = 1
	}
	fracPart = fracPart * frac / math.Pow(10, digits)
	return Number(intPart + fracPart)
}

// fnTextsplit splits text into a 2D array using column and optional row delimiters.
func fnTextsplit(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 6 {
		return Error(ErrValue)
	}
	textVal := args[0].Eval(ctx)
	if textVal.IsError() {
		return textVal
	}
	text := textVal.ToText()
	colDelims, ok, errVal := textsplitDelimiters(args[1], ctx)
	if !ok {
		return errVal
	}
	var rowDelims []string
	if len(args) >= 3 {
		rv := args[2].Eval(ctx)
		if !rv.IsError() && !rv.IsEmpty() {
			rowDelims, ok, errVal = textsplitDelimiters(args[2], ctx)
			if !ok {
				return errVal
			}
		}
	}
	ignoreEmpty := false
	if len(args) >= 4 {
		ieVal := args[3].Eval(ctx)
		if !ieVal.IsError() && !ieVal.IsEmpty() {
			b, err := ieVal.ToBool()
			if err == nil {
				ignoreEmpty = b
			}
		}
	}
	caseInsensitive := false
	if len(args) >= 5 {
		mmVal := args[4].Eval(ctx)
		if !mmVal.IsError() && !mmVal.IsEmpty() {
			mm, err := mmVal.ToNumber()
			if err == nil {
				caseInsensitive = mm != 0
			}
		}
	}
	padWith := Empty()
	if len(args) == 6 {
		padWith = args[5].Eval(ctx)
	}
	rows := []string{text}
	if len(rowDelims) > 0 {
		rows = textsplitSplit(text, rowDelims, ignoreEmpty, caseInsensitive)
	}
	var grid [][]string
	maxCols := 0
	for _, row := range rows {
		cols := textsplitSplit(row, colDelims, ignoreEmpty, caseInsensitive)
		if len(cols) > maxCols {
			maxCols = len(cols)
		}
		grid = append(grid, cols)
	}
	if len(grid) == 0 || maxCols == 0 {
		return Error(ErrNA)
	}
	totalRows := len(grid)
	values := make([]Value, totalRows*maxCols)
	for r, row := range grid {
		for c := 0; c < maxCols; c++ {
			if c < len(row) {
				values[r*maxCols+c] = Text(row[c])
			} else {
				values[r*maxCols+c] = padWith
			}
		}
	}
	return arrayResultValue(values, arrayShape{rows: totalRows, cols: maxCols}, ctx)
}

// textsplitDelimiters extracts delimiter strings from a TEXTSPLIT delimiter argument.
func textsplitDelimiters(arg Expression, ctx Context) ([]string, bool, Value) {
	delims := expandArgumentValues(arg, ctx)
	result := make([]string, 0, len(delims))
	for _, d := range delims {
		if d.IsError() {
			return nil, false, d
		}
		s := d.ToText()
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil, false, Error(ErrValue)
	}
	return result, true, Value{}
}

// textsplitSplit splits text by any of the given delimiters.
func textsplitSplit(text string, delims []string, ignoreEmpty, caseInsensitive bool) []string {
	parts := []string{text}
	for _, delim := range delims {
		var next []string
		for _, part := range parts {
			next = append(next, textsplitSplitOne(part, delim, caseInsensitive)...)
		}
		parts = next
	}
	if ignoreEmpty {
		filtered := parts[:0]
		for _, p := range parts {
			if p != "" {
				filtered = append(filtered, p)
			}
		}
		return filtered
	}
	return parts
}

// textsplitSplitOne splits a single string by one delimiter.
func textsplitSplitOne(text, delim string, caseInsensitive bool) []string {
	if !caseInsensitive {
		return strings.Split(text, delim)
	}
	lower := strings.ToLower(text)
	lowerDelim := strings.ToLower(delim)
	var parts []string
	for {
		idx := strings.Index(lower, lowerDelim)
		if idx < 0 {
			parts = append(parts, text)
			break
		}
		parts = append(parts, text[:idx])
		text = text[idx+len(delim):]
		lower = lower[idx+len(lowerDelim):]
	}
	return parts
}

// fnTextbefore returns the text occurring before the nth instance of a delimiter.
func fnTextbefore(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 6 {
		return Error(ErrValue)
	}
	textVal := args[0].Eval(ctx)
	if textVal.IsError() {
		return textVal
	}
	delimVal := args[1].Eval(ctx)
	if delimVal.IsError() {
		return delimVal
	}
	text := textVal.ToText()
	delim := delimVal.ToText()
	if delim == "" {
		return Error(ErrValue)
	}
	instanceNum := 1
	if len(args) >= 3 {
		inVal := args[2].Eval(ctx)
		if !inVal.IsEmpty() && !inVal.IsError() {
			n, err := inVal.ToNumber()
			if err == nil && n != 0 {
				instanceNum = int(n)
			}
		}
	}
	caseInsensitive := false
	if len(args) >= 4 {
		mmVal := args[3].Eval(ctx)
		if !mmVal.IsEmpty() && !mmVal.IsError() {
			mm, err := mmVal.ToNumber()
			if err == nil {
				caseInsensitive = mm != 0
			}
		}
	}
	ifNotFound := Error(ErrNA)
	if len(args) >= 6 {
		ifNotFound = args[5].Eval(ctx)
	}
	searchText := text
	searchDelim := delim
	if caseInsensitive {
		searchText = strings.ToLower(text)
		searchDelim = strings.ToLower(delim)
	}
	backward := instanceNum < 0
	abs := instanceNum
	if abs < 0 {
		abs = -abs
	}
	count := 0
	if !backward {
		start := 0
		for {
			idx := strings.Index(searchText[start:], searchDelim)
			if idx < 0 {
				break
			}
			count++
			if count == abs {
				return Text(text[:start+idx])
			}
			start += idx + len(delim)
		}
	} else {
		end := len(searchText)
		for {
			idx := strings.LastIndex(searchText[:end], searchDelim)
			if idx < 0 {
				break
			}
			count++
			if count == abs {
				return Text(text[:idx])
			}
			end = idx
		}
	}
	return ifNotFound
}

// fnTextafter returns the text occurring after the nth instance of a delimiter.
func fnTextafter(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 5 {
		return Error(ErrValue)
	}
	textVal := args[0].Eval(ctx)
	if textVal.IsError() {
		return textVal
	}
	delimVal := args[1].Eval(ctx)
	if delimVal.IsError() {
		return delimVal
	}
	text := textVal.ToText()
	delim := delimVal.ToText()
	if delim == "" {
		return Error(ErrValue)
	}
	instanceNum := 1
	if len(args) >= 3 {
		inVal := args[2].Eval(ctx)
		if !inVal.IsEmpty() && !inVal.IsError() {
			n, err := inVal.ToNumber()
			if err == nil && n != 0 {
				instanceNum = int(n)
			}
		}
	}
	caseInsensitive := false
	if len(args) >= 4 {
		mmVal := args[3].Eval(ctx)
		if !mmVal.IsEmpty() && !mmVal.IsError() {
			mm, err := mmVal.ToNumber()
			if err == nil {
				caseInsensitive = mm != 0
			}
		}
	}
	ifNotFound := Error(ErrNA)
	if len(args) == 5 {
		ifNotFound = args[4].Eval(ctx)
	}
	searchText := text
	searchDelim := delim
	if caseInsensitive {
		searchText = strings.ToLower(text)
		searchDelim = strings.ToLower(delim)
	}
	backward := instanceNum < 0
	abs := instanceNum
	if abs < 0 {
		abs = -abs
	}
	count := 0
	if !backward {
		start := 0
		for {
			idx := strings.Index(searchText[start:], searchDelim)
			if idx < 0 {
				break
			}
			count++
			if count == abs {
				return Text(text[start+idx+len(delim):])
			}
			start += idx + len(delim)
		}
	} else {
		end := len(searchText)
		for {
			idx := strings.LastIndex(searchText[:end], searchDelim)
			if idx < 0 {
				break
			}
			count++
			if count == abs {
				return Text(text[idx+len(delim):])
			}
			end = idx
		}
	}
	return ifNotFound
}

func splitQualifiedRangeRef(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		return "", ref
	}
	return ref[:idx+1], ref[idx+1:]
}

// ---------------------------------------------------------------------------
// Batch 4: Financial Instrument Functions
// ---------------------------------------------------------------------------

// financialDateArg evaluates an expression as an Excel serial date number.
func financialDateArg(expr Expression, ctx Context) (float64, Value, bool) {
	val := expr.Eval(ctx)
	if val.IsError() {
		return 0, val, false
	}
	num, err := val.ToNumber()
	if err != nil {
		return 0, Error(ErrValue), false
	}
	return num, Value{}, true
}

// coupDaysInPeriod returns the number of days in the coupon period containing date.
// Uses 30/360 day basis (basis=0) by default.
func coupDaysInPeriod(settlement, maturity float64, freq int) float64 {
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	// Find the next coupon date after settlement
	nextCoup := coupNcd(settleTime, matTime, freq)
	prevCoup := coupPcd(settleTime, matTime, freq)
	return nextCoup.Sub(prevCoup).Hours() / 24
}

// coupPcd returns the previous coupon date before settlement.
func coupPcd(settle, mat time.Time, freq int) time.Time {
	// Walk backwards from maturity by coupon periods
	coup := mat
	for !coup.Before(settle) {
		prev := coup
		switch freq {
		case 1:
			coup = prev.AddDate(-1, 0, 0)
		case 2:
			coup = prev.AddDate(0, -6, 0)
		case 4:
			coup = prev.AddDate(0, -3, 0)
		default:
			return prev
		}
	}
	return coup
}

// coupNcd returns the next coupon date after settlement.
func coupNcd(settle, mat time.Time, freq int) time.Time {
	coup := mat
	for coup.After(settle) {
		prev := coup
		switch freq {
		case 1:
			coup = prev.AddDate(-1, 0, 0)
		case 2:
			coup = prev.AddDate(0, -6, 0)
		case 4:
			coup = prev.AddDate(0, -3, 0)
		default:
			return prev
		}
	}
	return coup
}

// coupDays returns the number of days in the coupon period containing settlement.
func coupDays(settle, mat time.Time, freq int) float64 {
	next := coupNcd(settle, mat, freq)
	prev := coupPcd(settle, mat, freq)
	return next.Sub(prev).Hours() / 24
}

// coupDaysNc returns the number of days from settlement to the next coupon date.
func coupDaysNc(settle, mat time.Time, freq int) float64 {
	next := coupNcd(settle, mat, freq)
	return next.Sub(settle).Hours() / 24
}

// coupNum returns the number of coupon periods between settlement and maturity.
func coupNum(settle, mat time.Time, freq int) int {
	count := 0
	coup := mat
	for coup.After(settle) {
		count++
		switch freq {
		case 1:
			coup = coup.AddDate(-1, 0, 0)
		case 2:
			coup = coup.AddDate(0, -6, 0)
		case 4:
			coup = coup.AddDate(0, -3, 0)
		default:
			return count
		}
	}
	return count
}

// yearFrac returns the fraction of a year between two dates using basis.
// basis=0: 30/360 (NASD), basis=1: actual/actual, basis=2: actual/360, basis=3: actual/365, basis=4: 30/360 (European)
func yearFrac(start, end float64, basis int) float64 {
	startTime := serialToTime(start)
	endTime := serialToTime(end)
	switch basis {
	case 0, 4:
		// 30/360
		sd, sm, sy := startTime.Day(), int(startTime.Month()), startTime.Year()
		ed, em, ey := endTime.Day(), int(endTime.Month()), endTime.Year()
		if basis == 0 {
			if sd == 31 {
				sd = 30
			}
			if ed == 31 && sd == 30 {
				ed = 30
			}
		} else {
			// European 30/360
			if sd == 31 {
				sd = 30
			}
			if ed == 31 {
				ed = 30
			}
		}
		return float64((ey-sy)*360+(em-sm)*30+(ed-sd)) / 360
	case 1:
		// actual/actual
		return endTime.Sub(startTime).Hours() / 24 / 365
	case 2:
		// actual/360
		return endTime.Sub(startTime).Hours() / 24 / 360
	case 3:
		// actual/365
		return endTime.Sub(startTime).Hours() / 24 / 365
	default:
		return 0
	}
}

// fnAccrint returns accrued interest for a security paying periodic interest.
// ACCRINT(issue, first_interest, settlement, rate, par, frequency, [basis], [calc_method])
func fnAccrint(args []Expression, ctx Context) Value {
	if len(args) < 6 || len(args) > 8 {
		return Error(ErrValue)
	}
	issue, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	firstInterest, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	settlement, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	par, errVal, ok := financialOptionalNumberArg(args, 4, 1000, ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 7 {
		bv, errVal, ok := financialNumberArg(args[6], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if issue >= settlement || rate <= 0 || par <= 0 {
		return Error(ErrNum)
	}
	// Calculate accrued interest from issue to settlement
	// For periodic interest, use the coupon period approach
	settleTime := serialToTime(settlement)
	issueTime := serialToTime(issue)
	matTime := serialToTime(firstInterest)
	// Find the previous coupon date before settlement
	prevCoup := coupPcd(settleTime, matTime, freqInt)
	// Days from issue or previous coupon to settlement
	start := issue
	if prevCoup.After(issueTime) {
		start = timeToSerial(prevCoup)
	}
	daysAccrued := yearFrac(start, settlement, basis) * float64(freqInt)
	aiPerPeriod := par * rate / float64(freqInt)
	return Number(daysAccrued * aiPerPeriod)
}

// fnAccrintM returns accrued interest for a security paying at maturity.
// ACCRINTM(issue, settlement, rate, par, [basis])
func fnAccrintM(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	issue, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	settlement, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	par, errVal, ok := financialOptionalNumberArg(args, 3, 1000, ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if issue >= settlement || rate <= 0 || par <= 0 {
		return Error(ErrNum)
	}
	frac := yearFrac(issue, settlement, basis)
	return Number(par * rate * frac)
}

// fnPrice returns the price per $100 face value of a security paying periodic interest.
// PRICE(settlement, maturity, rate, yld, redemption, frequency, [basis])
func fnPrice(args []Expression, ctx Context) Value {
	if len(args) < 6 || len(args) > 7 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	yld, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 7 {
		bv, errVal, ok := financialNumberArg(args[6], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	n := coupNum(settleTime, matTime, freqInt)
	if n == 0 {
		return Error(ErrNum)
	}
	dsr := coupDaysNc(settleTime, matTime, freqInt)
	e := coupDays(settleTime, matTime, freqInt)
	// Price formula from Excel PRICE specification
	coupon := 100 * rate / float64(freqInt)
	yieldPerPeriod := yld / float64(freqInt)
	price := redemption / math.Pow(1+yieldPerPeriod, float64(n-1)+dsr/e)
	for k := 1; k <= n; k++ {
		price += coupon / math.Pow(1+yieldPerPeriod, float64(k-1)+dsr/e)
	}
	price -= coupon * dsr / e
	return Number(price)
}

// fnPriceDisc returns the price per $100 face value of a discounted security.
// PRICEDISC(settlement, maturity, discount, redemption, [basis])
func fnPriceDisc(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	discount, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || discount <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	return Number(redemption - discount*redemption*dsm)
}

// fnPriceMat returns the price per $100 face value of a security paying at maturity.
// PRICEMAT(settlement, maturity, issue, rate, yld, [basis])
func fnPriceMat(args []Expression, ctx Context) Value {
	if len(args) < 5 || len(args) > 6 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	issue, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	yld, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 6 {
		bv, errVal, ok := financialNumberArg(args[5], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || issue >= settlement || rate <= 0 || yld <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	dis := yearFrac(issue, settlement, basis)
	dim := yearFrac(issue, maturity, basis)
	denom := 1 + yld*dsm
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number((100+dim*rate*100)/denom - dis*rate*100)
}

// fnYield returns the yield of a security paying periodic interest.
// YIELD(settlement, maturity, rate, pr, redemption, frequency, [basis])
func fnYield(args []Expression, ctx Context) Value {
	if len(args) < 6 || len(args) > 7 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 7 {
		bv, errVal, ok := financialNumberArg(args[6], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || pr <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	n := coupNum(settleTime, matTime, freqInt)
	if n == 0 {
		return Error(ErrNum)
	}
	dsr := coupDaysNc(settleTime, matTime, freqInt)
	e := coupDays(settleTime, matTime, freqInt)
	coupon := 100 * rate / float64(freqInt)
	// Use Newton iteration to find yield
	yield := rate // initial guess
	for iter := 0; iter < 100; iter++ {
		yieldPerPeriod := yield / float64(freqInt)
		price := redemption / math.Pow(1+yieldPerPeriod, float64(n-1)+dsr/e)
		for k := 1; k <= n; k++ {
			price += coupon / math.Pow(1+yieldPerPeriod, float64(k-1)+dsr/e)
		}
		price -= coupon * dsr / e
		diff := price - pr
		if math.Abs(diff) < 1e-10 {
			return Number(yield)
		}
		// Numerical derivative
		step := 1e-7
		if math.Abs(yield) > 1 {
			step = math.Abs(yield) * 1e-7
		}
		yieldPerPeriod2 := (yield + step) / float64(freqInt)
		price2 := redemption / math.Pow(1+yieldPerPeriod2, float64(n-1)+dsr/e)
		for k := 1; k <= n; k++ {
			price2 += coupon / math.Pow(1+yieldPerPeriod2, float64(k-1)+dsr/e)
		}
		price2 -= coupon * dsr / e
		deriv := (price2 - price) / step
		if deriv == 0 || math.IsNaN(deriv) || math.IsInf(deriv, 0) {
			break
		}
		next := yield - diff/deriv
		if math.IsNaN(next) || math.IsInf(next, 0) {
			break
		}
		if math.Abs(next-yield) < 1e-12 {
			return Number(next)
		}
		yield = next
	}
	return Error(ErrNum)
}

// fnYieldDisc returns the yield of a discounted security.
// YIELDDISC(settlement, maturity, pr, redemption, [basis])
func fnYieldDisc(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || pr <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	if dsm == 0 || pr == 0 {
		return Error(ErrNum)
	}
	return Number((redemption - pr) / pr / dsm)
}

// fnYieldMat returns the yield of a security paying at maturity.
// YIELDMAT(settlement, maturity, issue, rate, pr, [basis])
func fnYieldMat(args []Expression, ctx Context) Value {
	if len(args) < 5 || len(args) > 6 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	issue, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 6 {
		bv, errVal, ok := financialNumberArg(args[5], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || issue >= settlement || rate <= 0 || pr <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	dis := yearFrac(issue, settlement, basis)
	dim := yearFrac(issue, maturity, basis)
	if dsm == 0 {
		return Error(ErrNum)
	}
	return Number(((100+dim*rate*100)/pr - 1 - dis*rate) / dsm)
}

// fnDisc returns the discount rate for a security.
// DISC(settlement, maturity, pr, redemption, [basis])
func fnDisc(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || pr <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	if dsm == 0 {
		return Error(ErrNum)
	}
	return Number((redemption - pr) / redemption / dsm)
}

// fnIntRate returns the interest rate for a fully invested security.
// INTRATE(settlement, maturity, investment, redemption, [basis])
func fnIntRate(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	investment, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || investment <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	if dsm == 0 || investment == 0 {
		return Error(ErrNum)
	}
	return Number((redemption - investment) / investment / dsm)
}

// fnReceived returns the amount received at maturity for a fully invested security.
// RECEIVED(settlement, maturity, investment, discount, [basis])
func fnReceived(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	investment, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	discount, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	basis := 0
	if len(args) >= 5 {
		bv, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || investment <= 0 || discount <= 0 {
		return Error(ErrNum)
	}
	dsm := yearFrac(settlement, maturity, basis)
	denom := 1 - discount*dsm
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number(investment / denom)
}

// fnTbillEq returns the bond-equivalent yield for a Treasury bill.
// TBILLEQ(settlement, maturity, discount)
func fnTbillEq(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	discount, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if settlement >= maturity || discount <= 0 {
		return Error(ErrNum)
	}
	dsm := math.Trunc(maturity - settlement)
	if dsm <= 0 || dsm > 365 {
		return Error(ErrNum)
	}
	denom := 365 - discount*dsm
	if denom == 0 {
		return Error(ErrDiv0)
	}
	return Number(365 * discount / denom)
}

// fnTbillPrice returns the price per $100 face value for a Treasury bill.
// TBILLPRICE(settlement, maturity, discount)
func fnTbillPrice(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	discount, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if settlement >= maturity || discount <= 0 {
		return Error(ErrNum)
	}
	dsm := math.Trunc(maturity - settlement)
	if dsm <= 0 || dsm > 365 {
		return Error(ErrNum)
	}
	return Number(100 * (1 - discount*dsm/360))
}

// fnTbillYield returns the yield for a Treasury bill.
// TBILLYIELD(settlement, maturity, pr)
func fnTbillYield(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if settlement >= maturity || pr <= 0 {
		return Error(ErrNum)
	}
	dsm := math.Trunc(maturity - settlement)
	if dsm <= 0 || dsm > 365 {
		return Error(ErrNum)
	}
	return Number((100 - pr) / pr * 360 / dsm)
}

// fnCoupDayBs returns the number of days from the beginning of the coupon period to settlement.
// COUPDAYBS(settlement, maturity, frequency, [basis])
func fnCoupDayBs(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	prevCoup := coupPcd(settleTime, matTime, freqInt)
	return Number(settlement - timeToSerial(prevCoup))
}

// fnCoupDays returns the number of days in the coupon period containing settlement.
// COUPDAYS(settlement, maturity, frequency, [basis])
func fnCoupDays(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	return Number(coupDays(settleTime, matTime, freqInt))
}

// fnCoupDaysNc returns the number of days from settlement to the next coupon date.
// COUPDAYSNC(settlement, maturity, frequency, [basis])
func fnCoupDaysNc(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	return Number(coupDaysNc(settleTime, matTime, freqInt))
}

// fnCoupNcd returns the next coupon date after settlement.
// COUPNCD(settlement, maturity, frequency, [basis])
func fnCoupNcd(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	nextCoup := coupNcd(settleTime, matTime, freqInt)
	return Number(timeToSerial(nextCoup))
}

// fnCoupPcd returns the previous coupon date before settlement.
// COUPPCD(settlement, maturity, frequency, [basis])
func fnCoupPcd(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	if settlement >= maturity {
		return Error(ErrNum)
	}
	settleTime := serialToTime(settlement)
	matTime := serialToTime(maturity)
	prevCoup := coupPcd(settleTime, matTime, freqInt)
	return Number(timeToSerial(prevCoup))
}

// fnOddFPrice returns the price per $100 face value of a security with an odd first period.
// ODDFPRICE(settlement, maturity, issue, first_coupon, rate, yld, redemption, frequency, [basis])
func fnOddFPrice(args []Expression, ctx Context) Value {
	if len(args) < 8 || len(args) > 9 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	issue, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	firstCoupon, errVal, ok := financialDateArg(args[3], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	yld, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[6], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[7], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 9 {
		bv, errVal, ok := financialNumberArg(args[8], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || issue >= settlement || firstCoupon <= settlement || rate <= 0 || yld <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	// Simplified: use standard PRICE formula as approximation
	return fnPrice(args, ctx)
}

// fnOddFYield returns the yield of a security with an odd first period.
// ODDFYIELD(settlement, maturity, issue, first_coupon, rate, pr, redemption, frequency, [basis])
func fnOddFYield(args []Expression, ctx Context) Value {
	if len(args) < 8 || len(args) > 9 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	issue, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	firstCoupon, errVal, ok := financialDateArg(args[3], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[6], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[7], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 9 {
		bv, errVal, ok := financialNumberArg(args[8], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || issue >= settlement || firstCoupon <= settlement || rate <= 0 || pr <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	// Simplified: use standard YIELD formula as approximation
	return fnYield(args, ctx)
}

// fnOddLPrice returns the price per $100 face value of a security with an odd last period.
// ODDLPRICE(settlement, maturity, last_interest, rate, yld, redemption, frequency, [basis])
func fnOddLPrice(args []Expression, ctx Context) Value {
	if len(args) < 7 || len(args) > 8 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	lastInterest, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	yld, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[6], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 8 {
		bv, errVal, ok := financialNumberArg(args[7], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || lastInterest >= settlement || rate <= 0 || yld <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	// Simplified: use standard PRICE formula as approximation
	return fnPrice(args, ctx)
}

// fnOddLYield returns the yield of a security with an odd last period.
// ODDLYIELD(settlement, maturity, last_interest, rate, pr, redemption, frequency, [basis])
func fnOddLYield(args []Expression, ctx Context) Value {
	if len(args) < 7 || len(args) > 8 {
		return Error(ErrValue)
	}
	settlement, errVal, ok := financialDateArg(args[0], ctx)
	if !ok {
		return errVal
	}
	maturity, errVal, ok := financialDateArg(args[1], ctx)
	if !ok {
		return errVal
	}
	lastInterest, errVal, ok := financialDateArg(args[2], ctx)
	if !ok {
		return errVal
	}
	rate, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	pr, errVal, ok := financialNumberArg(args[4], ctx)
	if !ok {
		return errVal
	}
	redemption, errVal, ok := financialNumberArg(args[5], ctx)
	if !ok {
		return errVal
	}
	freq, errVal, ok := financialNumberArg(args[6], ctx)
	if !ok {
		return errVal
	}
	freqInt := int(math.Trunc(freq))
	if freqInt != 1 && freqInt != 2 && freqInt != 4 {
		return Error(ErrNum)
	}
	basis := 0
	if len(args) >= 8 {
		bv, errVal, ok := financialNumberArg(args[7], ctx)
		if !ok {
			return errVal
		}
		basis = int(math.Trunc(bv))
		if basis < 0 || basis > 4 {
			return Error(ErrNum)
		}
	}
	if settlement >= maturity || lastInterest >= settlement || rate <= 0 || pr <= 0 || redemption <= 0 {
		return Error(ErrNum)
	}
	// Simplified: use standard YIELD formula as approximation
	return fnYield(args, ctx)
}

// ---------------------------------------------------------------------------
// Batch 5: Engineering and Complex Functions
// ---------------------------------------------------------------------------

// fnBesselI returns the modified Bessel function I_n(x).
func fnBesselI(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	nVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(nVal))
	if n < 0 || xVal < 0 {
		return Error(ErrNum)
	}
	// Use series expansion for modified Bessel I
	if xVal == 0 {
		if n == 0 {
			return Number(1)
		}
		return Number(0)
	}
	result := 0.0
	term := 1.0
	for k := 0; k < 100; k++ {
		if k > 0 {
			term *= (xVal * xVal) / (4 * float64(k) * float64(n+k))
		}
		result += term / (math.Gamma(float64(k+1)) * math.Gamma(float64(n+k+1)))
	}
	result *= math.Pow(xVal/2, float64(n))
	return Number(result)
}

// fnBesselJ returns the Bessel function J_n(x).
func fnBesselJ(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	nVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(nVal))
	if n < 0 || xVal < 0 {
		return Error(ErrNum)
	}
	if xVal == 0 {
		if n == 0 {
			return Number(1)
		}
		return Number(0)
	}
	// Use Go's math.Jn for Bessel J
	return Number(math.Jn(n, xVal))
}

// fnBesselK returns the modified Bessel function K_n(x).
func fnBesselK(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	nVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(nVal))
	if n < 0 || xVal <= 0 {
		return Error(ErrNum)
	}
	// Use series expansion for modified Bessel K
	// K_n(x) = (π/2) * (I_{-n}(x) - I_n(x)) / sin(nπ) for non-integer n
	// For integer n, use limiting form
	if xVal < 1e-10 {
		return Error(ErrNum)
	}
	// Compute using integral representation for small n
	// K_n(x) = ∫_0^∞ exp(-x cosh(t)) cosh(nt) dt
	// Use numerical integration (simple trapezoidal)
	steps := 200
	dt := 10.0 / float64(steps)
	sum := 0.0
	for i := 0; i <= steps; i++ {
		t := float64(i) * dt
		w := dt
		if i == 0 || i == steps {
			w /= 2
		}
		arg := xVal * math.Cosh(t)
		if arg > 700 { // avoid overflow
			continue
		}
		sum += w * math.Exp(-arg) * math.Cosh(float64(n)*t)
	}
	return Number(sum)
}

// fnBesselY returns the Bessel function Y_n(x).
func fnBesselY(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	nVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(nVal))
	if n < 0 || xVal <= 0 {
		return Error(ErrNum)
	}
	// Use Go's math.Yn for Bessel Y
	return Number(math.Yn(n, xVal))
}

// fnBitAnd returns the bitwise AND of two numbers.
func fnBitAnd(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	bVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	a, b := int64(aVal), int64(bVal)
	if aVal < 0 || bVal < 0 || aVal != float64(a) || bVal != float64(b) || aVal > 281474976710655 || bVal > 281474976710655 {
		return Error(ErrNum)
	}
	return Number(float64(a & b))
}

// fnBitOr returns the bitwise OR of two numbers.
func fnBitOr(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	bVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	a, b := int64(aVal), int64(bVal)
	if aVal < 0 || bVal < 0 || aVal != float64(a) || bVal != float64(b) || aVal > 281474976710655 || bVal > 281474976710655 {
		return Error(ErrNum)
	}
	return Number(float64(a | b))
}

// fnBitXor returns the bitwise XOR of two numbers.
func fnBitXor(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	bVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	a, b := int64(aVal), int64(bVal)
	if aVal < 0 || bVal < 0 || aVal != float64(a) || bVal != float64(b) || aVal > 281474976710655 || bVal > 281474976710655 {
		return Error(ErrNum)
	}
	return Number(float64(a ^ b))
}

// fnBitLShift returns the bitwise left shift of a number by shift_amount bits.
func fnBitLShift(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	numVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	shiftVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	num := int64(numVal)
	shift := int64(shiftVal)
	if numVal < 0 || shiftVal < 0 || numVal != float64(num) || shiftVal != float64(shift) || numVal > 281474976710655 || shiftVal > 53 {
		return Error(ErrNum)
	}
	return Number(float64(num << uint(shift)))
}

// fnBitRShift returns the bitwise right shift of a number by shift_amount bits.
func fnBitRShift(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	numVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	shiftVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	num := int64(numVal)
	shift := int64(shiftVal)
	if numVal < 0 || shiftVal < 0 || numVal != float64(num) || shiftVal != float64(shift) || numVal > 281474976710655 || shiftVal > 53 {
		return Error(ErrNum)
	}
	return Number(float64(num >> uint(shift)))
}

// parseComplex parses an Excel complex number string "x+yi" or "x-yi".
func parseComplex(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	// Handle pure imaginary like "i", "-i", "2i", "-2i"
	if !strings.ContainsAny(s, "+-") || strings.HasSuffix(s, "i") && !strings.ContainsAny(s[:len(s)-1], "+-") {
		// Try pure imaginary
		if strings.HasSuffix(s, "i") || strings.HasSuffix(s, "j") {
			imagStr := s[:len(s)-1]
			if imagStr == "" || imagStr == "+" {
				return 0, 1, true
			}
			if imagStr == "-" {
				return 0, -1, true
			}
			im, err := strconv.ParseFloat(imagStr, 64)
			if err == nil {
				return 0, im, true
			}
		}
		// Try pure real
		re, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return re, 0, true
		}
		return 0, 0, false
	}
	// Find the sign between real and imaginary parts
	// Format: "x+yi" or "x-yi"
	lastPlus := strings.LastIndex(s, "+")
	lastMinus := strings.LastIndex(s, "-")
	sep := lastPlus
	if lastMinus > lastPlus {
		sep = lastMinus
	}
	if sep <= 0 {
		return 0, 0, false
	}
	realStr := s[:sep]
	imagStr := s[sep:]
	if !strings.HasSuffix(imagStr, "i") && !strings.HasSuffix(imagStr, "j") {
		return 0, 0, false
	}
	imagStr = imagStr[:len(imagStr)-1]
	if imagStr == "+" || imagStr == "" {
		imagStr = "1"
	} else if imagStr == "-" {
		imagStr = "-1"
	}
	re, err1 := strconv.ParseFloat(realStr, 64)
	im, err2 := strconv.ParseFloat(imagStr, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return re, im, true
}

// formatComplex formats a complex number as an Excel complex string.
func formatComplex(re, im float64, suffix string) string {
	if suffix == "" {
		suffix = "i"
	}
	if im == 0 {
		return strconv.FormatFloat(re, 'g', -1, 64)
	}
	imStr := ""
	if im == 1 {
		imStr = suffix
	} else if im == -1 {
		imStr = "-" + suffix
	} else {
		imStr = strconv.FormatFloat(im, 'g', -1, 64) + suffix
	}
	if re == 0 {
		return imStr
	}
	sign := "+"
	if im < 0 {
		sign = ""
	}
	return strconv.FormatFloat(re, 'g', -1, 64) + sign + imStr
}

// fnComplex creates a complex number from real and imaginary parts.
func fnComplex(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	reVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	imVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	suffix := "i"
	if len(args) == 3 {
		suffixVal := args[2].Eval(ctx)
		if suffixVal.IsError() {
			return suffixVal
		}
		s := suffixVal.ToText()
		if s != "i" && s != "j" {
			return Error(ErrValue)
		}
		suffix = s
	}
	return Text(formatComplex(reVal, imVal, suffix))
}

// fnConvert converts a number from one unit system to another.
// CONVERT(number, from_unit, to_unit)
func fnConvert(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	numVal, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	fromVal := args[1].Eval(ctx)
	if fromVal.IsError() {
		return fromVal
	}
	toVal := args[2].Eval(ctx)
	if toVal.IsError() {
		return toVal
	}
	from := strings.TrimSpace(fromVal.ToText())
	to := strings.TrimSpace(toVal.ToText())
	if from == "" || to == "" {
		return Error(ErrValue)
	}
	// Simple unit conversion table for common units
	type conversion struct {
		category string
		toSI     float64
	}
	units := map[string]conversion{
		// Weight/Mass
		"g":  {"weight", 1},
		"kg": {"weight", 1000},
		"mg": {"weight", 0.001},
		"lb": {"weight", 453.59237},
		"oz": {"weight", 28.349523125},
		"st": {"weight", 6350.29318},
		// Distance
		"m":  {"distance", 1},
		"km": {"distance", 1000},
		"cm": {"distance", 0.01},
		"mm": {"distance", 0.001},
		"mi": {"distance", 1609.344},
		"in": {"distance", 0.0254},
		"ft": {"distance", 0.3048},
		"yd": {"distance", 0.9144},
		// Time
		"sec": {"time", 1},
		"mn":  {"time", 60},
		"hr":  {"time", 3600},
		"day": {"time", 86400},
		"yr":  {"time", 31536000},
		// Temperature (special handling)
		"C": {"temperature", 0},
		"F": {"temperature", 0},
		"K": {"temperature", 0},
		// Volume
		"l":   {"volume", 1},
		"L":   {"volume", 1},
		"ml":  {"volume", 0.001},
		"gal": {"volume", 3.785411784},
		"pt":  {"volume", 0.473176473},
		"qt":  {"volume", 0.946352946},
		"tsp": {"volume", 0.00492892159},
		"tbs": {"volume", 0.0147867648},
		// Area
		"m2":  {"area", 1},
		"cm2": {"area", 0.0001},
		"mm2": {"area", 0.000001},
		"km2": {"area", 1000000},
		"in2": {"area", 0.00064516},
		"ft2": {"area", 0.09290304},
		"yd2": {"area", 0.83612736},
		// Speed
		"m/s":  {"speed", 1},
		"km/h": {"speed", 0.277777778},
		"mph":  {"speed", 0.44704},
		"kn":   {"speed", 0.514444444},
		// Force
		"N":   {"force", 1},
		"dyn": {"force", 0.00001},
		"lbf": {"force", 4.4482216152605},
		// Energy
		"J":   {"energy", 1},
		"cal": {"energy", 4.184},
		"Wh":  {"energy", 3600},
		"kWh": {"energy", 3600000},
		// Power
		"W":  {"power", 1},
		"kW": {"power", 1000},
		"HP": {"power", 745.69987158227022},
		// Pressure
		"Pa":   {"pressure", 1},
		"kPa":  {"pressure", 1000},
		"atm":  {"pressure", 101325},
		"mmHg": {"pressure", 133.322},
	}
	fromUnit, fromOk := units[from]
	toUnit, toOk := units[to]
	if !fromOk || !toOk || fromUnit.category != toUnit.category {
		return Error(ErrNA)
	}
	// Temperature conversion (special)
	if fromUnit.category == "temperature" {
		var celsius float64
		switch from {
		case "C":
			celsius = numVal
		case "F":
			celsius = (numVal - 32) * 5 / 9
		case "K":
			celsius = numVal - 273.15
		default:
			return Error(ErrNA)
		}
		switch to {
		case "C":
			return Number(celsius)
		case "F":
			return Number(celsius*9/5 + 32)
		case "K":
			return Number(celsius + 273.15)
		default:
			return Error(ErrNA)
		}
	}
	return Number(numVal * fromUnit.toSI / toUnit.toSI)
}

// fnImAbs returns the absolute value (modulus) of a complex number.
func fnImAbs(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Number(math.Sqrt(re*re + im*im))
}

// fnImaginary returns the imaginary coefficient of a complex number.
func fnImaginary(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	_, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Number(im)
}

// fnImArgument returns the argument (angle) of a complex number in radians.
func fnImArgument(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Number(math.Atan2(im, re))
}

// fnImConjugate returns the complex conjugate of a complex number.
func fnImConjugate(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Text(formatComplex(re, -im, "i"))
}

// fnImDiv returns the quotient of two complex numbers.
func fnImDiv(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVal := args[0].Eval(ctx)
	if aVal.IsError() {
		return aVal
	}
	bVal := args[1].Eval(ctx)
	if bVal.IsError() {
		return bVal
	}
	re1, im1, ok := parseComplex(aVal.ToText())
	if !ok {
		return Error(ErrValue)
	}
	re2, im2, ok := parseComplex(bVal.ToText())
	if !ok {
		return Error(ErrValue)
	}
	denom := re2*re2 + im2*im2
	if denom == 0 {
		return Error(ErrNum)
	}
	re := (re1*re2 + im1*im2) / denom
	im := (im1*re2 - re1*im2) / denom
	return Text(formatComplex(re, im, "i"))
}

// fnImExp returns the exponential of a complex number.
func fnImExp(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	expRe := math.Exp(re)
	return Text(formatComplex(expRe*math.Cos(im), expRe*math.Sin(im), "i"))
}

// fnImLn returns the natural logarithm of a complex number.
func fnImLn(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	if re == 0 && im == 0 {
		return Error(ErrNum)
	}
	mod := math.Sqrt(re*re + im*im)
	arg := math.Atan2(im, re)
	return Text(formatComplex(math.Log(mod), arg, "i"))
}

// fnImLog10 returns the base-10 logarithm of a complex number.
func fnImLog10(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	if re == 0 && im == 0 {
		return Error(ErrNum)
	}
	mod := math.Sqrt(re*re + im*im)
	arg := math.Atan2(im, re)
	log10 := math.Log(10)
	return Text(formatComplex(math.Log(mod)/log10, arg/log10, "i"))
}

// fnImLog2 returns the base-2 logarithm of a complex number.
func fnImLog2(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	if re == 0 && im == 0 {
		return Error(ErrNum)
	}
	mod := math.Sqrt(re*re + im*im)
	arg := math.Atan2(im, re)
	log2 := math.Log(2)
	return Text(formatComplex(math.Log(mod)/log2, arg/log2, "i"))
}

// fnImPower returns a complex number raised to a power.
func fnImPower(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	powerVal, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	re, im, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	if re == 0 && im == 0 {
		if powerVal == 0 {
			return Error(ErrNum)
		}
		return Number(0)
	}
	mod := math.Sqrt(re*re + im*im)
	arg := math.Atan2(im, re)
	newMod := math.Pow(mod, powerVal)
	newArg := arg * powerVal
	return Text(formatComplex(newMod*math.Cos(newArg), newMod*math.Sin(newArg), "i"))
}

// fnImProduct returns the product of multiple complex numbers.
func fnImProduct(args []Expression, ctx Context) Value {
	if len(args) < 1 {
		return Error(ErrValue)
	}
	re := 1.0
	im := 0.0
	for _, arg := range args {
		for _, v := range expandArgumentValues(arg, ctx) {
			if v.IsError() {
				return v
			}
			r, i, ok := parseComplex(v.ToText())
			if !ok {
				return Error(ErrValue)
			}
			re, im = re*r-im*i, re*i+im*r
		}
	}
	return Text(formatComplex(re, im, "i"))
}

// fnImReal returns the real coefficient of a complex number.
func fnImReal(args []Expression, ctx Context) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	val := args[0].Eval(ctx)
	if val.IsError() {
		return val
	}
	re, _, ok := parseComplex(val.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Number(re)
}

// fnImSub returns the difference of two complex numbers.
func fnImSub(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	aVal := args[0].Eval(ctx)
	if aVal.IsError() {
		return aVal
	}
	bVal := args[1].Eval(ctx)
	if bVal.IsError() {
		return bVal
	}
	re1, im1, ok := parseComplex(aVal.ToText())
	if !ok {
		return Error(ErrValue)
	}
	re2, im2, ok := parseComplex(bVal.ToText())
	if !ok {
		return Error(ErrValue)
	}
	return Text(formatComplex(re1-re2, im1-im2, "i"))
}

// fnImSum returns the sum of multiple complex numbers.
func fnImSum(args []Expression, ctx Context) Value {
	if len(args) < 1 {
		return Error(ErrValue)
	}
	re := 0.0
	im := 0.0
	for _, arg := range args {
		for _, v := range expandArgumentValues(arg, ctx) {
			if v.IsError() {
				return v
			}
			r, i, ok := parseComplex(v.ToText())
			if !ok {
				return Error(ErrValue)
			}
			re += r
			im += i
		}
	}
	return Text(formatComplex(re, im, "i"))
}

// ---------------------------------------------------------------------------
// Batch 6: Statistical Distribution Functions
// ---------------------------------------------------------------------------

// gammaLn returns the natural log of the gamma function (Lanczos approximation).
func gammaLn(x float64) float64 {
	if x <= 0 {
		return math.Inf(1)
	}
	// Use math.Lgamma from Go standard library
	result, _ := math.Lgamma(x)
	return result
}

// gammaRegQ returns the regularized upper incomplete gamma function Q(a,x) = Γ(a,x)/Γ(a).
func gammaRegQ(a, x float64) float64 {
	if x < 0 || a <= 0 {
		return math.NaN()
	}
	if x == 0 {
		return 1
	}
	// Use series expansion for x < a+1, continued fraction otherwise
	if x < a+1 {
		return 1 - gammaRegPSeries(a, x)
	}
	return gammaRegQCF(a, x)
}

// gammaRegPSeries computes P(a,x) = γ(a,x)/Γ(a) using series expansion.
func gammaRegPSeries(a, x float64) float64 {
	ap := a
	sum := 1.0 / a
	del := sum
	for n := 1; n <= 200; n++ {
		ap++
		del *= x / ap
		sum += del
		if math.Abs(del) < math.Abs(sum)*1e-15 {
			break
		}
	}
	return sum * math.Exp(-x+a*math.Log(x)-gammaLn(a))
}

// gammaRegQCF computes Q(a,x) using continued fraction representation.
func gammaRegQCF(a, x float64) float64 {
	const fpmin = 1e-30
	b := x + 1 - a
	c := 1.0 / fpmin
	d := 1.0 / b
	h := d
	for i := 1; i <= 200; i++ {
		an := -float64(i) * (float64(i) - a)
		b += 2
		d = an*d + b
		if math.Abs(d) < fpmin {
			d = fpmin
		}
		c = b + an/c
		if math.Abs(c) < fpmin {
			c = fpmin
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-15 {
			break
		}
	}
	return math.Exp(-x+a*math.Log(x)-gammaLn(a)) * h
}

// gammaRegP returns the regularized lower incomplete gamma function P(a,x) = γ(a,x)/Γ(a).
func gammaRegP(a, x float64) float64 {
	return 1 - gammaRegQ(a, x)
}

// betaLn returns the natural log of the beta function.
func betaLn(a, b float64) float64 {
	return gammaLn(a) + gammaLn(b) - gammaLn(a+b)
}

// betaReg returns the regularized incomplete beta function I_x(a,b).
func betaReg(a, b, x float64) float64 {
	if x < 0 || x > 1 || a <= 0 || b <= 0 {
		return math.NaN()
	}
	if x == 0 || x == 1 {
		if x == 0 {
			return 0
		}
		return 1
	}
	// Use continued fraction representation
	// I_x(a,b) = (x^a * (1-x)^b) / (a * B(a,b)) * CF
	bt := math.Exp(betaLn(a, b) + a*math.Log(x) + b*math.Log(1-x))
	if x < (a+1)/(a+b+2) {
		return bt * betaRegCF(a, b, x) / a
	}
	return 1 - bt*betaRegCF(b, a, 1-x)/b
}

// betaRegCF evaluates the continued fraction for the incomplete beta function.
func betaRegCF(a, b, x float64) float64 {
	const fpmin = 1e-30
	qab := a + b
	qap := a + 1
	qam := a - 1
	c := 1.0
	d := 1 - qab*x/qap
	if math.Abs(d) < fpmin {
		d = fpmin
	}
	d = 1 / d
	h := d
	for m := 1; m <= 200; m++ {
		m2 := 2 * m
		// Even step
		aa := float64(m) * (b - float64(m)) * x / ((qam + float64(m2)) * (a + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < fpmin {
			d = fpmin
		}
		c = 1 + aa/c
		if math.Abs(c) < fpmin {
			c = fpmin
		}
		d = 1 / d
		h *= d * c
		// Odd step
		aa = -(a + float64(m)) * (qab + float64(m)) * x / ((a + float64(m2)) * (qap + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < fpmin {
			d = fpmin
		}
		c = 1 + aa/c
		if math.Abs(c) < fpmin {
			c = fpmin
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-15 {
			break
		}
	}
	return h
}

// fnBetaDist returns the beta distribution (CDF or PDF).
// BETA.DIST(x, alpha, beta, cumulative, [A], [B])
func fnBetaDist(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 6 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	beta, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	cumVal := args[3].Eval(ctx)
	if cumVal.IsError() {
		return cumVal
	}
	cumulative, err := cumVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	A := 0.0
	if len(args) >= 5 {
		aVal, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		A = aVal
	}
	B := 1.0
	if len(args) >= 6 {
		bVal, errVal, ok := financialNumberArg(args[5], ctx)
		if !ok {
			return errVal
		}
		B = bVal
	}
	if alpha <= 0 || beta <= 0 || A >= B {
		return Error(ErrNum)
	}
	if x < A || x > B {
		return Error(ErrNum)
	}
	// Normalize x to [0,1]
	t := (x - A) / (B - A)
	if cumulative != 0 {
		return Number(betaReg(alpha, beta, t))
	}
	// PDF: f(x) = (x^(a-1) * (1-x)^(b-1)) / B(a,b)
	if t <= 0 || t >= 1 {
		if alpha < 1 || beta < 1 {
			return Error(ErrNum)
		}
		return Number(0)
	}
	pdf := math.Exp((alpha-1)*math.Log(t)+(beta-1)*math.Log(1-t)-betaLn(alpha, beta)) / (B - A)
	return Number(pdf)
}

// fnBetaInv returns the inverse of the beta CDF.
// BETA.INV(probability, alpha, beta, [A], [B])
func fnBetaInv(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 5 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	beta, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	A := 0.0
	if len(args) >= 4 {
		aVal, errVal, ok := financialNumberArg(args[3], ctx)
		if !ok {
			return errVal
		}
		A = aVal
	}
	B := 1.0
	if len(args) >= 5 {
		bVal, errVal, ok := financialNumberArg(args[4], ctx)
		if !ok {
			return errVal
		}
		B = bVal
	}
	if p <= 0 || p >= 1 || alpha <= 0 || beta <= 0 || A >= B {
		return Error(ErrNum)
	}
	// Binary search for the inverse
	lo, hi := 0.0, 1.0
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		cdf := betaReg(alpha, beta, mid)
		if math.Abs(cdf-p) < 1e-12 {
			return Number(A + mid*(B-A))
		}
		if cdf < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Number(A + (lo+hi)/2*(B-A))
}

// fnBinomDist returns the binomial distribution.
// BINOM.DIST(number_s, trials, probability_s, cumulative)
func fnBinomDist(args []Expression, ctx Context) Value {
	if len(args) != 4 {
		return Error(ErrValue)
	}
	numberS, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	trials, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	probS, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	cumVal := args[3].Eval(ctx)
	if cumVal.IsError() {
		return cumVal
	}
	cumulative, err := cumVal.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	n := int(math.Trunc(trials))
	s := int(math.Trunc(numberS))
	if n < 0 || s < 0 || s > n || probS < 0 || probS > 1 {
		return Error(ErrNum)
	}
	if cumulative != 0 {
		// CDF: sum of PMF from 0 to s
		sum := 0.0
		for k := 0; k <= s; k++ {
			sum += combinFloat(n, k) * math.Pow(probS, float64(k)) * math.Pow(1-probS, float64(n-k))
		}
		return Number(sum)
	}
	// PMF
	return Number(combinFloat(n, s) * math.Pow(probS, float64(s)) * math.Pow(1-probS, float64(n-s)))
}

// fnBinomInv returns the smallest value for which the binomial CDF >= probability.
// BINOM.INV(trials, probability_s, alpha)
func fnBinomInv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	trials, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	probS, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(trials))
	if n < 0 || probS < 0 || probS > 1 || alpha < 0 || alpha > 1 {
		return Error(ErrNum)
	}
	cdf := 0.0
	for k := 0; k <= n; k++ {
		cdf += combinFloat(n, k) * math.Pow(probS, float64(k)) * math.Pow(1-probS, float64(n-k))
		if cdf >= alpha {
			return Number(float64(k))
		}
	}
	return Number(float64(n))
}

// fnChisqDist returns the chi-squared distribution.
// CHISQ.DIST(x, deg_freedom, cumulative)
func fnChisqDist(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	degFreedom, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df := int(math.Trunc(degFreedom))
	if x < 0 || df < 1 || df > 1e10 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 3 {
		cumVal := args[2].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		// χ² CDF = gammaRegP(df/2, x/2)
		return Number(gammaRegP(float64(df)/2, x/2))
	}
	// PDF = x^(df/2-1) * e^(-x/2) / (2^(df/2) * Γ(df/2))
	if x == 0 {
		if df == 2 {
			return Number(0.5)
		}
		if df < 2 {
			return Error(ErrNum)
		}
		return Number(0)
	}
	pdf := math.Exp((float64(df)/2-1)*math.Log(x) - x/2 - float64(df)/2*math.Log(2) - gammaLn(float64(df)/2))
	return Number(pdf)
}

// fnChisqInv returns the inverse of the chi-squared CDF.
// CHISQ.INV(probability, deg_freedom)
func fnChisqInv(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	degFreedom, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df := int(math.Trunc(degFreedom))
	if p <= 0 || p >= 1 || df < 1 {
		return Error(ErrNum)
	}
	// Binary search for inverse
	lo, hi := 0.0, 1e6
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		cdf := gammaRegP(float64(df)/2, mid/2)
		if math.Abs(cdf-p) < 1e-12 {
			return Number(mid)
		}
		if cdf < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Number((lo + hi) / 2)
}

// fnFDist returns the F-distribution.
// F.DIST(x, deg_freedom1, deg_freedom2, cumulative)
func fnFDist(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	df1, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df2, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if x < 0 || df1 <= 0 || df2 <= 0 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 4 {
		cumVal := args[3].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	// F CDF = I_{d1*x/(d1*x+d2)}(d1/2, d2/2)
	d1 := df1
	d2 := df2
	y := d1 * x / (d1*x + d2)
	if cumulative {
		return Number(betaReg(d1/2, d2/2, y))
	}
	// PDF
	if x <= 0 {
		if d1 < 2 {
			return Error(ErrNum)
		}
		return Number(0)
	}
	// Use log form for numerical stability
	logPDF := (d1/2)*math.Log(d1) + (d2/2)*math.Log(d2) + (d1/2-1)*math.Log(x) - ((d1+d2)/2)*math.Log(d1*x+d2) - betaLn(d1/2, d2/2)
	return Number(math.Exp(logPDF))
}

// fnFInv returns the inverse of the F CDF.
// F.INV(probability, deg_freedom1, deg_freedom2)
func fnFInv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	df1, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df2, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if p <= 0 || p >= 1 || df1 <= 0 || df2 <= 0 {
		return Error(ErrNum)
	}
	d1, d2 := df1, df2
	// Binary search
	lo, hi := 0.0, 1e6
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		y := d1 * mid / (d1*mid + d2)
		cdf := betaReg(d1/2, d2/2, y)
		if math.Abs(cdf-p) < 1e-12 {
			return Number(mid)
		}
		if cdf < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Number((lo + hi) / 2)
}

// fnGammaDist returns the gamma distribution.
// GAMMA.DIST(x, alpha, beta, cumulative)
func fnGammaDist(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	beta, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if x < 0 || alpha <= 0 || beta <= 0 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 4 {
		cumVal := args[3].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		// Gamma CDF = gammaRegP(alpha, x/beta)
		return Number(gammaRegP(alpha, x/beta))
	}
	// PDF = x^(a-1) * e^(-x/b) / (b^a * Γ(a))
	if x == 0 {
		if alpha < 1 {
			return Error(ErrNum)
		}
		if alpha == 1 {
			return Number(1 / beta)
		}
		return Number(0)
	}
	pdf := math.Exp((alpha-1)*math.Log(x) - x/beta - alpha*math.Log(beta) - gammaLn(alpha))
	return Number(pdf)
}

// fnGammaInv returns the inverse of the gamma CDF.
// GAMMA.INV(probability, alpha, beta)
func fnGammaInv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	beta, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if p <= 0 || p >= 1 || alpha <= 0 || beta <= 0 {
		return Error(ErrNum)
	}
	// Binary search
	lo, hi := 0.0, 1e6
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		cdf := gammaRegP(alpha, mid/beta)
		if math.Abs(cdf-p) < 1e-12 {
			return Number(mid)
		}
		if cdf < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Number((lo + hi) / 2)
}

// fnHypgeomDist returns the hypergeometric distribution.
// HYPGEOM.DIST(sample_s, number_sample, population_s, number_pop, cumulative)
func fnHypgeomDist(args []Expression, ctx Context) Value {
	if len(args) < 4 || len(args) > 5 {
		return Error(ErrValue)
	}
	sampleS, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	numberSample, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	populationS, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	numberPop, errVal, ok := financialNumberArg(args[3], ctx)
	if !ok {
		return errVal
	}
	x := int(math.Trunc(sampleS))
	n := int(math.Trunc(numberSample))
	K := int(math.Trunc(populationS))
	N := int(math.Trunc(numberPop))
	if x < 0 || n < 0 || K < 0 || N <= 0 || x > n || x > K || n > N || K > N {
		return Error(ErrNum)
	}
	cumulative := false
	if len(args) >= 5 {
		cumVal := args[4].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		sum := 0.0
		for k := 0; k <= x; k++ {
			sum += combinFloat(K, k) * combinFloat(N-K, n-k) / combinFloat(N, n)
		}
		return Number(sum)
	}
	return Number(combinFloat(K, x) * combinFloat(N-K, n-x) / combinFloat(N, n))
}

// fnLognormDist returns the lognormal distribution.
// LOGNORM.DIST(x, mean, stdev, cumulative)
func fnLognormDist(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	mean, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	stdev, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if x <= 0 || stdev <= 0 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 4 {
		cumVal := args[3].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	z := (math.Log(x) - mean) / stdev
	if cumulative {
		return Number(normCDF(z))
	}
	// PDF = (1/(x*σ*√(2π))) * exp(-(ln(x)-μ)^2/(2σ^2))
	return Number(normPDF(z) / (x * stdev))
}

// fnLognormInv returns the inverse of the lognormal CDF.
// LOGNORM.INV(probability, mean, stdev)
func fnLognormInv(args []Expression, ctx Context) Value {
	if len(args) != 3 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	mean, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	stdev, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if p <= 0 || p >= 1 || stdev <= 0 {
		return Error(ErrNum)
	}
	return Number(math.Exp(mean + stdev*normInvApprox(p)))
}

// fnNegbinomDist returns the negative binomial distribution.
// NEGBINOM.DIST(number_f, number_s, probability_s, cumulative)
func fnNegbinomDist(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	numberF, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	numberS, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	probS, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	f := int(math.Trunc(numberF))
	s := int(math.Trunc(numberS))
	if f < 0 || s < 1 || probS <= 0 || probS >= 1 {
		return Error(ErrNum)
	}
	cumulative := false
	if len(args) >= 4 {
		cumVal := args[3].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	// PMF: C(f+s-1, f) * p^s * (1-p)^f
	pmf := combinFloat(f+s-1, f) * math.Pow(probS, float64(s)) * math.Pow(1-probS, float64(f))
	if cumulative {
		sum := 0.0
		for k := 0; k <= f; k++ {
			sum += combinFloat(k+s-1, k) * math.Pow(probS, float64(s)) * math.Pow(1-probS, float64(k))
		}
		return Number(sum)
	}
	return Number(pmf)
}

// fnPoissonDist returns the Poisson distribution.
// POISSON.DIST(x, mean, cumulative)
func fnPoissonDist(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	mean, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	n := int(math.Trunc(x))
	if n < 0 || mean <= 0 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 3 {
		cumVal := args[2].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		// Poisson CDF = gammaRegQ(n+1, mean)
		return Number(1 - gammaRegP(float64(n+1), mean))
	}
	// PMF = e^(-mean) * mean^x / x!
	return Number(math.Exp(-mean + float64(n)*math.Log(mean) - gammaLn(float64(n+1))))
}

// fnTDist returns the Student's t-distribution.
// T.DIST(x, deg_freedom, cumulative)
func fnTDist(args []Expression, ctx Context) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	degFreedom, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df := int(math.Trunc(degFreedom))
	if df < 1 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 3 {
		cumVal := args[2].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		// t CDF = 1 - I_{df/(df+x^2)}(df/2, 1/2) for x >= 0
		// For x < 0, use symmetry: P(T <= x) = 1 - P(T <= -x)
		if x >= 0 {
			y := float64(df) / (float64(df) + x*x)
			return Number(1 - 0.5*betaReg(float64(df)/2, 0.5, y))
		}
		y := float64(df) / (float64(df) + x*x)
		return Number(0.5 * betaReg(float64(df)/2, 0.5, y))
	}
	// PDF = Γ((df+1)/2) / (√(dfπ) * Γ(df/2)) * (1 + x²/df)^(-(df+1)/2)
	logPDF := gammaLn((float64(df)+1)/2) - 0.5*math.Log(float64(df)*math.Pi) - gammaLn(float64(df)/2) - (float64(df)+1)/2*math.Log(1+x*x/float64(df))
	return Number(math.Exp(logPDF))
}

// fnTInv returns the inverse of the Student's t CDF.
// T.INV(probability, deg_freedom)
func fnTInv(args []Expression, ctx Context) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	p, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	degFreedom, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	df := int(math.Trunc(degFreedom))
	if p <= 0 || p >= 1 || df < 1 {
		return Error(ErrNum)
	}
	// Binary search for inverse (two-tailed: T.INV uses left-tail probability)
	lo, hi := -1e6, 1e6
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		var cdf float64
		if mid >= 0 {
			y := float64(df) / (float64(df) + mid*mid)
			cdf = 1 - 0.5*betaReg(float64(df)/2, 0.5, y)
		} else {
			y := float64(df) / (float64(df) + mid*mid)
			cdf = 0.5 * betaReg(float64(df)/2, 0.5, y)
		}
		if math.Abs(cdf-p) < 1e-12 {
			return Number(mid)
		}
		if cdf < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Number((lo + hi) / 2)
}

// fnWeibullDist returns the Weibull distribution.
// WEIBULL.DIST(x, alpha, beta, cumulative)
func fnWeibullDist(args []Expression, ctx Context) Value {
	if len(args) < 3 || len(args) > 4 {
		return Error(ErrValue)
	}
	x, errVal, ok := financialNumberArg(args[0], ctx)
	if !ok {
		return errVal
	}
	alpha, errVal, ok := financialNumberArg(args[1], ctx)
	if !ok {
		return errVal
	}
	beta, errVal, ok := financialNumberArg(args[2], ctx)
	if !ok {
		return errVal
	}
	if x < 0 || alpha <= 0 || beta <= 0 {
		return Error(ErrNum)
	}
	cumulative := true
	if len(args) >= 4 {
		cumVal := args[3].Eval(ctx)
		if cumVal.IsError() {
			return cumVal
		}
		c, err := cumVal.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		cumulative = c != 0
	}
	if cumulative {
		// Weibull CDF = 1 - exp(-(x/β)^α)
		return Number(1 - math.Exp(-math.Pow(x/beta, alpha)))
	}
	// PDF = (α/β) * (x/β)^(α-1) * exp(-(x/β)^α)
	if x == 0 {
		if alpha < 1 {
			return Error(ErrNum)
		}
		if alpha == 1 {
			return Number(1 / beta)
		}
		return Number(0)
	}
	pdf := (alpha / beta) * math.Pow(x/beta, alpha-1) * math.Exp(-math.Pow(x/beta, alpha))
	return Number(pdf)
}

// parseCellRef parses a cell reference and returns row and column.
// Returns 0,0 if invalid.
func parseCellRef(ref string) (row, col int) {
	ref = strings.TrimSpace(ref)
	if len(ref) == 0 {
		return 0, 0
	}

	// Skip $ if present
	i := 0
	if ref[i] == '$' {
		i++
	}

	// Read column letters
	colStr := ""
	for i < len(ref) && ((ref[i] >= 'A' && ref[i] <= 'Z') || (ref[i] >= 'a' && ref[i] <= 'z')) {
		colStr += string(ref[i])
		i++
	}

	// Skip $ if present
	if i < len(ref) && ref[i] == '$' {
		i++
	}

	// Read row number
	rowStr := ""
	for i < len(ref) && ref[i] >= '0' && ref[i] <= '9' {
		rowStr += string(ref[i])
		i++
	}

	// Parse column
	col = 0
	for _, ch := range colStr {
		col = col*26 + int(unicode.ToUpper(ch)-'A'+1)
	}

	// Parse row
	row, _ = strconv.Atoi(rowStr)

	return row, col
}

package xpath

type Tracer interface  {
	Do(string, Token)
}

type discardTracer struct {}

func (_ discardTracer) Do(_ string, _ Token) {}

type stdioTracer struct {}

func (t stdioTracer) Do(rule string, token Token) {}

type compiler struct {
	scan *Scanner
	curr Token
	peek Token

	tracer Tracer

	mode       StepMode
	strictMode bool

	infix  map[rune]func(Expr) (Expr, error)
	prefix map[rune]func() (Expr, error)
}

func CompileString(query string) (Expr, error) {
	return nil, nil
}

const (
	powLowest = iota
	powAssign // variable assignment
	powOr
	powAnd
	powRange
	powCmp
	powConcat
	powAdd
	powMul
	powPrefix
	powAlt    // union
	powPath // step
	powPred
	powCall
	powHighest
)

var bindings = map[rune]int{
	currLevel: powPath,
	anyLevel:  powPath,
	opAlt:     powAlt,
	opConcat:  powConcat,
	opAssign:  powAssign,
	opEq:      powCmp,
	opNe:      powCmp,
	opGt:      powCmp,
	opGe:      powCmp,
	opLt:      powCmp,
	opLe:      powCmp,
	opBefore:  powCmp,
	opAfter:   powCmp,
	opAnd:     powAnd,
	opOr:      powOr,
	opAdd:     powAdd,
	opSub:     powAdd,
	opMul:     powMul,
	opDiv:     powMul,
	opMod:     powMul,
	begGrp:    powCall,
	begPred:   powPred,
}

package xpath

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

const (
	CodeInvalidSyntax = "XPST0003"
	CodeUndefinedVar  = "XPST0017"
	CodeNumberArg     = "XPST0018"
	CodeBadUsage      = "XPST0051"
	CodeModuleError   = "XQST0039"
)

type SyntaxError struct {
	Code  string
	Expr  string
	Cause string
	Position
}

func syntaxError(expr, cause string, pos Position) error {
	return SyntaxError{
		Code:     CodeInvalidSyntax,
		Expr:     expr,
		Cause:    cause,
		Position: pos,
	}
}

func (e SyntaxError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Expr, e.Cause)
}

const (
	functionNS = "http://www.w3.org/2005/xpath-functions"
	schemaNS   = "http://www.w3.org/2001/XMLSchema"
)

var angleNS = map[string]string{
	"agl": "http://midbel.org/angle",
}

var defaultNS = map[string]string{
	"xs":    schemaNS,
	"fn":    functionNS,
	"map":   "http://www.w3.org/2005/xpath-functions/map",
	"array": "http://www.w3.org/2005/xpath-functions/array",
	"math":  "http://www.w3.org/2005/xpath-functions/math",
	"err":   "http://www.w3.org/2005/xqt-errors",
}

type Compiler struct {
	scan *Scanner
	curr Token
	peek Token

	Tracer

	namespaces environ.Environ[string]
	elemNS     string
	funcNS     string
	typeNS     string

	infix   map[rune]func(Expr) (Expr, error)
	postfix map[rune]func(Expr) (Expr, error)
	prefix  map[rune]func() (Expr, error)
}

func CompileString(q string) (Expr, error) {
	return Compile(strings.NewReader(q))
}

func Compile(r io.Reader) (Expr, error) {
	cp := NewCompiler(r)
	return cp.Compile()
}

func NewCompiler(r io.Reader) *Compiler {
	return createCompiler(r)
}

func createCompiler(r io.Reader) *Compiler {
	cp := Compiler{
		scan:       Scan(r),
		Tracer:     discardTracer{},
		namespaces: environ.Empty[string](),
	}

	cp.infix = map[rune]func(Expr) (Expr, error){
		currLevel:    cp.compileStep,
		anyLevel:     cp.compileDescendantStep,
		opArrow:      cp.compileArrow,
		opRange:      cp.compileRange,
		opConcat:     cp.compileBinary,
		opAdd:        cp.compileBinary,
		opSub:        cp.compileBinary,
		opMul:        cp.compileBinary,
		opDiv:        cp.compileBinary,
		opMod:        cp.compileBinary,
		opValEq:      cp.compileBinary,
		opValNe:      cp.compileBinary,
		opValGt:      cp.compileBinary,
		opValGe:      cp.compileBinary,
		opValLt:      cp.compileBinary,
		opValLe:      cp.compileBinary,
		opEq:         cp.compileBinary,
		opNe:         cp.compileBinary,
		opGt:         cp.compileBinary,
		opGe:         cp.compileBinary,
		opLt:         cp.compileBinary,
		opLe:         cp.compileBinary,
		opAnd:        cp.compileBinary,
		opOr:         cp.compileBinary,
		opBefore:     cp.compileBinary,
		opAfter:      cp.compileBinary,
		opUnion:      cp.compileUnion,
		opIntersect:  cp.compileIntersect,
		opExcept:     cp.compileExcept,
		opIs:         cp.compileIdentity,
		opCastAs:     cp.compileCast,
		opCastableAs: cp.compileCastable,
		opInstanceOf: cp.compileInstanceOf,
	}
	cp.postfix = map[rune]func(Expr) (Expr, error){
		begPred: cp.compileFilter,
		begGrp:  cp.compileCall,
	}
	cp.prefix = map[rune]func() (Expr, error){
		begPred:    cp.compileArray,
		currLevel:  cp.compileRoot,
		anyLevel:   cp.compileDescendantRoot,
		Name:       cp.compileName,
		variable:   cp.compileVariable,
		currNode:   cp.compileCurrent,
		parentNode: cp.compileParent,
		attrNode:   cp.compileAttr,
		Literal:    cp.compileLiteral,
		Digit:      cp.compileNumber,
		opSub:      cp.compileReverse,
		opMul:      cp.compileName,
		begGrp:     cp.compileSequence,
		reserved:   cp.compileReservedPrefix,
	}

	for prefix, ns := range defaultNS {
		cp.RegisterNS(prefix, ns)
	}
	for prefix, ns := range angleNS {
		cp.RegisterNS(prefix, ns)
	}

	cp.next()
	cp.next()
	return &cp
}

func (c *Compiler) Compile() (Expr, error) {
	expr, err := c.compile()
	if err != nil {
		return nil, err
	}
	q := query{
		expr: expr,
	}
	return q, err
}

func (c *Compiler) RegisterNS(prefix, uri string) {
	c.namespaces.Define(prefix, uri)
}

func (c *Compiler) compile() (Expr, error) {
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if c.is(opSeq) {
		return c.compileList(expr)
	}
	if !c.done() {
		return nil, c.unexpectedError("expression")
	}
	return expr, nil
}

func (c *Compiler) compileReservedPrefix() (Expr, error) {
	switch c.getCurrentLiteral() {
	case kwMap:
		return c.compileMap()
	case kwArray:
		return c.compileArrayFunc()
	case kwLet:
		return c.compileLet()
	case kwIf:
		return c.compileIf()
	case kwFor:
		return c.compileFor()
	case kwSome, kwEvery:
		return c.compileQuantified(c.getCurrentLiteral() == kwEvery)
	default:
		return c.compileName()
	}
}

func (c *Compiler) compileMap() (Expr, error) {
	c.Enter("map")
	defer c.Leave("map")

	c.scan.KeepBlanks()
	defer c.scan.DiscardBlanks()

	c.next()
	if !c.is(begCurl) {
		return nil, ErrSyntax
	}
	expr := hashmap{
		values: make(map[Expr]Expr),
	}
	c.next()
	for !c.done() && !c.is(endCurl) {
		c.skipBlank()
		key, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		c.skipBlank()
		if !c.is(Namespace) {
			return nil, c.syntaxError("map", "unexpected ':' after map key")
		}
		c.next()
		c.skipBlank()
		val, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		switch {
		case c.is(opSeq):
			c.next()
		case c.is(endCurl):
		default:
			return nil, c.syntaxError("map", "expected ',' or '}' after map value")
		}
		expr.values[key] = val
	}
	if !c.is(endCurl) {
		return nil, ErrSyntax
	}
	c.next()
	return expr, nil
}

func (c *Compiler) compileArray() (Expr, error) {
	c.Enter("array")
	defer c.Leave("array")

	c.next()
	var arr array
	for !c.done() && !c.is(endPred) {
		e, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		arr.all = append(arr.all, e)
		switch {
		case c.is(opSeq):
			c.next()
			if c.is(endPred) {
				return nil, c.syntaxError("array", "unexpected ',' before ']'")
			}
		case c.is(endPred):
		default:
			return nil, c.unexpectedError("array")
		}
	}
	if !c.is(endPred) {
		return nil, c.syntaxError("array", "expected ']'")
	}
	c.next()
	return arr, nil
}

func (c *Compiler) compileArrayFunc() (Expr, error) {
	c.Enter("array")
	defer c.Leave("array")

	c.next()
	if !c.is(begCurl) {
		return nil, c.syntaxError("array", "expected '{'")
	}
	c.next()

	var arr array
	for !c.done() && !c.is(endCurl) {
		e, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		arr.all = append(arr.all, e)
		switch {
		case c.is(opSeq):
			c.next()
			if c.is(endCurl) {
				return nil, c.syntaxError("array", "unexpected ',' before '}'")
			}
		case c.is(endCurl):
		default:
			return nil, c.syntaxError("array", "unexpected token")
		}
	}
	if !c.is(endCurl) {
		return nil, c.syntaxError("array", "expected '}'")
	}
	c.next()
	return arr, nil
}

func (c *Compiler) compileCdt() (Expr, error) {
	if !c.is(begGrp) {
		return nil, c.syntaxError("if", "expected '('")
	}
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endGrp) {
		return nil, c.syntaxError("if", "expected ')'")
	}
	c.next()
	return expr, nil
}

func (c *Compiler) compileIf() (Expr, error) {
	c.Enter("if")
	defer c.Leave("if")
	c.next()
	var (
		cdt conditional
		err error
	)
	if cdt.test, err = c.compileCdt(); err != nil {
		return nil, err
	}
	if !c.is(reserved) && c.getCurrentLiteral() != kwThen {
		return nil, c.syntaxError("if", "expected 'then'")
	}
	c.next()
	if cdt.csq, err = c.compileExpr(powLowest); err != nil {
		return nil, err
	}
	if !c.is(reserved) && c.getCurrentLiteral() != kwElse {
		return nil, c.syntaxError("if", "expected 'else'")
	}
	c.next()
	if cdt.alt, err = c.compileExpr(powLowest); err != nil {
		return nil, err
	}
	return cdt, nil
}

func (c *Compiler) compileFor() (Expr, error) {
	c.Enter("for")
	defer c.Leave("for")
	c.next()
	var q loop
	for !c.done() && !c.is(reserved) {
		bind, err := c.compileInClause()
		if err != nil {
			return nil, err
		}
		q.binds = append(q.binds, bind)
		switch c.curr.Type {
		case opSeq:
			c.next()
			if c.is(reserved) {
				return nil, ErrSyntax
			}
		case reserved:
		default:
			return nil, fmt.Errorf("unexpected operator")
		}
	}
	if !c.is(reserved) && c.getCurrentLiteral() != kwReturn {
		return nil, fmt.Errorf("expected return keyword")
	}
	c.next()
	expr, err := c.compile()
	if err != nil {
		return nil, err
	}
	q.body = expr
	return q, nil
}

func (c *Compiler) compileInClause() (binding, error) {
	c.Enter("in")
	defer c.Leave("in")
	var b binding
	if !c.is(variable) {
		return b, c.syntaxError("in", "expected identifier")
	}
	b.ident = c.getCurrentLiteral()
	c.next()
	if !c.is(reserved) && c.getCurrentLiteral() != kwIn {
		return b, c.syntaxError("in", "expected 'in' operator")
	}
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return b, err
	}
	b.expr = expr
	return b, nil
}

func (c *Compiler) compileLet() (Expr, error) {
	c.Enter("let")
	defer c.Leave("let")

	c.next()

	var q let
	for !c.done() && !c.is(reserved) {
		var b binding
		if !c.is(variable) {
			return nil, c.syntaxError("let", "identifier expected")
		}
		b.ident = c.getCurrentLiteral()
		c.next()
		if !c.is(opAssign) {
			return nil, c.syntaxError("let", "expected ':='")
		}
		c.next()
		expr, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		b.expr = expr
		q.binds = append(q.binds, b)
		switch c.curr.Type {
		case opSeq:
			c.next()
		case reserved:
		default:
			return nil, c.unexpectedError("let")
		}
	}
	if !c.is(reserved) && c.getCurrentLiteral() != kwReturn {
		return nil, c.syntaxError("let", "expected 'return'")
	}
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q.expr = expr
	return q, nil
}

func (c *Compiler) compileQuantified(every bool) (Expr, error) {
	c.Enter("quantified")
	defer c.Leave("quantified")
	c.next()
	var q quantified
	q.every = every
	for !c.done() && !c.is(reserved) {
		bind, err := c.compileInClause()
		if err != nil {
			return nil, err
		}
		q.binds = append(q.binds, bind)
		switch c.curr.Type {
		case opSeq:
			c.next()
			if c.is(reserved) {
				return nil, c.syntaxError("some/every", "unexpected ',' before keyword")
			}
		case reserved:
		default:
			return nil, c.unexpectedError("some/every")
		}
	}
	if !c.is(reserved) && c.getCurrentLiteral() != kwSatisfies {
		return nil, c.syntaxError("some/every", "expected 'satisfies' keyword")
	}
	c.next()
	test, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q.test = test
	return q, nil
}

func (c *Compiler) compileIdentity(left Expr) (Expr, error) {
	c.Enter("identity")
	defer c.Leave("identity")
	c.next()
	right, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	expr := identity{
		left:  left,
		right: right,
	}
	return expr, nil
}

func (c *Compiler) compileRange(left Expr) (Expr, error) {
	c.Enter("range")
	defer c.Leave("range")
	c.next()

	right, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	expr := rng{
		left:  left,
		right: right,
	}
	return expr, nil
}

func (c *Compiler) compileInstanceOf(left Expr) (Expr, error) {
	c.Enter("instanceof")
	defer c.Leave("instanceof")
	c.next()

	expr := instanceof{
		expr: left,
	}
	if c.is(begGrp) {
		c.next()
		for !c.done() && !c.is(endGrp) {
			t, err := c.compileType()
			if err != nil {
				return nil, err
			}
			expr.types = append(expr.types, t)
			switch {
			case c.is(opUnion):
				c.next()
			case c.is(endGrp):
			default:
				return nil, c.syntaxError("instance of", "expected '|' or ')'")
			}
		}
		if !c.is(endGrp) {
			return nil, c.syntaxError("instance of", "expected ')'")
		}
		c.next()
	} else {
		t, err := c.compileType()
		if err != nil {
			return nil, err
		}
		expr.types = append(expr.types, t)
	}
	switch {
	case c.is(opQuestion):
		expr.occurrence = ZeroOrOneOccurrence
	case c.is(opAdd):
		expr.occurrence = OneOrMoreOccurrence
	case c.is(opMul):
		expr.occurrence = ZeroOrMoreOccurrence
	default:
	}
	if expr.occurrence != 0 {
		c.next()
	}
	return expr, nil
}

func (c *Compiler) compileCast(left Expr) (Expr, error) {
	c.Enter("cast")
	defer c.Leave("cast")
	c.next()

	t, err := c.compileType()
	if err != nil {
		return nil, err
	}
	expr := cast{
		expr:          left,
		kind:          t,
		allowEmptySeq: c.is(opQuestion),
	}
	if c.is(opQuestion) {
		c.next()
	}
	return expr, nil
}

func (c *Compiler) compileCastable(left Expr) (Expr, error) {
	c.Enter("castable")
	defer c.Leave("castable")
	c.next()

	t, err := c.compileType()
	if err != nil {
		return nil, err
	}
	expr := castable{
		expr:          left,
		kind:          t,
		allowEmptySeq: c.is(opQuestion),
	}
	if c.is(opQuestion) {
		c.next()
	}
	return expr, nil
}

func (c *Compiler) compileType() (XdmType, error) {
	if !c.is(Name) {
		return nil, c.unexpectedError("type")
	}
	var qn xml.QName
	qn.Name = c.getCurrentLiteral()
	c.next()
	if c.is(Namespace) {
		c.next()
		qn.Space = qn.Name
		qn.Name = c.getCurrentLiteral()
		c.next()
	}
	qn.Uri, _ = c.resolveNS(qn)
	if qn.Uri == "" {
		qn.Uri = c.typeNS
	}
	xt, ok := supportedTypes[qn]
	if !ok {
		xt = xsUntyped
	}
	return xt, nil
}

func (c *Compiler) compileFilter(left Expr) (Expr, error) {
	c.Enter("filter")
	defer c.Leave("filter")
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endPred) {
		return nil, c.syntaxError("filter", "expected ']")
	}
	c.next()

	f := filter{
		expr:  left,
		check: expr,
	}
	return f, nil
}

func (c *Compiler) compileList(left Expr) (Expr, error) {
	c.Enter("list")
	defer c.Leave("list")

	c.next()

	var seq sequence
	seq.all = append(seq.all, left)

	right, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if other, ok := right.(sequence); ok {
		seq.all = slices.Concat(seq.all, other.all)
	} else {
		seq.all = append(seq.all, right)
	}
	return seq, nil
}

func (c *Compiler) compileSequence() (Expr, error) {
	c.Enter("sequence")
	defer c.Leave("sequence")
	c.next()
	var seq sequence
	for !c.done() && !c.is(endGrp) {
		expr, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		seq.all = append(seq.all, expr)
		switch {
		case c.is(opSeq):
			c.next()
			if c.is(endGrp) {
				return nil, c.syntaxError("sequence", "unexpected ',' before ')'")
			}
		case c.is(endGrp):
		default:
			return nil, c.unexpectedError("sequence")
		}
	}
	if !c.is(endGrp) {
		return nil, c.syntaxError("sequence", "expected ')'")
	}
	c.next()
	return seq, nil
}

func (c *Compiler) compileUnion(left Expr) (Expr, error) {
	c.Enter("union")
	defer c.Leave("union")
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	var res union
	res.all = []Expr{left, expr}
	return res, nil
}

func (c *Compiler) compileIntersect(left Expr) (Expr, error) {
	c.Enter("intersect")
	defer c.Leave("intersect")
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	var res intersect
	res.all = []Expr{left, expr}
	return res, nil
}

func (c *Compiler) compileExcept(left Expr) (Expr, error) {
	c.Enter("except")
	defer c.Leave("except")
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	var res except
	res.all = []Expr{left, expr}
	return res, nil
}

func (c *Compiler) compileArrow(left Expr) (Expr, error) {
	c.Enter("arrow")
	defer c.Leave("arrow")
	var (
		op  = c.curr.Type
		pow = bindings[op]
	)
	for !c.done() && c.is(opArrow) {
		c.next()
		next, err := c.compileExpr(pow)
		if err != nil {
			return nil, err
		}
		c, ok := next.(call)
		if !ok {
			return nil, fmt.Errorf("call expected")
		}
		c.args = append([]Expr{left}, c.args...)
		left = c
	}
	return left, nil
}

func (c *Compiler) compileBinary(left Expr) (Expr, error) {
	c.Enter("binary")
	defer c.Leave("binary")
	var (
		op  = c.curr.Type
		pow = bindings[op]
	)
	c.next()
	next, err := c.compileExpr(pow)
	if err != nil {
		return nil, err
	}
	b := binary{
		left:  left,
		right: next,
		op:    op,
	}
	return b, nil
}

func (c *Compiler) compileLiteral() (Expr, error) {
	c.Enter("literal")
	defer c.Leave("literal")
	defer c.next()
	i := literal{
		expr: c.getCurrentLiteral(),
	}
	return i, nil
}

func (c *Compiler) compileNumber() (Expr, error) {
	c.Enter("number")
	defer c.Leave("number")

	defer c.next()
	f, err := strconv.ParseFloat(c.getCurrentLiteral(), 64)
	if err != nil {
		return nil, err
	}
	n := number{
		expr: f,
	}
	return n, nil
}

func (c *Compiler) compileReverse() (Expr, error) {
	c.Enter("reverse")
	defer c.Leave("reverse")
	c.next()
	expr, err := c.compileExpr(powPrefix)
	if err != nil {
		return nil, err
	}
	r := reverse{
		expr: expr,
	}
	return r, nil
}

func (c *Compiler) compileAttr() (Expr, error) {
	c.Enter("attribute")
	defer c.Leave("attribute")
	defer c.next()
	a := attr{
		ident: c.getCurrentLiteral(),
	}
	return a, nil
}

func (c *Compiler) compileSubscriptCall(left Expr) (Expr, error) {
	c.Enter("subscript-call")
	defer c.Leave("subscript-call")
	c.next()

	index, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endGrp) {
		return nil, c.syntaxError("subscript", "expected ')'")
	}
	c.next()
	expr := subscript{
		expr:  left,
		index: index,
	}
	return expr, nil
}

func (c *Compiler) compileFunctionCall(left Expr) (Expr, error) {
	compile := func(left Expr) (call, error) {
		n, ok := left.(name)
		if !ok {
			return call{}, c.syntaxError("call", "expected identifier")
		}
		n.Uri, _ = c.resolveNS(n.QName)
		if n.Uri == "" || n.Space == "" {
			n.Uri = c.funcNS
		}
		fn := call{
			QName: n.QName,
		}
		c.next()
		for !c.done() && !c.is(endGrp) {
			arg, err := c.compileExpr(powLowest)
			if err != nil {
				return fn, err
			}
			fn.args = append(fn.args, arg)
			switch {
			case c.is(opSeq):
				c.next()
				if c.is(endGrp) {
					return fn, ErrSyntax
				}
			case c.is(endGrp):
			default:
				return fn, ErrSyntax
			}
		}
		if !c.is(endGrp) {
			return fn, c.syntaxError("call", "expected ')'")
		}
		c.next()
		return fn, nil
	}
	c.Enter("function-call")
	defer c.Leave("function-call")
	switch e := left.(type) {
	case axis:
		return compile(e.next)
	default:
		fn, err := compile(left)
		if err != nil {
			return nil, err
		}
		return fn, nil
	}
}

func (c *Compiler) compileCall(left Expr) (Expr, error) {
	c.Enter("call")
	defer c.Leave("call")

	switch left.(type) {
	case array, identifier, subscript:
		return c.compileSubscriptCall(left)
	default:
		return c.compileFunctionCall(left)
	}

}

func (c *Compiler) compileExpr(pow int) (Expr, error) {
	c.Enter("expr")
	defer c.Leave("expr")
	fn, ok := c.prefix[c.curr.Type]
	if !ok {
		return nil, c.unexpectedError("expression")
	}
	left, err := fn()
	if err != nil {
		return nil, err
	}
	for {
		fn, ok := c.postfix[c.curr.Type]
		if !ok {
			break
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	for !(c.done() || c.endExpr()) && pow < c.power() {
		fn, ok := c.infix[c.curr.Type]
		if !ok {
			return nil, c.unexpectedError("expression")
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (c *Compiler) compileVariable() (Expr, error) {
	c.Enter("variable")
	defer c.Leave("variable")
	defer c.next()
	v := identifier{
		ident: c.getCurrentLiteral(),
	}
	return v, nil
}

func (c *Compiler) compileKind() (Expr, error) {
	c.Enter("kind")
	defer c.Leave("kind")

	noarg := func() error {
		c.next()
		if !c.is(begGrp) {
			return ErrSyntax
		}
		c.next()
		if !c.is(endGrp) {
			return c.syntaxError("kind", "expected ')'")
		}
		c.next()
		return nil
	}
	qnarg := func(allowNoArg bool) (Expr, error) {
		c.next()
		if !c.is(begGrp) {
			return nil, ErrSyntax
		}
		c.next()
		if c.is(endGrp) {
			if !allowNoArg {
				return nil, ErrSyntax
			}
			c.next()
			return nil, nil
		}
		var (
			expr Expr
			err  error
		)
		if c.is(opMul) {
			expr = wildcard{}
			c.next()
		} else {
			expr, err = c.compileQName()
		}
		if err != nil {
			return nil, err
		}
		if !c.is(endGrp) {
			return nil, c.syntaxError("kind", "expected ')'")
		}
		c.next()
		return expr, nil
	}

	var (
		expr Expr
		err  error
	)
	switch c.getCurrentLiteral() {
	case "node":
		expr = typeNode{}
		err = noarg()
	case "element":
		var el typeElement
		el.name, err = qnarg(true)
		if err == nil {
			expr = el
		}
	case "text":
		expr = typeText{}
		err = noarg()
	case "comment":
		expr = typeComment{}
		err = noarg()
	case "attribute":
		var el typeAttribute
		el.name, err = qnarg(false)
		if err == nil {
			expr = el
		}
	case "processing-instruction":
		var el typeInstruction
		el.name, err = qnarg(true)
		if err == nil {
			expr = el
		}
	case "document-node":
		expr = typeDocument{}
		err = noarg()
	default:
		return nil, fmt.Errorf("kind test not supported")
	}
	return expr, err
}

func (c *Compiler) compileAxis() (Expr, error) {
	c.Enter("axis")
	defer c.Leave("axis")

	a := axis{
		kind: c.getCurrentLiteral(),
	}
	c.next()
	c.next()
	expr, err := c.compileNameTest()
	if err != nil {
		return nil, err
	}
	a.next = expr
	return a, nil
}

func (c *Compiler) compileName() (Expr, error) {
	c.Enter("name")
	defer c.Leave("name")

	if c.peek.Type == opAxis {
		return c.compileAxis()
	}
	name, err := c.compileNameTest()
	if err != nil {
		return nil, err
	}
	expr := axis{
		kind: childAxis,
		next: name,
	}
	if _, ok := name.(typeAttribute); ok {
		expr.kind = attributeAxis
	}
	return expr, nil
}

func (c *Compiler) compileNameTest() (Expr, error) {
	if c.is(opMul) && c.peek.Type != Namespace {
		c.next()
		return wildcard{}, nil
	}
	if isKind(c.getCurrentLiteral()) && c.peek.Type == begGrp {
		return c.compileKind()
	}
	return c.compileQName()
}

func (c *Compiler) compileQName() (Expr, error) {
	var (
		qn        = xml.LocalName(c.getCurrentLiteral())
		resolveNS = true
	)
	if c.is(opMul) {
		qn.Name = "*"
	}
	c.next()
	if c.is(Namespace) && c.peek.Type != blank {
		c.next()
		qn.Space = qn.Name
		if !c.is(Name) && !c.is(opMul) {
			return nil, c.syntaxError("name", "expected name")
		}
		qn.Name = c.getCurrentLiteral()
		if c.is(opMul) {
			qn.Name = "*"
		}
		if c.peek.Type == begGrp {
			resolveNS = false
		}
		c.next()
	}
	if resolveNS {
		qn.Uri, _ = c.resolveNS(qn)
		if qn.Uri == "" {
			qn.Uri = c.elemNS
		}
	}
	n := name{
		QName: qn,
	}
	return n, nil
}

func (c *Compiler) compileCurrent() (Expr, error) {
	c.Enter("current")
	defer c.Leave("current")
	c.next()
	return current{}, nil
}

func (c *Compiler) compileParent() (Expr, error) {
	c.Enter("parent")
	defer c.Leave("parent")
	c.next()
	expr := axis{
		kind: parentAxis,
		next: typeElement{},
	}
	return expr, nil
}

func (c *Compiler) compileStep(left Expr) (Expr, error) {
	c.Enter("step")
	defer c.Leave("step")

	c.next()
	next, err := c.compileExpr(powStep)
	if err != nil {
		return nil, err
	}
	expr := step{
		curr: left,
		next: next,
	}
	return expr, nil
}

func (c *Compiler) compileDescendantStep(left Expr) (Expr, error) {
	c.Enter("descendant-step")
	defer c.Leave("descendant-step")

	c.next()
	next, err := c.compileExpr(powStep)
	if err != nil {
		return nil, err
	}

	expr := step{
		curr: left,
		next: step{
			curr: axis{
				kind: descendantSelfAxis,
				next: typeNode{},
			},
			next: next,
		},
	}
	return expr, nil
}

func (c *Compiler) compileRoot() (Expr, error) {
	c.Enter("root")
	defer c.Leave("root")

	c.next()
	if c.done() {
		return root{}, nil
	}
	next, err := c.compileExpr(powStep)
	if err != nil {
		return nil, err
	}
	expr := step{
		curr: root{},
		next: next,
	}
	return expr, nil
}

func (c *Compiler) compileDescendantRoot() (Expr, error) {
	c.Enter("descendant-root")
	defer c.Leave("descendant-root")

	c.next()
	next, err := c.compileExpr(powStep)
	if err != nil {
		return nil, err
	}

	expr := step{
		curr: root{},
		next: step{
			curr: axis{
				kind: descendantSelfAxis,
				next: typeNode{},
			},
			next: next,
		},
	}
	return expr, nil
}

func (c *Compiler) resolveNS(qn xml.QName) (string, error) {
	if qn.Name == "" {
		return "", nil
	}
	uri, err := c.namespaces.Resolve(qn.Space)
	if err != nil {
		return "", fmt.Errorf("%s: namespace is not defined", qn.Space)
	}
	return uri, nil
}

func (c *Compiler) power() int {
	return bindings[c.curr.Type]
}

func (c *Compiler) getCurrentLiteral() string {
	return c.curr.Literal
}

func (c *Compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *Compiler) endExpr() bool {
	switch c.getCurrentLiteral() {
	case kwSatisfies:
	case kwReturn:
	case kwIn:
	case kwElse:
	case kwThen:
	default:
		return false
	}
	return true
}

func (c *Compiler) done() bool {
	return c.is(EOF)
}

func (c *Compiler) skipBlank() {
	for c.is(blank) {
		c.next()
	}
}

func (c *Compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

func (c *Compiler) syntaxError(expr, cause string) error {
	return syntaxError(expr, cause, c.curr.Position)
}

func (c *Compiler) unexpectedError(expr string) error {
	cause := fmt.Sprintf("unexpected token %s", c.getCurrentLiteral())
	return c.syntaxError(expr, cause)
}

const (
	powLowest = iota
	powAssign // variable assignment
	powOr
	powAnd
	powCast
	powInstanceOf
	powIdentity
	powRange
	powEqual
	powCmp
	powConcat
	powIntersect
	powUnion
	powAdd
	powMul
	powPrefix
	powStep // step
	powArrow
	powPred
	powCall
	powHighest
)

var bindings = map[rune]int{
	currLevel:    powStep,
	anyLevel:     powStep,
	opInstanceOf: powInstanceOf,
	opCastAs:     powCast,
	opCastableAs: powCast,
	opUnion:      powUnion,
	opIntersect:  powIntersect,
	opExcept:     powIntersect,
	opConcat:     powConcat,
	opAssign:     powAssign,
	opIs:         powIdentity,
	opValEq:      powEqual,
	opValNe:      powEqual,
	opValGt:      powCmp,
	opValGe:      powCmp,
	opValLt:      powCmp,
	opValLe:      powCmp,
	opEq:         powEqual,
	opNe:         powEqual,
	opGt:         powCmp,
	opGe:         powCmp,
	opLt:         powCmp,
	opLe:         powCmp,
	opBefore:     powCmp,
	opAfter:      powCmp,
	opAnd:        powAnd,
	opOr:         powOr,
	opAdd:        powAdd,
	opSub:        powAdd,
	opMul:        powMul,
	opDiv:        powMul,
	opMod:        powMul,
	opRange:      powRange,
	opArrow:      powArrow,
	begGrp:       powCall,
	begPred:      powPred,
}

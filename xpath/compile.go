package xpath

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

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

type Compiler struct {
	scan     *Scanner
	curr     Token
	peek     Token
	lastStep rune

	Tracer
	mode StepMode

	infix   map[rune]func(Expr) (Expr, error)
	postfix map[rune]func(Expr) (Expr, error)
	prefix  map[rune]func() (Expr, error)
}

func NewCompiler(r io.Reader) *Compiler {
	cp := Compiler{
		scan:   Scan(r),
		Tracer: discardTracer{},
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

	cp.next()
	cp.next()
	return &cp
}

func CompileString(q string) (Expr, error) {
	return Compile(strings.NewReader(q))
}

func Compile(r io.Reader) (Expr, error) {
	return CompileMode(r, ModeDefault)
}

func CompileMode(r io.Reader, mode StepMode) (Expr, error) {
	cp := NewCompiler(r)
	cp.mode = mode
	return cp.Compile()
}

func (c *Compiler) Compile() (Expr, error) {
	expr, err := c.compile()
	if err != nil {
		return nil, err
	}
	if IsXsl(c.mode) {
		var base current
		expr = fromBase(expr, base)
	}
	q := query{
		expr: expr,
	}
	return q, err
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
		return nil, c.unexpectedError(c.getCurrentLiteral())
	}
}

func (c *Compiler) compileMap() (Expr, error) {
	c.Enter("map")
	defer c.Leave("map")

	c.next()
	if !c.is(begCurl) {
		return nil, ErrSyntax
	}
	c.next()
	for !c.done() && !c.is(endCurl) {

	}
	if !c.is(endCurl) {
		return nil, ErrSyntax
	}
	c.next()
	return nil, nil
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

	t, err := c.compileType()
	if err != nil {
		return nil, err
	}
	expr := instanceof{
		expr: left,
		kind: t,
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
		expr: left,
		kind: t,
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
		expr: left,
		kind: t,
	}
	return expr, nil
}

func (c *Compiler) compileType() (Type, error) {
	var t Type
	if !c.is(Name) {
		return t, c.unexpectedError("type")
	}
	t.Name = c.getCurrentLiteral()
	c.next()
	if c.is(Namespace) {
		c.next()
		if !c.is(Name) {
			return t, c.unexpectedError("type")
		}
		t.Space = t.Name
		t.Name = c.getCurrentLiteral()
		c.next()
	}
	return t, nil
}

func (c *Compiler) compileIndex(left Expr) (Expr, error) {
	c.Enter("index")
	defer c.Leave("index")
	p, err := strconv.Atoi(c.getCurrentLiteral())
	if err != nil {
		return nil, err
	}
	i := index{
		expr: left,
		pos:  p,
	}
	c.next()
	if !c.is(endPred) {
		return nil, c.syntaxError("index", "expected ']")
	}
	c.next()
	return i, nil
}

func (c *Compiler) compileFilter(left Expr) (Expr, error) {
	c.Enter("filter")
	defer c.Leave("filter")
	c.next()
	if c.is(Digit) && c.peek.Type == endPred {
		return c.compileIndex(left)
	}
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
	c.next()
	next, err := c.compileExpr(pow)
	if err != nil {
		return nil, err
	}
	a := arrow{
		left:  left,
		right: next,
	}
	return a, nil
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
	var expr kind
	switch c.getCurrentLiteral() {
	case "node":
		expr.kind = xml.TypeNode
	case "element":
		expr.kind = xml.TypeElement
	case "text":
		expr.kind = xml.TypeText
	case "comment":
		expr.kind = xml.TypeComment
	case "attribute":
		expr.kind = xml.TypeAttribute
	case "processing-instruction":
		expr.kind = xml.TypeInstruction
	case "document-node":
		expr.kind = xml.TypeDocument
	default:
		return nil, fmt.Errorf("kind test not supported")
	}
	c.next()
	if !c.is(begGrp) {
		return nil, ErrSyntax
	}
	c.next()
	if expr.kind == xml.TypeElement || expr.kind == xml.TypeAttribute {
		if !c.is(Name) {
			return nil, c.syntaxError("kind", "expected name")
		}
		expr.localName = c.getCurrentLiteral()
		c.next()
		if c.is(opSeq) {
			c.next()
			if !c.is(Name) {
				return nil, c.syntaxError("kind", "expected type")
			}
			c.next()
		}
	}
	if !c.is(endGrp) {
		return nil, c.syntaxError("kind", "expected ')'")
	}
	c.next()
	return expr, nil
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
	expr, err := c.compileNameTest()
	if err != nil {
		return nil, err
	}
	expr = axis{
		kind: childAxis,
		next: expr,
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
	qn := xml.LocalName(c.getCurrentLiteral())
	if c.is(opMul) {
		qn.Name = "*"
	}
	c.next()
	if c.is(Namespace) {
		c.next()
		qn.Space = qn.Name
		if !c.is(Name) {
			return nil, c.syntaxError("name", "expected name")
		}
		qn.Name = c.getCurrentLiteral()
		c.next()
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
	next := kind{
		kind: xml.TypeElement,
	}
	expr := axis{
		kind: parentAxis,
		next: next,
	}
	return expr, nil
}

func (c *Compiler) compileStep(left Expr) (Expr, error) {
	c.Enter("step")
	defer c.Leave("step")

	c.next()
	if c.is(begGrp) {
		return c.compileStepmap(left)
	}
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

func (c *Compiler) compileStepmap(left Expr) (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endGrp) {
		return nil, c.syntaxError("step", "expected ')'")
	}
	c.next()
	if !c.is(opSeq) && !c.is(endPred) && !c.done() {
		return nil, c.syntaxError("step", "erroneous expression")
	}
	ctx := stepmap{
		step: left,
		expr: expr,
	}
	return ctx, nil
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
				next: kind{
					kind: xml.TypeNode,
				},
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
				next: kind{
					kind: xml.TypeNode,
				},
			},
			next: next,
		},
	}
	return expr, nil
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
	powCmp
	powConcat
	powIntersect
	powUnion
	powAdd
	powMul
	powPrefix
	powStep // step
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
	opEq:         powCmp,
	opNe:         powCmp,
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
	begGrp:       powCall,
	begPred:      powPred,
}

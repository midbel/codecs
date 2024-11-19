package xml

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	doc, err := NewParser(r).Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := doc.Write(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(121)
	}

	var (
		w  bytes.Buffer
		ws = NewWriter(&w)
	)

	if path := flag.Arg(1); path != "" {
		qs := ScanQuery(strings.NewReader(path))
		for {
			tok := qs.Scan()
			fmt.Println(tok)
			if tok.Type == EOF || tok.Type == Invalid {
				break
			}
		}

		expr, err := Compile(strings.NewReader(path))
		if err != nil {
			fmt.Fprintln(os.Stderr, "compilation failed", err)
			os.Exit(1)
		}
		list, err := expr.Next(doc.Root())
		if err != nil {
			fmt.Println(os.Stderr, err)
			return
		}
		el := NewElement(LocalName("result"))
		el.Nodes = list.Nodes()
		if err := ws.Write(NewDocument(el)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(21)
		}
		fmt.Println(w.String())
	}
}

var (
	errType        = errors.New("invalid type")
	errZero        = errors.New("division by zero")
	errDiscard     = errors.New("discard")
	errImplemented = errors.New("not implemented")
	errArgument    = errors.New("invalid number of argument(s)")
)

const (
	powLowest = iota
	powLevel
	powOr
	powAnd
	powNe
	powEq
	powCmp
	powAdd
	powMul
	powPrefix
	powCall
	powPred
)

var bindings = map[rune]int{
	currLevel: powLevel,
	anyLevel:  powLevel,
	opEq:      powEq,
	opNe:      powNe,
	opGt:      powCmp,
	opGe:      powCmp,
	opLt:      powCmp,
	opLe:      powCmp,
	opAnd:     powAnd,
	opOr:      powOr,
	opAdd:     powAdd,
	opSub:     powAdd,
	opMul:     powMul,
	opDiv:     powMul,
	opMod:     powMul,
	opChain:   powCall,
	begGrp:    powCall,
	begPred:   powPred,
}

type NodeList struct {
	nodes []Node
}

func createList() *NodeList {
	return &NodeList{}
}

func (n *NodeList) Nodes() []Node {
	tmp := slices.Clone(n.nodes)
	return tmp
}

func (n *NodeList) Merge(other *NodeList) {
	n.nodes = slices.Concat(n.nodes, other.nodes)
}

func (n *NodeList) Push(node Node) {
	n.nodes = append(n.nodes, node)
}

func (n *NodeList) Empty() bool {
	return len(n.nodes) == 0
}

func (n *NodeList) Len() int {
	return len(n.nodes)
}

func (n *NodeList) All() iter.Seq[Node] {
	do := func(yield func(Node) bool) {
		for _, n := range n.nodes {
			if !yield(n) {
				break
			}
		}
	}
	return do
}

const (
	childAxis          = "child"
	parentAxis         = "parent"
	selfAxis           = "self"
	ancestorAxis       = "ancestor"
	ancestorSelfAxis   = "ancestor-or-self"
	descendantAxis     = "descendant"
	descendantSelfAxis = "descendant-or-self"
	prevAxis           = "preceding"
	prevSiblingAxis    = "preceding-sibling"
	nextAxis           = "following"
	nextSiblingAxis    = "following-sibling"
)

type all struct{}

func (_ all) Next(curr Node) (*NodeList, error) {
	_, ok := curr.(*Element)
	if !ok {
		return nil, fmt.Errorf("element node expected")
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type current struct{}

func (_ current) Next(curr Node) (*NodeList, error) {
	list := createList()
	list.Push(curr)
	return list, nil
}

type absolute struct {
	expr Expr
}

func (a absolute) Next(curr Node) (*NodeList, error) {
	return a.expr.Next(a.root(curr))
}

func (a absolute) root(node Node) Node {
	n := node.Parent()
	if n == nil {
		return node
	}
	return a.root(n)
}

type root struct{}

func (_ root) Next(curr Node) (*NodeList, error) {
	n := curr.Parent()
	if n != nil {
		return nil, fmt.Errorf("root element expected")
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type parent struct{}

func (_ parent) Next(curr Node) (*NodeList, error) {
	n := curr.Parent()
	if n == nil {
		return nil, fmt.Errorf("root element has no parent")
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type name struct {
	axis  string
	space string
	ident string
}

func (n name) Next(curr Node) (*NodeList, error) {
	list := createList()
	switch n.axis {
	case childAxis, "":
		el, ok := curr.(*Element)
		if !ok {
			return nil, fmt.Errorf("element node expected")
		}
		for _, c := range el.Nodes {
			if c.LocalName() != n.ident {
				continue
			}
			list.Push(c)
		}
	case parentAxis:
		p := curr.Parent()
		if p != nil && p.LocalName() == n.ident {
			list.Push(p)
		}
	case selfAxis:
		list.Push(curr)
	case ancestorAxis, ancestorSelfAxis:
		for x := curr; ; {
			p := x.Parent()
			if p == nil {
				break
			}
			if p.LocalName() == n.ident {
				list.Push(p)
			}
			x = p
		}
		if n.axis == ancestorSelfAxis && curr.LocalName() == n.ident {
			list.Push(curr)
		}
	case descendantAxis, descendantSelfAxis:
		el, ok := curr.(*Element)
		if !ok {
			return nil, fmt.Errorf("element node expected")
		}
		for i := range el.Nodes {
			other, err := n.Next(el.Nodes[i])
			if err != nil {
				continue
			}
			list.Merge(other)
		}
		if n.axis == descendantSelfAxis && curr.LocalName() == n.ident {
			list.Push(curr)
		}
	default:
		return nil, errImplemented
	}
	return list, nil
}

func (n name) Eval(curr Node) (any, error) {
	el, ok := curr.(*Element)
	if !ok {
		return nil, fmt.Errorf("element node expected")
	}
	child := el.Find(n.ident)
	if child == nil {
		return "", nil
	}
	return child.Value(), nil
}

type descendant struct {
	curr Expr
	next Expr
	deep bool
}

func (d descendant) Next(node Node) (*NodeList, error) {
	ns, err := d.curr.Next(node)
	if err != nil {
		return nil, err
	}
	list := createList()
	for i := range ns.All() {
		xs, err := d.traverse(i)
		if err != nil {
			continue
		}
		list.Merge(xs)
	}
	return list, nil
}

func (d *descendant) traverse(n Node) (*NodeList, error) {
	list, err := d.next.Next(n)
	if err == nil && list.Len() > 0 {
		return list, nil
	}
	if !d.deep {
		return nil, errDiscard
	}
	el, ok := n.(*Element)
	if !ok {
		return nil, errDiscard
	}
	list = createList()
	for i := range el.Nodes {
		tmp, err := d.next.Next(el.Nodes[i])
		if (err != nil || tmp.Len() == 0) && d.deep {
			tmp, err = d.traverse(el.Nodes[i])
		}
		if err == nil {
			list.Merge(tmp)
		}
	}

	return list, nil
}

type Predicate interface {
	Eval(Node) (any, error)
}

type noopExpr struct {
	Predicate
}

func createNoop(p Predicate) Expr {
	return noopExpr{
		Predicate: p,
	}
}

func evalExpr(e Expr, node Node) (any, error) {
	p, ok := e.(Predicate)
	if !ok {
		return nil, fmt.Errorf("expression can not be use as a predicate")
	}
	return p.Eval(node)
}

func (_ noopExpr) Next(_ Node) (*NodeList, error) {
	return nil, errImplemented
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Eval(node Node) (any, error) {
	left, err := evalExpr(b.left, node)
	if err != nil {
		return nil, err
	}
	right, err := evalExpr(b.right, node)
	if err != nil {
		return nil, err
	}
	switch b.op {
	case opAdd:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left + right, nil
		})
	case opSub:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left - right, nil
		})
	case opMul:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left * right, nil
		})
	case opDiv:
		return apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return left / right, nil
		})
	case opMod:
		return apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return math.Mod(left, right), nil
		})
	case opAnd:
		return toBool(left) && toBool(right), nil
	case opOr:
		return toBool(left) || toBool(right), nil
	case opEq:
		ok, err := isEqual(left, right)
		return ok, err
	case opNe:
		ok, err := isEqual(left, right)
		return !ok, err
	case opLt:
		ok, err := isLess(left, right)
		return ok, err
	case opLe:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
		}
		return ok, err
	case opGt:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
			ok = !ok
		}
		return ok, err
	case opGe:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
			ok = !ok
		}
		return ok, err
	default:
		return nil, errImplemented
	}
}

type reverse struct {
	expr Expr
}

func (r reverse) Eval(node Node) (any, error) {
	v, err := evalExpr(r.expr, node)
	if err != nil {
		return nil, err
	}
	x, err := toFloat(v)
	if err == nil {
		x = -x
	}
	return x, err
}

type literal struct {
	expr string
}

func (i literal) Eval(_ Node) (any, error) {
	return i.expr, nil
}

type number struct {
	expr float64
}

func (n number) Eval(_ Node) (any, error) {
	return n.expr, nil
}

type chain struct {
	expr Expr
	next Expr
}

func (c chain) Eval(node Node) (any, error) {
	return nil, errImplemented
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Eval(node Node) (any, error) {
	fn, ok := builtins[c.ident]
	if !ok {
		return nil, fmt.Errorf("undefined function")
	}
	if fn == nil {
		return nil, errImplemented
	}
	var args []any
	for i := range c.args {
		a, err := evalExpr(c.args[i], node)
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
	return fn(node, args)
}

type attr struct {
	ident string
}

func (a attr) Next(node Node) (*NodeList, error) {
	return nil, errImplemented
}

func (a attr) Eval(node Node) (any, error) {
	el, ok := node.(*Element)
	if !ok {
		return nil, fmt.Errorf("element node expected")
	}
	ix := slices.IndexFunc(el.Attrs, func(attr Attribute) bool {
		return attr.Name == a.ident
	})
	if ix >= 0 {
		return el.Attrs[ix].Value, nil
	}
	return "", nil
}

type alternative struct {
	all []Expr
}

func (a alternative) Next(node Node) (*NodeList, error) {
	list := createList()
	for i := range a.all {
		res, err := a.all[i].Next(node)
		if err != nil {
			return nil, err
		}
		list.Merge(res)
	}
	return list, nil
}

func apply(left, right any, do func(left, right float64) (float64, error)) (any, error) {
	x, err := toFloat(left)
	if err != nil {
		return nil, err
	}
	y, err := toFloat(right)
	if err != nil {
		return nil, err
	}
	return do(x, y)
}

func isLess(left, right any) (bool, error) {
	switch x := left.(type) {
	case float64:
		y, err := toFloat(right)
		return x < y, err
	case string:
		y, err := toString(right)
		return x < y, err
	default:
		return false, errType
	}
}

func isEqual(left, right any) (bool, error) {
	switch x := left.(type) {
	case float64:
		y, err := toFloat(right)
		if err != nil {
			return false, err
		}
		return x == y, nil
	case string:
		y, err := toString(right)
		if err != nil {
			return false, err
		}
		return x == y, nil
	case bool:
		return x == toBool(right), nil
	default:
		return false, errType
	}
}

func toFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, errType
	}
}

func toString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", errType
	}
}

func toBool(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return len(v) > 0
	default:
		return false
	}
}

type filter struct {
	expr  Expr
	check Expr
}

func (f filter) Next(curr Node) (*NodeList, error) {
	list, err := f.expr.Next(curr)
	if err != nil {
		return nil, err
	}
	ret := createList()
	for n := range list.All() {
		res, err := evalExpr(f.check, n)
		if err != nil {
			continue
		}
		switch x := res.(type) {
		case float64:
			if int(x) == n.Position() {
				ret.Push(n)
			}
		case bool:
			if x {
				ret.Push(n)
			}
		default:
			return nil, errType
		}
	}
	return ret, nil
}

type Expr interface {
	Next(Node) (*NodeList, error)
}

type query struct {
	expr Expr
}

func (q query) Next(node Node) (*NodeList, error) {
	if r := node.Parent(); r == nil {
		var qn QName
		root := NewElement(qn)
		root.Append(node)
		node = root
	}
	return q.expr.Next(node)
}

type compiler struct {
	scan *QueryScanner
	curr Token
	peek Token

	infix  map[rune]func(Expr) (Expr, error)
	prefix map[rune]func() (Expr, error)
}

func Compile(r io.Reader) (Expr, error) {
	cp := compiler{
		scan: ScanQuery(r),
	}

	cp.infix = map[rune]func(Expr) (Expr, error){
		currLevel: cp.compileDescendant,
		anyLevel:  cp.compileDescendant,
		begPred:   cp.compileFilter,
		opAdd:     cp.compileBinary,
		opSub:     cp.compileBinary,
		opMul:     cp.compileBinary,
		opDiv:     cp.compileBinary,
		opMod:     cp.compileBinary,
		opEq:      cp.compileBinary,
		opNe:      cp.compileBinary,
		opGt:      cp.compileBinary,
		opGe:      cp.compileBinary,
		opLt:      cp.compileBinary,
		opLe:      cp.compileBinary,
		opChain:   cp.compileChain,
		begGrp:    cp.compileCall,
	}
	cp.prefix = map[rune]func() (Expr, error){
		currLevel:  cp.compileRoot,
		Name:       cp.compileName,
		currNode:   cp.compileCurrent,
		parentNode: cp.compileParent,
		attrNode:   cp.compileAttr,
		Literal:    cp.compileLiteral,
		Digit:      cp.compileNumber,
		opSub:      cp.compileReverse,
		opMul:      cp.compileName,
	}

	cp.next()
	cp.next()
	return cp.Compile()
}

func (c *compiler) Compile() (Expr, error) {
	expr, err := c.compile()
	if err == nil {
		q := query{
			expr: expr,
		}
		expr = q
	}
	return expr, err
}

func (c *compiler) compile() (Expr, error) {
	var (
		alt alternative
		do  = func() (Expr, error) {
			if c.is(currLevel) {
				return c.compileRoot()
			}
			if c.is(anyLevel) {
				var expr root
				return c.compileDescendant(expr)
			}
			return c.compileExpr(powLowest)
		}
	)
	for !c.done() {
		e, err := do()
		if err != nil {
			return nil, err
		}
		alt.all = append(alt.all, e)
		switch {
		case c.is(opAlt):
			c.next()
			if c.done() {
				return nil, fmt.Errorf("syntax error")
			}
		case c.done():
		default:
			return nil, fmt.Errorf("syntax error")
		}
	}
	if len(alt.all) == 1 {
		return alt.all[0], nil
	}
	return alt, nil
}

func (c *compiler) compileFilter(left Expr) (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endPred) {
		return nil, fmt.Errorf("syntax error: missing ']' after filter")
	}
	c.next()

	f := filter{
		expr:  left,
		check: expr,
	}
	return f, nil
}

func (c *compiler) compileBinary(left Expr) (Expr, error) {
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
	return createNoop(b), nil
}

func (c *compiler) compileLiteral() (Expr, error) {
	defer c.next()
	i := literal{
		expr: c.curr.Literal,
	}
	return createNoop(i), nil
}

func (c *compiler) compileNumber() (Expr, error) {
	defer c.next()
	f, err := strconv.ParseFloat(c.curr.Literal, 64)
	if err != nil {
		return nil, err
	}
	n := number{
		expr: f,
	}
	return createNoop(n), nil
}

func (c *compiler) compileReverse() (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powPrefix)
	if err != nil {
		return nil, err
	}
	r := reverse{
		expr: expr,
	}
	return createNoop(r), nil
}

func (c *compiler) compileAttr() (Expr, error) {
	defer c.next()
	a := attr{
		ident: c.curr.Literal,
	}
	return createNoop(a), nil
}

func (c *compiler) compileChain(left Expr) (Expr, error) {
	ch := chain{
		expr: left,
	}
	return createNoop(ch), nil
}

func (c *compiler) compileCall(left Expr) (Expr, error) {
	n, ok := left.(name)
	if !ok {
		return nil, fmt.Errorf("invalid function identifier")
	}
	c.next()
	fn := call{
		ident: n.ident,
	}
	for !c.done() && !c.is(endGrp) {
		arg, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		fn.args = append(fn.args, arg)
		switch {
		case c.is(opSeq):
			c.next()
			if c.is(endGrp) {
				return nil, fmt.Errorf("syntax error")
			}
		case c.is(endGrp):
		default:
			return nil, fmt.Errorf("syntax error")
		}
	}
	if !c.is(endGrp) {
		return nil, fmt.Errorf("syntax error: missing closing ')'")
	}
	c.next()
	return createNoop(fn), nil
}

func (c *compiler) compileExpr(pow int) (Expr, error) {
	fn, ok := c.prefix[c.curr.Type]
	if !ok {
		return nil, fmt.Errorf("unexpected prefix expression")
	}
	left, err := fn()
	if err != nil {
		return nil, err
	}
	for !c.done() && !c.is(opAlt) && pow < bindings[c.curr.Type] {
		fn, ok := c.infix[c.curr.Type]
		if !ok {
			return nil, fmt.Errorf("unexpected infix expression")
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (c *compiler) compileRoot() (Expr, error) {
	c.next()
	if c.done() {
		return root{}, nil
	}
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	a := absolute{
		expr: next,
	}
	return a, nil
}

func (c *compiler) compileName() (Expr, error) {
	if c.is(opMul) {
		c.next()
		var a all
		return a, nil
	}
	n := name{
		axis:  childAxis,
		ident: c.curr.Literal,
	}
	c.next()
	if c.is(opAxis) {
		c.next()
		n.axis = n.ident
		if !c.is(Name) {
			return nil, fmt.Errorf("name expected")
		}
		n.ident = c.curr.Literal
		c.next()
	}
	if c.is(Namespace) {
		c.next()
		n.space = n.ident
		if !c.is(Name) {
			return nil, fmt.Errorf("name expected")
		}
		n.ident = c.curr.Literal
		c.next()
	}
	return n, nil
}

func (c *compiler) compileCurrent() (Expr, error) {
	c.next()
	return current{}, nil
}

func (c *compiler) compileParent() (Expr, error) {
	c.next()
	return parent{}, nil
}

func (c *compiler) compileDescendant(left Expr) (Expr, error) {
	d := descendant{
		curr: left,
		deep: c.is(anyLevel),
	}
	c.next()
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	d.next = next
	return d, nil
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *compiler) done() bool {
	return c.is(EOF)
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

const (
	currNode = -(iota + 1000)
	parentNode
	attrNode
	currLevel
	anyLevel
	begPred
	endPred
	begGrp
	endGrp
	opAdd
	opSub
	opMul
	opDiv
	opMod
	opEq
	opNe
	opGt
	opGe
	opLt
	opLe
	opAlt
	opAnd
	opOr
	opSeq
	opChain
	opAxis
)

type QueryScanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	predicate bool
}

func ScanQuery(r io.Reader) *QueryScanner {
	scan := &QueryScanner{
		input: bufio.NewReader(r),
	}
	scan.Line = 1
	scan.read()
	return scan
}

func (s *QueryScanner) Scan() Token {
	var tok Token
	if s.done() {
		tok.Position = s.Position
		tok.Type = EOF
		return tok
	}
	s.str.Reset()

	s.skipBlank()
	tok.Position = s.Position
	switch {
	case isOperator(s.char):
		s.scanOperator(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case s.char == arobase:
		s.scanAttr(&tok)
	case s.char == apos || s.char == quote:
		s.scanLiteral(&tok)
	case unicode.IsLetter(s.char):
		s.scanIdent(&tok)
	case unicode.IsDigit(s.char):
		s.scanNumber(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *QueryScanner) scanOperator(tok *Token) {
	switch k := s.peek(); s.char {
	case plus:
		tok.Type = opAdd
	case dash:
		tok.Type = opSub
	case star:
		tok.Type = opMul
	case percent:
		tok.Type = opMod
	case equal:
		tok.Type = opEq
		if k == rangle {
			s.read()
			tok.Type = opChain
		}
	case bang:
		tok.Type = Invalid
		if k == equal {
			s.read()
			tok.Type = opNe
		}
	case langle:
		tok.Type = opLt
		if k == equal {
			s.read()
			tok.Type = opLe
		}
	case rangle:
		tok.Type = opGt
		if k == equal {
			s.read()
			tok.Type = opGe
		}
	case lparen:
		tok.Type = begGrp
	case rparen:
		tok.Type = endGrp
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
		s.skipBlank()
	}
}

func (s *QueryScanner) scanDelimiter(tok *Token) {
	switch k := s.peek(); s.char {
	case colon:
		tok.Type = Namespace
		if k == colon {
			s.read()
			tok.Type = opAxis
		}
	case dot:
		tok.Type = currNode
		s.read()
		if s.char == dot {
			tok.Type = parentNode
		}
	case comma:
		tok.Type = opSeq
	case pipe:
		tok.Type = opAlt
	case lsquare:
		tok.Type = begPred
		s.enterPredicate()
	case rsquare:
		tok.Type = endPred
		s.leavePredicate()
	case slash:
		tok.Type = currLevel
		if k := s.peek(); k == slash {
			s.read()
			tok.Type = anyLevel
		}
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
		s.skipBlank()
	}
}

func (s *QueryScanner) scanLiteral(tok *Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.str.String()
	if s.char != quote {
		tok.Type = Invalid
	}
	s.read()
}

func (s *QueryScanner) scanAttr(tok *Token) {
	s.read()
	s.scanIdent(tok)
	tok.Type = attrNode
}

func (s *QueryScanner) scanNumber(tok *Token) {
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Digit
	tok.Literal = s.str.String()
	if s.char != dot {
		return
	}
	s.write()
	s.read()
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
}

func (s *QueryScanner) scanIdent(tok *Token) {
	accept := func() bool {
		return unicode.IsLetter(s.char) || unicode.IsDigit(s.char) ||
			s.char == dash || s.char == underscore
	}
	for !s.done() && accept() {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "and":
		tok.Type = opAnd
	case "or":
		tok.Type = opOr
	case "div":
		tok.Type = opDiv
	case "mod":
		tok.Type = opMod
	default:
		tok.Type = Name
	}
	s.skipBlank()
}

func (s *QueryScanner) enterPredicate() {
	s.predicate = true
}

func (s *QueryScanner) leavePredicate() {
	s.predicate = false
}

func (s *QueryScanner) skipBlank() {
	s.skip(unicode.IsSpace)
}

func (s *QueryScanner) skip(accept func(r rune) bool) {
	for accept(s.char) {
		s.read()
	}
}

func (s *QueryScanner) write() {
	s.str.WriteRune(s.char)
}

func (s *QueryScanner) read() {
	s.old = s.Position
	if s.char == '\n' {
		s.Column = 0
		s.Line++
	}
	s.Column++
	c, _, err := s.input.ReadRune()
	if err != nil {
		s.char = utf8.RuneError
	} else {
		s.char = c
	}
}

func (s *QueryScanner) peek() rune {
	defer s.input.UnreadRune()
	c, _, _ := s.input.ReadRune()
	return c
}

func (s *QueryScanner) done() bool {
	return s.char == utf8.RuneError
}

func isDelimiter(c rune) bool {
	return c == comma || c == dot || c == pipe || c == slash ||
		c == lsquare || c == rsquare || c == colon
}

func isOperator(c rune) bool {
	return c == plus || c == dash || c == star || c == percent ||
		c == equal || c == bang || c == langle || c == rangle ||
		c == lparen || c == rparen
}

type Writer struct {
	writer *bufio.Writer

	Compact  bool
	Indent   string
	NoProlog bool
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
		Indent: "  ",
	}
}

func (w *Writer) Write(doc *Document) error {
	if w.Compact {
		w.Indent = ""
	}
	if err := w.writeProlog(); err != nil {
		return err
	}
	w.writeNL()
	return w.writeNode(doc.root, -1)
}

func (w *Writer) writeNode(node Node, depth int) error {
	switch node := node.(type) {
	case *Element:
		return w.writeElement(node, depth+1)
	case *CharData:
		return w.writeCharData(node, depth+1)
	case *Text:
		return w.writeLiteral(node, depth+1)
	case *Instruction:
		return w.writeInstruction(node, depth+1)
	case *Comment:
		return w.writeComment(node, depth+1)
	default:
		return fmt.Errorf("node: unknown type")
	}
}

func (w *Writer) writeElement(node *Element, depth int) error {
	w.writeNL()

	prefix := strings.Repeat(w.Indent, depth)
	if prefix != "" {
		w.writer.WriteString(prefix)
	}
	w.writer.WriteRune(langle)
	w.writer.WriteString(node.QualifiedName())
	level := depth + 1
	if len(node.Attrs) == 1 {
		level = 0
	}
	if err := w.writeAttributes(node.Attrs, level); err != nil {
		return err
	}
	if len(node.Nodes) == 0 {
		w.writer.WriteRune(slash)
		w.writer.WriteRune(rangle)
		return w.writer.Flush()
	}
	w.writer.WriteRune(rangle)
	for _, n := range node.Nodes {
		if err := w.writeNode(n, depth+1); err != nil {
			return err
		}
	}
	if n := len(node.Nodes); n > 0 {
		_, ok := node.Nodes[n-1].(*Text)
		if !ok {
			w.writeNL()
			w.writer.WriteString(prefix)
		}
	}
	w.writer.WriteRune(langle)
	w.writer.WriteRune(slash)
	w.writer.WriteString(node.QualifiedName())
	w.writer.WriteRune(rangle)
	return w.writer.Flush()
}

func (w *Writer) writeLiteral(node *Text, _ int) error {
	_, err := w.writer.WriteString(node.Content)
	return err
}

func (w *Writer) writeCharData(node *CharData, _ int) error {
	w.writer.WriteRune(langle)
	w.writer.WriteRune(bang)
	w.writer.WriteRune(lsquare)
	w.writer.WriteString("CDATA")
	w.writer.WriteRune(lsquare)
	w.writer.WriteString(node.Content)
	w.writer.WriteRune(rsquare)
	w.writer.WriteRune(rsquare)
	w.writer.WriteRune(rangle)
	return nil
}

func (w *Writer) writeComment(node *Comment, depth int) error {
	w.writeNL()
	prefix := strings.Repeat(w.Indent, depth)
	w.writer.WriteString(prefix)
	w.writer.WriteRune(langle)
	w.writer.WriteRune(bang)
	w.writer.WriteRune(dash)
	w.writer.WriteRune(dash)
	w.writer.WriteString(node.Content)
	w.writer.WriteRune(dash)
	w.writer.WriteRune(dash)
	w.writer.WriteRune(rangle)
	return nil
}

func (w *Writer) writeInstruction(node *Instruction, depth int) error {
	if depth > 0 {
		w.writeNL()
	}
	prefix := strings.Repeat(w.Indent, depth)
	if prefix != "" {
		w.writer.WriteString(prefix)
	}
	w.writer.WriteRune(langle)
	w.writer.WriteRune(question)
	w.writer.WriteString(node.Name)
	if err := w.writeAttributes(node.Attrs, 0); err != nil {
		return err
	}
	w.writer.WriteRune(question)
	w.writer.WriteRune(rangle)
	return w.writer.Flush()
}

func (w *Writer) writeProlog() error {
	if w.NoProlog {
		return nil
	}
	prolog := NewInstruction(LocalName("xml"))
	prolog.Attrs = []Attribute{
		NewAttribute(LocalName("version"), SupportedVersion),
		NewAttribute(LocalName("encoding"), SupportedEncoding),
	}
	return w.writeInstruction(prolog, 0)
}

func (w *Writer) writeAttributes(attrs []Attribute, depth int) error {
	prefix := strings.Repeat(w.Indent, depth)
	for _, a := range attrs {
		if depth == 0 || w.Compact {
			w.writer.WriteRune(' ')
		} else {
			w.writeNL()
			w.writer.WriteString(prefix)
		}
		w.writer.WriteString(a.QualifiedName())
		w.writer.WriteRune(equal)
		w.writer.WriteRune(quote)
		w.writer.WriteString(a.Value)
		w.writer.WriteRune(quote)
	}
	return nil
}

func (w *Writer) writeNL() {
	if w.Compact {
		return
	}
	w.writer.WriteRune('\n')
}

const MaxDepth = 512

const (
	SupportedVersion  = "1.0"
	SupportedEncoding = "UTF-8"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	depth int

	TrimSpace  bool
	KeepEmpty  bool
	OmitProlog bool
	MaxDepth   int
}

func NewParser(r io.Reader) *Parser {
	p := Parser{
		scan:      Scan(r),
		TrimSpace: true,
		MaxDepth:  MaxDepth,
	}
	p.next()
	p.next()
	return &p
}

func (p *Parser) Parse() (*Document, error) {
	if _, err := p.parseProlog(); err != nil {
		return nil, err
	}
	for p.is(Literal) {
		p.next()
	}
	var (
		doc Document
		err error
	)
	doc.Version = SupportedVersion
	doc.Encoding = SupportedEncoding
	doc.root, err = p.parseNode()
	return &doc, err
}

func (p *Parser) parseProlog() (Node, error) {
	if !p.is(ProcInstTag) {
		if !p.OmitProlog {
			return nil, p.formatError("xml: xml prolog missing")
		}
		return nil, nil
	}
	node, err := p.parseProcessingInstr()
	if err != nil {
		return nil, err
	}
	pi, ok := node.(*Instruction)
	if !ok {
		return nil, fmt.Errorf("processing instruction expected")
	}
	ok = slices.ContainsFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "version" && a.Value == SupportedVersion
	})
	if !ok {
		return nil, fmt.Errorf("xml version not supported!")
	}
	ok = slices.ContainsFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "encoding" && a.Value == SupportedEncoding
	})
	if !ok {
		return nil, fmt.Errorf("xml encoding not supported!")
	}
	return pi, nil
}

func (p *Parser) parseNode() (Node, error) {
	p.enter()
	defer p.leave()
	if p.depth >= p.MaxDepth {
		return nil, p.formatError("xml: maximum depth reached!")
	}
	var (
		node Node
		err  error
	)
	switch p.curr.Type {
	case OpenTag:
		node, err = p.parseElement()
	case CommentTag:
		node, err = p.parseComment()
	case ProcInstTag:
		node, err = p.parseProcessingInstr()
	case Cdata:
		node, _ = p.parseCharData()
	case Literal:
		node, _ = p.parseLiteral()
	default:
		return nil, p.formatError("xml: unexpected element type")
	}
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (p *Parser) parseElement() (Node, error) {
	p.next()
	var (
		elem Element
		err  error
	)
	if p.is(Namespace) {
		elem.Space = p.curr.Literal
		p.next()
	}
	if !p.is(Name) {
		return nil, p.formatError("element: missing name")
	}
	elem.Name = p.curr.Literal
	p.next()

	elem.Attrs, err = p.parseAttributes(func() bool {
		return p.is(EndTag) || p.is(EmptyElemTag)
	})
	if err != nil {
		return nil, err
	}
	switch p.curr.Type {
	case EmptyElemTag:
		p.next()
		return &elem, nil
	case EndTag:
		p.next()
		var pos int
		for !p.done() && !p.is(CloseTag) {
			child, err := p.parseNode()
			if err != nil {
				return nil, err
			}
			if child != nil {
				child.setPosition(pos)
				child.setParent(&elem)
				elem.Nodes = append(elem.Nodes, child)
				pos++
			}
		}
		if !p.is(CloseTag) {
			return nil, p.formatError("element: missing closing element")
		}
		p.next()
		return &elem, p.parseCloseElement(elem)
	default:
		return nil, p.formatError("element: malformed - expected end of element")
	}
}

func (p *Parser) parseCloseElement(elem Element) error {
	if p.is(Namespace) {
		if elem.Space != p.curr.Literal {
			return p.formatError("element: namespace mismatched!")
		}
		p.next()
	}
	if !p.is(Name) {
		return fmt.Errorf("element: missing name")
	}
	if p.curr.Literal != elem.Name {
		return p.formatError("element: name mismatched!")
	}
	p.next()
	if !p.is(EndTag) {
		return p.formatError("element: malformed - expected end of element")
	}
	p.next()
	return nil
}

func (p *Parser) parseProcessingInstr() (Node, error) {
	p.next()
	if !p.is(Name) {
		return nil, p.formatError("pi: missing name")
	}
	var elem Instruction
	elem.Name = p.curr.Literal
	p.next()
	var err error
	elem.Attrs, err = p.parseAttributes(func() bool {
		return p.is(ProcInstTag)
	})
	if err != nil {
		return nil, err
	}
	if !p.is(ProcInstTag) {
		return nil, p.formatError("pi: malformed - expected end of element")
	}
	p.next()
	return &elem, nil
}

func (p *Parser) parseAttributes(done func() bool) ([]Attribute, error) {
	var attrs []Attribute
	for !p.done() && !done() {
		attr, err := p.parseAttr()
		if err != nil {
			return nil, err
		}
		ok := slices.ContainsFunc(attrs, func(a Attribute) bool {
			return attr.QualifiedName() == a.QualifiedName()
		})
		if ok {
			return nil, p.formatError("attribute: duplicate attribute")
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (p *Parser) parseAttr() (Attribute, error) {
	var attr Attribute
	if p.is(Namespace) {
		attr.Space = p.curr.Literal
		p.next()
	}
	if !p.is(Attr) {
		return attr, p.formatError("attribute: attribute name expected")
	}
	attr.Name = p.curr.Literal
	p.next()
	if !p.is(Literal) {
		return attr, p.formatError("attribute: missing attribute value")
	}
	attr.Value = p.curr.Literal
	p.next()
	return attr, nil
}

func (p *Parser) parseComment() (Node, error) {
	defer p.next()
	node := Comment{
		Content: p.curr.Literal,
	}
	return &node, nil
}

func (p *Parser) parseCharData() (Node, error) {
	defer p.next()
	char := CharData{
		Content: p.curr.Literal,
	}
	return &char, nil
}

func (p *Parser) parseLiteral() (Node, error) {
	text := Text{
		Content: p.curr.Literal,
	}
	if p.TrimSpace {
		text.Content = strings.TrimSpace(text.Content)
	}
	p.next()
	if !p.KeepEmpty && text.Content == "" {
		return nil, nil
	}
	return &text, nil
}

func (p *Parser) formatError(msg string) error {
	return fmt.Errorf("(%d:%d) %s", p.curr.Line, p.curr.Column, msg)
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) enter() {
	p.depth++
}

func (p *Parser) leave() {
	p.depth--
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

const (
	EOF rune = -(1 + iota)
	Name
	Namespace // name:
	Attr      // name=
	Literal
	Digit
	Cdata
	CommentTag   // <!--
	OpenTag      // <
	EndTag       // >
	CloseTag     // </
	EmptyElemTag // />
	ProcInstTag  // <?, ?>
	Invalid
)

type Position struct {
	Line   int
	Column int
}

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	switch t.Type {
	case opAxis:
		return "<axis>"
	case currNode:
		return "<current-node>"
	case parentNode:
		return "<parent-node>"
	case attrNode:
		return fmt.Sprintf("attribute(%s)", t.Literal)
	case currLevel:
		return fmt.Sprintf("current-level(%s)", t.Literal)
	case anyLevel:
		return fmt.Sprintf("any-level(%s)", t.Literal)
	case begPred:
		return "<begin-predicate>"
	case endPred:
		return "<end-predicate>"
	case begGrp:
		return "<begin-group>"
	case endGrp:
		return "<end-group>"
	case opAdd:
		return "<add>"
	case opSub:
		return "<subtract>"
	case opMul:
		return "<multiply>"
	case opDiv:
		return "<divide>"
	case opMod:
		return "<modulo>"
	case opEq:
		return "<equal>"
	case opNe:
		return "<not-equal>"
	case opGt:
		return "<greater-than>"
	case opGe:
		return "<greater-eq>"
	case opLt:
		return "<lesser-than>"
	case opLe:
		return "<lesser-eq>"
	case opAlt:
		return "<alternative>"
	case opAnd:
		return "<and>"
	case opOr:
		return "<or>"
	case opSeq:
		return "<sequence>"
	case EOF:
		return "<eof>"
	case Digit:
		return fmt.Sprintf("number(%s)", t.Literal)
	case CommentTag:
		return fmt.Sprintf("comment(%s)", t.Literal)
	case Name:
		return fmt.Sprintf("name(%s)", t.Literal)
	case Namespace:
		return fmt.Sprintf("namespace(%s)", t.Literal)
	case Attr:
		return fmt.Sprintf("attr(%s)", t.Literal)
	case Cdata:
		return fmt.Sprintf("chardata(%s)", t.Literal)
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case OpenTag:
		return "<open-elem-tag>"
	case EndTag:
		return "<end-elem-tag>"
	case CloseTag:
		return "<close-elem-tag>"
	case EmptyElemTag:
		return "<empty-elem-tag>"
	case ProcInstTag:
		return "<processing-instruction>"
	case Invalid:
		return "<invalid>"
	default:
		return "<unknown>"
	}
}

const (
	langle     = '<'
	rangle     = '>'
	lsquare    = '['
	rsquare    = ']'
	lparen     = '('
	rparen     = ')'
	colon      = ':'
	quote      = '"'
	apos       = '\''
	slash      = '/'
	question   = '?'
	bang       = '!'
	equal      = '='
	ampersand  = '&'
	semicolon  = ';'
	dash       = '-'
	underscore = '_'
	dot        = '.'
	arobase    = '@'
	comma      = ','
	plus       = '+'
	star       = '*'
	percent    = '%'
	pipe       = '|'
)

type state int8

const (
	literalState state = 1 << iota
)

type Scanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	state
}

func Scan(r io.Reader) *Scanner {
	scan := &Scanner{
		input: bufio.NewReader(r),
	}
	scan.Position.Line = 1
	scan.read()
	return scan
}

func (s *Scanner) Scan() Token {
	var tok Token
	tok.Position = s.Position
	if s.done() {
		tok.Type = EOF
		return tok
	}

	if s.state == literalState {
		s.scanLiteral(&tok)
		return tok
	}
	s.str.Reset()
	switch {
	case s.char == langle:
		s.scanOpeningTag(&tok)
	case s.char == rangle:
		s.scanEndTag(&tok)
	case s.char == quote:
		s.scanValue(&tok)
	case s.char == slash || s.char == question:
		s.scanClosingTag(&tok)
	case unicode.IsLetter(s.char):
		s.scanName(&tok)
	default:
		s.scanLiteral(&tok)
	}
	return tok
}

func (s *Scanner) scanOpeningTag(tok *Token) {
	s.read()
	tok.Type = OpenTag
	switch s.char {
	case bang:
		s.read()
		if s.char == lsquare {
			s.scanCharData(tok)
			return
		}
		if s.char == dash {
			s.scanComment(tok)
			return
		}
		tok.Type = Invalid
	case question:
		tok.Type = ProcInstTag
	case slash:
		tok.Type = CloseTag
	default:
	}
	if tok.Type == ProcInstTag || tok.Type == CloseTag {
		s.read()
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	if s.char != dash {
		tok.Type = Invalid
		return
	}
	s.read()
	var done bool
	for !s.done() {
		if s.char == dash && s.peek() == s.char {
			s.read()
			s.read()
			if done = s.char == rangle; done {
				s.read()
				break
			}
			s.str.WriteRune(dash)
			s.str.WriteRune(dash)
			continue
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = CommentTag
	if !done {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanCharData(tok *Token) {
	s.read()
	for !s.done() && s.char != lsquare {
		s.write()
		s.read()
	}
	s.read()
	if s.str.String() != "CDATA" {
		tok.Type = Invalid
		return
	}
	s.str.Reset()
	var done bool
	for !s.done() {
		if s.char == rsquare && s.peek() == s.char {
			s.read()
			s.read()
			if done = s.char == rangle; done {
				s.read()
				break
			}
			s.str.WriteRune(rsquare)
			s.str.WriteRune(rsquare)
			continue
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Cdata
	if !done {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanEndTag(tok *Token) {
	tok.Type = EndTag
	s.state = literalState
	s.read()
}

func (s *Scanner) scanClosingTag(tok *Token) {
	tok.Type = Invalid
	if s.char == question {
		tok.Type = ProcInstTag
	} else if s.char == slash {
		tok.Type = EmptyElemTag
	}
	s.read()
	if s.char != rangle {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *Scanner) scanValue(tok *Token) {
	s.read()
	for !s.done() && s.char != quote {
		s.write()
		s.read()
		if s.char == ampersand {
			s.char = s.scanEntity()
			if s.char == utf8.RuneError {
				break
			}
		}
	}
	tok.Type = Literal
	tok.Literal = s.str.String()
	if s.char != quote {
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()

}

func (s *Scanner) scanEntity() rune {
	s.read()
	var str bytes.Buffer
	for !s.done() && s.char != semicolon {
		str.WriteRune(s.char)
	}
	if s.char != semicolon {
		return utf8.RuneError
	}
	s.read()
	switch str.String() {
	case "lt":
		return langle
	case "gt":
		return rangle
	case "amp":
		return ampersand
	case "apos":
		return apos
	case "quot":
		return quote
	default:
		return utf8.RuneError
	}
}

func (s *Scanner) scanLiteral(tok *Token) {
	for !s.done() && s.char != langle {
		s.write()
		s.read()
		if s.char == ampersand {
			s.char = s.scanEntity()
			if s.char == utf8.RuneError {
				break
			}
		}
	}
	tok.Type = Literal
	tok.Literal = s.str.String()

	if s.char == langle {
		s.state = 0
	}
}

func (s *Scanner) scanName(tok *Token) {
	accept := func() bool {
		return unicode.IsLetter(s.char) || unicode.IsDigit(s.char) ||
			s.char == dash || s.char == underscore || s.char == dot
	}
	for !s.done() && accept() {
		s.write()
		s.read()
	}
	tok.Type = Name
	tok.Literal = s.str.String()
	if s.char == equal {
		tok.Type = Attr
		s.read()
	} else if s.char == colon {
		tok.Type = Namespace
		s.read()
	} else {
		s.skipBlank()
	}
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) read() {
	s.old = s.Position
	if s.char == '\n' {
		s.Column = 0
		s.Line++
	}
	s.Column++
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) skipBlank() {
	for !s.done() && unicode.IsSpace(s.char) {
		s.read()
	}
}

package xml

import (
	"errors"
	"io"
	"slices"
	"strings"
)

var (
	ErrClosed  = errors.New("closed")
	ErrBreak   = errors.New("break")
	ErrDiscard = errors.New("discard")
)

type (
	OnElementFunc func(*Reader, *Element, bool) error
	OnNodeFunc    func(*Reader, Node) error
)

type Reader struct {
	scan *Scanner
	curr Token
	peek Token

	openEls   map[QName]OnElementFunc
	closedEls map[QName]OnElementFunc
	nodes     map[NodeType]OnNodeFunc

	parent *Reader
}

func NewReader(r io.Reader) *Reader {
	rs := Reader{
		scan:      Scan(r),
		closedEls: make(map[QName]OnElementFunc),
		openEls:   make(map[QName]OnElementFunc),
		nodes:     make(map[NodeType]OnNodeFunc),
	}
	rs.next()
	rs.next()
	return &rs
}

func (r *Reader) Sub() *Reader {
	rs := Reader{
		scan:      r.scan,
		curr:      r.curr,
		peek:      r.peek,
		closedEls: make(map[QName]OnElementFunc),
		openEls:   make(map[QName]OnElementFunc),
		nodes:     make(map[NodeType]OnNodeFunc),
		parent:    r,
	}
	return &rs
}

func (r *Reader) OnNode(kind NodeType, fn OnNodeFunc) {
	r.nodes[kind] = fn
}

func (r *Reader) ClearNodeFunc(kind NodeType) {
	delete(r.nodes, kind)
}

func (r *Reader) OnElement(name QName, fn OnElementFunc) {
	r.OnElementOpen(name, fn)
	r.OnElementClosed(name, fn)
}

func (r *Reader) ClearElementFunc(name QName) {
	r.ClearElementOpenFunc(name)
	r.ClearElementClosedFunc(name)
}

func (r *Reader) OnElementOpen(name QName, fn OnElementFunc) {
	r.openEls[name] = fn
}

func (r *Reader) OnElementClosed(name QName, fn OnElementFunc) {
	r.closedEls[name] = fn
}

func (r *Reader) ClearElementOpenFunc(name QName) {
	delete(r.openEls, name)
}

func (r *Reader) ClearElementClosedFunc(name QName) {
	delete(r.closedEls, name)
}

func (r *Reader) Start() error {
	for {
		var closed bool
		node, err := r.Read()
		if closed = errors.Is(err, ErrClosed); !closed && err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err = r.emit(node, closed); err != nil {
			if errors.Is(err, ErrBreak) {
				break
			}
			if !closed && errors.Is(err, ErrDiscard) {
				err = r.discard(node)
			}
			return err
		}
	}
	return nil
}

func (r *Reader) discard(node Node) error {
	for {
		n, err := r.Read()
		if errors.Is(err, ErrClosed) {
			if n.QualifiedName() == node.QualifiedName() {
				break
			}
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) emit(node Node, closed bool) error {
	if fn, ok := r.getNodeFunc(node.Type()); ok {
		if err := fn(r, node); err != nil {
			return err
		}
	}
	switch n := node.(type) {
	case *Element:
		var (
			fn OnElementFunc
			ok bool
		)
		if closed {
			fn, ok = r.getClosedElementFunc(n.QName)
		} else {
			fn, ok = r.getOpenElementFunc(n.QName)
		}
		if ok {
			return fn(r, n, closed)
		}
	default:
		// pass
	}
	return nil
}

func (r *Reader) getOpenElementFunc(name QName) (OnElementFunc, bool) {
	fn, ok := r.openEls[name]
	if ok {
		return fn, ok
	}
	if r.parent != nil {
		return r.parent.getOpenElementFunc(name)
	}
	return nil, false
}

func (r *Reader) getClosedElementFunc(name QName) (OnElementFunc, bool) {
	fn, ok := r.closedEls[name]
	if ok {
		return fn, ok
	}
	if r.parent != nil {
		return r.parent.getClosedElementFunc(name)
	}
	return nil, false
}

func (r *Reader) getNodeFunc(kind NodeType) (OnNodeFunc, bool) {
	fn, ok := r.nodes[kind]
	if ok {
		return fn, ok
	}
	if r.parent != nil {
		return r.parent.getNodeFunc(kind)
	}
	return nil, false
}

func (r *Reader) Read() (Node, error) {
	if r.done() {
		return nil, io.EOF
	}
	switch {
	case r.is(ProcInstTag):
		return r.readPI()
	case r.is(OpenTag):
		return r.readStartElement()
	case r.is(CloseTag):
		return r.readEndElement()
	case r.is(CommentTag):
		return r.readComment()
	case r.is(Cdata):
		return r.readChardata()
	case r.is(Literal):
		node, err := r.readLiteral()
		if err != nil {
			return nil, err
		}
		txt, ok := node.(*Text)
		if ok && txt.Content == "" {
			return r.Read()
		}
		return node, nil
	default:
		return nil, r.createError("reader", "unexpected element type")
	}
}

func (r *Reader) readPI() (Node, error) {
	r.next()
	if !r.is(Name) {
		return nil, r.createError("processing instruction", "name is missing")
	}
	var elem Instruction
	elem.Name = r.curr.Literal
	r.next()
	var err error
	elem.Attrs, err = r.readAttributes(func() bool {
		return r.is(ProcInstTag)
	})
	if err != nil {
		return nil, err
	}
	if !r.is(ProcInstTag) {
		return nil, r.createError("processing instruction", "end of element expected")
	}
	r.next()
	return &elem, nil
}

func (r *Reader) readStartElement() (Node, error) {
	r.next()
	var (
		elem Element
		err  error
	)
	if r.is(Namespace) {
		elem.Space = r.curr.Literal
		r.next()
	}
	if !r.is(Name) {
		return nil, r.createError("element", "name is missing")
	}
	elem.Name = r.curr.Literal
	r.next()

	elem.Attrs, err = r.readAttributes(func() bool {
		return r.is(EndTag) || r.is(EmptyElemTag)
	})
	if err != nil {
		return nil, err
	}
	switch {
	case r.is(EmptyElemTag) || r.is(EndTag):
		if r.is(EmptyElemTag) {
			err = ErrClosed
		}
		r.next()
		return &elem, err
	default:
		return nil, r.createError("element", "end of element expected")
	}
}

func (r *Reader) readEndElement() (Node, error) {
	r.next()
	var elem Element
	if r.is(Namespace) {
		elem.Space = r.curr.Literal
		r.next()
	}
	if !r.is(Name) {
		return nil, r.createError("element", "name is missing")
	}
	elem.Name = r.curr.Literal
	r.next()
	if !r.is(EndTag) {
		return nil, r.createError("element", "end of element expected")
	}
	r.next()
	return &elem, ErrClosed
}

func (r *Reader) readAttributes(done func() bool) ([]Attribute, error) {
	var attrs []Attribute
	for !r.done() && !done() {
		attr, err := r.readAttr()
		if err != nil {
			return nil, err
		}
		ok := slices.ContainsFunc(attrs, func(a Attribute) bool {
			return attr.QualifiedName() == a.QualifiedName()
		})
		if ok {
			return nil, r.createError("attribute", "attribute is already defined")
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (r *Reader) readAttr() (Attribute, error) {
	var attr Attribute
	if r.is(Namespace) {
		attr.Space = r.curr.Literal
		r.next()
	}
	if !r.is(Attr) {
		return attr, r.createError("attribute", "name is expected")
	}
	attr.Name = r.curr.Literal
	r.next()
	if !r.is(Literal) {
		return attr, r.createError("attribute", "value is missing")
	}
	attr.Datum = r.curr.Literal
	r.next()
	return attr, nil
}

func (r *Reader) readComment() (Node, error) {
	defer r.next()
	node := Comment{
		Content: r.curr.Literal,
	}
	return &node, nil
}

func (r *Reader) readChardata() (Node, error) {
	defer r.next()
	char := CharData{
		Content: strings.TrimSpace(r.curr.Literal),
	}
	return &char, nil
}

func (r *Reader) readLiteral() (Node, error) {
	defer r.next()
	text := Text{
		Content: strings.TrimSpace(r.curr.Literal),
	}
	return &text, nil
}

func (r *Reader) done() bool {
	return r.is(EOF)
}

func (r *Reader) is(kind rune) bool {
	return r.curr.Type == kind
}

func (r *Reader) next() {
	r.curr = r.peek
	r.peek = r.scan.Scan()

	if r.parent != nil {
		r.parent.curr = r.curr
		r.parent.peek = r.peek
	}
}

func (r *Reader) createError(elem, msg string) error {
	return createParseError(elem, msg, r.curr.Position)
}

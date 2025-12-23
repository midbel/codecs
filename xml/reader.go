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

type OnElementFunc func(*Reader, *Element) error

type OnTextFunc func(*Reader, string) error

type OnNodeFunc func(*Reader, Node) error

type OnSet struct {
	onOpen  map[QName]OnElementFunc
	onClose map[QName]OnElementFunc
	onNode  map[NodeType]OnNodeFunc
	onText  OnTextFunc
}

type Reader struct {
	scan *Scanner
	curr Token
	peek Token

	stack []OnSet
}

func NewReader(r io.Reader) *Reader {
	rs := Reader{
		scan: Scan(r),
	}
	rs.Push()
	rs.next()
	rs.next()
	return &rs
}

func (r *Reader) Start() error {
	for {
		node, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil && !errors.Is(err, ErrClosed) {
			return err
		}
		closed := errors.Is(err, ErrClosed)

		if err := r.dispatch(node, closed); err != nil {
			if errors.Is(err, ErrBreak) {
				break
			}
			if errors.Is(err, ErrDiscard) && !closed {
				if err := r.discard(node); err != nil {
					return err
				}
				continue
			}
			return err
		}
	}
	return nil
}

func (r *Reader) OnText(fn OnTextFunc) {
	if i := len(r.stack) - 1; i >= 0 {
		r.stack[i].onText = fn
	}
}

func (r *Reader) OnOpen(name QName, fn OnElementFunc) {
	if i := len(r.stack) - 1; i >= 0 {
		r.stack[i].onOpen[name] = fn
	}
}

func (r *Reader) OnClose(name QName, fn OnElementFunc) {
	if i := len(r.stack) - 1; i >= 0 {
		r.stack[i].onClose[name] = fn
	}
}

func (r *Reader) OnNode(kind NodeType, fn OnNodeFunc) {
	if i := len(r.stack) - 1; i >= 0 {
		r.stack[i].onNode[kind] = fn
	}
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

func (r *Reader) Push() {
	s := OnSet{
		onOpen:  make(map[QName]OnElementFunc),
		onClose: make(map[QName]OnElementFunc),
		onNode:  make(map[NodeType]OnNodeFunc),
	}
	r.stack = append(r.stack, s)
}

func (r *Reader) Pop() {
	if i := len(r.stack); i > 1 {
		r.stack = r.stack[:i-1]
	}
}

func (r *Reader) dispatch(node Node, closed bool) error {
	if err := r.dispatchNode(node); err != nil {
		return err
	}
	var err error
	switch e := node.(type) {
	case *Element:
		if closed {
			err = r.dispatchClose(e)
		} else {
			err = r.dispatchOpen(e)
		}
	case *Instruction:
	case *Text:
		err = r.dispatchText(e.Content)
	case *CharData:
		err = r.dispatchText(e.Content)
	default:
	}
	return err
}

func (r *Reader) dispatchNode(node Node) error {
	for i := len(r.stack) - 1; i >= 0; i-- {
		fn, ok := r.stack[i].onNode[node.Type()]
		if ok {
			if err := fn(r, node); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (r *Reader) dispatchOpen(elem *Element) error {
	for i := len(r.stack) - 1; i >= 0; i-- {
		fn, ok := r.stack[i].onOpen[elem.QName]
		if ok {
			if err := fn(r, elem); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (r *Reader) dispatchClose(elem *Element) error {
	for i := len(r.stack) - 1; i >= 0; i-- {
		fn, ok := r.stack[i].onClose[elem.QName]
		if ok {
			if err := fn(r, elem); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (r *Reader) dispatchText(str string) error {
	for i := len(r.stack) - 1; i >= 0; i-- {
		fn := r.stack[i].onText
		if fn != nil {
			if err := fn(r, str); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (r *Reader) discard(node Node) error {
	if _, ok := node.(*Element); !ok {
		return nil
	}
	var depth int
	depth++
	for depth > 0 {
		n, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return io.ErrUnexpectedEOF
			}
			if errors.Is(err, ErrClosed) {
				_, ok := n.(*Element)
				if ok && n.QualifiedName() == node.QualifiedName() {
					depth--
				}
				continue
			}
			return err
		}
		if _, ok := n.(*Element); ok {
			depth++
		}
	}
	return nil
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
}

func (r *Reader) createError(elem, msg string) error {
	return createParseError(elem, msg, r.curr.Position)
}

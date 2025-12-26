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

func isClosed(err error) bool {
	return errors.Is(err, ErrClosed)
}

// Element
type E struct {
	QName
	Type       NodeType
	Attrs      []A
	SelfClosed bool
}

func (e E) GetAttribute(name string) A {
	i := slices.IndexFunc(e.Attrs, func(a A) bool {
		return a.Name == name
	})
	var a A
	if i >= 0 {
		a = e.Attrs[i]
	}
	return a
}

func (e E) GetAttributeValue(name string) string {
	a := e.GetAttribute(name)
	return a.Value
}

// Attribute
type A struct {
	QName
	Value string
}

// Text or CharData
type T struct {
	Content string
}

// Comment
type C struct {
	Content string
}

type OnElementFunc func(*Reader, E) error

type OnTextFunc func(*Reader, string) error

type OnSet struct {
	onOpen  map[QName]OnElementFunc
	onClose map[QName]OnElementFunc
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
		if err != nil && !isClosed(err) {
			return err
		}
		closed := isClosed(err)

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

func (r *Reader) Element(name QName, fn OnElementFunc) {
	r.OnOpen(name, func(rs *Reader, el E) error {
		rs.Push()
		return fn(rs, el)
	})
	r.OnClose(name, func(rs *Reader, _ E) error {
		rs.Pop()
		return nil
	})
}

func (r *Reader) Read() (any, error) {
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
		if node.Content == "" {
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
	}
	r.stack = append(r.stack, s)
}

func (r *Reader) Pop() {
	if i := len(r.stack); i > 1 {
		r.stack = r.stack[:i-1]
	}
}

func (r *Reader) dispatch(node any, closed bool) error {
	var err error
	switch e := node.(type) {
	case E:
		if closed {
			if e.SelfClosed {
				err = r.dispatchOpen(e)
				if err != nil {
					return err
				}
			}
			err = r.dispatchClose(e)
		} else {
			err = r.dispatchOpen(e)
		}
	case T:
		err = r.dispatchText(e.Content)
	case C:
		err = r.dispatchText(e.Content)
	default:
	}
	return err
}

func (r *Reader) dispatchOpen(elem E) error {
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

func (r *Reader) dispatchClose(elem E) error {
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

func (r *Reader) discard(node any) error {
	root, ok := node.(E)
	if !ok {
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
				a, ok := n.(E)
				if ok && a.QualifiedName() == root.QualifiedName() {
					depth--
				}
				continue
			}
			return err
		}
		if _, ok := n.(E); ok {
			depth++
		}
	}
	return nil
}

func (r *Reader) readPI() (E, error) {
	r.next()
	var elem E
	if !r.is(Name) {
		return elem, r.createError("processing instruction", "name is missing")
	}
	elem.Name = r.curr.Literal
	r.next()
	var err error
	elem.Attrs, err = r.readAttributes(func() bool {
		return r.is(ProcInstTag)
	})
	if err != nil {
		return elem, err
	}
	if !r.is(ProcInstTag) {
		return elem, r.createError("processing instruction", "end of element expected")
	}
	r.next()
	return elem, nil
}

func (r *Reader) readStartElement() (E, error) {
	r.next()
	var (
		elem E
		err  error
	)
	if r.is(Namespace) {
		elem.Space = r.curr.Literal
		r.next()
	}
	if !r.is(Name) {
		return elem, r.createError("element", "name is missing")
	}
	elem.Name = r.curr.Literal
	r.next()

	elem.Attrs, err = r.readAttributes(func() bool {
		return r.is(EndTag) || r.is(EmptyElemTag)
	})
	if err != nil {
		return elem, err
	}
	switch {
	case r.is(EmptyElemTag) || r.is(EndTag):
		if r.is(EmptyElemTag) {
			elem.SelfClosed = true
			err = ErrClosed
		}
		r.next()
		return elem, err
	default:
		return elem, r.createError("element", "end of element expected")
	}
}

func (r *Reader) readEndElement() (E, error) {
	r.next()
	var elem E
	if r.is(Namespace) {
		elem.Space = r.curr.Literal
		r.next()
	}
	if !r.is(Name) {
		return elem, r.createError("element", "name is missing")
	}
	elem.Name = r.curr.Literal
	r.next()
	if !r.is(EndTag) {
		return elem, r.createError("element", "end of element expected")
	}
	r.next()
	return elem, ErrClosed
}

func (r *Reader) readAttributes(done func() bool) ([]A, error) {
	var attrs []A
	for !r.done() && !done() {
		attr, err := r.readAttr()
		if err != nil {
			return nil, err
		}
		ok := slices.ContainsFunc(attrs, func(a A) bool {
			return attr.QualifiedName() == a.QualifiedName()
		})
		if ok {
			return nil, r.createError("attribute", "attribute is already defined")
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (r *Reader) readAttr() (A, error) {
	var attr A
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
	attr.Value = r.curr.Literal
	r.next()
	return attr, nil
}

func (r *Reader) readComment() (C, error) {
	defer r.next()
	node := C{
		Content: r.curr.Literal,
	}
	return node, nil
}

func (r *Reader) readChardata() (T, error) {
	defer r.next()
	char := T{
		Content: strings.TrimSpace(r.curr.Literal),
	}
	return char, nil
}

func (r *Reader) readLiteral() (T, error) {
	defer r.next()
	text := T{
		Content: strings.TrimSpace(r.curr.Literal),
	}
	return text, nil
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

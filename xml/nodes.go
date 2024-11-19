package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
)

var ErrElement = errors.New("element expected")

type Document struct {
	root     Node
	Version  string
	Encoding string

	Namespaces []string
}

func NewDocument(root Node) *Document {
	return &Document{
		root: root,
	}
}

func (d *Document) Write(w io.Writer) error {
	return NewWriter(w).Write(d)
}

func (d *Document) WriteString() (string, error) {
	var (
		buf bytes.Buffer
		err = d.Write(&buf)
	)
	return buf.String(), err
}

func (d *Document) GetElementById(id string) (Node, error) {
	if el, ok := d.root.(*Element); ok {
		return el.GetElementById(id)
	}
	return nil, nil
}

func (d *Document) GetElementsByTagName(tag string) ([]Node, error) {
	if el, ok := d.root.(*Element); ok {
		return el.GetElementsByTagName(tag)
	}
	return nil, nil
}

func (d *Document) Find(name string) (Node, error) {
	if el, ok := d.root.(*Element); ok {
		return el.Find(name), nil
	}
	return nil, ErrElement
}

func (d *Document) FindAll(name string) ([]Node, error) {
	if el, ok := d.root.(*Element); ok {
		return el.FindAll(name), nil
	}
	return nil, ErrElement
}

func (d *Document) Append(node Node) error {
	if el, ok := d.root.(*Element); ok {
		el.Append(node)
	}
	return ErrElement
}

func (d *Document) Insert(node Node, index int) error {
	if el, ok := d.root.(*Element); ok {
		el.Insert(node, index)
	}
	return ErrElement
}

func (d *Document) Map() (map[string]any, error) {
	if el, ok := d.root.(*Element); ok {
		return el.Map(), nil
	}
	return nil, ErrElement
}

func (d *Document) Root() Node {
	return d.root
}

type Node interface {
	LocalName() string
	QualifiedName() string
	Leaf() bool
	Position() int
	Parent() Node
	Value() string

	setParent(Node)
	setPosition(int)
}

type QName struct {
	Space string
	Name  string
}

func LocalName(name string) QName {
	return QualifiedName(name, "")
}

func QualifiedName(name, space string) QName {
	return QName{
		Name:  name,
		Space: space,
	}
}

func (q QName) LocalName() string {
	return q.Name
}

func (q QName) QualifiedName() string {
	if q.Space == "" {
		return q.LocalName()
	}
	return fmt.Sprintf("%s:%s", q.Space, q.Name)
}

func (q QName) isDocumentNode() bool {
	return q.Space == "" && q.Name == ""
}

type Attribute struct {
	QName
	Value string
}

func NewAttribute(name QName, value string) Attribute {
	return Attribute{
		QName: name,
		Value: value,
	}
}

type Element struct {
	QName
	Attrs []Attribute
	Nodes []Node

	parent   Node
	position int
}

func NewElement(name QName) *Element {
	return &Element{
		QName: name,
	}
}

func (e *Element) Map() map[string]any {
	values := make(map[string]any)
	if len(e.Attrs) > 0 {
		attrs := make(map[string]any)
		for i := range e.Attrs {
			attrs[e.Attrs[i].QualifiedName()] = e.Attrs[i].Value
		}
		values["attrs"] = attrs
	}

	for _, n := range e.Nodes {
		var val any
		switch n := n.(type) {
		case *Element:
			if !n.Leaf() {
				val = n.Map()
			} else {
				val = n.Value()
			}
		case *Text:
			val = n.Value()
		default:
			continue
		}
		if arr, ok := values[n.QualifiedName()]; ok {
			if x, ok := arr.([]any); ok {
				values[n.QualifiedName()] = append(x, val)
			} else {
				values[n.QualifiedName()] = append(x, arr, val)
			}
		} else {
			values[n.QualifiedName()] = val
		}
	}

	return values
}

func (e *Element) Root() bool {
	return e.parent == nil
}

func (e *Element) Leaf() bool {
	if len(e.Nodes) == 1 {
		_, ok := e.Nodes[0].(*Text)
		return ok
	}
	return len(e.Nodes) == 0
}

func (e *Element) Value() string {
	if len(e.Nodes) != 1 {
		return ""
	}
	el, ok := e.Nodes[0].(*Text)
	if !ok {
		return ""
	}
	return el.Content
}

func (e *Element) Has(name string) bool {
	return e.Find(name) != nil
}

func (e *Element) Find(name string) Node {
	ix := slices.IndexFunc(e.Nodes, func(n Node) bool {
		return n.LocalName() == name
	})
	if ix < 0 {
		return nil
	}
	return e.Nodes[ix]
}

func (e *Element) FindAll(name string) []Node {
	var nodes []Node
	for i := range e.Nodes {
		if e.Nodes[i].LocalName() != name {
			continue
		}
		nodes = append(nodes, e.Nodes[i])
	}
	return nodes
}

func (e *Element) GetElementById(id string) (Node, error) {
	for _, n := range e.Nodes {
		sub, ok := n.(*Element)
		if !ok {
			continue
		}
		x := slices.IndexFunc(sub.Attrs, func(a Attribute) bool {
			return a.Name == "id" && a.Value == id
		})
		if x >= 0 {
			return sub, nil
		}
		other, err := sub.GetElementById(id)
		if err == nil && other != nil {
			return other, nil
		}
	}
	return nil, fmt.Errorf("element with id not found")
}

func (e *Element) GetElementsByTagName(tag string) ([]Node, error) {
	var list []Node
	for _, n := range e.Nodes {
		sub, ok := n.(*Element)
		if !ok {
			continue
		}
		if sub.LocalName() == tag {
			list = append(list, sub)
		}
		if others, _ := e.GetElementsByTagName(tag); len(others) > 0 {
			list = append(list, others...)
		}
	}
	return list, nil
}

func (e *Element) Append(node Node) {
	node.setParent(e)
	node.setPosition(len(e.Nodes))
	e.Nodes = append(e.Nodes, node)
}

func (e *Element) Insert(node Node, index int) {
	if index < 0 || index > len(e.Nodes) {
		return
	}
	e.Nodes = slices.Insert(e.Nodes, index, node)
}

func (e *Element) Len() int {
	return len(e.Nodes)
}

func (e *Element) Clear() {
	for i := range e.Nodes {
		e.Nodes[i].setParent(nil)
	}
	e.Nodes = e.Nodes[:0]
}

func (e *Element) Position() int {
	return e.position
}

func (e *Element) Parent() Node {
	return e.parent
}

func (e *Element) setPosition(pos int) {
	e.position = pos
}

func (e *Element) setParent(parent Node) {
	e.parent = parent
}

func (e *Element) SetAttribute(attr Attribute) error {
	ix := slices.IndexFunc(e.Attrs, func(a Attribute) bool {
		return a.QualifiedName() == attr.QualifiedName()
	})
	if ix < 0 {
		e.Attrs = append(e.Attrs, attr)
	} else {
		e.Attrs[ix] = attr
	}
	return nil
}

type Instruction struct {
	QName
	Attrs []Attribute

	parent   Node
	position int
}

func NewInstruction(name QName) *Instruction {
	return &Instruction{
		QName: name,
	}
}

func (i *Instruction) Leaf() bool {
	return true
}

func (i *Instruction) Value() string {
	return ""
}

func (i *Instruction) SetAttribute(attr Attribute) error {
	ix := slices.IndexFunc(i.Attrs, func(a Attribute) bool {
		return a.QualifiedName() == attr.QualifiedName()
	})
	if ix < 0 {
		i.Attrs = append(i.Attrs, attr)
	} else {
		i.Attrs[ix] = attr
	}
	return nil
}

func (i *Instruction) Position() int {
	return i.position
}

func (i *Instruction) Parent() Node {
	return i.parent
}

func (i *Instruction) setPosition(pos int) {
	i.position = pos
}

func (i *Instruction) setParent(parent Node) {
	i.parent = parent
}

type CharData struct {
	Content string

	parent   Node
	position int
}

func NewCharacterData(chardata string) *CharData {
	return &CharData{
		Content: chardata,
	}
}

func (c *CharData) LocalName() string {
	return ""
}

func (c *CharData) QualifiedName() string {
	return ""
}

func (c *CharData) Leaf() bool {
	return true
}

func (c *CharData) Value() string {
	return c.Content
}

func (c *CharData) Position() int {
	return c.position
}

func (c *CharData) Parent() Node {
	return c.parent
}

func (c *CharData) setPosition(pos int) {
	c.position = pos
}

func (c *CharData) setParent(parent Node) {
	c.parent = parent
}

type Text struct {
	Content string

	parent   Node
	position int
}

func NewText(text string) *Text {
	return &Text{
		Content: text,
	}
}

func (t *Text) LocalName() string {
	return ""
}

func (t *Text) QualifiedName() string {
	return ""
}

func (t *Text) Leaf() bool {
	return true
}

func (t *Text) Value() string {
	return t.Content
}

func (t *Text) Position() int {
	return t.position
}

func (t *Text) Parent() Node {
	return t.parent
}

func (t *Text) setPosition(pos int) {
	t.position = pos
}

func (t *Text) setParent(parent Node) {
	t.parent = parent
}

type Comment struct {
	Content string

	parent   Node
	position int
}

func NewComment(comment string) *Comment {
	return &Comment{
		Content: comment,
	}
}

func (c *Comment) LocalName() string {
	return ""
}

func (c *Comment) QualifiedName() string {
	return ""
}

func (c *Comment) Leaf() bool {
	return true
}

func (c *Comment) Value() string {
	return c.Content
}

func (c *Comment) Position() int {
	return c.position
}

func (c *Comment) Parent() Node {
	return c.parent
}

func (c *Comment) setPosition(pos int) {
	c.position = pos
}

func (c *Comment) setParent(parent Node) {
	c.parent = parent
}

package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

type NodeType int8

const (
	TypeDocument NodeType = 1 << iota
	TypeElement
	TypeComment
	TypeAttribute
	TypeInstruction
	TypeText
	typeAll
)

const typeNode = TypeDocument | TypeElement | TypeComment | TypeAttribute | TypeInstruction

func isNode(n Node) bool {
	return n.Type()&typeNode > 0
}

type Cloner interface {
	Clone() Node
}

type Node interface {
	Type() NodeType
	LocalName() string
	QualifiedName() string
	Leaf() bool
	Position() int
	Parent() Node
	Value() string
	Identity() string

	setParent(Node)
	setPosition(int)
	path() []int
}

type NS struct {
	Prefix string
	Uri    string
}

func (n NS) Default() bool {
	return n.Prefix == ""
}

type BaseNode struct {
	Nodes    []Node
	parent   Node
	position int
}

func (n *BaseNode) setParent(node Node) {
	n.parent = node
}

func (n *BaseNode) setPosition(pos int) {
	n.position = pos
}

var ErrElement = errors.New("element expected")

type Document struct {
	Version  string
	Encoding string

	Nodes []Node
}

func NewDocument(root Node) *Document {
	doc := Document{
		Version:  SupportedVersion,
		Encoding: SupportedEncoding,
	}
	doc.Nodes = append(doc.Nodes, root)
	return &doc
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

func (d *Document) SetRootName(name string) {
	if name == "" {
		return
	}
	root := d.Root()
	if root == nil {
		return
	}
	if el, ok := root.(*Element); ok {
		el.Name = name
	}
}

func (d *Document) SetRootNamespace(name string) {
	if name == "" {
		return
	}
	root := d.Root()
	if root == nil {
		return
	}
	if el, ok := root.(*Element); ok {
		el.Space = name
	}
}

func (d *Document) GetNodesCount() int {
	root := d.Root()
	if root == nil {
		return 0
	}
	el, ok := root.(*Element)
	if ok {
		return len(el.Nodes)
	}
	return 0
}

func (d *Document) GetElementById(id string) (Node, error) {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		return el.GetElementById(id)
	}
	return nil, nil
}

func (d *Document) GetElementsByTagName(tag string) ([]Node, error) {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		return el.GetElementsByTagName(tag)
	}
	return nil, nil
}

func (d *Document) Query(expr string) (*Document, error) {
	if expr == "" {
		return d, nil
	}
	q, err := CompileString(expr)
	if err != nil {
		return nil, err
	}
	items, err := q.Find(d)
	if err != nil {
		return nil, err
	}
	root := NewElement(LocalName(d.Root().LocalName()))
	for i := range items {
		root.Append(items[i].Node())
	}
	return NewDocument(root), nil
}

func (d *Document) Find(name string) (Node, error) {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		return el.Find(name), nil
	}
	return nil, ErrElement
}

func (d *Document) FindAll(name string) ([]Node, error) {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		return el.FindAll(name), nil
	}
	return nil, ErrElement
}

func (d *Document) Append(node Node) error {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		el.Append(node)
	}
	return ErrElement
}

func (d *Document) Insert(node Node, index int) error {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		el.Insert(node, index)
	}
	return ErrElement
}

func (d *Document) Map() (map[string]any, error) {
	root := d.Root()
	if el, ok := root.(*Element); ok {
		return el.Map(), nil
	}
	return nil, ErrElement
}

func (d *Document) Root() Node {
	if len(d.Nodes) == 0 {
		return nil
	}
	root := d.Nodes[len(d.Nodes)-1]
	if root.Type() != TypeElement {
		return nil
	}
	return root
}

func (d *Document) Type() NodeType {
	return TypeDocument
}

func (d *Document) LocalName() string {
	return ""
}

func (d *Document) QualifiedName() string {
	return ""
}

func (d *Document) Leaf() bool {
	return false
}

func (d *Document) Position() int {
	return 0
}

func (d *Document) Parent() Node {
	return nil
}

func (d *Document) Value() string {
	return ""
}

func (d *Document) attach(node Node) {
	node.setParent(d)
	node.setPosition(len(d.Nodes))
	d.Nodes = append(d.Nodes, node)
}

func (_ *Document) Identity() string {
	return "document"
}

func (_ *Document) path() []int {
	return nil
}

func (d *Document) setParent(_ Node) {}

func (d *Document) setPosition(_ int) {}

type QName struct {
	Space string
	Name  string
}

func ParseName(name string) (QName, error) {
	var (
		qn QName
		ok bool
	)
	qn.Space, qn.Name, ok = strings.Cut(name, ":")
	if !ok {
		qn.Name, qn.Space = qn.Space, ""
	}
	if ok && qn.Space == "" {
		return qn, fmt.Errorf("invalid namespace")
	}
	return qn, nil
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

func (q QName) Zero() bool {
	return q.isDocumentNode()
}

func (q QName) Equal(other QName) bool {
	return q.Space == other.Space && q.Name == other.Name
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
	Datum string

	parent   Node
	position int
}

func NewAttribute(name QName, value string) Attribute {
	return Attribute{
		QName: name,
		Datum: value,
	}
}

func (_ *Attribute) Type() NodeType {
	return TypeAttribute
}

func (_ *Attribute) Leaf() bool {
	return true
}

func (a *Attribute) Position() int {
	return a.position
}

func (a *Attribute) Parent() Node {
	return a.parent
}

func (a *Attribute) Value() string {
	return a.Datum
}

func (a *Attribute) Identity() string {
	var list []string
	for _, p := range a.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("attr(%s)[%s]", a.QualifiedName(), strings.Join(list, "/"))
}

func (a *Attribute) path() []int {
	if a.parent == nil {
		return []int{a.position}
	}
	steps := a.parent.path()
	return append(steps, a.position)
}

func (a *Attribute) setParent(node Node) {
	a.parent = node
}

func (a *Attribute) setPosition(pos int) {
	a.position = pos
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

func (e *Element) Namespaces() []NS {
	var ns []NS
	for _, a := range e.Attrs {
		if a.Name == "xmlns" || a.Space == "xmlns" {
			n := NS{
				Prefix: a.Name,
				Uri:    a.Value(),
			}
			if n.Prefix == "xmlns" {
				n.Prefix = ""
			}
			ns = append(ns, n)
		}
	}
	return ns
}

func (e *Element) Clone() Node {
	c := &Element{
		QName:    e.QName,
		Attrs:    slices.Clone(e.Attrs),
		parent:   e.parent,
		position: e.position,
	}
	for i := range e.Nodes {
		if x, ok := e.Nodes[i].(Cloner); ok {
			if y := x.Clone(); y != nil {
				c.Append(y)
			}
		} else {
			c.Append(e.Nodes[i])
		}
	}
	return c
}

func (e *Element) RemoveNode(at int) error {
	if at < 0 || at >= len(e.Nodes) {
		return fmt.Errorf("%s: removing node with bad index (%d - %d)", e.QualifiedName(), at, len(e.Nodes))
	}
	e.Nodes = slices.Delete(e.Nodes, at, at+1)
	for i := range e.Nodes {
		e.Nodes[i].setPosition(i)
	}
	return nil
}

func (e *Element) ReplaceNode(at int, node Node) error {
	if at < 0 || at >= len(e.Nodes) {
		return fmt.Errorf("%s: replacing node with bad index (%d - %d)", e.QualifiedName(), at, len(e.Nodes))
	}
	node.setParent(e)
	node.setPosition(at)
	e.Nodes[at] = node
	return nil
}

func (e *Element) InsertNode(at int, node Node) error {
	return e.InsertNodes(at, []Node{node})
}

func (e *Element) InsertNodes(at int, nodes []Node) error {
	if at < 0 || at >= len(e.Nodes) {
		return fmt.Errorf("%s: inserting nodes with bad index (%d - %d)", e.QualifiedName(), at, len(e.Nodes))
	}
	var (
		before = e.Nodes[:at]
		after  = e.Nodes[at+1:]
	)
	e.Nodes = slices.Concat(before, nodes, after)
	for i := at + len(nodes); i < len(e.Nodes); i++ {
		e.Nodes[i].setPosition(i)
	}
	for i := range nodes {
		nodes[i].setParent(e)
	}
	return nil
}

func (_ *Element) Type() NodeType {
	return TypeElement
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
	return e.Empty()
}

func (e *Element) Empty() bool {
	return len(e.Nodes) == 0
}

func (e *Element) Value() string {
	var list []string
	for _, n := range e.Nodes {
		str := n.Value()
		list = append(list, str)
	}
	return strings.Join(list, " ")
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
			return a.Name == "id" && a.Value() == id
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

func (e *Element) Identity() string {
	var list []string
	for _, p := range e.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("node(%s)[%s]", e.QualifiedName(), strings.Join(list, "/"))
}

func (e *Element) path() []int {
	if e.parent == nil {
		return []int{e.position}
	}
	steps := e.parent.path()
	return append(steps, e.position)
}

func (e *Element) setPosition(pos int) {
	e.position = pos
}

func (e *Element) setParent(parent Node) {
	e.parent = parent
}

func (e *Element) RemoveAttribute(name QName) error {
	ix := slices.IndexFunc(e.Attrs, func(a Attribute) bool {
		return a.Qname == name
	})
	if ix < 0 {
		return nil
	}
	return e.RemoveAttr(ix)
}

func (e *Element) RemoveAttr(at int) error {
	if at < 0 || at >= len(e.Attrs) {
		return fmt.Errorf("bad index")
	}
	a := e.Attrs[at]
	a.setParent(nil)
	e.Attrs = slices.Delete(e.Attrs, at, at+1)
	for i := range e.Attrs {
		e.Attrs[i].setPosition(i)
	}
	return nil
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

func (_ *Instruction) Type() NodeType {
	return TypeInstruction
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

func (i *Instruction) Identity() string {
	var list []string
	for _, p := range i.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("instr(%s)[%s]", i.QualifiedName(), strings.Join(list, "/"))
}

func (i *Instruction) path() []int {
	if i.parent == nil {
		return []int{i.position}
	}
	steps := i.parent.path()
	return append(steps, i.position)
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

func (_ *CharData) Type() NodeType {
	return TypeText
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

func (c *CharData) Identity() string {
	var list []string
	for _, p := range c.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("%s[%s]", "chardata", strings.Join(list, "/"))
}

func (c *CharData) path() []int {
	if c.parent == nil {
		return []int{c.position}
	}
	steps := c.parent.path()
	return append(steps, c.position)
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

func (_ *Text) Type() NodeType {
	return TypeText
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

func (t *Text) Identity() string {
	var list []string
	for _, p := range t.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("%s[%s]", "text", strings.Join(list, "/"))
}

func (t *Text) path() []int {
	if t.parent == nil {
		return []int{t.position}
	}
	steps := t.parent.path()
	return append(steps, t.position)
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

func (_ *Comment) Type() NodeType {
	return TypeComment
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

func (c *Comment) Identity() string {
	var list []string
	for _, p := range c.path() {
		list = append(list, strconv.Itoa(p))
	}
	return fmt.Sprintf("%s[%s]", "comment", strings.Join(list, "/"))
}

func (c *Comment) path() []int {
	if c.parent == nil {
		return []int{c.position}
	}
	steps := c.parent.path()
	return append(steps, c.position)
}

func (c *Comment) setPosition(pos int) {
	c.position = pos
}

func (c *Comment) setParent(parent Node) {
	c.parent = parent
}

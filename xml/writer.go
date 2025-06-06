package xml

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/midbel/codecs/casing"
)

type WriterOptions uint64

const (
	OptionCompact WriterOptions = 1 << iota
	OptionNoNamespace
	OptionNoComment
	OptionNoProlog
	OptionCharDataToText
	OptionNameSnakeCase
	OptionNameKebabCase
	OptionNameLowerCase
	OptionNamespaceSnakeCase
	OptionNamespaceKebabCase
	OptionNamespaceLowerCase
)

func (w WriterOptions) Compact() bool {
	return w&OptionCompact > 0
}

func (w WriterOptions) NoNamespace() bool {
	return w&OptionNoNamespace > 0
}

func (w WriterOptions) NoComment() bool {
	return w&OptionNoComment > 0
}

func (w WriterOptions) NoProlog() bool {
	return w&OptionNoProlog > 0
}

func (w WriterOptions) CharDataToText() bool {
	return w&OptionCharDataToText > 0
}

func (w WriterOptions) NamespaceToSnakeCase() bool {
	return w&OptionNamespaceSnakeCase > 0
}

func (w WriterOptions) NameToSnakeCase() bool {
	return w&OptionNameSnakeCase > 0
}

func (w WriterOptions) NamespaceToKebabCase() bool {
	return w&OptionNamespaceKebabCase > 0
}

func (w WriterOptions) NameToKebabCase() bool {
	return w&OptionNameKebabCase > 0
}

func (w WriterOptions) NamespaceToLowerCase() bool {
	return w&OptionNamespaceLowerCase > 0
}

func (w WriterOptions) NameToLowerCase() bool {
	return w&OptionNameLowerCase > 0
}

func (w WriterOptions) rewriteQName(name QName) QName {
	if w.NameToKebabCase() {
		name.Name = casing.To(casing.KebabCase, name.Name)
	} else if w.NameToSnakeCase() {
		name.Name = casing.To(casing.SnakeCase, name.Name)
	} else if w.NameToLowerCase() {
		name.Name = strings.ToLower(name.Name)
	}
	if w.NamespaceToSnakeCase() {
		name.Space = casing.To(casing.SnakeCase, name.Space)
	} else if w.NamespaceToKebabCase() {
		name.Space = casing.To(casing.KebabCase, name.Space)
	} else if w.NamespaceToKebabCase() {
		name.Space = strings.ToLower(name.Space)
	}
	return name
}

type PrologWriterFunc func(w io.Writer) error

func (fn PrologWriterFunc) WriteProlog(w io.Writer) error {
	return fn(w)
}

type PrologWriter interface {
	WriteProlog(w io.Writer) error
}

type Writer struct {
	writer *bufio.Writer

	Indent   string
	Doctype  string
	MaxDepth int
	WriterOptions
	PrologWriter
}

func WriteNode(node Node) string {
	return writeNode(node, 0)
}

func WriteNodeDepth(node Node, depth int) string {
	return writeNode(node, depth)
}

func writeNode(node Node, maxdepth int) string {
	var buf bytes.Buffer

	ws := NewWriter(&buf)
	ws.MaxDepth = maxdepth
	ws.writeNode(node, 0)
	return buf.String()
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
		Indent: "  ",
	}
}

func (w *Writer) Write(doc *Document) error {
	if err := w.writeProlog(); err != nil {
		return err
	}
	w.writeNL()
	for _, n := range doc.Nodes {
		if err := w.writeNode(n, -1); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeNode(node Node, depth int) error {
	switch node := node.(type) {
	case *Document:
		return w.writeNode(node.Root(), depth)
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
	case *Attribute:
		return w.writeAttributeAsNode(node, depth+1)
	default:
		return fmt.Errorf("node: unknown type (%T)", node)
	}
}

func (w *Writer) writeElement(node *Element, depth int) error {
	w.writeNL()

	prefix := w.getIndent(depth)
	if prefix != "" {
		w.writer.WriteString(prefix)
	}
	w.writer.WriteRune(langle)
	name := w.rewriteQName(node.QName)
	if w.NoNamespace() {
		w.writer.WriteString(name.LocalName())
	} else {
		w.writer.WriteString(name.QualifiedName())
	}
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
	if w.MaxDepth == 0 || depth < w.MaxDepth {
		w.writer.WriteRune(rangle)
		for _, n := range node.Nodes {
			if err := w.writeNode(n, depth+1); err != nil {
				return err
			}
		}
	} else if node.Leaf() {
		w.writer.WriteRune(rangle)
		w.writeNode(node.Nodes[0], depth+1)
	} else {
		w.writer.WriteRune(slash)
		w.writer.WriteRune(rangle)
		return w.writer.Flush()
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

	if w.NoNamespace() {
		w.writer.WriteString(name.LocalName())
	} else {
		w.writer.WriteString(name.QualifiedName())
	}
	w.writer.WriteRune(rangle)
	return w.writer.Flush()
}

func (w *Writer) writeLiteral(node *Text, _ int) error {
	_, err := w.writer.WriteString(escapeText(node.Content))
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
	if w.NoComment() {
		return nil
	}
	w.writeNL()
	prefix := w.getIndent(depth)
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
	prefix := w.getIndent(depth)
	if prefix != "" {
		w.writer.WriteString(prefix)
	}
	w.writer.WriteRune(langle)
	w.writer.WriteRune(question)

	name := w.rewriteQName(node.QName)
	w.writer.WriteString(name.Name)
	if err := w.writeAttributes(node.Attrs, 0); err != nil {
		return err
	}
	w.writer.WriteRune(question)
	w.writer.WriteRune(rangle)
	return w.writer.Flush()
}

func (w *Writer) writeProlog() error {
	if w.NoProlog() {
		return nil
	}
	if w.PrologWriter == nil {
		prolog := NewInstruction(LocalName("xml"))
		prolog.Attrs = []Attribute{
			NewAttribute(LocalName("version"), SupportedVersion),
			NewAttribute(LocalName("encoding"), SupportedEncoding),
		}
		return w.writeInstruction(prolog, 0)
	} else {
		return w.WriteProlog(w.writer)
	}
}

func (w *Writer) writeAttributeAsNode(attr *Attribute, depth int) error {
	el := NewElement(attr.QName)
	el.Append(NewText(attr.Value()))
	return w.writeNode(el, depth)
}

func (w *Writer) writeAttributes(attrs []Attribute, depth int) error {
	prefix := w.getIndent(depth)
	for i, a := range attrs {
		if w.NoNamespace() && (a.Space == "xmlns" || a.Name == "xmlns") && a.Value() != "" {
			continue
		}
		if i == 0 || depth == 0 || w.Compact() {
			w.writer.WriteRune(' ')
		} else {
			w.writeNL()
			w.writer.WriteString(prefix)
		}
		name := w.rewriteQName(a.QName)
		if w.NoNamespace() {
			w.writer.WriteString(name.LocalName())
		} else {
			w.writer.WriteString(name.QualifiedName())
		}
		w.writer.WriteRune(equal)
		w.writer.WriteRune(quote)
		w.writer.WriteString(escapeText(a.Value()))
		w.writer.WriteRune(quote)
	}
	return nil
}

func (w *Writer) writeNL() {
	if w.Compact() {
		return
	}
	w.writer.WriteRune('\n')
}

func (w *Writer) getIndent(depth int) string {
	if w.Compact() {
		return ""
	}
	return strings.Repeat(w.Indent, depth)
}

func escapeText(str string) string {
	var buf bytes.Buffer
	for i := 0; i < len(str); {
		r, z := utf8.DecodeRuneInString(str[i:])
		i += z

		switch r {
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '&':
			buf.WriteString("&amp;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

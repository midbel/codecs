package xml

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type Writer struct {
	writer *bufio.Writer

	Compact     bool
	Indent      string
	NoProlog    bool
	NoNamespace bool
	NoComment   bool
}

func WriteNode(node Node) string {
	var buf bytes.Buffer

	ws := NewWriter(&buf)
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
	if w.NoNamespace {
		w.writer.WriteString(node.LocalName())
	} else {
		w.writer.WriteString(node.QualifiedName())
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
	if w.NoNamespace {
		w.writer.WriteString(node.LocalName())
	} else {
		w.writer.WriteString(node.QualifiedName())
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
	if w.NoComment {
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

func (w *Writer) writeAttributeAsNode(attr *Attribute, depth int) error {
	el := NewElement(attr.QName)
	el.Append(NewText(attr.Value()))
	return w.writeNode(el, depth)
}

func (w *Writer) writeAttributes(attrs []Attribute, depth int) error {
	prefix := w.getIndent(depth)
	for _, a := range attrs {
		if w.NoNamespace && (a.Space == "xmlns" || a.Name == "xmlns") && a.Value() != "" {
			continue
		}
		if depth == 0 || w.Compact {
			w.writer.WriteRune(' ')
		} else {
			w.writeNL()
			w.writer.WriteString(prefix)
		}
		if w.NoNamespace {
			w.writer.WriteString(a.LocalName())
		} else {
			w.writer.WriteString(a.QualifiedName())
		}
		w.writer.WriteRune(equal)
		w.writer.WriteRune(quote)
		w.writer.WriteString(escapeText(a.Value()))
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

func (w *Writer) getIndent(depth int) string {
	if w.Compact {
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

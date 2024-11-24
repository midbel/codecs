package json

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Writer struct {
	ws *bufio.Writer

	Indent  string
	Pretty  bool
	Compact bool

	level int
}

func NewWriter(w io.Writer) *Writer {
	ws := Writer{
		ws:     bufio.NewWriter(w),
		Indent: "  ",
	}
	return &ws
}

func (w *Writer) Write(value any) error {
	defer func() {
		w.reset()
		w.ws.Flush()
	}()
	return w.writeValue(value)
}

func (w *Writer) writeValue(value any) error {
	switch v := value.(type) {
	case map[string]any:
		return w.writeObject(v)
	case []any:
		return w.writeArray(v)
	default:
		return w.writeLiteral(value)
	}
}

func (w *Writer) writeObject(value map[string]any) error {
	w.enter()

	w.ws.WriteRune('{')
	w.writeNL()
	var i int
	for k, v := range value {
		if i > 0 {
			w.ws.WriteRune(',')
			w.writeNL()
		}
		w.writePrefix()
		if err := w.writeKey(k); err != nil {
			return err
		}
		if err := w.writeValue(v); err != nil {
			return err
		}
		i++
	}
	w.leave()
	w.writeNL()
	w.writePrefix()
	w.ws.WriteRune('}')
	return nil
}

func (w *Writer) writeArray(value []any) error {
	w.enter()

	w.ws.WriteRune('[')
	w.writeNL()
	for i := range value {
		if i > 0 {
			w.ws.WriteRune(',')
			w.writeNL()
		}
		w.writePrefix()
		if err := w.writeValue(value[i]); err != nil {
			return err
		}
	}
	w.leave()
	w.writeNL()
	w.writePrefix()
	w.ws.WriteRune(']')
	return nil
}

func (w *Writer) writeLiteral(value any) error {
	if value == nil {
		w.ws.WriteString("null")
		return nil
	}
	switch v := value.(type) {
	case bool:
		if v {
			w.ws.WriteString("true")
		} else {
			w.ws.WriteString("false")
		}
	case float64:
		w.ws.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case int64:
		w.ws.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 64))
	case string:
		w.writeString(v)
	default:
		return fmt.Errorf("unsupported json type")
	}
	return nil
}

func (w *Writer) writeKey(key string) error {
	w.writeString(key)
	w.ws.WriteRune(':')
	if !w.Compact {
		w.ws.WriteRune(' ')
	}
	return nil
}

func (w *Writer) writeString(value string) error {
	w.ws.WriteRune('"')
	w.ws.WriteString(value)
	w.ws.WriteRune('"')
	return nil
}

func (w *Writer) writePrefix() {
	if w.Compact || w.level == 0 {
		return
	}
	space := strings.Repeat(w.Indent, w.level)
	w.ws.WriteString(space)
}

func (w *Writer) writeNL() {
	if w.Compact {
		return
	}
	w.ws.WriteRune('\n')
}

func (w *Writer) enter() {
	w.level++
}

func (w *Writer) leave() {
	w.level--
}

func (w *Writer) reset() {
	w.level = 0
}

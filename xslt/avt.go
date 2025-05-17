package xslt

import (
	"iter"
	"strings"

	"github.com/midbel/codecs/xml"
)

func processAVT(ctx *Context, node xml.Node) error {
	el := node.(*xml.Element)
	for i, a := range el.Attrs {
		var (
			value = a.Value()
			str   strings.Builder
		)
		for q, ok := range iterAVT(value) {
			if !ok {
				str.WriteString(q)
				continue
			}
			items, err := ctx.ExecuteQuery(q, ctx.CurrentNode)
			if err != nil {
				return err
			}
			for i := range items {
				str.WriteString(toString(items[i]))
			}
		}
		el.Attrs[i].Datum = str.String()
	}
	return nil
}

func iterAVT(str string) iter.Seq2[string, bool] {
	fn := func(yield func(string, bool) bool) {
		var offset int
		for {
			var (
				ix  = strings.IndexRune(str[offset:], '{')
				ptr = offset
			)
			if ix < 0 {
				yield(str[offset:], false)
				break
			}
			offset += ix + 1
			ix = strings.IndexRune(str[offset:], '}')
			if ix < 0 {
				yield(str[offset-1:], false)
				break
			}
			if !yield(str[ptr:offset-1], false) {
				break
			}
			if !yield(str[offset:offset+ix], true) {
				break
			}
			offset += ix + 1
		}
	}
	return fn
}

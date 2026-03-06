package studio

import (
	"bytes"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type documentMsg struct {
	doc string
	err error
}

func parseDocument(file string) tea.Cmd {
	return func() tea.Msg {
		var (
			msg documentMsg
			doc *xml.Document
		)
		doc, msg.err = xml.ParseFile(file)
		if msg.err == nil {
			var str bytes.Buffer
			msg.err = xml.NewWriter(&str).Write(doc)
			msg.doc = strings.TrimSpace(str.String())
		}
		return msg
	}
}

type queryMsg struct {
	query string
}

type resultMsg struct {
	query  string
	result string
	count  int
	err    error
}

func executeQuery(file, query string) tea.Cmd {
	return func() tea.Msg {
		var (
			msg resultMsg
			doc *xml.Document
		)
		doc, msg.err = xml.ParseFile(file)
		if msg.err != nil {
			return msg
		}
		eval := xpath.NewEvaluator()
		expr, err := eval.Create(query)
		if err != nil {
			msg.err = err
			return msg
		}
		results, err := expr.Find(doc)
		if err != nil {
			msg.err = err
			return msg
		}
		var nodes []string
		for i := range results {
			s := xml.WriteNodeDepth(results[i].Node(), 10)
			nodes = append(nodes, s)
		}
		msg.query = query
		msg.result = strings.TrimSpace(strings.Join(nodes, "\n"))
		msg.count = len(nodes)
		return msg
	}
}

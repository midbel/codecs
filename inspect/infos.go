package inspect

import (
	"os"

	"github.com/midbel/codecs/xml"
)

type DocStats struct {
	MaxDepth   int
	Depth      map[int]int
	Namespaces map[xml.NS]struct{}

	Elements       map[string]int
	QualifiedEls   map[string]int
	Attributes     map[string]int
	QualifiedAttrs map[string]int
	Types          map[string]int
}

func Infos(file string) (*DocStats, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	stats := DocStats{
		Namespaces:     make(map[xml.NS]struct{}),
		Elements:       make(map[string]int),
		QualifiedEls:   make(map[string]int),
		Attributes:     make(map[string]int),
		QualifiedAttrs: make(map[string]int),
		Types:          make(map[string]int),
		Depth:          make(map[int]int),
	}

	var (
		rs    = xml.NewReader(r)
		depth int
	)
	rs.OnOpenAny(func(rs *xml.Reader, e xml.E) error {
		stats.Elements[e.LocalName()]++
		stats.QualifiedEls[e.QualifiedName()]++
		stats.Types[e.Type.String()]++
		for _, a := range e.Attrs {
			stats.Types[xml.TypeAttribute.String()]++
			if a.Space == "xmlns" {
				ns := xml.NS{
					Prefix: a.Name,
					Uri:    a.Value,
				}
				if _, ok := stats.Namespaces[ns]; !ok {
					stats.Namespaces[ns] = struct{}{}
				}
			} else {
				stats.Attributes[a.LocalName()]++
				stats.QualifiedAttrs[a.QualifiedName()]++
			}
		}
		depth++

		stats.Depth[depth]++

		return nil
	})
	rs.OnCloseAny(func(rs *xml.Reader, _ xml.E) error {
		stats.MaxDepth = max(depth, stats.MaxDepth)
		depth--
		return nil
	})
	rs.OnText(func(rs *xml.Reader, _ string) error {
		stats.Types[xml.TypeText.String()]++
		return nil
	})
	return &stats, rs.Start()
}

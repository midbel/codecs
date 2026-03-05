package inspect

import (
	"os"
	"strings"

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

	Paths map[string]int
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
		Paths:          make(map[string]int),
	}

	var (
		rs    = xml.NewReader(r)
		depth int
		path  []string
	)
	rs.OnOpenAny(func(rs *xml.Reader, e xml.E) error {
		stats.Elements[e.LocalName()]++
		stats.QualifiedEls[e.QualifiedName()]++
		stats.Types[e.Type.String()]++

		path = append(path, e.LocalName())

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
		stats.Paths["/"+strings.Join(path, "/")]++

		return nil
	})
	rs.OnCloseAny(func(rs *xml.Reader, _ xml.E) error {
		stats.MaxDepth = max(depth, stats.MaxDepth)
		depth--
		if n := len(path); n > 0 {
			path = path[:n-1]
		}
		return nil
	})
	rs.OnText(func(rs *xml.Reader, _ string) error {
		stats.Types[xml.TypeText.String()]++
		return nil
	})
	return &stats, rs.Start()
}

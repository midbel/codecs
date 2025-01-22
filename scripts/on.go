package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	rs := xml.NewReader(r)
	rs.OnElement(xml.LocalName("book"), onBook)
	rs.OnElement(xml.LocalName("author"), onAuthor)
	if err := rs.Parse(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

type Book struct {
	Title string
	Price float64
}

func discardClosed(fn xml.OnElementFunc) xml.OnElementFunc {
	return func(rs *xml.Reader, el *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		return fn(rs, el, closed)
	}
}

func onBook(rs *xml.Reader, el *xml.Element, closed bool) error {
	fmt.Println("onBook", el.Identity(), closed)
	rs.ClearElementFunc(xml.LocalName("title"))
	rs.ClearElementFunc(xml.LocalName("title"))
	return nil
}

func onAuthor(rs *xml.Reader, el *xml.Element, closed bool) error {
	fmt.Println("onAuthor", el.Identity(), closed)
	return nil
}

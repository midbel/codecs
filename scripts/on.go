package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

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
	if err := rs.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

type Book struct {
	Title string
	Price float64
}

func onBook(rs *xml.Reader, el *xml.Element, closed bool) error {
	var b Book
	if closed {
		fmt.Printf("onBook(done): %+v\n", b)
		return nil
	}
	fmt.Println("onBook", el.Identity(), closed)

	sub := rs.Sub()
	el1 := xml.LocalName("title")
	sub.OnElement(el1, func(rs *xml.Reader, _ *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		fmt.Println("element(book.title)")
		str, err := getValue(rs)
		if err == nil {
			b.Title = str
		}
		return err
	})
	el2 := xml.LocalName("price")
	sub.OnElement(el2, func(rs *xml.Reader, _ *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		fmt.Println("element(book.price)")
		str, err := getValue(rs)
		if err == nil {
			b.Title = str
		}
		b.Price, err = strconv.ParseFloat(str, 64)
		return err
	})

	return sub.Until(func(n xml.Node, err error) bool {
		return n.QualifiedName() == "book" && errors.Is(err, xml.ErrClosed)
	})
}

func onAuthor(rs *xml.Reader, el *xml.Element, closed bool) error {
	// fmt.Println("onAuthor", el.Identity(), closed)
	return nil
}

func discardClosed(fn xml.OnElementFunc) xml.OnElementFunc {
	return func(rs *xml.Reader, el *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		return fn(rs, el, closed)
	}
}

func getValue(rs *xml.Reader) (string, error) {
	text, err := rs.Read()
	if err != nil {
		return "", err
	}
	return text.Value(), nil
}

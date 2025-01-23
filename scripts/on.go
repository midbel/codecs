package main

import (
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

	var store Store

	rs := xml.NewReader(r)
	rs.OnElementOpen(xml.LocalName("book"), store.onBook)
	rs.OnElementOpen(xml.LocalName("author"), store.onAuthor)
	if err := rs.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(store.books)
}

type Book struct {
	Title string
	Price float64
}

type Store struct {
	books []Book
}

func (s *Store) onBook(rs *xml.Reader, el *xml.Element, closed bool) error {
	var b Book

	sub := rs.Sub()
	el1 := xml.LocalName("title")
	sub.OnElementOpen(el1, func(rs *xml.Reader, _ *xml.Element, closed bool) error {
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
	sub.OnElementOpen(el2, func(rs *xml.Reader, _ *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		fmt.Println("element(book.price)")
		str, err := getValue(rs)
		if err == nil {
			b.Price, err = strconv.ParseFloat(str, 64)
		}
		return err
	})

	bk := xml.LocalName("book")
	sub.OnElementClosed(bk, func(_ *xml.Reader, _ *xml.Element, closed bool) error {
		fmt.Printf("onBook(done): %+v\n", b)
		s.books = append(s.books, b)
		return nil
	})

	return sub.Start()
}

func (s *Store) onAuthor(rs *xml.Reader, el *xml.Element, closed bool) error {
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

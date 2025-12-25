package main

import (
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strconv"

	"github.com/midbel/codecs/xml"
)

type Cell struct {
	Type  string
	Index string

	ParsedValue any
	RawValue    string
	Formula     string
}

type Row struct {
	Line  int
	Cells []*Cell
}

type Sheet struct {
	Name      string
	Dimension string
	Rows      []*Row
}

type Reader struct {
	reader         *xml.Reader
	sheet          *Sheet
	sharedFormulas []string
	sharedStrings  []string
}

func New(r io.Reader, sharedStrings []string) *Reader {
	rs := Reader{
		reader:        xml.NewReader(r),
		sheet:         new(Sheet),
		sharedStrings: sharedStrings,
	}
	if n, ok := r.(interface{ Name() string }); ok {
		rs.sheet.Name = filepath.Base(n.Name())
	}
	return &rs
}

func (r *Reader) Each() iter.Seq[*Row] {
	return nil
}

func (r *Reader) Get() (*Sheet, error) {
	r.reader.OnClose(xml.LocalName("dimension"), r.onDimension)
	r.reader.Element(xml.LocalName("row"), r.onRow)
	r.reader.Element(xml.LocalName("c"), r.onCell)
	return r.sheet, r.reader.Start()
}

func (r *Reader) onCell(rs *xml.Reader, el *xml.Element) error {
	var (
		kind  = el.GetAttribute("t")
		index = el.GetAttribute("r")
		cell  = &Cell{
			Index: index.Value(),
			Type:  kind.Value(),
		}
	)

	rs.Element(xml.LocalName("v"), func(rs *xml.Reader, _ *xml.Element) error {
		rs.OnText(func(_ *xml.Reader, str string) error {
			cell.RawValue = str
			switch cell.Type {
			case "s":
				n, err := strconv.Atoi(str)
				if err != nil {
					return err
				}
				n--
				if n >= 0 && n < len(r.sharedStrings) {
					cell.ParsedValue = r.sharedStrings[n]
				}
			case "d":
				// date: pass
			case "str":
				// function: pass - handle in next attached handler
			case "b":
				b, err := strconv.ParseBool(str)
				if err != nil {
					return err
				}
				cell.ParsedValue = b
			default:
				n, err := strconv.ParseFloat(str, 64)
				if err != nil {
					return err
				}
				cell.ParsedValue = n
			}
			return nil
		})
		return nil
	})
	rs.Element(xml.LocalName("f"), func(rs *xml.Reader, el *xml.Element) error {
		var (
			shared = el.GetAttribute("t")
			index  = el.GetAttribute("si")
		)
		if shared.Value() == "shared" {
			ix, err := strconv.Atoi(index.Value())
			if err != nil {
				return err
			}
			if ix < len(r.sharedFormulas) {

			}
		}
		rs.OnText(func(_ *xml.Reader, str string) error {
			cell.Formula = str
			return nil
		})
		return nil
	})

	if i := len(r.sheet.Rows) - 1; i >= 0 {
		r.sheet.Rows[i].Cells = append(r.sheet.Rows[i].Cells, cell)
	}
	return nil
}

func (r *Reader) onRow(rs *xml.Reader, el *xml.Element) error {
	var (
		row  Row
		err  error
		attr = el.GetAttribute("r")
	)
	row.Line, err = strconv.Atoi(attr.Value())
	if err == nil {
		r.sheet.Rows = append(r.sheet.Rows, &row)
	}
	return err
}

func (r *Reader) onDimension(rs *xml.Reader, el *xml.Element) error {
	attr := el.GetAttribute("ref")
	r.sheet.Dimension = attr.Value()
	return nil
}

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	commons := []string{
		"project",
		"commits",
		"repository",
	}

	rs := New(r, commons)
	sheet, err := rs.Get()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(sheet.Name)
	fmt.Println(sheet.Dimension)
	for _, r := range sheet.Rows {
		fmt.Println("row", r.Line)
		for _, c := range r.Cells {
			fmt.Println(">> cell", c.Type, c.Index, c.RawValue, c.ParsedValue, c.Formula)
		}
	}
}

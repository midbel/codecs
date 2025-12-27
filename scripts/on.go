package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	Hidden string
	Line   int
	Cells  []*Cell
}

type Sheet struct {
	Name      string
	Dimension string
	Rows      []*Row
}

type Reader struct {
	reader         *xml.Reader
	sheet          *Sheet
	sharedFormulas map[int]string
	sharedStrings  []string
}

func New(r io.Reader, sharedStrings []string) *Reader {
	rs := Reader{
		reader:         xml.NewReader(r),
		sheet:          new(Sheet),
		sharedStrings:  sharedStrings,
		sharedFormulas: make(map[int]string),
	}
	if n, ok := r.(interface{ Name() string }); ok {
		rs.sheet.Name = filepath.Base(n.Name())
	}
	return &rs
}

func (r *Reader) Parse() (*Sheet, error) {
	r.reader.Element(xml.LocalName("dimension"), r.onDimension)
	r.reader.Element(xml.LocalName("row"), r.onRow)
	r.reader.Element(xml.LocalName("c"), r.onCell)
	return r.sheet, r.reader.Start()
}

func (r *Reader) parseCellValue(cell *Cell, str string) error {
	cell.RawValue = str
	switch cell.Type {
	case "s":
		n, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("invalid shared string index: %s", str)
		}
		if n < 0 || n >= len(r.sharedStrings) {
			return fmt.Errorf("shared string index out of bounds")
		}
		cell.ParsedValue = r.sharedStrings[n]
	case "d":
		// date: TBW
	case "str":
	case "b":
		b, err := strconv.ParseBool(str)
		if err != nil {
			return err
		}
		cell.ParsedValue = b
	default:
		n, err := strconv.ParseFloat(strings.TrimSpace(str), 64)
		if err != nil {
			return err
		}
		cell.ParsedValue = n
	}
	return nil
}

func (r *Reader) parseCellFormula(cell *Cell, el xml.E, rs *xml.Reader) error {
	var (
		shared = el.GetAttributeValue("t")
		index  = el.GetAttributeValue("si")
		id     int
	)
	if shared == "shared" {
		ix, err := strconv.Atoi(index)
		if err != nil {
			return err
		}
		id = ix
		cell.Formula = r.sharedFormulas[ix]
	}
	rs.OnText(func(_ *xml.Reader, str string) error {
		if shared == "shared" {
			r.sharedFormulas[id] = str
		}
		cell.Formula = str
		return nil
	})
	return nil
}

func (r *Reader) onCell(rs *xml.Reader, el xml.E) error {
	if len(r.sheet.Rows) == 0 {
		return fmt.Errorf("no row in worksheet")
	}

	var (
		kind  = el.GetAttributeValue("t")
		index = el.GetAttributeValue("r")
		pos   = len(r.sheet.Rows) - 1
		cell  = &Cell{
			Index: index,
			Type:  kind,
		}
	)
	r.sheet.Rows[pos].Cells = append(r.sheet.Rows[pos].Cells, cell)

	rs.Element(xml.LocalName("v"), func(rs *xml.Reader, _ xml.E) error {
		rs.OnText(func(_ *xml.Reader, str string) error {
			return r.parseCellValue(cell, str)
		})
		return nil
	})
	rs.Element(xml.LocalName("f"), func(rs *xml.Reader, el xml.E) error {
		return r.parseCellFormula(cell, el, rs)
	})
	return nil
}

func (r *Reader) onRow(rs *xml.Reader, el xml.E) error {
	var (
		row Row
		err error
	)
	row.Line, err = strconv.Atoi(el.GetAttributeValue("r"))
	if err == nil {
		r.sheet.Rows = append(r.sheet.Rows, &row)
	}
	return err
}

func (r *Reader) onDimension(rs *xml.Reader, el xml.E) error {
	r.sheet.Dimension = el.GetAttributeValue("ref")
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
		"dockit",
		"project",
		"commits",
		"repository",
		"sweet",
		"packit",
		"angle",
	}

	rs := New(r, commons)
	sheet, err := rs.Parse()
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
	sw, _ := xml.Stream(os.Stdout)
	sw.Open(xml.LocalName("test"), nil)
	sw.Open(xml.LocalName("foo"), []xml.A{
		{
			QName: xml.LocalName("id"),
			Value: "test",
		},
	})
	sw.Text("foobar")
	sw.Close(xml.LocalName("foo"))
	sw.Empty(xml.LocalName("bar"), nil)
	sw.Close(xml.LocalName("test"))
	sw.Flush()
}

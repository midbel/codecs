package main

import (
	"encoding/csv"
	"io"

	"github.com/midbel/codecs/sch"
)

type ReporterOptions struct {
	Quiet      bool
	FailFast   bool
	IgnoreZero bool
}

type ReporterStatus struct {
	File  string
	Count int
	Pass  int
}

func (r ReporterStatus) Update(res *sch.Result) {
	r.Count++
	if !res.Failed() {
		r.Pass++
	}
}

func (r ReporterStatus) ErrorCount() int {
	return r.Count - r.Pass
}

func (r ReporterStatus) Succeed() bool {
	return r.Count == r.Pass
}

type Reporter interface {
	Run(*sch.Schema, xml.Node) error
}

type HtmlReport struct {
	Dir string
}

func (r HtmlReport) Run(schema *sch.Schema, doc xml.Node) error {
	return nil
}

type XmlReport struct {
	Dir string
}

func (r XmlReport) Run(schema *sch.Schema, doc xml.Node) error {
	return nil
}

type FileReport struct {
	w io.Writer
}

func (r FileReport) Run(schema *sch.Schema, doc xml.Node) error {
	return nil
}

type CsvReport struct {
	ws *csv.Writer
}

func (r CsvReport) Run(schema *sch.Schema, doc xml.Node) error {
	return nil
}

package main

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/codecs/sch"
	"github.com/midbel/codecs/xml"
)

//go:embed templates/overview.html
var allTemplate string

//go:embed templates/detail.html
var oneTemplate string

//go:embed templates/index.html
var siteTemplate string

type ReportOptions struct {
	Timeout time.Duration
	Group   string
	Level   string

	RootSpace  string
	Quiet      bool
	FailFast   bool
	IgnoreZero bool
	ErrorOnly  bool
}

func (r ReportOptions) Keep() sch.FilterFunc {
	return keepAssert(r.Group, r.Level)
}

type ReportStatus struct {
	File  string
	Count int
	Pass  int
}

func (r *ReportStatus) Reset() {
	r.File = ""
	r.Count = 0
	r.Pass = 0
}

func (r *ReportStatus) SetFile(file string) {
	file = filepath.Base(file)
	r.File = strings.TrimSuffix(file, ".xml")
}

func (r *ReportStatus) Update(res sch.Result) {
	r.Count++
	if !res.Failed() {
		r.Pass++
	}
}

func (r *ReportStatus) ErrorCount() int {
	return r.Count - r.Pass
}

func (r ReportStatus) Succeed() bool {
	return r.Count == r.Pass
}

type Reporter interface {
	Run(*sch.Schema, string) error
}

type fileResult struct {
	File    string
	Status  ReportStatus
	Results []sch.Result
}

type htmlReport struct {
	ReportOptions
	status ReportStatus
	all    *template.Template
	one    *template.Template
	site   *template.Template
}

const reportTitle = "Execution Results"

var fnmap = template.FuncMap{
	"stringify": func(n xml.Node) string {
		return strings.TrimSpace(xml.WriteNode(n))
	},
	"increment": func(n int) int {
		return n + 1
	},
	"statusClass": func(stats ReportStatus) string {
		ratio := float64(stats.Pass) / float64(stats.Count)
		switch {
		case ratio < 0.4:
			return "fail"
		case ratio >= 0.4 && ratio < 0.6:
			return "warn"
		default:
			return ""
		}
	},
	"percent": func(stats ReportStatus) float64 {
		ratio := float64(stats.Pass) / float64(stats.Count)
		return ratio * 100.0
	},
}

func HtmlReport(opts ReportOptions) Reporter {
	var (
		all, _  = template.New("template").Funcs(fnmap).Parse(allTemplate)
		one, _  = template.New("template").Funcs(fnmap).Parse(oneTemplate)
		site, _ = template.New("template").Funcs(fnmap).Parse(siteTemplate)
	)
	return &htmlReport{
		ReportOptions: opts,
		all:           all,
		one:           one,
		site:          site,
	}
}

func (r *htmlReport) Exec(schema *sch.Schema, files []string) error {
	var (
		res []*fileResult
		ctx = context.Background()
	)
	for i := range files {
		r.status.Reset()
		fr, err := r.exec(ctx, schema, files[i])
		if err != nil {
			continue
		}
		res = append(res, fr)
	}
	if schema.Title == "" {
		schema.Title = reportTitle
	}
	return r.generateSite(filepath.Dir(files[0]), schema.Title, res)
}

func (r *htmlReport) Run(schema *sch.Schema, file string) error {
	ctx, _ := context.WithTimeout(context.Background(), r.Timeout)
	res, err := r.exec(ctx, schema, file)
	if err != nil {
		return err
	}
	return r.generateReport(filepath.Dir(file), res)
}

func (r *htmlReport) exec(ctx context.Context, schema *sch.Schema, file string) (*fileResult, error) {
	doc, err := parseDocument(file, r.RootSpace)
	if err != nil {
		return nil, err
	}
	r.status.SetFile(file)
	all := r.run(ctx, schema, doc)
	res := fileResult{
		File:    file,
		Results: all,
		Status:  r.status,
	}
	return &res, nil
}

func (r *htmlReport) run(ctx context.Context, schema *sch.Schema, doc *xml.Document) []sch.Result {
	var (
		it   = schema.ExecContext(ctx, doc, r.Keep())
		list []sch.Result
	)
	for res := range it {
		if err := ctx.Err(); err != nil {
			return nil
		}
		r.status.Update(res)
		list = append(list, res)
	}
	return list
}

func (r htmlReport) generateSite(dir, title string, files []*fileResult) error {
	tmp := filepath.Join(dir, "reports")
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return err
	}
	out := filepath.Join(tmp, "index.html")
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		Title string
		Files []*fileResult
	}{
		Title: title,
		Files: files,
	}

	if err := r.site.Execute(w, ctx); err != nil {
		return err
	}
	for i := range files {
		if err := r.generateReport(dir, files[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r htmlReport) generateReport(dir string, file *fileResult) error {
	tmp := filepath.Join(dir, "reports", file.Status.File)
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return err
	}
	if err := r.createOverviewReport(tmp, file); err != nil {
		return err
	}
	for _, res := range file.Results {
		if err := r.createDetailReport(tmp, res, file.Status.File); err != nil {
			return err
		}
	}
	return nil
}

func (r htmlReport) createDetailReport(dir string, res sch.Result, back string) error {
	out := filepath.Join(dir, filepath.Base(res.Ident+".html"))
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		Back string
		sch.Result
	}{
		Back:   back,
		Result: res,
	}
	return r.one.Execute(w, ctx)
}

func (r htmlReport) createOverviewReport(dir string, file *fileResult) error {
	out := filepath.Join(dir, "index.html")
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		File string
		List []sch.Result
	}{
		File: filepath.Clean(file.File),
		List: file.Results,
	}
	return r.all.Execute(w, ctx)
}

type fileReport struct {
	writer io.Writer
	status ReportStatus
	ReportOptions
}

func StdoutReport(opts ReportOptions) Reporter {
	if opts.Timeout == 0 {
		opts.Timeout = time.Second * 60
	}
	return fileReport{
		writer:        os.Stdout,
		ReportOptions: opts,
	}
}

func (r fileReport) Run(schema *sch.Schema, file string) error {
	ctx, _ := context.WithTimeout(context.Background(), r.Timeout)
	doc, err := parseDocument(file, r.RootSpace)
	if err != nil {
		return err
	}
	r.status.SetFile(file)
	for res := range schema.ExecContext(ctx, doc, r.Keep()) {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.status.Update(res)
		if res.Total == 0 && r.IgnoreZero {
			continue
		}
		if !r.Quiet {
			if r.ErrorOnly && !res.Failed() {
				continue
			}
			r.print(res)
		}
		if r.FailFast && res.Failed() {
			break
		}
	}
	fmt.Fprintf(r.writer, "%d assertions", r.status.Count)
	fmt.Fprintln(r.writer)
	fmt.Fprintf(r.writer, "%d pass", r.status.Pass)
	fmt.Fprintln(r.writer)
	fmt.Fprintf(r.writer, "%d failed", r.status.ErrorCount())
	fmt.Fprintln(r.writer)
	if !r.status.Succeed() {
		return fmt.Errorf("document is not valid")
	}
	return nil
}

const pattern = "%s | %-4d | %8s | %-32s | %3d/%-3d | %s"

func (r fileReport) print(res sch.Result) {
	var msg string
	if res.Failed() {
		msg = res.Error.Error()
		msg = shorten(msg, 48)
	} else {
		msg = "ok"
	}
	fmt.Fprint(r.writer, getColor(res))
	fmt.Fprintf(r.writer, pattern, r.status.File, r.status.Count, res.Level, res.Ident, res.Pass, res.Total, msg)
	fmt.Fprintln(r.writer, "\033[0m")
}

func getColor(res sch.Result) string {
	if !res.Failed() {
		return ""
	}
	switch res.Level {
	case sch.LevelWarn:
		return "\033[33m"
	case sch.LevelFatal:
		return "\033[31m"
	default:
		return ""
	}
}

func shorten(str string, maxLength int) string {
	z := len(str)
	if z <= maxLength {
		return str
	}
	x := strings.IndexRune(str[maxLength:], ' ')
	if x < 0 {
		return str
	}
	return str[:maxLength+x] + "..."
}

func keepAssert(group, level string) sch.FilterFunc {
	var groups []string
	if len(group) > 0 {
		groups = strings.Split(group, "-")
	}

	keep := func(a *sch.Assert) bool {
		if len(groups) == 0 {
			return true
		}
		parts := strings.Split(a.Ident, "-")
		if len(parts) < len(groups) {
			return false
		}
		for i := range groups {
			if parts[i] != groups[i] {
				return false
			}
		}
		if level != "" && level != a.Flag {
			return false
		}
		return true
	}
	return keep
}

func parseDocument(file, rootns string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	doc, err := xml.NewParser(r).Parse()
	if err == nil {
		doc.SetRootNamespace(rootns)
	}
	return doc, err
}

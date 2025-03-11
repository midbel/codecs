package sch

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/midbel/codecs/xml"
)

//go:embed templates/*
var reportsTemplate embed.FS

type ReportOptions struct {
	Timeout time.Duration

	Format string

	RootSpace  string
	Quiet      bool
	FailFast   bool
	IgnoreZero bool
	ErrorOnly  bool
	ReportDir  string
}

type ReportStatus struct {
	File  string
	Count int
	Pass  int
}

func (r *ReportStatus) Clone() ReportStatus {
	c := *r
	return c
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

func (r *ReportStatus) Update(res Result) {
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
	Run(*Schema, []string) error
}

type fileResult struct {
	File    string
	LastMod time.Time
	Results []Result

	Building bool
	Error    error
}

func (r fileResult) Status() ReportStatus {
	var rs ReportStatus
	rs.SetFile(r.File)
	for i := range r.Results {
		rs.Update(r.Results[i])
	}
	return rs
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
		if stats.Pass == 0 || stats.Count == 0 {
			return 0
		}
		ratio := float64(stats.Pass) / float64(stats.Count)
		return ratio * 100.0
	},
	"percent2": func(curr, total int) float64 {
		if curr == 0 || total == 0 {
			return 0
		}
		ratio := float64(curr) / float64(total)
		return ratio * 100.0
	},
	"datetimefmt": func(w time.Time) string {
		return w.Format("2006-01-02 15:05:04")
	},
}

type htmlReport struct {
	ReportOptions
	site   *template.Template
	static bool
}

func HtmlReport(opts ReportOptions) (Reporter, error) {
	return staticHtmlReport(opts)
}

func staticHtmlReport(opts ReportOptions) (*htmlReport, error) {
	site, err := template.New("angle").Funcs(fnmap).ParseFS(reportsTemplate, "templates/*")
	if err != nil {
		return nil, err
	}
	report := htmlReport{
		ReportOptions: opts,
		site:          site,
		static:        true,
	}
	if opts.ReportDir == "" {
		opts.ReportDir = filepath.Join(os.TempDir(), "angle", "reports")
		if err := os.RemoveAll(opts.ReportDir); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(opts.ReportDir, 0755); err != nil {
			return nil, err
		}
	}
	return &report, nil
}

func (r *htmlReport) Run(schema *Schema, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files given")
	}
	if schema.Title == "" {
		schema.Title = reportTitle
	}
	list := make([]*fileResult, len(files))
	for i := range files {
		list[i] = &fileResult{
			File:     files[i],
			LastMod:  time.Now(),
			Building: true,
		}
	}
	if err := r.generateIndex(schema.Title, list); err != nil {
		return err
	}
	for i := range list {
		now := time.Now()
		list[i].Error = r.exec(schema, list[i])
		list[i].Building = false

		if err := r.generateIndex(schema.Title, list); err != nil {
			return err
		}
		if !r.Quiet {
			fmt.Fprintf(os.Stdout, "done processing %s (%s)", list[i].File, time.Since(now))
			fmt.Fprintln(os.Stdout)
		}
	}
	return nil
}

func (r *htmlReport) exec(schema *Schema, file *fileResult) error {
	doc, err := parseDocument(file.File, r.RootSpace)
	if err != nil {
		return err
	}
	res := schema.Exec(doc)
	file.LastMod = time.Now()
	file.Results = slices.Collect(res)

	return r.generateReport(file)
}

func (r htmlReport) generateIndex(title string, files []*fileResult) error {
	if err := os.MkdirAll(r.ReportDir, 0755); err != nil {
		return err
	}
	out := filepath.Join(r.ReportDir, "index.html")
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		Static bool
		Title  string
		Count  int
		Total  int
		Files  []*fileResult
	}{
		Static: r.static,
		Title:  title,
		Total:  len(files),
		Files:  files,
	}
	return r.site.ExecuteTemplate(w, "index.html", ctx)
}

func (r htmlReport) generateReport(file *fileResult) error {
	var (
		status = file.Status()
		tmp    = filepath.Join(r.ReportDir, status.File)
	)
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return err
	}
	if err := r.createOverviewReport(tmp, file); err != nil {
		return err
	}
	for _, res := range file.Results {
		if err := r.createDetailReport(tmp, res, status.File); err != nil {
			return err
		}
	}
	return nil
}

func (r htmlReport) createDetailReport(dir string, res Result, back string) error {
	out := filepath.Join(dir, filepath.Base(res.Ident+".html"))
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		Back string
		Result
	}{
		Back:   back,
		Result: res,
	}
	return r.site.ExecuteTemplate(w, "detail.html", ctx)
}

func (r htmlReport) createOverviewReport(dir string, file *fileResult) error {
	out := filepath.Join(dir, "index.html")
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx := struct {
		File   string
		Result *fileResult
		List   []Result
		Static bool
	}{
		File:   filepath.Clean(file.File),
		Result: file,
		List:   file.Results,
		Static: r.static,
	}
	return r.site.ExecuteTemplate(w, "overview.html", ctx)
}

const pattern = "%s | %-4d | %8s | %-32s | %3d/%-3d | %s"

type fileReport struct {
	writer io.Writer
	status ReportStatus
	tpl    *template.Template
	ReportOptions
}

func StdoutReport(opts ReportOptions) (Reporter, error) {
	if opts.Timeout == 0 {
		opts.Timeout = time.Second * 60
	}
	report := fileReport{
		writer:        os.Stdout,
		ReportOptions: opts,
	}

	if opts.Format != "" {
		tpl, err := template.New("line").Parse(strings.TrimSpace(opts.Format) + "\n")
		if err != nil {
			return nil, err
		}
		report.tpl = tpl
	}
	return report, nil
}

func (r fileReport) Run(schema *Schema, files []string) error {
	return nil
}

func (r fileReport) print(res Result) {
	if r.tpl != nil {
		r.printFormat(res)
		return
	}
	var msg string
	if res.Failed() {
		msg = res.Error.Error()
		msg = shorten(msg, 128)
	} else {
		msg = "ok"
	}
	fmt.Fprint(r.writer, getColor(res))
	fmt.Fprintf(r.writer, pattern, r.status.File, r.status.Count, res.Level, res.Ident, res.Pass, res.Total, msg)
	fmt.Fprintln(r.writer, "\033[0m")
}

func (r fileReport) printFormat(res Result) {
	ctx := struct {
		ID    string
		File  string
		Level string
		Total int
		Pass  int
	}{
		ID:    res.Ident,
		File:  r.status.File,
		Level: res.Level,
		Total: res.Total,
		Pass:  res.Pass,
	}
	r.tpl.Execute(r.writer, ctx)
}

func getColor(res Result) string {
	if !res.Failed() {
		return ""
	}
	switch res.Level {
	case LevelWarn:
		return "\033[33m"
	case LevelFatal:
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

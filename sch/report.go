package sch

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/codecs/xml"
)

//go:embed templates/*
var reportsTemplate embed.FS

type ReportOptions struct {
	Timeout time.Duration
	Group   string
	Level   string

	Format string

	RootSpace  string
	Quiet      bool
	FailFast   bool
	IgnoreZero bool
	ErrorOnly  bool
	ReportDir  string
	ListenAddr string
}

func (r ReportOptions) Keep() FilterFunc {
	return keepAssert(r.Group, r.Level)
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
	Run(*Schema, string) error
}

type fileResult struct {
	File    string
	LastMod time.Time
	Status  ReportStatus
	Results []Result

	Building bool
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
	status ReportStatus
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

func (r *htmlReport) Run(schema *Schema, file string) error {
	return nil
}

func (r htmlReport) generateSite(title string, files []*fileResult) error {
	if err := r.generateIndex(title, files); err != nil {
		return err
	}
	for i := range files {
		if err := r.generateReport(files[i]); err != nil {
			return err
		}
	}
	return nil
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
	for i := range files {
		if !files[i].Building {
			ctx.Count++
		}
	}
	return r.site.ExecuteTemplate(w, "index.html", ctx)
}

func (r htmlReport) generateReport(file *fileResult) error {
	tmp := filepath.Join(r.ReportDir, file.Status.File)
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

func (r fileReport) Run(schema *Schema, file string) error {
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

func keepAssert(group, level string) FilterFunc {
	var groups []string
	if len(group) > 0 {
		groups = strings.Split(group, "-")
	}

	keep := func(a *Assert) bool {
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

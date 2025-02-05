package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/codecs/sch"
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
	site   *template.Template

	serv *http.Server
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
	"percent2": func(curr, total int) float64 {
		ratio := float64(curr) / float64(total)
		return ratio * 100.0
	},
}

type reportHandler struct {
	reportDir string
	tpl       *template.Template
	site      http.Handler

	total int
	count int
	file  string
}

func handleReport(reportDir string, tpl *template.Template) http.Handler {
	h := reportHandler{
		tpl:       tpl,
		reportDir: reportDir,
		site:      http.FileServer(http.Dir(reportDir)),
	}
	return &h
}

func (h *reportHandler) SetTotal(total int) {
	h.total = total
}

func (h *reportHandler) SetFile(file string) {
	h.file = file
	h.count++
}

func (h *reportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(filepath.Join(h.reportDir, "index.html")); err != nil {
		w.Header().Set("refresh", "5")
		ctx := struct {
			Total int
			Count int
			File  string
		}{
			Total: h.total,
			Count: h.count,
			File:  filepath.Base(h.file),
		}
		h.tpl.ExecuteTemplate(w, "building.html", ctx)
		return
	}
	h.site.ServeHTTP(w, r)
}

func HtmlReport(opts ReportOptions) (Reporter, error) {
	site, err := template.New("angle").Funcs(fnmap).ParseFS(reportsTemplate, "templates/*.html")
	if err != nil {
		return nil, err
	}
	report := htmlReport{
		ReportOptions: opts,
		site:          site,
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
	if opts.ListenAddr != "" {
		report.serv = &http.Server{
			Addr:         opts.ListenAddr,
			Handler:      handleReport(opts.ReportDir, report.site),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		go report.serv.ListenAndServe()
	}
	return &report, nil
}

func (r *htmlReport) Exec(schema *sch.Schema, files []string) error {
	var (
		res         []*fileResult
		ctx, cancel = context.WithCancel(context.Background())
	)
	if r.serv != nil {
		if r, ok := r.serv.Handler.(interface{ SetTotal(int) }); ok {
			r.SetTotal(len(files))
		}
	}
	sig := make(chan os.Signal, 1)
	if r.serv != nil {
		signal.Notify(sig, os.Interrupt, os.Kill)
	}
	for i := range files {
		select {
		case s := <-sig:
			signal.Reset(s)
			cancel()
		default:
			// pass
		}
		if err := ctx.Err(); err != nil {
			break
		}
		r.status.Reset()
		if r.serv != nil {
			if r, ok := r.serv.Handler.(interface{ SetFile(string) }); ok {
				r.SetFile(files[i])
			}
		}
		fr, err := r.exec(ctx, schema, files[i])
		if err != nil {
			continue
		}
		res = append(res, fr)
	}
	if schema.Title == "" {
		schema.Title = reportTitle
	}
	r.generateSite(filepath.Dir(files[0]), schema.Title, res)

	if r.serv != nil {
		if err := ctx.Err(); err == nil {
			signal.Reset(<-sig)
		}
		return r.serv.Shutdown(ctx)
	}
	return nil
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
		Checkable bool
	}{
		Title: title,
		Files: files,
		Checkable: r.serv != nil,
	}

	if err := r.site.ExecuteTemplate(w, "index.html", ctx); err != nil {
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
		File string
		List []sch.Result
		Checkable bool
	}{
		File: filepath.Clean(file.File),
		List: file.Results,
		Checkable: r.serv != nil,
	}
	return r.site.ExecuteTemplate(w, "overview.html", ctx)
}

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
	if r.tpl == nil {
		fmt.Fprintf(r.writer, "%d assertions", r.status.Count)
		fmt.Fprintln(r.writer)
		fmt.Fprintf(r.writer, "%d pass", r.status.Pass)
		fmt.Fprintln(r.writer)
		fmt.Fprintf(r.writer, "%d failed", r.status.ErrorCount())
		fmt.Fprintln(r.writer)
	}
	if !r.status.Succeed() {
		return fmt.Errorf("document is not valid")
	}
	return nil
}

const pattern = "%s | %-4d | %8s | %-32s | %3d/%-3d | %s"

func (r fileReport) print(res sch.Result) {
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

func (r fileReport) printFormat(res sch.Result) {
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

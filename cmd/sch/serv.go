package main

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/midbel/codecs/sch"
)

type Server interface {
	io.Closer
	ListenAndServe() error
}

type serverReporter struct {
	*http.Server

	files   []string
	results []*fileResult
	report  *htmlReport
	schema  *sch.Schema

	ReportOptions
}

func Serve(schema *sch.Schema, files []string, opts ReportOptions) (Server, error) {
	rp, err := HtmlReport(opts)
	if err != nil {
		return nil, err
	}
	report, _ := rp.(*htmlReport)

	sh := serverReporter{
		report:        report,
		schema:        schema,
		files:         files,
		ReportOptions: opts,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(opts.ReportDir)))
	mux.HandleFunc("POST /process/", sh.processFile)
	mux.HandleFunc("POST /upload/", sh.uploadFile)

	sh.Server = &http.Server{
		Addr:         opts.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go sh.execute()

	return &sh, nil
}

func (r *serverReporter) execute() error {
	ctx := context.Background()
	for i := range r.files {
		res, _ := r.report.exec(ctx, r.schema, r.files[i])
		r.results = append(r.results, res)

		r.report.generateReport(res)
	}
	return nil
}

func (h *serverReporter) uploadFile(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(503)
}

func (h *serverReporter) processFile(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(503)
}

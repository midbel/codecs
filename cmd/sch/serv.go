package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
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
	mux.HandleFunc("POST /process/{file}", sh.processFile)
	mux.HandleFunc("POST /upload", sh.uploadFile)

	sh.Server = &http.Server{
		Addr:         opts.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go sh.run()

	return &sh, nil
}

func (s *serverReporter) run() error {
	now := time.Now()
	for i := range s.files {
		res := fileResult{
			File: s.files[i],
			LastMod: now,
			Building: true,
		}
		res.Status.SetFile(s.files[i])
		s.results = append(s.results, &res)
	}
	s.report.generateIndex(s.schema.Title, s.results)

	ctx := context.Background()
	for i := range s.files {
		if err := s.execute(ctx, s.files[i]); err != nil {
			continue
		}
		s.report.generateIndex(s.schema.Title, s.results)
	}
	return nil
}

func (s *serverReporter) execute(ctx context.Context, file string) error {
	ix := slices.IndexFunc(s.results, func(other *fileResult) bool {
		return other.File == file
	})
	if ix >= 0 {
		res := s.results[ix]
		res.Building = true
		s.report.generateReport(res)
	}	
	res, err := s.report.exec(ctx, s.schema, file)
	if err != nil {
		return err
	}

	res.Building = false
	if ix < 0 {
		s.results = append(s.results, res)
	} else {
		s.results[ix] = res
	}
	s.report.generateReport(res)
	return nil
}

func (s *serverReporter) executeFile(file string) error {
	ix := slices.IndexFunc(s.files, func(other string) bool {
		return filepath.Base(other) == file
	})
	if ix < 0 {
		return fmt.Errorf("file does not exist")
	}
	go s.execute(context.TODO(), s.files[ix])
	return nil
}

func (s *serverReporter) processFile(w http.ResponseWriter, r *http.Request) {
	if err := s.executeFile(r.PathValue("file")); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *serverReporter) uploadFile(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
}

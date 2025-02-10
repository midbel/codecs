package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/gorilla/websocket"
	"github.com/midbel/codecs/sch"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

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

	events  chan *fileResult
	running chan struct{}

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
		events:        make(chan *fileResult),
		running:       make(chan struct{}, 10),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(opts.ReportDir)))
	mux.HandleFunc("POST /process/{file}", sh.processFile)
	mux.HandleFunc("POST /upload", sh.uploadFile)
	mux.HandleFunc("GET /ws", sh.ws)
	mux.HandleFunc("GET /status", sh.statusFiles)

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
			File:     s.files[i],
			LastMod:  now,
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
		res.Results = res.Results[:0]
		s.report.generateReport(res)
		s.sendResult(res)
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
	s.sendResult(res)
	return nil
}

func (s *serverReporter) sendResult(res *fileResult) {
	select {
	case s.events <- res:
	default:
	}
}

func (s *serverReporter) executeFile(file string) error {
	ix := slices.IndexFunc(s.files, func(other string) bool {
		return filepath.Base(other) == file+".xml"
	})
	if ix < 0 {
		return fmt.Errorf("file does not exist")
	}
	select {
	case s.running <- struct{}{}:
		go func() {
			s.execute(context.TODO(), s.files[ix])
			<-s.running
		}()
	default:
		return fmt.Errorf("too many jobs already running")
	}
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

func (s *serverReporter) statusFiles(w http.ResponseWriter, r *http.Request) {
	if len(s.results) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	data := s.getStatus(time.Now())
	json.NewEncoder(w).Encode(data)
}

func (s *serverReporter) ws(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	go s.handleConn(ws)
}

func (s *serverReporter) handleConn(ws *websocket.Conn) {
	defer ws.Close()
	t := time.NewTicker(time.Second)
	for w := range t.C {
		data := s.getStatus(w)
		ws.WriteJSON(data)
	}
}

func (s *serverReporter) getStatus(w time.Time) any {
	data := struct {
		When    time.Time     `json:"when"`
		Files   []string      `json:"files"`
		Results []*fileResult `json:"results"`
	}{
		When:    w,
		Files:   s.files,
		Results: s.results,
	}
	return data
}

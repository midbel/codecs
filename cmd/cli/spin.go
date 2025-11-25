package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Spinner struct {
	frames  []string
	message string

	mu      sync.Mutex
	running bool

	stop   sync.Once
	ticker *time.Ticker
	done   chan struct{}
}

func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		ticker: time.NewTicker(time.Millisecond * 90),
		done:   make(chan struct{}),
	}
}

func (s *Spinner) SetMessage(msg string) {
	msg = strings.TrimSpace(msg)
	msg = strings.TrimRight(msg, ".")
	s.message = msg
}

func (s *Spinner) Run(fn func()) {
	s.Start()
	defer s.Stop()
	fn()
}

func (s *Spinner) Stop() {
	s.stop.Do(func() {
		close(s.done)
		s.ticker.Stop()
		clearLine()
	})
}

func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	go s.run()
}

func (s *Spinner) run() {
	for i := 0; ; i++ {
		select {
		case <-s.ticker.C:
			f := s.frames[i%len(s.frames)]
			io.WriteString(os.Stdout, fmt.Sprintf("\r%s", f))
			if s.message != "" && i == 0 {
				io.WriteString(os.Stdout, " ")
				io.WriteString(os.Stdout, s.message)
				io.WriteString(os.Stdout, "...")
			}
		case <-s.done:
			return
		}
	}
}

func clearLine() {
	io.WriteString(os.Stdout, "\x1b[0G\x1b[2K\x1b[0G")
}

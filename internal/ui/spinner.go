package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner renders a lightweight terminal spinner for long-running operations.
type Spinner struct {
	message  string
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewSpinner creates a new spinner with a default frame interval.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message:  message,
		interval: 100 * time.Millisecond,
	}
}

// Start begins spinner output.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}

	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.running = true

	go func() {
		defer close(s.doneCh)
		frames := []rune{'|', '/', '-', '\\'}
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		idx := 0
		for {
			select {
			case <-ticker.C:
				outputMu.Lock()
				_, _ = fmt.Fprintf(output, "\r  %c  %-20s", frames[idx%len(frames)], s.message)
				outputMu.Unlock()
				idx++
			case <-s.stopCh:
				outputMu.Lock()
				_, _ = fmt.Fprint(output, "\r")
				outputMu.Unlock()
				return
			}
		}
	}()
}

// Stop halts spinner output and optionally replaces it with a final status line.
func (s *Spinner) Stop(finalDetail string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	doneCh := s.doneCh
	s.running = false
	s.mu.Unlock()

	<-doneCh
	if finalDetail != "" {
		Info(s.message, finalDetail)
	}
}

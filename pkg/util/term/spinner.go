package term

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner animates a terminal progress indicator with a message.
type Spinner struct {
	message   string
	initial   string // message at start; non-TTY Success prints only if it changed
	tty       bool
	stop      chan struct{}
	mu        sync.Mutex
	running   bool
	finalized bool
}

// StartSpinner begins the spinner. On non-TTY it prints msg once and does
// not animate; Failure, and Success after an Update, print a final line.
func StartSpinner(msg string) *Spinner {
	if !IsTerminal() {
		fmt.Println(msg)
		return &Spinner{message: msg, initial: msg}
	}

	s := &Spinner{
		message: msg,
		initial: msg,
		tty:     true,
		stop:    make(chan struct{}),
		running: true,
	}
	go s.run()
	return s
}

func (s *Spinner) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if !s.running {
				s.mu.Unlock()
				return
			}
			msg := s.message
			s.mu.Unlock()
			fmt.Printf("\r\x1b[K%s %s", spinnerFrames[i], msg)
			i = (i + 1) % len(spinnerFrames)

		case <-s.stop:
			return
		}
	}
}

// Stop halts animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stop)
	fmt.Print("\r\x1b[K")
}

// Success prints a green ✓ and finalizes the spinner. On non-TTY the start
// line already said it, so it prints only if Update changed the message.
func (s *Spinner) Success() {
	s.mu.Lock()
	if s.finalized {
		s.mu.Unlock()
		return
	}
	s.finalized = true
	msg := s.message
	quiet := !s.tty && msg == s.initial
	s.mu.Unlock()
	if quiet {
		return
	}
	s.Stop()
	fmt.Printf("  %s %s\n", SuccessMark(), msg)
}

// Failure prints a red ✗ and finalizes the spinner.
func (s *Spinner) Failure() {
	s.mu.Lock()
	if s.finalized {
		s.mu.Unlock()
		return
	}
	s.finalized = true
	msg := s.message
	s.mu.Unlock()
	s.Stop()
	fmt.Printf("  %s %s\n", FailureMark(), msg)
}

// Update changes the spinner's message while running.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

func IsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
